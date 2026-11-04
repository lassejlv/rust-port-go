// Port of rustc-literal-escaper 0.0.8 src/lib.rs.
// SPDX-License-Identifier: Apache-2.0 OR MIT
package literalescaper

import (
	"fmt"
	"strings"
	"unicode/utf8"

	unicodecore "github.com/lassejlv/rust-port-go/port/rust/library/core/src/unicode"
)

// Range uses the original literal's UTF-8 byte offsets; End is exclusive.
type Range struct{ Start, End int }

type Result[T any] struct {
	Value T
	Error EscapeError
}

func (r Result[T]) IsOK() bool { return r.Error == NoError }

type EscapeError uint8

const (
	NoError EscapeError = iota
	ZeroChars
	MoreThanOneChar
	LoneSlash
	InvalidEscape
	BareCarriageReturn
	BareCarriageReturnInRawString
	EscapeOnlyChar
	TooShortHexEscape
	InvalidCharInHexEscape
	OutOfRangeHexEscape
	NoBraceInUnicodeEscape
	InvalidCharInUnicodeEscape
	EmptyUnicodeEscape
	UnclosedUnicodeEscape
	LeadingUnderscoreUnicodeEscape
	OverlongUnicodeEscape
	LoneSurrogateUnicodeEscape
	OutOfRangeUnicodeEscape
	UnicodeEscapeInByte
	NonAsciiCharInByte
	NulInCStr
	UnskippedWhitespaceWarning
	MultipleSkippedLinesWarning
)

var escapeErrorNames = [...]string{
	"", "ZeroChars", "MoreThanOneChar", "LoneSlash", "InvalidEscape",
	"BareCarriageReturn", "BareCarriageReturnInRawString", "EscapeOnlyChar",
	"TooShortHexEscape", "InvalidCharInHexEscape", "OutOfRangeHexEscape",
	"NoBraceInUnicodeEscape", "InvalidCharInUnicodeEscape", "EmptyUnicodeEscape",
	"UnclosedUnicodeEscape", "LeadingUnderscoreUnicodeEscape", "OverlongUnicodeEscape",
	"LoneSurrogateUnicodeEscape", "OutOfRangeUnicodeEscape", "UnicodeEscapeInByte",
	"NonAsciiCharInByte", "NulInCStr", "UnskippedWhitespaceWarning", "MultipleSkippedLinesWarning",
}

func (e EscapeError) String() string {
	if int(e) < len(escapeErrorNames) {
		return escapeErrorNames[e]
	}
	return fmt.Sprintf("EscapeError(%d)", e)
}

func (e EscapeError) Error() string { return e.String() }

func (e EscapeError) IsFatal() bool {
	return e != NoError && e != UnskippedWhitespaceWarning && e != MultipleSkippedLinesWarning
}

type NonZeroChar struct{ value rune }
type NonZeroByte struct{ value byte }

func NewNonZeroChar(c rune) (NonZeroChar, bool) {
	if !utf8.ValidRune(c) {
		panic("char must be a Unicode scalar value")
	}
	return NonZeroChar{c}, c != 0
}

func NewNonZeroByte(b byte) (NonZeroByte, bool) { return NonZeroByte{b}, b != 0 }
func (c NonZeroChar) Get() rune {
	if c.value == 0 {
		panic("zero NonZeroChar")
	}
	return c.value
}
func (b NonZeroByte) Get() byte {
	if b.value == 0 {
		panic("zero NonZeroByte")
	}
	return b.value
}

type MixedUnitKind uint8

const (
	CharUnit MixedUnitKind = iota
	HighByteUnit
)

// MixedUnit distinguishes a Unicode character from an escaped high byte.
// Constructors preserve the source's NonZero restrictions.
type MixedUnit struct {
	kind  MixedUnitKind
	value uint32
}

func (u MixedUnit) Kind() MixedUnitKind { return u.kind }
func (u MixedUnit) Char() (NonZeroChar, bool) {
	if u.kind != CharUnit {
		return NonZeroChar{}, false
	}
	c, ok := NewNonZeroChar(rune(u.value))
	if !ok {
		panic("zero MixedUnit")
	}
	return c, true
}
func (u MixedUnit) HighByte() (NonZeroByte, bool) {
	if u.kind != HighByteUnit {
		return NonZeroByte{}, false
	}
	b, ok := NewNonZeroByte(byte(u.value))
	if !ok {
		panic("zero MixedUnit")
	}
	return b, true
}

