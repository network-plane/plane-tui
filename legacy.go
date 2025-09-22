package tui

// LegacyCommand describes the original interface from the minimal framework.
type LegacyCommand interface {
	Name() string
	Help() string
	Exec(args []string)
}

// LegacyAdapter wraps a LegacyCommand into the new Command interface.
type LegacyAdapter struct {
	legacy  LegacyCommand
	context string
}

// NewLegacyAdapter creates a CommandFactory from a legacy command.
func NewLegacyAdapter(cmd LegacyCommand, ctx string) CommandFactory {
	return &legacyFactory{adapter: &LegacyAdapter{legacy: cmd, context: ctx}}
}

type legacyFactory struct {
	adapter *LegacyAdapter
}

func (f *legacyFactory) Spec() CommandSpec {
	return CommandSpec{
		Name:    f.adapter.legacy.Name(),
		Context: f.adapter.context,
		Summary: f.adapter.legacy.Help(),
	}
}

func (f *legacyFactory) New(rt CommandRuntime) (Command, error) {
	return f.adapter, nil
}

// Spec returns metadata for the wrapped command.
func (a *LegacyAdapter) Spec() CommandSpec {
	return CommandSpec{Name: a.legacy.Name(), Summary: a.legacy.Help(), Context: a.context}
}

// Execute delegates to the legacy command, capturing output.
func (a *LegacyAdapter) Execute(rt CommandRuntime, input CommandInput) CommandResult {
	a.legacy.Exec(input.Raw)
	return CommandResult{Status: StatusSuccess}
}
