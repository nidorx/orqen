// =============================================================================
// z_next_adr.go — print the next ADR number
// =============================================================================
//
// Scans docs/adr/ for existing ADR files matching the pattern NNNN-*.md,
// finds the highest number, and prints the next one zero-padded to 4 digits.
//
// Usage:
//   go run ./do -next-adr
//
// Output (stdout):
//   0005   (if 0001–0004 already exist)
// =============================================================================

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

var adrRE = regexp.MustCompile(`^(\d{4})-.*\.md$`)

func runNextADR() error {
	flagSet := flag.NewFlagSet("do next-adr", flag.ExitOnError)
	flagSet.Parse(subArgsFor("next-adr"))

	projectRoot, err := findProjectRoot()
	if err != nil {
		return fmt.Errorf("find project root: %w", err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		return fmt.Errorf("chdir to project root: %w", err)
	}

	adrDir := filepath.Join(projectRoot, "docs", "adr")

	entries, err := os.ReadDir(adrDir)
	if err != nil {
		// Directory doesn't exist yet — start at 0001
		if os.IsNotExist(err) {
			fmt.Println("0001")
			return nil
		}
		return fmt.Errorf("read docs/adr: %w", err)
	}

	var maxNum int
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := adrRE.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		if n > maxNum {
			maxNum = n
		}
	}

	next := maxNum + 1
	fmt.Printf("%04d\n", next)
	return nil
}
