package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"

	"github.com/coder/acp-go-sdk"
	"github.com/nidorx/orqen/pkg/cli"
	"github.com/nidorx/orqen/pkg/utils"
)

var messages = cli.Messages{
	"pt-BR": {
		"stdin_pipe_error": "stdin pipe error: %v",
	},
	"en": {
		"stdin_pipe_error": "stdin pipe error: %v",
	},
}

var (
	agents   = map[string]*Agent{}
	agentsMu sync.Mutex
)

type Agent struct {
	ctx    context.Context
	cmd    *exec.Cmd // defer cmd.Process.Kill()
	client *Client
	conn   *acp.ClientSideConnection
}

func getAgent(projectId string, agentName string, command []string) (*Agent, error) {

	agentId := utils.HashXxh64([]byte(fmt.Sprintf("%s-%s", agentName, projectId)))

	agentsMu.Lock()
	defer agentsMu.Unlock()

	agent, exists := agents[agentId]
	if exists {
		return agent, nil
	}

	ctx := context.Background()
	logger := newLogger(agentName, "")

	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		logger.Log(cli.Sprintf(messages, "stdin_pipe_error", err))
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("[%s] stdout pipe error: %v", agentName, err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("[%s] failed to start acp agent : %v", agentName, err)
	}

	client := &Client{
		logger:    logger,
		terminals: NewTerminalManager(),
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
				return nil, fmt.Errorf("[%s] initialize error: %s", agentName, string(b))
			} else {
				return nil, fmt.Errorf("[%s] initialize error (%d): %s", agentName, re.Code, re.Message)
			}
		} else {
			return nil, fmt.Errorf("[%s] initialize error: %v", agentName, err)
		}
	}
	logger.Log("connected (protocol v%v)\n", initResp.ProtocolVersion)

	agent = &Agent{
		ctx:    ctx,
		cmd:    cmd,
		conn:   conn,
		client: client,
	}

	agents[agentId] = agent
	return agent, nil
}
