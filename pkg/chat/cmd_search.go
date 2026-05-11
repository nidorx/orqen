package chat

import (
	"context"
	"fmt"
	"strings"
)

const searchLimit = 10

func init() {
	RegisterCommand(CommandDef{
		Name:        "search",
		Description: "Search chat memory",
		Handler:     handleSearch,
	})
}

func handleSearch(ctx context.Context, args string, bot *TelegramBot, userID string) (string, error) {
	query := strings.TrimSpace(args)
	if query == "" {
		return "Usage: /search <query>", nil
	}

	if userID == "" {
		return "Search requires a user session. Use /start first.", nil
	}

	chatStore := bot.ChatStore
	if chatStore == nil {
		return "Chat store not available.", nil
	}

	results, err := chatStore.Search(userID, query, searchLimit)
	if err != nil {
		return fmt.Sprintf("Search error: %v", err), nil
	}

	if len(results) == 0 {
		return fmt.Sprintf("No results found for %q.", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for %q:\n\n", query))

	for i, r := range results {
		content := r.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n    %s\n\n", i+1, r.Role, r.SessionID[:8], content))
	}

	return strings.TrimSuffix(sb.String(), "\n"), nil
}
