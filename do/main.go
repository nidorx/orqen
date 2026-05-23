package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	var (
		flgLint     bool
		flgPing     bool
		flgBuild    bool
		flgExamples bool
	)

	// Use the default flag set but parse manually so unknown flags
	// (meant for sub-commands) are not rejected.
	topFlags := flag.NewFlagSet("do", flag.ContinueOnError)
	topFlags.BoolVar(&flgPing, "ping", false, "ping test")
	topFlags.BoolVar(&flgLint, "lint", false, "execute golangci-lint")
	topFlags.BoolVar(&flgBuild, "build", false, "build binaries via container runtimes")
	topFlags.BoolVar(&flgExamples, "examples", false, "package examples as .zip files")
	topFlags.SetOutput(nil) // suppress errors; sub-commands handle their own

	// Parse only the flags we know; skip unknown ones so sub-commands get them.
	args := os.Args[1:]
	for _, a := range args {
		switch a {
		case "-ping":
			flgPing = true
		case "-lint":
			flgLint = true
		case "-build":
			flgBuild = true
		case "-examples":
			flgExamples = true
		}
	}

	if flgPing {
		cmd := exec.Command("ping", "google.com")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		Must(cmd.Run())
		return
	}

	if flgLint {
		runGolangCILint()
		return
	}

	if flgBuild {
		if err := runBuild(); err != nil {
			fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if flgExamples {
		if err := runExamples(); err != nil {
			fmt.Fprintf(os.Stderr, "examples failed: %v\n", err)
			os.Exit(1)
		}
		return
	}

	_ = topFlags
	// no flag matched — print usage
	topFlags.Usage()
}

// subArgsFor returns command-line args after the "-<cmd>" flag.
func subArgsFor(cmd string) []string {
	flag := "-" + cmd
	for i, a := range os.Args[1:] {
		if a == flag {
			return os.Args[i+2:]
		}
	}
	return nil
}
