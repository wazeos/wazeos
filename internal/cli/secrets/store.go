package secrets

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FileStore implements a simple file-based secrets store for CLI usage
type FileStore struct {
	filePath string
	mu       sync.RWMutex
	secrets  map[string]map[string]interface{}
}

// NewFileStore creates a new file-based secrets store
func NewFileStore(dataPath string) (*FileStore, error) {
	secretsFile := filepath.Join(dataPath, "secrets.json")

	store := &FileStore{
		filePath: secretsFile,
		secrets:  make(map[string]map[string]interface{}),
	}

	// Load existing secrets if file exists
	if _, err := os.Stat(secretsFile); err == nil {
		if err := store.load(); err != nil {
			return nil, fmt.Errorf("failed to load secrets: %w", err)
		}
	}

	return store, nil
}

// load reads secrets from the JSON file
func (s *FileStore) load() error {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	if len(data) == 0 {
		return nil
	}

	return json.Unmarshal(data, &s.secrets)
}

// save writes secrets to the JSON file
func (s *FileStore) save() error {
	data, err := json.MarshalIndent(s.secrets, "", "  ")
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0700); err != nil {
		return err
	}

	// Write with restrictive permissions (secrets file should be private)
	return os.WriteFile(s.filePath, data, 0600)
}

// Set stores a secret value
func (s *FileStore) Set(ctx context.Context, key string, value interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store as a map with "value" field to match server behavior
	s.secrets[key] = map[string]interface{}{
		"value": value,
	}

	return s.save()
}

// Get retrieves a secret value
func (s *FileStore) Get(ctx context.Context, key string) (interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	secret, ok := s.secrets[key]
	if !ok {
		return nil, fmt.Errorf("secret not found: %s", key)
	}

	return secret["value"], nil
}

// List returns all secret keys
func (s *FileStore) List(ctx context.Context) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.secrets))
	for k := range s.secrets {
		keys = append(keys, k)
	}

	return keys, nil
}

// Delete removes a secret
func (s *FileStore) Delete(ctx context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.secrets[key]; !ok {
		return fmt.Errorf("secret not found: %s", key)
	}

	delete(s.secrets, key)
	return s.save()
}

// Match returns secrets matching a prefix
func (s *FileStore) Match(ctx context.Context, prefix string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matches := make(map[string]interface{})
	for k, v := range s.secrets {
		if prefix == "" || strings.HasPrefix(k, prefix) {
			matches[k] = v["value"]
		}
	}

	return matches, nil
}
