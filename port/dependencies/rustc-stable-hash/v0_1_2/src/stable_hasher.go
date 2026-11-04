// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: rustc-stable-hash 0.1.2 src/stable_hasher.rs. Trait witnesses retain
// arbitrary non-Copy state/outputs and individual default-method dispatch.
package stablehash

import (
	"encoding/binary"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"strconv"
	"unicode/utf8"
)

// ExtendedHasherOps is the source ExtendedHasher/Hasher trait dictionary.
// A Ref always addresses the original state cell. A nil optional method uses
// the actual source default; required operations and trait bounds need witnesses.
// Dictionaries are static operations for H, not additional owned closures:
// destructible captures belong in H and its complete Drop witness. Ref values
// are temporary source borrows. Write/typed/ShortWrite callbacks borrow H
// exclusively; Finish64/Clone/Debug borrow it shared (with source interior
// mutation allowed). Write's byte payload is a temporary shared, read-only
// borrow. ShortWrite instead receives an independent owned, trivial-drop array
// representation: its override may mutate it or move it into H for retention.
// The default short-write forwarding only borrows that array for Write.
// Callbacks must preserve those alias/lifetime rules across Move, Take and Drop.
type ExtendedHasherOps[H, Hash any] struct {
	Write             func(Ref[H], []byte)
	Finish64          func(Ref[H]) uint64
	Finish            func(*Owned[H]) *Owned[Hash]
	ShortWrite        func(Ref[H], uint, []byte)
	Default           func(DropContext) H
	Clone             func(Ref[H]) H
	Drop              func(*H, DropContext)
	Debug             func(Ref[H], *rustfmt.Formatter) error
	WriteString       func(Ref[H], string)
	WriteLengthPrefix func(Ref[H], uint)
	WriteU8           func(Ref[H], uint8)
	WriteU16          func(Ref[H], uint16)
	WriteU32          func(Ref[H], uint32)
	WriteU64          func(Ref[H], uint64)
	WriteU128         func(Ref[H], UInt128)
	WriteUsize        func(Ref[H], uint)
	WriteI8           func(Ref[H], int8)
	WriteI16          func(Ref[H], int16)
	WriteI32          func(Ref[H], int32)
	WriteI64          func(Ref[H], int64)
	WriteI128         func(Ref[H], Int128)
	WriteIsize        func(Ref[H], int)
}
type StableHasher[H, Hash any] struct {
	state   *Owned[H]
	ops     ExtendedHasherOps[H, Hash]
	context DropContext
}

