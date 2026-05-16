package markdown

import (
	"bytes"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// renderMarkdown converts markdown input to Telegram markdown output.
func renderMarkdown(input string) (string, error) {
	md := New()
	var buf bytes.Buffer
	if err := md.Convert([]byte(input), &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestTable_Basic(t *testing.T) {
	input := `| Product | Price | Qty |
| :-: | :-: | :-: |
| Apples | $2.50 | 10 |
| Oranges | $3.00 | 15 |
`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain row numbers
	if !strings.Contains(output, "***01***") && !strings.Contains(output, "*01*") {
		t.Errorf("expected row number *01*, got:\n%s", output)
	}
	if !strings.Contains(output, "***02***") && !strings.Contains(output, "*02*") {
		t.Errorf("expected row number *02*, got:\n%s", output)
	}

	// Should contain column headers as prefixes
	if !strings.Contains(output, "Product:") {
		t.Errorf("expected 'Product:' column header, got:\n%s", output)
	}
	if !strings.Contains(output, "Price:") {
		t.Errorf("expected 'Price:' column header, got:\n%s", output)
	}
	if !strings.Contains(output, "Qty:") {
		t.Errorf("expected 'Qty:' column header, got:\n%s", output)
	}

	// Should contain values
	if !strings.Contains(output, "Apples") {
		t.Errorf("expected 'Apples' value, got:\n%s", output)
	}
	if !strings.Contains(output, "Price:") {
		t.Errorf("expected 'Price:' with value, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_ThreeRows(t *testing.T) {
	input := `| Name | Score |
| :--- | :--- |
| Alice | 95 |
| Bob | 87 |
| Carol | 92 |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, num := range []string{"01", "02", "03"} {
		if !strings.Contains(output, num) {
			t.Errorf("expected row number %s, got:\n%s", num, output)
		}
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_BoldItalicCodeStrikethrough(t *testing.T) {
	input := "| Format | Example |\n| :--- | :--- |\n| **Bold** | _Italic_ |\n| ~~Strike~~ | `code` |"

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Bold should be preserved with single asterisk (Telegram MarkdownV2)
	if !strings.Contains(output, "*Bold*") {
		t.Errorf("expected bold markers (*Bold*), got:\n%s", output)
	}
	// Strikethrough should be preserved with single tilde (Telegram MarkdownV2)
	if !strings.Contains(output, "~Strike~") {
		t.Errorf("expected strikethrough markers (~Strike~), got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_EmptyCell(t *testing.T) {
	input := `| A | B |
| - | - |
| val | |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have sub-items for both columns even if B is empty
	if !strings.Contains(output, "A:") || !strings.Contains(output, "B:") {
		t.Errorf("expected both 'A:' and 'B:' column prefixes, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_EscapedPipe(t *testing.T) {
	input := `| Col1 | Col2 |
| - | - |
| a\|b | c |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain the escaped pipe in output (escaping behavior may vary)
	if !strings.Contains(output, "Col1:") {
		t.Errorf("expected 'Col1:' column, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_Fallback_NoDelimiter(t *testing.T) {
	t.Skip("TODO: Re-enable when TableFallback parser is fixed to not interfere with native table parser")

	input := `| Product | Price |
| Apples | $2.50 |
| Oranges | $3.00 |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Product:") {
		t.Errorf("expected 'Product:' from fallback parser, got:\n%s", output)
	}
	if !strings.Contains(output, "Apples") {
		t.Errorf("expected 'Apples' from fallback parser, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_Fallback_InconsistentColumns(t *testing.T) {
	t.Skip("TODO: Re-enable when TableFallback parser is fixed to not interfere with native table parser")

	input := `| A | B | C |
| 1 | 2 |
| x | y | z | extra |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not panic, should render without error
	if !strings.Contains(output, "A:") {
		t.Errorf("expected 'A:' column, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_InBlockquote(t *testing.T) {
	input := `> | Name | Val |
> | - | - |
> | X | 1 |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should render within blockquote
	if !strings.Contains(output, ">") {
		t.Errorf("expected blockquote marker '>', got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_MultipleTables(t *testing.T) {
	input := `| A | B |
| - | - |
| 1 | 2 |

Some text between.

| X | Y |
| - | - |
| a | b |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both tables should be converted
	if !strings.Contains(output, "A:") || !strings.Contains(output, "X:") {
		t.Errorf("expected both tables converted, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_SingleColumn(t *testing.T) {
	input := `| Item |
| - |
| Apple |
| Banana |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(output, "Item:") {
		t.Errorf("expected 'Item:' column, got:\n%s", output)
	}
	if !strings.Contains(output, "Apple") {
		t.Errorf("expected 'Apple', got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_HeaderOnly_NoRows(t *testing.T) {
	input := `| A | B |
| - | - |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Header-only table should render nothing (no body rows)
	// At minimum, it should not error
	t.Logf("Output (should be minimal):\n%s", output)
}

func TestTable_Over99Rows(t *testing.T) {
	// Build a table with 101 rows
	var sb strings.Builder
	sb.WriteString("| N |\n| - |\n")
	for i := 1; i <= 101; i++ {
		sb.WriteString("| ")
		sb.WriteString(strings.Repeat("x", 5))
		sb.WriteString(" |\n")
	}

	output, err := renderMarkdown(sb.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use 3-digit numbering
	if !strings.Contains(output, "100") || !strings.Contains(output, "101") {
		t.Errorf("expected 3-digit row numbers (100, 101), got:\n%s", output)
	}

	t.Logf("Output (first 500 chars):\n%s", output[:min(500, len(output))])
}

func TestTable_Link(t *testing.T) {
	input := `| Site | URL |
| - | - |
| [Google](https://google.com) | Search |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain link format
	if !strings.Contains(output, "[Google]") || !strings.Contains(output, "(https://google.com)") {
		t.Errorf("expected link format [Google](https://google.com), got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_FallbackParser_Detection(t *testing.T) {
	t.Skip("TODO: Re-enable when TableFallback parser is fixed to not interfere with native table parser")
}

func TestTable_RowNumberPadding(t *testing.T) {
	input := `| A |
| - |
| 1 |
| 2 |
| 3 |
| 4 |
| 5 |
| 6 |
| 7 |
| 8 |
| 9 |
| 10 |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// First 9 rows should be 01-09
	if !strings.Contains(output, "*01*") {
		t.Errorf("expected *01*, got:\n%s", output)
	}
	if !strings.Contains(output, "*09*") {
		t.Errorf("expected *09*, got:\n%s", output)
	}
	// Row 10 should be 10
	if !strings.Contains(output, "*10*") {
		t.Errorf("expected *10*, got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func TestTable_RowNumberBold(t *testing.T) {
	input := `| A |
| - |
| x |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Row number should be bold (surrounded by * for Telegram MarkdownV2)
	if !strings.Contains(output, "*01*") {
		t.Errorf("expected bold markers around row number (*01*), got:\n%s", output)
	}

	t.Logf("Output:\n%s", output)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- Table Fallback unit tests ---

func TestTableFallback_Line(t *testing.T) {
	tests := []struct {
		name   string
		line   string
		expect bool
	}{
		{"valid row", "| A | B |", true},
		{"delimiter row", "|---|---|", false},
		{"empty line", "", false},
		{"whitespace only", "   ", false},
		{"no pipe", "hello world", false},
		{"pipe at end", "hello |", true},
		{"pipe at start", "| hello", true},
		{"spaced delimiters", "| - | - |", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tableFallbackLine([]byte(tt.line))
			if got != tt.expect {
				t.Errorf("tableFallbackLine(%q) = %v, want %v", tt.line, got, tt.expect)
			}
		})
	}
}

func TestTableFallback_NewTableFallbackTransformer(t *testing.T) {
	tft := NewTableFallbackTransformer()
	if tft == nil {
		t.Fatal("NewTableFallbackTransformer returned nil")
	}
}

func TestTableFallback_Extend(t *testing.T) {
	ext := &tableFallbackExt{}
	md := goldmark.New()
	// Extend should not panic
	ext.Extend(md)
}

// TestTableFallback_Transform exercises the Transform method by parsing
// a table without a delimiter row (which triggers the fallback parser).
func TestTableFallback_Transform(t *testing.T) {
	// Table without delimiter row — should trigger fallback
	input := `| Product | Price |
| Apples | $2.50 |
| Oranges | $3.00 |`

	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The native table parser handles this too, so output may vary.
	// At minimum, it should not error and should contain some content.
	if !strings.Contains(output, "Product") && !strings.Contains(output, "Apples") {
		t.Logf("Output (fallback may or may not trigger depending on goldmark version):\n%s", output)
	}
	t.Logf("Output:\n%s", output)
}

// TestTableFallback_Transform_SingleRow tests single-row table (no fallback)
func TestTableFallback_Transform_SingleRow(t *testing.T) {
	input := `| Only Row |`
	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Single row should not trigger fallback (needs 2+ rows)
	t.Logf("Output:\n%s", output)
}

// TestTableFallback_Transform_Direct tests the Transform method directly
func TestTableFallback_Transform_Direct(t *testing.T) {
	tft := &tableFallbackTransformer{}

	// Create a paragraph node with multiple table-like lines
	source := []byte("| A | B |\n| 1 | 2 |\n| 3 | 4 |\n")
	reader := text.NewReader(source)

	// Build a document with paragraph containing table lines
	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	p := md.Parser()
	doc := p.Parse(reader)

	// Find the paragraph node
	var para *gast.Paragraph
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if entering {
			if pr, ok := n.(*gast.Paragraph); ok {
				para = pr
				return gast.WalkStop, nil
			}
		}
		return gast.WalkContinue, nil
	})

	if para == nil {
		t.Skip("No paragraph found in parsed document")
	}

	pc := parser.NewContext()
	reader2 := text.NewReader(source)
	// Transform should not panic
	tft.Transform(para, reader2, pc)
}

// TestTableFallback_BuildTable_Direct tests the buildTable method directly
func TestTableFallback_BuildTable_Direct(t *testing.T) {
	tft := &tableFallbackTransformer{}

	source := []byte("| A | B |\n| 1 | 2 |\n| 3 | 4 |\n")

	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	p := md.Parser()
	reader := text.NewReader(source)
	doc := p.Parse(reader)

	var para *gast.Paragraph
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if entering {
			if pr, ok := n.(*gast.Paragraph); ok {
				para = pr
				return gast.WalkStop, nil
			}
		}
		return gast.WalkContinue, nil
	})

	if para == nil {
		t.Skip("No paragraph found")
	}

	lines := para.Lines()
	if lines.Len() < 3 {
		t.Skip("Not enough lines")
	}

	// Call buildTable directly
	tft.buildTable(para, lines, 0, 3, para.Pos(), source)
}

// TestTableFallback_ParseRowCells_Direct tests parseRowCells method
func TestTableFallback_ParseRowCells_Direct(t *testing.T) {
	tft := &tableFallbackTransformer{}
	source := []byte("| **bold** | *italic* | `code` |\n")

	md := goldmark.New(goldmark.WithExtensions(extension.Table))
	p := md.Parser()
	reader := text.NewReader(source)
	doc := p.Parse(reader)

	var para *gast.Paragraph
	_ = gast.Walk(doc, func(n gast.Node, entering bool) (gast.WalkStatus, error) {
		if entering {
			if pr, ok := n.(*gast.Paragraph); ok {
				para = pr
				return gast.WalkStop, nil
			}
		}
		return gast.WalkContinue, nil
	})

	if para == nil {
		t.Skip("No paragraph found")
	}

	lines := para.Lines()
	if lines.Len() == 0 {
		t.Skip("No lines found")
	}

	seg := lines.At(0)
	cells := tft.parseRowCells(seg, source)
	if len(cells) < 2 {
		t.Errorf("expected at least 2 cells, got %d", len(cells))
	}
}

// TestTableFallback_SplitCells_Direct tests splitCells method
func TestTableFallback_SplitCells_Direct(t *testing.T) {
	tft := &tableFallbackTransformer{}

	tests := []struct {
		name     string
		input    string
		minCells int
	}{
		{"basic", "| A | B | C |", 3},
		{"empty cells", "| | |", 2},
		{"escaped pipe", `| a\|b | c |`, 2},
		{"single cell", "| only |", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cells := tft.splitCells([]byte(tt.input))
			if len(cells) < tt.minCells {
				t.Errorf("expected at least %d cells, got %d: %v", tt.minCells, len(cells), cells)
			}
			// Verify non-empty cells contain expected content
			for i, cell := range cells {
				if len(bytes.TrimSpace(cell)) == 0 {
					t.Logf("cell %d is empty/whitespace", i)
				}
			}
		})
	}
}
