// SPDX-License-Identifier: MIT OR Apache-2.0
// Port of rust-lang/rust compiler/rustc_span/src/edition.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
// Source SHA-256: a1dd82ce1a5b22b27e3360bcf8bd606fd4fa6669f6de6305536d6436d63ac7ae
// License texts: ../../../LICENSE-MIT and LICENSE-APACHE.

// Package rustcspan contains source-ported compiler span support types.
package rustcspan

import (
	"bytes"
	"fmt"
	"io"
)

// Edition is a compiler language edition. Its valid values are the five
// declared constants, ordered from oldest to newest. Casts to other values
// introduce a Go-only input outside the Rust enum's domain and panic in methods.
// Assignment copies the value, and native comparison/equality preserve the
// Rust enum's derived Copy, Eq and PartialOrd behavior for valid editions.
type Edition uint8

const (
	Edition2015 Edition = iota
	Edition2018
	Edition2021
	Edition2024
	EditionFuture
)

const (
	EditionNameList       = "<2015|2018|2021|2024|future>"
	EditionNameListStable = "<2015|2018|2021|2024>"
	DefaultEdition        = Edition2015
	LatestStableEdition   = Edition2024
)

// AllEditions returns the complete ordered edition list. A fresh slice keeps
// callers from mutating the immutable upstream ALL_EDITIONS constant.
func AllEditions() []Edition {
	return []Edition{Edition2015, Edition2018, Edition2021, Edition2024, EditionFuture}
}

func (edition Edition) validate() {
	if edition > EditionFuture {
		panic("rustcspan: invalid Edition discriminant")
	}
}

// Clone implements the derived Clone operation; ordinary assignment is Copy.
func (edition Edition) Clone() Edition { edition.validate(); return edition }

// CloneFrom is Clone's default replacement operation.
func (edition *Edition) CloneFrom(source Edition) { source.validate(); *edition = source }

func (edition Edition) AsStr() string {
	edition.validate()
	return [...]string{"2015", "2018", "2021", "2024", "future"}[edition]
}

func (edition Edition) String() string { return edition.AsStr() }

// DisplayTo preserves Display's write-error propagation for io.Writer callers.
func (edition Edition) DisplayTo(writer io.Writer) error {
	return writeEditionText(writer, edition.AsStr())
}

