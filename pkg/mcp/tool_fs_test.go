package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nidorx/orqen/pkg/engine"
)

func TestFsCatHandler(t *testing.T) {
	proj := createTestProject(t)

	// Create a test file
	testFile := filepath.Join(proj.DirAbs, "test.txt")
	if err := os.WriteFile(testFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(proj.DirAbs)

	t.Run("read file successfully", func(t *testing.T) {
		input := &FsCatInput{Filepath: "test.txt"}
		_, out, err := FsCatHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if out.Content != "hello world" {
			t.Errorf("expected 'hello world', got %q", out.Content)
		}
	})

	t.Run("file not exist", func(t *testing.T) {
		input := &FsCatInput{Filepath: "nonexistent.txt"}
		_, out, err := FsCatHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if out.Error == "" {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("directory not file", func(t *testing.T) {
		input := &FsCatInput{Filepath: "."}
		_, out, err := FsCatHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if out.Error == "" {
			t.Error("expected error for directory")
		}
	})

	t.Run("blocked path", func(t *testing.T) {
		input := &FsCatInput{Filepath: ".orqen/config.yaml"}
		_, out, err := FsCatHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if out.Error == "" {
			t.Error("expected error for blocked path")
		}
	})
}

func TestFsCopyHandler(t *testing.T) {
	proj := createTestProject(t)

	// Create a test file
	testFile := filepath.Join(proj.DirAbs, "source.txt")
	if err := os.WriteFile(testFile, []byte("copy me"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(proj.DirAbs)

	t.Run("copy file", func(t *testing.T) {
		input := &FsCopyInput{Src: "source.txt", Dst: "dest.txt"}
		_, out, err := FsCopyHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if !out.Success {
			t.Errorf("expected success, got error: %s", out.Error)
		}

		// Verify file was copied
		data, err := os.ReadFile(filepath.Join(proj.DirAbs, "dest.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "copy me" {
			t.Errorf("expected 'copy me', got %q", string(data))
		}
	})

	t.Run("copy directory", func(t *testing.T) {
		// Create source directory
		srcDir := filepath.Join(proj.DirAbs, "src_dir")
		if err := os.MkdirAll(srcDir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "file.txt"), []byte("test"), 0644); err != nil {
			t.Fatal(err)
		}

		input := &FsCopyInput{Src: "src_dir", Dst: "dest_dir"}
		_, out, err := FsCopyHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if !out.Success {
			t.Errorf("expected success, got error: %s", out.Error)
		}

		// Verify directory was copied
		data, err := os.ReadFile(filepath.Join(proj.DirAbs, "dest_dir", "file.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "test" {
			t.Errorf("expected 'test', got %q", string(data))
		}
	})
}

func TestFsMoveHandler(t *testing.T) {
	proj := createTestProject(t)

	// Create a test file
	testFile := filepath.Join(proj.DirAbs, "move_me.txt")
	if err := os.WriteFile(testFile, []byte("move me"), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(proj.DirAbs)

	t.Run("move file", func(t *testing.T) {
		input := &FsMoveInput{Src: "move_me.txt", Dst: "moved.txt"}
		_, out, err := FsMoveHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if !out.Success {
			t.Errorf("expected success, got error: %s", out.Error)
		}

		// Verify file was moved
		if _, err := os.Stat(filepath.Join(proj.DirAbs, "moved.txt")); os.IsNotExist(err) {
			t.Error("file was not moved")
		}

		// Verify source no longer exists
		if _, err := os.Stat(filepath.Join(proj.DirAbs, "move_me.txt")); !os.IsNotExist(err) {
			t.Error("source file still exists")
		}
	})
}

func TestFsListHandler(t *testing.T) {
	proj := createTestProject(t)

	// Create test files and directories
	os.WriteFile(filepath.Join(proj.DirAbs, "file1.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(proj.DirAbs, "file2.go"), []byte("test"), 0644)
	os.MkdirAll(filepath.Join(proj.DirAbs, "subdir"), 0755)
	defer os.RemoveAll(proj.DirAbs)

	t.Run("list directory", func(t *testing.T) {
		input := &FsListInput{Dir: "."}
		_, out, err := FsListHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Entries) < 3 {
			t.Errorf("expected at least 3 entries, got %d", len(out.Entries))
		}

		// Check that blocked paths are excluded
		for _, entry := range out.Entries {
			if entry.Name == ".orqen" || entry.Name == ".git" {
				t.Errorf("blocked path %s should not be in results", entry.Name)
			}
		}
	})
}

func TestFsTreeHandler(t *testing.T) {
	proj := createTestProject(t)

	// Create test structure
	os.MkdirAll(filepath.Join(proj.DirAbs, "a", "b"), 0755)
	os.WriteFile(filepath.Join(proj.DirAbs, "a", "file.txt"), []byte("test"), 0644)
	defer os.RemoveAll(proj.DirAbs)

	t.Run("tree with default depth", func(t *testing.T) {
		input := &FsTreeInput{Dir: "."}
		_, out, err := FsTreeHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Tree) == 0 {
			t.Error("expected tree output")
		}
	})
}

func TestFsFindHandler(t *testing.T) {
	proj := createTestProject(t)

	// Create test files
	os.WriteFile(filepath.Join(proj.DirAbs, "test.go"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(proj.DirAbs, "main.go"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(proj.DirAbs, "README.md"), []byte("test"), 0644)
	defer os.RemoveAll(proj.DirAbs)

	t.Run("find go files", func(t *testing.T) {
		input := &FsFindInput{Pattern: "*.go"}
		_, out, err := FsFindHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if out.Count != 2 {
			t.Errorf("expected 2 matches, got %d", out.Count)
		}
	})

	t.Run("find with max results", func(t *testing.T) {
		maxResults := 1
		input := &FsFindInput{Pattern: "*.go", MaxResults: &maxResults}
		_, out, err := FsFindHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if out.Count != 1 {
			t.Errorf("expected 1 match with max_results=1, got %d", out.Count)
		}
	})
}

func TestFsGrepHandler(t *testing.T) {
	proj := createTestProject(t)

	// Create test file
	content := "line one\nline TWO\nline three\nLINE FOUR\n"
	os.WriteFile(filepath.Join(proj.DirAbs, "test.txt"), []byte(content), 0644)
	defer os.RemoveAll(proj.DirAbs)

	t.Run("grep case sensitive", func(t *testing.T) {
		input := &FsGrepInput{Pattern: "LINE", Filepath: "test.txt"}
		_, out, err := FsGrepHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if out.Count != 1 {
			t.Errorf("expected 1 match (case sensitive), got %d", out.Count)
		}
	})

	t.Run("grep case insensitive", func(t *testing.T) {
		ignoreCase := true
		input := &FsGrepInput{Pattern: "line", Filepath: "test.txt", IgnoreCase: &ignoreCase}
		_, out, err := FsGrepHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if out.Count != 4 {
			t.Errorf("expected 4 matches (case insensitive), got %d", out.Count)
		}
	})
}

func TestFsDiffHandler(t *testing.T) {
	proj := createTestProject(t)

	// Create test files
	os.WriteFile(filepath.Join(proj.DirAbs, "file1.txt"), []byte("line1\nline2\nline3\n"), 0644)
	os.WriteFile(filepath.Join(proj.DirAbs, "file2.txt"), []byte("line1\nmodified\nline3\n"), 0644)
	defer os.RemoveAll(proj.DirAbs)

	t.Run("diff two files", func(t *testing.T) {
		input := &FsDiffInput{File1: "file1.txt", File2: "file2.txt"}
		_, out, err := FsDiffHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if out.Diff == "" {
			t.Error("expected diff output")
		}
	})

	t.Run("diff identical files", func(t *testing.T) {
		input := &FsDiffInput{File1: "file1.txt", File2: "file1.txt"}
		_, out, err := FsDiffHandler(t.Context(), nil, input, proj)
		if err != nil {
			t.Fatal(err)
		}
		if out.Diff != "" {
			t.Errorf("expected empty diff for identical files, got: %s", out.Diff)
		}
	})
}

func createTestProject(t *testing.T) *engine.Project {
	t.Helper()
	dir := t.TempDir()
	return &engine.Project{
		DirAbs: dir,
	}
}