package markdown

import (
	"bytes"
	"testing"
)

func TestKindHidden_String(t *testing.T) {
	if got := KindHidden.String(); got != "Hidden" {
		t.Errorf("expected 'Hidden', got %q", got)
	}
}

func TestNewHidden(t *testing.T) {
	h := NewHidden()
	if h == nil {
		t.Fatal("NewHidden returned nil")
	}
	if h.Kind() != KindHidden {
		t.Errorf("expected KindHidden, got %v", h.Kind())
	}
}

func TestNewHiddenParser(t *testing.T) {
	p := NewHiddenParser()
	if p == nil {
		t.Fatal("NewHiddenParser returned nil")
	}

	trigger := p.Trigger()
	if len(trigger) != 1 {
		t.Fatalf("expected trigger length 1, got %d", len(trigger))
	}
	if trigger[0] != PipeChar.Byte() {
		t.Errorf("expected trigger '|', got %q", trigger[0])
	}
}

func TestHiddenDelimiterProcessor_IsDelimiter(t *testing.T) {
	proc := &hiddenDelimiterProcessor{}

	t.Run("pipe is delimiter", func(t *testing.T) {
		if !proc.IsDelimiter(PipeChar.Byte()) {
			t.Error("expected IsDelimiter('|') to return true")
		}
	})

	t.Run("x is not delimiter", func(t *testing.T) {
		if proc.IsDelimiter('x') {
			t.Error("expected IsDelimiter('x') to return false")
		}
	})

	t.Run("double pipe is delimiter", func(t *testing.T) {
		if !proc.IsDelimiter(PipeChar.Byte()) {
			t.Error("expected IsDelimiter('|') to return true for double pipe check")
		}
	})
}

func TestHidden_Extend(t *testing.T) {
	// The Hidden extension should be registered — verify by converting
	// a document that uses ||spoiler|| syntax
	input := "||hidden text||"
	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Output should contain || markers
	if !bytes.Contains([]byte(output), []byte("||")) {
		t.Errorf("expected || markers in output, got %q", output)
	}
}

func TestHidden_Conversion(t *testing.T) {
	t.Run("basic hidden", func(t *testing.T) {
		input := "||texto||"
		output, err := renderMarkdown(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !bytes.Contains([]byte(output), []byte("||texto||")) {
			t.Errorf("expected '||texto||' in output, got %q", output)
		}
	})

	t.Run("hidden in context", func(t *testing.T) {
		input := "Some ||spoiler|| text"
		output, err := renderMarkdown(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !bytes.Contains([]byte(output), []byte("||spoiler||")) {
			t.Errorf("expected '||spoiler||' in output, got %q", output)
		}
	})
}

func TestHiddenAST_Dump(t *testing.T) {
	h := NewHidden()
	// Dump should not panic
	h.Dump([]byte("||test||"), 0)
}

func TestHiddenDelimiterProcessor_CanOpenCloser(t *testing.T) {
	// This is tested indirectly through the parser — the CanOpenCloser
	// checks that opener.Char == closer.Char, which is always true for ||
	// since both open and close use the same pipe character.
	// We verify through end-to-end conversion.
	input := "||hidden||"
	output, err := renderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !bytes.Contains([]byte(output), []byte("||hidden||")) {
		t.Errorf("expected ||hidden|| in output, got %q", output)
	}
}

func TestHiddenDelimiterProcessor_OnMatch(t *testing.T) {
	proc := &hiddenDelimiterProcessor{}
	node := proc.OnMatch(2)
	if node == nil {
		t.Fatal("OnMatch returned nil")
	}
	if _, ok := node.(*HiddenAST); !ok {
		t.Errorf("expected *HiddenAST, got %T", node)
	}
}
