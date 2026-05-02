package project

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/goccy/go-yaml"
)

var (
	projectsMu sync.Mutex
	projects   = map[string]*Project{}
)

func Get(id string) *Project {
	projectsMu.Lock()
	defer projectsMu.Unlock()
	return projects[id]
}

// Load loads and validates the project configuration from the given project directory.
// It expects the file .orqen/orqen.yaml to exist within the directory.
func Load(projectDir string) (*Project, error) {

	projectDir = filepath.Clean(projectDir)

	id := hashXxh64([]byte(projectDir))

	projectsMu.Lock()
	defer projectsMu.Unlock()
	if proj, exists := projects[id]; exists {
		return proj, nil
	}

	// Validate directory structure
	if err := ValidateDir(projectDir); err != nil {
		return nil, fmt.Errorf("Invalid project directory: %v\n", err)
	}

	configPath := filepath.Join(projectDir, ProjectConfigDir, ProjectConfigFilename)

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read project config file %s: %w", configPath, err)
	}

	var proj Project
	if err := yaml.Unmarshal(data, &proj); err != nil {
		return nil, fmt.Errorf("failed to parse project config file: %w", err)
	}

	proj.Id = id
	proj.DirAbs = projectDir

	// Apply defaults
	applyDefaults(&proj)

	// Validate configuration
	if err := validate(&proj); err != nil {
		return nil, fmt.Errorf("invalid project configuration: %w", err)
	}

	// create directories and prompts
	if err := initialize(&proj); err != nil {
		return nil, err
	}
	projects[id] = &proj
	return &proj, nil
}

// ValidateDir checks if the given directory contains a valid .orqen/orqen.yaml file.
// Returns an error if the directory doesn't exist, doesn't contain .orqen, or the config file is missing.
func ValidateDir(projectDir string) error {
	projectDir = filepath.Clean(projectDir)

	// Check if directory exists
	info, err := os.Stat(projectDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory does not exist: %s", projectDir)
		}
		return fmt.Errorf("error accessing directory %s: %w", projectDir, err)
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", projectDir)
	}

	// Check if .orqen directory exists
	orqenDir := filepath.Join(projectDir, ProjectConfigDir)
	orqenInfo, err := os.Stat(orqenDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("directory %s does not contain a %s subdirectory", projectDir, ProjectConfigDir)
		}
		return fmt.Errorf("error accessing %s subdirectory: %w", ProjectConfigDir, err)
	}

	if !orqenInfo.IsDir() {
		return fmt.Errorf("%s exists but is not a directory", ProjectConfigDir)
	}

	// Check if orqen.yaml exists
	configPath := filepath.Join(orqenDir, ProjectConfigFilename)
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%s subdirectory does not contain %s", ProjectConfigDir, ProjectConfigFilename)
		}
		return fmt.Errorf("error accessing config file %s: %w", configPath, err)
	}

	return nil
}

