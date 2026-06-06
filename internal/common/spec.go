// Package common defines shared constants used across the provider.
package common

const (
	// ProviderName is the canonical name of this provider.
	// Must match the Provider CR name registered in OpenEverest.
	ProviderName = "provider-strimzi-kafka"

	// ComponentEngine is the logical name of the Kafka engine component.
	ComponentEngine = "engine"

	// ComponentTypeKafka is the component type name, matching versions.yaml.
	ComponentTypeKafka = "kafka"

	// TopologyStandalone is the single-broker topology name.
	TopologyStandalone = "standalone"

	// TopologyReplicated is the replicated topology name (3+ brokers, KRaft quorum).
	TopologyReplicated = "replicated"

	// KafkaClusterName is the cluster name used inside the Kafka CR.
	// Strimzi uses this as part of the pod and service naming scheme.
	KafkaClusterName = "kafka"

	// DefaultStandaloneReplicas is the broker count for the standalone topology.
	DefaultStandaloneReplicas = 1

	// DefaultReplicatedReplicas is the default broker count for the replicated topology.
	// Minimum 3 for Raft quorum and replication factor safety.
	DefaultReplicatedReplicas = 3

	// BootstrapPort is the plain (non-TLS) Kafka client port exposed by Strimzi.
	BootstrapPort = "9092"

	// KafkaMetadataVersion4_2 is the KRaft metadata version for Kafka 4.2.x.
	KafkaMetadataVersion4_2 = "4.2-IV0"

	// KafkaMetadataVersion4_1 is the KRaft metadata version for Kafka 4.1.x.
	KafkaMetadataVersion4_1 = "4.1-IV0"

	// DefaultMetadataVersion is used when the version-specific value is not resolved.
	DefaultMetadataVersion = KafkaMetadataVersion4_2
)
