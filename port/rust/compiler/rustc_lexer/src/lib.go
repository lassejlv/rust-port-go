// SPDX-License-Identifier: MIT OR Apache-2.0
// Ported from rust-lang/rust compiler/rustc_lexer/src/lib.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. See upstream licenses in port/rust.

// Package rustclexer implements the low-level Rust lexer. Its tokens preserve
// byte lengths and lexical error metadata; they are not the parser's tokens.
// Like Rust's &str inputs, all string inputs must be valid UTF-8. Public entry
// points panic on invalid UTF-8 rather than silently replace invalid bytes.
package rustclexer

import (
	"fmt"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"strings"
	"unicode/utf8"
)

// Token is a token kind and its length in bytes of the original input.
type Token struct {
	Kind TokenKind
	Len  uint32
}

// TokenTag is the discriminant of TokenKind, corresponding to upstream's enum.
type TokenTag uint8

const (
	LineComment TokenTag = iota
	BlockComment
	Whitespace
	Frontmatter
	Ident
	InvalidIdent
	RawIdent
	ForcedKeywordIdent
	UnknownPrefix
	UnknownPrefixLifetime
	RawLifetime
	GuardedStrPrefix
	Literal
	Lifetime
	Semi
	Comma
	Dot
	OpenParen
	CloseParen
	OpenBrace
	CloseBrace
	OpenBracket
	CloseBracket
	At
	Pound
	Tilde
	Question
	Colon
	Dollar
	Eq
	Bang
	Lt
	Gt
	Minus
	And
	Or
	Plus
	Star
	Slash
	Caret
	Percent
	Unknown
	Eof
)

// TokenKind represents every upstream TokenKind variant. Fields not belonging
// to Tag's variant are zero. DocStyle is used by comments; Terminated by block
// comments; the two frontmatter flags by Frontmatter; Literal and SuffixStart by
// Literal; StartsWithNumber by Lifetime (also true for emoji lifetime names).
type TokenKind struct {
	Tag                           TokenTag
	DocStyle                      DocStyle
	Terminated                    bool
	HasInvalidPrecedingWhitespace bool
	InvalidInfostring             bool
	Literal                       LiteralKind
	SuffixStart                   uint32
	StartsWithNumber              bool
}

type DocStyle uint8

const (
	NoDocStyle DocStyle = iota // upstream None
	Outer
	Inner
)

type LiteralTag uint8

const (
	Int LiteralTag = iota
	Float
	Char
	Byte
	Str
	ByteStr
	CStr
	RawStr
	RawByteStr
	RawCStr
)

// LiteralKind records the literal independently of its suffix. Thus 1f32 is
// Int, not Float. HasNHashes represents upstream Option<u8>: false means an
// invalid raw literal, while true with NHashes == 0 is a valid raw literal.
type LiteralKind struct {
	Tag           LiteralTag
	Base          Base
	EmptyInt      bool
	EmptyExponent bool
	Terminated    bool
	NHashes       uint8
	HasNHashes    bool
}

// GuardedStr can have fewer closing hashes than opening hashes. Terminated
// indicates that the quoted string terminated, irrespective of its hashes.
type GuardedStr struct {
	NHashes    uint32
	Terminated bool
	TokenLen   uint32
}

type RawStrErrorTag uint8

const (
	InvalidStarter RawStrErrorTag = iota
	NoTerminator
	TooManyDelimiters
)

// RawStrError retains all upstream raw-string diagnostics. The optional offset
// is measured in bytes, including prefixLen as in upstream.
type RawStrError struct {
	Tag                         RawStrErrorTag
	BadChar                     rune
	Expected                    uint32
	Found                       uint32
	PossibleTerminatorOffset    uint32
	HasPossibleTerminatorOffset bool
}

func (e RawStrError) Error() string { return e.RustDebug() }

type Base uint8

