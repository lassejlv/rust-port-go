// SPDX-License-Identifier: MIT OR Apache-2.0
// Port of compiler/rustc_data_structures/src/fingerprint.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
// Git blob: 1855d1ff921fcd43c3daecd97c017562209f95df.
// Source SHA-256: b34cbc70955dd34efe762559bf5704908a4f93a3f9d6efb88de2f91c9a26127f.
package datastructures

import (
	"encoding/binary"
	"math/bits"
	"os"

	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Fingerprint preserves the source's two-limb order. Its Go zero value is ZERO;
// assignment and native equality supply Copy and Eq. Compare compares the first
// limb before the second, independently of the unsigned u128 numeric order.
type Fingerprint struct{ first, second uint64 }

// FingerprintComponent is the source's open borrowed conversion contract.
// Hash64 implements it. Use FingerprintU64 for the source's primitive u64 impl.
// Components with destructors use NewFingerprintOwned below.
type FingerprintComponent interface{ AsU64() uint64 }
type FingerprintU64 uint64

func (value FingerprintU64) AsU64() uint64 { return uint64(value) }

func ZeroFingerprint() Fingerprint { return Fingerprint{} }
func NewFingerprint(first, second FingerprintComponent) Fingerprint {
	return Fingerprint{first.AsU64(), second.AsU64()}
}
func FingerprintFromU64(first, second uint64) Fingerprint { return Fingerprint{first, second} }

// FingerprintDropContext carries explicit ambient source panic state together
// with local unwinding. It never discovers panic state through Go globals.
type FingerprintDropContext = rustfmt.Context

// OwnedFingerprintComponent transfers exclusive ownership of Value to one
// constructor call; do not reuse the descriptor after consumption. Callbacks
// temporarily borrow the actual local cell and must not retain its pointer.
// Close models Drop. Context-aware callbacks take precedence when supplied.
// Translated panic sites must call Context.Panic for source timing. Raw Go
// panic/defer callbacks retain callback-local Go timing; processing-mode fallback
// guards can abort only after such a panic escapes the callback.
type OwnedFingerprintComponent[T any] struct {
	Value            T
	AsU64            func(*T) uint64
	AsU64WithContext func(*T, FingerprintDropContext) uint64
	Close            func(*T)
	CloseWithContext func(*T, FingerprintDropContext)
}

func deferFingerprintComponent[T any](component *OwnedFingerprintComponent[T], ambient FingerprintDropContext) {
	panicValue := recover()
	// A raw Go panic has already run any callback-local defers. Once it
	// reaches this boundary during panic processing, abort before argument
	// destruction. Context.Panic is required to abort inside the callback.
	if panicValue != nil && ambient.ProcessingPanic {
		os.Exit(134)
	}
	if ambient.ProcessingPanic {
		// A normal-path Close can itself raise a raw Go panic. Catch it here
		// before the next component's cleanup gets a chance to run.
		defer func() {
			if recover() != nil {
				os.Exit(134)
			}
		}()
	}
	context := ambient
	context.Unwinding = context.Unwinding || panicValue != nil
	close := func() {
		if component.CloseWithContext != nil {
			component.CloseWithContext(&component.Value, context)
		} else if component.Close != nil {
			component.Close(&component.Value)
		}
	}
	if panicValue != nil {
		// A second panic escaping a destructor during unwinding aborts Rust.
		// As in index storage, exit134 represents that unrecoverable class;
		// this does not promise a SIGABRT ABI or core-dump behavior.
		func() {
			defer func() {
				if recover() != nil {
					context.AbortCleanup()
				}
			}()
			close()
		}()
		context.ResumeUnwind(panicValue)
	}
	close()
}

func NewFingerprintOwned[A, B any](first OwnedFingerprintComponent[A], second OwnedFingerprintComponent[B]) Fingerprint {
	return NewFingerprintOwnedWithContext(FingerprintDropContext{}, first, second)
}

// NewFingerprintOwnedWithContext preserves inherited thread-panicking state.
// An ambient unwind alone does not abort a newly raised panic: a caller can
// catch it inside its destructor. A second panic escaping this constructor's
// local argument cleanup remains unrecoverable. Translated panic sites use
// context.Panic to distinguish panic-message processing from ordinary unwind.
// Raw Go callback-local defers cannot be intercepted or undone; an escaping
// processing-mode panic aborts at the component boundary before argument cleanup.
func NewFingerprintOwnedWithContext[A, B any](context FingerprintDropContext, first OwnedFingerprintComponent[A], second OwnedFingerprintComponent[B]) Fingerprint {
	defer deferFingerprintComponent(&first, context)
	defer deferFingerprintComponent(&second, context)
	convertFirst := func() uint64 {
		if first.AsU64WithContext != nil {
			return first.AsU64WithContext(&first.Value, context)
		}
		return first.AsU64(&first.Value)
	}
	convertSecond := func() uint64 {
		if second.AsU64WithContext != nil {
			return second.AsU64WithContext(&second.Value, context)
		}
		return second.AsU64(&second.Value)
	}
	return Fingerprint{convertFirst(), convertSecond()}
}

func (value Fingerprint) ToSmallerHash() rustchashes.Hash64 {
	return rustchashes.NewHash64(value.first*3 + value.second)
}
func (value Fingerprint) Split() (rustchashes.Hash64, rustchashes.Hash64) {
	return rustchashes.NewHash64(value.first), rustchashes.NewHash64(value.second)
}
func (value Fingerprint) Combine(other Fingerprint) Fingerprint {
	return Fingerprint{value.first*3 + other.first, value.second*3 + other.second}
}
func (value Fingerprint) asU128() rustchashes.Uint128 {
	return rustchashes.Uint128{Lo: value.first, Hi: value.second}
}
func (value Fingerprint) CombineCommutative(other Fingerprint) Fingerprint {
	first, carry := bits.Add64(value.first, other.first, 0)
	second, _ := bits.Add64(value.second, other.second, carry)
	return Fingerprint{first, second}
}
func (value Fingerprint) ToLEBytes() [16]byte {
	var result [16]byte
	binary.LittleEndian.PutUint64(result[:8], value.first)
	binary.LittleEndian.PutUint64(result[8:], value.second)
	return result
}
func FingerprintFromLEBytes(bytes [16]byte) Fingerprint {
	return Fingerprint{binary.LittleEndian.Uint64(bytes[:8]), binary.LittleEndian.Uint64(bytes[8:])}
}
func (value Fingerprint) Clone() Fingerprint           { return value }
func (value *Fingerprint) CloneFrom(other Fingerprint) { *value = other }
func (value Fingerprint) Equal(other Fingerprint) bool { return value == other }
func (value Fingerprint) Compare(other Fingerprint) int {
	if value.first < other.first {
		return -1
	}
	if value.first > other.first {
		return 1
	}
	if value.second < other.second {
		return -1
	}
	if value.second > other.second {
		return 1
	}
	return 0
}

// Hash preserves the private source trait's concrete Unhasher specialization.
// Embedding or wrapping Unhasher in another type selects the generic two-write
// implementation, just as a distinct Rust Hasher implementation does.
func (value Fingerprint) Hash(hasher rustchashes.U64Hasher) {
	if unhash, ok := hasher.(*Unhasher); ok {
		unhash.WriteU64(value.first + value.second)
		return
	}
	hasher.WriteU64(value.first)
	hasher.WriteU64(value.second)
}
func FingerprintFromStableHash(hash rustchashes.StableHasherHash) Fingerprint {
	return Fingerprint{hash[0], hash[1]}
}

// StableHash and StableCompare are the exact trivial-type macro's operations.
// The caller supplies the real algorithm and existing state. Context is unused.
func (value Fingerprint) StableHash(context any, hasher rustchashes.U64Hasher) {
	// StableHasher is a concrete source type, so this is always the generic
	// two-write path, even if a caller supplies an Unhasher as a Go witness.
	hasher.WriteU64(value.first)
	hasher.WriteU64(value.second)
}

const FingerprintCanUseUnstableSort = true

// StableOrdTripleChecked represents the macro's associated unit marker. It
// supplies a proof marker only, with no context or hashing operation.
func (value Fingerprint) StableOrdTripleChecked() struct{} { return struct{}{} }

func (value Fingerprint) StableCompare(other Fingerprint) int { return value.Compare(other) }

// These are the precise operations needed from the full codec traits. Callback
// side effects and panics remain caller-owned; the port does not use varints.
// EmitRawBytes receives a temporary borrow; copy the bytes to retain them.
type FingerprintEncoder interface{ EmitRawBytes([]byte) }
type FingerprintDecoder interface{ ReadRawBytes(int) []byte }

func (value Fingerprint) Encode(encoder FingerprintEncoder) {
	bytes := value.ToLEBytes()
	encoder.EmitRawBytes(bytes[:])
}
func DecodeFingerprint(decoder FingerprintDecoder) Fingerprint {
	return DecodeFingerprintWithContext(FingerprintDropContext{}, decoder)
}
func DecodeFingerprintWithContext(context FingerprintDropContext, decoder FingerprintDecoder) Fingerprint {
	bytes := decoder.ReadRawBytes(16)
	if len(bytes) != 16 {
		context.Panic("called `Result::unwrap()` on an `Err` value: TryFromSliceError(())")
	}
	var array [16]byte
	copy(array[:], bytes)
	return FingerprintFromLEBytes(array)
}

// PackedFingerprint has size16 and alignment1 in Go. Native-endian private
// storage represents the source's packed pair; codecs still use little endian.
// There are no exposed references to possibly unaligned integer fields.
type PackedFingerprint struct{ bytes [16]byte }

func PackFingerprint(value Fingerprint) PackedFingerprint {
	var packed PackedFingerprint
	binary.NativeEndian.PutUint64(packed.bytes[:8], value.first)
	binary.NativeEndian.PutUint64(packed.bytes[8:], value.second)
	return packed
}
func (value PackedFingerprint) Fingerprint() Fingerprint {
	return Fingerprint{binary.NativeEndian.Uint64(value.bytes[:8]), binary.NativeEndian.Uint64(value.bytes[8:])}
}
func (value PackedFingerprint) Clone() PackedFingerprint           { return value }
func (value *PackedFingerprint) CloneFrom(other PackedFingerprint) { *value = other }
func (value PackedFingerprint) Equal(other PackedFingerprint) bool { return value == other }
func (value PackedFingerprint) Compare(other PackedFingerprint) int {
	return value.Fingerprint().Compare(other.Fingerprint())
}
func (value PackedFingerprint) Hash(hasher rustchashes.U64Hasher) { value.Fingerprint().Hash(hasher) }
func (value PackedFingerprint) Encode(encoder FingerprintEncoder) {
	value.Fingerprint().Encode(encoder)
}
func DecodePackedFingerprint(decoder FingerprintDecoder) PackedFingerprint {
	return PackFingerprint(DecodeFingerprint(decoder))
}

func DecodePackedFingerprintWithContext(context FingerprintDropContext, decoder FingerprintDecoder) PackedFingerprint {
	return PackFingerprint(DecodeFingerprintWithContext(context, decoder))
}
