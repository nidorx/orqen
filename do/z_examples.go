// =============================================================================
// z_examples.go — package examples as .zip files
// =============================================================================
//
// Creates one .zip per example in examples/<lang>/<name>/, containing only the
// .orqen/ configuration directory.  Runtime artefacts (generated/, tasks/,
// conteudo/, .claude, etc.) are excluded so the zip ships a clean template.
//
// Usage:
//   go run ./do -examples                          # Zip all examples
//   go run ./do -examples -name conteudo-medium    # Zip a single example
//
// Output (written to .dist/ at the project root):
//   example-pt-conteudo-medium.zip
//   example-pt-dev-assistido.zip
//   example-pt-dev-autonomo.zip
//   example-pt-portal-autonomo.zip
//
// Excluded from every .orqen/ tree:
//   **/generated/   — runtime-generated prompt files
// =============================================================================

package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// dirsToSkip are directory names excluded from the .orqen/ tree.
var dirsToSkip = map[string]bool{
	"generated": true,
}

// --- shared ------------------------------------------------------------------

// findProjectRoot walks up from the CWD to find the topmost go.mod,
// returning its directory as the project root.  This handles nested modules
// (e.g. do/go.mod vs the root go.mod) by continuing past the first match.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	var lastMatch string
	for {
		if _, err := os.Stat(filepath.Join(abs, "go.mod")); err == nil {
			lastMatch = abs
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		abs = parent
	}

	if lastMatch == "" {
		return "", fmt.Errorf("could not find project root (no go.mod found)")
	}
	return lastMatch, nil
}

// --- entry -------------------------------------------------------------------

func runExamples() error {
	// Find the project root (where go.mod lives).
	// Works whether invoked from do/ (via `go run .`) or from the project root.
	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		return fmt.Errorf("chdir to project root: %w", err)
	}

	var flagSet = flag.NewFlagSet("do examples", flag.ExitOnError)
	var nameFilter string
	flagSet.StringVar(&nameFilter, "name", "", "zip only this example (e.g. conteudo-medium)")

	flagSet.Parse(subArgsFor("examples"))

	distDir := filepath.Join(projectRoot, ".dist")
	if err := os.MkdirAll(distDir, 0o755); err != nil {
		return fmt.Errorf("create .dist: %w", err)
	}

	examplesDir := filepath.Join(projectRoot, "examples")

	examples, err := collectExamples(examplesDir)
	if err != nil {
		return fmt.Errorf("scan examples: %w", err)
	}

	for _, ex := range examples {
		if nameFilter != "" && ex.name != nameFilter {
			continue
		}
		if err := zipExample(ex, examplesDir, distDir); err != nil {
			return fmt.Errorf("zip %s: %w", ex.name, err)
		}
		fmt.Printf("  created .dist/%s\n", ex.zipName())
	}

	return nil
}

// --- example discovery -------------------------------------------------------

type example struct {
	lang string // e.g. "pt"
	name string // e.g. "conteudo-medium"
}

func (e example) zipName() string {
	return fmt.Sprintf("example-%s-%s.zip", e.lang, e.name)
}

// collectExamples walks examples/<lang>/* and returns all discovered examples.
func collectExamples(examplesDir string) ([]example, error) {
	var result []example

	entries, err := os.ReadDir(examplesDir)
	if err != nil {
		return nil, err
	}

	for _, langEntry := range entries {
		if !langEntry.IsDir() {
			continue
		}
		lang := langEntry.Name()
		langPath := filepath.Join(examplesDir, lang)

		subEntries, err := os.ReadDir(langPath)
		if err != nil {
			continue
		}
		for _, sub := range subEntries {
			if !sub.IsDir() {
				continue
			}
			// Only include dirs that have a .orqen/ subdirectory
			orqenPath := filepath.Join(langPath, sub.Name(), ".orqen")
			if _, err := os.Stat(orqenPath); err != nil {
				continue
			}
			result = append(result, example{lang: lang, name: sub.Name()})
		}
	}

	return result, nil
}

// --- zip creation ------------------------------------------------------------

func zipExample(ex example, examplesDir, distDir string) error {
	srcDir := filepath.Join(examplesDir, ex.lang, ex.name)
	outPath := filepath.Join(distDir, ex.zipName())

	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// Only copy .orqen/ tree, excluding dirsToSkip
	orqenDir := filepath.Join(srcDir, ".orqen")
	baseDir := filepath.Dir(orqenDir) // the example root (e.g. examples/pt/conteudo-medium)

	err = filepath.WalkDir(orqenDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories listed in dirsToSkip
		if d.IsDir() && dirsToSkip[d.Name()] {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		// Compute the path relative to the example root so the zip
		// contains ".orqen/..." at the top level.
		relPath, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}

		// Also skip any "generated" component in the relative path
		for _, part := range strings.Split(relPath, string(os.PathSeparator)) {
			if dirsToSkip[part] {
				return nil
			}
		}

		return zipFile(w, path, relPath)
	})

	return err
}

func zipFile(w *zip.Writer, srcPath, zipPath string) error {
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}

	// Use forward slashes for zip paths (cross-platform)
	header.Name = filepath.ToSlash(zipPath)
	header.Method = zip.Deflate

	hw, err := w.CreateHeader(header)
	if err != nil {
		return err
	}

	_, err = io.Copy(hw, f)
	return err
}
