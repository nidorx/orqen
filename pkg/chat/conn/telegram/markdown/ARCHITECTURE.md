# Architecture

## 1. Overview

This package extends [goldmark](https://github.com/yuin/goldmark), a Go Markdown parser, to render Markdown into **Telegram MarkdownV2** format. It is a fork of [goldmark-tgmd](https://github.com/Mad-Pixels/goldmark-tgmd) by Mad Pixels, incorporated into the Orqen project.

**Purpose**: Telegram's MarkdownV2 dialect differs significantly from standard Markdown. This package bridges that gap by providing custom goldmark extensions and a renderer that translates standard Markdown AST nodes into Telegram-compatible output.

**Role in Orqen**: Used in `pkg/chat/conn/telegram/telegram.go` (via `var md = markdown.New()`) to convert agent response text from standard Markdown into Telegram MarkdownV2 before sending messages through the Telegram Bot API.

---

## 2. Architecture Diagram

```
Markdown Input
       │
       ▼
┌─────────────────────────┐
│  preprocessInput()      │   Pre-processing: fixes indented list lines
│  (in markdown.go)       │   so goldmark parses them as lists, not code
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│  goldmark Parser        │   Standard goldmark parser + custom extensions:
│                         │   • Strikethroughs  (~~text~~)
│                         │   • Hidden          (||text||)
│                         │   • DoubleSpace     (trailing "  " → newline)
│                         │   • extension.Table  (native goldmark tables)
│                         │   • TableFallback    (tables without delimiter row)
└────────────┬────────────┘
             │
             ▼
┌─────────────────────────┐
│  AST (Abstract Syntax   │   goldmark AST with custom node types:
│  Tree)                  │   • KindHidden, KindDoubleSpace
│                         │   • ext.KindStrikethrough, ext.KindTable, etc.
└────────────┬────────────┘
             │
             ▼
┌──────────────────────────┐
│  Renderer (custom)       │   Walks the AST and emits Telegram MarkdownV2:
│  (Renderer in markdown.go│   • Character escaping (escape map)
│  + handlers)             │   • Node-specific rendering (bold, italic, etc.)
│                          │   • Table → numbered list conversion
└────────────┬─────────────┘
             │
             ▼
┌─────────────────────────┐
│  Telegram MarkdownV2    │   Output: ***bold***, _italic_, ~~~strike~~~,
│  Output                 │   ||hidden||, `code`, > blockquote, etc.
└─────────────────────────┘
```

---

## 3. File Inventory

| File | Responsibility | Key Types/Functions | Dependencies |
|------|---------------|--------------------|--------------|
| `markdown.go` | Core renderer setup, `New()` factory, all render handlers | `New()`, `preprocessedMarkdown`, `Renderer`, `RegisterFuncs()`, all `r.*` handlers (`heading`, `paragraph`, `list`, `listItem`, `code`, `emphasis`, `link`, `blockquote`, `codeSpan`, `strikethrough`, `hidden`, `doubleSpace`, `document`, `table`, `tableHeader`, `tableRow`, `tableCell`, `renderCellInline`) | goldmark, goldmark/extension, goldmark/ast, all local files |
| `chars.go` | Custom rune/byte/tag types, Telegram tag constants, escape maps | `SpecialRune`, `SpecialChar`, `SpecialTag`, character constants (`AsteriskChar`, `PipeChar`, etc.), symbol constants (`CircleSymbol`, etc.), Telegram tags (`BoldTg`, `ItalicsTg`, `HiddenTg`, etc.), escape maps (`escape`, `escapeCode`, `escapeURL`) | `reflect`, `unsafe` |
| `config.go` | Global configuration for heading styles and list bullets | `Config` (global), `config`, `Element`, `UpdateHeading[1-6]()`, `Update[Primary/Secondary/Additional]ListBullet()` | goldmark/util, local (`SpecialTag`) |
| `strikethrough.go` | Extension for `~~text~~` → `~~~text~~~` | `Strikethroughs` (extension), `strikethrough.Extend()` | goldmark, goldmark/extension, goldmark/parser, goldmark/renderer |
| `hidden.go` | Extension for `\|\|text\|\|` → `\|\|text\|\|` (spoiler/hidden) | `KindHidden`, `HiddenAST`, `Hidden` (extension), `hiddenParser`, `hiddenDelimiterProcessor` | goldmark, goldmark/ast, goldmark/parser, goldmark/renderer |
| `doublespace.go` | Extension for trailing `  ` (double space) → newline | `KindDoubleSpace`, `DoubleSpace` (extension), `doubleSpaceParser`, `unclosedCounter` | goldmark, goldmark/ast, goldmark/parser, goldmark/renderer |
| `table_fallback.go` | Extension for tables **without** delimiter rows | `TableFallback` (extension), `tableFallbackTransformer`, `tableFallbackLine()`, `parseRowCells()`, `splitCells()` | goldmark, goldmark/ast, goldmark/extension/ast, goldmark/parser, regexp, bytes |
| `writer.go` | Low-level output helpers (escaping, tag writing) | `writeNewLine()`, `writeRune()`, `writeRowBytes()`, `writeCustomBytes()`, `writeEscapedBytes()`, `writeSpecialTagStart/End()` | goldmark/util, os |
| `utils.go` | String-to-bytes conversion | `StringToBytes()` | `reflect`, `unsafe` |
| `*_test.go` | Unit tests for each component | Test functions for each feature | testing, local package |

---

## 4. Renderer Pipeline

1. **Pre-processing**: `preprocessInput()` scans input for indented bullet patterns (4+ spaces followed by `- ` / `* ` / `+ `) and inserts blank lines before them, stripping the indent. This prevents goldmark from treating them as indented code blocks.

2. **Parser phase**: goldmark + registered extensions parse the (preprocessed) text into an AST. Each extension's `Extend()` method registers custom parsers with priorities:
   - `Strikethroughs`: inline parser at priority 500
   - `Hidden`: inline parser at priority 500
   - `DoubleSpace`: inline parser at priority 500
   - `TableFallback`: paragraph transformer at priority 100

3. **Render phase**: `Renderer.RegisterFuncs()` maps `ast.NodeKind` values to rendering functions. goldmark's renderer walks the AST depth-first, calling each handler with `(entering bool)` — `true` on node entry, `false` on exit.

4. **Walk control**: Each handler returns `ast.WalkStatus`:
   - `ast.WalkContinue` — continue walking children
   - `ast.WalkSkipChildren` — skip this node's children

---

## 5. Node Kind Registry

All node kinds registered in `Renderer.RegisterFuncs()`:

| NodeKind | Handler | Description |
|----------|---------|-------------|
| `ast.KindDocument` | `r.document` | Root node — no-op |
| `ast.KindParagraph` | `r.paragraph` | Paragraph blocks; adds newlines, detects preceding headings and following lists |
| `ast.KindHeading` | `r.heading` | H1-H6; renders as bold (Telegram has no native heading), handles pre-bold children |
| `ast.KindText` | `r.renderText` | Raw text nodes; applies character escaping |
| `ast.KindString` | `r.renderString` | String nodes; applies character escaping |
| `ast.KindEmphasis` | `r.emphasis` | Bold (level 2) → `***`, italic (level 1) → `_` |
| `ast.KindList` | `r.list` | List container; adds trailing newline at document level |
| `ast.KindListItem` | `r.listItem` | List items; adds bullet with indentation based on nesting level (3 levels) |
| `ast.KindLink` | `r.link` | `[text](url)` → `[text](escaped-url)` |
| `ast.KindBlockquote` | `r.blockquote` | `>` prefix on entering |
| `ast.KindFencedCodeBlock` | `r.code` | Triple-backtick code blocks; renders with language tag, escapes `` ` `` and `\` |
| `ast.KindCodeSpan` | `r.codeSpan` | Single-backtick inline code; sets `inCode` flag for different escaping |
| `ext.KindStrikethrough` | `r.strikethrough` | `~~~text~~~` (Telegram uses triple tilde) |
| `KindHidden` | `r.hidden` | `\|\|text\|\|` (Telegram spoiler/hidden) |
| `KindDoubleSpace` | `r.doubleSpace` | No-op renderer (double space already converted to newline at parse time) |
| `ext.KindTable` | `r.table` | Converts table AST to numbered list (Telegram doesn't support tables) |
| `ext.KindTableHeader` | `r.tableHeader` | No-op (handled by `r.table`) |
| `ext.KindTableRow` | `r.tableRow` | No-op (handled by `r.table`) |
| `ext.KindTableCell` | `r.tableCell` | No-op (handled by `r.table`) |

---

## 6. Extension Details

### Strikethroughs (`strikethrough.go`)

- **Parser**: Wraps `ext.NewStrikethroughParser()` from goldmark's built-in strikethrough extension.
- **Delimiter**: `~~text~~` (double tilde).
- **Renderer**: `r.strikethrough()` writes `~~~` (triple tilde) on both enter and exit, converting `~~text~~` → `~~~text~~~` per Telegram MarkdownV2 spec.

### Hidden (`hidden.go`)

- **Custom node type**: `HiddenAST` with `KindHidden`.
- **Parser**: `hiddenParser` uses goldmark's delimiter scanning (`parser.ScanDelimiter`) to detect `||` (double pipe). The `hiddenDelimiterProcessor` requires exactly 2 pipes as both opener and closer.
- **Delimiter**: `||text||` → renders as `||text||` (Telegram's spoiler syntax).
- **How detection works**: On encountering `|`, `ScanDelimiter` checks for 2 consecutive pipes. If matched, creates a `HiddenAST` node and pushes it onto the delimiter stack for matching closer.

### DoubleSpace (`doublespace.go`)

- **Parser**: `doubleSpaceParser` triggers on space characters. Detects exactly two consecutive spaces `  ` and converts them to a newline character (`\n`) at parse time.
- **Renderer**: `r.doubleSpace()` is a no-op — the newline is already in the AST as a `String` node.
- **Use case**: Markdown's "two trailing spaces = hard line break" convention.

### Table (native) (`extension.Table` from goldmark)

- Goldmark's built-in table extension. Detects tables with a delimiter row (`|---|---|`).
- **Renderer**: Handled by `r.table` — converts to numbered list (see Section 7).

### TableFallback (`table_fallback.go`)

- **Purpose**: Detects tables that **lack** a delimiter row (e.g., `| A | B |\n| 1 | 2 |`), which goldmark's native parser would miss.
- **Parser**: `tableFallbackTransformer` implements `parser.ParagraphTransformer`. It scans paragraph lines for consecutive pipe-delimited rows (2+).
- **Detection logic**: `tableFallbackLine()` checks if a line starts/ends with `|` and is NOT a delimiter row (matched by `tableFallbackDelimRow` regex).
- **AST construction**: Builds `ast.Table` with `ast.TableHeader` from the first row and `ast.TableRow` nodes for subsequent rows.
- **Currently disabled**: Commented out in `New()` due to interference with the native table parser (see `// TODO` in `markdown.go`).

---

## 7. Table-to-List Conversion

### Why it's needed

Telegram MarkdownV2 **does not support tables**. The only way to represent tabular data is as a structured list.

### Format transformation

**Input** (standard Markdown table):
```
| Product   | Price | Qty |
| :---      | :---  | :---|
| **Apples**| $2.50 | 10  |
| **Oranges**| $3.00| 15  |
```

**Output** (Telegram numbered list):
```

 • *01*
   • Product: **Apples**
   • Price: $2.50
   • Qty: 10
 • *02*
   • Product: **Oranges**
   • Price: $3.00
   • Qty: 15
```

### Implementation approach

1. **AST transformation at render time**: The `r.table()` handler does not modify the AST — instead, it extracts data and renders directly as list output.
2. **Header extraction**: Walks `ext.KindTableHeader` children, extracting plain text via `cellText()` (reads `Lines()` segments).
3. **Row extraction**: Iterates sibling `ext.KindTableRow` nodes after the header.
4. **Numbering**: Row numbers are zero-padded (`01`, `02`, ...). If > 99 rows, padding increases to 3 digits (`001`).
5. **Nested items**: Each column becomes a level-2 nested list item: `ColumnName: cellValue`.
6. **Formatting preservation**: Cell content is re-parsed through a temporary goldmark instance (`renderCellInline`) to preserve bold, italic, code, links, strikethrough, and hidden formatting.
7. **Helpers**: `renderTableListItemStart/End()` and `renderTableNestedItemStart/End()` manage indentation and bullet characters using `Config.listBullets`.

### Fallback parser

`TableFallback` handles tables without delimiter rows. It is a `ParagraphTransformer` that detects 2+ consecutive pipe-delimited lines and constructs an `ast.Table` AST node. Currently disabled in `New()` pending resolution of conflicts with the native table parser.

---

## 8. Configuration

The global `Config` variable (`config.go`) controls heading styles and list bullet characters:

### Headings

`Config.headings` is a `[6]Element` array for H1 through H6. Each `Element` has:

| Field | Type | Purpose |
|-------|------|---------|
| `Style` | `SpecialTag` | Wrapping tag (e.g., `ItalicsTg` for `_text_`) |
| `Prefix` | `string` | Text prepended before content |
| `Postfix` | `string` | Text appended after content |

**Default heading styles**:

| Level | Style | Prefix | Postfix |
|-------|-------|--------|---------|
| H1 | _(none)_ | _(none)_ | _(none)_ |
| H2 | _(none)_ | _(none)_ | _(none)_ |
| H3 | `ItalicsTg` | `# ` | _(none)_ |
| H4 | `ItalicsTg` | _(none)_ | _(none)_ |
| H5 | `ItalicsTg` | `~` | _(none)_ |
| H6 | `ItalicsTg` | _(none)_ | _(none)_ |

**Note**: In the current renderer (`r.heading`), headings are rendered as bold regardless of config — the config heading styles are defined but the heading handler overrides with bold. This is a known divergence.

**Update methods**:
```go
Config.UpdateHeading3(markdown.Element{Style: markdown.BoldTg, Prefix: "### "})
Config.UpdateHeading6(markdown.Element{Style: markdown.UnderlineTg})
```

### List Bullets

`Config.listBullets` is a `[3]rune` array for three nesting levels:

| Level | Default | Update method |
|-------|---------|---------------|
| Primary (top-level) | `•` (bullet) | `Config.UpdatePrimaryListBullet('◦')` |
| Secondary (nested) | `‣` (square) | `Config.UpdateSecondaryListBullet('-')` |
| Additional (deep) | `⁃` (triangle) | `Config.UpdateAdditionalListBullet('*')` |

---

## 9. Telegram MarkdownV2 Tags

Mapping from standard Markdown to Telegram MarkdownV2 output:

| Standard Markdown | Telegram Output | Tag Var | Notes |
|-------------------|----------------|---------|-------|
| `**bold**` | `***bold***` | `BoldTg` | Single `*` in Telegram spec; rendered as `***` (triple asterisk) |
| `_italic_` | `_italic_` | `ItalicsTg` | Same syntax |
| `~~strike~~` | `~~~strike~~~` | `StrikethroughTg` | Telegram uses triple `~` |
| `` `code` `` | `` `code` `` | `SpanTg` | Inline code |
| `\`\`\`lang\n...\`\`\`` | `\`\`\`lang\n...\`\`\`` | `CodeTg` | Code block with language |
| `\|\|hidden\|\|` | `\|\|hidden\|\|` | `HiddenTg` | Telegram spoiler syntax |
| `> quote` | `>` prefix | _(raw char)_ | Blockquote via `>` character |
| `[text](url)` | `[text](url)` | _(native)_ | Links, URL escaped separately |

Telegram MarkdownV2 spec: https://core.telegram.org/bots/api#markdownv2-style

---

## 10. Character Escaping

Telegram MarkdownV2 requires certain characters to be escaped with a backslash (`\`) in normal text, code blocks, and URLs.

### Escape maps (`chars.go`)

**`escape`** (used for normal text via `writeCustomBytes`):
| Character | Escaped as |
|-----------|-----------|
| `\` | `\\` |
| `_` | `\_` |
| `*` | `\*` |
| `[` | `\[` |
| `]` | `\]` |
| `(` | `\(` |
| `)` | `\)` |
| `{` | `\{` |
| `}` | `\}` |
| `#` | `\#` |
| `+` | `\+` |
| `-` | `\-` |
| `=` | `\=` |
| `\|` | `\|` |
| `.` | `\.` |
| `!` | `\!` |
| `>` | `\>` |
| `<` | `\<` |
| `~` | `\~` |
| `` ` `` | `` \` `` |

**`escapeCode`** (used inside code spans and code blocks):
| Character | Escaped as |
|-----------|-----------|
| `\` | `\\` |
| `` ` `` | `` \` `` |

**`escapeURL`** (used inside link URLs):
| Character | Escaped as |
|-----------|-----------|
| `\` | `\\` |
| `)` | `\)` |

### How escaping is applied

- `writeCustomBytes()` → uses `escape` map (normal text).
- `writeEscapedBytes(w, data, escapeCode)` → used inside code spans and code blocks.
- `writeEscapedBytes(w, n.Destination, escapeURL)` → used for link URLs.
- The `Renderer.inCode` flag toggles between `escape` and `escapeCode` for text/string nodes.

---

## 11. Usage Example

```go
package main

