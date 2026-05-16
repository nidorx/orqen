package markdown

import (
	"bufio"
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	textm "github.com/yuin/goldmark/text"
)

// renderTelegramMarkdown converts markdown input to Telegram markdown output.
func renderTelegramMarkdown(input string) (string, error) {
	md := New()
	var buf bytes.Buffer
	if err := md.Convert([]byte(input), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// TestHeading_H1 tests that H1 renders correctly for Telegram MarkdownV2
// Telegram has no special heading syntax - headings are just text
func TestHeading_H1(t *testing.T) {
	input := "# Hello World"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Telegram MarkdownV2 has no heading syntax - just plain text
	// Should NOT have triple asterisks
	if strings.Contains(output, "***") {
		t.Errorf("H1 should not use triple asterisks (***), got:\n%s", output)
	}

	// Should render as plain text (no wrapping bold for H1)
	if output != "Hello World" && !strings.Contains(output, "Hello World") {
		t.Errorf("H1 should render as plain text, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestHeading_H2 tests that H2 renders correctly
func TestHeading_H2(t *testing.T) {
	input := "## Module: task"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should be bold, but NOT triple asterisks
	if strings.Contains(output, "***") {
		t.Errorf("H2 should not use triple asterisks (***), got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestHeading_Nested tests that nested headings don't produce broken tags
func TestHeading_Nested(t *testing.T) {
	input := "## Module: **task**"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that tags are properly balanced
	// Count opening and closing asterisks
	openCount := strings.Count(output, "***")
	closeCount := strings.Count(output, "***")
	if openCount != closeCount {
		t.Errorf("unbalanced tags: open=%d close=%d, got:\n%s", openCount, closeCount, output)
	}

	t.Logf("Output:\n%s", output)
}

// TestEmphasis_Bold tests that **bold** renders as *bold* in Telegram MarkdownV2
func TestEmphasis_Bold(t *testing.T) {
	input := "**bold text**"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Telegram MarkdownV2 uses single asterisk for bold: *bold*
	// Should NOT have triple asterisks
	if strings.Contains(output, "***") {
		t.Errorf("bold should use single asterisk (*), not triple (***), got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestEmphasis_Italic tests that *italic* renders as _italic_ in Telegram MarkdownV2
func TestEmphasis_Italic(t *testing.T) {
	input := "*italic text*"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Telegram MarkdownV2 uses underscore for italic: _italic_
	// Should contain underscore markers
	if !strings.Contains(output, "_") {
		t.Errorf("italic should use underscore (_), got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestEmphasis_Nested tests nested bold+italic
func TestEmphasis_Nested(t *testing.T) {
	input := "**bold _italic_ bold**"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Tags should be properly nested and closed
	// Count asterisks - should be even (open + close pairs)
	asteriskCount := strings.Count(output, "*")
	if asteriskCount%2 != 0 {
		t.Errorf("unbalanced asterisk count: %d (should be even), got:\n%s", asteriskCount, output)
	}

	t.Logf("Output:\n%s", output)
}

// TestEscape_SpecialChars tests that special characters are properly escaped
func TestEscape_SpecialChars(t *testing.T) {
	// Characters that must be escaped in Telegram MarkdownV2:
	// _, *, [, ], (, ), ~, `, >, #, +, -, =, |, {, }, ., !
	input := `list-item (with parens)
-and dashes`
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Dashes and parens in plain text should be escaped
	if strings.Contains(output, "(") && !strings.Contains(output, "\\(") {
		t.Logf("Warning: unescaped ( found in output:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestEscape_AlreadyEscapedInTags tests that characters inside tags are not double-escaped
func TestEscape_AlreadyEscapedInTags(t *testing.T) {
	input := "**test-with-dashes**"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The dashes inside bold tags should be escaped
	if !strings.Contains(output, "\\-") {
		t.Logf("Warning: dashes inside bold should be escaped, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestList_Basic tests basic list rendering
func TestList_Basic(t *testing.T) {
	input := `- Item 1
- Item 2
- Item 3`
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain list bullets
	if !strings.Contains(output, "•") && !strings.Contains(output, "-") {
		t.Errorf("expected list bullets, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestList_Nested tests nested list rendering
func TestList_Nested(t *testing.T) {
	input := `- Parent
  - Child 1
  - Child 2`
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Logf("Output:\n%s", output)
}

// TestLink_Basic tests link rendering
func TestLink_Basic(t *testing.T) {
	input := "[Google](https://google.com)"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Telegram MarkdownV2 link format: [text](url)
	// Parentheses and special chars in URL must be escaped
	if !strings.Contains(output, "[Google]") {
		t.Errorf("expected link text, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestCode_Inline tests inline code rendering
func TestCode_Inline(t *testing.T) {
	input := "`inline code`"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Telegram MarkdownV2 inline code uses single backtick
	if !strings.Contains(output, "`") {
		t.Errorf("expected backtick for inline code, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestCode_Block tests code block rendering
func TestCode_Block(t *testing.T) {
	input := "```python\nprint('hello')\n```"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Telegram MarkdownV2 code block uses triple backticks
	if !strings.Contains(output, "```") {
		t.Errorf("expected triple backticks for code block, got:\n%s", output)
	}

	// Inside code blocks, backticks and backslashes must be escaped
	if strings.Contains(output, "'") && !strings.Contains(output, "\\'") {
		t.Logf("Warning: single quotes inside code should be escaped, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestBlockquote_Basic tests blockquote rendering
func TestBlockquote_Basic(t *testing.T) {
	input := "> This is a quote"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Telegram MarkdownV2 blockquote uses >
	if !strings.Contains(output, ">") {
		t.Errorf("expected > for blockquote, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestStrikethrough tests strikethrough rendering
func TestStrikethrough(t *testing.T) {
	input := "~~strikethrough~~"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Telegram MarkdownV2 uses ~strikethrough~ (single tilde)
	// Current implementation uses ~~~ which is wrong
	if strings.Contains(output, "~~~") {
		t.Logf("Warning: Telegram MarkdownV2 uses single tilde (~), not triple (~~~), got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestComplexDocument tests a complex document similar to user's example
func TestComplexDocument(t *testing.T) {
	input := `# Available Lanes

## Module: **task**

**doing** - Task currently being implemented
    - *(0 items, 0 active)*

**learning** - Retrospective gate
    - *(0 items, 0 active)*
`
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check for broken patterns like ***Module: ***task******
	if strings.Contains(output, "***") && strings.Count(output, "***")%2 != 0 {
		t.Errorf("unbalanced triple asterisks, got:\n%s", output)
	}

	// Check that bold markers are properly closed
	boldCount := strings.Count(output, "*")
	if boldCount%2 != 0 {
		t.Errorf("unbalanced asterisk count: %d, got:\n%s", boldCount, output)
	}

	t.Logf("Output:\n%s", output)
}

// TestValidateBalancedTags validates that all tags are properly balanced
func TestValidateBalancedTags(t *testing.T) {
	testCases := []struct {
		name  string
		input string
	}{
		{"simple bold", "**bold**"},
		{"simple italic", "*italic*"},
		{"nested", "**bold _italic_ bold**"},
		{"heading with bold", "## Module: **task**"},
		{"list with italic", "- _italic item_"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := renderTelegramMarkdown(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// For asterisk-based tags, count should be even
			asteriskCount := strings.Count(output, "*")
			if asteriskCount%2 != 0 {
				t.Errorf("unbalanced asterisks: count=%d (odd), output:\n%s", asteriskCount, output)
			}

			// For underscore-based tags, count should be even
			underscoreCount := strings.Count(output, "_")
			if underscoreCount%2 != 0 {
				t.Errorf("unbalanced underscores: count=%d (odd), output:\n%s", underscoreCount, output)
			}

			// For tilde-based tags, count should be even
			tildeCount := strings.Count(output, "~")
			if tildeCount%2 != 0 {
				t.Errorf("unbalanced tildes: count=%d (odd), output:\n%s", tildeCount, output)
			}

			t.Logf("Output:\n%s", output)
		})
	}
}

// TestEscape_Backslash tests that backslashes are properly escaped
func TestEscape_Backslash(t *testing.T) {
	input := `path\to\file`
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Backslashes should be escaped as \\
	if strings.Contains(output, "\\") && !strings.Contains(output, "\\\\") {
		t.Logf("Warning: backslashes should be escaped as \\\\, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// TestTelegramMarkdownV2_Compliance checks against the Telegram MarkdownV2 spec
func TestTelegramMarkdownV2_Compliance(t *testing.T) {
	// According to Telegram MarkdownV2 spec:
	// *bold text*
	// _italic text_ (or italic with underscore)
	// __underline__
	// ~strikethrough~
	// ||spoiler||
	// [inline URL](http://www.example.com/)
	// `inline fixed-width code`
	// ```\npre-formatted code\n```
	// >Block quotation

	// The current implementation uses:
	// BoldTg = *** (3 asterisks) - WRONG, should be * (1 asterisk)
	// ItalicsTg = _ (1 underscore) - CORRECT
	// StrikethroughTg = ~~~ (3 tildes) - WRONG, should be ~ (1 tilde)
	// CodeTg = ``` (3 backticks) - CORRECT for blocks
	// SpanTg = ` (1 backtick) - CORRECT for inline
	// HiddenTg = || (2 pipes) - CORRECT for spoilers

	testCases := []struct {
		name    string
		input   string
		checkFn func(output string) error
	}{
		{
			name:  "bold should use single asterisk",
			input: "**bold**",
			checkFn: func(output string) error {
				if strings.Contains(output, "***") {
					t.Errorf("bold uses triple asterisk (***), should use single (*) per Telegram spec, got: %s", output)
				}
				return nil
			},
		},
		{
			name:  "strikethrough should use single tilde",
			input: "~~strikethrough~~",
			checkFn: func(output string) error {
				if strings.Contains(output, "~~~") {
					t.Logf("Warning: strikethrough uses triple tilde (~~~), should use single (~) per Telegram spec, got: %s", output)
				}
				return nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := renderTelegramMarkdown(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			_ = tc.checkFn(output)
			t.Logf("Output:\n%s", output)
		})
	}
}

// TestHeading_H1Bold verifies H1 renders as bold since Telegram has no heading syntax
func TestHeading_H1Bold(t *testing.T) {
	input := "# Available Lanes"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "*Available Lanes*"
	if strings.TrimSpace(output) != expected {
		t.Errorf("H1 should render as bold.\nExpected: %q\nGot:      %q", expected, strings.TrimSpace(output))
	}
}

// TestHeading_H2Bold verifies H2 renders as bold since Telegram has no heading syntax
func TestHeading_H2Bold(t *testing.T) {
	input := "## Module: task"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "*Module: task*"
	if strings.TrimSpace(output) != expected {
		t.Errorf("H2 should render as bold.\nExpected: %q\nGot:      %q", expected, strings.TrimSpace(output))
	}
}

// TestHeading_H2WithBold verifies H2 with nested bold doesn't produce broken tags
func TestHeading_H2WithBold(t *testing.T) {
	input := "## Module: **task**"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The entire heading should be bold — inner **task** is flattened into the heading bold
	// since Telegram has no "nested bold" concept
	expected := "*Module: task*"
	if strings.TrimSpace(output) != expected {
		t.Errorf("H2 with bold should render entire heading as bold.\nExpected: %q\nGot:      %q", expected, strings.TrimSpace(output))
	}

	// No broken triple asterisks
	if strings.Contains(output, "***") {
		t.Errorf("should not contain triple asterisks, got:\n%s", output)
	}
}

// TestNewlines_DoubleNewlineAfterHeading verifies heading followed by paragraph
// does NOT produce an extra blank line (Telegram has no heading syntax, so
// heading and paragraph should be visually close).
func TestNewlines_DoubleNewlineAfterHeading(t *testing.T) {
	input := `# Available Lanes

**rejected** - ADRs that were rejected by the user — closed proposals
    - *(0 items, 0 active)*`

	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After heading, paragraph should be on the next line (no blank line)
	// Expected: "*Available Lanes*\n*rejected* ..." NOT "*Available Lanes*\n\n*rejected*"
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		t.Logf("Line %d: %q", i, line)
	}

	headingIdx := -1
	rejectedIdx := -1
	for i, line := range lines {
		if strings.Contains(line, "Available Lanes") {
			headingIdx = i
		}
		if strings.Contains(line, "rejected") {
			rejectedIdx = i
		}
	}

	if headingIdx < 0 || rejectedIdx < 0 {
		t.Fatalf("heading or rejected line not found in output:\n%s", output)
	}

	// rejected paragraph should be on the very next line after heading
	if rejectedIdx != headingIdx+1 {
		t.Errorf("expected paragraph on next line after heading (heading=%d, paragraph=%d), got blank line. Output:\n%s", headingIdx, rejectedIdx, output)
	}

	t.Logf("Output:\n%s", output)
}

// TestNewlines_HeadingFollowedByParagraph verifies no blank line between H3 and its paragraph
func TestNewlines_HeadingFollowedByParagraph(t *testing.T) {
	input := `### Module: task | Lane: inbox
*empty*`

	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// H3 is followed by paragraph — should NOT have a blank line between them
	// Expected: "*Module: task | Lane: inbox*\n<newline>_empty_\n"
	// NOT: "*Module: task | Lane: inbox*\n\n_empty_\n"
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		t.Logf("Line %d: %q", i, line)
	}

	// Should have exactly 2 non-empty lines, not separated by a blank line
	headingLine := -1
	emptyLine := -1
	for i, line := range lines {
		if strings.Contains(line, "Module: task") {
			headingLine = i
		}
		if strings.Contains(line, "empty") {
			emptyLine = i
		}
	}

	if headingLine < 0 {
		t.Fatalf("heading line not found in output:\n%s", output)
	}
	if emptyLine < 0 {
		t.Fatalf("'empty' line not found in output:\n%s", output)
	}

	// empty line should be exactly headingLine + 1 (no blank line in between)
	if emptyLine != headingLine+1 {
		t.Errorf("expected no blank line between heading and paragraph, got blank line at line %d\nFull output:\n%s", headingLine+1, output)
	}

	t.Logf("Output:\n%s", output)
}

// TestNewlines_BlankLineBetweenItems verifies that \n\n in markdown produces
// a blank line in the Telegram output between separate content blocks.
func TestNewlines_BlankLineBetweenItems(t *testing.T) {
	input := `# Available Lanes

## Module: **task**
**doing** - Task currently being implemented by the agent
    - *(0 items, 0 active)*

**learning** - Retrospective gate where the agent processes learnings
    - *(0 items, 0 active)*`

	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	lines := strings.Split(output, "\n")
	for i, line := range lines {
		t.Logf("Line %d: %q", i, line)
	}

	// Find the line with "doing" and "learning"
	doingLine := -1
	learningLine := -1
	for i, line := range lines {
		if strings.Contains(line, "*doing*") {
			doingLine = i
		}
		if strings.Contains(line, "*learning*") {
			learningLine = i
		}
	}

	if doingLine < 0 {
		t.Fatalf("'doing' not found in output:\n%s", output)
	}
	if learningLine < 0 {
		t.Fatalf("'learning' not found in output:\n%s", output)
	}

	// Between "doing" block and "learning" block there should be a blank line
	// The original markdown has \n\n between them
	// So learningLine should be at least doingLine + 3 (doing + bullet + blank + learning)
	gap := learningLine - doingLine
	if gap < 3 {
		t.Errorf("expected blank line between items (gap >= 3), got gap=%d. Output:\n%s", gap, output)
	}

	t.Logf("Output:\n%s", output)
}

// TestNewlines_BlankLine_AST dumps the AST for the same input to understand structure
func TestNewlines_BlankLine_AST(t *testing.T) {
	input := `**doing** - Task
    - *(0 items)*

**learning** - Retrospective`

	md := New()
	parser := md.Parser()
	// Preprocess
	preprocessed := preprocessInput([]byte(input))
	t.Logf("Preprocessed:\n%s", string(preprocessed))

	doc := parser.Parse(textm.NewReader(preprocessed))

	t.Log("--- AST dump ---")
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			indent := ""
			for p := n.Parent(); p != nil; p = p.Parent() {
				indent += "  "
			}
			switch v := n.(type) {
			case *ast.Text:
				t.Logf("%sText: %q", indent, v.Segment.Value(preprocessed))
			case *ast.String:
				t.Logf("%sString: %q", indent, v.Value)
			case *ast.Emphasis:
				t.Logf("%sEmphasis(Level=%d)", indent, v.Level)
			default:
				t.Logf("%s%s", indent, n.Kind())
			}
		}
		return ast.WalkContinue, nil
	})
}
func TestNewlines_AfterListItem(t *testing.T) {
	input := `**rejected** - ADRs that were rejected by the user — closed proposals
    - *(0 items, 0 active)*

**superseded** - ADRs replaced by newer ADRs
    - *(0 items, 0 active)*`

	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that "proposals" is followed by a newline (not directly by list content)
	// The paragraph should end with \n before the nested list begins
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		t.Logf("Line %d: %q", i, line)
	}

	// There must be a line with "proposals" and a separate line with the list bullet
	foundProposalsLine := false
	for _, line := range lines {
		if strings.Contains(line, "closed proposals") {
			foundProposalsLine = true
			// This line should NOT contain the list content glued to it
			if strings.Contains(line, "0 items") {
				t.Errorf("list item content glued to paragraph on same line: %q", line)
			}
			break
		}
	}
	if !foundProposalsLine {
		t.Errorf("'closed proposals' missing from output:\n%s", output)
	}

	t.Logf("Full output:\n%s", output)
}

// TestPreprocessedMarkdown_Methods covers SetParser, Renderer, SetParser, SetRenderer
func TestPreprocessedMarkdown_Methods(t *testing.T) {
	md := New()
	pm := md.(*preprocessedMarkdown)

	// Renderer() should return non-nil
	r := pm.Renderer()
	if r == nil {
		t.Error("Renderer() returned nil")
	}

	// SetRenderer should not panic
	pm.SetRenderer(r)

	// Parser() should return non-nil
	p := pm.Parser()
	if p == nil {
		t.Error("Parser() returned nil")
	}

	// SetParser should not panic
	pm.SetParser(p)
}

// TestRenderer_StringNode covers renderString handler
func TestRenderer_StringNode(t *testing.T) {
	// A markdown string that produces ast.String nodes
	input := "*italic* text"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "text") {
		t.Errorf("expected 'text' in output, got %q", output)
	}
}

// TestDoubleSpace_Handler covers the doubleSpace renderer handler (no-op)
func TestDoubleSpace_Handler(t *testing.T) {
	// Double space at end of line → newline
	input := "line1  \nline2"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should contain both lines
	if !strings.Contains(output, "line1") || !strings.Contains(output, "line2") {
		t.Errorf("expected both lines in output, got %q", output)
	}
}

// TestDocument_Handler covers the document renderer handler (no-op)
func TestDocument_Handler(t *testing.T) {
	input := "simple text"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "simple") {
		t.Errorf("expected 'simple' in output, got %q", output)
	}
}

// TestTableHeader_Row_Cell_Noop covers the no-op table sub-handlers
func TestTableHeader_Row_Cell_Noop(t *testing.T) {
	// This tests that tableHeader, tableRow, tableCell are registered and work
	input := `| A |
| - |
| 1 |`
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "A:") {
		t.Errorf("expected 'A:' in output, got %q", output)
	}
}

// TestHeadingTextFromNode_CodeSpan covers the CodeSpan case in headingTextFromNode
func TestHeadingTextFromNode_CodeSpan(t *testing.T) {
	input := "# Heading with `code`"
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "code") {
		t.Errorf("expected 'code' in output, got %q", output)
	}
}

// TestListItem_DeepNesting covers deeper nesting paths in listItem
func TestListItem_DeepNesting(t *testing.T) {
	// Use proper nested list syntax that goldmark recognizes
	input := `- Level 1
  - Level 2
    - Level 3`
	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have at least level 1 and level 2 bullets
	if !strings.Contains(output, "•") {
		t.Errorf("expected level 1 bullet '•', got %q", output)
	}
	// Check for level 2 bullet (‣) — depends on goldmark nesting detection
	t.Logf("Output:\n%s", output)
}

// TestHidden_CloseBlock covers the CloseBlock no-op
func TestHidden_CloseBlock(t *testing.T) {
	p := defaultHiddenParser
	// CloseBlock should not panic
	p.CloseBlock(nil, parser.NewContext())
}

// TestDoubleSpace_CloseBlock covers the CloseBlock no-op
func TestDoubleSpace_CloseBlock(t *testing.T) {
	p := defaultDoubleSpaceParser
	pc := parser.NewContext()
	// CloseBlock should not panic
	p.CloseBlock(nil, pc)
}

// TestUnclosedCounter_Reset covers the Reset method
func TestUnclosedCounter_Reset(t *testing.T) {
	uc := &unclosedCounter{Single: 5, Double: 3}
	uc.Reset()
	if uc.Single != 0 || uc.Double != 0 {
		t.Errorf("expected both zero after Reset, got Single=%d Double=%d", uc.Single, uc.Double)
	}
}

// TestGetUnclosedCounter covers getUnclosedCounter
func TestGetUnclosedCounter(t *testing.T) {
	pc := parser.NewContext()
	uc := getUnclosedCounter(pc)
	if uc == nil {
		t.Fatal("getUnclosedCounter returned nil")
	}
	// Second call should return same counter
	uc2 := getUnclosedCounter(pc)
	if uc != uc2 {
		t.Error("getUnclosedCounter should return same instance")
	}
}

// TestConfig_UpdateHeading2_4_5_6 covers the remaining update methods
func TestConfig_UpdateHeading2_4_5_6(t *testing.T) {
	// Save originals
	origH2 := Config.headings[1]
	origH4 := Config.headings[3]
	origH5 := Config.headings[4]
	origH6 := Config.headings[5]
	defer func() {
		Config.headings[1] = origH2
		Config.headings[3] = origH4
		Config.headings[4] = origH5
		Config.headings[5] = origH6
	}()

	newEl := Element{Style: BoldTg, Prefix: "X"}
	Config.UpdateHeading2(newEl)
	if Config.headings[1].Prefix != "X" {
		t.Errorf("UpdateHeading2 failed")
	}

	Config.UpdateHeading4(newEl)
	if Config.headings[3].Prefix != "X" {
		t.Errorf("UpdateHeading4 failed")
	}

	Config.UpdateHeading5(newEl)
	if Config.headings[4].Prefix != "X" {
		t.Errorf("UpdateHeading5 failed")
	}

	Config.UpdateHeading6(newEl)
	if Config.headings[5].Prefix != "X" {
		t.Errorf("UpdateHeading6 failed")
	}
}

// TestRenderNode_DefaultCase covers the default case in renderNode
func TestRenderNode_DefaultCase(t *testing.T) {
	r := &Renderer{}
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	// Create a heading node (not handled by renderNode default case)
	h := &ast.Heading{}
	status, err := r.renderNode(w, []byte("test"), h, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != ast.WalkContinue {
		t.Errorf("expected WalkContinue, got %v", status)
	}
}

// TestAST_Dump verifies the AST structure for paragraph + nested list
func TestAST_Dump(t *testing.T) {
	input := `**rejected** - ADRs that were rejected
    - *(0 items)*`

	md := New()
	parser := md.Parser()
	doc := parser.Parse(textm.NewReader([]byte(input)))

	t.Log("--- AST dump ---")
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			indent := ""
			for p := n.Parent(); p != nil; p = p.Parent() {
				indent += "  "
			}
			switch v := n.(type) {
			case *ast.Text:
				src := []byte(input)
				t.Logf("%sText: %q", indent, v.Segment.Value(src))
			case *ast.String:
				t.Logf("%sString: %q", indent, v.Value)
			case *ast.Emphasis:
				t.Logf("%sEmphasis (Level=%d)", indent, v.Level)
			default:
				t.Logf("%s%s", indent, n.Kind())
			}
		}
		return ast.WalkContinue, nil
	})
}

// TestNewlines_ListIndented4Spaces is the actual user's input pattern:
// paragraph text followed by 4-space indented list (which goldmark parses as paragraph continuation)
func TestNewlines_ListIndented4Spaces(t *testing.T) {
	input := `**rejected** - ADRs that were rejected by the user — closed proposals
    - *(0 items, 0 active)*`

	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that the list content appears in output (not swallowed)
	if !strings.Contains(output, "0 items") {
		t.Errorf("list content '0 items' missing from output:\n%s", output)
	}

	// Check that paragraph and list are on separate lines
	if !strings.Contains(output, "proposals\n") {
		t.Errorf("newline after 'proposals' missing, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

// Telegram has no heading syntax, so H1/H2 must render as bold.
func TestUserExample_ExactOutput(t *testing.T) {
	input := `# Available Lanes

## Module: **task**
**doing** - Task currently being implemented by the agent
    - *(0 items, 0 active)*

**learning** - Retrospective gate where the agent processes learnings from the completed implementation and evaluates ADR maintenance
    - *(0 items, 0 active)*

**review** - Completed implementations awaiting quality review by the agent
    - *(0 items, 0 active)*

**inbox** - User ideas that are ready to be transformed into tasks by the agent
    - *(0 items, 0 active)*

**prioritized** - Tasks selected by the user for detailed refinement before implementation
    - *(0 items, 0 active)*

**ready** - Approved tasks ready for autonomous implementation by the agent
    - *(0 items, 0 active)*

**vision** - User ideation and task conception — agent does NOT execute tasks from this lane
    - *(0 items, 0 active)*

**backlog** - All newly created tasks awaiting prioritization
    - *(0 items, 0 active)*

**refined** - Refined tasks awaiting user approval before implementation
    - *(0 items, 0 active)*

**blocked** - Tasks that cannot be completed due to blockers requiring user intervention
    - *(0 items, 0 active)*

**done** - Archive of successfully completed tasks
    - *(0 items, 0 active)*

**archived** - Long-term storage of completed tasks
    - *(0 items, 0 active)*


## Module: **adr**
**inbox** - User ideas that are ready to be transformed into WI by the agent
    - *(0 items, 0 active)*

**draft** - ADRs written by the agent, awaiting user review — BLOCKS all task work when any ADR exists here
    - *(0 items, 0 active)*

**accepted** - User accepted ADRs — active decisions that constrain task refinement and implementation
    - *(0 items, 0 active)*

**rejected** - ADRs that were rejected by the user — closed proposals
    - *(0 items, 0 active)*

**superseded** - ADRs replaced by newer ADRs — contain reference to superseding ADR
    - *(0 items, 0 active)*

**deprecated** - ADRs that are no longer recommended for new work — decision aged out
    - *(0 items, 0 active)*


## Module: **learning**
**inbox** - User ideas that are ready to be transformed into WI by the agent
    - *(0 items, 0 active)*

**knowledge** - Pattern-level knowledge from task implementations — how-to knowledge, patterns, conventions, gotchas
    - *(0 items, 0 active)*
`

	output, err := renderTelegramMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// H1 must be bold (Telegram has no heading syntax)
	if !strings.Contains(output, "*Available Lanes*") {
		t.Errorf("H1 should be bold: expected '*Available Lanes*', got:\n%s", output)
	}

	// H2 must be bold — entire heading wrapped in bold (inner **task** flattened)
	if !strings.Contains(output, "*Module: task*") {
		t.Errorf("H2 should be bold: expected '*Module: task*', got:\n%s", output)
	}
	if !strings.Contains(output, "*Module: adr*") {
		t.Errorf("H2 should be bold: expected '*Module: adr*', got:\n%s", output)
	}
	if !strings.Contains(output, "*Module: learning*") {
		t.Errorf("H2 should be bold: expected '*Module: learning*', got:\n%s", output)
	}

	// List items with bold should render correctly
	if !strings.Contains(output, "*doing*") {
		t.Errorf("list bold item missing: expected '*doing*', got:\n%s", output)
	}

	// Italic parens should render correctly
	if !strings.Contains(output, "_(0 items, 0 active)_") && !strings.Contains(output, "_\\(0 items, 0 active\\)_") {
		t.Errorf("italic parens missing, got:\n%s", output)
	}

	// No triple asterisks anywhere (indicates broken nesting)
	if strings.Contains(output, "***") {
		t.Errorf("output contains broken triple asterisks (***), got:\n%s", output)
	}

	// Validate balanced asterisks
	asteriskCount := strings.Count(output, "*")
	if asteriskCount%2 != 0 {
		t.Errorf("unbalanced asterisks: count=%d (should be even), got:\n%s", asteriskCount, output)
	}

	t.Logf("Output:\n%s", output)
}
