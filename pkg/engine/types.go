package engine

import (
	"embed"
)

//go:embed prompts
var embedPromptsFS embed.FS

const (

	// projectConfigDir is the name of the directory containing project configuration.
	projectConfigDir = ".orqen"
	// projectConfigFile is the name of the project configuration file.
	projectConfigFile = "orqen.yaml"
)

// Agent holds project-level agent settings.
//
// YAML structure:
//
//	agents:
//	  default: "qwen"              # default agent client name
//	  clients:
//	    qwen:
//	      command: ["qwen", "--yolo", "--acp"]
type Agent struct {
	Default string                 `yaml:"default"`
	Clients map[string]AgentClient `yaml:"clients"`
}

func (a *Agent) GetName(agent string) string {
	if agent == "" {
		agent = a.Default
	}

	return agent
}

func (a *Agent) GetCommand(agent string) []string {
	if agent == "" {
		agent = a.Default
	}

	client := a.Clients[agent]

	return client.Command
}

// AgentClient defines a single agent client configuration.
//
// YAML structure:
//
//	command: ["qwen", "--yolo", "--acp"]   # CLI command and arguments to invoke the agent
type AgentClient struct {
	Command []string `yaml:"command"`
}

// Execution holds project-level execution settings.
//
// YAML structure:
//
//	execution:
//	  max_agents: 10                  # maximum concurrent agents across all modules (0 = unlimited)
//	  sleep_interval_seconds: 60      # seconds between work cycles
type Execution struct {
	MaxAgents            int `yaml:"max_agents"`
	SleepIntervalSeconds int `yaml:"sleep_interval_seconds"`
}

// AgentInvoker defines the function signature for invoking agent executions.
// It receives the synthesized prompt and the work item being processed.
type AgentInvoker func(prompt string, item *WorkItem) error

// WorkItemInvoker defines the function signature for invoking and managing agent executions.
// Returns an InvocationHandle that can be used to track progress.
type WorkItemInvoker func(project *Project, module *Module, lane *Lane, item *WorkItem) (InvocationHandle, error)

// InvocationHandle represents a running agent invocation.
type InvocationHandle struct {
	Item *WorkItem     // the work item being processed
	Done chan struct{} // closed when the invocation completes
	err  error         // error from the invocation (if any)
}

// Wait blocks until the invocation completes.
func (h InvocationHandle) Wait() error {
	<-h.Done
	return h.err
}

// IsComplete returns true if the invocation has completed.
func (h InvocationHandle) IsComplete() bool {
	select {
	case <-h.Done:
		return true
	default:
		return false
	}
}

// SchemaField describes an observed front matter field across a module.
type SchemaField struct {
	Field  string   `json:"field"`  // field name
	Types  []string `json:"types"`  // observed Go/YAML types (string, bool, int, list, map)
	Values []any    `json:"values"` // unique observed values (up to schemaMaxValues)
}
