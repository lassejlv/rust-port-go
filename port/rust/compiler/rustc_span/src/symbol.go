// SPDX-License-Identifier: MIT OR Apache-2.0
// Port of compiler/rustc_span/src/symbol.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. The symbols! invocation's data is
// generated in symbol_predefined.go; identifier behavior is in symbol_ident.go.
package rustcspan

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const SymbolIndexMaxAsU32 uint32 = 0xffffff00

// Symbol and ByteSymbol are indices into their originating session's shared
// immutable interner. Assignment implements Copy. As in Rust, mixing sessions
// can give an unrelated value; these IDs do not carry an extra session tag.
type Symbol struct{ value uint32 }
type ByteSymbol struct{ value uint32 }

func NewSymbol(index uint32) Symbol { return NewSymbolWithContext(index, rustfmt.Context{}) }
func NewSymbolWithContext(index uint32, context rustfmt.Context) Symbol {
	if index > SymbolIndexMaxAsU32 {
		symbolStaticPanic(context, PanicSymbolIndexSite, "assertion failed: value <= 0xFFFF_FF00")
	}
	return Symbol{value: index}
}
func NewByteSymbol(index uint32) ByteSymbol { return ByteSymbol{value: NewSymbol(index).value} }
func NewByteSymbolWithContext(index uint32, context rustfmt.Context) ByteSymbol {
	return ByteSymbol{value: NewSymbolWithContext(index, context).value}
}
func (value Symbol) AsU32() uint32                           { return value.value }
func (value ByteSymbol) AsU32() uint32                       { return value.value }
func (value Symbol) Clone() Symbol                           { return value }
func (value ByteSymbol) Clone() ByteSymbol                   { return value }
func (value *Symbol) CloneFrom(other Symbol)                 { *value = other }
func (value *ByteSymbol) CloneFrom(other ByteSymbol)         { *value = other }
func (value Symbol) Equal(other Symbol) bool                 { return value == other }
func (value ByteSymbol) Equal(other ByteSymbol) bool         { return value == other }
func (value Symbol) NotEqual(other Symbol) bool              { return !value.Equal(other) }
func (value ByteSymbol) NotEqual(other ByteSymbol) bool      { return !value.Equal(other) }
func (value Symbol) Less(other Symbol) bool                  { return value.Compare(other) < 0 }
func (value Symbol) LessEqual(other Symbol) bool             { return value.Compare(other) <= 0 }
func (value Symbol) Greater(other Symbol) bool               { return value.Compare(other) > 0 }
func (value Symbol) GreaterEqual(other Symbol) bool          { return value.Compare(other) >= 0 }
func (value Symbol) PartialCompare(other Symbol) Option[int] { return Some(value.Compare(other)) }
func (value ByteSymbol) Less(other ByteSymbol) bool          { return value.Compare(other) < 0 }
func (value ByteSymbol) LessEqual(other ByteSymbol) bool     { return value.Compare(other) <= 0 }
func (value ByteSymbol) Greater(other ByteSymbol) bool       { return value.Compare(other) > 0 }
func (value ByteSymbol) GreaterEqual(other ByteSymbol) bool  { return value.Compare(other) >= 0 }
func (value ByteSymbol) PartialCompare(other ByteSymbol) Option[int] {
	return Some(value.Compare(other))
}
func (value Symbol) Min(other Symbol) Symbol {
	if value.LessEqual(other) {
		return value
	}
	return other
}
func (value Symbol) Max(other Symbol) Symbol {
	if value.GreaterEqual(other) {
		return value
	}
	return other
}
func (value ByteSymbol) Min(other ByteSymbol) ByteSymbol {
	if value.LessEqual(other) {
		return value
	}
	return other
}
func (value ByteSymbol) Max(other ByteSymbol) ByteSymbol {
	if value.GreaterEqual(other) {
		return value
	}
	return other
}
func (value Symbol) Clamp(minimum, maximum Symbol) Symbol {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value Symbol) ClampWithContext(minimum, maximum Symbol, context rustfmt.Context) Symbol {
	if minimum.Greater(maximum) {
		symbolStaticPanic(context, PanicSymbolClampSite, "assertion failed: min <= max")
	}
	if value.Less(minimum) {
		return minimum
	}
	if value.Greater(maximum) {
		return maximum
	}
	return value
}
func (value ByteSymbol) Clamp(minimum, maximum ByteSymbol) ByteSymbol {
	return value.ClampWithContext(minimum, maximum, rustfmt.Context{})
}
func (value ByteSymbol) ClampWithContext(minimum, maximum ByteSymbol, context rustfmt.Context) ByteSymbol {
	if minimum.Greater(maximum) {
		symbolStaticPanic(context, PanicSymbolClampSite, "assertion failed: min <= max")
	}
	if value.Less(minimum) {
		return minimum
	}
	if value.Greater(maximum) {
		return maximum
	}
	return value
}
func (value Symbol) Compare(other Symbol) int { return compareSymbolIndices(value.value, other.value) }
func (value ByteSymbol) Compare(other ByteSymbol) int {
	return compareSymbolIndices(value.value, other.value)
}
func compareSymbolIndices(a, b uint32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// Hash borrows the caller's actual hasher. Source write_u32 overrides are
// optional: the complete Hasher bridge retains std's native-endian default.
func (value Symbol) Hash(hasher stablehash.Hasher)     { stablehash.HashU32(hasher, value.value) }
func (value ByteSymbol) Hash(hasher stablehash.Hasher) { stablehash.HashU32(hasher, value.value) }

// HashSymbolSlice/HashByteSymbolSlice preserve Hash::hash_slice's default:
// each element hashes in order, with no slice-length prefix or bulk-u32 shortcut.
func HashSymbolSlice(values []Symbol, hasher stablehash.Hasher) {
	for _, value := range values {
		value.Hash(hasher)
	}
}
func HashByteSymbolSlice(values []ByteSymbol, hasher stablehash.Hasher) {
	for _, value := range values {
		value.Hash(hasher)
	}
}

func InternSymbol(session *SessionGlobals, text string) Symbol {
	return InternSymbolWithContext(session, text, rustfmt.Context{})
}
func InternSymbolWithContext(session *SessionGlobals, text string, context rustfmt.Context) Symbol {
	if !utf8.ValidString(text) {
		context.Panic("rustcspan: str must be valid UTF-8")
	}
	return NewSymbolWithContext(symbolSessionInterner(session, context).intern(text, context), context)
}
func InternByteSymbol(session *SessionGlobals, bytes []byte) ByteSymbol {
	return InternByteSymbolWithContext(session, bytes, rustfmt.Context{})
}
func InternByteSymbolWithContext(session *SessionGlobals, bytes []byte, context rustfmt.Context) ByteSymbol {
	return NewByteSymbolWithContext(symbolSessionInterner(session, context).intern(string(bytes), context), context)
}
func (value Symbol) AsStr(session *SessionGlobals) string {
	return value.AsStrWithContext(session, rustfmt.Context{})
}
func (value Symbol) AsStrWithContext(session *SessionGlobals, context rustfmt.Context) string {
	return symbolSessionInterner(session, context).get(value.value, context)
}

// AsByteStr returns an immutable borrowed byte string. A Go string holds every
// byte value, including malformed UTF-8. AsBytes is a copying convenience; no
// API exposes a mutable alias into the shared Symbol/ByteSymbol allocation.
func (value ByteSymbol) AsByteStr(session *SessionGlobals) string {
	return value.AsByteStrWithContext(session, rustfmt.Context{})
}
func (value ByteSymbol) AsByteStrWithContext(session *SessionGlobals, context rustfmt.Context) string {
	return symbolSessionInterner(session, context).get(value.value, context)
}
func (value ByteSymbol) AsBytes(session *SessionGlobals) []byte {
	return []byte(value.AsByteStr(session))
}

// A caller constructing Symbol directly must ensure the referenced entry was
// interned as valid UTF-8, matching the original unchecked UTF-8 precondition.
func symbolSessionInterner(session *SessionGlobals, context rustfmt.Context) *symbolInterner {
	if session == nil || session.symbolInterner == nil {
		symbolStaticPanic(context, PanicSymbolSessionUnsetSite, "cannot access a scoped thread local variable without calling `set` first")
	}
	return session.symbolInterner
}

// symbolInterner ports the single locked indices/byte_strs store. Go's map
// supplies the private hash table and GC retains its immutable arena strings;
// table hash/bucket placement never determines the source's insertion indices.
type symbolInterner struct {
	inner *Lock[symbolInternerInner]
}
type symbolInternerInner struct {
	indices     map[string]uint32
	byteStrings []string
}

func newSymbolInterner(extra []string) *symbolInterner {
	return newSymbolInternerWithContext(extra, rustfmt.Context{})
}
func newSymbolInternerWithContext(extra []string, context rustfmt.Context) *symbolInterner {
	return prefillSymbolInterner(predefinedSymbolStrings[:], extra, context)
}
func prefillSymbolInterner(initial, extra []string, context rustfmt.Context) *symbolInterner {
	capacity := len(initial) + len(extra) + 2400
	inner := symbolInternerInner{indices: make(map[string]uint32, capacity), byteStrings: make([]string, 0, capacity)}
	conflicting := make([]string, 0)
	for _, values := range [2][]string{initial, extra} {
		for _, value := range values {
			if !utf8.ValidString(value) {
				context.Panic("rustcspan: str must be valid UTF-8")
			}
			if index, exists := inner.indices[value]; exists {
				conflicting = append(conflicting, inner.byteStrings[index])
			} else {
				inner.indices[value] = uint32(len(inner.byteStrings))
				inner.byteStrings = append(inner.byteStrings, value)
			}
		}
	}
	if len(conflicting) != 0 {
		symbolOwnedPanic(context, PanicSymbolDuplicatePrefillSite, func(rustfmt.Context) any {
			var text strings.Builder
			text.WriteString("duplicate symbols in the rustc symbol list and the extra symbols added by the driver: ")
			text.WriteString(symbolDebugByteStringList(conflicting))
			return text.String()
		})
	}
	return &symbolInterner{inner: NewLockWithDrop(inner, func(value *symbolInternerInner, _ rustfmt.Context) {
		value.indices = nil
		value.byteStrings = nil
	})}
}
func (interner *symbolInterner) CloseWithContext(context rustfmt.Context) {
	if interner != nil {
		interner.inner.CloseWithContext(context)
	}
}
func (interner *symbolInterner) intern(value string, context rustfmt.Context) uint32 {
	guard := interner.inner.LockWithContext(context)
	defer guard.Close()
	inner := guard.Get()
	if index, exists := inner.indices[value]; exists {
		return index
	}
	// The original vector length is cast to u32 before Symbol::new validates it.
	index := uint32(len(inner.byteStrings))
	// A vacant source entry copies its input into the interner's arena. Keep
	// that allocation distinct from the caller's still-live borrow. Prefill
	// deliberately keeps its static input allocations instead.
	value = strings.Clone(value)
	inner.indices[value] = index
	inner.byteStrings = append(inner.byteStrings, value)
	return index
}
func (interner *symbolInterner) get(index uint32, context rustfmt.Context) string {
	guard := interner.inner.LockWithContext(context)
	defer guard.Close()
	inner := guard.Get()
	if uint64(index) >= uint64(len(inner.byteStrings)) {
		symbolOwnedPanic(context, PanicSymbolInternerBoundsSite, func(rustfmt.Context) any {
			return fmt.Sprintf("index out of bounds: the len is %d but the index is %d", len(inner.byteStrings), index)
		})
	}
	return inner.byteStrings[index]
}

func (value Symbol) IsEmpty() bool   { return value == Sym_empty() }
func (value Symbol) isSpecial() bool { return value.value <= KwUnderscore().value }
func (value Symbol) isUsedKeywordAlways() bool {
	return value.value >= KwAs().value && value.value <= KwWhile().value
}
func (value Symbol) isUnusedKeywordAlways() bool {
	return value.value >= KwAbstract().value && value.value <= KwYield().value
}
func (value Symbol) isUsedKeywordConditional(edition func() Edition) bool {
	return value.value >= KwAsync().value && value.value <= KwDyn().value && edition() >= Edition2018
}
func (value Symbol) isUnusedKeywordConditional(edition func() Edition) bool {
	return value == KwGen() && edition().AtLeastRust2024() || value == KwTry() && edition().AtLeastRust2018()
}

// edition is the source Copy + FnOnce witness. It may observe or mutate state
// through shared references; calls preserve Rust's short-circuit order.
func (value Symbol) IsReserved(edition func() Edition) bool {
	return value.isSpecial() || value.isUsedKeywordAlways() || value.isUnusedKeywordAlways() ||
		value.isUsedKeywordConditional(edition) || value.isUnusedKeywordConditional(edition)
}
func (value Symbol) IsWeak() bool {
	return value.value >= KwAuto().value && value.value <= KwYeet().value
}
func (value Symbol) IsPathSegmentKeyword() bool {
	return value == KwSuper() || value == KwSelfLower() || value == KwSelfUpper() || value == KwCrate() || value == KwPathRoot() || value == KwDollarCrate()
}
func (value Symbol) IsBoolLit() bool { return value == KwTrue() || value == KwFalse() }
func (value Symbol) CanBeRaw() bool {
	return value != Sym_empty() && value != KwUnderscore() && !value.IsPathSegmentKeyword()
}
func IsPredefinedSymbol(index uint32) bool { return index < PredefinedSymbolsCount }
func UsedKeywords(edition func() Edition) []Symbol {
	result := make([]Symbol, 0)
	for index := KwDollarCrate().value; index < KwYeet().value; index++ {
		value := NewSymbol(index)
		if value.isUsedKeywordAlways() || value.isUsedKeywordConditional(edition) {
			result = append(result, value)
		}
	}
	return result
}

var stdlibStableCrates = [4]Symbol{Sym_std(), Sym_core(), Sym_alloc(), Sym_proc_macro()}

// StdlibStableCratesBorrowed is the source STDLIB_STABLE_CRATES static borrow.
// The returned array must be treated as read-only, just like a Rust shared
// reference. Repeated borrows point to the same immutable source storage.
func StdlibStableCratesBorrowed() *[4]Symbol { return &stdlibStableCrates }

// StdlibStableCrates is the host convenience that returns an independent copy.
func StdlibStableCrates() []Symbol {
	return append([]Symbol(nil), stdlibStableCrates[:]...)
}
func SymDummy() Symbol      { return Sym_dummy() }
func SymInclude() Symbol    { return Sym_include() }
func SymMacroRules() Symbol { return KwMacroRules() }
func SymC() Symbol          { value, _ := ASCIILetterDigitSymbol('C'); return value }
func SymF() Symbol          { value, _ := ASCIILetterDigitSymbol('f'); return value }
func SymD() Symbol          { value, _ := ASCIILetterDigitSymbol('d'); return value }
func ASCIILetterDigitSymbol(c rune) (Symbol, bool) {
	switch {
	case c >= '0' && c <= '9':
		return NewSymbol(symbolDigitsBase + uint32(c-'0')), true
	case c >= 'A' && c <= 'Z':
		return NewSymbol(symbolUppercaseLettersBase + uint32(c-'A')), true
	case c >= 'a' && c <= 'z':
		return NewSymbol(symbolLowercaseLettersBase + uint32(c-'a')), true
	default:
		return Symbol{}, false
	}
}
func CharacterSymbol(session *SessionGlobals, c rune) Symbol {
	return CharacterSymbolWithContext(session, c, rustfmt.Context{})
}
func CharacterSymbolWithContext(session *SessionGlobals, c rune, context rustfmt.Context) Symbol {
	if !utf8.ValidRune(c) {
		context.Panic("rustcspan: char must be a Unicode scalar")
	}
	if value, ok := ASCIILetterDigitSymbol(c); ok {
		return value
	}
	return InternSymbolWithContext(session, string(c), context)
}

type SymbolInteger interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

func IntegerSymbol[N SymbolInteger](session *SessionGlobals, n N) Symbol {
	return IntegerSymbolWithContext(session, n, rustfmt.Context{})
}
func IntegerSymbolWithContext[N SymbolInteger](session *SessionGlobals, n N, context rustfmt.Context) Symbol {
	// Decimal formatting is exact for every builtin integer width; native usize
	// conversion fails for negatives and values above the host word, as in Rust.
	if n >= 0 && n < 10 {
		return NewSymbol(symbolDigitsBase + uint32(n))
	}
	if n < 0 {
		return InternSymbolWithContext(session, strconv.FormatInt(int64(n), 10), context)
	}
	return InternSymbolWithContext(session, strconv.FormatUint(uint64(n), 10), context)
}

// Integer128Symbol covers itoa's i128/u128 domain. Values outside the selected
// Rust integer width are rejected; the big.Int is borrowed and never modified.
func Integer128Symbol(session *SessionGlobals, n *big.Int, signed bool) Symbol {
	return Integer128SymbolWithContext(session, n, signed, rustfmt.Context{})
}
func Integer128SymbolWithContext(session *SessionGlobals, n *big.Int, signed bool, context rustfmt.Context) Symbol {
	if n == nil || signed && (n.BitLen() > 127 && !(n.Sign() < 0 && n.Cmp(new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 127))) == 0)) || !signed && (n.Sign() < 0 || n.BitLen() > 128) {
		context.Panic("rustcspan: integer outside selected 128-bit domain")
	}
	if n.Sign() >= 0 && n.IsUint64() && n.Uint64() < 10 {
		return NewSymbol(symbolDigitsBase + uint32(n.Uint64()))
	}
	return InternSymbolWithContext(session, n.String(), context)
}

func symbolDebugByteStringList(values []string) string {
	var text strings.Builder
	text.WriteByte('[')
	for i, value := range values {
		if i != 0 {
			text.WriteString(", ")
		}
		text.WriteByte('[')
		for j := 0; j < len(value); j++ {
			if j != 0 {
				text.WriteString(", ")
			}
			text.WriteString(strconv.FormatUint(uint64(value[j]), 10))
		}
		text.WriteByte(']')
	}
	text.WriteByte(']')
	return text.String()
}