func WithHasher[H, Hash any](value H, ops ExtendedHasherOps[H, Hash]) *StableHasher[H, Hash] {
	return WithOwnedHasher(NewOwned(value, ValueOps[H]{Drop: ops.Drop}), ops)
}
func WithOwnedHasher[H, Hash any](value *Owned[H], ops ExtendedHasherOps[H, Hash]) *StableHasher[H, Hash] {
	if ops.Write == nil || ops.Finish64 == nil || ops.Finish == nil {
		panic("stablehash: missing required ExtendedHasher trait witness")
	}
	return &StableHasher[H, Hash]{state: value.Move(), ops: ops, context: value.Context()}
}
func NewStableHasher[H, Hash any](ops ExtendedHasherOps[H, Hash]) *StableHasher[H, Hash] {
	return NewStableHasherWithContext(ops, DropContext{})
}
func NewStableHasherWithContext[H, Hash any](ops ExtendedHasherOps[H, Hash], context DropContext) *StableHasher[H, Hash] {
	if ops.Default == nil {
		context.Panic("stablehash: missing Default trait witness")
	}
	var value H
	runContext(context, func() { value = ops.Default(context) })
	h := WithHasher(value, ops)
	h.context = context
	return h
}
func (h *StableHasher[H, Hash]) SetContext(context DropContext) { h.context = context }
func (h *StableHasher[H, Hash]) ref() Ref[H] {
	if h == nil || !h.state.IsLive() {
		h.context.Panic("stablehash: use of consumed StableHasher")
	}
	return Ref[H]{Value: h.state.Borrow(), Context: h.context}
}
func (h *StableHasher[H, Hash]) Close() { h.CloseWithContext(h.context) }
func (h *StableHasher[H, Hash]) CloseWithContext(context DropContext) {
	h.state.DropWithContext(context)
}
func (h *StableHasher[H, Hash]) Clone() *StableHasher[H, Hash] { return h.cloneWithContext(h.context) }
func (h *StableHasher[H, Hash]) cloneWithContext(context DropContext) *StableHasher[H, Hash] {
	if h.ops.Clone == nil {
		context.Panic("stablehash: missing Clone trait witness")
	}
	ref := h.ref()
	ref.Context = context
	var value H
	runContext(context, func() { value = h.ops.Clone(ref) })
	clone := WithHasher(value, h.ops)
	clone.context = context
	return clone
}
func (h *StableHasher[H, Hash]) CloneFrom(source *StableHasher[H, Hash]) {
	h.ref()
	if h == source {
		h.context.Panic("stablehash: aliased mutable and shared CloneFrom source")
	}
	clone := source.cloneWithContext(h.context)
	defer func() { h.ops = source.ops }()
	replaceOwned(h.state, clone.state, h.context)
}
func (h *StableHasher[H, Hash]) Finish64() uint64 {
	ref := h.ref()
	var value uint64
	runContext(h.context, func() { value = h.ops.Finish64(ref) })
	return value
}
func (h *StableHasher[H, Hash]) Write(bytes []byte) {
	ref := h.ref()
	runContext(h.context, func() { h.ops.Write(ref, bytes) })
}
func (h *StableHasher[H, Hash]) shortWrite(length uint, bytes []byte) {
	if uint(len(bytes)) != length {
		h.context.Panic("stablehash: const LEN differs from array length")
	}
	ref := h.ref()
	copyOfArray := append([]byte(nil), bytes...)
	runContext(h.context, func() {
		if h.ops.ShortWrite != nil {
			h.ops.ShortWrite(ref, length, copyOfArray)
		} else {
			h.ops.Write(ref, copyOfArray)
		}
	})
}
func (h *StableHasher[H, Hash]) FinishOwned() *Owned[Hash] {
	h.ref()
	arg := h.state.MoveWithContext(h.context)
	return callOwned(arg, h.context, h.ops.Finish)
}

// FinishHash transfers the associated result as a plain Go value; callers that
// need nontrivial result destruction use FinishOwned/FinishInto instead.
func (h *StableHasher[H, Hash]) FinishHash() Hash { return h.FinishOwned().Take() }

type FromStableHash[Hash, W any] struct{ From func(*Owned[Hash]) *Owned[W] }

