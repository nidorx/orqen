package markdown

import "testing"

func TestSpecialRune_Rune(t *testing.T) {
	t.Run("bullet circle", func(t *testing.T) {
		sr := SpecialRune('•')
		if got := sr.Rune(); got != '•' {
			t.Errorf("expected '•', got %q", got)
		}
	})

	t.Run("bullet square", func(t *testing.T) {
		sr := SpecialRune('‣')
		if got := sr.Rune(); got != '‣' {
			t.Errorf("expected '‣', got %q", got)
		}
	})

	t.Run("bullet triangle", func(t *testing.T) {
		sr := SpecialRune('⁃')
		if got := sr.Rune(); got != '⁃' {
			t.Errorf("expected '⁃', got %q", got)
		}
	})
}

func TestSpecialChar_Byte(t *testing.T) {
	t.Run("underscore", func(t *testing.T) {
		sc := SpecialChar('_')
		if got := sc.Byte(); got != '_' {
			t.Errorf("expected '_', got %q", got)
		}
	})

	t.Run("asterisk", func(t *testing.T) {
		sc := SpecialChar('*')
		if got := sc.Byte(); got != '*' {
			t.Errorf("expected '*', got %q", got)
		}
	})

	t.Run("pipe", func(t *testing.T) {
		sc := SpecialChar('|')
		if got := sc.Byte(); got != '|' {
			t.Errorf("expected '|', got %q", got)
		}
	})
}

func TestSpecialChar_Escaped(t *testing.T) {
	tests := []struct {
		name   string
		char   SpecialChar
		expect []byte
	}{
		{"underscore", UnderscoreChar, []byte{'\\', '_'}},
		{"asterisk", AsteriskChar, []byte{'\\', '*'}},
		{"pipe", PipeChar, []byte{'\\', '|'}},
		{"open bracket", OpenBracketChar, []byte{'\\', '['}},
		{"close bracket", CloseBracketChar, []byte{'\\', ']'}},
		{"open paren", OpenParenChar, []byte{'\\', '('}},
		{"close paren", CloseParenChar, []byte{'\\', ')'}},
		{"tilde", TildeChar, []byte{'\\', '~'}},
		{"backtick", BackqouteChar, []byte{'\\', '`'}},
		{"hash", HashChar, []byte{'\\', '#'}},
		{"slash", SlashChar, []byte{'\\', '\\'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.char.Escaped()
			if len(got) != 2 {
				t.Errorf("expected 2 bytes, got %d: %v", len(got), got)
			}
			if got[0] != '\\' {
				t.Errorf("expected first byte to be '\\', got %q", got[0])
			}
			if got[1] != tt.char.Byte() {
				t.Errorf("expected second byte to be %q, got %q", tt.char.Byte(), got[1])
			}
		})
	}
}

func TestSpecialTag_Bytes(t *testing.T) {
	t.Run("asterisk triple", func(t *testing.T) {
		st := SpecialTag{AsteriskChar, AsteriskChar, AsteriskChar}
		got := st.Bytes()
		if string(got) != "***" {
			t.Errorf("expected '***', got %q", string(got))
		}
	})

	t.Run("bold tag", func(t *testing.T) {
		got := BoldTg.Bytes()
		if string(got) != "*" {
			t.Errorf("BoldTg: expected '*', got %q", string(got))
		}
	})

	t.Run("hidden tag", func(t *testing.T) {
		got := HiddenTg.Bytes()
		if string(got) != "||" {
			t.Errorf("HiddenTg: expected '||', got %q", string(got))
		}
	})

	t.Run("strikethrough tag", func(t *testing.T) {
		got := StrikethroughTg.Bytes()
		if string(got) != "~" {
			t.Errorf("StrikethroughTg: expected '~', got %q", string(got))
		}
	})

	t.Run("italics tag", func(t *testing.T) {
		got := ItalicsTg.Bytes()
		if string(got) != "_" {
			t.Errorf("ItalicsTg: expected '_', got %q", string(got))
		}
	})

	t.Run("code tag", func(t *testing.T) {
		got := CodeTg.Bytes()
		if string(got) != "```" {
			t.Errorf("CodeTg: expected '```', got %q", string(got))
		}
	})

	t.Run("span tag", func(t *testing.T) {
		got := SpanTg.Bytes()
		if string(got) != "`" {
			t.Errorf("SpanTg: expected '`', got %q", string(got))
		}
	})

	t.Run("underline tag", func(t *testing.T) {
		got := UnderlineTg.Bytes()
		if string(got) != "__" {
			t.Errorf("UnderlineTg: expected '__', got %q", string(got))
		}
	})
}

func TestEscapeMap_ContainsRequiredEntries(t *testing.T) {
	requiredChars := []SpecialChar{
		PipeChar,
		UnderscoreChar,
		AsteriskChar,
		OpenBracketChar,
		CloseBracketChar,
		OpenParenChar,
		CloseParenChar,
		TildeChar,
		BackqouteChar,
		GreaterThanChar,
		HashChar,
		PlusChar,
		MinusChar,
		EqualChar,
		OpenBraceChar,
		CloseBraceChar,
		DotChar,
		ExclamationChar,
		SlashChar,
	}

	for _, sc := range requiredChars {
		t.Run(string([]byte{sc.Byte()}), func(t *testing.T) {
			b := sc.Byte()
			if _, ok := escape[b]; !ok {
				t.Errorf("escape map missing entry for byte %q", b)
			}
		})
	}
}

func TestEscapeCode_Map(t *testing.T) {
	// escapeCode should only contain backslash and backtick
	if _, ok := escapeCode[SlashChar.Byte()]; !ok {
		t.Error("escapeCode missing entry for backslash")
	}
	if _, ok := escapeCode[BackqouteChar.Byte()]; !ok {
		t.Error("escapeCode missing entry for backtick")
	}
	// escapeCode should NOT contain underscore
	if _, ok := escapeCode[UnderscoreChar.Byte()]; ok {
		t.Error("escapeCode should NOT contain underscore")
	}
}

func TestEscapeURL_Map(t *testing.T) {
	// escapeURL should only contain backslash and close paren
	if _, ok := escapeURL[SlashChar.Byte()]; !ok {
		t.Error("escapeURL missing entry for backslash")
	}
	if _, ok := escapeURL[CloseParenChar.Byte()]; !ok {
		t.Error("escapeURL missing entry for close paren")
	}
}