// applyDefaults sets sensible defaults for zero-value fields in ProjectConfig.
func applyDefaults(proj *Project) {

	if proj.Execution.MaxAgents <= 0 {
		proj.Execution.MaxAgents = 10
	}

	if proj.Execution.SleepIntervalSeconds <= 0 {
		proj.Execution.SleepIntervalSeconds = 60
	}

	// Apply defaults for each module
	for _, mod := range proj.Modules {
		mod.Project = proj

		if mod.Dir == "" {
			mod.Dir = "."
		}

		mod.DirAbs = filepath.Clean(path.Join(proj.DirAbs, mod.Dir))
		mod.DirPrompts = filepath.Clean(path.Join(mod.DirAbs, "prompts"))

		modType := strings.ToUpper(mod.Name)

		// special lane
		var inbox *Lane

		for _, lane := range mod.Lanes {
			if lane.Name == "inbox" {
				inbox = lane
				break
			}
		}

		if inbox == nil {
			inbox = &Lane{Name: "inbox"}
			mod.Lanes = append([]*Lane{inbox}, mod.Lanes...)
		}

		if inbox.Purpose == "" {
			inbox.Purpose = fmt.Sprintf("User ideas that are ready to be transformed into %s by the agent", modType)
		}

		if inbox.MaxAgents <= 0 {
			inbox.MaxAgents = 2
		}

		if len(inbox.AgentBehavior) == 0 {
			inbox.AgentBehavior = []string{
				"Read the inbox file to understand the idea",
				fmt.Sprintf("Analyze and decompose into %s (see `%s/%s.md`)", modType, path.Join(mod.Dir, "prompts"), modType),
				fmt.Sprintf("An idea may result in one or more %s - the agent should make appropriate judgment", modType),
				fmt.Sprintf(
					"Create %s using `orqen` MCP tool `create_%s`, following the template structure (`%s/%s.md`)",
					modType, modType, path.Join(mod.Dir, "prompts"), modType,
				),
				"**Terminate execution** after processing the inbox file",
			}
		}

		if len(inbox.CriticalRules) == 0 {
			inbox.CriticalRules = []string{
				fmt.Sprintf("Move the idea file to the directory of the first %s created from that idea. Do not remove the file, just move it.", modType),
			}
		}

		for j, lane := range mod.Lanes {
			if lane.MaxAgents <= 0 {
				lane.MaxAgents = 1
			}

			lane.Dir = fmt.Sprintf("%02d_%s", j+1, strings.ToLower(lane.Name))
			lane.DirAbs = filepath.Clean(path.Join(mod.DirAbs, lane.Dir))
			lane.Module = mod
		}
	}
}