const (
	Binary      Base = 2
	Octal       Base = 8
	Decimal     Base = 10
	Hexadecimal Base = 16
)

type FrontmatterAllowed uint8

const (
	FrontmatterNo FrontmatterAllowed = iota
	FrontmatterYes
)

const EOFChar rune = '\x00'

func requireUTF8(input string) {
	if !utf8.ValidString(input) {
		panic("rustclexer: input must be valid UTF-8 (Rust &str contract)")
	}
}

// StripShebang returns the number of bytes in the first line of a shebang,
// excluding its line ending. A #! whose next significant token is [ is Rust.
func StripShebang(input string) (int, bool) {
	return StripShebangWithContext(input, rustfmt.Context{})
}

// StripShebangWithContext carries source ambient panic state into its lexer.
func StripShebangWithContext(input string, context rustfmt.Context) (int, bool) {
	return StripShebangWithConfigAndContext(input, DefaultCursorConfig(), context)
}
func StripShebangWithConfig(input string, config CursorConfig) (int, bool) {
	return StripShebangWithConfigAndContext(input, config, rustfmt.Context{})
}
func StripShebangWithConfigAndContext(input string, config CursorConfig, context rustfmt.Context) (int, bool) {
	requireUTF8WithContext(input, context)
	if !strings.HasPrefix(input, "#!") {
		return 0, false
	}
	tail := input[2:]
	cursor := NewCursorWithConfigAndContext(tail, FrontmatterNo, config, context)
	for {
		t := cursor.AdvanceToken().Kind
		if t.Tag == Whitespace || ((t.Tag == LineComment || t.Tag == BlockComment) && t.DocStyle == NoDocStyle) {
			continue
		}
		if t.Tag == OpenBracket {
			return 0, false
		}
		break
	}
	lineLen := len(tail)
	if i := strings.IndexByte(tail, '\n'); i >= 0 {
		lineLen = i
		if i > 0 && tail[i-1] == '\r' {
			lineLen--
		}
	}
	return 2 + lineLen, true
}

// ValidateRawStr obtains detailed errors for an invalid raw literal. prefixLen
// is the number of leading characters to skip (1 for r, 2 for br or cr).
// As in upstream, a missing prefix character panics.
func ValidateRawStr(input string, prefixLen uint32) *RawStrError {
	return ValidateRawStrWithConfig(input, prefixLen, DefaultCursorConfig())
}

// Tokenize lexes input, excluding the EOF token. When lexing a complete document,
// first StripShebang and then allow frontmatter. Disallow it for document slices.
func Tokenize(input string, frontmatter FrontmatterAllowed) []Token {
	iterator := NewTokenizer(input, frontmatter)
	tokens := make([]Token, 0)
	for {
		token, ok := iterator.Next()
		if !ok {
			return tokens
		}
		tokens = append(tokens, token)
	}
}

// IsWhitespace implements Rust's stable Pattern_White_Space set.
func IsWhitespace(c rune) bool {
	switch c {
	case '\n', '\v', '\f', '\r', '\u0085', '\u2028', '\u2029', '\u200e', '\u200f', '\t', ' ':
		return true
	}
	return false
}

func IsHorizontalWhitespace(c rune) bool { return c == '\t' || c == ' ' }
func IsIDStart(c rune) bool              { return c == '_' || isXIDStart(c) }
func IsIDContinue(c rune) bool           { return isXIDContinue(c) }

// IsIdent tests the lexical identifier grammar, including the spelling _.
func IsIdent(input string) bool {
	requireUTF8(input)
	first := true
	for _, c := range input {
		if first {
			if !IsIDStart(c) {
				return false
			}
			first = false
		} else if !IsIDContinue(c) {
			return false
		}
	}
	return !first
}

// Cursor is a peekable UTF-8 character cursor. Its positions count bytes.
// Copying a Cursor makes an independent snapshot, analogous to cloning Chars.
type Cursor struct {
	remaining          string
	lenRemaining       int
	frontmatterAllowed FrontmatterAllowed
	prev               rune
	config             CursorConfig
	context            rustfmt.Context
}