func FinishInto[H, Hash, W any](h *StableHasher[H, Hash], convert FromStableHash[Hash, W]) *Owned[W] {
	if convert.From == nil {
		h.context.Panic("stablehash: missing FromStableHash trait witness")
	}
	result := h.FinishOwned()
	return callOwned(result, h.context, convert.From)
}
func (h *StableHasher[H, Hash]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	if h.ops.Debug == nil {
		f.Context.Panic("stablehash: missing Debug trait witness")
	}
	ref := h.ref()
	ref.Context = f.Context
	child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}
	var err error
	runContext(f.Context, func() { err = h.ops.Debug(ref, child) })
	return err
}
func (h *StableHasher[H, Hash]) RustDebug() string { return rustfmt.String(h.WriteDebugFormatter) }
func (h *StableHasher[H, Hash]) WriteU8(v uint8)   { h.stateU8(v) }
func (h *StableHasher[H, Hash]) WriteU16(v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	h.shortWrite(2, b[:])
}
func (h *StableHasher[H, Hash]) WriteU32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	h.shortWrite(4, b[:])
}
func (h *StableHasher[H, Hash]) WriteU64(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	h.shortWrite(8, b[:])
}
func (h *StableHasher[H, Hash]) WriteU128(v UInt128) { h.WriteU64(v[0]); h.WriteU64(v[1]) }
func (h *StableHasher[H, Hash]) WriteUsize(v uint) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	h.shortWrite(8, b[:])
}
func (h *StableHasher[H, Hash]) WriteI8(v int8) { h.stateI8(v) }
func (h *StableHasher[H, Hash]) WriteI16(v int16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], uint16(v))
	h.shortWrite(2, b[:])
}
func (h *StableHasher[H, Hash]) WriteI32(v int32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(v))
	h.shortWrite(4, b[:])
}
func (h *StableHasher[H, Hash]) WriteI64(v int64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(v))
	h.shortWrite(8, b[:])
}
func (h *StableHasher[H, Hash]) WriteI128(v Int128) {
	var b [16]byte
	binary.LittleEndian.PutUint64(b[:8], v[0])
	binary.LittleEndian.PutUint64(b[8:], v[1])
	h.Write(b[:])
}
func (h *StableHasher[H, Hash]) WriteIsize(v int) {
	value := uint64(v)
	if value < 255 {
		h.stateU8(uint8(value))
	} else {
		h.stateU8(255)
		var b [8]byte
		binary.LittleEndian.PutUint64(b[:], value)
		h.shortWrite(8, b[:])
	}
}
func (h *StableHasher[H, Hash]) stateU8(v uint8)      { h.ops.Bind(h.ref()).WriteU8(v) }
func (h *StableHasher[H, Hash]) stateU16(v uint16)    { h.ops.Bind(h.ref()).WriteU16(v) }
func (h *StableHasher[H, Hash]) stateU32(v uint32)    { h.ops.Bind(h.ref()).WriteU32(v) }
func (h *StableHasher[H, Hash]) stateU64(v uint64)    { h.ops.Bind(h.ref()).WriteU64(v) }
func (h *StableHasher[H, Hash]) stateU128(v UInt128)  { h.ops.Bind(h.ref()).WriteU128(v) }
func (h *StableHasher[H, Hash]) stateUsize(v uint)    { h.ops.Bind(h.ref()).WriteUsize(v) }
func (h *StableHasher[H, Hash]) stateI8(v int8)       { h.ops.Bind(h.ref()).WriteI8(v) }
func (h *StableHasher[H, Hash]) stateI16(v int16)     { h.ops.Bind(h.ref()).WriteI16(v) }
func (h *StableHasher[H, Hash]) stateI32(v int32)     { h.ops.Bind(h.ref()).WriteI32(v) }
func (h *StableHasher[H, Hash]) stateI64(v int64)     { h.ops.Bind(h.ref()).WriteI64(v) }
func (h *StableHasher[H, Hash]) stateI128(v Int128)   { h.ops.Bind(h.ref()).WriteI128(v) }
func (h *StableHasher[H, Hash]) stateIsize(v int)     { h.ops.Bind(h.ref()).WriteIsize(v) }
func (h *StableHasher[H, Hash]) stateString(v string) { h.ops.Bind(h.ref()).WriteString(v) }

// HasherRef exposes the full required/default Hasher operations of an H state
// through the trait dictionary, without constructing a StableHasher or copying
// the borrowed state. The caller retains the source borrow lifetime contract.
type HasherRef[H, Hash any] struct {
	Ref Ref[H]
	Ops ExtendedHasherOps[H, Hash]
}