import (
    "bytes"
    "fmt"

    "github.com/nidorx/orqen/pkg/chat/conn/telegram/markdown"
)

func main() {
    md := markdown.New()
    var buf bytes.Buffer
    _ = md.Convert([]byte("**Hello** _world_"), &buf)
    fmt.Println(buf.String()) // ***Hello*** _world_
}
```

---

## 12. How to Extend

To add a new custom extension:

1. **Create a new file**: e.g., `myfeature.go` in this package.

2. **Define a NodeKind** (if introducing a new AST node type):
   ```go
   var KindMyFeature = ast.NewNodeKind("MyFeature")
   ```

3. **Create the AST node type** (if custom):
   ```go
   type MyFeatureAST struct { ast.BaseInline }
   func (n *MyFeatureAST) Kind() ast.NodeKind { return KindMyFeature }
   ```

4. **Create a parser** (if needed):
   - For inline syntax: implement `parser.InlineParser` (`Trigger()`, `Parse()`, `CloseBlock()`).
   - For paragraph-level syntax: implement `parser.ParagraphTransformer` (`Transform()`).

5. **Create a renderer handler** in `Renderer` (in `markdown.go` or `myfeature.go`):
   ```go
   func (r *Renderer) myfeature(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
       if entering {
           writeRowBytes(w, MyTag.Bytes())
       } else {
           writeRowBytes(w, MyTag.Bytes())
       }
       return ast.WalkContinue, nil
   }
   ```

6. **Register in `RegisterFuncs()`**:
   ```go
   reg.Register(KindMyFeature, r.myfeature)
   ```

7. **Create the extension** (`Extend()` method):
   ```go
   type myFeature struct{}
   var MyFeature = &myFeature{}
   func (e *myFeature) Extend(m goldmark.Markdown) {
       m.Parser().AddOptions(parser.WithInlineParsers(
           util.Prioritized(NewMyFeatureParser(), 500),
       ))
       m.Renderer().AddOptions(renderer.WithNodeRenderers(
           util.Prioritized(NewRenderer(), 500),
       ))
   }
   ```

8. **Register in `New()`**:
   ```go
   goldmark.WithExtensions(MyFeature)
   ```

9. **Add tests**: Create `myfeature_test.go` with test cases.

---

## 13. Known Limitations

1. **No native table support in Telegram**: Tables are converted to numbered lists. Complex table layouts (merged cells, multi-line cells) are not fully supported.
2. **4096 character message limit**: Handled in `telegram.go`, not in this package. The renderer does not split output.
3. **DoubleSpace only detects exactly 2 spaces**: Three or more consecutive spaces are not converted.
4. **Multi-line table cells not supported**: The table parser and fallback both assume single-line cells.
5. **TableFallback is disabled**: Currently commented out in `New()` because it interferes with the native `extension.Table` parser.
6. **Heading config styles not used**: `Config.headings` defines styles, but `r.heading()` overrides with bold rendering.
7. **Deep nesting limited to 3 levels**: `listItem` only handles 3 nesting depths via `Config.listBullets`.
8. **No image support**: Telegram MarkdownV2 does not support `![alt](url)` images; they are rendered as links only (text + URL).


