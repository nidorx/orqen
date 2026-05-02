package project

import (
	"embed"
	"time"
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

// AgentClient defines a single agent client at project level.
type AgentClient struct {
	Command []string `yaml:"command"`
}

// Execution holds project-level execution settings.
type Execution struct {
	MaxAgents            int `yaml:"max_agents"`
	SleepIntervalSeconds int `yaml:"sleep_interval_seconds"`
}

// WorkItem representa uma tarefa que está disponível em uma Lane
type WorkItem struct {
	ID           int         // unique identifier for this work item
	JobID        string      // InvocationHandle ID
	Name         string      // directory/file name (e.g., TASK-001-create-project)
	Files        []string    // files in directory (e.g., TASK-001.md, TASK-001-SUmMARY.md)
	Lane         *Lane       // the lane this item belongs to
	InProgress   bool        // indica que um agente está executando a tarefa
	ModTime      time.Time   // atualização mais recente do item
	Dependencies []*WorkItem // todas as dependencias desse WorkItem
}

// AgentInvoker defines the function for invoking agent executions
// AgentInvoker starts an agent execution for the given work item
type AgentInvoker func(prompt string, item *WorkItem) error

// WorkItemInvoker defines the interface for invoking and managing agent executions
// WorkItemInvoker starts an execution for the given work item
// Returns an invocation handle that can be used to track progress
type WorkItemInvoker func(project *Project, module *Module, lane *Lane, item *WorkItem) (InvocationHandle, error)

// InvocationHandle represents a running agent invocation
type InvocationHandle struct {
	ID   string        // unique identifier for this invocation
	Item *WorkItem     // the work item being processed
	Done chan struct{} // closed when the invocation completes
	err  error         // error from the invocation (if any)
}

// Wait blocks until the invocation completes
func (h InvocationHandle) Wait() error {
	<-h.Done
	return h.err
}

// IsComplete returns true if the invocation has completed
func (h InvocationHandle) IsComplete() bool {
	select {
	case <-h.Done:
		return true
	default:
		return false
	}
}
