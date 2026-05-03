package tinylfu

import (
	"testing"
)

func TestNvec(t *testing.T) {

	n := newNvec(8)

	n.inc(0)
	if n[0] != 0x01 {
		t.Errorf("n[0]=0x%02x, want 0x01: (n=% 02x)", n[0], n)
	}
	if w := n.get(0); w != 1 {
		t.Errorf("n.get(0)=%d, want 1", w)
	}
	if w := n.get(1); w != 0 {
		t.Errorf("n.get(1)=%d, want 0", w)
	}

	n.inc(1)
	if n[0] != 0x11 {
		t.Errorf("n[0]=0x%02x, want 0x11: (n=% 02x)", n[0], n)
	}
	if w := n.get(0); w != 1 {
		t.Errorf("n.get(0)=%d, want 1", w)
	}
	if w := n.get(1); w != 1 {
		t.Errorf("n.get(1)=%d, want 1", w)
	}

	for i := 0; i < 14; i++ {
		n.inc(1)
	}
	if n[0] != 0xf1 {
		t.Errorf("n[1]=0x%02x, want 0xf1: (n=% 02x)", n[0], n)
	}
	if w := n.get(1); w != 15 {
		t.Errorf("n.get(1)=%d, want 15", w)
	}
	if w := n.get(0); w != 1 {
		t.Errorf("n.get(0)=%d, want 1", w)
	}

	// ensure clamped
	for i := 0; i < 3; i++ {
		n.inc(1)
		if n[0] != 0xf1 {
			t.Errorf("n[0]=0x%02x, want 0xf1: (n=% 02x)", n[0], n)
		}
	}

	n.reset()

	if n[0] != 0x70 {
		t.Errorf("n[0]=0x%02x, want 0x70 (n=% 02x)", n[0], n)
	}
}

func TestCM4(t *testing.T) {

	cm := newCM4(32)

	hash := uint64(0x0ddc0ffeebadf00d)

	cm.add(hash)
	cm.add(hash)

	if got := cm.estimate(hash); got != 2 {
		t.Errorf("cm.estimate(%x)=%d, want 2\n", hash, got)
	}
}

func TestNvecEdgeCases(t *testing.T) {
	// Test with odd size
	n := newNvec(7)
	if len(n) != 4 { // (7+1)/2 = 4
		t.Errorf("newNvec(7) length = %d, want 4", len(n))
	}

	// Test boundary indices
	n.inc(5)
	if got := n.get(5); got != 1 {
		t.Errorf("n.get(5) = %d, want 1", got)
	}

	// Test max value clamping
	for i := 0; i < 20; i++ {
		n.inc(6)
	}
	if got := n.get(6); got != 15 {
		t.Errorf("n.get(6) after 20 incs = %d, want 15 (clamped)", got)
	}
}

func TestCM4Reset(t *testing.T) {
	cm := newCM4(16)

	hash := uint64(0xcafebabe)

	// Add multiple times
	for i := 0; i < 10; i++ {
		cm.add(hash)
	}

	// Estimate should be non-zero
	est := cm.estimate(hash)
	if est == 0 {
		t.Error("cm.estimate should be > 0 after adds")
	}

	// Reset
	cm.reset()

	// After reset, estimate should be halved (aging mechanism)
	estAfter := cm.estimate(hash)
	if estAfter >= est {
		t.Errorf("cm.estimate after reset = %d, expected < %d (aging)", estAfter, est)
	}
}

func TestNextPowerOfTwo(t *testing.T) {
	tests := []struct {
		input    uint32
		expected uint32
	}{
		{1, 1},
		{2, 2},
		{3, 4},
		{5, 8},
		{8, 8},
		{9, 16},
		{100, 128},
		{1024, 1024},
		{1025, 2048},
	}

	for _, tt := range tests {
		if got := nextPowerOfTwo(tt.input); got != tt.expected {
			t.Errorf("nextPowerOfTwo(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
