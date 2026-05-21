package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nidorx/orqen/pkg/cli"
	"github.com/nidorx/orqen/pkg/conf"
	"github.com/nidorx/orqen/pkg/mcp"
	"github.com/nidorx/orqen/pkg/service"
)

var Version = "v0.0.1"

const banner = `
 ██████╗ ██████╗  ██████╗ ███████╗███╗   ██╗
██╔═══██╗██╔══██╗██╔═══██╗██╔════╝████╗  ██║
██║   ██║██████╔╝██║   ██║█████╗  ██╔██╗ ██║
██║   ██║██╔══██╗██║▄▄ ██║██╔══╝  ██║╚██╗██║
╚██████╔╝██║  ██║╚██████╔╝███████╗██║ ╚████║
 ╚═════╝ ╚═╝  ╚═╝ ╚══▀▀═╝ ╚══════╝╚═╝  ╚═══╝
%s - %s

`

// messages defines all i18n messages used during CLI startup.
// Consumers (main, services) reference keys from this map.
var messages = cli.Messages{
	"pt-BR": {
		"what_is_orqen":   "Oi 🙂 tudo bem com você? Eu sou a Orqen. Vou organizar e executar seus fluxos de trabalho com AI, passo a passo.",
		"starting_engine": "Perfeito. Vou iniciar a execução agora. Se quiser interromper, é só fechar a janela ou usar Ctrl+C.",
		"shutting_down":   "Entendido. Vou encerrar tudo com calma. Quando quiser recomeçar, é só me chamar de novo 😉.",
	},
	"en": {
		"what_is_orqen":   "Hi 🙂 how are you doing? I'm Orqen. I'll organize and run your workflows with AI, step by step.",
		"starting_engine": "Alright. I'll start the execution now. If you need to stop, just close the window or use Ctrl+C.",
		"shutting_down":   "Got it. I'll wrap everything up nicely. When you're ready to start again, just call me back 😉.",
	},
}

func main() {

	conf.SetInfo(conf.Info{
		Version: Version,
		Website: "https://github.com/nidorx/orqen", // https://orqen.ai.br
	})

	isMCP := flag.Bool("mcp", false, "Run as a MCP Stdio")
	mcpPort := flag.String("port", "8080", "Orqen port (MCP Stdio)")
	mcpProjectId := flag.String("project", "", "Orqen project id (MCP Stdio)")
	mcpWorkItemID := flag.String("workitem", "", "Current Work Item ID (MCP Stdio)")
	flag.Parse()

	if *isMCP {
		// mcp.DEBUG_STDIO = true
		mcp.StartStdio(*mcpPort, *mcpProjectId, *mcpWorkItemID)
		return
	}

	if len(os.Args) > 1 {
		panic("invalid args")
	}

	fmt.Printf("\033[36m"+banner+"\033[0m", conf.GetInfo().Version, conf.GetInfo().Website)
	time.Sleep(1000 * time.Millisecond)
	cli.Printf(messages, "what_is_orqen")
	time.Sleep(500 * time.Millisecond)

	orqenPort, err := getOrqenPort()
	if err != nil {
		panic(err)
	}

	conf.SetHttpServer(conf.HttpServer{
		IP:           "0.0.0.0",
		Port:         orqenPort,
		ReadTimeout:  0,
		WriteTimeout: 0,
	})

	// start services
	var wg sync.WaitGroup

	service.Start()

	// graceful stop services
	wg.Go(func() {
		quit := make(chan os.Signal, 1)
		// kill (no param) default send syscall.SIGTERM
		// kill -2 is syscall.SIGINT
		// kill -9 is syscall.SIGKILL but can't be catch, so don't need add it
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		cli.Printf(messages, "shutting_down")

		service.Stop()
		time.Sleep(500 * time.Millisecond)
	})
	wg.Wait()
}

func getOrqenPort() (int, error) {
	candidates := []int{
		6180, // main
		6181,
		6182,
		6318,
		7420,
		9094,
	}

	for _, port := range candidates {
		if isPortFree(port) {
			return port, nil
		}
	}

	// fallback
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("failed to allocate random port: %w", err)
	}
	defer l.Close()

	addr := l.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

func isPortFree(port int) bool {
	l, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}
