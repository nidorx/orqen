package mcp

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/nidorx/orqen/pkg/chat/memory"
	"github.com/nidorx/orqen/pkg/engine"
)

func newTestStore(t *testing.T) *memory.ChatStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "chat.db")
	s, err := memory.NewChatStore(dbPath)
	if err != nil {
		t.Fatalf("NewChatStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// newTestChatEnv creates a ChatStore and SessionManager backed by a temp dir.
func newTestChatEnv(t *testing.T) (*memory.ChatStore, *memory.SessionManager) {
	t.Helper()
	store := newTestStore(t)
	mgr := memory.NewSessionManager(store, 24*time.Hour)
	return store, mgr
}

// newTestProject creates a minimal engine.Project with one module and one lane.
func newTestProject(t *testing.T) *engine.Project {
	t.Helper()
	dir := t.TempDir()

	moduleDir := filepath.Join(dir, "tasks")
	backlogDir := filepath.Join(moduleDir, "backlog")
	os.MkdirAll(backlogDir, 0755)

	lane := &engine.Lane{
		Name:   "backlog",
		DirAbs: backlogDir,
	}
	mod := &engine.Module{
		Name:   "tasks",
		DirAbs: moduleDir,
		Lanes:  []*engine.Lane{lane},
	}
	lane.Module = mod

	return &engine.Project{
		Id:      "test-project",
		DirAbs:  dir,
		Modules: []*engine.Module{mod},
	}
}

// nilReq returns a nil *mcp.CallToolRequest since handlers don't use it for test purposes.
func nilReq() *mcp.CallToolRequest {
	return nil
}

// setupTestProjectWithLane creates a minimal project with one module and one lane.
func setupTestProjectWithLane(t *testing.T, dir string) *engine.Project {
	t.Helper()

	moduleDir := filepath.Join(dir, "tasks")
	backlogDir := filepath.Join(moduleDir, "backlog")
	os.MkdirAll(backlogDir, 0755)

	lane := &engine.Lane{
		Name:   "backlog",
		DirAbs: backlogDir,
	}
	mod := &engine.Module{
		Name:   "tasks",
		DirAbs: moduleDir,
		Lanes:  []*engine.Lane{lane},
	}
	lane.Module = mod

	return &engine.Project{
		Id:      "test",
		DirAbs:  dir,
		Modules: []*engine.Module{mod},
	}
}
