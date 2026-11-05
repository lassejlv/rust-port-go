// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"

// SpanEncoderRefByteSliceStrategy supplies the explicitly selected source S's
// [u8] implementation. OpaqueBytes is a TYPE dictionary fact, not a virtual
// callback: S must be exactly the original MemEncoder/FileEncoder to set it.
// This free function cannot be accidentally inherited by a new wrapper type.
func SpanEncoderRefByteSliceStrategy[S any](encoder *SpanEncoderRef[S]) rustcserialize.VectorEncodeOps[uint8] {
	if encoder.ops.OpaqueBytes {
		return rustcserialize.OpaqueByteVectorEncodeOps()
	}
	return rustcserialize.ByteVectorEncodeOps(encoder)
}

func spanByteSliceStrategy(encoder rustcserialize.Encoder) rustcserialize.VectorEncodeOps[uint8] {
	switch encoder.(type) {
	case *SpanMemEncoder, *SpanFileEncoder:
		// These exact Go projections ARE the original concrete source Self,
		// implementing foreign Span traits. A distinct embedding wrapper does
		// not match and uses its source generic implementation.
		return rustcserialize.OpaqueByteVectorEncodeOps()
	default:
		return rustcserialize.ByteVectorEncodeOps(encoder)
	}
}
