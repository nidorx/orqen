package chat

import (
	"context"
	"sort"
	"strings"
)

// commands holds all registered commands. Populated by init() in each cmd_*.go file.
var commands = map[string]CommandDef{}

// RegisterCommand adds a command to the global registry.
func RegisterCommand(cmd CommandDef) {
	commands[cmd.Name] = cmd
}

// GetCommand looks up a command by name.
func GetCommand(name string) (CommandDef, bool) {
	cmd, ok := commands[name]
	return cmd, ok
}

// ListCommands returns formatted help text with all registered commands.
func ListCommands() string {
	// Sort command names for deterministic output
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	for _, name := range names {
		cmd := commands[name]
		sb.WriteString("/")
		sb.WriteString(name)
		sb.WriteString(" — ")
		sb.WriteString(cmd.Description)
		sb.WriteString("\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// HandleCommand dispatches a command name + args to the appropriate handler.
func HandleCommand(ctx context.Context, cmd string, args string, bot *TelegramBot, userID string) (string, error) {
	c, ok := GetCommand(cmd)
	if !ok {
		return "Unknown command. Type /help for available commands.", nil
	}
	return c.Handler(ctx, args, bot, userID)
}

// ParseCommand extracts the command name and arguments from a message starting with '/'.
// Returns ("command", "args", true) or ("", "", false) if not a command.
func ParseCommand(text string) (cmd string, args string, ok bool) {
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
