# plane-tui v1

This branch contains the original, minimal terminal user interface loop used by Plane. It provides a lightweight command dispatch system with readline-powered input, contextual navigation, and auto-generated help.

## Features

- Simple `Command` interface (`Name`, `Help`, `Exec`) for rapid command implementation
- Context-aware prompt with nested command groups
- Built-in help (`help`, `ls`, `?`) and exit (`exit`, `quit`, `q`) commands
- Readline history and tab completion driven by registered contexts/commands
- Configurable prompt prefix and help header text

## Installation

```bash
go get github.com/network-plane/plane-tui@v1
```

## Quick Start

```go
package main

import (
    "fmt"
    "strings"

    "github.com/chzyer/readline"
    tui "github.com/network-plane/plane-tui"
)

type echoCommand struct{}

func (c *echoCommand) Name() string { return "echo" }
func (c *echoCommand) Help() string { return "echo back any arguments" }
func (c *echoCommand) Exec(args []string) {
    fmt.Println(strings.Join(args, " "))
}

func main() {
    rl, err := readline.NewEx(&readline.Config{Prompt: "> "})
    if err != nil {
        panic(err)
    }
    defer rl.Close()

    // Register a global command (root context uses an empty string).
    tui.RegisterCommand("", &echoCommand{})

    if err := tui.Run(rl); err != nil {
        panic(err)
    }
}
```

## Working with Contexts

Commands can be grouped under named contexts to create a directory-like experience:

```go
type listServers struct{}

func (*listServers) Name() string { return "list" }
func (*listServers) Help() string { return "list known servers" }
func (*listServers) Exec(args []string) { fmt.Println("server-1\nserver-2") }

func initCommands() {
    tui.RegisterContext("servers", "Server management commands")
    tui.RegisterCommand("servers", &listServers{})
}
```

With the above registration you can type `servers` to enter the context, then run `list`. Use `/` or `ctx ..` to return to the root context.

## Customising the Prompt and Help Header

```go
tui.SetPrompt("plane> ")
tui.SetHelpHeader("Available contexts and commands:")
```

## Built-in Commands

- `help`, `?`, `h`, `ls` – show help for the current context
- `clear` – clear the screen (root context only)
- `/` or `ctx ..` – return to the root context
- `exit`, `quit`, `q` – terminate the session

## License

MIT
