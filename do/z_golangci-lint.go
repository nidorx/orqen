package main

func runGolangCILint() {
	lintCACHE := Must2(TempDir("cache-golangci-lint"))
	Must(RunContainer(
		"golangci/golangci-lint:v2.12.2",
		[]string{
			"-v", lintCACHE + ":/.cache/golangci-lint", "-e", "GOLANGCI_LINT_CACHE=/.cache/golangci-lint",
		},
		map[string]string{},
		"golangci-lint", "run",
	))
}
