// SPDX-License-Identifier: MIT OR Apache-2.0
// Encodable/Decodable implementations and derives owned by def_id.rs.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func (value LocalDefId) Encode(encoder SpanEncoder)   { SpanEncodeLocalDefId(encoder, value) }
func DecodeLocalDefId(decoder BlobDecoder) LocalDefId { return decoder.DecodeLocalDefId() }
func (value ModId) Encode(encoder SpanEncoder)        { value.defId.Encode(encoder) }
func DecodeModId(decoder SpanDecoder) ModId           { return NewModIdUnchecked(DecodeDefId(decoder)) }
func (value LocalModId) Encode(encoder SpanEncoder)   { value.localDefId.Encode(encoder) }
func DecodeLocalModId(decoder SpanDecoder) LocalModId {
	return NewLocalModIdUnchecked(DecodeLocalDefId(decoder))
}
func (value DefPathHash) Encode(encoder SpanEncoder) { value.Fingerprint.Encode(encoder) }
func DecodeDefPathHash(decoder SpanDecoder) DefPathHash {
	return DefPathHash{datastructures.DecodeFingerprintWithContext(defIdCodecContext(decoder), defIdFingerprintDecoder{decoder})}
}
func (value StableCrateId) Encode(encoder SpanEncoder) {
	rustcserialize.EncodeHash64(encoder, value.hash)
}
func DecodeStableCrateId(decoder BlobDecoder) StableCrateId {
	return StableCrateId{rustcserialize.DecodeHash64(decoder)}
}

type defIdFingerprintDecoder struct{ decoder SpanDecoder }

func (decoder defIdFingerprintDecoder) ReadRawBytes(length int) []byte {
	return decoder.decoder.ReadRawBytes(uint(length))
}
func defIdCodecContext(value any) rustfmt.Context {
	if typed, ok := value.(interface{ RuntimeContext() rustfmt.Context }); ok {
		return typed.RuntimeContext()
	}
	return rustfmt.Context{}
}
