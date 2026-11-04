//go:build rustc_stable_hash_nightly

// SPDX-License-Identifier: MIT OR Apache-2.0
package stablehash

import "unicode/utf8"

const FeatureNightly = true

func (h *SipHasher128) WriteString(s string) {
	if !utf8.ValidString(s) {
		h.context.Panic("stablehash: Rust str requires valid UTF-8")
	}
	h.Write([]byte(s))
	h.WriteU8(255)
}
func (h *StableHasher[H, Hash]) WriteString(s string)     { h.stateString(s) }
func (h *StableHasher[H, Hash]) WriteLengthPrefix(n uint) { h.WriteUsize(n) }
