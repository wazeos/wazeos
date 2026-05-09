package iobus

import (
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

// ============================================================================
// Session Manager - Handle Lifecycle Management
// ============================================================================

// SessionManager manages handle lifecycle and cleanup
type SessionManager struct {
	handles   map[string]*HandleEntry
	mu        sync.RWMutex
	gcTicker  *time.Ticker
	gcStop    chan struct{}
	logger    *slog.Logger
	defaultTTL time.Duration
}

// HandleEntry stores handle metadata
type HandleEntry struct {
	Handle    Handle
	CreatedAt time.Time
	ExpiresAt time.Time
	LastUsed  time.Time
	RefCount  atomic.Int32
	Owner     string  // Principal who created it
	Context   Context // Context with permissions
}

// NewSessionManager creates a new session manager
func NewSessionManager(logger *slog.Logger, gcInterval, defaultTTL time.Duration) *SessionManager {
	sm := &SessionManager{
		handles:    make(map[string]*HandleEntry),
		gcTicker:   time.NewTicker(gcInterval),
		gcStop:     make(chan struct{}),
		logger:     logger,
		defaultTTL: defaultTTL,
	}

	// Start garbage collection loop
	go sm.gcLoop()

	return sm
}

// Store registers a new handle
func (sm *SessionManager) Store(handle Handle, owner string, ctx Context, ttl time.Duration) string {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Use default TTL if not specified
	if ttl == 0 {
		ttl = sm.defaultTTL
	}

	// Generate handle ID
	id := sm.generateHandleID()

	// Create entry
	entry := &HandleEntry{
		Handle:    handle,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(ttl),
		LastUsed:  time.Now(),
		Owner:     owner,
		Context:   ctx,
	}
	entry.RefCount.Store(1)

	sm.handles[id] = entry

	sm.logger.Info("handle created",
		"id", id,
		"owner", owner,
		"ttl", ttl,
	)

	return id
}

// Get retrieves a handle and increments reference count
func (sm *SessionManager) Get(id string, principal string) (Handle, Context, error) {
	sm.mu.RLock()
	entry, ok := sm.handles[id]
	sm.mu.RUnlock()

	if !ok {
		return nil, nil, ErrHandleNotFound
	}

	// Permission check: only owner can use handle
	if entry.Owner != principal {
		sm.logger.Warn("handle access denied",
			"id", id,
			"owner", entry.Owner,
			"principal", principal,
		)
		return nil, nil, ErrPermissionDenied
	}

	// Check expiration
	if time.Now().After(entry.ExpiresAt) {
		sm.Delete(id)
		return nil, nil, ErrHandleExpired
	}

	// Update last used timestamp and increment ref count
	sm.mu.Lock()
	entry.LastUsed = time.Now()
	entry.RefCount.Add(1)
	sm.mu.Unlock()

	return entry.Handle, entry.Context, nil
}

// Release decrements reference count
func (sm *SessionManager) Release(id string) error {
	sm.mu.RLock()
	entry, ok := sm.handles[id]
	sm.mu.RUnlock()

	if !ok {
		return ErrHandleNotFound
	}

	// Decrement ref count
	newCount := entry.RefCount.Add(-1)

	// If ref count reaches 0, schedule for cleanup
	if newCount <= 0 {
		return sm.Delete(id)
	}

	return nil
}

// Delete removes a handle and closes it
func (sm *SessionManager) Delete(id string) error {
	sm.mu.Lock()
	entry, ok := sm.handles[id]
	if !ok {
		sm.mu.Unlock()
		return ErrHandleNotFound
	}
	delete(sm.handles, id)
	sm.mu.Unlock()

	// Close the handle (releases resources)
	if err := entry.Handle.Close(); err != nil {
		sm.logger.Error("failed to close handle",
			"id", id,
			"error", err,
		)
		return err
	}

	sm.logger.Info("handle deleted",
		"id", id,
		"owner", entry.Owner,
		"lifetime", time.Since(entry.CreatedAt),
		"total_uses", entry.RefCount.Load(),
	)

	return nil
}

// Extend extends the TTL of a handle
func (sm *SessionManager) Extend(id string, principal string, additionalTTL time.Duration) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	entry, ok := sm.handles[id]
	if !ok {
		return ErrHandleNotFound
	}

	// Permission check
	if entry.Owner != principal {
		return ErrPermissionDenied
	}

	// Extend expiration
	entry.ExpiresAt = entry.ExpiresAt.Add(additionalTTL)

	sm.logger.Info("handle TTL extended",
		"id", id,
		"new_expiration", entry.ExpiresAt,
	)

	return nil
}

