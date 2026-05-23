package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// containerRuntimes lists Docker-compatible runtimes tried in priority order.
var containerRuntimes = []string{"docker", "podman", "nerdctl"}

// detectRuntime returns the first available container runtime or errors out.
func detectRuntime() (string, error) {
	for _, rt := range containerRuntimes {
		if _, err := exec.LookPath(rt); err == nil {
			return rt, nil
		}
	}
	return "", fmt.Errorf("no container runtime found (tried: %s)", strings.Join(containerRuntimes, ", "))
}

// RunContainer executes a command inside a Docker-compatible container.
// It auto-detects docker vs podman on first call.
func RunContainer(image string, xtras []string, env map[string]string, args ...string) error {

	// Find the project root (where go.mod lives).
	// Works whether invoked from do/ (via `go run .`) or from the project root.
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}

	distDir := filepath.Join(projectRoot, ".dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		return fmt.Errorf("create .dist: %w", err)
	}

	rt, err := detectRuntime()
	if err != nil {
		return err
	}

	goCACHE := Must2(TempDir("cache-go-build"))
	goMODCACHE := Must2(TempDir("cache-go-mod"))

	baseArgs := []string{
		"run", "--rm",
		"-v", projectRoot + ":/src",
		"-v", distDir + ":/dist",
		"-v", goCACHE + ":/.cache/go-build", "-e", "GOCACHE=/.cache/go-build",
		"-v", goMODCACHE + ":/.cache/mod", "-e", "GOMODCACHE=/.cache/mod",
		"-w", "/src",
	}
	baseArgs = append(baseArgs, xtras...)

	for k, v := range env {
		baseArgs = append(baseArgs, "-e", k+"="+v)
	}
	baseArgs = append(baseArgs, image)

	cmd := exec.Command(rt, append(baseArgs, args...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
