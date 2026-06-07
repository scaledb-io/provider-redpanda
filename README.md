# provider-redpanda

An [OpenEverest](https://github.com/openeverest/openeverest) provider plugin for [Redpanda](https://redpanda.com) — a Kafka-compatible streaming platform.

Wraps the **[Redpanda Operator](https://docs.redpanda.com/current/deploy/deployment-option/self-hosted/kubernetes/kubernetes-operator/)** to provision and manage Redpanda clusters via the standard OpenEverest `Instance` API.

---

## ⚠️ License Notice

Redpanda and its operator are licensed under the [Business Source License 1.1 (BSL)](https://github.com/redpanda-data/redpanda-operator/blob/main/licenses/bsl.md).

This provider plugin (the Go code wrapping the operator) is **Apache 2.0**. Users who deploy Redpanda through this provider are responsible for complying with Redpanda's BSL terms.

If you need a fully open-source Kafka-compatible broker, consider [`provider-strimzi-kafka`](https://github.com/scaledb-io/provider-strimzi-kafka) (Apache Kafka + Strimzi, both Apache 2.0).

---

## Features

- **Kafka-compatible API** — drop-in replacement for Kafka clients (port 9092)
- **Built-in Schema Registry** (port 8081) — no extra operator needed
- **Built-in Admin API** (port 9644) — `rpk` and HTTP clients
- **No ZooKeeper** — Redpanda uses its own Raft-based metadata store
- **Two topologies:**
  - `standalone` — single broker, development/testing
  - `replicated` — 3+ brokers, Raft quorum, production HA

---

## Supported Versions

| Redpanda | Image | Default |
|----------|-------|---------|
| 25.1.8 | `docker.redpanda.com/redpandadata/redpanda:v25.1.8` | ✅ |
| 24.3.12 | `docker.redpanda.com/redpandadata/redpanda:v24.3.12` | |

---

## Prerequisites

- OpenEverest v2 (provider plugin architecture, spec-001)
- **[Redpanda Operator](https://docs.redpanda.com/current/deploy/deployment-option/self-hosted/kubernetes/kubernetes-operator/)** installed in the target namespace

### Install the Redpanda Operator

```bash
helm repo add redpanda https://charts.redpanda.com && helm repo update
helm install redpanda-operator redpanda/operator \
  --namespace redpanda-system \
  --create-namespace \
  --set watchNamespaces='{everest-dev}'
```

---

## Install

```bash
helm install provider-redpanda charts/provider-redpanda \
  --namespace everest-dev \
  --create-namespace
```

---

## Usage

### Standalone (single broker)

```yaml
apiVersion: core.openeverest.io/v1alpha1
kind: Instance
metadata:
  name: my-redpanda
  namespace: everest-dev
spec:
  provider: provider-redpanda
  topology: standalone
  components:
    engine:
      replicas: 1
      resources:
        limits:
          cpu: "1"
          memory: 2Gi
      storage:
        size: 20Gi
```

### Replicated (3+ brokers, HA)

```yaml
apiVersion: core.openeverest.io/v1alpha1
kind: Instance
metadata:
  name: my-redpanda-ha
  namespace: everest-dev
spec:
  provider: provider-redpanda
  topology: replicated
  components:
    engine:
      replicas: 3
      resources:
        limits:
          cpu: "2"
          memory: 4Gi
      storage:
        size: 50Gi
```

---

## Connection Details

Once the cluster is `Ready`, OpenEverest exposes:

| Field | Value |
|-------|-------|
| `host` | `<instance>.<namespace>.svc` |
| `port` | `9092` (Kafka API) |
| `uri` | `<instance>.<namespace>.svc:9092` |
| `adminAPI` | `http://<instance>.<namespace>.svc:9644` |
| `schemaRegistry` | `http://<instance>.<namespace>.svc:8081` |

---

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
Redpanda StatefulSet + Services
```

---

## Development

```bash
# Build
make build

# Run locally (against a live cluster)
make run

# Lint
make lint

# Generate RBAC + provider spec
make generate
```

---

## Repository Structure

```
cmd/provider/main.go       # Entry point
internal/
  common/spec.go           # Shared constants (GVK, ports, topology names)
  provider/
    provider.go            # Validate / Sync / Status / Cleanup
    rbac.go                # Kubebuilder RBAC markers for Redpanda CRDs
definition/
  provider.yaml            # Provider identity (name, component types)
  versions.yaml            # Supported Redpanda versions and images
  topologies/
    standalone/            # Single-broker topology
    replicated/            # 3-broker HA topology
charts/provider-redpanda/  # Helm chart for deploying the provider
examples/                  # Example Instance CRs
```

---

## Related

- [openeverest/openeverest#2335](https://github.com/openeverest/openeverest/issues/2335) — tracking issue
- [OpenEverest Provider SDK](https://github.com/openeverest/provider-sdk)
- [Provider spec-001](https://github.com/openeverest/specs/blob/main/specs/001-plugins-architecture.md)
- [`provider-strimzi-kafka`](https://github.com/scaledb-io/provider-strimzi-kafka) — Apache 2.0 alternative