func NewCursor(input string, frontmatter FrontmatterAllowed) *Cursor {
	return NewCursorWithConfig(input, frontmatter, DefaultCursorConfig())
}

func (c *Cursor) AsStr() string { return c.remaining }
func (c *Cursor) Prev() rune {
	if !c.config.DebugAssertions {
		return EOFChar
	}
	return c.prev
}
func (c *Cursor) First() rune {
	if c.remaining == "" {
		return EOFChar
	}
	r, _ := utf8.DecodeRuneInString(c.remaining)
	return r
}
func (c *Cursor) Second() rune { return c.peek(1) }
func (c *Cursor) Third() rune  { return c.peek(2) }
func (c *Cursor) peek(n int) rune {
	rest := c.remaining
	for i := 0; rest != ""; i++ {
		r, width := utf8.DecodeRuneInString(rest)
		if i == n {
			return r
		}
		rest = rest[width:]
	}
	return EOFChar
}
func (c *Cursor) IsEOF() bool            { return c.remaining == "" }
func (c *Cursor) PosWithinToken() uint32 { return uint32(c.lenRemaining - len(c.remaining)) }
func (c *Cursor) ResetPosWithinToken()   { c.lenRemaining = len(c.remaining) }

func (c *Cursor) Bump() (rune, bool) {
	if c.remaining == "" {
		return 0, false
	}
	r, width := utf8.DecodeRuneInString(c.remaining)
	c.remaining = c.remaining[width:]
	if c.config.DebugAssertions {
		c.prev = r
	}
	return r, true
}

// BumpBytes advances by bytes, requiring a UTF-8 character boundary like &str
// slicing. Like upstream, it does not update Prev.
func (c *Cursor) BumpBytes(n int) {
	if n < 0 || n > len(c.remaining) || (n < len(c.remaining) && !utf8.RuneStart(c.remaining[n])) {
		c.context.Panic("rustclexer: byte advance is not a UTF-8 character boundary")
	}
	c.remaining = c.remaining[n:]
}

func (c *Cursor) EatWhile(predicate func(rune) bool) {
	for predicate(c.First()) && !c.IsEOF() {
		c.Bump()
	}
}

func (c *Cursor) EatUntil(b byte) {
	if index := strings.IndexByte(c.remaining, b); index >= 0 {
		c.BumpBytes(index)
	} else {
		c.remaining = ""
	}
}

func simple(tag TokenTag) TokenKind { return TokenKind{Tag: tag} }
func literal(kind LiteralKind, suffixStart uint32) TokenKind {
	return TokenKind{Tag: Literal, Literal: kind, SuffixStart: suffixStart}
}

