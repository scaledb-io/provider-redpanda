// Copyright (C) 2026 The OpenEverest Contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package provider implements the OpenEverest provider for Redpanda via the
// Redpanda Operator (https://docs.redpanda.com/current/deploy/deployment-option/self-hosted/kubernetes/kubernetes-operator/).
//
// IMPORTANT — License notice:
// The Redpanda Operator and Redpanda broker are licensed under the
// Business Source License 1.1 (BSL). This provider plugin (the Go code
// wrapping the operator) is Apache 2.0. Users who deploy Redpanda through
// this provider are responsible for complying with Redpanda's BSL terms.
// See https://github.com/redpanda-data/redpanda-operator/blob/main/licenses/bsl.md
//
// Implementation note:
// We use unstructured Kubernetes objects to create Redpanda CRs so that
// this module has zero direct dependency on the BSL-licensed operator
// codebase. The Redpanda CR is built as a plain map[string]interface{}.
package provider

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/log"

	corev1alpha1 "github.com/openeverest/openeverest/v2/api/core/v1alpha1"
	"github.com/openeverest/openeverest/v2/provider-runtime/controller"

	"github.com/scaledb-io/provider-redpanda/internal/common"
)

// redpandaGVK is the GroupVersionKind for the Redpanda cluster CR.
var redpandaGVK = schema.GroupVersionKind{
	Group:   common.RedpandaGroup,
	Version: common.RedpandaVersion,
	Kind:    common.RedpandaKind,
}

// Compile-time check.
var _ controller.ProviderInterface = (*Provider)(nil)

// Provider implements controller.ProviderInterface for Redpanda via the Redpanda Operator.
type Provider struct {
	controller.BaseProvider
}

// New creates a new Provider instance.
func New() *Provider {
	return &Provider{
		BaseProvider: controller.BaseProvider{
			ProviderName: common.ProviderName,
			// No SchemeFuncs needed — we use unstructured objects to avoid
			// importing the BSL-licensed Redpanda operator module.
			SchemeFuncs: nil,
			// NOTE: We intentionally do NOT watch Redpanda CRs here.
			// Watching them causes a tight feedback loop: operator updates
			// (finalizers, status) re-trigger Apply, which updates the object,
			// which triggers the Redpanda Operator again.
			// Status() polls via c.Get() on each Instance reconcile instead.
			WatchConfigs: []controller.WatchConfig{},
		},
	}
}

// Validate checks the Instance spec before reconciliation.
func (p *Provider) Validate(c *controller.Context) error {
	l := log.FromContext(c.Context())
	l.Info("Validating Redpanda instance", "name", c.Name())

	engine, ok := c.Instance().Spec.Components[common.ComponentEngine]
	if !ok {
		return fmt.Errorf("engine component is required")
	}

	if engine.Resources != nil && engine.Resources.Limits != nil {
		lim := engine.Resources.Limits
		if cpu := lim.Cpu(); cpu != nil && !cpu.IsZero() {
			if cpu.Cmp(resource.MustParse("1")) < 0 {
				return fmt.Errorf("engine CPU limit must be at least 1 core")
			}
		}
		if mem := lim.Memory(); mem != nil && !mem.IsZero() {
			if mem.Cmp(resource.MustParse("2Gi")) < 0 {
				return fmt.Errorf("engine memory limit must be at least 2Gi")
			}
		}
	}

	if c.Instance().GetTopologyType() == common.TopologyReplicated {
		if engine.Replicas != nil && *engine.Replicas < 3 {
			return fmt.Errorf("replicated topology requires at least 3 brokers for Raft quorum")
		}
	}

	return nil
}

// Sync creates or waits on the Redpanda CR for the selected topology.
//
// Create-only semantics: once created, the Redpanda Operator owns the CR and
// we must not overwrite its changes on every reconcile. WaitError is returned
// while provisioning is in progress so the runtime requeues after 15s.
func (p *Provider) Sync(c *controller.Context) error {
	l := log.FromContext(c.Context())
	topology := c.Instance().GetTopologyType()
	l.Info("Syncing Redpanda instance", "name", c.Name(), "topology", topology)

	existing := newRedpandaObj(c.Name(), c.Namespace())
	if err := c.Get(existing, c.Name()); err != nil {
		replicas := brokerReplicas(c)
		rp, buildErr := buildRedpanda(c, replicas)
		if buildErr != nil {
			return fmt.Errorf("build Redpanda CR: %w", buildErr)
		}
		if applyErr := c.Apply(rp); applyErr != nil {
			return fmt.Errorf("create Redpanda CR: %w", applyErr)
		}
		l.Info("Redpanda CR created", "name", c.Name(), "brokers", replicas)
		return controller.WaitForDuration("waiting for Redpanda Operator to provision cluster", 15*time.Second)
	}

	return waitForRedpanda(c, existing)
}

