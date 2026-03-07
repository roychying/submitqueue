// Copyright (c) 2025 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mysql

//go:generate mockgen -source=stores.go -destination=mock_stores.go -package=mysql

import (
	"context"

	"github.com/uber/submitqueue/entity/queue"
)

const (
	// Fixed table names for single-table design
	MessagesTableName             = "queue_messages"
	PartitionLeasesTableName      = "queue_partition_leases"
	OffsetsTableName              = "queue_offsets"
	DLQTableName                  = "queue_dlq"
	SubscriberHeartbeatsTableName = "queue_subscriber_heartbeats"
	DeliveryStateTableName        = "queue_delivery_state"
)

// messageRow represents a row from the messages table (internal use only)
type messageRow struct {
	// Offset is the auto-incrementing sequence number for message ordering within a partition
	Offset int64
	// ID is the unique message identifier
	ID string
	// Payload is the message body in bytes
	Payload []byte
	// Metadata contains key-value pairs for message attributes
	Metadata map[string]string
	// PartitionKey determines which partition this message belongs to for ordering guarantees
	PartitionKey string
	// PublishedAt is the Unix timestamp in milliseconds when message was published
	PublishedAt int64
	// FailedAt is the Unix timestamp in milliseconds when the message failed (0 for normal messages, >0 for DLQ)
	FailedAt int64
	// FailureCount tracks total failures before moving to DLQ (0 for normal messages, >0 for DLQ)
	FailureCount int
	// LastError contains the error message from the final failure ("" for normal messages)
	LastError string
	// OriginalTopic is the topic where the message originally failed ("" for normal messages)
	OriginalTopic string
}

// messageStore handles message table operations (internal use only)
type messageStore interface {
	// Insert inserts messages into the topic table
	Insert(ctx context.Context, topic string, messages []queue.Message) error

	// Delete deletes a message by topic, partition key, and ID
	Delete(ctx context.Context, topic string, partitionKey string, messageID string) error

	// FetchByOffset fetches messages with offset > currentOffset for a specific partition.
	// Messages are returned from the immutable log; per-consumer-group visibility
	// is handled by the deliveryStateStore.
	FetchByOffset(ctx context.Context, topic string, partitionKey string, currentOffset int64, limit int) ([]messageRow, error)

	// MoveToDLQ moves a message to the dead letter queue
	// dlqTopicSuffix is appended to the original topic to form the DLQ topic name
	MoveToDLQ(ctx context.Context, topic string, partitionKey string, messageID string, failureCount int, lastError string, dlqTopicSuffix string) error

	// GarbageCollect deletes messages that all consumer groups have acked past.
	// Returns the number of rows deleted.
	GarbageCollect(ctx context.Context, topic string, partitionKey string) (int64, error)

	// GetOffsetsAbove returns message offsets above afterOffset for a partition,
	// ordered ascending. Used by the subscriber to drive watermark advancement
	// without requiring a cross-table JOIN in the delivery state store.
	GetOffsetsAbove(ctx context.Context, topic string, partitionKey string, afterOffset int64) ([]int64, error)
}

// offsetStore handles offset table operations for per-partition offset tracking (internal use only)
type offsetStore interface {
	// Initialize creates an offset entry for a topic+partition if it doesn't exist
	Initialize(ctx context.Context, topic string, partitionKey string, consumerGroup string) error

	// GetAckedOffset returns the current acked offset for a topic+partition
	GetAckedOffset(ctx context.Context, topic string, partitionKey string, consumerGroup string) (int64, error)

	// UpdateAckedOffset updates the offset_acked for a topic+partition (only if new offset is greater)
	UpdateAckedOffset(ctx context.Context, topic string, partitionKey string, offset int64, consumerGroup string) error
}

