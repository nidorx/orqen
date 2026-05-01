package project

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/coder/acp-go-sdk"
	"github.com/nidorx/orqen/pkg/agent"
	"github.com/nidorx/orqen/pkg/cli"
	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/project"
)

// serviceMessages defines all i18n messages used by SingleProjectService.
var serviceMessages = cli.Messages{
	"pt-BR": {
		"ask_cwd":             "O projeto que vamos trabalhar agora está neste diretório?\nCaminho: %s\nSe estiver correto, responda com Y ou pressione Enter. Caso contrário, responda com N.",
		"ask_project_dir":     "Certo. Me informe o caminho do diretório do projeto para que eu possa continuar.",
		"project_loaded":      "Pronto. Carreguei o projeto a partir de %s. Encontrei %d módulos e defini %s como agente padrão. Intervalo de execução: %ds.",
		"error_loading":       "Não consegui carregar o projeto: %v\nRevise o arquivo de configuração e tente novamente.",
		"error_empty_dir":     "O caminho do diretório não pode estar vazio.\nMe informe um caminho válido para continuar.",
		"error_reading_input": "Não consegui ler sua resposta: %v\nTente novamente.",
		"starting_engine":     "Perfeito. Vou iniciar a execução agora. Se quiser interromper, é só fechar a janela ou usar Ctrl+C.",
		"listening":           "O serviço de integração HTTP está disponível em http://%s:%d",
	},
	"en": {
		"ask_cwd":             "Is this the project we're working on?\nPath: %s\nIf it looks correct, reply with Y or press Enter. Otherwise, reply with N.",
		"ask_project_dir":     "Alright. Please share the project directory path so I can continue.",
		"project_loaded":      "Done. I loaded the project from %s. Found %d modules and set %s as the default agent. Execution interval: %ds.",
		"error_loading":       "I couldn't load the project: %v\nReview the configuration file and try again.",
		"error_empty_dir":     "Directory path cannot be empty.\nPlease provide a valid path so I can continue.",
		"error_reading_input": "I couldn't read your input: %v\nPlease try again.",
		"starting_engine":     "Alright. I'll start the execution now. If you need to stop, just close the window or use Ctrl+C.",
		"listening":           "The HTTP integration service is available at http://%s:%d",
	},
}

type Service struct {
	proj *project.Project
}

func (s *Service) Name() string {
	return "SingleProjectService"
}

func (s *Service) OnStart() error {

	orqenExec := os.Args[0]
	orqenPort := conf.GetHttpServer().Port

	// Prompt user for proj directory and load configuration
	proj := loadProject()
	proj.WithInvoker(func(prompt string, itm *project.WorkItem) error {
		cwd := proj.DirAbs
		command := proj.Agents.GetCommand(itm.Lane.Agent)

		// @TODO: sent context item.Files

		// initialize agent (ACP)
		return agent.Exec(cwd, prompt, command, []acp.McpServer{
			{
				// disponibiliza acesso ao mcp do próprio orqen
				Stdio: &acp.McpServerStdio{
					Name:    "orqen",
					Command: orqenExec,
					Args: []string{
						"--mcp",
						"--port=" + strconv.Itoa(orqenPort),
						"--project=" + strconv.Itoa(orqenPort),
						"--job=" + itm.JobID,
					},
					Env: make([]acp.EnvVariable, 0),
				},
			},
		})
	})
	proj.Start()

	cli.Printf(serviceMessages, "starting_engine")

	return nil
}

func (s *Service) OnStop() error {
	return nil
}

func (s *Service) String() string {
	cfg := conf.GetHttpServer()
	return cli.Sprintf(serviceMessages, "listening", cfg.IP, cfg.Port)
}

func New() *Service {
	return &Service{}
}

// loadProject prompts the user for a project directory path, validates it,
// and loads the project configuration. Returns the validated project directory path.
func loadProject() *project.Project {
	reader := bufio.NewReader(os.Stdin)

	// Ask once if user wants to use the current directory
	cwd, err := os.Getwd()
	if err == nil {
		cli.Printf(serviceMessages, "ask_cwd", cwd)
		answer, readErr := reader.ReadString('\n')
		if readErr == nil {
			answer = strings.ToUpper(strings.TrimSpace(answer))
			if answer == "" || strings.EqualFold(answer, "Y") || strings.EqualFold(answer, "YES") {
				proj, loadErr := project.Load(cwd)
				if loadErr == nil {
					cli.Printf(serviceMessages, "project_loaded", cwd, len(proj.Modules), proj.Agents.Default, proj.Execution.SleepIntervalSeconds)
					return proj
				}
				cli.Printf(serviceMessages, "error_loading", loadErr)
			}
		}
	}

	for {
		cli.Printf(serviceMessages, "ask_project_dir")
		var input string

		input, err = reader.ReadString('\n')
		if err != nil {
			cli.Printf(serviceMessages, "error_reading_input", err)
			continue
		}

		projectDir := strings.TrimSpace(input)
		if projectDir == "" {
			cli.Printf(serviceMessages, "error_empty_dir")
			continue
		}

		// validate and load configuration
		proj, err := project.Load(projectDir)
		if err != nil {
			cli.Printf(serviceMessages, "error_loading", err)
			continue
		}

		cli.Printf(serviceMessages, "project_loaded", projectDir, len(proj.Modules), proj.Agents.Default, proj.Execution.SleepIntervalSeconds)
		return proj
	}
}