// AdvanceToken parses one token. EOF can be called repeatedly and has length 0.
func (c *Cursor) AdvanceToken() Token {
	first, ok := c.Bump()
	if !ok {
		return Token{Kind: simple(Eof)}
	}
	var kind TokenKind
	switch {
	case c.frontmatterAllowed == FrontmatterYes && IsWhitespace(first):
		last := first
		for IsWhitespace(c.First()) {
			r, ok := c.Bump()
			if !ok {
				break
			}
			last = r
		}
		if last != '\n' && strings.HasPrefix(c.AsStr(), "---") {
			c.Bump()
			kind = c.frontmatter(true)
		} else {
			kind = simple(Whitespace)
		}
	case first == '-' && c.frontmatterAllowed == FrontmatterYes && strings.HasPrefix(c.AsStr(), "--"):
		kind = c.frontmatter(false)
	case first == '/':
		switch c.First() {
		case '/':
			kind = c.lineComment()
		case '*':
			kind = c.blockComment()
		default:
			kind = simple(Slash)
		}
	case IsWhitespace(first):
		kind = c.whitespace()
	case first == 'r':
		switch {
		case c.First() == '#' && IsIDStart(c.Second()):
			c.Bump()
			c.eatIdentifier()
			kind = simple(RawIdent)
		case c.First() == '#' || c.First() == '"':
			nHashes, err := c.rawDoubleQuotedString(1)
			suffix := c.PosWithinToken()
			if err == nil {
				c.eatLiteralSuffix()
			}
			kind = literal(LiteralKind{Tag: RawStr, NHashes: nHashes, HasNHashes: err == nil}, suffix)
		default:
			kind = c.identOrUnknownPrefix(false)
		}
	case first == 'k':
		if c.First() == '#' && IsIDStart(c.Second()) {
			c.Bump()
			c.eatIdentifier()
			kind = simple(ForcedKeywordIdent)
		} else {
			kind = c.identOrUnknownPrefix(false)
		}
	case first == 'b':
		kind = c.cOrByteString(ByteStr, RawByteStr, true)
	case first == 'c':
		kind = c.cOrByteString(CStr, RawCStr, false)
	case IsIDStart(first):
		kind = c.identOrUnknownPrefix(false)
	case asciiDigit(first):
		lit := c.number(first)
		suffix := c.PosWithinToken()
		c.eatLiteralSuffix()
		kind = literal(lit, suffix)
	case first == '#' && (c.First() == '"' || c.First() == '#'):
		c.Bump()
		kind = simple(GuardedStrPrefix)
	case first == '\'':
		kind = c.lifetimeOrChar()
	case first == '"':
		terminated := c.doubleQuotedString()
		suffix := c.PosWithinToken()
		if terminated {
			c.eatLiteralSuffix()
		}
		kind = literal(LiteralKind{Tag: Str, Terminated: terminated}, suffix)
	default:
		switch first {
		case ';':
			kind = simple(Semi)
		case ',':
			kind = simple(Comma)
		case '.':
			kind = simple(Dot)
		case '(':
			kind = simple(OpenParen)
		case ')':
			kind = simple(CloseParen)
		case '{':
			kind = simple(OpenBrace)
		case '}':
			kind = simple(CloseBrace)
		case '[':
			kind = simple(OpenBracket)
		case ']':
			kind = simple(CloseBracket)
		case '@':
			kind = simple(At)
		case '#':
			kind = simple(Pound)
		case '~':
			kind = simple(Tilde)
		case '?':
			kind = simple(Question)
		case ':':
			kind = simple(Colon)
		case '$':
			kind = simple(Dollar)
		case '=':
			kind = simple(Eq)
		case '!':
			kind = simple(Bang)
		case '<':
			kind = simple(Lt)
		case '>':
			kind = simple(Gt)
		case '-':
			kind = simple(Minus)
		case '&':
			kind = simple(And)
		case '|':
			kind = simple(Or)
		case '+':
			kind = simple(Plus)
		case '*':
			kind = simple(Star)
		case '^':
			kind = simple(Caret)
		case '%':
			kind = simple(Percent)
		default:
			if isEmoji(first) {
				kind = c.invalidIdent()
			} else {
				kind = simple(Unknown)
			}
		}
	}
	if c.frontmatterAllowed == FrontmatterYes && kind.Tag != Whitespace {
		c.frontmatterAllowed = FrontmatterNo
	}
	token := Token{Kind: kind, Len: c.PosWithinToken()}
	c.ResetPosWithinToken()
	return token
}