// partitionLeaseStore handles partition lease operations (internal use only)
type partitionLeaseStore interface {
	// TryAcquireLease attempts to acquire or renew a lease for a partition
	// Returns true if lease is acquired/owned by this worker
	// leaseDurationMs is how long the lease is valid (in milliseconds)
	TryAcquireLease(ctx context.Context, topic string, partitionKey string, subscriberName string, consumerGroup string, leaseDurationMs int64) (bool, error)

	// RenewLease renews the lease for a partition owned by this worker
	// leaseDurationMs is how long the lease is valid (in milliseconds)
	RenewLease(ctx context.Context, topic string, partitionKey string, subscriberName string, consumerGroup string, leaseDurationMs int64) error

	// ReleaseLease releases the lease for a partition owned by this worker
	ReleaseLease(ctx context.Context, topic string, partitionKey string, subscriberName string, consumerGroup string) error

	// GetLeasedPartitions returns all partitions currently leased by this worker
	GetLeasedPartitions(ctx context.Context, topic string, subscriberName string, consumerGroup string) ([]string, error)

	// DiscoverAndAcquirePartitions discovers partitions from messages table and tries to acquire leases
	// Returns the number of new leases acquired
	// leaseDurationMs is how long the lease is valid (in milliseconds)
	// maxPartitions limits how many total partitions this subscriber can own (0 = unlimited)
	DiscoverAndAcquirePartitions(ctx context.Context, topic string, subscriberName string, consumerGroup string, leaseDurationMs int64, maxPartitions int) (int, error)

	// DiscoverPartitions returns all distinct partition keys for a topic
	DiscoverPartitions(ctx context.Context, topic string) ([]string, error)
}

// subscriberHeartbeatStore handles subscriber heartbeat operations for fair partition leasing (internal use only)
type subscriberHeartbeatStore interface {
	// Heartbeat registers or renews a subscriber's heartbeat
	Heartbeat(ctx context.Context, topic string, subscriberName string, consumerGroup string) error

	// ActiveSubscribers returns the names of subscribers with a recent heartbeat.
	// staleDurationMs defines the staleness threshold: subscribers without a heartbeat
	// within this duration are considered dead.
	ActiveSubscribers(ctx context.Context, topic string, consumerGroup string, staleDurationMs int64) ([]string, error)

	// Deregister removes a subscriber's heartbeat entry
	Deregister(ctx context.Context, topic string, subscriberName string, consumerGroup string) error
}

// deliveryStateStore handles per-consumer-group delivery tracking (internal use only)
type deliveryStateStore interface {
	// MarkDelivered inserts a row marking message as in-flight for this consumer group.
	MarkDelivered(ctx context.Context, consumerGroup, topic, partitionKey string, offset int64, visibilityTimeoutMs int64) error

	// MarkAcked sets invisible_until = MAX to indicate this group has processed the message.
	MarkAcked(ctx context.Context, consumerGroup, topic, partitionKey string, offset int64) error

	// MarkNacked sets invisible_until = now + delay to schedule redelivery.
	MarkNacked(ctx context.Context, consumerGroup, topic, partitionKey string, offset int64, delayMs int64) error

	// GetRetryCount returns the retry count for a specific message and consumer group.
	GetRetryCount(ctx context.Context, consumerGroup, topic, partitionKey string, offset int64) (int, error)

	// IsDeliverable checks if a message offset is deliverable for this consumer group.
	// Returns true if the message has no delivery state (new) or if invisible_until <= now.
	// Returns false if acked (invisible_until = MAX) or still invisible.
	IsDeliverable(ctx context.Context, consumerGroup, topic, partitionKey string, offset int64) (bool, error)

	// AdvanceWatermark computes the new contiguous acked watermark and cleans up
	// delivery state rows behind it.
	// offsets are the actual message offsets above the current watermark (from messageStore).
	// Returns the new watermark (highest contiguous acked offset from currentWatermark).
	AdvanceWatermark(ctx context.Context, consumerGroup, topic, partitionKey string, currentWatermark int64, offsets []int64) (int64, error)
}