func (ops ExtendedHasherOps[H, Hash]) Bind(ref Ref[H]) HasherRef[H, Hash] {
	if ref.Value == nil || ops.Write == nil || ops.Finish64 == nil {
		ref.Context.Panic("stablehash: invalid borrowed Hasher witness")
	}
	return HasherRef[H, Hash]{ref, ops}
}
func (h HasherRef[H, Hash]) Write(bytes []byte) {
	runContext(h.Ref.Context, func() { h.Ops.Write(h.Ref, bytes) })
}
func (h HasherRef[H, Hash]) Finish64() uint64 {
	var result uint64
	runContext(h.Ref.Context, func() { result = h.Ops.Finish64(h.Ref) })
	return result
}
func (h HasherRef[H, Hash]) WriteLengthPrefix(n uint) {
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteLengthPrefix != nil {
			h.Ops.WriteLengthPrefix(h.Ref, n)
		} else {
			h.WriteUsize(n)
		}
	})
}
func (ops ExtendedHasherOps[H, Hash]) ShortWriteArray(ref Ref[H], length uint, bytes []byte) {
	if length != uint(len(bytes)) {
		ref.Context.Panic("stablehash: const LEN differs from array length")
	}
	array := append([]byte(nil), bytes...)
	runContext(ref.Context, func() {
		if ops.ShortWrite != nil {
			ops.ShortWrite(ref, length, array)
		} else {
			if ops.Write == nil {
				ref.Context.Panic("stablehash: missing Hasher write witness")
			}
			ops.Write(ref, array)
		}
	})
}
func FinishExtended[H, Hash any](value *Owned[H], ops ExtendedHasherOps[H, Hash]) *Owned[Hash] {
	if ops.Finish == nil {
		value.Context().Panic("stablehash: missing ExtendedHasher finish witness")
	}
	context := value.Context()
	return callOwned(value.MoveWithContext(context), context, ops.Finish)
}
func (h HasherRef[H, Hash]) WriteU8(v uint8) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteU8 != nil {
			h.Ops.WriteU8(ref, v)
			return
		}
		h.Ops.Write(ref, []byte{v})
	})
}
func (h HasherRef[H, Hash]) WriteU16(v uint16) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteU16 != nil {
			h.Ops.WriteU16(ref, v)
			return
		}
		var b [2]byte
		binary.NativeEndian.PutUint16(b[:], v)
		h.Ops.Write(ref, b[:])
	})
}
func (h HasherRef[H, Hash]) WriteU32(v uint32) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteU32 != nil {
			h.Ops.WriteU32(ref, v)
			return
		}
		var b [4]byte
		binary.NativeEndian.PutUint32(b[:], v)
		h.Ops.Write(ref, b[:])
	})
}
func (h HasherRef[H, Hash]) WriteU64(v uint64) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteU64 != nil {
			h.Ops.WriteU64(ref, v)
			return
		}
		var b [8]byte
		binary.NativeEndian.PutUint64(b[:], v)
		h.Ops.Write(ref, b[:])
	})
}
func (h HasherRef[H, Hash]) WriteU128(v UInt128) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteU128 != nil {
			h.Ops.WriteU128(ref, v)
			return
		}
		b := native128(v)
		h.Ops.Write(ref, b[:])
	})
}
func (h HasherRef[H, Hash]) WriteUsize(v uint) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteUsize != nil {
			h.Ops.WriteUsize(ref, v)
			return
		}
		if strconv.IntSize == 64 {
			var b [8]byte
			binary.NativeEndian.PutUint64(b[:], uint64(v))
			h.Ops.Write(ref, b[:])
		} else {
			var b [4]byte
			binary.NativeEndian.PutUint32(b[:], uint32(v))
			h.Ops.Write(ref, b[:])
		}
	})
}
func (h HasherRef[H, Hash]) WriteI8(v int8) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteI8 != nil {
			h.Ops.WriteI8(ref, v)
			return
		}
		h.WriteU8(uint8(v))
	})
}
func (h HasherRef[H, Hash]) WriteI16(v int16) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteI16 != nil {
			h.Ops.WriteI16(ref, v)
			return
		}
		h.WriteU16(uint16(v))
	})
}
func (h HasherRef[H, Hash]) WriteI32(v int32) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteI32 != nil {
			h.Ops.WriteI32(ref, v)
			return
		}
		h.WriteU32(uint32(v))
	})
}
func (h HasherRef[H, Hash]) WriteI64(v int64) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteI64 != nil {
			h.Ops.WriteI64(ref, v)
			return
		}
		h.WriteU64(uint64(v))
	})
}
func (h HasherRef[H, Hash]) WriteI128(v Int128) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteI128 != nil {
			h.Ops.WriteI128(ref, v)
			return
		}
		h.WriteU128(UInt128(v))
	})
}
func (h HasherRef[H, Hash]) WriteIsize(v int) {
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteIsize != nil {
			h.Ops.WriteIsize(ref, v)
			return
		}
		h.WriteUsize(uint(v))
	})
}
func (h HasherRef[H, Hash]) WriteString(value string) {
	if !utf8.ValidString(value) {
		h.Ref.Context.Panic("stablehash: Rust str requires valid UTF-8")
	}
	ref := h.Ref
	runContext(h.Ref.Context, func() {
		if h.Ops.WriteString != nil {
			h.Ops.WriteString(ref, value)
		} else {
			h.Ops.Write(ref, []byte(value))
			h.WriteU8(255)
		}
	})
}