// waitForRedpanda checks the Redpanda CR status conditions and returns a
// WaitError if the cluster is not yet ready.
func waitForRedpanda(c *controller.Context, rp *unstructured.Unstructured) error {
	l := log.FromContext(c.Context())

	ready, msg := redpandaReadyCondition(rp)
	if ready {
		l.Info("Redpanda cluster is Ready", "name", rp.GetName())
		return nil
	}

	l.Info("Redpanda cluster still provisioning", "name", rp.GetName(), "message", msg)
	return controller.WaitForDuration(
		fmt.Sprintf("waiting for Redpanda Operator to complete provisioning: %s", msg),
		15*time.Second,
	)
}

// Status reports the current status of the Redpanda instance.
func (p *Provider) Status(c *controller.Context) (controller.Status, error) {
	rp := newRedpandaObj(c.Name(), c.Namespace())
	if err := c.Get(rp, c.Name()); err != nil {
		return controller.Provisioning("Waiting for Redpanda CR"), nil
	}

	ready, msg := redpandaReadyCondition(rp)
	if ready {
		return controller.ReadyWithConnectionDetails(buildConnectionDetails(c)), nil
	}

	return controller.Provisioning(fmt.Sprintf("Cluster is being created: %s", msg)), nil
}

// Cleanup removes the Redpanda CR when the Instance is deleted.
func (p *Provider) Cleanup(c *controller.Context) error {
	l := log.FromContext(c.Context())
	l.Info("Cleaning up Redpanda instance", "name", c.Name())

	rp := newRedpandaObj(c.Name(), c.Namespace())
	if err := c.Delete(rp); err != nil {
		return fmt.Errorf("delete Redpanda CR: %w", err)
	}

	l.Info("Redpanda instance cleaned up", "name", c.Name())
	return nil
}

// =============================================================================
// Builder
// =============================================================================

// buildRedpanda constructs an unstructured Redpanda CR for the Redpanda Operator.
// We use unstructured to avoid importing the BSL-licensed operator module.
//
// Resulting CR (cluster.redpanda.com/v1alpha2):
//
//	spec:
//	  chartRef:
//	    useFlux: false     # use operator's embedded chart, not Flux
//	  clusterSpec:
//	    statefulset:
//	      replicas: <n>
//	    resources:
//	      cpu:
//	        cores: <n>     # sets --smp + CPU request+limit (Redpanda-specific)
//	      requests:
//	        memory: <qty> # standard k8s memory request
//	      limits:
//	        memory: <qty> # equals request → Guaranteed QoS
//	    storage:
//	      persistentVolume:
//	        enabled: true
//	        size: <quantity>
//	        storageClass: <optional>
func buildRedpanda(c *controller.Context, replicas int) (*unstructured.Unstructured, error) {
	engine := c.Instance().Spec.Components[common.ComponentEngine]
	image, err := resolveImage(c, engine)
	if err != nil {
		return nil, err
	}

	repo, tag := splitImage(image)
	cpu, memory := resolveResources(engine)
	storageSize, storageClass := resolveStorage(engine)

	// Build storage spec.
	pvSpec := map[string]interface{}{
		"enabled": true,
		"size":    storageSize.String(),
	}
	if storageClass != nil && *storageClass != "" {
		pvSpec["storageClass"] = *storageClass
	}

	// Build the full clusterSpec.
	//
	// Resource spec for v1alpha2 (verified against Redpanda docs, 2026-06-07):
	//   resources.cpu.cores   — Redpanda-specific; sets --smp and both CPU request+limit
	//   resources.requests.memory — standard k8s memory request
	//   resources.limits.memory   — standard k8s memory limit (set equal → Guaranteed QoS)
	//
	// The older v1alpha1 structure (resources.memory.container.max/min) is NOT used in v1alpha2.
	clusterSpec := map[string]interface{}{
		"statefulset": map[string]interface{}{
			"replicas": int64(replicas),
		},
		"resources": map[string]interface{}{
			"cpu": map[string]interface{}{
				// Redpanda uses a thread-per-core (TPC) model via Seastar's --smp flag.
				// resources.cpu.cores sets --smp and both CPU request+limit simultaneously.
				"cores": cpu.String(),
			},
			// Standard k8s memory request/limit (set equal to enforce Guaranteed QoS).
			"requests": map[string]interface{}{
				"memory": memory.String(),
			},
			"limits": map[string]interface{}{
				"memory": memory.String(),
			},
		},
		"storage": map[string]interface{}{
			"persistentVolume": pvSpec,
		},
	}

	// Optionally set the image. When the operator embeds its own Helm chart
	// (useFlux: false), the default image repository is the official Redpanda
	// image. We only override when the version deviates from the chart default.
	if repo != "" || tag != "" {
		imageSpec := map[string]interface{}{}
		if repo != "" {
			imageSpec["repository"] = repo
		}
		if tag != "" {
			imageSpec["tag"] = tag
		}
		clusterSpec["image"] = imageSpec
	}

	rp := newRedpandaObj(c.Name(), c.Namespace())
	rp.Object["spec"] = map[string]interface{}{
		"chartRef": map[string]interface{}{
			// useFlux: false — operator uses its embedded Helm chart directly.
			// All other chartRef fields are ignored when useFlux is false.
			"useFlux": false,
		},
		"clusterSpec": clusterSpec,
	}

	return rp, nil
}

