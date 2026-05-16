package markdown

import (
	"bufio"
	"bytes"
	"testing"
)

func TestWriteNewLine(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	writeNewLine(w)
	w.Flush()

	if buf.String() != "\n" {
		t.Errorf("expected '\\n', got %q", buf.String())
	}
}

func TestWriteRune(t *testing.T) {
	t.Run("bullet circle", func(t *testing.T) {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)
		writeRune(w, '•')
		w.Flush()

		if buf.String() != "•" {
			t.Errorf("expected '•', got %q", buf.String())
		}
	})

	t.Run("bullet square", func(t *testing.T) {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)
		writeRune(w, '‣')
		w.Flush()

		if buf.String() != "‣" {
			t.Errorf("expected '‣', got %q", buf.String())
		}
	})
}

func TestWriteRowBytes(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	writeRowBytes(w, []byte("hello"))
	w.Flush()

	if buf.String() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.String())
	}
}

func TestWriteCustomBytes_Escaping(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		expect string
	}{
		{"underscore", []byte("hello_world"), "hello\\_world"},
		{"asterisk", []byte("hello*world"), "hello\\*world"},
		{"open bracket", []byte("hello[world"), "hello\\[world"},
		{"close bracket", []byte("hello]world"), "hello\\]world"},
		{"open paren", []byte("hello(world"), "hello\\(world"},
		{"close paren", []byte("hello)world"), "hello\\)world"},
		{"tilde", []byte("hello~world"), "hello\\~world"},
		{"backtick", []byte("hello`world"), "hello\\`world"},
		{"hash", []byte("hello#world"), "hello\\#world"},
		{"plus", []byte("hello+world"), "hello\\+world"},
		{"minus", []byte("hello-world"), "hello\\-world"},
		{"equal", []byte("hello=world"), "hello\\=world"},
		{"pipe", []byte("hello|world"), "hello\\|world"},
		{"open brace", []byte("hello{world"), "hello\\{world"},
		{"close brace", []byte("hello}world"), "hello\\}world"},
		{"dot", []byte("hello.world"), "hello\\.world"},
		{"exclamation", []byte("hello!world"), "hello\\!world"},
		{"backslash", []byte("hello\\world"), "hello\\\\world"},
		{"greater than", []byte("hello>world"), "hello\\>world"},
		{"less than", []byte("hello<world"), "hello\\<world"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := bufio.NewWriter(&buf)
			writeCustomBytes(w, tt.input)
			w.Flush()

			if buf.String() != tt.expect {
				t.Errorf("expected %q, got %q", tt.expect, buf.String())
			}
		})
	}
}

func TestWriteCustomBytes_NoEscaping(t *testing.T) {
	// Normal letters and digits should NOT be escaped
	input := []byte("hello123abc")
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	writeCustomBytes(w, input)
	w.Flush()

	if buf.String() != "hello123abc" {
		t.Errorf("expected 'hello123abc' (no escaping), got %q", buf.String())
	}
}

func TestWriteSpecialTagStart(t *testing.T) {
	t.Run("bold tag with no prefix", func(t *testing.T) {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)
		writeSpecialTagStart(w, BoldTg, nil)
		w.Flush()

		if buf.String() != "*" {
			t.Errorf("expected '*', got %q", buf.String())
		}
	})

	t.Run("italic tag with prefix", func(t *testing.T) {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)
		writeSpecialTagStart(w, ItalicsTg, StringToBytes("pre "))
		w.Flush()

		// writeSpecialTagStart writes tag first, then prefix: _ + "pre "
		if buf.String() != "_pre " {
			t.Errorf("expected '_pre ', got %q", buf.String())
		}
	})

	t.Run("hidden tag", func(t *testing.T) {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)
		writeSpecialTagStart(w, HiddenTg, nil)
		w.Flush()

		if buf.String() != "||" {
			t.Errorf("expected '||', got %q", buf.String())
		}
	})
}

func TestWriteSpecialTagEnd(t *testing.T) {
	t.Run("bold tag with no postfix", func(t *testing.T) {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)
		writeSpecialTagEnd(w, BoldTg, nil)
		w.Flush()

		if buf.String() != "*" {
			t.Errorf("expected '*', got %q", buf.String())
		}
	})

	t.Run("italic tag with postfix", func(t *testing.T) {
		var buf bytes.Buffer
		w := bufio.NewWriter(&buf)
		writeSpecialTagEnd(w, ItalicsTg, StringToBytes(" end"))
		w.Flush()

		// writeCustomBytes escapes special chars
		if buf.String() != " end_" {
			t.Errorf("expected ' end_', got %q", buf.String())
		}
	})
}

func TestWriteEscapedBytes_CodeMap(t *testing.T) {
	// Inside code blocks, only ` and \ should be escaped
	input := []byte("hello`world\\test_value")
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	writeEscapedBytes(w, input, escapeCode)
	w.Flush()

	// ` and \ escaped, but _ NOT escaped in code
	expected := "hello\\`world\\\\test_value"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestWriteEscapedBytes_URLMap(t *testing.T) {
	// Inside link URLs, only ) and \ should be escaped
	input := []byte("http://example.com/path(file)")
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	writeEscapedBytes(w, input, escapeURL)
	w.Flush()

	// escapeURL only escapes ) and \ — ( is NOT escaped per Telegram spec
	expected := "http://example.com/path(file\\)"
	if buf.String() != expected {
		t.Errorf("expected %q, got %q", expected, buf.String())
	}
}

func TestRender(t *testing.T) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	render(w, []byte("hello"))
	w.Flush()

	if buf.String() != "hello" {
		t.Errorf("expected 'hello', got %q", buf.String())
	}
}

func TestWriteWrapper_ErrNil(t *testing.T) {
	// Should not panic on nil error
	writeWrapper(nil)
}

func TestWriteWrapperArr_ErrNil(t *testing.T) {
	// Should not panic on nil error
	writeWrapperArr(0, nil)
}
