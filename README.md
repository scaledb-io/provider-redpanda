# provider-redpanda

An [OpenEverest](https://github.com/openeverest/openeverest) provider for [Redpanda](https://redpanda.com),
built on the [Redpanda Operator](https://docs.redpanda.com/current/deploy/deployment-option/self-hosted/kubernetes/kubernetes-operator/).

## ⚠️ License Notice

Redpanda and the Redpanda Operator are licensed under the [Business Source License 1.1 (BSL)](https://github.com/redpanda-data/redpanda-operator/blob/main/licenses/bsl.md).
This provider plugin is **Apache 2.0**. Users who deploy Redpanda clusters are responsible for complying with Redpanda's BSL terms.

If you need a fully open-source Kafka-compatible broker, consider [`provider-strimzi-kafka`](https://github.com/scaledb-io/provider-strimzi-kafka) (Apache Kafka + Strimzi, both Apache 2.0).

## Prerequisites

- Go 1.26+
- Kubernetes cluster (k3d, kind, or remote)
- [OpenEverest](https://github.com/openeverest/openeverest) installed
- [Redpanda Operator](https://docs.redpanda.com/current/deploy/deployment-option/self-hosted/kubernetes/kubernetes-operator/) installed

```bash
helm repo add redpanda-operator https://charts.redpanda.com && helm repo update redpanda-operator

# --set crds.enabled=true is required — CRDs are not in the chart's crds/ directory
helm install redpanda-operator redpanda-operator/operator \
  --namespace redpanda-operator \
  --create-namespace \
  --set fullnameOverride=redpanda-operator \
  --set crds.enabled=true
```

> **CRDs not auto-installed:** The Helm chart has no `crds/` directory. Pass
> `--set crds.enabled=true` or the operator pod will crash with:
> `no matches for kind "Console" in version "cluster.redpanda.com/v1alpha2"`.
> If the pod crashed before CRDs were created, force a restart after upgrading:
> ```bash
> kubectl rollout restart deployment/redpanda-operator -n redpanda-operator
> ```

> **TLS requires cert-manager:** The operator enables TLS by default. Without
> cert-manager, pods get stuck in `Init:0/3` waiting for secrets
> `<name>-default-cert` and `<name>-external-cert`. Either install cert-manager
> or disable TLS via `spec.clusterSpec.tls.enabled: false`.

## Supported Topologies

| Topology     | Description                                              | Status       |
|--------------|----------------------------------------------------------|--------------|
| `standalone` | Single broker, development/testing                       | ✅ Available |
| `replicated` | 3+ brokers, Raft quorum, production HA                   | ✅ Available |

### standalone

Single Redpanda broker. No coordination dependency. Suitable for development,
testing, and low-volume CDC pipelines.

### replicated

Three or more Redpanda brokers forming a Raft quorum. Redpanda uses its own
metadata store — no ZooKeeper or external coordination service required.
Supports `RF:3` replication for fault tolerance (survives one broker loss).

## Supported Versions

| Version | Default |
|---------|---------|
| `25.1.8` | ✅ Yes |
| `24.3.12` | |

## Quick Start

```bash
# Generate all manifests (RBAC, provider spec, Helm chart)
make generate

# Run the provider locally against your cluster
make run

# Or deploy via Helm
helm install provider-redpanda charts/provider-redpanda/ --create-namespace
```

## Development

### Project Structure

```
cmd/provider/              # Entry point
internal/
  provider/
    provider.go            # ProviderInterface implementation (Validate/Sync/Status/Cleanup)
    rbac.go                # Kubebuilder RBAC markers
  common/
    spec.go                # Component name constants and port definitions
definition/
  provider.yaml            # Provider name + component→type mapping
  versions.yaml            # Component type version/image catalog
  topologies/
    standalone/
      topology.yaml        # Single-broker topology config + UI schema
    replicated/
      topology.yaml        # 3-broker HA topology config + UI schema
config/
  rbac/
    role.yaml              # Generated ClusterRole (do not edit manually)
charts/provider-redpanda/  # Helm chart for deploying the provider
  generated/
    rbac-rules.yaml        # Generated RBAC rules (do not edit manually)
    provider-spec.yaml     # Generated Provider CR spec (do not edit manually)
examples/                  # Example Instance CRs
Makefile                   # Build, generate, and deploy targets
```

### Make Targets

| Target          | Description                                                |
|-----------------|------------------------------------------------------------|
| `make generate` | Run all code generation (RBAC + Helm sync + provider spec) |
| `make run`      | Run the provider locally                                   |
| `make build`    | Build the provider binary                                  |
| `make lint`     | Run golangci-lint                                          |
| `make verify`   | Check generated files are up-to-date (CI)                  |

> For development patterns (RBAC, watches, code generation), see
> [PROVIDER_DEVELOPMENT.md](https://github.com/openeverest/provider-sdk/blob/main/PROVIDER_DEVELOPMENT.md).

### Known Gotcha — `--smp` flag required (Operator v26.x)

Redpanda Operator v26.x does **not** translate `resources.cpu.cores` into a
container CPU limit. Seastar auto-detects all CPUs on the node — on a 16-core
host this creates 16 shards at ~140 MiB each, which crashes Redpanda (minimum
~1.5 GiB/shard in production mode).

The provider handles this automatically via a `cpuToSMP()` helper that computes
the correct `--smp=N` value and injects it via
`statefulset.additionalRedpandaCmdFlags`. No manual action required.

### Known Gotcha — Kafka internal port is 9093, not 9092

The Redpanda Helm chart (embedded in the operator) exposes two Kafka listeners:

| Listener  | Port | Use case |
|-----------|------|----------|
| `internal` | 9093 | In-cluster clients (Debezium, Kafka Connect, etc.) |
| `external` | 9094 | Out-of-cluster access via NodePort |

Clients inside the cluster (Debezium, Kafka Connect) should use `:9093`.

## Connection Details

Once the cluster is `Ready`, OpenEverest exposes:

| Field            | Value |
|------------------|-------|
| `host`           | `<instance>.<namespace>.svc` |
| `port`           | `9093` (Kafka internal listener) |
| `uri`            | `<instance>.<namespace>.svc:9093` |
| `adminAPI`       | `http://<instance>.<namespace>.svc:9644` |
| `schemaRegistry` | `http://<instance>.<namespace>.svc:8081` |
| `console`        | `http://<instance>-console.<namespace>.svc:8080` |

The Redpanda Console is deployed automatically by the operator alongside the
cluster — no separate installation needed.

## Architecture

```
OpenEverest Instance CR
        │
        ▼
  provider-redpanda
  (this plugin)
        │  creates
        ▼
Redpanda CR (cluster.redpanda.com/v1alpha2)
        │
        ▼
  Redpanda Operator
        │  reconciles
        ▼
Redpanda StatefulSet + Services + Console Deployment
```

The provider uses `k8s.io/apimachinery/pkg/apis/meta/v1/unstructured` to create
Redpanda CRs — no BSL-licensed operator module in the import graph. The provider
itself stays Apache 2.0.

## Related

- [openeverest/openeverest#2335](https://github.com/openeverest/openeverest/issues/2335) — tracking issue
- [OpenEverest Provider SDK](https://github.com/openeverest/provider-sdk)
- [Provider spec-001](https://github.com/openeverest/specs/blob/main/specs/001-plugins-architecture.md)
- [`provider-strimzi-kafka`](https://github.com/scaledb-io/provider-strimzi-kafka) — Apache 2.0 alternative

## License

Apache License 2.0 — see [LICENSE](LICENSE) for details.
