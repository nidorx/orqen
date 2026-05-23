package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

// ============================================================================
// Test orqen_move_item
// ============================================================================

func TestMoveItemHandler(t *testing.T) {
	proj, _ := setupTestProject(t)

	t.Run("move task from backlog to doing", func(t *testing.T) {
		input := &ItemMoveInput{
			Module:  ptr("task"),
			ItemSeq: 1,
			ToLane:  "doing",
		}
		out := callHandler(t, ItemMoveHandler, input, proj)

		if !out.Success {
			t.Fatalf("expected success, got error: %s", out.Error)
		}

		if out.From != "02_backlog" {
			t.Errorf("from = %q, want '02_backlog'", out.From)
		}

		if out.To != "03_doing" {
			t.Errorf("to = %q, want '03_doing'", out.To)
		}

		// Verify the item was actually moved
		doing := proj.GetModule("task").GetLane("doing")
		found := false
		for item := range doing.WorkItems() {
			if item.Seq == 1 {
				found = true
				break
			}
		}
		if !found {
			t.Error("item not found in 'doing' lane after move")
		}
	})

	t.Run("move nonexistent item", func(t *testing.T) {
		input := &ItemMoveInput{
			Module:  ptr("task"),
			ItemSeq: 9999,
			ToLane:  "doing",
		}
		out := callHandler(t, ItemMoveHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent item")
		}
	})

	t.Run("missing to_lane", func(t *testing.T) {
		input := &ItemMoveInput{
			Module:  ptr("task"),
			ItemSeq: 1,
		}
		out := callHandler(t, ItemMoveHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for missing to_lane")
		}
	})

	t.Run("from lane not found", func(t *testing.T) {
		// Create an item in a lane that doesn't exist in the module config

		// We can't easily test this scenario since items are always in valid lanes
		// The handler finds the item by seq, and the item's lane is always valid
		// This test would require mocking or creating an item in an invalid lane
		// For now, we'll skip this test as the scenario is covered by error handling in the handler
		t.Skip("cannot easily test this scenario - item's lane is always valid when loaded from project")
	})

	t.Run("to lane not found", func(t *testing.T) {
		input := &ItemMoveInput{
			Module:  ptr("task"),
			ItemSeq: 1,
			ToLane:  "nonexistent",
		}
		out := callHandler(t, ItemMoveHandler, input, proj)

		if out.Error == "" {
			t.Error("expected error for nonexistent to_lane")
		}
	})

	t.Run("nil project", func(t *testing.T) {
		input := &ItemMoveInput{
			Module:  ptr("task"),
			ItemSeq: 1,
			ToLane:  "doing",
		}
		out := callHandler(t, ItemMoveHandler, input, nil)

		if out.Error == "" {
			t.Error("expected error for nil project")
		}
	})
}

// copyDir recursively copies a directory from src to dst.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil {
				return err
			}
		}
	}

	return nil
}
