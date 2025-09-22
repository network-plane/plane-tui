package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
)

// Middleware wraps command execution with cross-cutting logic.
type Middleware func(CommandRuntime, CommandInput, CommandEntry, NextFunc) CommandResult

// NextFunc represents the next handler in the middleware chain.
type NextFunc func(CommandRuntime, CommandInput) CommandResult

// Engine orchestrates command resolution and execution.
type Engine struct {
	registry     *CommandRegistry
	contexts     *ContextManager
	session      SessionStore
	services     ServiceRegistry
	parser       *ArgsParser
	middleware   []Middleware
	outputWriter io.Writer
	outputLevel  OutputLevel
	helpHeader   string
	promptBase   string
	tasks        *TaskManager
	mu           sync.RWMutex
}

// Option configures the engine.
type Option func(*Engine)

// WithMiddleware appends middleware functions.
func WithMiddleware(mw ...Middleware) Option {
	return func(e *Engine) {
		e.middleware = append(e.middleware, mw...)
	}
}

// WithServices seeds the service registry.
func WithServices(register func(ServiceRegistry)) Option {
	return func(e *Engine) {
		if register != nil {
			register(e.services)
		}
	}
}

// WithPrompt sets the base prompt prefix.
func WithPrompt(prompt string) Option {
	return func(e *Engine) { e.promptBase = prompt }
}

// WithHelpHeader customises the help header string.
func WithHelpHeader(header string) Option {
	return func(e *Engine) { e.helpHeader = header }
}

// WithOutputLevel sets default output verbosity.
func WithOutputLevel(level OutputLevel) Option {
	return func(e *Engine) { e.outputLevel = level }
}

// WithOutputWriter overrides the engine output writer.
func WithOutputWriter(w io.Writer) Option {
	return func(e *Engine) {
		if w != nil {
			e.outputWriter = w
		}
	}
}

// NewEngine constructs an Engine with defaults.
func NewEngine(options ...Option) *Engine {
	registry := NewCommandRegistry()
	contexts := NewContextManager(registry)
	session := NewSessionStore()
	services := NewServiceRegistry()
	engine := &Engine{
		registry:     registry,
		contexts:     contexts,
		session:      session,
		services:     services,
		parser:       NewArgsParser(),
		outputWriter: os.Stdout,
		outputLevel:  OutputNormal,
		helpHeader:   "Available commands:",
		promptBase:   "> ",
	}
	engine.middleware = []Middleware{RecoveryMiddleware}
	engine.registerBuiltins()
	for _, opt := range options {
		opt(engine)
	}
	engine.tasks = NewTaskManager(NewOutputChannel(engine.outputWriter))
	return engine
}

// Registry exposes the command registry for external registration.
func (e *Engine) Registry() *CommandRegistry { return e.registry }

// Contexts returns the context manager.
func (e *Engine) Contexts() *ContextManager { return e.contexts }

// Session exposes the session store.
func (e *Engine) Session() SessionStore { return e.session }

// Services exposes the service registry.
func (e *Engine) Services() ServiceRegistry { return e.services }

// RegisterContext adds a context specification to the registry.
func (e *Engine) RegisterContext(spec ContextSpec) {
	e.registry.RegisterContext(spec)
}

// RegisterCommand registers a command factory.
func (e *Engine) RegisterCommand(factory CommandFactory) {
	e.registry.RegisterCommand(factory)
}

// SetPrompt updates the base prompt string.
func (e *Engine) SetPrompt(prompt string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if prompt != "" {
		e.promptBase = prompt
	}
}

// SetHelpHeader updates the help header string.
func (e *Engine) SetHelpHeader(header string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if header != "" {
		e.helpHeader = header
	}
}

// SetOutputLevel updates output verbosity.
func (e *Engine) SetOutputLevel(level OutputLevel) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.outputLevel = level
}

// SetOutputWriter swaps the engine's writer for command output, returning the previous writer.
func (e *Engine) SetOutputWriter(w io.Writer) io.Writer {
	e.mu.Lock()
	defer e.mu.Unlock()
	prev := e.outputWriter
	if w == nil {
		e.outputWriter = os.Stdout
	} else {
		e.outputWriter = w
	}
	if e.tasks != nil {
		e.tasks.SetOutputChannel(NewOutputChannel(e.outputWriter))
	}
	return prev
}

