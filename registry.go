package tui

import (
	"fmt"
	"path/filepath"
	"plugin"
	"sort"
	"strings"
	"sync"
)

// CommandEntry stores command factory and resolved names.
type CommandEntry struct {
	Factory CommandFactory
	Spec    CommandSpec
}

// CommandRegistry manages contexts and command registrations.
type CommandRegistry struct {
	mu       sync.RWMutex
	contexts map[string]ContextSpec
	aliases  map[string]string
	commands map[string]map[string]CommandEntry // context -> name -> entry
}

// NewCommandRegistry constructs a registry.
func NewCommandRegistry() *CommandRegistry {
	return &CommandRegistry{
		contexts: map[string]ContextSpec{"": {Name: "", Prompt: "> "}},
		aliases:  map[string]string{},
		commands: map[string]map[string]CommandEntry{},
	}
}

// RegisterContext registers a new context spec.
func (r *CommandRegistry) RegisterContext(spec ContextSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.contexts[spec.Name] = spec
	for _, alias := range spec.Aliases {
		r.aliases[alias] = spec.Name
	}
}

// Context retrieves a context specification.
func (r *CommandRegistry) Context(name string) (ContextSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if canonical, ok := r.aliases[name]; ok {
		name = canonical
	}
	spec, ok := r.contexts[name]
	return spec, ok
}

// RegisterCommand registers a command factory.
func (r *CommandRegistry) RegisterCommand(factory CommandFactory) {
	spec := factory.Spec()
	if spec.Name == "" {
		panic("command spec must define name")
	}
	ctx := spec.Context

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.commands[ctx]; !ok {
		r.commands[ctx] = map[string]CommandEntry{}
	}
	entry := CommandEntry{Factory: factory, Spec: spec}
	r.commands[ctx][spec.Name] = entry
	for _, alias := range spec.Aliases {
		r.commands[ctx][alias] = entry
	}
}

// UnregisterCommand removes a command by name.
func (r *CommandRegistry) UnregisterCommand(ctx, name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if commands, ok := r.commands[ctx]; ok {
		delete(commands, name)
	}
}

// Resolve finds a command entry for a context.
func (r *CommandRegistry) Resolve(ctx, name string) (CommandEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if commands, ok := r.commands[ctx]; ok {
		entry, ok := commands[name]
		return entry, ok
	}
	return CommandEntry{}, false
}

// Commands returns command names for a context.
func (r *CommandRegistry) Commands(ctx string, includeHidden bool) []CommandSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	entries := r.commands[ctx]
	seen := map[string]bool{}
	var specs []CommandSpec
	for _, entry := range entries {
		if seen[entry.Spec.Name] {
			continue
		}
		seen[entry.Spec.Name] = true
		if entry.Spec.Hidden && !includeHidden {
			continue
		}
		specs = append(specs, entry.Spec)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs
}

// Contexts lists registered contexts.
func (r *CommandRegistry) Contexts(includeHidden bool) []ContextSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var specs []ContextSpec
	for _, spec := range r.contexts {
		if spec.Name == "" {
			continue
		}
		if spec.Hidden && !includeHidden {
			continue
		}
		specs = append(specs, spec)
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs
}

// LoadPlugins loads Go plugins from directory.
func (r *CommandRegistry) LoadPlugins(dir string) error {
	matches, err := filepath.Glob(filepath.Join(dir, "*.so"))
	if err != nil {
		return err
	}
	for _, path := range matches {
		mod, err := plugin.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open plugin %s: %w", path, err)
		}
		sym, err := mod.Lookup("Register")
		if err != nil {
			return fmt.Errorf("plugin %s missing Register symbol", path)
		}
		fn, ok := sym.(func(CommandRegistryWriter) error)
		if !ok {
			return fmt.Errorf("plugin %s has invalid Register signature", path)
		}
		if err := fn(r); err != nil {
			return fmt.Errorf("plugin %s registration failed: %w", path, err)
		}
	}
	return nil
}

// CommandRegistryWriter exposes safe registration subset for plugins.
type CommandRegistryWriter interface {
	RegisterContext(spec ContextSpec)
	RegisterCommand(factory CommandFactory)
}

// Ensure CommandRegistry satisfies writer interface.
var _ CommandRegistryWriter = (*CommandRegistry)(nil)

// ResolveContextName returns canonical context name, considering aliases.
func (r *CommandRegistry) ResolveContextName(name string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if name == "" {
		return "", true
	}
	if canonical, ok := r.aliases[name]; ok {
		return canonical, true
	}
	_, ok := r.contexts[name]
	return name, ok
}

// NamespaceCommands returns commands across contexts matching prefix.
func (r *CommandRegistry) NamespaceCommands(namespace string) []CommandSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var specs []CommandSpec
	for ctx, commands := range r.commands {
		if namespace != "" && !strings.HasPrefix(ctx, namespace) {
			continue
		}
		seen := map[string]bool{}
		for _, entry := range commands {
			if seen[entry.Spec.Name] {
				continue
			}
			seen[entry.Spec.Name] = true
			specs = append(specs, entry.Spec)
		}
	}
	sort.Slice(specs, func(i, j int) bool { return specs[i].Name < specs[j].Name })
	return specs
}
