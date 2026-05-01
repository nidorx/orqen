package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/coder/acp-go-sdk"
	"github.com/nidorx/orqen/pkg/cli"
)

var messages = cli.Messages{
	"pt-BR": {
		"stdin_pipe_error": "stdin pipe error: %v",
	},
	"en": {
		"stdin_pipe_error": "stdin pipe error: %v",
	},
}

func Exec(
	jobName string,
	laneName string,
	cwd string,
	prompt string,
	command []string,
	mcps []acp.McpServer,
) error {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent := command[0]

	logger := newLogger(agent, laneName, jobName)

	cmd := exec.CommandContext(ctx, agent, command[1:]...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		logger.Log(cli.Sprintf(messages, "stdin_pipe_error", err))
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("[%s] stdout pipe error: %v", agent, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[%s] failed to start acp agent : %v", agent, err)
	}
	defer cmd.Process.Kill()

	client := &GenericClient{
		autoApprove: true,
		logger:      logger,
	}
	conn := acp.NewClientSideConnection(client, stdin, stdout)
	conn.SetLogger(slog.Default())

	// Initialize
	initResp, err := conn.Initialize(ctx, acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersionNumber,
		ClientCapabilities: acp.ClientCapabilities{
			Fs: acp.FileSystemCapabilities{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: true,
		},
	})
	if err != nil {
		if re, ok := err.(*acp.RequestError); ok {
			if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
				return fmt.Errorf("[%s] initialize error: %s", agent, string(b))
			} else {
				return fmt.Errorf("[%s] initialize error (%d): %s", agent, re.Code, re.Message)
			}
		} else {
			return fmt.Errorf("[%s] initialize error: %v", agent, err)
		}
	}
	logger.Log("connected (protocol v%v)\n", initResp.ProtocolVersion)

	// New session
	newSess, err := conn.NewSession(ctx, acp.NewSessionRequest{
		Cwd:        cwd,
		McpServers: mcps,
	})
	if err != nil {
		if re, ok := err.(*acp.RequestError); ok {
			if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
				return fmt.Errorf("[%s] newSession error: %s", agent, string(b))
			} else {
				return fmt.Errorf("[%s] newSession error (%d): %s", agent, re.Code, re.Message)
			}
		} else {
			return fmt.Errorf("[%s] newSession error: %v", agent, err)
		}
	}
	logger.Log("session created: %s\n", newSess.SessionId)

	// Send prompt and wait for completion while streaming updates are printed via SessionUpdate
	_, err = conn.Prompt(ctx, acp.PromptRequest{
		SessionId: newSess.SessionId,
		Prompt:    []acp.ContentBlock{acp.TextBlock(prompt)},
	})
	if err != nil {
		// If it's a JSON-RPC RequestError, surface more detail for troubleshooting
		if re, ok := err.(*acp.RequestError); ok {
			if b, mErr := json.MarshalIndent(re, "", "  "); mErr == nil {
				return fmt.Errorf("[%s] prompt error: %s", agent, string(b))
			} else {
				return fmt.Errorf("[%s] prompt error (%d): %s", agent, re.Code, re.Message)
			}
		} else {
			return fmt.Errorf("[%s] prompt error: %v", agent, err)
		}
	}

	logger.Log("finished\n")

	return nil
}
