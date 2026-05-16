package cmd

import (
	"context"
	"strings"

	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

type Request struct {
	ExtId          string // external chat id (telegram chat id)
	Content        string
	Project        *engine.Project
	SessionManager *memory.SessionManager
}

// Handler is the signature for deterministic command handlers.
// Returns an error if the command fails; response is sent via the bot.
type Handler func(ctx context.Context, req *Request) (string, error)

// Command registers a command name, description, and its handler.
type Command struct {
	Name        string
	Description string
	Handler     Handler
}

// commands holds all registered commands. Populated by init() in each cmd_*.go file.
var commands = map[string]Command{}

// Register adds a command to the global registry.
func Register(cmd Command) {
	commands[cmd.Name] = cmd
}

// Get looks up a command by name.
func Get(name string) (Command, bool) {
	cmd, ok := commands[name]
	return cmd, ok
}

// Parse extracts the command name and arguments from a message starting with '/'.
// Returns ("command", "args", true) or ("", "", false) if not a command.
func Parse(text string) (cmd string, args string, ok bool) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "/") {
		return "", "", false
	}
	text = strings.TrimPrefix(text, "/")
	parts := strings.SplitN(text, " ", 2)
	cmd = strings.ToLower(parts[0])
	if len(parts) > 1 {
		args = strings.TrimSpace(parts[1])
	}
	return cmd, args, true
}