func (c *Cursor) frontmatter(hasInvalidPrecedingWhitespace bool) TokenKind {
	c.debugAssert(c.Prev() == '-')
	pos := c.PosWithinToken()
	c.EatWhile(func(r rune) bool { return r == '-' })
	lengthOpening := c.addU32(c.subU32(c.PosWithinToken(), pos), 1)
	c.debugAssert(lengthOpening >= 3)
	c.EatWhile(func(r rune) bool { return r != '\n' && IsHorizontalWhitespace(r) })
	if IsIDStart(c.First()) {
		c.Bump()
		c.EatWhile(func(r rune) bool { return IsIDContinue(r) || r == '-' || r == '.' })
	}
	c.EatWhile(func(r rune) bool { return r != '\n' && IsHorizontalWhitespace(r) })
	invalidInfostring := c.First() != '\n'
	found := false
	nlFencePattern := "\n" + strings.Repeat("-", int(lengthOpening))
	if closing := strings.Index(c.AsStr(), nlFencePattern); closing >= 0 {
		c.BumpBytes(closing + len(nlFencePattern))
		c.EatUntil('\n')
		found = true
	}
	if !found {
		rest := c.AsStr()
		potentialClosing := -1
		if i := strings.Index(rest, "\n---"); i >= 0 {
			potentialClosing = i + 1
		} else {
			for _, pattern := range []string{"\nuse ", "\n//!", "\n#!["} {
				if i := strings.Index(rest, pattern); i >= 0 {
					potentialClosing = i
					break
				}
			}
		}
		if potentialClosing < 0 {
			baseIndex := 0
			for {
				closing := strings.Index(rest, "---")
				if closing < 0 {
					break
				}
				precedingStart := strings.LastIndexByte(rest[:closing], '\n') + 1
				allHorizontal := true
				for _, r := range rest[precedingStart:closing] {
					if !IsHorizontalWhitespace(r) {
						allHorizontal = false
						break
					}
				}
				if allHorizontal {
					potentialClosing = closing + baseIndex
					break
				}
				rest = rest[closing+3:]
				baseIndex += closing + 3
			}
		}
		if potentialClosing >= 0 {
			c.BumpBytes(potentialClosing)
			c.EatUntil('\n')
		} else {
			c.EatWhile(func(rune) bool { return true })
		}
	}
	return TokenKind{Tag: Frontmatter, HasInvalidPrecedingWhitespace: hasInvalidPrecedingWhitespace, InvalidInfostring: invalidInfostring}
}

func (c *Cursor) lineComment() TokenKind {
	c.debugAssert(c.Prev() == '/' && c.First() == '/')
	c.Bump()
	doc := NoDocStyle
	if c.First() == '!' {
		doc = Inner
	} else if c.First() == '/' && c.Second() != '/' {
		doc = Outer
	}
	c.EatUntil('\n')
	return TokenKind{Tag: LineComment, DocStyle: doc}
}

func (c *Cursor) blockComment() TokenKind {
	c.debugAssert(c.Prev() == '/' && c.First() == '*')
	c.Bump()
	doc := NoDocStyle
	if c.First() == '!' {
		doc = Inner
	} else if c.First() == '*' && c.Second() != '*' && c.Second() != '/' {
		doc = Outer
	}
	depth := 1
	for {
		r, ok := c.Bump()
		if !ok {
			break
		}
		if r == '/' && c.First() == '*' {
			c.Bump()
			depth++
		} else if r == '*' && c.First() == '/' {
			c.Bump()
			depth--
			if depth == 0 {
				break
			}
		}
	}
	return TokenKind{Tag: BlockComment, DocStyle: doc, Terminated: depth == 0}
}

func (c *Cursor) whitespace() TokenKind {
	c.debugAssert(IsWhitespace(c.Prev()))
	c.EatWhile(IsWhitespace)
	return simple(Whitespace)
}

func (c *Cursor) identOrUnknownPrefix(alreadyInvalid bool) TokenKind {
	c.debugAssert(IsIDStart(c.Prev()) || alreadyInvalid)
	// alreadyInvalid participates in the source debug assertion above.
	c.EatWhile(IsIDContinue)
	switch c.First() {
	case '#', '"', '\'':
		return simple(UnknownPrefix)
	default:
		if isEmoji(c.First()) {
			return c.invalidIdent()
		}
		return simple(Ident)
	}
}

