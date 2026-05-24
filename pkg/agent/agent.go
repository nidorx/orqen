package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"time"

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
	agents        = map[string]*Agent{}
	agentsMu      sync.Mutex
	agentsIdleTms = map[string]*time.Timer{}
)

type Agent struct {
	id          string
	ctx         context.Context
	cmd         *exec.Cmd // defer cmd.Process.Kill()
	client      *Client
	conn        *acp.ClientSideConnection
	loadSession bool // whether the agent supports session loading via LoadSession
}

func (a *Agent) NewSession(ctx context.Context, params acp.NewSessionRequest) (acp.NewSessionResponse, error) {
	return a.conn.NewSession(ctx, params)
}

func (a *Agent) Cancel(ctx context.Context, params acp.CancelNotification) error {
	return a.conn.Cancel(ctx, params)
}

func (a *Agent) CloseSession(ctx context.Context, params acp.CloseSessionRequest) (acp.CloseSessionResponse, error) {
	return a.conn.CloseSession(ctx, params)
}

func (a *Agent) Prompt(ctx context.Context, params acp.PromptRequest) (acp.PromptResponse, error) {
	return a.conn.Prompt(ctx, params)
}

func (a *Agent) LoadSession(ctx context.Context, params acp.LoadSessionRequest) (acp.LoadSessionResponse, error) {
	return a.conn.LoadSession(ctx, params)
}

// ScheduleIdle sets a timer to kill the subprocess after the idle timeout.
func (a *Agent) ScheduleIdle(timeout time.Duration) {
	agentId := a.id

	agentsMu.Lock()
	defer agentsMu.Unlock()

	// Don't schedule if there are active sessions
	if _, exists := agents[agentId]; exists {
		timer := time.AfterFunc(timeout, func() {
			agentsMu.Lock()
			defer agentsMu.Unlock()
			// Double-check: maybe a new session started while waiting
			if p, stillExists := agents[agentId]; stillExists {
				if p.cmd != nil && p.cmd.Process != nil {
					_ = p.cmd.Process.Kill()
				}
				delete(agents, agentId)
			}
			delete(agentsIdleTms, agentId)
		})
		agentsIdleTms[agentId] = timer
	}
}

// GetAgent returns a cached agent subprocess or spawns a new one.
func GetAgent(projectId string, agentName string, command []string) (*Agent, error) {

	agentId := utils.HashXxh64([]byte(fmt.Sprintf("%s-%s", agentName, projectId)))

	agentsMu.Lock()
	defer agentsMu.Unlock()

	// Cancel idle timer if running
	if timer, exists := agentsIdleTms[agentId]; exists {
		timer.Stop()
		delete(agentsIdleTms, agentId)
	}

	agent, exists := agents[agentId]
	if exists {
		return agent, nil
	}

	ctx := context.Background()
	logger := NewLogger(agentName, "")

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
		ctx:         ctx,
		cmd:         cmd,
		conn:        conn,
		client:      client,
		loadSession: initResp.AgentCapabilities.LoadSession,
	}

	agents[agentId] = agent
	return agent, nil
}