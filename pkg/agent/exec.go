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
	mcpServers []acp.McpServer,
	priorSessionID string,
	onSessionId func(sessionID string),
) (err error) {

	agent, err := GetAgent(projectId, agentName, command)
	if err != nil {
		return err
	}

	logger := NewLogger(agentName, fmt.Sprintf(" [%s] [%s]", laneName, itemName))

	ctx, cancel := context.WithCancel(agent.ctx)
	defer cancel()

	// Session handling: attempt reload if prior session exists and agent supports it
	var sessID acp.SessionId

	if priorSessionID != "" && agent.loadSession {
		logger.Log("attempting to reload session: %s\n", priorSessionID)
		_, loadErr := agent.LoadSession(ctx, acp.LoadSessionRequest{
			SessionId:  acp.SessionId(priorSessionID),
			Cwd:        cwd,
			McpServers: mcpServers,
		})
		if loadErr == nil {
			sessID = acp.SessionId(priorSessionID)
			logger.Log("session reloaded: %s\n", sessID)
		} else {
			logger.Log("session reload failed (%v), falling back to new session\n", loadErr)
		}
	}

	// Create new session if no prior session or reload failed
	if sessID == "" {
		sess, err := agent.NewSession(ctx, acp.NewSessionRequest{
			Cwd:        cwd,
			McpServers: mcpServers,
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
		sessID = sess.SessionId
		logger.Log("session created: %s\n", sessID)
		onSessionId(string(sessID))
	}

	ClientSessionSet(sessID, ClientSessionNew(logger, nil))

	defer ClientSessionDel(sessID)

	// Send prompt and wait for completion while streaming updates are printed via SessionUpdate
	_, err = agent.Prompt(ctx, acp.PromptRequest{
		SessionId: sessID,
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
