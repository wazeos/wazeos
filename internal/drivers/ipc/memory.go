package ipc

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wazeos/wazeos/internal/types"
)

// MemoryIPCDriver implements an in-memory message queue for IPC.
// Messages are stored in memory and lost on restart.
// Suitable for single-instance deployments and testing.
type MemoryIPCDriver struct {
	mu      sync.RWMutex
	topics  map[string]*topic
	maxSize int // Max messages per topic (0 = unlimited)
}

// topic represents an in-memory message queue.
type topic struct {
	name      string
	messages  []*types.IPCMessage
	consumers map[string]*consumerState // group -> state
	config    map[string]string
	createdAt time.Time
	mu        sync.RWMutex
}

// consumerState tracks consumer group position.
type consumerState struct {
	offset    int64
	lastRead  time.Time
	committed int64
}

// NewMemoryIPCDriver creates a new in-memory IPC driver.
func NewMemoryIPCDriver(maxMessagesPerTopic int) *MemoryIPCDriver {
	return &MemoryIPCDriver{
		topics:  make(map[string]*topic),
		maxSize: maxMessagesPerTopic,
	}
}

// Name returns the driver class.
func (m *MemoryIPCDriver) Name() string {
	return "kernel.ipc.memory"
}

// Patterns returns URI patterns this driver handles.
func (m *MemoryIPCDriver) Patterns() []string {
	return []string{"queue://*/*", "ipc://*/*"}
}

// Produce sends a message to a topic.
func (m *MemoryIPCDriver) Produce(ctx context.Context, req *types.IPCProduceRequest) (*types.IPCProduceResult, error) {
	if req == nil || req.Topic == "" {
		return nil, types.ErrInvalidRequest
	}

	m.mu.Lock()
	t, exists := m.topics[req.Topic]
	if !exists {
		// Auto-create topic
		t = &topic{
			name:      req.Topic,
			messages:  make([]*types.IPCMessage, 0),
			consumers: make(map[string]*consumerState),
			config:    make(map[string]string),
			createdAt: time.Now(),
		}
		m.topics[req.Topic] = t
	}
	m.mu.Unlock()

	// Create message
	msg := &types.IPCMessage{
		ID:        uuid.New().String(),
		Topic:     req.Topic,
		Key:       req.Key,
		Headers:   req.Headers,
		Body:      req.Body,
		Timestamp: time.Now(),
	}

	// Add to topic
	t.mu.Lock()
	defer t.mu.Unlock()

	// Enforce max size
	if m.maxSize > 0 && len(t.messages) >= m.maxSize {
		// Remove oldest message
		t.messages = t.messages[1:]
	}

	offset := int64(len(t.messages))
	t.messages = append(t.messages, msg)

	return &types.IPCProduceResult{
		MessageID: msg.ID,
		Topic:     req.Topic,
		Offset:    offset,
	}, nil
}

// Consume receives messages from a topic.
func (m *MemoryIPCDriver) Consume(ctx context.Context, req *types.IPCConsumeRequest) (*types.IPCConsumeResult, error) {
	if req == nil || req.Topic == "" {
		return nil, types.ErrInvalidRequest
	}

	m.mu.RLock()
	t, exists := m.topics[req.Topic]
	m.mu.RUnlock()

	if !exists {
		return &types.IPCConsumeResult{
			Messages: []types.IPCMessage{},
			HasMore:  false,
		}, nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Get or create consumer state
	group := req.Group
	if group == "" {
		group = "default"
	}

	state, exists := t.consumers[group]
	if !exists {
		state = &consumerState{
			offset:   0,
			lastRead: time.Now(),
		}
		t.consumers[group] = state
	}

	// Determine starting offset
	startOffset := state.offset
	if req.StartFrom == "earliest" {
		startOffset = 0
	} else if req.StartFrom == "latest" {
		startOffset = int64(len(t.messages))
	}

	// Determine max count
	maxCount := req.MaxCount
	if maxCount <= 0 {
		maxCount = 100 // Default batch size
	}

	// Collect messages
	messages := make([]types.IPCMessage, 0, maxCount)
	for i := startOffset; i < int64(len(t.messages)) && len(messages) < maxCount; i++ {
		msg := t.messages[i]
		messages = append(messages, *msg)
	}

	// Update consumer state
	if len(messages) > 0 {
		newOffset := startOffset + int64(len(messages))
		state.offset = newOffset
		state.lastRead = time.Now()

		if req.AutoCommit {
			state.committed = newOffset
		}
	}

	hasMore := startOffset+int64(len(messages)) < int64(len(t.messages))

	return &types.IPCConsumeResult{
		Messages: messages,
		HasMore:  hasMore,
	}, nil
}

// CreateTopic creates a new topic.
func (m *MemoryIPCDriver) CreateTopic(ctx context.Context, name string, config map[string]string) error {
	if name == "" {
		return types.ErrInvalidRequest
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.topics[name]; exists {
		return fmt.Errorf("topic %s already exists", name)
	}

	m.topics[name] = &topic{
		name:      name,
		messages:  make([]*types.IPCMessage, 0),
		consumers: make(map[string]*consumerState),
		config:    config,
		createdAt: time.Now(),
	}

	return nil
}

// DeleteTopic deletes a topic.
func (m *MemoryIPCDriver) DeleteTopic(ctx context.Context, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.topics[name]; !exists {
		return types.ErrNotFound
	}

	delete(m.topics, name)
	return nil
}

// ListTopics returns all topics.
func (m *MemoryIPCDriver) ListTopics(ctx context.Context) ([]*types.IPCTopicInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	topics := make([]*types.IPCTopicInfo, 0, len(m.topics))
	for _, t := range m.topics {
		t.mu.RLock()
		info := &types.IPCTopicInfo{
			Name:      t.name,
			Config:    t.config,
			CreatedAt: t.createdAt,
		}
		t.mu.RUnlock()
		topics = append(topics, info)
	}

	return topics, nil
}

// GetTopic returns information about a specific topic.
func (m *MemoryIPCDriver) GetTopic(ctx context.Context, name string) (*types.IPCTopicInfo, error) {
	m.mu.RLock()
	t, exists := m.topics[name]
	m.mu.RUnlock()

	if !exists {
		return nil, types.ErrNotFound
	}

	t.mu.RLock()
	defer t.mu.RUnlock()

	return &types.IPCTopicInfo{
		Name:      t.name,
		Config:    t.config,
		CreatedAt: t.createdAt,
	}, nil
}

// Commit acknowledges message consumption.
func (m *MemoryIPCDriver) Commit(ctx context.Context, topic, group string, offset int64) error {
	m.mu.RLock()
	t, exists := m.topics[topic]
	m.mu.RUnlock()

	if !exists {
		return types.ErrNotFound
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	state, exists := t.consumers[group]
	if !exists {
		return fmt.Errorf("consumer group %s not found", group)
	}

	state.committed = offset
	return nil
}
