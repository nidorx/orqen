package cmd

import (
	"context"
)

func init() {
	Register(Command{
		Name:        "start",
		Description: "Initialize chat session and welcome message",
		Handler:     startCommandHandler,
	})
}

func startCommandHandler(ctx context.Context, req *Request) (string, error) {
	return "## Welcome to Orqen Chat! 🤖\n\n" +
		"I'm your remote assistant for the Orqen project.\n" +
		"You can ask me to create workitems, check status, explore files, and more.\n\n" +
		"Type `/help` to see available commands.", nil
}
