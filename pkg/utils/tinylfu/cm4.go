package tinylfu

// cm4 is a conservative-update Count-Min Sketch with 4-bit counters.
// It tracks item access frequencies using minimal memory.
// Each counter is a nibble (4 bits), allowing values 0-15.
type cm4 struct {
	s    [depth]nvec
	mask uint32
}

const depth = 4

// newCM4 creates a new Count-Min Sketch with the given width.
// The width is rounded up to the next power of two for efficient masking.
func newCM4(w int) *cm4 {
	if w < 1 {
		panic("cm4: bad width")
	}

	w32 := nextPowerOfTwo(uint32(w))
	c := cm4{
		mask: w32 - 1,
	}

	for i := range depth {
		c.s[i] = newNvec(int(w32))
	}

	return &c
}

// add increments the frequency counter for the given hash across all depth levels.
func (c *cm4) add(keyh uint64) {
	h1, h2 := uint32(keyh), uint32(keyh>>32)

	for i := range c.s {
		pos := (h1 + uint32(i)*h2) & c.mask
		c.s[i].inc(pos)
	}
}

// estimate returns the minimum estimated frequency for the given hash.
// Returns a value between 0 and 15 (4-bit counter range).
func (c *cm4) estimate(keyh uint64) byte {
	h1, h2 := uint32(keyh), uint32(keyh>>32)

	var min byte = 255
	for i := range depth {
		pos := (h1 + uint32(i)*h2) & c.mask
		v := c.s[i].get(pos)
		if v < min {
			min = v
		}
	}
	return min
}

// reset halves all counters (age-out mechanism) using the 0x77 mask.
func (c *cm4) reset() {
	for _, n := range c.s {
		n.reset()
	}
}

// nvec is a packed nibble (4-bit) vector storage.
// Each byte stores two 4-bit counters to save memory.
type nvec []byte

func newNvec(w int) nvec {
	// Round up to handle odd widths correctly
	return make(nvec, (w+1)/2)
}

// get retrieves the 4-bit counter at index i.
func (n nvec) get(i uint32) byte {
	// Ugly, but as a single expression so the compiler will inline it :/
	return byte(n[i/2]>>((i&1)*4)) & 0x0f
}

// inc increments the 4-bit counter at index i, clamping at 15.
func (n nvec) inc(i uint32) {
	idx := i / 2
	shift := (i & 1) * 4
	v := (n[idx] >> shift) & 0x0f
	if v < 15 {
		n[idx] += 1 << shift
	}
}

// reset halves all counters using a right-shift aging mechanism.
func (n nvec) reset() {
	for i := range n {
		n[i] = (n[i] >> 1) & 0x77
	}
}