// newRedpandaObj creates an empty unstructured Redpanda CR with the correct GVK.
func newRedpandaObj(name, namespace string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(redpandaGVK)
	u.SetName(name)
	u.SetNamespace(namespace)
	return u
}

// =============================================================================
// Status helpers
// =============================================================================

// redpandaReadyCondition inspects the Redpanda CR status conditions.
// The Redpanda Operator uses standard metav1.Condition with Type="Ready".
func redpandaReadyCondition(rp *unstructured.Unstructured) (bool, string) {
	conditions, found, err := unstructured.NestedSlice(rp.Object, "status", "conditions")
	if err != nil || !found {
		return false, "waiting for status conditions"
	}
	for _, raw := range conditions {
		cond, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _, _ := unstructured.NestedString(cond, "type")
		if condType != "Ready" {
			continue
		}
		status, _, _ := unstructured.NestedString(cond, "status")
		if status == "True" {
			return true, ""
		}
		msg, _, _ := unstructured.NestedString(cond, "message")
		return false, msg
	}
	return false, "Ready condition not yet reported"
}

// =============================================================================
// Connection details
// =============================================================================

// buildConnectionDetails returns the Redpanda Kafka-compatible bootstrap endpoint.
// The Redpanda Operator creates a ClusterIP Service named after the instance.
// Internal Kafka bootstrap: <name>.<namespace>.svc:9092
func buildConnectionDetails(c *controller.Context) controller.ConnectionDetails {
	host := fmt.Sprintf("%s.%s.svc", c.Name(), c.Namespace())
	return controller.ConnectionDetails{
		Type:     "redpanda",
		Provider: common.ProviderName,
		Host:     host,
		Port:     common.KafkaPort,
		URI:      fmt.Sprintf("%s:%s", host, common.KafkaPort),
		AdditionalProperties: map[string]string{
			"adminAPI":       fmt.Sprintf("http://%s:%s", host, common.AdminPort),
			"schemaRegistry": fmt.Sprintf("http://%s:%s", host, common.SchemaRegistryPort),
		},
	}
}

// =============================================================================
// Helpers
// =============================================================================

// brokerReplicas returns the configured replica count or the topology default.
func brokerReplicas(c *controller.Context) int {
	engine := c.Instance().Spec.Components[common.ComponentEngine]
	if engine.Replicas != nil && *engine.Replicas > 0 {
		return int(*engine.Replicas)
	}
	if c.Instance().GetTopologyType() == common.TopologyReplicated {
		return common.DefaultReplicatedReplicas
	}
	return common.DefaultStandaloneReplicas
}

// resolveImage returns the full container image for the engine component.
func resolveImage(c *controller.Context, engine corev1alpha1.ComponentSpec) (string, error) {
	if engine.Image != "" {
		return engine.Image, nil
	}
	spec, err := c.ProviderSpec()
	if err != nil {
		return "", fmt.Errorf("get provider spec: %w", err)
	}
	if engine.Version != "" {
		if img := controller.GetImageForVersion(spec, common.ComponentEngine, engine.Version); img != "" {
			return img, nil
		}
	}
	if img := controller.GetDefaultImageForComponent(spec, common.ComponentEngine); img != "" {
		return img, nil
	}
	return "", fmt.Errorf("no image found for engine component")
}

// splitImage splits a full image reference (e.g. "docker.redpanda.com/org/redpanda:v25.1.8")
// into its repository and tag components.
func splitImage(image string) (repo, tag string) {
	if idx := strings.LastIndex(image, ":"); idx >= 0 {
		return image[:idx], image[idx+1:]
	}
	return image, ""
}

// resolveResources returns CPU and memory quantities with defaults applied.
func resolveResources(engine corev1alpha1.ComponentSpec) (cpu, memory resource.Quantity) {
	cpu = resource.MustParse("1")
	memory = resource.MustParse("2Gi")
	if engine.Resources == nil || engine.Resources.Limits == nil {
		return
	}
	if v := engine.Resources.Limits.Cpu(); v != nil && !v.IsZero() {
		cpu = v.DeepCopy()
	}
	if v := engine.Resources.Limits.Memory(); v != nil && !v.IsZero() {
		memory = v.DeepCopy()
	}
	return
}

// resolveStorage returns the storage size and optional storage class.
func resolveStorage(engine corev1alpha1.ComponentSpec) (size resource.Quantity, storageClass *string) {
	size = resource.MustParse("20Gi")
	if engine.Storage == nil {
		return
	}
	if !engine.Storage.Size.IsZero() {
		size = engine.Storage.Size.DeepCopy()
	}
	storageClass = engine.Storage.StorageClass
	return
}