// Run starts the interactive loop.
func (e *Engine) Run(rl *readline.Instance) error {
	if rl == nil {
		return errors.New("readline instance is required")
	}
	for {
		e.refreshAutocomplete(rl)
		prompt := e.contexts.Prompt(e.promptBase)
		rl.SetPrompt(prompt)
		line, err := rl.Readline()
		if err != nil {
			if errors.Is(err, readline.ErrInterrupt) {
				return nil
			}
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		tokens := tokenize(line)
		if len(tokens) == 0 {
			continue
		}
		if exitRequested(tokens[0]) {
			fmt.Fprintln(e.outputWriter, "Shutting down.")
			return nil
		}
		if err := rl.SaveHistory(line); err != nil {
			fmt.Fprintf(e.outputWriter, "Error saving history: %v\n", err)
		}
		if err := e.process(tokens); err != nil {
			fmt.Fprintf(e.outputWriter, "Error: %v\n", err)
		}
	}
}

func (e *Engine) refreshAutocomplete(rl *readline.Instance) {
	ctx := e.contexts.Current().Spec.Name
	if ctx == "" {
		var items []readline.PrefixCompleterInterface
		contexts := e.registry.Contexts(false)
		for _, ctxSpec := range contexts {
			commands := e.registry.Commands(ctxSpec.Name, false)
			var subitems []readline.PrefixCompleterInterface
			for _, cmd := range commands {
				subitems = append(subitems, readline.PcItem(cmd.Name))
			}
			items = append(items, readline.PcItem(ctxSpec.Name, subitems...))
		}
		rootCmds := e.registry.Commands("", false)
		for _, cmd := range rootCmds {
			items = append(items, readline.PcItem(cmd.Name))
		}
		rl.Config.AutoComplete = readline.NewPrefixCompleter(items...)
		return
	}
	commands := e.registry.Commands(ctx, false)
	var completions []string
	for _, cmd := range commands {
		completions = append(completions, cmd.Name)
	}
	rl.Config.AutoComplete = readline.NewPrefixCompleter(
		readline.PcItemDynamic(func(prefix string) []string { return completions }),
	)
}

func (e *Engine) process(tokens []string) error {
	ctx := e.contexts.Current().Spec.Name
	switch tokens[0] {
	case "help", "?", "h", "ls":
		e.renderHelp(ctx)
		return nil
	case "contexts":
		e.listContexts()
		return nil
	case "ctx":
		return e.handleCtxCommand(tokens[1:])
	case "back", "..":
		return e.contexts.Pop()
	case "/":
		return e.contexts.PopToRoot()
	case "history":
		e.showHistory()
		return nil
	}

	if ctx == "" {
		if spec, ok := e.registry.Context(tokens[0]); ok && spec.Name != "" {
			return e.contexts.Navigate(spec.Name, nil)
		}
	}

	entry, ok := e.registry.Resolve(ctx, tokens[0])
	if !ok {
		return fmt.Errorf("unknown command: %s", tokens[0])
	}

	return e.invoke(entry, tokens[1:])
}

func (e *Engine) invoke(entry CommandEntry, args []string) error {
	parsedArgs, parsedFlags, err := e.parser.Parse(args, entry.Spec)
	if err != nil {
		return err
	}

	current := e.contexts.Current()
	ctxObj, cancel := context.WithCancel(context.Background())
	execRT := &executionRuntime{
		engine:   e,
		ctx:      ctxObj,
		cancel:   cancel,
		output:   NewOutputChannel(e.outputWriter),
		pipeline: current.Payload,
	}
	defer cancel()
	execRT.output.SetLevel(e.outputLevel)

	input := CommandInput{
		Context:  ctxObj,
		Raw:      args,
		Args:     parsedArgs,
		Flags:    parsedFlags,
		Pipeline: current.Payload,
	}

	handler := e.coreHandler(entry)
	result := handler(execRT, input)
	if result.Status == "" {
		if result.Error != nil {
			result.Status = StatusFailed
		} else {
			result.Status = StatusSuccess
		}
	}

	AggregateMessages(execRT.output, result.Messages)

	if result.Error != nil {
		msg := result.Error.Message
		if msg == "" && result.Error.Err != nil {
			msg = result.Error.Err.Error()
		}
		execRT.output.Error(msg)
		for _, hint := range result.Error.Hints {
			execRT.output.Info(fmt.Sprintf("hint: %s", hint))
		}
	}

	if result.Status == StatusFailed {
		return nil
	}
	if result.NextContext != "" && execRT.nextContext == "" {
		execRT.nextContext = result.NextContext
		execRT.nextPayload = result.Pipeline
	}

	if entry.Spec.AllowPipes && result.Pipeline != nil {
		execRT.SetPipelineData(result.Pipeline)
	}

	if execRT.nextContext != "" {
		if err := e.contexts.Navigate(execRT.nextContext, execRT.nextPayload); err != nil {
			execRT.output.Error(err.Error())
		}
	}

	return nil
}

func (e *Engine) coreHandler(entry CommandEntry) func(CommandRuntime, CommandInput) CommandResult {
	h := func(rt CommandRuntime, input CommandInput) CommandResult {
		cmd, err := entry.Factory.New(rt)
		if err != nil {
			return CommandResult{Status: StatusFailed, Error: &CommandError{Err: err, Message: "failed to create command", Severity: SeverityError}}
		}
		if entry.Spec.Usage == "" {
			entry.Spec.Usage = FormatUsage(entry.Spec)
		}
		return cmd.Execute(rt, input)
	}

	for i := len(e.middleware) - 1; i >= 0; i-- {
		mw := e.middleware[i]
		next := h
		h = func(rt CommandRuntime, input CommandInput) CommandResult {
			return mw(rt, input, entry, next)
		}
	}

	return h
}

func (e *Engine) renderHelp(ctx string) {
	fmt.Fprintln(e.outputWriter, e.helpHeader)
	if ctx == "" {
		contexts := e.registry.Contexts(false)
		if len(contexts) > 0 {
			fmt.Fprintln(e.outputWriter, "Contexts:")
			sort.Slice(contexts, func(i, j int) bool { return contexts[i].Name < contexts[j].Name })
			for _, c := range contexts {
				fmt.Fprintf(e.outputWriter, "  %-15s %s\n", c.Name, c.Description)
			}
		}
		rootCmds := e.registry.Commands("", false)
		if len(rootCmds) > 0 {
			fmt.Fprintln(e.outputWriter, "\nGlobal Commands:")
			for _, cmd := range rootCmds {
				fmt.Fprintf(e.outputWriter, "  %-20s %s\n", cmd.Name, cmd.Summary)
			}
		}
		fmt.Fprintln(e.outputWriter, "\nType a context name to enter it or 'ctx goto <name>'.")
		return
	}

	cmds := e.registry.Commands(ctx, false)
	if len(cmds) == 0 {
		fmt.Fprintf(e.outputWriter, "No commands registered for context %s\n", ctx)
		return
	}
	fmt.Fprintf(e.outputWriter, "Commands in %s:\n", ctx)
	for _, cmd := range cmds {
		fmt.Fprintf(e.outputWriter, "  %-20s %s\n", cmd.Name, cmd.Summary)
	}
}

func (e *Engine) listContexts() {
	contexts := e.registry.Contexts(false)
	if len(contexts) == 0 {
		fmt.Fprintln(e.outputWriter, "No contexts registered.")
		return
	}
	fmt.Fprintln(e.outputWriter, "Contexts:")
	for _, ctx := range contexts {
		fmt.Fprintf(e.outputWriter, "  %-15s %s\n", ctx.Name, ctx.Description)
	}
}

func (e *Engine) handleCtxCommand(args []string) error {
	if len(args) == 0 {
		return errors.New("ctx command requires arguments")
	}
	switch args[0] {
	case "goto":
		if len(args) < 2 {
			return errors.New("ctx goto <name>")
		}
		return e.contexts.Navigate(args[1], nil)
	case "push":
		if len(args) < 2 {
			return errors.New("ctx push <name>")
		}
		return e.contexts.Push(args[1], nil)
	case "pop":
		return e.contexts.Pop()
	default:
		return fmt.Errorf("unknown ctx action: %s", args[0])
	}
}

func (e *Engine) showHistory() {
	fmt.Fprintln(e.outputWriter, "Readline history is managed by the readline library. Advanced history tracking TBD.")
}

func exitRequested(token string) bool {
	switch token {
	case "exit", "quit", "q":
		return true
	default:
		return false
	}
}

func tokenize(input string) []string {
	fields := strings.Fields(input)
	return fields
}

// executionRuntime implements CommandRuntime.
type executionRuntime struct {
	engine      *Engine
	ctx         context.Context
	cancel      context.CancelFunc
	output      OutputChannel
	pipeline    any
	nextContext string
	nextPayload any
}

func (r *executionRuntime) Session() SessionStore { return r.engine.session }

func (r *executionRuntime) Services() ServiceRegistry { return r.engine.services }

func (r *executionRuntime) Output() OutputChannel { return r.output }

func (r *executionRuntime) ContextManager() *ContextManager { return r.engine.contexts }

func (r *executionRuntime) TaskManager() *TaskManager { return r.engine.tasks }

func (r *executionRuntime) Cancellation() context.Context { return r.ctx }

func (r *executionRuntime) NavigateTo(name string, payload any) error {
	r.nextContext = name
	r.nextPayload = payload
	return nil
}

func (r *executionRuntime) PushContext(name string, payload any) error {
	return r.engine.contexts.Push(name, payload)
}

func (r *executionRuntime) PopContext() error {
	return r.engine.contexts.Pop()
}

func (r *executionRuntime) PipelineData() any { return r.pipeline }

func (r *executionRuntime) SetPipelineData(v any) { r.pipeline = v }

func (r *executionRuntime) Close() { r.cancel() }

func (e *Engine) registerBuiltins() {
	e.registry.RegisterCommand(&helpCommandFactory{engine: e})
	e.registry.RegisterCommand(&tasksCommandFactory{engine: e})
}

// help command implementation -------------------------------------------------

type helpCommandFactory struct {
	engine *Engine
	spec   CommandSpec
}

func (f *helpCommandFactory) Spec() CommandSpec {
	if f.spec.Name == "" {
		f.spec = CommandSpec{
			Name:    "help",
			Aliases: []string{"?", "h"},
			Summary: "Show help for commands and contexts",
			Context: "",
		}
	}
	return f.spec
}

func (f *helpCommandFactory) New(rt CommandRuntime) (Command, error) {
	return &helpCommand{engine: f.engine, spec: f.Spec()}, nil
}

type helpCommand struct {
	engine *Engine
	spec   CommandSpec
}

func (c *helpCommand) Spec() CommandSpec { return c.spec }

func (c *helpCommand) Execute(rt CommandRuntime, input CommandInput) CommandResult {
	ctx := rt.ContextManager().Current().Spec.Name
	c.engine.renderHelp(ctx)
	return CommandResult{Status: StatusSuccess}
}

// tasks command ---------------------------------------------------------------

type tasksCommandFactory struct {
	engine *Engine
	spec   CommandSpec
}

func (f *tasksCommandFactory) Spec() CommandSpec {
	if f.spec.Name == "" {
		f.spec = CommandSpec{
			Name:    "tasks",
			Summary: "List background tasks",
			Context: "",
		}
	}
	return f.spec
}

func (f *tasksCommandFactory) New(rt CommandRuntime) (Command, error) {
	return &tasksCommand{engine: f.engine, spec: f.Spec()}, nil
}

type tasksCommand struct {
	engine *Engine
	spec   CommandSpec
}

func (c *tasksCommand) Spec() CommandSpec { return c.spec }

func (c *tasksCommand) Execute(rt CommandRuntime, input CommandInput) CommandResult {
	tasks := rt.TaskManager().Tasks()
	rows := make([][]string, 0, len(tasks))
	for _, task := range tasks {
		err := ""
		if task.Error != nil {
			err = task.Error.Error()
		}
		rows = append(rows, []string{task.ID, task.Name, string(task.Status), err})
	}
	rt.Output().WriteTable([]string{"ID", "Name", "Status", "Error"}, rows)
	return CommandResult{Status: StatusSuccess, Payload: tasks}
}

// Default middleware ---------------------------------------------------------

// RecoveryMiddleware recovers from panics in commands.
func RecoveryMiddleware(rt CommandRuntime, input CommandInput, entry CommandEntry, next NextFunc) CommandResult {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("command %s panicked: %v", entry.Spec.Name, r)
			rt.Output().Error(msg)
		}
	}()
	return next(rt, input)
}

// TimingMiddleware measures execution duration.
func TimingMiddleware(rt CommandRuntime, input CommandInput, entry CommandEntry, next NextFunc) CommandResult {
	start := time.Now()
	result := next(rt, input)
	dur := time.Since(start)
	rt.Output().Info(fmt.Sprintf("%s finished in %s", entry.Spec.Name, dur.Truncate(time.Millisecond)))
	return result
}
