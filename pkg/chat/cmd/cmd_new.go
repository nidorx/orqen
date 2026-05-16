package cmd

import (
	"context"
	"fmt"
)

func init() {
	Register(Command{
		Name:        "new",
		Description: "Start a new conversation session",
		Handler:     newCommandHandler,
	})
}

func newCommandHandler(ctx context.Context, req *Request) (string, error) {
	if req.ExtId == "" {
		return "**Info:** Use `/start` to initialize a session first.", nil
	}

	sessionMgr := req.SessionManager
	if sessionMgr == nil {
		return "**Error:** Session manager not available.", nil
	}

	newSession, err := sessionMgr.NewSession(req.ExtId)
	if err != nil {
		return fmt.Sprintf("**Error:** Error creating new session: %v", err), nil
	}

	return fmt.Sprintf("## New Session Started\n\nSession ID: `%s`\n\nType `/help` to see available commands.", newSession.ID[:8]), nil
}
