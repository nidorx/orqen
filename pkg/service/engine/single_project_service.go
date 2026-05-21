package engine

import (
	"bufio"
	"os"
	"strconv"
	"strings"

	"github.com/coder/acp-go-sdk"
	"github.com/nidorx/orqen/pkg/agent"
	"github.com/nidorx/orqen/pkg/chat"
	"github.com/nidorx/orqen/pkg/cli"
	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/engine"
	project "github.com/nidorx/orqen/pkg/engine"
)

var messages = cli.Messages{
	"pt-BR": {
		"ask_cwd":         "O projeto que vamos trabalhar agora está neste diretório?\nCaminho: %s\nSe estiver correto, responda com Y ou pressione Enter. Caso contrário, responda com N.",
		"ask_project_dir": "Certo. Me informe o caminho do diretório do projeto para que eu possa continuar.",
		"project_loaded": `Pronto. Carreguei o projeto a partir de %s. 
			- Encontrei %d módulos e defini %s como agente padrão. 
			- Intervalo de execução: %ds.
			- O serviço de integração MCP está disponível em http://127.0.0.1:%d/mcp/http/%s`,
		"error_loading":       "Não consegui carregar o projeto: %v\nRevise o arquivo de configuração e tente novamente.",
		"error_empty_dir":     "O caminho do diretório não pode estar vazio.\nMe informe um caminho válido para continuar.",
		"error_reading_input": "Não consegui ler sua resposta: %v\nTente novamente.",
		"starting_engine":     "Perfeito. Vou iniciar a execução agora. Se quiser interromper, é só fechar a janela ou usar Ctrl+C.",
	},
	"en": {
		"ask_cwd":         "Is this the project we're working on?\nPath: %s\nIf it looks correct, reply with Y or press Enter. Otherwise, reply with N.",
		"ask_project_dir": "Alright. Please share the project directory path so I can continue.",
		"project_loaded": `Done. I loaded the project from %s. 
			- Found %d modules and set %s as the default agent. 
			- Execution interval: %ds.
			- The MCP integration service is available at http://127.0.0.1:%d/mcp/http/%s`,
		"error_loading":       "I couldn't load the project: %v\nReview the configuration file and try again.",
		"error_empty_dir":     "Directory path cannot be empty.\nPlease provide a valid path so I can continue.",
		"error_reading_input": "I couldn't read your input: %v\nPlease try again.",
		"starting_engine":     "Alright. I'll start the execution now. If you need to stop, just close the window or use Ctrl+C.",
	},
}

type Service struct {
	proj *engine.Project
}

func (s *Service) Name() string {
	return "SingleProjectService"
}

func (s *Service) OnStart() error {

	orqenExec := os.Args[0]
	orqenPort := conf.GetHttpServer().Port

	// Prompt user for proj directory and load configuration
	proj := loadProject()
	proj.WithInvoker(func(prompt string, item *engine.WorkItem) error {
		cwd := proj.DirAbs
		lane := item.Lane

		// @TODO: sent context item.Files

		// initialize agent (ACP)
		return agent.Exec(
			proj.Id,
			proj.Agents.GetName(lane.Agent),
			lane.Name,
			item.Name,
			cwd,
			prompt,
			proj.Agents.GetCommand(lane.Agent),
			[]acp.McpServer{
				{
					// disponibiliza acesso ao mcp do próprio orqen
					Stdio: &acp.McpServerStdio{
						Name:    "orqen",
						Command: orqenExec,
						Args: []string{
							"--mcp",
							"--port=" + strconv.Itoa(orqenPort),
							"--project=" + proj.Id,
							"--workitem=" + item.ID,
						},
						Env: make([]acp.EnvVariable, 0),
					},
				},
			})
	})
	proj.Start()

	cli.Printf(messages, "starting_engine")

	return nil
}

func (s *Service) OnStop() error {
	return nil
}

func New() *Service {
	return &Service{}
}

// GetProject returns the loaded project instance.
func (s *Service) GetProject() *engine.Project {
	return s.proj
}

// loadProject prompts the user for a project directory path, validates it,
// and loads the project configuration. Returns the validated project directory path.
func loadProject() *engine.Project {
	reader := bufio.NewReader(os.Stdin)

	orqenPort := conf.GetHttpServer().Port

	// Ask once if user wants to use the current directory
	cwd, err := os.Getwd()
	if err == nil {
		if err := project.ValidateDir(cwd); err == nil {
			cli.Printf(messages, "ask_cwd", cwd)
			answer, readErr := reader.ReadString('\n')
			if readErr == nil {
				answer = strings.ToUpper(strings.TrimSpace(answer))
				if answer == "" || strings.EqualFold(answer, "Y") || strings.EqualFold(answer, "YES") {
					// do nothing
				} else {
					cwd = ""
				}
			}
		}
	}

	for {
		projectDir := strings.TrimSpace(cwd)
		if projectDir != "" {
			// validate and load configuration
			if proj, err := project.Load(projectDir); err == nil {
				cli.Printf(
					messages,
					"project_loaded",
					projectDir,
					len(proj.Modules),
					proj.Agents.Default,
					proj.Execution.SleepIntervalSeconds,
					orqenPort,
					proj.Id,
				)

				// 3. ChatService — needs project loaded and HTTP port known
				chatSrv := chat.New(proj)
				if chatSrv != nil {
					if err := chatSrv.OnStart(); err != nil {
						panic(err)
					}
				}
				return proj
			} else {
				cli.Printf(messages, "error_loading", err)
			}
		}

		cli.Printf(messages, "ask_project_dir")

		cwd, err = reader.ReadString('\n')
		if err != nil {
			cli.Printf(messages, "error_reading_input", err)
			continue
		}

		projectDir = strings.TrimSpace(cwd)
		if projectDir == "" {
			cli.Printf(messages, "error_empty_dir")
			continue
		}
	}
}
