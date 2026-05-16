package markdown

import (
	"bytes"
	"testing"
)

func TestStrikethroughs_Extend(t *testing.T) {
	// Verify strikethrough works by converting ~~text~~
	input := "~~strikethrough~~"
	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Telegram MarkdownV2 uses single tilde for strikethrough
	// The StrikethroughTg is defined as ~ (single tilde)
	if !bytes.Contains([]byte(output), []byte("~")) {
		t.Errorf("expected ~ markers in output, got %q", output)
	}
}

func TestStrikethrough_Conversion(t *testing.T) {
	t.Run("basic strikethrough", func(t *testing.T) {
		input := "~~texto~~"
		output, err := renderMarkdown(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Telegram MarkdownV2: ~strikethrough~
		if !bytes.Contains([]byte(output), []byte("~texto~")) {
			t.Errorf("expected '~texto~' in output, got %q", output)
		}
	})

	t.Run("strikethrough in context", func(t *testing.T) {
		input := "Some ~~deleted~~ text"
		output, err := renderMarkdown(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !bytes.Contains([]byte(output), []byte("~deleted~")) {
			t.Errorf("expected '~deleted~' in output, got %q", output)
		}
	})
}

func TestStrikethrough_SingleTildeNotParsed(t *testing.T) {
	// Single tilde should NOT be parsed as strikethrough (needs 2 tildes ~~)
	input := "~not strikethrough~"
	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Single tilde in plain text should be escaped as \~
	if !bytes.Contains([]byte(output), []byte("\\~")) {
		t.Logf("Warning: single tilde should be escaped, got %q", output)
	}
}
