package markdown

import (
	"bytes"
	"testing"
)

func TestStringToBytes_Empty(t *testing.T) {
	got := StringToBytes("")
	if len(got) != 0 {
		t.Errorf("expected empty slice, got len=%d", len(got))
	}
}

func TestStringToBytes_Hello(t *testing.T) {
	got := StringToBytes("hello")
	if string(got) != "hello" {
		t.Errorf("expected 'hello', got %q", string(got))
	}
}

func TestStringToBytes_ContentEqual(t *testing.T) {
	input := "hello"
	got := StringToBytes(input)
	expected := []byte("hello")
	if !bytes.Equal(got, expected) {
		t.Errorf("expected %v, got %v", expected, got)
	}
}

func TestStringToBytes_Unicode(t *testing.T) {
	// Test with unicode characters
	input := "hello • world ‣ test ⁃"
	got := StringToBytes(input)
	if string(got) != input {
		t.Errorf("expected %q, got %q", input, string(got))
	}
}

func TestStringToBytes_SpecialChars(t *testing.T) {
	// Test with special markdown characters
	input := "_*[]()~` > # + - = | { } . !"
	got := StringToBytes(input)
	if string(got) != input {
		t.Errorf("expected %q, got %q", input, string(got))
	}
}