// ============================================================================
// Garbage Collection
// ============================================================================

// gcLoop periodically cleans up expired handles
func (sm *SessionManager) gcLoop() {
	for {
		select {
		case <-sm.gcTicker.C:
			sm.gc()
		case <-sm.gcStop:
			return
		}
	}
}

// gc performs garbage collection
func (sm *SessionManager) gc() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	expired := []string{}

	for id, entry := range sm.handles {
		// Check expiration
		if now.After(entry.ExpiresAt) {
			expired = append(expired, id)
			continue
		}

		// Check idle timeout (30 minutes of inactivity)
		idleTimeout := 30 * time.Minute
		if now.Sub(entry.LastUsed) > idleTimeout {
			expired = append(expired, id)
			continue
		}
	}

	// Cleanup expired handles
	for _, id := range expired {
		entry := sm.handles[id]
		delete(sm.handles, id)

		sm.logger.Info("handle expired (GC)",
			"id", id,
			"owner", entry.Owner,
			"lifetime", time.Since(entry.CreatedAt),
		)

		// Close in background to avoid blocking GC
		go func(h Handle) {
			if err := h.Close(); err != nil {
				sm.logger.Error("failed to close expired handle",
					"error", err,
				)
			}
		}(entry.Handle)
	}

	if len(expired) > 0 {
		sm.logger.Info("garbage collection completed",
			"cleaned", len(expired),
			"remaining", len(sm.handles),
		)
	}
}

// ============================================================================
// Statistics & Inspection
// ============================================================================

// Stats returns session manager statistics
type SessionStats struct {
	TotalHandles   int
	HandlesByOwner map[string]int
	OldestHandle   time.Time
	NewestHandle   time.Time
}

// Stats returns current statistics
func (sm *SessionManager) Stats() SessionStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	stats := SessionStats{
		TotalHandles:   len(sm.handles),
		HandlesByOwner: make(map[string]int),
	}

	var oldest, newest time.Time
	for _, entry := range sm.handles {
		stats.HandlesByOwner[entry.Owner]++

		if oldest.IsZero() || entry.CreatedAt.Before(oldest) {
			oldest = entry.CreatedAt
		}
		if newest.IsZero() || entry.CreatedAt.After(newest) {
			newest = entry.CreatedAt
		}
	}

	stats.OldestHandle = oldest
	stats.NewestHandle = newest

	return stats
}

// ListHandles returns all handle IDs (for debugging)
func (sm *SessionManager) ListHandles(principal string) []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var handles []string
	for id, entry := range sm.handles {
		// Only list handles owned by the principal
		if entry.Owner == principal {
			handles = append(handles, id)
		}
	}

	return handles
}

// ============================================================================
// Shutdown
// ============================================================================

// Shutdown closes all handles and stops the GC loop
func (sm *SessionManager) Shutdown() error {
	// Stop GC loop
	close(sm.gcStop)
	sm.gcTicker.Stop()

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Close all handles
	for id, entry := range sm.handles {
		if err := entry.Handle.Close(); err != nil {
			sm.logger.Error("failed to close handle during shutdown",
				"id", id,
				"error", err,
			)
		} else {
			sm.logger.Info("handle closed (shutdown)", "id", id)
		}
	}

	// Clear map
	sm.handles = make(map[string]*HandleEntry)

	sm.logger.Info("session manager shutdown complete")
	return nil
}

// ============================================================================
// Internal Helpers
// ============================================================================

// generateHandleID creates a unique handle ID
func (sm *SessionManager) generateHandleID() string {
	return fmt.Sprintf("kernel://session/%s", uuid.New().String())
}
