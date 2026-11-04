// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete owned ToBaseN/default-method adapters for the pinned base_n.rs.
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"unsafe"
)

// BaseNConfig represents the source's ordinary arithmetic-check selection.
// There are no debug-only assertions in base_n.rs; assert_eq! stays unconditional.
type BaseNConfig struct{ OverflowChecks bool }

func DefaultBaseNConfig() BaseNConfig { return BaseNConfig{OverflowChecks: true} }

func BaseNU32OwnedTrait(config BaseNConfig) BaseNTrait[uint32] {
	return BaseNTrait[uint32]{
		Into128: func(value *rustvec.Owned[uint32], _ rustfmt.Context) BaseNU128 {
			return BaseNU128{Lo: uint64(value.Take())}
		},
		EncodedLen: func(base uint, context rustfmt.Context) uint {
			return BaseNEncodedLen32WithContext(base, config, context)
		},
	}
}
func BaseNU64OwnedTrait(config BaseNConfig) BaseNTrait[uint64] {
	return BaseNTrait[uint64]{
		Into128: func(value *rustvec.Owned[uint64], _ rustfmt.Context) BaseNU128 { return BaseNU128{Lo: value.Take()} },
		EncodedLen: func(base uint, context rustfmt.Context) uint {
			return BaseNEncodedLen64WithContext(base, config, context)
		},
	}
}
func BaseNU128OwnedTrait(config BaseNConfig) BaseNTrait[BaseNU128] {
	return BaseNTrait[BaseNU128]{
		Into128: func(value *rustvec.Owned[BaseNU128], _ rustfmt.Context) BaseNU128 { return value.Take() },
		EncodedLen: func(base uint, context rustfmt.Context) uint {
			return BaseNEncodedLen128WithContext(base, config, context)
		},
	}
}

func baseNIncrementLength(length *uint, config BaseNConfig, context rustfmt.Context) {
	if config.OverflowChecks && *length == ^uint(0) {
		context.Panic("attempt to add with overflow")
	}
	*length++
}
func BaseNEncodedLen32WithContext(base uint, config BaseNConfig, context rustfmt.Context) (length uint) {
	max := ^uint32(0)
	for max > 0 {
		baseNIncrementLength(&length, config, context)
		divisor := uint32(base)
		if divisor == 0 {
			context.Panic("attempt to divide by zero")
		}
		max /= divisor
	}
	return
}
func BaseNEncodedLen64WithContext(base uint, config BaseNConfig, context rustfmt.Context) (length uint) {
	max := ^uint64(0)
	for max > 0 {
		baseNIncrementLength(&length, config, context)
		if base == 0 {
			context.Panic("attempt to divide by zero")
		}
		max /= uint64(base)
	}
	return
}
func BaseNEncodedLen128WithContext(base uint, config BaseNConfig, context rustfmt.Context) (length uint) {
	max := BaseNU128{Hi: ^uint64(0), Lo: ^uint64(0)}
	for max.Hi != 0 || max.Lo != 0 {
		baseNIncrementLength(&length, config, context)
		if base == 0 {
			context.Panic("attempt to divide by zero")
		}
		max, _ = baseNDiv128(max, uint64(base))
	}
	return
}

// BaseNTrait is static ToBaseN/Into<u128> implementation metadata. EncodedLen
// is an associated function: it does not receive or borrow the consumed value.
// Into128 is required; the two optional method slots select genuine overrides.
// Operations receive the actual owned argument cell; full Drop/field glue is
// supplied by that argument's rustvec.ValueOps, not by dictionary captures.
// Ownership transfers into the selected operation; the caller must not reuse
// its input handle. A callback may take/move the argument, or leave it live for
// source parameter cleanup before return (including cleanup on panic). Borrows
// of the argument cell must obey the source operation's lifetime.
//
// Translated Rust panic sites use the supplied Context.Panic. Raw Go panic/defer
// callbacks retain their own local Go cleanup timing; the owned guard can only
// enforce source cleanup/abort when such a panic escapes that callback. Context
// is an explicit operation guard, not an implicit current-caller lookup. If a
// callback stores an owned handle for later, its later scope must pass its fresh
// context to DropWithContext or MoveWithContext. Runtime metadata is borrowed.
type BaseNTrait[T any] struct {
	Into128        func(*rustvec.Owned[T], rustfmt.Context) BaseNU128
	EncodedLen     func(uint, rustfmt.Context) uint
	ToBase         func(*rustvec.Owned[T], uint, rustfmt.Context) BaseNString
	ToBaseFixedLen func(*rustvec.Owned[T], uint, rustfmt.Context) BaseNString
}

