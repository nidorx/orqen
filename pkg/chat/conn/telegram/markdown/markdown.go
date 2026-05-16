package markdown

import (
	"bytes"
	"fmt"
	"io"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	ext "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	textm "github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// preprocessInput inserts blank lines before indented bullet patterns so goldmark
// recognizes them as list blocks instead of paragraph continuation.
// After the blank line, we strip the leading 4-space indent so goldmark sees
// it as a regular list (not an indented code block).
func preprocessInput(src []byte) []byte {
	lines := bytes.Split(src, []byte{'\n'})
	var out [][]byte
	for i, line := range lines {
		if i > 0 && len(bytes.TrimSpace(lines[i-1])) > 0 {
			// Previous line was not blank — check if current line is an indented bullet
			trimmed := bytes.TrimLeft(line, " \t")
			indent := len(line) - len(trimmed)
			if indent >= 4 && indent <= 7 {
				if len(trimmed) >= 2 && (trimmed[0] == '-' || trimmed[0] == '*' || trimmed[0] == '+') && trimmed[1] == ' ' {
					// Insert blank line before and strip the 4-space indent
					// so goldmark sees it as a regular list, not code block
					out = append(out, []byte{}, trimmed)
					continue
				}
			}
		}
		out = append(out, line)
	}
	return bytes.Join(out, []byte{'\n'})
}

// New endpoint.
func New() goldmark.Markdown {
	return &preprocessedMarkdown{
		md: goldmark.New(
			goldmark.WithRenderer(
				renderer.NewRenderer(
					renderer.WithNodeRenderers(util.Prioritized(NewRenderer(), 1000)),
				),
			),
			goldmark.WithExtensions(Strikethroughs),
			goldmark.WithExtensions(Hidden),
			goldmark.WithExtensions(DoubleSpace),
			goldmark.WithExtensions(extension.Table),
			// TODO: Re-enable TableFallback once it doesn't interfere with native table parser
			// goldmark.WithExtensions(TableFallback),
		),
	}
}

// preprocessedMarkdown wraps goldmark.Markdown and preprocesses input before parsing.
type preprocessedMarkdown struct {
	md goldmark.Markdown
}

func (p *preprocessedMarkdown) Convert(source []byte, writer io.Writer, opts ...parser.ParseOption) error {
	preprocessed := preprocessInput(source)
	return p.md.Convert(preprocessed, writer, opts...)
}

func (p *preprocessedMarkdown) Parser() parser.Parser {
	return p.md.Parser()
}

func (p *preprocessedMarkdown) SetParser(prs parser.Parser) {
	p.md.SetParser(prs)
}

func (p *preprocessedMarkdown) Renderer() renderer.Renderer {
	return p.md.Renderer()
}

func (p *preprocessedMarkdown) SetRenderer(r renderer.Renderer) {
	p.md.SetRenderer(r)
}

// Renderer implement renderer.NodeRenderer object.
type Renderer struct {
	// inCode tracks whether we're currently inside a code span or code block,
	// so we use the correct escape map (escapeCode vs escape).
	inCode bool
}

// NewRenderer initialize Renderer as renderer.NodeRenderer.
func NewRenderer() renderer.NodeRenderer {
	return &Renderer{}
}

// RegisterFuncs add AST objects to Renderer.
func (r *Renderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindDocument, r.document)
	reg.Register(ast.KindParagraph, r.paragraph)

	reg.Register(ast.KindText, r.renderText)
	reg.Register(ast.KindString, r.renderString)
	reg.Register(ast.KindEmphasis, r.emphasis)

	reg.Register(ast.KindHeading, r.heading)
	reg.Register(ast.KindList, r.list)
	reg.Register(ast.KindListItem, r.listItem)
	reg.Register(ast.KindLink, r.link)

	reg.Register(ast.KindBlockquote, r.blockquote)
	reg.Register(ast.KindFencedCodeBlock, r.code)
	reg.Register(ast.KindCodeSpan, r.codeSpan)

	reg.Register(ext.KindStrikethrough, r.strikethrough)
	reg.Register(KindHidden, r.hidden)
	reg.Register(KindDoubleSpace, r.doubleSpace)

	reg.Register(ext.KindTable, r.table)
	reg.Register(ext.KindTableHeader, r.tableHeader)
	reg.Register(ext.KindTableRow, r.tableRow)
	reg.Register(ext.KindTableCell, r.tableCell)
}

