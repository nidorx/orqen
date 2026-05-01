package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/coder/acp-go-sdk"
)

func Exec(cwd string, prompt string, command []string, mcps []acp.McpServer) error {

	// peerInput io.Writer, peerOutput io.Reader

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent := command[0]

	cmd := exec.CommandContext(ctx, agent, command[1:]...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("[%s] stdin pipe error: %v", agent, err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("[%s] stdout pipe error: %v", agent, err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("[%s] failed to start acp agent : %v", agent, err)
	}
	defer cmd.Process.Kill()

	client := &GenericClient{autoApprove: true}
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
	fmt.Printf("[%s] connected (protocol v%v)\n", agent, initResp.ProtocolVersion)

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
	fmt.Printf("[%s] created session: %s\n", agent, newSess.SessionId)

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

	// fmt.Print("\n\n")

	fmt.Printf("\n\n[%s] prompt finished\n", agent)

	return nil
}

// func mustCwd() string {
// 	wd, err := os.Getwd()
// 	if err != nil {
// 		return "."
// 	}
// 	return wd
// }
