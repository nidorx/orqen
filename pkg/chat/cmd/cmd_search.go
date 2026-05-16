package cmd

import (
	"context"
	"fmt"
	"strings"
)

const searchLimit = 10

func init() {
	Register(Command{
		Name:        "search",
		Description: "Search chat memory",
		Handler:     searchCommandHandler,
	})
}

func searchCommandHandler(ctx context.Context, req *Request) (string, error) {
	query := strings.TrimSpace(req.Content)
	if query == "" {
		return "**Usage:** `/search <query>`", nil
	}

	if req.ExtId == "" {
		return "**Info:** Search requires a user session. Use `/start` first.", nil
	}

	if req.SessionManager == nil {
		return "**Error:** Chat store not available.", nil
	}

	chatStore := req.SessionManager.Store
	if chatStore == nil {
		return "**Error:** Chat store not available.", nil
	}

	results, err := chatStore.Search(req.ExtId, query, searchLimit)
	if err != nil {
		return fmt.Sprintf("**Error:** Search error: %v", err), nil
	}

	if len(results) == 0 {
		return fmt.Sprintf("No results found for %q.", query), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Search Results for %q\n\n", query))

	for i, r := range results {
		content := r.Content
		if len(content) > 200 {
			content = content[:200] + "..."
		}
		sb.WriteString(fmt.Sprintf("%d. **[%s]** `%s`\n    > %s\n\n", i+1, r.Role, r.SessionID[:8], content))
	}

	return strings.TrimSuffix(sb.String(), "\n"), nil
}
