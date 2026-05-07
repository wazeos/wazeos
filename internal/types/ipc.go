package types

import (
	"context"
	"time"
)

// IPCMessage represents a message in the IPC system.
type IPCMessage struct {
	ID        string            `json:"id"`                  // Unique message ID
	Topic     string            `json:"topic"`               // Topic/queue name
	Key       string            `json:"key,omitempty"`       // Optional partition key
	Headers   map[string]string `json:"headers,omitempty"`   // Message headers/metadata
	Body      []byte            `json:"body"`                // Message payload
	Timestamp time.Time         `json:"timestamp,omitempty"` // Message timestamp
}

// IPCProduceRequest represents a request to produce/send a message.
type IPCProduceRequest struct {
	Topic   string            `json:"topic"`             // Target topic/queue
	Key     string            `json:"key,omitempty"`     // Optional partition key
	Headers map[string]string `json:"headers,omitempty"` // Message headers
	Body    []byte            `json:"body"`              // Message payload
}

// IPCProduceResult represents the result of producing a message.
type IPCProduceResult struct {
	MessageID string `json:"messageId"` // Assigned message ID
	Topic     string `json:"topic"`     // Topic/queue name
	Offset    int64  `json:"offset"`    // Position in queue (driver-specific)
}

// IPCConsumeRequest represents a request to consume messages.
type IPCConsumeRequest struct {
	Topic      string        `json:"topic"`                // Topic/queue to consume from
	Group      string        `json:"group,omitempty"`      // Consumer group ID
	MaxCount   int           `json:"maxCount,omitempty"`   // Max messages to return (0 = default)
	Timeout    time.Duration `json:"timeout,omitempty"`    // How long to wait for messages
	StartFrom  string        `json:"startFrom,omitempty"`  // "earliest", "latest", or offset
	AutoCommit bool          `json:"autoCommit,omitempty"` // Auto-acknowledge messages
}

// IPCConsumeResult represents the result of consuming messages.
type IPCConsumeResult struct {
	Messages []IPCMessage `json:"messages"` // Consumed messages
	HasMore  bool         `json:"hasMore"`  // More messages available
}

// IPCTopicInfo represents metadata about a topic/queue.
type IPCTopicInfo struct {
	Name       string            `json:"name"`                 // Topic name
	Partitions int               `json:"partitions,omitempty"` // Number of partitions
	Replicas   int               `json:"replicas,omitempty"`   // Replication factor
	Config     map[string]string `json:"config,omitempty"`     // Topic configuration
	CreatedAt  time.Time         `json:"createdAt,omitempty"`  // Creation timestamp
}

// IPCDriver is the interface for IPC/message queue drivers.
// Drivers implement this to provide message passing capabilities.
// Examples: in-memory queue, Kafka, RabbitMQ, Redis streams, etc.
type IPCDriver interface {
	// Name returns the driver class (e.g., "kernel.ipc.memory", "kernel.ipc.kafka").
	Name() string

	// Patterns returns URI patterns this driver handles.
	// Examples: "queue://*/*", "ipc://*/*", "kafka://*/*"
	Patterns() []string

	// Produce sends a message to a topic/queue.
	Produce(ctx context.Context, req *IPCProduceRequest) (*IPCProduceResult, error)

	// Consume receives messages from a topic/queue.
	Consume(ctx context.Context, req *IPCConsumeRequest) (*IPCConsumeResult, error)

	// CreateTopic creates a new topic/queue.
	CreateTopic(ctx context.Context, name string, config map[string]string) error

	// DeleteTopic deletes a topic/queue.
	DeleteTopic(ctx context.Context, name string) error

	// ListTopics returns information about all topics/queues.
	ListTopics(ctx context.Context) ([]*IPCTopicInfo, error)

	// GetTopic returns information about a specific topic/queue.
	GetTopic(ctx context.Context, name string) (*IPCTopicInfo, error)

	// Commit acknowledges message consumption (for non-auto-commit scenarios).
	Commit(ctx context.Context, topic, group string, offset int64) error
}

// IPCBus manages IPC drivers and routes IPC operations.
type IPCBus interface {
	// RegisterDriver adds an IPC driver to the bus.
	RegisterDriver(driver IPCDriver) error

	// Produce sends a message through the appropriate driver.
	Produce(ctx context.Context, uri string, req *IPCProduceRequest) (*IPCProduceResult, error)

	// Consume receives messages through the appropriate driver.
	Consume(ctx context.Context, uri string, req *IPCConsumeRequest) (*IPCConsumeResult, error)

	// CreateTopic creates a topic through the appropriate driver.
	CreateTopic(ctx context.Context, uri string, config map[string]string) error

	// DeleteTopic deletes a topic through the appropriate driver.
	DeleteTopic(ctx context.Context, uri string) error

	// ListTopics lists topics from the appropriate driver.
	ListTopics(ctx context.Context, uri string) ([]*IPCTopicInfo, error)
}