func (c *Cursor) invalidIdent() TokenKind {
	c.EatWhile(func(r rune) bool { return IsIDContinue(r) || isEmoji(r) })
	return simple(InvalidIdent)
}

func (c *Cursor) cOrByteString(normal, raw LiteralTag, allowSingleQuoted bool) TokenKind {
	switch {
	case c.First() == '\'' && allowSingleQuoted:
		c.Bump()
		terminated := c.singleQuotedString()
		suffix := c.PosWithinToken()
		if terminated {
			c.eatLiteralSuffix()
		}
		return literal(LiteralKind{Tag: Byte, Terminated: terminated}, suffix)
	case c.First() == '"':
		c.Bump()
		terminated := c.doubleQuotedString()
		suffix := c.PosWithinToken()
		if terminated {
			c.eatLiteralSuffix()
		}
		return literal(LiteralKind{Tag: normal, Terminated: terminated}, suffix)
	case c.First() == 'r' && (c.Second() == '"' || c.Second() == '#'):
		c.Bump()
		nHashes, err := c.rawDoubleQuotedString(2)
		suffix := c.PosWithinToken()
		if err == nil {
			c.eatLiteralSuffix()
		}
		return literal(LiteralKind{Tag: raw, NHashes: nHashes, HasNHashes: err == nil}, suffix)
	default:
		return c.identOrUnknownPrefix(false)
	}
}

func asciiDigit(r rune) bool { return '0' <= r && r <= '9' }

func (c *Cursor) number(firstDigit rune) LiteralKind {
	c.debugAssert(c.Prev() >= '0' && c.Prev() <= '9')
	base := Decimal
	if firstDigit == '0' {
		switch c.First() {
		case 'b':
			base = Binary
			c.Bump()
			if !c.eatDecimalDigits() {
				return LiteralKind{Tag: Int, Base: base, EmptyInt: true}
			}
		case 'o':
			base = Octal
			c.Bump()
			if !c.eatDecimalDigits() {
				return LiteralKind{Tag: Int, Base: base, EmptyInt: true}
			}
		case 'x':
			base = Hexadecimal
			c.Bump()
			if !c.eatHexadecimalDigits() {
				return LiteralKind{Tag: Int, Base: base, EmptyInt: true}
			}
		case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '_':
			c.eatDecimalDigits()
		case '.', 'e', 'E':
		default:
			return LiteralKind{Tag: Int, Base: base}
		}
	} else {
		c.eatDecimalDigits()
	}
	switch {
	case c.First() == '.' && c.Second() != '.' && !IsIDStart(c.Second()):
		c.Bump()
		emptyExponent := false
		if asciiDigit(c.First()) {
			c.eatDecimalDigits()
			if c.First() == 'e' || c.First() == 'E' {
				c.Bump()
				emptyExponent = !c.eatFloatExponent()
			}
		}
		return LiteralKind{Tag: Float, Base: base, EmptyExponent: emptyExponent}
	case c.First() == 'e' || c.First() == 'E':
		c.Bump()
		return LiteralKind{Tag: Float, Base: base, EmptyExponent: !c.eatFloatExponent()}
	default:
		return LiteralKind{Tag: Int, Base: base}
	}
}