func (r *Renderer) heading(w util.BufWriter, source []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	n := node.(*ast.Heading)
	if entering {
		if n.Level > 1 && n.Level < 4 {
			writeNewLine(w)
		}

		// Telegram has no heading syntax — render as bold.
		// If the heading already contains bold emphasis, we can't wrap the whole
		// thing in bold (would produce broken tags like *Module: *task**).
		// Instead, render bold manually per child node.
		if headingHasBoldChild(n) {
			r.renderHeadingWithBold(w, source, n)
			return ast.WalkSkipChildren, nil
		}

		// No bold children — safe to wrap entire heading in bold
		writeRowBytes(w, BoldTg.Bytes())
	} else {
		// Only close wrapper if we opened it above (not for hasBold case, already done)
		if !headingHasBoldChild(n) {
			writeRowBytes(w, BoldTg.Bytes())
		}
		// Always add trailing newline after heading to separate from following content
		writeNewLine(w)
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) paragraph(w util.BufWriter, source []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	n := node.(*ast.Paragraph)
	if entering {
		if n.Parent().Kind().String() != ast.KindBlockquote.String() {
			if !hasPrecedingHeading(n) {
				writeNewLine(w)
			}
		}
	} else {
		// If the next sibling is a List, skip the trailing newline —
		// the first ListItem will add its own leading newline, and we
		// want no blank line between paragraph and its attached list.
		if !hasFollowingList(n) {
			writeNewLine(w)
		}
	}
	return ast.WalkContinue, nil
}

// hasPrecedingHeading checks if the node's IMMEDIATE previous sibling is a heading.
func hasPrecedingHeading(node ast.Node) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}
	var prev ast.Node
	for s := parent.FirstChild(); s != nil; s = s.NextSibling() {
		if s == node {
			return prev != nil && prev.Kind() == ast.KindHeading
		}
		prev = s
	}
	return false
}

// hasFollowingList checks if any sibling after node is a List.
func hasFollowingList(node ast.Node) bool {
	for s := node.NextSibling(); s != nil; s = s.NextSibling() {
		if s.Kind() == ast.KindList {
			return true
		}
	}
	return false
}

// headingHasBoldChild checks if the heading contains any bold (Level==2) emphasis.
func headingHasBoldChild(heading *ast.Heading) bool {
	for c := heading.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() == ast.KindEmphasis {
			if em, ok := c.(*ast.Emphasis); ok && em.Level == 2 {
				return true
			}
		}
	}
	return false
}

// renderHeadingWithBold renders heading content with bold applied to the entire heading
// (flattening inner emphasis to avoid broken nested tags).
func (r *Renderer) renderHeadingWithBold(w util.BufWriter, source []byte, heading *ast.Heading) {
	writeRowBytes(w, BoldTg.Bytes())
	writeRowBytes(w, headingText(heading, source))
	writeRowBytes(w, BoldTg.Bytes())
}

// headingText extracts all text content from a heading node (flattens emphasis, etc.)
// by walking the AST and collecting Text/String values.
func headingText(heading *ast.Heading, source []byte) []byte {
	var buf []byte
	for c := heading.FirstChild(); c != nil; c = c.NextSibling() {
		headingTextFromNode(c, source, &buf)
	}
	return buf
}

// headingTextFromNode recursively extracts text from a heading child node.
func headingTextFromNode(node ast.Node, source []byte, buf *[]byte) {
	switch n := node.(type) {
	case *ast.Text:
		*buf = append(*buf, n.Segment.Value(source)...)
	case *ast.String:
		*buf = append(*buf, n.Value...)
	case *ast.Emphasis:
		// Skip emphasis wrapper, just get the text content
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			headingTextFromNode(c, source, buf)
		}
	case *ast.CodeSpan:
		// Include code content
		for c := n.FirstChild(); c != nil; c = c.NextSibling() {
			headingTextFromNode(c, source, buf)
		}
	default:
		for c := node.FirstChild(); c != nil; c = c.NextSibling() {
			headingTextFromNode(c, source, buf)
		}
	}
}

