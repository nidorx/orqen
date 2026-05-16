package markdown

import (
	"bytes"
	"testing"
)

// renderTelegramMarkdown converts markdown input to Telegram markdown output.
func renderTM(input string) (string, error) {
	md := New()
	var buf bytes.Buffer
	if err := md.Convert([]byte(input), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// TestEscaping_Backslash verifies that backslashes in code are escaped as \\
func TestEscaping_Backslash(t *testing.T) {
	// Windows path with backslashes
	input := "Path: `D:\\dev\\projetos\\orqen`"
	output, err := renderTM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inside inline code, backslashes must be escaped as \\
	// Input has 3 backslashes, output should have 6 (each \ becomes \\)
	if !contains(output, `\\`) {
		t.Errorf("backslashes in code not escaped, got:\n%s", output)
	}
	t.Logf("Output: %s", output)
}

// TestEscaping_BackslashInPlainText verifies backslash in plain text is escaped
func TestEscaping_BackslashInPlainText(t *testing.T) {
	input := `See path D:\dev\projects`
	output, err := renderTM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// In plain text, backslash must be escaped
	if !contains(output, `\\`) {
		t.Errorf("backslash in plain text not escaped, got:\n%s", output)
	}
	t.Logf("Output: %s", output)
}

// TestEscaping_CodeBlockBackslash verifies backslashes in code blocks are escaped
func TestEscaping_CodeBlockBackslash(t *testing.T) {
	input := "```python\npath = 'C:\\Users\\test'\n```"
	output, err := renderTM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inside code blocks, backslashes must be escaped
	if !contains(output, `\\`) {
		t.Errorf("backslashes in code block not escaped, got:\n%s", output)
	}
	t.Logf("Output: %s", output)
}

// TestEscaping_CodeBlockBacktick verifies backticks in code blocks are escaped
func TestEscaping_CodeBlockBacktick(t *testing.T) {
	input := "```\ncode with `backticks` inside\n```"
	output, err := renderTM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inside code blocks, backticks must be escaped
	if !contains(output, "\\`") {
		t.Errorf("backticks in code block not escaped, got:\n%s", output)
	}
	t.Logf("Output: %s", output)
}

// TestEscaping_LinkParentheses verifies parentheses in URLs are escaped
func TestEscaping_LinkParentheses(t *testing.T) {
	// URL with parentheses — goldmark parses link destination up to first unescaped )
	// So we test with ) in the middle (escaped in markdown source)
	input := "[test](http://example.com/path\\(file\\))"
	output, err := renderTM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inside link URL, parentheses must be escaped
	if !contains(output, "\\(") || !contains(output, "\\)") {
		t.Errorf("parentheses in link URL not escaped, got:\n%s", output)
	}
	t.Logf("Output: %s", output)
}

// TestEscaping_AllSpecialChars verifies all special chars from Telegram spec are escaped
func TestEscaping_AllSpecialChars(t *testing.T) {
	// Characters that must be escaped in Telegram MarkdownV2 (outside code/pre/link):
	// _, *, [, ], (, ), ~, `, >, #, +, -, =, |, {, }, ., !
	// We wrap them in bold or use contexts where they won't be parsed as markdown syntax
	testCases := []struct {
		name    string
		input   string
		mustEsc string
	}{
		{"underscore", "text \\_hello\\_", "\\_"},
		{"open_bracket", "text [open", "\\["},
		{"close_bracket", "text close]", "\\]"},
		{"open_paren", "text (open", "\\("},
		{"close_paren", "text close)", "\\)"},
		{"tilde", "text ~test", "\\~"},
		{"backtick", "text `code", "\\`"},
		{"hash", "text #heading", "\\#"},
		{"plus", "text +plus", "\\+"},
		{"minus", "text -minus", "\\-"},
		{"equal", "text =equal", "\\="},
		{"pipe", "text |pipe", "\\|"},
		{"open_brace", "text {brace", "\\{"},
		{"close_brace", "text brace}", "\\}"},
		{"dot", "test.txt", "\\."},
		{"exclamation", "test!", "\\!"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := renderTM(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !contains(output, tc.mustEsc) {
				t.Errorf("expected escaped char %q in output, got:\n%s", tc.mustEsc, output)
			}
		})
	}
}

// TestEscaping_BackslashMustBeEscaped verifies the fundamental rule:
// "This implies that '\' character usually must be escaped with a preceding '\' character."
func TestEscaping_BackslashMustBeEscaped(t *testing.T) {
	input := `path\to\file`
	output, err := renderTM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Every backslash in the output must be doubled
	bsCount := count(output, '\\')
	if bsCount%2 != 0 {
		t.Errorf("unescaped backslash found (odd count=%d), got:\n%s", bsCount, output)
	}
	t.Logf("Output: %s", output)
}

// TestEscaping_RealWorldProjectInfo tests the user's exact scenario
func TestEscaping_RealWorldProjectInfo(t *testing.T) {
	input := `## Project: t1NPxcRJNVI

- **Directory:** ` + "`D:\\dev\\projetos\\orqen`" + `
- **Modules:** ` + "`3`" + `
- **Lanes:** ` + "`20`" + `
- **Workitems:** ` + "`0`" + `
- **Active agents:** ` + "`0`" + ``

	output, err := renderTM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify backslashes in the code span are escaped
	if !contains(output, `D:\\`) {
		t.Errorf("backslashes not escaped in code span, got:\n%s", output)
	}

	// Verify balanced asterisks
	asteriskCount := count(output, '*')
	if asteriskCount%2 != 0 {
		t.Errorf("unbalanced asterisks: count=%d, got:\n%s", asteriskCount, output)
	}

	t.Logf("Output:\n%s", output)
}

// TestEscaping_CodeSpan verifies inline code escaping
func TestEscaping_CodeSpan(t *testing.T) {
	input := "`code with \\ backslash`"
	output, err := renderTM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inside inline code, backslashes must be escaped
	if !contains(output, `\\`) {
		t.Errorf("backslash in code span not escaped, got:\n%s", output)
	}
	t.Logf("Output: %s", output)
}

// TestEscaping_EmphasizedTextWithSpecialChars verifies special chars in bold/italic are escaped
func TestEscaping_EmphasizedTextWithSpecialChars(t *testing.T) {
	// Bold text containing special chars
	input := "**hello-world (test)**"
	output, err := renderTM(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Dashes and parens inside bold should be escaped
	if !contains(output, "\\-") || !contains(output, "\\(") || !contains(output, "\\)") {
		t.Errorf("special chars in bold not escaped, got:\n%s", output)
	}
	t.Logf("Output: %s", output)
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func count(s string, c byte) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			n++
		}
	}
	return n
}
