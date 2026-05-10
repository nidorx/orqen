package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/engine"
	project "github.com/nidorx/orqen/pkg/engine"
)

// ============================================================================
// Test Helpers
// ============================================================================

// setupTestProject creates a temporary project with modules and work items.
func setupTestProject(t *testing.T) (*engine.Project, string) {
	t.Helper()

	tempDir := t.TempDir()

	// Create .orqen directory
	orqenDir := filepath.Join(tempDir, ".orqen")
	if err := os.MkdirAll(orqenDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create orqen.yaml
	config := `
agents:
  default: "qwen"
  clients:
    qwen:
      command: ["qwen", "--yolo", "--acp"]

execution:
  max_agents: 10
  sleep_interval_seconds: 60

modules:
  - name: task
    dir: "tasks"
    order: ["doing", "ready", "inbox"]
    lanes:
      - name: "inbox"
        purpose: "User ideas"
      - name: "backlog"
        purpose: "New tasks"
      - name: "doing"
        purpose: "Being implemented"
      - name: "done"
        purpose: "Completed"

  - name: adr
    dir: "docs/adr"
    lanes:
      - name: "draft"
        purpose: "Draft ADRs"
      - name: "accepted"
        purpose: "Accepted ADRs"
`
	if err := os.WriteFile(filepath.Join(orqenDir, "orqen.yaml"), []byte(config), 0644); err != nil {
		t.Fatal(err)
	}

	// Create some test items
	proj, err := project.Load(tempDir)
	if err != nil {
		t.Fatalf("Failed to load project: %v", err)
	}

	// Create task items
	taskModule := proj.GetModule("task")
	if taskModule != nil {
		backlog := taskModule.GetLane("backlog")
		if backlog != nil {
			// TASK-0001-write-tests
			itemDir := filepath.Join(backlog.DirAbs, "TASK-0001-write-tests")
			os.MkdirAll(itemDir, 0755)
			os.WriteFile(filepath.Join(itemDir, "TASK-0001.yaml"), []byte("title: Write Tests\n"), 0644)

			// TASK-0002-add-feature
			itemDir2 := filepath.Join(backlog.DirAbs, "TASK-0002-add-feature")
			os.MkdirAll(itemDir2, 0755)
			os.WriteFile(filepath.Join(itemDir2, "TASK-0002.yaml"), []byte("title: Add Feature\n"), 0644)
		}

		doing := taskModule.GetLane("doing")
		if doing != nil {
			itemDir := filepath.Join(doing.DirAbs, "TASK-0003-refactor")
			os.MkdirAll(itemDir, 0755)
			os.WriteFile(filepath.Join(itemDir, "TASK-0003.yaml"), []byte("title: Refactor\n"), 0644)
		}
	}

	// Create ADR items
	adrModule := proj.GetModule("adr")
	if adrModule != nil {
		draft := adrModule.GetLane("draft")
		if draft != nil {
			itemDir := filepath.Join(draft.DirAbs, "ADR-0001-use-go")
			os.MkdirAll(itemDir, 0755)
			os.WriteFile(filepath.Join(itemDir, "ADR-0001.md"), []byte("---\ntitle: Use Go\nstatus: draft\n---\n"), 0644)
		}

		accepted := adrModule.GetLane("accepted")
		if accepted != nil {
			itemDir := filepath.Join(accepted.DirAbs, "ADR-0002-use-postgres")
			os.MkdirAll(itemDir, 0755)
			os.WriteFile(filepath.Join(itemDir, "ADR-0002.md"), []byte("---\ntitle: Use PostgreSQL\nstatus: accepted\nauthor: nidorx\n---\n"), 0644)
		}
	}

	return proj, tempDir
}

// callHandler is a helper to invoke a tool handler and get the output.
func callHandler[In, Out any](t *testing.T, handler func(context.Context, *mcp.CallToolRequest, In, *engine.Project) (*mcp.CallToolResult, Out, error), input In, proj *engine.Project) Out {
	t.Helper()
	_, out, err := handler(context.Background(), nil, input, proj)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	return out
}

// ptr returns a pointer to a value.
func ptr[T any](v T) *T {
	return &v
}