func MixedChar(c NonZeroChar) MixedUnit { return MixedUnit{CharUnit, uint32(c.Get())} }

// MixedHighByte constructs the public HighByte variant for any nonzero byte,
// including ASCII. MixedByte performs Rust's separate From conversion.
func MixedHighByte(b NonZeroByte) MixedUnit { return MixedUnit{HighByteUnit, uint32(b.Get())} }

func MixedByte(b NonZeroByte) MixedUnit {
	value := b.Get()
	if value < 128 {
		return MixedUnit{CharUnit, uint32(value)}
	}
	return MixedUnit{HighByteUnit, uint32(value)}
}
func MixedUnitFromChar(c rune) (MixedUnit, EscapeError) {
	value, ok := NewNonZeroChar(c)
	if !ok {
		return MixedUnit{}, NulInCStr
	}
	return MixedChar(value), NoError
}
func MixedUnitFromByte(b byte) (MixedUnit, EscapeError) {
	value, ok := NewNonZeroByte(b)
	if !ok {
		return MixedUnit{}, NulInCStr
	}
	return MixedByte(value), NoError
}

type Mode uint8

const (
	Char Mode = iota
	Byte
	Str
	RawStr
	ByteStr
	RawByteStr
	CStr
	RawCStr
)

func (m Mode) InDoubleQuotes() bool {
	switch m {
	case Str, RawStr, ByteStr, RawByteStr, CStr, RawCStr:
		return true
	case Char, Byte:
		return false
	default:
		panic("invalid literal mode")
	}
}
func (m Mode) PrefixNoRaw() string {
	switch m {
	case Char, Str, RawStr:
		return ""
	case Byte, ByteStr, RawByteStr:
		return "b"
	case CStr, RawCStr:
		return "c"
	default:
		panic("invalid literal mode")
	}
}

type literalKind uint8

const (
	unicodeLiteral literalKind = iota
	byteLiteral
	cLiteral
)

type unit struct {
	value    rune
	highByte bool
}

type chars struct {
	source   string
	position int
}

func newChars(source string) chars {
	if !utf8.ValidString(source) {
		panic("literal contents must be valid UTF-8, as required by Rust str")
	}
	return chars{source: source}
}
func (c *chars) next() (rune, bool) {
	if c.position == len(c.source) {
		return 0, false
	}
	value, size := utf8.DecodeRuneInString(c.source[c.position:])
	c.position += size
	return value, true
}

func char2unit(c rune, kind literalKind) (unit, EscapeError) {
	if kind == byteLiteral && c >= 128 {
		return unit{}, NonAsciiCharInByte
	}
	if kind == cLiteral && c == 0 {
		return unit{}, NulInCStr
	}
	return unit{value: c}, NoError
}
func hex2unit(b byte, kind literalKind) (unit, EscapeError) {
	if kind == unicodeLiteral && b >= 128 {
		return unit{}, OutOfRangeHexEscape
	}
	if kind == cLiteral && b == 0 {
		return unit{}, NulInCStr
	}
	return unit{value: rune(b), highByte: kind == cLiteral && b >= 128}, NoError
}
func mixed(u unit) MixedUnit {
	kind := CharUnit
	if u.highByte {
		kind = HighByteUnit
	}
	return MixedUnit{kind, uint32(u.value)}
}

func checkRaw(src string, kind literalKind, callback func(Range, unit, EscapeError)) {
	chars := newChars(src)
	for chars.position < len(src) {
		start := chars.position
		c, _ := chars.next()
		var value unit
		var err EscapeError
		if c == '\r' {
			err = BareCarriageReturnInRawString
		} else {
			value, err = char2unit(c, kind)
		}
		callback(Range{start, chars.position}, value, err)
	}
}
func CheckRawStr(src string, callback func(Range, Result[rune])) {
	checkRaw(src, unicodeLiteral, func(span Range, value unit, err EscapeError) { callback(span, Result[rune]{value.value, err}) })
}
func CheckRawByteStr(src string, callback func(Range, Result[byte])) {
	checkRaw(src, byteLiteral, func(span Range, value unit, err EscapeError) { callback(span, Result[byte]{byte(value.value), err}) })
}
func CheckRawCStr(src string, callback func(Range, Result[NonZeroChar])) {
	checkRaw(src, cLiteral, func(span Range, value unit, err EscapeError) {
		callback(span, Result[NonZeroChar]{NonZeroChar{value.value}, err})
	})
}

