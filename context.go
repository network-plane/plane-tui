package tui

import (
	"fmt"
	"strings"
	"sync"
)

// ContextSpec defines metadata for an execution context.
type ContextSpec struct {
	Name        string
	Parent      string
	Description string
	Prompt      string
	Aliases     []string
	Tags        []string
	Hidden      bool
}

// ExecutionContext is an active context on the stack.
type ExecutionContext struct {
	Spec    ContextSpec
	State   map[string]any
	Payload any
}

// ContextManager manages context stack and transitions.
type ContextManager struct {
	mu       sync.RWMutex
	stack    []ExecutionContext
	registry *CommandRegistry
}

// NewContextManager constructs a manager.
func NewContextManager(registry *CommandRegistry) *ContextManager {
	root := ExecutionContext{Spec: ContextSpec{Name: "", Prompt: "> "}, State: map[string]any{}}
	return &ContextManager{stack: []ExecutionContext{root}, registry: registry}
}

// Current returns the active context on the stack.
func (m *ContextManager) Current() ExecutionContext {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stack[len(m.stack)-1]
}

// Stack returns a copy of the current stack.
func (m *ContextManager) Stack() []ExecutionContext {
	m.mu.RLock()
	defer m.mu.RUnlock()
	clone := make([]ExecutionContext, len(m.stack))
	copy(clone, m.stack)
	return clone
}

// Navigate sets the stack to the specified context, replacing current.
func (m *ContextManager) Navigate(name string, payload any) error {
	if name == "" {
		return m.PopToRoot()
	}
	spec, ok := m.registry.Context(name)
	if !ok {
		return fmt.Errorf("unknown context: %s", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stack = append(m.stack[:1], ExecutionContext{Spec: spec, State: map[string]any{}, Payload: payload})
	return nil
}

// Push adds a context to the stack.
func (m *ContextManager) Push(name string, payload any) error {
	spec, ok := m.registry.Context(name)
	if !ok {
		return fmt.Errorf("unknown context: %s", name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stack = append(m.stack, ExecutionContext{Spec: spec, State: map[string]any{}, Payload: payload})
	return nil
}

// Pop removes the top context if not root.
func (m *ContextManager) Pop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.stack) <= 1 {
		return fmt.Errorf("already at root context")
	}
	m.stack = m.stack[:len(m.stack)-1]
	return nil
}

// PopToRoot resets stack to root context.
func (m *ContextManager) PopToRoot() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stack = m.stack[:1]
	return nil
}

// ResolveAliases returns canonical context name.
func (m *ContextManager) ResolveAliases(name string) (string, bool) {
	if name == "" {
		return "", true
	}
	if strings.Contains(name, "::") {
		name = strings.ReplaceAll(name, "::", ".")
	}
	spec, ok := m.registry.Context(name)
	if ok {
		return spec.Name, true
	}
	return "", false
}

// Prompt returns the prompt for current context.
func (m *ContextManager) Prompt(base string) string {
	ctx := m.Current()
	prompt := ctx.Spec.Prompt
	if prompt == "" {
		prompt = fmt.Sprintf("%s%s> ", base, ctx.Spec.Name)
	} else {
		prompt = strings.ReplaceAll(prompt, "{base}", base)
		prompt = strings.ReplaceAll(prompt, "{context}", ctx.Spec.Name)
	}
	if ctx.Spec.Name == "" {
		return base
	}
	return prompt
}
