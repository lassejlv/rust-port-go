// SPDX-License-Identifier: CC0-1.0 OR Apache-2.0 OR Apache-2.0 WITH LLVM-exception
// Source-derived portable unkeyed hashing from blake3 1.8.2. This is a scoped
// dependency adapter, not a complete port of its source files or public API.
package blake3

import (
	"encoding/binary"
	"math/bits"
)

const (
	blockLen          = 64
	chunkLen          = 1024
	chunkStart uint32 = 1
	chunkEnd   uint32 = 2
	parent     uint32 = 4
	root       uint32 = 8
)

var iv = [8]uint32{0x6a09e667, 0xbb67ae85, 0x3c6ef372, 0xa54ff53a, 0x510e527f, 0x9b05688c, 0x1f83d9ab, 0x5be0cd19}

var schedule = [7][16]uint8{
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
	{2, 6, 3, 10, 7, 0, 4, 13, 1, 11, 12, 5, 9, 14, 15, 8},
	{3, 4, 10, 12, 13, 2, 7, 14, 6, 5, 9, 0, 11, 15, 8, 1},
	{10, 7, 12, 9, 14, 3, 13, 15, 4, 0, 11, 2, 5, 8, 1, 6},
	{12, 13, 9, 11, 15, 10, 14, 8, 7, 2, 5, 3, 0, 1, 6, 4},
	{9, 14, 11, 5, 8, 12, 15, 1, 13, 3, 0, 10, 2, 6, 4, 7},
	{11, 15, 5, 0, 1, 9, 8, 6, 14, 10, 2, 12, 3, 4, 7, 13},
}

func mix(s *[16]uint32, a, b, c, d int, x, y uint32) {
	s[a] += s[b] + x
	s[d] = bits.RotateLeft32(s[d]^s[a], -16)
	s[c] += s[d]
	s[b] = bits.RotateLeft32(s[b]^s[c], -12)
	s[a] += s[b] + y
	s[d] = bits.RotateLeft32(s[d]^s[a], -8)
	s[c] += s[d]
	s[b] = bits.RotateLeft32(s[b]^s[c], -7)
}

func compress(cv [8]uint32, block *[blockLen]byte, length int, counter uint64, flags uint32) (result [8]uint32) {
	var words [16]uint32
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(block[4*i:])
	}
	s := [16]uint32{cv[0], cv[1], cv[2], cv[3], cv[4], cv[5], cv[6], cv[7], iv[0], iv[1], iv[2], iv[3], uint32(counter), uint32(counter >> 32), uint32(length), flags}
	for _, r := range schedule {
		mix(&s, 0, 4, 8, 12, words[r[0]], words[r[1]])
		mix(&s, 1, 5, 9, 13, words[r[2]], words[r[3]])
		mix(&s, 2, 6, 10, 14, words[r[4]], words[r[5]])
		mix(&s, 3, 7, 11, 15, words[r[6]], words[r[7]])
		mix(&s, 0, 5, 10, 15, words[r[8]], words[r[9]])
		mix(&s, 1, 6, 11, 12, words[r[10]], words[r[11]])
		mix(&s, 2, 7, 8, 13, words[r[12]], words[r[13]])
		mix(&s, 3, 4, 9, 14, words[r[14]], words[r[15]])
	}
	for i := range result {
		result[i] = s[i] ^ s[i+8]
	}
	return
}

type output struct {
	cv      [8]uint32
	block   [blockLen]byte
	length  int
	counter uint64
	flags   uint32
}

func (o output) chainingValue() [8]uint32 {
	return compress(o.cv, &o.block, o.length, o.counter, o.flags)
}
func (o output) rootHash() (result [32]byte) {
	words := compress(o.cv, &o.block, o.length, 0, o.flags|root)
	for i, word := range words {
		binary.LittleEndian.PutUint32(result[4*i:], word)
	}
	return
}

func parentOutput(left, right [8]uint32) output {
	o := output{cv: iv, length: blockLen, flags: parent}
	for i := range left {
		binary.LittleEndian.PutUint32(o.block[4*i:], left[i])
		binary.LittleEndian.PutUint32(o.block[32+4*i:], right[i])
	}
	return o
}

type chunkState struct {
	cv             [8]uint32
	counter        uint64
	buf            [blockLen]byte
	bufLen, blocks int
}

func (c *chunkState) length() int { return c.blocks*blockLen + c.bufLen }
func (c *chunkState) startFlag() uint32 {
	if c.blocks == 0 {
		return chunkStart
	}
	return 0
}
func (c *chunkState) update(input []byte) {
	for len(input) > 0 {
		if c.bufLen == blockLen {
			c.cv = compress(c.cv, &c.buf, blockLen, c.counter, c.startFlag())
			c.blocks++
			c.buf = [blockLen]byte{}
			c.bufLen = 0
		}
		n := copy(c.buf[c.bufLen:], input)
		c.bufLen += n
		input = input[n:]
	}
}
func (c *chunkState) output() output {
	return output{cv: c.cv, block: c.buf, length: c.bufLen, counter: c.counter, flags: c.startFlag() | chunkEnd}
}

// Hasher supports the unkeyed Update/Finalize operations used by rustc_span.
// It serially combines the source's portable compression nodes. SIMD, keyed
// hashing, XOF, hazmat offsets and optional crate features remain unported.
// It has value-owned state; callers must synchronize concurrent mutation.
type Hasher struct {
	chunk       chunkState
	stack       [55][8]uint32
	stackLen    int
	initialized bool
}

func NewHasher() *Hasher { return &Hasher{chunk: chunkState{cv: iv}, initialized: true} }
func (h *Hasher) initialize() {
	if !h.initialized {
		*h = *NewHasher()
	}
}

// Update borrows input only for this call; input may be changed afterward.
func (h *Hasher) Update(input []byte) {
	h.initialize()
	for len(input) > 0 {
		if h.chunk.length() == chunkLen {
			cv := h.chunk.output().chainingValue()
			total := h.chunk.counter + 1
			for count := total; count&1 == 0; count >>= 1 {
				h.stackLen--
				cv = parentOutput(h.stack[h.stackLen], cv).chainingValue()
			}
			h.stack[h.stackLen] = cv
			h.stackLen++
			h.chunk = chunkState{cv: iv, counter: total}
		}
		n := min(len(input), chunkLen-h.chunk.length())
		h.chunk.update(input[:n])
		input = input[n:]
	}
}

// Finalize borrows the state. Repeated calls and later Update are supported.
func (h *Hasher) Finalize() [32]byte {
	h.initialize()
	o := h.chunk.output()
	for i := h.stackLen; i > 0; i-- {
		o = parentOutput(h.stack[i-1], o.chainingValue())
	}
	return o.rootHash()
}

func Hash(input []byte) [32]byte {
	h := NewHasher()
	h.Update(input)
	return h.Finalize()
}