func writeEditionText(writer io.Writer, text string) error {
	n, err := io.WriteString(writer, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}

func (edition Edition) DebugString() string {
	edition.validate()
	return [...]string{"Edition2015", "Edition2018", "Edition2021", "Edition2024", "EditionFuture"}[edition]
}

// DebugTo is the derived Debug operation with write-error propagation.
func (edition Edition) DebugTo(writer io.Writer) error {
	return writeEditionText(writer, edition.DebugString())
}

// Format adapts Display (%s/%v) and Debug (%#v). The original Display forwards
// its bare string with write!, ignoring outer width, precision and sign flags;
// the derived Debug for these unit variants likewise writes only their name.
func (edition Edition) Format(state fmt.State, verb rune) {
	if verb == 's' || verb == 'v' {
		text := edition.AsStr()
		if verb == 'v' && state.Flag('#') {
			text = edition.DebugString()
		}
		_, _ = state.Write([]byte(text))
		return
	}
	_, _ = fmt.Fprintf(state, "%%!%c(rustcspan.Edition=%s)", verb, edition.DebugString())
}

func (edition Edition) LintName() string {
	edition.validate()
	return [...]string{"rust_2015_compatibility", "rust_2018_compatibility", "rust_2021_compatibility", "rust_2024_compatibility", "edition_future_compatibility"}[edition]
}

func (edition Edition) IsStable() bool        { edition.validate(); return edition != EditionFuture }
func (edition Edition) IsRust2015() bool      { edition.validate(); return edition == Edition2015 }
func (edition Edition) AtLeastRust2018() bool { edition.validate(); return edition >= Edition2018 }
func (edition Edition) AtLeastRust2021() bool { edition.validate(); return edition >= Edition2021 }
func (edition Edition) AtLeastRust2024() bool { edition.validate(); return edition >= Edition2024 }
func (edition Edition) AtLeastEditionFuture() bool {
	edition.validate()
	return edition >= EditionFuture
}

// Compare is the total comparison returned by the derived PartialOrd trait.
func (edition Edition) Compare(other Edition) int {
	edition.validate()
	other.validate()
	if edition < other {
		return -1
	}
	if edition > other {
		return 1
	}
	return 0
}

// EditionFromStr preserves FromStr's unit error as a false boolean.
func EditionFromStr(text string) (Edition, bool) {
	switch text {
	case "2015":
		return Edition2015, true
	case "2018":
		return Edition2018, true
	case "2021":
		return Edition2021, true
	case "2024":
		return Edition2024, true
	case "future":
		return EditionFuture, true
	default:
		return 0, false
	}
}

// EditionHasher supplies the typed operation used by the derived Hash impl.
// Keeping WriteIsize distinct from a raw byte write preserves custom Hasher
// overrides, caller state, and the host-sized discriminant operation.
type EditionHasher interface{ WriteIsize(int) }

func (edition Edition) Hash(hasher EditionHasher) {
	edition.validate()
	hasher.WriteIsize(int(edition))
}

// EditionStableHasher supplies the stable hasher's byte operation. StableHash
// uses syntactic variant order as u8 rather than Hash's isize discriminant.
type EditionStableHasher interface{ WriteU8(uint8) }

// StableHash leaves the caller's hashing algorithm/state intact. The source
// accepts a StableHashCtxt but never calls it for these fieldless variants;
// context is an opaque, unused token in this scoped Go trait adapter.
func (edition Edition) StableHash(context any, hasher EditionStableHasher) {
	edition.validate()
	hasher.WriteU8(uint8(edition))
}

// EditionSpanEncoder is the operation subset of SpanEncoder used by Edition.
// It does not claim to implement serialization for spans/symbols/definition IDs.
// EmitU8 owns any delayed-error state, as in the original encoder trait.
type EditionSpanEncoder interface{ EmitU8(uint8) }

func (edition Edition) Encode(encoder EditionSpanEncoder) {
	edition.validate()
	encoder.EmitU8(uint8(edition))
}

// EditionBlobDecoder is the operation subset of BlobDecoder used by Edition.
type EditionBlobDecoder interface{ ReadU8() uint8 }

func DecodeEdition(decoder EditionBlobDecoder) Edition {
	tag := decoder.ReadU8()
	if tag > uint8(EditionFuture) {
		panic(fmt.Sprintf("invalid enum variant tag while decoding `Edition`, expected 0..5, actual %d", tag))
	}
	return Edition(tag)
}

// EditionMemoryEncoder is a working memory adapter for the operation above.
// It uses the actual rustc_serialize MemEncoder byte protocol for Edition,
// without claiming to port unrelated serialization operations.
type EditionMemoryEncoder struct {
	data     []byte
	finished bool
}

func NewEditionMemoryEncoder() *EditionMemoryEncoder { return &EditionMemoryEncoder{} }

func (encoder *EditionMemoryEncoder) EmitU8(value uint8) {
	if encoder.finished {
		panic("rustcspan: memory encoder used after Finish")
	}
	encoder.data = append(encoder.data, value)
}

func (encoder *EditionMemoryEncoder) Position() uint { return uint(len(encoder.data)) }

// Finish transfers the buffer. As with consuming Rust self, further writes
// through this encoder are rejected. The serialization footer is caller-owned.
func (encoder *EditionMemoryEncoder) Finish() []byte {
	if encoder.finished {
		panic("rustcspan: memory encoder used after Finish")
	}
	encoder.finished = true
	data := encoder.data
	encoder.data = nil
	return data
}

// EditionBlobEndBytes is the rustc_serialize memory decoder's required footer.
const EditionBlobEndBytes = "rust-end-file"

// EditionMemoryDecoder implements the actual byte-reading/position protocol
// needed by Edition. It snapshots the input because Go cannot enforce Rust's
// immutable borrow for the decoder's lifetime.
type EditionMemoryDecoder struct {
	data     []byte
	position uint
}

func NewEditionMemoryDecoder(blob []byte, position uint) (*EditionMemoryDecoder, bool) {
	if !bytes.HasSuffix(blob, []byte(EditionBlobEndBytes)) {
		return nil, false
	}
	data := blob[:len(blob)-len(EditionBlobEndBytes)]
	if position > uint(len(data)) {
		panic("rustcspan: memory decoder position out of bounds")
	}
	return &EditionMemoryDecoder{data: append([]byte(nil), data...), position: position}, true
}

func (decoder *EditionMemoryDecoder) ReadU8() uint8 {
	if decoder.position == uint(len(decoder.data)) {
		panic("MemDecoder exhausted")
	}
	value := decoder.data[decoder.position]
	decoder.position++
	return value
}

func (decoder *EditionMemoryDecoder) Len() uint       { return uint(len(decoder.data)) }
func (decoder *EditionMemoryDecoder) Position() uint  { return decoder.position }
func (decoder *EditionMemoryDecoder) Remaining() uint { return decoder.Len() - decoder.position }