type StableSipHasher128 = StableHasher[*SipHasher128, SipHasher128Hash]

func NewStableSipHasher128() *StableSipHasher128 {
	return NewStableSipHasher128WithProfile(DefaultProfile())
}
func NewStableSipHasher128WithProfile(profile ArithmeticProfile) *StableSipHasher128 {
	return NewStableHasher(sipOps(profile))
}
func NewStableSipHasher128WithKeys(k0, k1 uint64) *StableSipHasher128 {
	return WithHasher(NewSipHasher128WithKeys(k0, k1), sipOps(DefaultProfile()))
}
func sipOps(profile ArithmeticProfile) ExtendedHasherOps[*SipHasher128, SipHasher128Hash] {
	return ExtendedHasherOps[*SipHasher128, SipHasher128Hash]{
		Write: func(ref Ref[*SipHasher128], bytes []byte) {
			(*ref.Value).SetContext(ref.Context)
			(*ref.Value).Write(bytes)
		},
		Finish64: func(ref Ref[*SipHasher128]) uint64 {
			(*ref.Value).SetContext(ref.Context)
			return (*ref.Value).Finish64()
		},
		Finish: func(value *Owned[*SipHasher128]) *Owned[SipHasher128Hash] {
			state := *value.Borrow()
			state.SetContext(value.Context())
			return Own(state.Finish128())
		},
		ShortWrite: func(ref Ref[*SipHasher128], length uint, bytes []byte) {
			(*ref.Value).SetContext(ref.Context)
			(*ref.Value).ShortWrite(length, bytes)
		},
		Default: func(context DropContext) *SipHasher128 { return NewSipHasher128WithProfile(0, 0, profile) },
		Clone: func(ref Ref[*SipHasher128]) *SipHasher128 {
			(*ref.Value).SetContext(ref.Context)
			return (*ref.Value).Clone()
		},
		Debug:   func(ref Ref[*SipHasher128], f *rustfmt.Formatter) error { return (*ref.Value).WriteDebugFormatter(f) },
		WriteU8: func(ref Ref[*SipHasher128], v uint8) { (*ref.Value).SetContext(ref.Context); (*ref.Value).WriteU8(v) },
		WriteI8: func(ref Ref[*SipHasher128], v int8) { (*ref.Value).SetContext(ref.Context); (*ref.Value).WriteI8(v) },
		WriteString: func(ref Ref[*SipHasher128], value string) {
			(*ref.Value).SetContext(ref.Context)
			HashString(*ref.Value, value)
		},
	}
}