func (r *Renderer) list(w util.BufWriter, _ []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	n := node.(*ast.List)
	if !entering {
		if n.Parent().Kind().String() == ast.KindDocument.String() {
			writeNewLine(w)
		}
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) listItem(w util.BufWriter, _ []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	n := node.(*ast.ListItem)
	if entering {
		writeNewLine(w)
		if n.Parent().Parent().Kind().String() == ast.KindDocument.String() {
			writeRowBytes(w, []byte{SpaceChar.Byte(), SpaceChar.Byte()})
			writeRune(w, Config.listBullets[0])
		} else {
			if n.Parent().Parent().Parent().Parent() != nil {
				if n.Parent().Parent().Parent().Parent().Kind().String() == ast.KindListItem.String() {
					writeRowBytes(w, []byte{SpaceChar.Byte(), SpaceChar.Byte(), SpaceChar.Byte(), SpaceChar.Byte(), SpaceChar.Byte(), SpaceChar.Byte()})
					writeRune(w, Config.listBullets[2])
				} else {
					writeRowBytes(w, []byte{SpaceChar.Byte(), SpaceChar.Byte(), SpaceChar.Byte(), SpaceChar.Byte()})
					writeRune(w, Config.listBullets[1])
				}
			}
		}
		writeRowBytes(w, []byte{SpaceChar.Byte()})
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) code(w util.BufWriter, source []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	n := node.(interface {
		Lines() *textm.Segments
	})
	var content []byte
	l := n.Lines().Len()
	for i := 0; i < l; i++ {
		line := n.Lines().At(i)
		content = append(content, line.Value(source)...)
	}
	content = bytes.ReplaceAll(
		content,
		[]byte{TabChar.Byte()},
		[]byte{SpaceChar.Byte(), SpaceChar.Byte(), SpaceChar.Byte()},
	)
	nn := node.(*ast.FencedCodeBlock)
	if entering {
		writeNewLine(w)
		writeWrapperArr(w.Write(CodeTg.Bytes()))
		writeWrapperArr(w.Write(nn.Language(source)))
		writeNewLine(w)
	} else {
		// Inside code blocks, escape ` and \ per Telegram spec
		writeEscapedBytes(w, content, escapeCode)
		writeWrapperArr(w.Write(CodeTg.Bytes()))
		writeNewLine(w)
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderText(w util.BufWriter, source []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Text)
	text := n.Segment.Value(source)
	if n.HardLineBreak() {
		text = append(text, "\n"...)
	}
	// Escape special characters per Telegram MarkdownV2 spec
	// Use code escape map when inside code spans
	if r.inCode {
		writeEscapedBytes(w, text, escapeCode)
	} else {
		writeCustomBytes(w, text)
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) renderString(w util.BufWriter, source []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.String)
	// Escape special characters per Telegram MarkdownV2 spec
	// Use code escape map when inside code spans
	if r.inCode {
		writeEscapedBytes(w, n.Value, escapeCode)
	} else {
		writeCustomBytes(w, n.Value)
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) emphasis(w util.BufWriter, _ []byte, node ast.Node, _ bool) (
	ast.WalkStatus, error,
) {
	n := node.(*ast.Emphasis)
	if n.Level == 2 {
		writeRowBytes(w, BoldTg.Bytes())
	}
	if n.Level == 1 {
		writeRowBytes(w, ItalicsTg.Bytes())
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) link(w util.BufWriter, _ []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	n := node.(*ast.Link)
	if entering {
		writeRowBytes(w, []byte{OpenBracketChar.Byte()})
	} else {
		writeRowBytes(w, []byte{CloseBracketChar.Byte(), OpenParenChar.Byte()})
		// Inside link URL, escape ) and \ per Telegram spec
		writeEscapedBytes(w, n.Destination, escapeURL)
		writeRowBytes(w, []byte{CloseParenChar.Byte()})
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) blockquote(w util.BufWriter, _ []byte, _ ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	if entering {
		writeNewLine(w)
		writeRowBytes(w, []byte{GreaterThanChar.Byte()})
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) codeSpan(w util.BufWriter, source []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	if entering {
		r.inCode = true
		writeWrapperArr(w.Write(SpanTg.Bytes()))
	} else {
		writeWrapperArr(w.Write(SpanTg.Bytes()))
		r.inCode = false
	}
	return ast.WalkContinue, nil
}

func (r *Renderer) strikethrough(w util.BufWriter, _ []byte, _ ast.Node, _ bool) (
	ast.WalkStatus, error,
) {
	writeWrapperArr(w.Write(StrikethroughTg.Bytes()))
	return ast.WalkContinue, nil
}

func (r *Renderer) hidden(w util.BufWriter, _ []byte, _ ast.Node, _ bool) (
	ast.WalkStatus, error,
) {
	writeWrapperArr(w.Write(HiddenTg.Bytes()))
	return ast.WalkContinue, nil
}

func (r *Renderer) doubleSpace(_ util.BufWriter, _ []byte, _ ast.Node, _ bool) (
	ast.WalkStatus, error,
) {
	return ast.WalkContinue, nil
}

func (r *Renderer) document(_ util.BufWriter, _ []byte, _ ast.Node, _ bool) (
	ast.WalkStatus, error,
) {
	return ast.WalkContinue, nil
}

// --- Table handlers ---

// tableHeader is a no-op - the table handler does all the work.
func (r *Renderer) tableHeader(_ util.BufWriter, _ []byte, _ ast.Node, _ bool) (
	ast.WalkStatus, error,
) {
	return ast.WalkContinue, nil
}

// tableRow is a no-op - the table handler does all the work.
func (r *Renderer) tableRow(_ util.BufWriter, _ []byte, _ ast.Node, _ bool) (
	ast.WalkStatus, error,
) {
	return ast.WalkContinue, nil
}

// tableCell is a no-op - the table handler does all the work.
func (r *Renderer) tableCell(_ util.BufWriter, _ []byte, _ ast.Node, _ bool) (
	ast.WalkStatus, error,
) {
	return ast.WalkContinue, nil
}

// table converts a table AST into a numbered list format for Telegram.
// It extracts headers and rows, then for each row renders:
//   - *NN* (row number in bold)
//   - HeaderName: cellValue (with formatting preserved)
func (r *Renderer) table(w util.BufWriter, source []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}

	// Extract headers from TableHeader
	var headers []string
	headerNode := findChild(node, ext.KindTableHeader)
	if headerNode != nil {
		for c := headerNode.FirstChild(); c != nil; c = c.NextSibling() {
			if cell, ok := c.(*ext.TableCell); ok {
				headers = append(headers, cellText(cell, source))
			}
		}
	}

	// Extract body rows (siblings after TableHeader)
	type bodyRow struct {
		cells []ast.Node // TableCell nodes
	}
	var rows []bodyRow
	for n := headerNode.NextSibling(); n != nil; n = n.NextSibling() {
		if rowNode, ok := n.(*ext.TableRow); ok {
			var cells []ast.Node
			for c := rowNode.FirstChild(); c != nil; c = c.NextSibling() {
				if _, ok := c.(*ext.TableCell); ok {
					cells = append(cells, c)
				}
			}
			rows = append(rows, bodyRow{cells: cells})
		}
	}

	if len(rows) == 0 {
		return ast.WalkSkipChildren, nil
	}

	// Determine digit padding
	digitCount := 2
	if len(rows) > 99 {
		digitCount = 3
	}

	colCount := len(headers)

	for rowIdx, row := range rows {
		// Render level-1 list item: bullet + bold row number
		r.renderTableListItemStart(w)

		// Bold row number
		numStr := fmt.Sprintf("%0*d", digitCount, rowIdx+1)
		writeRowBytes(w, BoldTg.Bytes())
		writeRowBytes(w, []byte(numStr))
		writeRowBytes(w, BoldTg.Bytes())

		r.renderTableListItemEnd(w)

		// Render level-2 nested list items: one per column
		for colIdx := 0; colIdx < colCount; colIdx++ {
			r.renderTableNestedItemStart(w)

			// Column name prefix
			if colIdx < len(headers) {
				writeRowBytes(w, []byte(headers[colIdx]))
				writeRowBytes(w, []byte{ColonChar.Byte(), SpaceChar.Byte()})
			}

			// Render cell content with formatting preserved.
			// Cell content is stored in Lines() as text segments.
			// For formatting (bold, italic, etc.), we need to parse the cell
			// content as inline markdown. The cell Lines() contain raw text
			// that was already parsed - we need to re-parse it.
			if colIdx < len(row.cells) {
				cell := row.cells[colIdx].(*ext.TableCell)
				segs := cell.Lines()
				for i := 0; i < segs.Len(); i++ {
					seg := segs.At(i)
					cellContent := seg.Value(source)
					// Re-parse cell content as inline markdown to preserve formatting
					r.renderCellInline(w, source, cellContent)
				}
			}

			r.renderTableNestedItemEnd(w)
		}
	}

	return ast.WalkSkipChildren, nil
}

// renderTableListItemStart renders the start of a level-1 list item
// (same prefix as listItem for top-level: 2 spaces + bullet + space).
func (r *Renderer) renderTableListItemStart(w util.BufWriter) {
	writeNewLine(w)
	writeRowBytes(w, []byte{SpaceChar.Byte(), SpaceChar.Byte()})
	writeRune(w, Config.listBullets[0])
	writeRowBytes(w, []byte{SpaceChar.Byte()})
}

// renderTableListItemEnd renders the end of a level-1 list item.
func (r *Renderer) renderTableListItemEnd(_ util.BufWriter) {
	// Nothing special - content is inline
}

// renderTableNestedItemStart renders the start of a level-2 list item
// (4 spaces + bullet + space, matching nested listItem behavior).
func (r *Renderer) renderTableNestedItemStart(w util.BufWriter) {
	writeNewLine(w)
	writeRowBytes(w, []byte{SpaceChar.Byte(), SpaceChar.Byte(), SpaceChar.Byte(), SpaceChar.Byte()})
	writeRune(w, Config.listBullets[1])
	writeRowBytes(w, []byte{SpaceChar.Byte()})
}

// renderTableNestedItemEnd renders the end of a level-2 list item.
func (r *Renderer) renderTableNestedItemEnd(_ util.BufWriter) {
	// Nothing special - content is inline
}

// findChild returns the first child of node with the given kind.
func findChild(node ast.Node, kind ast.NodeKind) ast.Node {
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		if c.Kind() == kind {
			return c
		}
	}
	return nil
}

// cellText extracts plain text from a TableCell (for header names).
// Cell content is stored in Lines(), not as child nodes.
func cellText(cell *ext.TableCell, source []byte) string {
	var buf bytes.Buffer
	segs := cell.Lines()
	for i := 0; i < segs.Len(); i++ {
		seg := segs.At(i)
		buf.Write(seg.Value(source))
	}
	return buf.String()
}

// renderNode dispatches to the appropriate renderer for a node kind.
func (r *Renderer) renderNode(w util.BufWriter, source []byte, node ast.Node, entering bool) (
	ast.WalkStatus, error,
) {
	switch node.Kind() {
	case ast.KindText:
		return r.renderText(w, source, node, entering)
	case ast.KindString:
		return r.renderString(w, source, node, entering)
	case ast.KindEmphasis:
		return r.emphasis(w, source, node, entering)
	case ast.KindLink:
		return r.link(w, source, node, entering)
	case ast.KindCodeSpan:
		return r.codeSpan(w, source, node, entering)
	case ext.KindStrikethrough:
		return r.strikethrough(w, source, node, entering)
	case KindHidden:
		return r.hidden(w, source, node, entering)
	default:
		return ast.WalkContinue, nil
	}
}

// ColonChar is the ':' character for table cell rendering.
const ColonChar SpecialChar = ':'

// renderCellInline re-parses cell content through goldmark to preserve formatting
// (bold, italic, code, links, strikethrough, hidden).
func (r *Renderer) renderCellInline(w util.BufWriter, source []byte, content []byte) {
	if len(content) == 0 {
		return
	}
	// Create a temporary goldmark instance with the same renderer to parse cell content
	cellMd := goldmark.New(
		goldmark.WithRenderer(
			renderer.NewRenderer(
				renderer.WithNodeRenderers(util.Prioritized(NewRenderer(), 1000)),
			),
		),
		goldmark.WithExtensions(Strikethroughs),
		goldmark.WithExtensions(Hidden),
		goldmark.WithExtensions(DoubleSpace),
	)

	// Parse cell content as a paragraph
	doc := cellMd.Parser().Parse(textm.NewReader(content))

	// Walk the AST and render inline elements
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		switch n.Kind() {
		case ast.KindParagraph:
			// Skip paragraph wrapper
			return ast.WalkContinue, nil
		case ast.KindText:
			return r.renderText(w, content, n, entering)
		case ast.KindString:
			return r.renderString(w, content, n, entering)
		case ast.KindEmphasis:
			return r.emphasis(w, content, n, entering)
		case ast.KindLink:
			return r.link(w, content, n, entering)
		case ast.KindCodeSpan:
			return r.codeSpan(w, content, n, entering)
		case ext.KindStrikethrough:
			return r.strikethrough(w, content, n, entering)
		case KindHidden:
			return r.hidden(w, content, n, entering)
		case ast.KindDocument:
			return ast.WalkContinue, nil
		default:
			return ast.WalkContinue, nil
		}
	})
}