func (c *Cursor) lifetimeOrChar() TokenKind {
	c.debugAssert(c.Prev() == '\'')
	invalid := false
	canBeLifetime := false
	if c.Second() != '\'' {
		r := c.First()
		invalid = asciiDigit(r) || isEmoji(r)
		canBeLifetime = IsIDStart(r) || invalid
	}
	if !canBeLifetime {
		terminated := c.singleQuotedString()
		suffix := c.PosWithinToken()
		if terminated {
			c.eatLiteralSuffix()
		}
		return literal(LiteralKind{Tag: Char, Terminated: terminated}, suffix)
	}
	if c.First() == 'r' && c.Second() == '#' && IsIDStart(c.Third()) {
		c.Bump()
		c.Bump()
		c.Bump()
		c.EatWhile(IsIDContinue)
		return simple(RawLifetime)
	}
	startsWithNumber := asciiDigit(c.First())
	c.Bump()
	identKind := c.identOrUnknownPrefix(invalid)
	invalid = invalid || identKind.Tag == InvalidIdent
	switch {
	case c.First() == '\'':
		c.Bump()
		return literal(LiteralKind{Tag: Char, Terminated: true}, c.PosWithinToken())
	case c.First() == '#' && !startsWithNumber:
		return simple(UnknownPrefixLifetime)
	default:
		return TokenKind{Tag: Lifetime, StartsWithNumber: invalid}
	}
}

func (c *Cursor) singleQuotedString() bool {
	c.debugAssert(c.Prev() == '\'')
	if c.Second() == '\'' && c.First() != '\\' {
		c.Bump()
		c.Bump()
		return true
	}
	for {
		switch c.First() {
		case '\'':
			c.Bump()
			return true
		case '/':
			return false
		case '\n':
			if c.Second() != '\'' {
				return false
			}
			c.Bump()
		case EOFChar:
			if c.IsEOF() {
				return false
			}
			c.Bump()
		case '\\':
			c.Bump()
			c.Bump()
		default:
			c.Bump()
		}
	}
}

func (c *Cursor) doubleQuotedString() bool {
	c.debugAssert(c.Prev() == '"')
	for {
		r, ok := c.Bump()
		if !ok {
			return false
		}
		if r == '"' {
			return true
		}
		if r == '\\' && (c.First() == '\\' || c.First() == '"') {
			c.Bump()
		}
	}
}

// GuardedDoubleQuotedString consumes a possible guarded string, including its
// suffix. Failure does not restore the cursor. Successful recognition resets the
// byte position within the token. Call on a cursor before its leading hashes.
func (c *Cursor) GuardedDoubleQuotedString() (GuardedStr, bool) {
	c.debugAssert(c.Prev() != '#')
	var startHashes uint32
	for c.First() == '#' {
		startHashes = c.addU32(startHashes, 1)
		c.Bump()
	}
	if c.First() != '"' {
		return GuardedStr{}, false
	}
	c.Bump()
	terminated := c.doubleQuotedString()
	if !terminated {
		tokenLen := c.PosWithinToken()
		c.ResetPosWithinToken()
		return GuardedStr{NHashes: startHashes, Terminated: false, TokenLen: tokenLen}, true
	}
	var endHashes uint32
	for c.First() == '#' && endHashes < startHashes {
		endHashes = c.addU32(endHashes, 1)
		c.Bump()
	}
	c.eatLiteralSuffix()
	tokenLen := c.PosWithinToken()
	c.ResetPosWithinToken()
	return GuardedStr{NHashes: startHashes, Terminated: true, TokenLen: tokenLen}, true
}

func (c *Cursor) rawDoubleQuotedString(prefixLen uint32) (uint8, *RawStrError) {
	nHashes, err := c.rawStringUnvalidated(prefixLen)
	if err != nil {
		return 0, err
	}
	if nHashes > 255 {
		return 0, &RawStrError{Tag: TooManyDelimiters, Found: nHashes}
	}
	return uint8(nHashes), nil
}

