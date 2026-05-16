package cmd

import (
	"context"
	"sort"
	"strings"
)

func init() {
	Register(Command{
		Name:        "help",
		Description: "List available commands",
		Handler:     helpCommandHandler,
	})
}

// helpCommandHandler returns formatted help text with all registered commands.
func helpCommandHandler(ctx context.Context, req *Request) (string, error) {

	// Sort command names for deterministic output
	names := make([]string, 0, len(commands))
	for name := range commands {
		names = append(names, name)
	}
	sort.Strings(names)

	var sb strings.Builder
	for _, name := range names {
		cmd := commands[name]
		sb.WriteString("- /")
		sb.WriteString(name)
		sb.WriteString(" - ")
		sb.WriteString(cmd.Description)
		sb.WriteString("\n")
	}
	return strings.TrimSuffix(sb.String(), "\n"), nil

}
