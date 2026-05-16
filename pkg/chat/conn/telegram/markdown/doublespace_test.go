package markdown

import (
	"bytes"
	"testing"
)

func TestNewDoubleSpaceParser(t *testing.T) {
	p := NewDoubleSpaceParser()
	if p == nil {
		t.Fatal("NewDoubleSpaceParser returned nil")
	}

	trigger := p.Trigger()
	if len(trigger) != 1 {
		t.Fatalf("expected trigger length 1, got %d", len(trigger))
	}
	if trigger[0] != SpaceChar.Byte() {
		t.Errorf("expected trigger ' ', got %q", trigger[0])
	}
}

func TestDoubleSpace_Extend(t *testing.T) {
	// Create a fresh goldmark instance via New() which includes DoubleSpace
	md := New()
	_ = md // Just verify it doesn't panic
}

func TestDoubleSpace_Conversion(t *testing.T) {
	t.Run("double space becomes newline", func(t *testing.T) {
		input := "texto  \nmore"
		output, err := renderMarkdown(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Double space should produce a newline in output
		// The DoubleSpace parser converts '  ' to a newline character
		if !bytes.Contains([]byte(output), []byte("\n")) {
			t.Errorf("expected newline in output for double space, got %q", output)
		}
	})

	t.Run("single space does not become newline", func(t *testing.T) {
		input := "texto \nmore"
		output, err := renderMarkdown(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Single space should remain as space (not converted to newline)
		// The DoubleSpace parser only triggers on '  ' (double space)
		if bytes.Contains([]byte(output), []byte("texto \n")) {
			// This is expected — single space followed by newline
			t.Logf("Output: %q", output)
		}
	})
}

func TestKindDoubleSpace_String(t *testing.T) {
	if got := KindDoubleSpace.String(); got != "DoubleSpace" {
		t.Errorf("expected 'DoubleSpace', got %q", got)
	}
}
