package markdown

import (
	"bufio"
	"bytes"
	"testing"
)

func TestConfig_DefaultHeadings(t *testing.T) {
	// H1 (index 0): no extra wrapper, content has its own emphasis
	t.Run("H1 default", func(t *testing.T) {
		h := Config.headings[0]
		// Style should be empty (no extra wrapper)
		if len(h.Style) != 0 {
			t.Errorf("H1 Style should be empty, got %v", h.Style)
		}
		if h.Prefix != "" {
			t.Errorf("H1 Prefix should be empty, got %q", h.Prefix)
		}
	})

	// H2 (index 1): no extra wrapper, content has its own emphasis
	t.Run("H2 default", func(t *testing.T) {
		h := Config.headings[1]
		if len(h.Style) != 0 {
			t.Errorf("H2 Style should be empty, got %v", h.Style)
		}
		if h.Prefix != "" {
			t.Errorf("H2 Prefix should be empty, got %q", h.Prefix)
		}
	})

	// H3 (index 2): italic + "# " prefix
	t.Run("H3 default", func(t *testing.T) {
		h := Config.headings[2]
		if string(h.Style.Bytes()) != "_" {
			t.Errorf("H3 Style should be '_', got %q", string(h.Style.Bytes()))
		}
		if h.Prefix != "# " {
			t.Errorf("H3 Prefix should be '# ', got %q", h.Prefix)
		}
	})

	// H4 (index 3): italic, no prefix
	t.Run("H4 default", func(t *testing.T) {
		h := Config.headings[3]
		if string(h.Style.Bytes()) != "_" {
			t.Errorf("H4 Style should be '_', got %q", string(h.Style.Bytes()))
		}
		if h.Prefix != "" {
			t.Errorf("H4 Prefix should be empty, got %q", h.Prefix)
		}
	})

	// H5 (index 4): italic + "~" prefix
	t.Run("H5 default", func(t *testing.T) {
		h := Config.headings[4]
		if string(h.Style.Bytes()) != "_" {
			t.Errorf("H5 Style should be '_', got %q", string(h.Style.Bytes()))
		}
		if h.Prefix != "~" {
			t.Errorf("H5 Prefix should be '~', got %q", h.Prefix)
		}
	})

	// H6 (index 5): italic, no prefix
	t.Run("H6 default", func(t *testing.T) {
		h := Config.headings[5]
		if string(h.Style.Bytes()) != "_" {
			t.Errorf("H6 Style should be '_', got %q", string(h.Style.Bytes()))
		}
		if h.Prefix != "" {
			t.Errorf("H6 Prefix should be empty, got %q", h.Prefix)
		}
	})
}

func TestConfig_DefaultListBullets(t *testing.T) {
	t.Run("level 1 bullet is circle", func(t *testing.T) {
		if Config.listBullets[0] != CircleSymbol.Rune() {
			t.Errorf("level 1 bullet should be '•' (CircleSymbol), got %q", Config.listBullets[0])
		}
	})

	t.Run("level 2 bullet is square", func(t *testing.T) {
		if Config.listBullets[1] != SquareSymbol.Rune() {
			t.Errorf("level 2 bullet should be '‣' (SquareSymbol), got %q", Config.listBullets[1])
		}
	})

	t.Run("level 3 bullet is triangle", func(t *testing.T) {
		if Config.listBullets[2] != TriangleSymbol.Rune() {
			t.Errorf("level 3 bullet should be '⁃' (TriangleSymbol), got %q", Config.listBullets[2])
		}
	})
}

func TestConfig_UpdateHeading(t *testing.T) {
	// Save original to restore after test
	origH1 := Config.headings[0]
	defer func() { Config.headings[0] = origH1 }()

	t.Run("UpdateHeading1", func(t *testing.T) {
		newEl := Element{
			Style:   BoldTg,
			Prefix:  "!!!",
			Postfix: "!!!",
		}
		Config.UpdateHeading1(newEl)
		if Config.headings[0].Prefix != "!!!" {
			t.Errorf("UpdateHeading1 failed: prefix=%q", Config.headings[0].Prefix)
		}
		if string(Config.headings[0].Style.Bytes()) != "*" {
			t.Errorf("UpdateHeading1 failed: style=%q", string(Config.headings[0].Style.Bytes()))
		}
	})

	t.Run("UpdateHeading3", func(t *testing.T) {
		origH3 := Config.headings[2]
		defer func() { Config.headings[2] = origH3 }()

		newEl := Element{
			Style:   BoldTg,
			Prefix:  ">>> ",
			Postfix: " <<<",
		}
		Config.UpdateHeading3(newEl)
		if Config.headings[2].Prefix != ">>> " {
			t.Errorf("UpdateHeading3 failed: prefix=%q", Config.headings[2].Prefix)
		}
	})
}

func TestConfig_UpdateListBullet(t *testing.T) {
	// Save original to restore after test
	origBullets := Config.listBullets
	defer func() { Config.listBullets = origBullets }()

	t.Run("UpdatePrimaryListBullet", func(t *testing.T) {
		Config.UpdatePrimaryListBullet('◦')
		if Config.listBullets[0] != '◦' {
			t.Errorf("UpdatePrimaryListBullet failed: got %q", Config.listBullets[0])
		}
	})

	t.Run("UpdateSecondaryListBullet", func(t *testing.T) {
		Config.UpdateSecondaryListBullet('▪')
		if Config.listBullets[1] != '▪' {
			t.Errorf("UpdateSecondaryListBullet failed: got %q", Config.listBullets[1])
		}
	})

	t.Run("UpdateAdditionalListBullet", func(t *testing.T) {
		Config.UpdateAdditionalListBullet('▸')
		if Config.listBullets[2] != '▸' {
			t.Errorf("UpdateAdditionalListBullet failed: got %q", Config.listBullets[2])
		}
	})
}

func TestElement_writeStart_writeEnd(t *testing.T) {
	t.Run("bold element", func(t *testing.T) {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)

		el := Element{
			Style:   BoldTg,
			Prefix:  "",
			Postfix: "",
		}

		el.writeStart(w)
		w.Flush()
		if buf.String() != "*" {
			t.Errorf("writeStart: expected '*', got %q", buf.String())
		}

		buf.Reset()
		el.writeEnd(w)
		w.Flush()
		if buf.String() != "*" {
			t.Errorf("writeEnd: expected '*', got %q", buf.String())
		}
	})

	t.Run("element with prefix and postfix", func(t *testing.T) {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)

		el := Element{
			Style:   ItalicsTg,
			Prefix:  "pre ",
			Postfix: " end",
		}

		el.writeStart(w)
		w.Flush()
		// writeSpecialTagStart: tag first, then prefix → _ + "pre "
		if buf.String() != "_pre " {
			t.Errorf("writeStart: expected '_pre ', got %q", buf.String())
		}

		buf.Reset()
		el.writeEnd(w)
		w.Flush()
		// writeSpecialTagEnd: postfix first, then tag → " end" + _
		if buf.String() != " end_" {
			t.Errorf("writeEnd: expected ' end_', got %q", buf.String())
		}
	})
}
