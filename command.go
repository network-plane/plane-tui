package tui

import "context"

// Command is the primary interface implemented by concrete commands.
type Command interface {
	Spec() CommandSpec
	Execute(rt CommandRuntime, input CommandInput) CommandResult
}

// CommandFactory builds command instances with access to runtime services.
type CommandFactory interface {
	Spec() CommandSpec
	New(rt CommandRuntime) (Command, error)
}

// CommandSpec describes command metadata for discovery and help.
type CommandSpec struct {
	Name         string
	Aliases      []string
	Summary      string
	Description  string
	Examples     []Example
	Args         []ArgSpec
	Flags        []FlagSpec
	Permissions  []string
	Hidden       bool
	Tags         []string
	Category     string
	Context      string
	Usage        string
	AllowPipes   bool
	DefaultAlias string
}

// Example documents an example invocation of a command.
type Example struct {
	Description string
	Command     string
}

// ArgType enumerates supported argument data types.
type ArgType string

const (
	ArgTypeString   ArgType = "string"
	ArgTypeInt      ArgType = "int"
	ArgTypeFloat    ArgType = "float"
	ArgTypeBool     ArgType = "bool"
	ArgTypeDuration ArgType = "duration"
	ArgTypeEnum     ArgType = "enum"
	ArgTypeJSON     ArgType = "json"
)

// ArgSpec defines positional argument metadata.
type ArgSpec struct {
	Name        string
	Type        ArgType
	Required    bool
	Repeatable  bool
	Description string
	Default     any
	EnumValues  []string
}

// FlagSpec defines flag metadata.
type FlagSpec struct {
	Name        string
	Shorthand   string
	Type        ArgType
	Required    bool
	Description string
	Default     any
	EnumValues  []string
	Hidden      bool
}

// CommandStatus indicates the result of a command invocation.
type CommandStatus string

const (
	StatusSuccess CommandStatus = "success"
	StatusFailed  CommandStatus = "failed"
	StatusPartial CommandStatus = "partial"
	StatusPending CommandStatus = "pending"
)

// SeverityLevel indicates the severity of an error surfaced to the user.
type SeverityLevel string

const (
	SeverityInfo    SeverityLevel = "info"
	SeverityWarning SeverityLevel = "warning"
	SeverityError   SeverityLevel = "error"
)

// CommandError wraps an error with user facing metadata.
type CommandError struct {
	Err         error
	Message     string
	Severity    SeverityLevel
	Hints       []string
	Recoverable bool
}

// CommandResult conveys the outcome of command execution.
type CommandResult struct {
	Status      CommandStatus
	Error       *CommandError
	Payload     any
	Messages    []OutputMessage
	NextContext string
	Pipeline    any
}

// OutputMessage allows commands to suggest standardised output.
type OutputMessage struct {
	Level   SeverityLevel
	Format  string
	Content string
}

// CommandInput contains parsed arguments, flags, and runtime context.
type CommandInput struct {
	Context  context.Context
	Raw      []string
	Args     ValueSet
	Flags    ValueSet
	Pipeline any
}

// CommandRuntime presents runtime services to commands.
type CommandRuntime interface {
	Session() SessionStore
	Services() ServiceRegistry
	Output() OutputChannel
	ContextManager() *ContextManager
	TaskManager() *TaskManager
	Cancellation() context.Context
	NavigateTo(name string, payload any) error
	PushContext(name string, payload any) error
	PopContext() error
	PipelineData() any
	SetPipelineData(v any)
}
