// Package common defines shared constants used across the provider.
package common

const (
	// ProviderName is the canonical name of this provider.
	// Must match the Provider CR name registered in OpenEverest.
	ProviderName = "provider-redpanda"

	// ComponentEngine is the logical name of the Redpanda engine component.
	ComponentEngine = "engine"

	// ComponentTypeRedpanda is the component type name, matching versions.yaml.
	ComponentTypeRedpanda = "redpanda"

	// TopologyStandalone is the single-broker topology name.
	TopologyStandalone = "standalone"

	// TopologyReplicated is the replicated topology name (3+ brokers, Raft quorum).
	TopologyReplicated = "replicated"

	// DefaultStandaloneReplicas is the broker count for the standalone topology.
	DefaultStandaloneReplicas = 1

	// DefaultReplicatedReplicas is the default broker count for the replicated topology.
	// Minimum 3 for Raft quorum.
	DefaultReplicatedReplicas = 3

	// KafkaPort is the Kafka-compatible API port exposed by Redpanda.
	KafkaPort = "9092"

	// AdminPort is the Redpanda Admin API port.
	AdminPort = "9644"

	// SchemaRegistryPort is the Schema Registry port (built-in to Redpanda).
	SchemaRegistryPort = "8081"

	// RedpandaGroup is the Kubernetes API group for the Redpanda cluster CR.
	// Source: github.com/redpanda-data/redpanda-operator/operator/api/redpanda/v1alpha2
	RedpandaGroup = "cluster.redpanda.com"

	// RedpandaVersion is the Kubernetes API version for the Redpanda cluster CR.
	RedpandaVersion = "v1alpha2"

	// RedpandaKind is the Kubernetes kind for the Redpanda cluster CR.
	RedpandaKind = "Redpanda"
)
