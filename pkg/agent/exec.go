package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/coder/acp-go-sdk"
)

func Exec(
	projectId string,
	agentName string,
	laneName string,
	itemName string,
	cwd string,
	prompt string,
	command []string,
	mcps []acp.McpServer,
) error {

	agent, err := getAgent(projectId, agentName, command)
	if err != nil {
		return err
	}

	logger := newLogger(agentName, fmt.Sprintf(" [%s] [%s]", laneName, itemName))

	ctx, cancel := context.WithCancel(agent.ctx)
	defer cancel()

	// New session
	sess, err := agent.conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        cwd,
		McpServers: mcps,
	})
	if err != nil {
		if re, ok := err.(*acp.RequestError); ok {
			if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
				return fmt.Errorf("[%s] newSession error: %s", agentName, string(b))
			} else {
				return fmt.Errorf("[%s] newSession error (%d): %s", agentName, re.Code, re.Message)
			}
		} else {
			return fmt.Errorf("[%s] newSession error: %v", agentName, err)
		}
	}
	logger.Log("session created: %s\n", sess.SessionId)

	sessionSetClient(sess.SessionId, &ClientSession{
		logger:       logger,
		toolCallById: make(map[acp.ToolCallId]*acp.SessionUpdateToolCall),
		agentChunk:   &Chunk{logger: logger},
		userChunk:    &Chunk{logger: logger, prefix: "User"},
		thoughtChunk: &Chunk{logger: logger, prefix: "Thinking"},
	})
	defer sessionDelClient(sess.SessionId)

	// Send prompt and wait for completion while streaming updates are printed via SessionUpdate
	_, err = agent.conn.Prompt(ctx, acp.PromptRequest{
		SessionId: sess.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	})
	if err != nil {
		// If it's a JSON-RPC RequestError, surface more detail for troubleshooting
		if re, ok := err.(*acp.RequestError); ok {
			if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
				return fmt.Errorf("[%s] prompt error: %s", agentName, string(b))
			} else {
				return fmt.Errorf("[%s] prompt error (%d): %s", agentName, re.Code, re.Message)
			}
		} else {
			return fmt.Errorf("[%s] prompt error: %v", agentName, err)
		}
	}

	logger.Log("finished\n")

	return nil
}
