package tui

import "sync"

// SessionStore provides shared state across commands during a session.
type SessionStore interface {
	Get(key string) (any, bool)
	Set(key string, value any)
	Delete(key string)
	Keys() []string
}

// MemorySessionStore is an in-memory implementation of SessionStore.
type MemorySessionStore struct {
	mu   sync.RWMutex
	data map[string]any
}

// NewSessionStore constructs a MemorySessionStore.
func NewSessionStore() *MemorySessionStore {
	return &MemorySessionStore{data: map[string]any{}}
}

// Get retrieves a value.
func (s *MemorySessionStore) Get(key string) (any, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, ok := s.data[key]
	return val, ok
}

// Set stores a key/value pair.
func (s *MemorySessionStore) Set(key string, value any) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
}

// Delete removes a key.
func (s *MemorySessionStore) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
}

// Keys lists stored keys.
func (s *MemorySessionStore) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.data))
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// ServiceRegistry exposes shared dependencies to commands.
type ServiceRegistry interface {
	Register(name string, value any)
	Get(name string) (any, bool)
}

// SimpleServiceRegistry is a basic map-backed ServiceRegistry.
type SimpleServiceRegistry struct {
	mu   sync.RWMutex
	data map[string]any
}

// NewServiceRegistry constructs a SimpleServiceRegistry.
func NewServiceRegistry() *SimpleServiceRegistry {
	return &SimpleServiceRegistry{data: map[string]any{}}
}

// Register stores a service instance.
func (r *SimpleServiceRegistry) Register(name string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[name] = value
}

// Get retrieves a service.
func (r *SimpleServiceRegistry) Get(name string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	val, ok := r.data[name]
	return val, ok
}
