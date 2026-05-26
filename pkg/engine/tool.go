package engine

import (
	"fmt"
)

// Tool represents a user-defined tool configuration from orqen.yaml.
type Tool struct {
	Command     []string          `yaml:"command,omitempty"`     // default command
	Windows     []string          `yaml:"windows,omitempty"`     // windows command
	Linux       []string          `yaml:"linux,omitempty"`       // linux command
	Darwin      []string          `yaml:"darwin,omitempty"`      // darwin command
	Timeout     int               `yaml:"timeout,omitempty"`     // timeout in seconds (default 30)
	Description string            `yaml:"description,omitempty"` // tool description for MCP
	Args        map[string]string `yaml:"args,omitempty"`        // param_name -> description
}

// Validate a tool definition for required fields and valid OS keys.
func (t *Tool) Validate() error {
	// Check if there's at least one OS-specific command
	for _, goos := range []string{"windows", "linux", "darwin"} {
		if _, err := t.GetCommandForOS(goos); err == nil {
			return nil
		}
	}

	return fmt.Errorf("tool has no command and no valid OS-specific commands (windows/darwin/linux)")
}

// GetCommandForOS returns the command array for the current OS, falling back to the default command.
func (t *Tool) GetCommandForOS(goos string) ([]string, error) {

	command := t.Command

	// Check OS-specific override
	switch goos {
	case "windows":
		if len(t.Windows) > 0 {
			command = t.Windows
		}
	case "linux":
		if len(t.Linux) > 0 {
			command = t.Linux
		}
	case "darwin":
		if len(t.Darwin) > 0 {
			command = t.Darwin
		}
	}

	// Fall back to default
	if len(command) > 0 {
		return command, nil
	}
	return nil, fmt.Errorf("no command defined for tool (neither default nor %s-specific)", goos)
}