func unescapeSingle(chars *chars, kind literalKind) (unit, EscapeError) {
	c, ok := chars.next()
	if !ok {
		return unit{}, ZeroChars
	}
	var value unit
	var err EscapeError
	switch c {
	case '\\':
		value, err = unescapeOne(chars, kind)
	case '\n', '\t', '\'':
		err = EscapeOnlyChar
	case '\r':
		err = BareCarriageReturn
	default:
		value, err = char2unit(c, kind)
	}
	if err != NoError {
		return unit{}, err
	}
	if _, ok := chars.next(); ok {
		return unit{}, MoreThanOneChar
	}
	return value, NoError
}
func UnescapeChar(src string) (rune, EscapeError) {
	chars := newChars(src)
	value, err := unescapeSingle(&chars, unicodeLiteral)
	return value.value, err
}
func UnescapeByte(src string) (byte, EscapeError) {
	chars := newChars(src)
	value, err := unescapeSingle(&chars, byteLiteral)
	return byte(value.value), err
}

func unescapeOne(chars *chars, kind literalKind) (unit, EscapeError) {
	c, ok := chars.next()
	if !ok {
		return unit{}, LoneSlash
	}
	if c == '0' {
		return char2unit(0, kind)
	}
	if value, ok := simpleEscape(c); ok {
		return unit{value: rune(value)}, NoError
	}
	switch c {
	case 'x':
		value, err := hexEscape(chars)
		if err != NoError {
			return unit{}, err
		}
		return hex2unit(value, kind)
	case 'u':
		value, err := unicodeEscape(chars)
		if err != NoError {
			return unit{}, err
		}
		// Syntax errors above precede byte-mode rejection; value errors do not.
		if kind == byteLiteral {
			return unit{}, UnicodeEscapeInByte
		}
		if value > utf8.MaxRune {
			return unit{}, OutOfRangeUnicodeEscape
		}
		if value >= 0xd800 && value <= 0xdfff {
			return unit{}, LoneSurrogateUnicodeEscape
		}
		return char2unit(rune(value), kind)
	default:
		return unit{}, InvalidEscape
	}
}

func simpleEscape(c rune) (byte, bool) {
	switch c {
	case '"':
		return '"', true
	case 'n':
		return '\n', true
	case 'r':
		return '\r', true
	case 't':
		return '\t', true
	case '\\':
		return '\\', true
	case '\'':
		return '\'', true
	default:
		return 0, false
	}
}
func hexDigit(c rune) (uint32, bool) {
	switch {
	case c >= '0' && c <= '9':
		return uint32(c - '0'), true
	case c >= 'a' && c <= 'f':
		return uint32(c-'a') + 10, true
	case c >= 'A' && c <= 'F':
		return uint32(c-'A') + 10, true
	default:
		return 0, false
	}
}
func hexEscape(chars *chars) (byte, EscapeError) {
	hi, ok := chars.next()
	if !ok {
		return 0, TooShortHexEscape
	}
	high, ok := hexDigit(hi)
	if !ok {
		return 0, InvalidCharInHexEscape
	}
	lo, ok := chars.next()
	if !ok {
		return 0, TooShortHexEscape
	}
	low, ok := hexDigit(lo)
	if !ok {
		return 0, InvalidCharInHexEscape
	}
	return byte(high*16 + low), NoError
}
func unicodeEscape(chars *chars) (uint32, EscapeError) {
	c, ok := chars.next()
	if !ok || c != '{' {
		return 0, NoBraceInUnicodeEscape
	}
	c, ok = chars.next()
	if !ok {
		return 0, UnclosedUnicodeEscape
	}
	if c == '_' {
		return 0, LeadingUnderscoreUnicodeEscape
	}
	if c == '}' {
		return 0, EmptyUnicodeEscape
	}
	value, ok := hexDigit(c)
	if !ok {
		return 0, InvalidCharInUnicodeEscape
	}
	nDigits := int32(1)
	for {
		c, ok = chars.next()
		if !ok {
			return 0, UnclosedUnicodeEscape
		}
		switch c {
		case '_':
			continue
		case '}':
			if nDigits > 6 {
				return 0, OverlongUnicodeEscape
			}
			return value, NoError
		default:
			digit, ok := hexDigit(c)
			if !ok {
				return 0, InvalidCharInUnicodeEscape
			}
			nDigits++
			if nDigits <= 6 {
				value = value*16 + digit
			}
		}
	}
}