// initialize create directories and prompts
func initialize(proj *Project) error {

	for _, mod := range proj.Modules {

		modType := strings.ToUpper(mod.Name)

		// create prompts directory
		modPromptsDir := mod.DirPrompts
		if err := os.MkdirAll(modPromptsDir, os.ModeDir); err != nil {
			return err
		}

		var defaultPrompts = []string{
			"HEADER.md",
		}

		for _, promptFile := range defaultPrompts {
			dstPromptFileLoc := path.Join(modPromptsDir, promptFile)
			_, err := os.Stat(dstPromptFileLoc)

			if err == nil {
				// exists
				prompt, err := os.ReadFile(dstPromptFileLoc)
				if err != nil {
					return err
				}

				mod.Prompt = string(prompt)
				continue
			} else if !errors.Is(err, os.ErrNotExist) {
				return err
			}

			srcPromptFile, err := embedPromptsFS.Open("prompts/" + promptFile)
			if err != nil {
				return err
			}

			promptBytes, err := io.ReadAll(srcPromptFile)
			if err != nil {
				return err
			}
			prompt := string(promptBytes)

			if promptFile == "HEADER.md" {

				// obtém artefatos de exemplo
				var (
					artifacts         []string
					artifactsExamples []string
					artifactsByName   = map[string]bool{}
				)
				for i, lane := range mod.Lanes {
					if len(lane.AgentBehavior) > 0 && len(lane.Artifacts) > 0 {
						for _, artifact := range lane.Artifacts {
							if exists := artifactsByName[artifact]; !exists {
								artifactsByName[artifact] = true
								artifacts = append(artifacts, artifact)
								artifactsExamples = append(
									artifactsExamples,
									fmt.Sprintf("- `$MOD_TYPE-%04d-%s.md`", i+1, artifact),
								)
								artifactsExamples = append(
									artifactsExamples,
									fmt.Sprintf("- `$MOD_TYPE-%04d-%s-02.md`", i+2, artifact),
								)
								artifactsExamples = append(
									artifactsExamples,
									fmt.Sprintf("- `$MOD_TYPE-%04d-%s-03.md`", i+3, artifact),
								)
							}
						}
					}
				}

				if len(artifacts) > 0 {
					artifactsInstructions := append([]string{
						"### $MOD_TYPE Artifacts",
						"Pattern: `$MOD_TYPE-${SEQUENCE}-${ARTIFACT}[-{ARTIFACT_SEQUENCE}].md`",
						"",
						fmt.Sprintf("- `${ARTIFACT}`: %s", strings.Join(artifacts, ", ")),
						"- `${ARTIFACT_SEQUENCE}`: Optional 2-digit sequence number",
						"",
						"**Examples:**",
					}, artifactsExamples...)

					prompt = strings.Replace(prompt, "$ARTIFACTS_INSTRUCTIONS", strings.Join(artifactsInstructions, "\n"), 1)
				}
				prompt = prompt + strings.TrimSpace(mod.ExtraPrompt)
				prompt = strings.ReplaceAll(prompt, "$MOD_TYPE", modType)
				mod.Prompt = prompt
			} else {
				prompt = strings.ReplaceAll(prompt, "$MOD_TYPE", modType)
			}

			// Create the destination file on the OS
			dstPromptFile, err := os.Create(dstPromptFileLoc)
			if err != nil {
				srcPromptFile.Close()
				return err
			}

			// stream the content from embed to OS
			_, err = io.WriteString(dstPromptFile, prompt)

			srcPromptFile.Close()
			dstPromptFile.Close()

			if err != nil {
				return err
			}
		}

		// create lanes
		for i, lane := range mod.Lanes {
			if err := os.MkdirAll(lane.DirAbs, os.ModeDir); err != nil {
				return err
			}

			// create lane prompt
			if len(lane.AgentBehavior) > 0 {
				promptFile := strings.ToUpper(lane.Dir) + ".md"
				dstPromptFileLoc := path.Join(modPromptsDir, promptFile)
				_, err := os.Stat(dstPromptFileLoc)

				if err == nil {
					// exists
					prompt, err := os.ReadFile(dstPromptFileLoc)
					if err != nil {
						return err
					}

					lane.Prompt = string(prompt)
					continue
				} else if !errors.Is(err, os.ErrNotExist) {
					return err
				}

				promptLines := []string{
					"## Workflow Definition - Lanes",
				}

				for j, olane := range mod.Lanes {
					if j != i {
						promptLines = append(promptLines,
							fmt.Sprintf("### %s", olane.Dir),
							fmt.Sprintf("**Purpose:** %s\n", strings.TrimSpace(olane.Purpose)),
						)
						continue
					}

					promptLines = append(promptLines,
						fmt.Sprintf("### %s", lane.Dir),
						fmt.Sprintf("**Purpose:** %s\n", strings.TrimSpace(lane.Purpose)),
						"**Agent Behavior:**",
					)
					for i, behavior := range lane.AgentBehavior {
						promptLines = append(promptLines, fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(behavior)))
					}
					promptLines = append(promptLines, "")

					if len(lane.CriticalRules) > 0 {
						promptLines = append(promptLines, "**Critical Rules:** ")
						for i, rule := range lane.CriticalRules {
							promptLines = append(promptLines, fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(rule)))
						}
						promptLines = append(promptLines, "")
					}

					promptLines = append(promptLines, strings.TrimSpace(lane.ExtraPrompt))
				}

				prompt := strings.Join(promptLines, "\n")
				lane.Prompt = prompt

				// Create the destination file on the OS
				dstPromptFile, err := os.Create(dstPromptFileLoc)
				if err != nil {
					return err
				}

				// stream the content to OS
				_, err = io.WriteString(dstPromptFile, prompt)
				dstPromptFile.Close()
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validate checks the project configuration for required fields.
func validate(proj *Project) error {
	if len(proj.Modules) == 0 {
		return fmt.Errorf("no modules defined")
	}

	for i, mod := range proj.Modules {
		if mod.Name == "" {
			return fmt.Errorf("module at index %d has no name", i)
		}
		if len(mod.Lanes) == 0 {
			return fmt.Errorf("module %q has no lanes defined", mod.Name)
		}

		// Validate lane names are unique
		laneNames := make(map[string]bool)
		for j, lane := range mod.Lanes {
			if lane.Name == "" {
				return fmt.Errorf("module %q has lane at index %d with no name", mod.Name, j)
			}
			if laneNames[lane.Name] {
				return fmt.Errorf("module %q has duplicate lane name: %s", mod.Name, lane.Name)
			}
			laneNames[lane.Name] = true
		}
	}

	// Validate agent clients exist
	for name, client := range proj.Agents.Clients {
		if len(client.Command) == 0 {
			return fmt.Errorf("agent client %q has empty command", name)
		}
	}

	return nil
}
