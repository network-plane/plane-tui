package tui

import (
	"io"

	"github.com/chzyer/readline"
)

var defaultEngine = NewEngine()

// DefaultEngine returns the shared default engine instance.
func DefaultEngine() *Engine { return defaultEngine }

// ResetEngine replaces the default engine (primarily for tests).
func ResetEngine(options ...Option) {
	defaultEngine = NewEngine(options...)
}

// SetPrompt customises the base prompt for the default engine.
func SetPrompt(prompt string) { defaultEngine.SetPrompt(prompt) }

// SetHelpHeader customises the help header for the default engine.
func SetHelpHeader(header string) { defaultEngine.SetHelpHeader(header) }

// SetOutputLevel adjusts default verbosity for the default engine.
func SetOutputLevel(level OutputLevel) { defaultEngine.SetOutputLevel(level) }

// RegisterContext registers a context with optional modifiers.
func RegisterContext(name, description string, opts ...ContextOption) {
	spec := ContextSpec{Name: name, Description: description}
	for _, opt := range opts {
		opt(&spec)
	}
	defaultEngine.RegisterContext(spec)
}

// ContextOption mutates a ContextSpec before registration.
type ContextOption func(*ContextSpec)

// WithContextPrompt sets a custom prompt template for the context.
func WithContextPrompt(prompt string) ContextOption {
	return func(spec *ContextSpec) { spec.Prompt = prompt }
}

// WithContextAliases adds aliases for a context name.
func WithContextAliases(aliases ...string) ContextOption {
	return func(spec *ContextSpec) { spec.Aliases = append(spec.Aliases, aliases...) }
}

// WithContextTags assigns tags to a context.
func WithContextTags(tags ...string) ContextOption {
	return func(spec *ContextSpec) { spec.Tags = append(spec.Tags, tags...) }
}

// RegisterCommand registers a command factory with the default engine.
func RegisterCommand(factory CommandFactory) {
	defaultEngine.RegisterCommand(factory)
}

// RegisterLegacyCommand adapts a legacy command into the new runtime.
func RegisterLegacyCommand(ctx string, cmd LegacyCommand) {
	defaultEngine.RegisterCommand(NewLegacyAdapter(cmd, ctx))
}

// UseMiddleware appends middleware to the default engine.
func UseMiddleware(mw ...Middleware) {
	WithMiddleware(mw...)(defaultEngine)
}

// SetOutputWriter sets the writer used for command output, returning the previous writer.
func SetOutputWriter(w io.Writer) io.Writer {
	return defaultEngine.SetOutputWriter(w)
}

// Run starts the main loop using the default engine.
func Run(rl *readline.Instance) error {
	return defaultEngine.Run(rl)
}
