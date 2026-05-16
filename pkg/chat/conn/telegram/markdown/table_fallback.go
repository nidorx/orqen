package markdown

import (
	"bytes"
	"regexp"

	"github.com/yuin/goldmark"
	gast "github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// tableFallbackDelimRow matches rows that are ONLY delimiter characters
// (i.e. `|---|---|`, `|:--|:--|`, etc.) - these are NOT data rows.
var tableFallbackDelimRow = regexp.MustCompile(`^[ |:-]*-[ |:-]*$`)

// tableFallbackLine checks if a line looks like a table row
// (starts and/or ends with `|`), but is NOT a delimiter row.
func tableFallbackLine(line []byte) bool {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return false
	}
	// Must start or end with pipe
	if trimmed[0] != '|' && trimmed[len(trimmed)-1] != '|' {
		return false
	}
	// Not a delimiter row
	if tableFallbackDelimRow.Match(trimmed) {
		return false
	}
	return true
}

// tableFallbackTransformer is a parser.ParagraphTransformer that detects
// tables without a delimiter row (fallback when goldmark native doesn't match).
type tableFallbackTransformer struct{}

var defaultTableFallbackTransformer = &tableFallbackTransformer{}

// NewTableFallbackTransformer returns a ParagraphTransformer that detects
// tables that lack a delimiter row (e.g. `| A | B |\n| 1 | 2 |`).
func NewTableFallbackTransformer() parser.ParagraphTransformer {
	return defaultTableFallbackTransformer
}

// Transform scans paragraph lines for pipe-delimited table rows and
// converts them into an AST Table when 2+ consecutive data rows are found.
func (t *tableFallbackTransformer) Transform(node *gast.Paragraph, reader text.Reader, pc parser.Context) {
	lines := node.Lines()
	if lines.Len() < 2 {
		return
	}

	source := reader.Source()
	ppos := node.Pos()

	start := -1
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		line := seg.Value(source)
		if tableFallbackLine(line) {
			if start == -1 {
				start = i
			}
		} else {
			if start != -1 && i-start >= 2 {
				t.buildTable(node, lines, start, i, ppos, source)
				return
			}
			start = -1
		}
	}
	if start != -1 && lines.Len()-start >= 2 {
		t.buildTable(node, lines, start, lines.Len(), ppos, source)
	}
}

func (t *tableFallbackTransformer) buildTable(
	node *gast.Paragraph,
	lines *text.Segments,
	start, end, ppos int,
	source []byte,
) {
	headerCells := t.parseRowCells(lines.At(start), source)
	if len(headerCells) == 0 {
		return
	}
	// Verify header cells have actual content (not just whitespace/empty)
	hasContent := false
	for _, cell := range headerCells {
		v := cell.Value(source)
		trimmed := bytes.TrimSpace(v)
		if len(trimmed) > 0 {
			hasContent = true
			break
		}
	}
	if !hasContent {
		return
	}
	colCount := len(headerCells)

	table := ast.NewTable()
	table.Alignments = make([]ast.Alignment, colCount)
	for i := range table.Alignments {
		table.Alignments[i] = ast.AlignNone
	}

	headerRow := ast.NewTableRow(table.Alignments)
	for _, seg := range headerCells {
		cell := ast.NewTableCell()
		cell.SetPos(seg.Start)
		cell.Lines().Append(seg)
		headerRow.AppendChild(headerRow, cell)
	}
	table.AppendChild(table, ast.NewTableHeader(headerRow))

	for i := start + 1; i < end; i++ {
		cells := t.parseRowCells(lines.At(i), source)
		row := ast.NewTableRow(table.Alignments)

		// Tolerate inconsistent column counts
		for j := 0; j < colCount; j++ {
			cell := ast.NewTableCell()
			if j < len(cells) {
				cell.SetPos(cells[j].Start)
				cell.Lines().Append(cells[j])
			}
			row.AppendChild(row, cell)
		}
		table.AppendChild(table, row)
	}

	table.SetPos(ppos)
	node.Lines().SetSliced(0, start)
	node.Parent().InsertAfter(node.Parent(), node, table)

	if node.Lines().Len() == 0 {
		node.Parent().RemoveChild(node.Parent(), node)
	} else {
		lastIdx := node.Lines().Len() - 1
		last := node.Lines().At(lastIdx)
		last.Stop = last.Stop - 1
		node.Lines().Set(lastIdx, last)
	}
}

// parseRowCells splits a single pipe-delimited line segment into cell segments.
func (t *tableFallbackTransformer) parseRowCells(seg text.Segment, source []byte) []text.Segment {
	origLine := seg.Value(source)

	// Trim whitespace for processing
	trimmed := bytes.TrimSpace(origLine)
	if len(trimmed) == 0 {
		return nil
	}

	// Calculate offset from original segment start to trimmed content
	trimOffset := 0
	for trimOffset < len(origLine) && origLine[trimOffset] == ' ' {
		trimOffset++
	}

	// Strip leading/trailing pipes
	innerStart := 0
	innerEnd := len(trimmed)
	if trimmed[0] == '|' {
		innerStart = 1
	}
	if innerEnd > innerStart && trimmed[innerEnd-1] == '|' {
		innerEnd--
	}

	inner := trimmed[innerStart:innerEnd]

	// Split cells by unescaped pipes
	cells := t.splitCells(inner)
	result := make([]text.Segment, 0, len(cells))

	// Calculate absolute position of inner content
	absInnerStart := seg.Start + trimOffset + innerStart

	pos := 0
	for _, cellContent := range cells {
		cellStart := absInnerStart + pos
		cellLen := len(cellContent)
		if cellStart+cellLen <= len(source) {
			cellSeg := text.NewSegment(cellStart, cellStart+cellLen)
			result = append(result, cellSeg)
		}
		pos += cellLen
		// Skip to next pipe in inner content
		for pos < len(inner) && inner[pos] != '|' {
			pos++
		}
		if pos < len(inner) && inner[pos] == '|' {
			pos++
		}
	}

	return result
}

// splitCells splits a pipe-delimited string into cell contents,
// handling escaped pipes (`\|`).
func (t *tableFallbackTransformer) splitCells(s []byte) [][]byte {
	var cells [][]byte
	var current []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '|' && i > 0 && s[i-1] == '\\' {
			// Escaped pipe - replace `\|` with `|` in content
			current = append(current[:len(current)-1], '|')
		} else if s[i] == '|' {
			cells = append(cells, bytes.TrimSpace(current))
			current = nil
		} else {
			current = append(current, s[i])
		}
	}
	cells = append(cells, bytes.TrimSpace(current))
	return cells
}

// tableFallbackExt is the goldmark extension wrapper.
type tableFallbackExt struct{}

// TableFallback is the exported extension variable (like Hidden, Strikethroughs).
var TableFallback = &tableFallbackExt{}

// Extend registers the fallback table parser.
func (e *tableFallbackExt) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithParagraphTransformers(
			util.Prioritized(NewTableFallbackTransformer(), 100),
		),
	)
}