func (c *Cursor) rawStringUnvalidated(prefixLen uint32) (uint32, *RawStrError) {
	c.debugAssert(c.Prev() == 'r')
	startPos := c.PosWithinToken()
	var possibleOffset, maxHashes, startHashes uint32
	hasPossibleOffset := false
	for c.First() == '#' {
		startHashes = c.addU32(startHashes, 1)
		c.Bump()
	}
	r, ok := c.Bump()
	if !ok || r != '"' {
		return 0, &RawStrError{Tag: InvalidStarter, BadChar: r}
	}
	for {
		c.EatUntil('"')
		if c.IsEOF() {
			return 0, &RawStrError{Tag: NoTerminator, Expected: startHashes, Found: maxHashes,
				PossibleTerminatorOffset: possibleOffset, HasPossibleTerminatorOffset: hasPossibleOffset}
		}
		c.Bump()
		var endHashes uint32
		for c.First() == '#' && endHashes < startHashes {
			endHashes = c.addU32(endHashes, 1)
			c.Bump()
		}
		if endHashes == startHashes {
			return startHashes, nil
		} else if endHashes > maxHashes {
			possibleOffset = c.addU32(c.subU32(c.subU32(c.PosWithinToken(), startPos), endHashes), prefixLen)
			hasPossibleOffset = true
			maxHashes = endHashes
		}
	}
}

func (c *Cursor) eatDecimalDigits() bool {
	hasDigits := false
	for {
		if c.First() == '_' {
			c.Bump()
		} else if asciiDigit(c.First()) {
			hasDigits = true
			c.Bump()
		} else {
			return hasDigits
		}
	}
}

func (c *Cursor) eatHexadecimalDigits() bool {
	hasDigits := false
	for {
		r := c.First()
		if r == '_' {
			c.Bump()
		} else if asciiDigit(r) || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F') {
			hasDigits = true
			c.Bump()
		} else {
			return hasDigits
		}
	}
}

func (c *Cursor) eatFloatExponent() bool {
	c.debugAssert(c.Prev() == 'e' || c.Prev() == 'E')
	if c.First() == '-' || c.First() == '+' {
		c.Bump()
	}
	return c.eatDecimalDigits()
}

func (c *Cursor) eatLiteralSuffix() { c.eatIdentifier() }
func (c *Cursor) eatIdentifier() {
	if !IsIDStart(c.First()) {
		return
	}
	c.Bump()
	c.EatWhile(IsIDContinue)
}

func isEmoji(c rune) bool { return c > 0x7f && isEmojiChar(c) }

var tokenTagNames = [...]string{
	"LineComment", "BlockComment", "Whitespace", "Frontmatter", "Ident", "InvalidIdent", "RawIdent",
	"ForcedKeywordIdent", "UnknownPrefix", "UnknownPrefixLifetime", "RawLifetime", "GuardedStrPrefix",
	"Literal", "Lifetime", "Semi", "Comma", "Dot", "OpenParen", "CloseParen", "OpenBrace", "CloseBrace",
	"OpenBracket", "CloseBracket", "At", "Pound", "Tilde", "Question", "Colon", "Dollar", "Eq", "Bang",
	"Lt", "Gt", "Minus", "And", "Or", "Plus", "Star", "Slash", "Caret", "Percent", "Unknown", "Eof",
}

func (tag TokenTag) String() string {
	if int(tag) < len(tokenTagNames) {
		return tokenTagNames[tag]
	}
	return fmt.Sprintf("TokenTag(%d)", tag)
}

func (b Base) String() string {
	switch b {
	case Binary:
		return "Binary"
	case Octal:
		return "Octal"
	case Decimal:
		return "Decimal"
	case Hexadecimal:
		return "Hexadecimal"
	}
	return fmt.Sprintf("Base(%d)", b)
}

func (d DocStyle) String() string {
	switch d {
	case NoDocStyle:
		return "None"
	case Outer:
		return "Some(Outer)"
	case Inner:
		return "Some(Inner)"
	}
	return fmt.Sprintf("DocStyle(%d)", d)
}

// RustDebug uses the ordinary default source Debug formatter.
func (t Token) RustDebug() string       { return rustfmt.String(t.FormatDebug) }
func (k TokenKind) RustDebug() string   { return rustfmt.String(k.FormatDebug) }
func (k LiteralKind) RustDebug() string { return rustfmt.String(k.FormatDebug) }
func (e RawStrError) RustDebug() string { return rustfmt.String(e.FormatDebug) }
