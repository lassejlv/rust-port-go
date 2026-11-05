// SPDX-License-Identifier: MIT OR Apache-2.0
// Full std Hasher dictionary paths for lib.rs derives. Typed Hash methods are
// retained as compatibility conveniences for existing primitive-only sinks.
package rustcspan

import stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"

func (value BytePos) HashWithHasher(h stablehash.Hasher) { stablehash.HashU32(h, uint32(value)) }
func (value RelativeBytePos) HashWithHasher(h stablehash.Hasher) {
	stablehash.HashU32(h, uint32(value))
}
func (value Span) HashWithHasher(h stablehash.Hasher) { stablehash.HashU64(h, value.ToRawSpan().Value) }
func (value SpanData) HashWithHasher(h stablehash.Hasher) {
	value.Lo.HashWithHasher(h)
	value.Hi.HashWithHasher(h)
	stablehash.HashU32(h, value.Ctxt.AsU32())
	if parent, ok := value.Parent.Get(); ok {
		stablehash.HashIsize(h, 1)
		stablehash.HashU32(h, parent.LocalDefIndex.AsU32())
	} else {
		stablehash.HashIsize(h, 0)
	}
}
func (value AttrId) HashWithHasher(h stablehash.Hasher) { stablehash.HashU32(h, value.AsU32()) }
func (value StableSourceFileId) HashWithHasher(h stablehash.Hasher) {
	stablehash.HashU64(h, value.hash.Truncate().AsU64())
}
func (value RemapPathScopeComponents) HashWithHasher(h stablehash.Hasher) {
	stablehash.HashU8(h, uint8(value))
}