func BaseNToBaseOwned[T any](value *rustvec.Owned[T], base uint, implementation BaseNTrait[T]) BaseNString {
	return BaseNToBaseOwnedWithContext(value, base, implementation, DefaultBaseNConfig(), rustfmt.Context{})
}
func BaseNToBaseOwnedWithContext[T any](value *rustvec.Owned[T], base uint, implementation BaseNTrait[T], config BaseNConfig, context rustfmt.Context) BaseNString {
	return rustvec.ConsumeValue(value, context, func(value *rustvec.Owned[T]) BaseNString {
		if implementation.ToBase != nil {
			return implementation.ToBase(value, base, context)
		}
		n := rustvec.ConsumeValue(value, context, func(value *rustvec.Owned[T]) BaseNU128 { return implementation.Into128(value, context) })
		return baseNEncodeWithContext(n, base, config, context)
	})
}

func BaseNToBaseFixedLenOwned[T any](value *rustvec.Owned[T], base uint, implementation BaseNTrait[T]) BaseNString {
	return BaseNToBaseFixedLenOwnedWithContext(value, base, implementation, DefaultBaseNConfig(), rustfmt.Context{})
}
func BaseNToBaseFixedLenOwnedWithContext[T any](value *rustvec.Owned[T], base uint, implementation BaseNTrait[T], config BaseNConfig, context rustfmt.Context) BaseNString {
	return rustvec.ConsumeValue(value, context, func(value *rustvec.Owned[T]) BaseNString {
		if implementation.ToBaseFixedLen != nil {
			return implementation.ToBaseFixedLen(value, base, context)
		}
		encoded := BaseNToBaseOwnedWithContext(value, base, implementation, config, context)
		width := implementation.EncodedLen(base, context)
		if config.OverflowChecks && width > uint(len(encoded.buf)) {
			context.Panic("attempt to subtract with overflow")
		}
		encoded.start = uint(len(encoded.buf)) - width
		return encoded
	})
}

func baseNEncodeWithContext(n BaseNU128, base uint, config BaseNConfig, context rustfmt.Context) BaseNString {
	encoded := BaseNString{}
	for i := range encoded.buf {
		encoded.buf[i] = '0'
	}
	index := uint(len(encoded.buf))
	for {
		if config.OverflowChecks && index == 0 {
			context.Panic("attempt to subtract with overflow")
		}
		index--
		if base == 0 {
			context.Panic("attempt to calculate remainder with a divisor of zero")
		}
		quotient, digit := baseNDiv128(n, uint64(base))
		if digit >= uint64(len(baseNDigits)) {
			context.Panic("base_n: digit index out of bounds")
		}
		if index >= uint(len(encoded.buf)) {
			context.Panic("base_n: output index out of bounds")
		}
		encoded.buf[index] = baseNDigits[digit]
		n = quotient
		if n.Hi == 0 && n.Lo == 0 {
			break
		}
	}
	encoded.start = index
	return encoded
}

// Deref and AsRefBorrowed are the source's actual shared str views. They alias
// this object's private ASCII buffer rather than a copied receiver or string.
// Keep the object in place and do not mutate it while these borrows are used.
// String/AsRef remain independently owned Go-string conveniences.
func (value *BaseNString) Deref() string         { return value.DerefWithContext(rustfmt.Context{}) }
func (value *BaseNString) AsRefBorrowed() string { return value.Deref() }
func (value *BaseNString) AsRefBorrowedWithContext(context rustfmt.Context) string {
	return value.DerefWithContext(context)
}
func (value *BaseNString) DerefWithContext(context rustfmt.Context) string {
	if value.start > uint(len(value.buf)) {
		context.Panic("base_n: string start out of bounds")
	}
	data := (*byte)(unsafe.Add(unsafe.Pointer(&value.buf[0]), value.start))
	return unsafe.String(data, len(value.buf)-int(value.start))
}

// WriteDisplayFormatter is precisely f.write_str(self). All options and caller
// state pass through unchanged; even an empty suffix produces a string write.
func (value *BaseNString) WriteDisplayFormatter(formatter *rustfmt.Formatter) error {
	return formatter.WriteString(value.DerefWithContext(formatter.Context))
}