func rustWhitespace(c rune) bool {
	return c == ' ' || (c >= '\t' && c <= '\r') || unicodecore.WhiteSpaceLookup(c)
}
func skipASCIIWhitespace(chars *chars, start int, callback func(Range, EscapeError)) {
	rest := chars.source[chars.position:]
	firstNonSpace := 0
	for firstNonSpace < len(rest) {
		b := rest[firstNonSpace]
		if b != ' ' && b != '\t' && b != '\n' && b != '\r' {
			break
		}
		firstNonSpace++
	}
	space := rest[:firstNonSpace]
	end := start + 2 + firstNonSpace
	if strings.ContainsRune(space, '\n') {
		callback(Range{start, end}, MultipleSkippedLinesWarning)
	}
	chars.position += firstNonSpace
	if chars.position < len(chars.source) {
		c, size := utf8.DecodeRuneInString(chars.source[chars.position:])
		if rustWhitespace(c) {
			callback(Range{start, end + size}, UnskippedWhitespaceWarning)
		}
	}
}
func unescape(src string, kind literalKind, callback func(Range, unit, EscapeError)) {
	chars := newChars(src)
	for chars.position < len(src) {
		start := chars.position
		c, _ := chars.next()
		var value unit
		var err EscapeError
		switch c {
		case '\\':
			if chars.position < len(src) && src[chars.position] == '\n' {
				chars.position++
				skipASCIIWhitespace(&chars, start, func(span Range, err EscapeError) { callback(span, unit{}, err) })
				continue
			}
			value, err = unescapeOne(&chars, kind)
		case '"':
			err = EscapeOnlyChar
		case '\r':
			err = BareCarriageReturn
		default:
			value, err = char2unit(c, kind)
		}
		callback(Range{start, chars.position}, value, err)
	}
}
func UnescapeStr(src string, callback func(Range, Result[rune])) {
	unescape(src, unicodeLiteral, func(span Range, value unit, err EscapeError) { callback(span, Result[rune]{value.value, err}) })
}
func UnescapeByteStr(src string, callback func(Range, Result[byte])) {
	unescape(src, byteLiteral, func(span Range, value unit, err EscapeError) { callback(span, Result[byte]{byte(value.value), err}) })
}
func UnescapeCStr(src string, callback func(Range, Result[MixedUnit])) {
	unescape(src, cLiteral, func(span Range, value unit, err EscapeError) { callback(span, Result[MixedUnit]{mixed(value), err}) })
}

func CheckForErrors(src string, mode Mode, callback func(Range, EscapeError)) {
	onError := func(span Range, _ unit, err EscapeError) {
		if err != NoError {
			callback(span, err)
		}
	}
	switch mode {
	case Char, Byte:
		chars := newChars(src)
		kind := unicodeLiteral
		if mode == Byte {
			kind = byteLiteral
		}
		if _, err := unescapeSingle(&chars, kind); err != NoError {
			callback(Range{0, chars.position}, err)
		}
	case Str:
		unescape(src, unicodeLiteral, onError)
	case ByteStr:
		unescape(src, byteLiteral, onError)
	case CStr:
		unescape(src, cLiteral, onError)
	case RawStr:
		checkRaw(src, unicodeLiteral, onError)
	case RawByteStr:
		checkRaw(src, byteLiteral, onError)
	case RawCStr:
		checkRaw(src, cLiteral, onError)
	default:
		panic("invalid literal mode")
	}
}
