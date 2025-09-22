# PlanetUI

PlanetUI is a composable terminal user interface (TUI) framework for Go. It extends the original minimal command loop with structured command metadata, lifecycle hooks, dependency injection, context navigation, typed argument parsing, output management, and background task execution—while preserving compatibility with legacy commands.

## Features

- **Structured commands** with `CommandSpec`, rich metadata, aliases, tags, and auto-generated usage strings
- **Lifecycle + middleware** support so authentication, logging, and validation run consistently before/after commands
- **Built-in argument parsing** using declarative `ArgSpec`/`FlagSpec` with typed accessors
- **Context manager** providing hierarchical navigation, alias resolution, and payload/state passing
- **Session + services** stores for sharing data and dependencies across commands
- **Async/background tasks** with cancellation, progress output, and task inspection
- **Output channels** enabling leveled messaging, JSON/table rendering, and test-friendly capture
- **Legacy compatibility** through `RegisterLegacyCommand` adapters so existing commands keep working

## Installation

```bash
go get github.com/network-plane/planetui
```

## Quick Start

```go
package main

import (
    "log"

    "github.com/chzyer/readline"
    tui "github.com/network-plane/planetui"
)

type helloFactory struct {
    spec tui.CommandSpec
}

type helloCommand struct {
    spec tui.CommandSpec
}

func newHelloFactory() tui.CommandFactory {
    spec := tui.CommandSpec{
        Name:        "hello",
        Summary:     "Print a greeting",
        Description: "Outputs a greeting and optional name.",
        Context:     "demo",
        Args: []tui.ArgSpec{
            {Name: "name", Type: tui.ArgTypeString, Required: false, Description: "Name to greet"},
        },
    }
    return &helloFactory{spec: spec}
}

func (f *helloFactory) Spec() tui.CommandSpec { return f.spec }

func (f *helloFactory) New(rt tui.CommandRuntime) (tui.Command, error) {
    return &helloCommand{spec: f.spec}, nil
}

func (c *helloCommand) Spec() tui.CommandSpec { return c.spec }

func (c *helloCommand) Execute(rt tui.CommandRuntime, input tui.CommandInput) tui.CommandResult {
    name := input.Args.String("name")
    if name == "" {
        name = "world"
    }
    rt.Output().Info("hello " + name + "!")
    return tui.CommandResult{Status: tui.StatusSuccess}
}

func main() {
    rl, err := readline.NewEx(&readline.Config{})
    if err != nil {
        log.Fatal(err)
    }
    defer rl.Close()

    tui.RegisterContext("demo", "Example commands")
    tui.RegisterCommand(newHelloFactory())

    if err := tui.Run(rl); err != nil {
        log.Fatal(err)
    }
}
```

## Working With Commands

- Describe metadata in `CommandSpec`; PlanetUI uses it for help text, autocomplete, and validation.
- Use `CommandInput.Args/Flags` typed helpers (`String`, `Int`, `Bool`, `Duration`, `DecodeJSON`, etc.).
- Return a `CommandResult` to signal success, surface structured errors, pass pipeline payloads, or request context navigation.
- Access shared session data via `CommandRuntime.Session()`, services via `Services()`, and spawn background work with `TaskManager().Spawn`.
- Emit output through `CommandRuntime.Output()`; messages are automatically captured for tests and respect verbosity levels.
- Register middleware with `tui.UseMiddleware` or when constructing a custom `Engine` to add logging, auth, timing, etc.

## Migration from the Original Minimal TUI

The original `planetui` package exposed a very small surface area:

```go
type Command interface {
    Name() string
    Help() string
    Exec(args []string)
}

func RegisterContext(name, description string)
func RegisterCommand(ctx string, cmd Command)
func Run(rl *readline.Instance)
```

Moving to the new framework provides far richer behaviour. The steps below help migrate existing apps incrementally:

1. **Wrap legacy commands (optional bridge).** Call `tui.RegisterLegacyCommand(ctx, legacyCmd)` to keep using the old `Command` interface while you migrate. Legacy commands run exactly as before, but without access to new features.
2. **Adopt factories.** Replace direct command instances with a `CommandFactory` that returns a fresh `Command` per execution. This unlocks dependency injection and isolates per-run state.
3. **Describe metadata.** Implement `Spec() CommandSpec` on your command (and factory) to declare name, aliases, contexts, arguments, and flags. PlanetUI now drives help/autocomplete from the spec.
4. **Return results instead of printing.** Change `Exec` implementations to `Execute(rt, input) CommandResult`. Use `CommandResult.Status`, `Error`, `Messages`, and `Payload` to communicate outcomes instead of calling `fmt.Print` directly.
5. **Use typed inputs.** Replace manual `[]string` parsing with `input.Args`/`input.Flags` based on the specs declared in step 3.
6. **Adopt runtime services.** Access session storage, shared dependencies, output channels, context navigation, and task management through the provided `CommandRuntime` methods rather than global variables.
7. **Clean up legacy helpers.** Once all commands implement the new interface, remove `RegisterLegacyCommand` calls and rely exclusively on `RegisterCommand` with factories.

### Key API Changes

| Legacy API                                 | New API                                                                |
| ------------------------------------------ | ---------------------------------------------------------------------- |
| `Command.Name/Help/Exec([]string)`         | `Command.Spec() CommandSpec` + `Execute(CommandRuntime, CommandInput)` |
| `RegisterCommand(ctx string, cmd Command)` | `RegisterCommand(factory CommandFactory)`                              |
| `fmt.Print` inside commands                | `rt.Output().Info/Warn/Error/WriteJSON/WriteTable`                     |
| `currentCtx` global / manual navigation    | `rt.NavigateTo`, `rt.PushContext`, `rt.PopContext`                     |
| No argument parsing helpers                | Declarative `ArgSpec`/`FlagSpec` + typed `ValueSet` access             |
| No async support                           | `rt.TaskManager().Spawn` with cancellation + inspection                |

Following these steps lets you layer the advanced command system on top of existing functionality without a flag-day rewrite.

## Contributing

Contributions are always welcome!
All contributions are required to follow the [Google Go Style Guide](https://google.github.io/styleguide/go/).

## Authors

- [@earentir](https://www.github.com/earentir)

## License

I will always follow the Linux Kernel License as primary, if you require any other OPEN license please let me know and I will try to accomodate it.

[![License](https://img.shields.io/github/license/earentir/gitearelease)](https://opensource.org/license/gpl-2-0)
