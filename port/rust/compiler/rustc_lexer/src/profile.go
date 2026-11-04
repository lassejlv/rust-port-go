// SPDX-License-Identifier: MIT OR Apache-2.0
package rustclexer

import (
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"unicode/utf8"
)

// CursorConfig selects independent source build settings. Deliberate source
// usize-to-u32 casts still truncate; only source arithmetic expressions use
// OverflowChecks. Constructors without a config use checked debug settings.
type CursorConfig struct{ OverflowChecks, DebugAssertions bool }

func DefaultCursorConfig() CursorConfig { return CursorConfig{true, true} }
func (c *Cursor) debugAssert(condition bool) {
	if c.config.DebugAssertions && !condition {
		c.context.Panic("rustclexer: source debug assertion failed")
	}
}
func (c *Cursor) addU32(a, b uint32) uint32 {
	if c.config.OverflowChecks && b > ^uint32(0)-a {
		c.context.Panic("rustclexer: u32 addition overflow")
	}
	return a + b
}
func (c *Cursor) subU32(a, b uint32) uint32 {
	if c.config.OverflowChecks && b > a {
		c.context.Panic("rustclexer: u32 subtraction overflow")
	}
	return a - b
}

func NewCursorWithConfig(input string, frontmatter FrontmatterAllowed, config CursorConfig) *Cursor {
	return NewCursorWithConfigAndContext(input, frontmatter, config, rustfmt.Context{})
}

// WithContext entry points make source thread panic state explicit, including
// internal assertions/bounds/overflow sites. No global Go panic interceptor is
// used. Context is independent from debug/overflow configuration.
func NewCursorWithContext(input string, frontmatter FrontmatterAllowed, context rustfmt.Context) *Cursor {
	return NewCursorWithConfigAndContext(input, frontmatter, DefaultCursorConfig(), context)
}
func NewCursorWithConfigAndContext(input string, frontmatter FrontmatterAllowed, config CursorConfig, context rustfmt.Context) *Cursor {
	requireUTF8WithContext(input, context)
	return &Cursor{remaining: input, lenRemaining: len(input), frontmatterAllowed: frontmatter, config: config, context: context}
}
func requireUTF8WithContext(input string, context rustfmt.Context) {
	if !utf8.ValidString(input) {
		context.Panic("rustclexer: input must be valid UTF-8 (Rust &str contract)")
	}
}
func mergeCursorContext(a, b rustfmt.Context) rustfmt.Context {
	return rustfmt.Context{Unwinding: a.Unwinding || b.Unwinding, ProcessingPanic: a.ProcessingPanic || b.ProcessingPanic}
}

// A source operation exclusively borrows its cursor. The temporary context
// scope also covers callback destruction and restores ambient state afterward.
func (c *Cursor) enterContext(context rustfmt.Context) (rustfmt.Context, func()) {
	if c == nil {
		return context, func() {}
	}
	prior := c.context
	c.context = mergeCursorContext(prior, context)
	return c.context, func() { c.context = prior }
}

// Per-operation context forms preserve an existing cursor's state when the
// source thread enters/leaves panic processing between method calls. The
// explicit current context replaces the stored default for that one borrow.
func (c *Cursor) operationContext(context rustfmt.Context) func() {
	prior := c.context
	c.context = context
	return func() { c.context = prior }
}
func (c *Cursor) BumpBytesWithContext(n int, context rustfmt.Context) {
	restore := c.operationContext(context)
	defer restore()
	c.BumpBytes(n)
}
func (c *Cursor) EatUntilWithContext(b byte, context rustfmt.Context) {
	restore := c.operationContext(context)
	defer restore()
	c.EatUntil(b)
}
func (c *Cursor) AdvanceTokenWithContext(context rustfmt.Context) Token {
	restore := c.operationContext(context)
	defer restore()
	return c.AdvanceToken()
}
func (c *Cursor) GuardedDoubleQuotedStringWithContext(context rustfmt.Context) (GuardedStr, bool) {
	restore := c.operationContext(context)
	defer restore()
	return c.GuardedDoubleQuotedString()
}
func (it *Tokenizer) NextWithContext(context rustfmt.Context) (Token, bool) {
	restore := it.cursor.operationContext(context)
	defer restore()
	return it.Next()
}
func (it *Tokenizer) NthWithContext(n uint, context rustfmt.Context) (Token, bool) {
	restore := it.cursor.operationContext(context)
	defer restore()
	return it.Nth(n)
}
func (it *Tokenizer) CountWithContext(context rustfmt.Context) uint {
	restore := it.cursor.operationContext(context)
	defer restore()
	return it.Count()
}
func (it *Tokenizer) LastWithContext(context rustfmt.Context) (Token, bool) {
	restore := it.cursor.operationContext(context)
	defer restore()
	return it.Last()
}
func NewTokenizerWithConfig(input string, frontmatter FrontmatterAllowed, config CursorConfig) *Tokenizer {
	return &Tokenizer{NewCursorWithConfig(input, frontmatter, config)}
}
func NewTokenizerWithContext(input string, frontmatter FrontmatterAllowed, context rustfmt.Context) *Tokenizer {
	return &Tokenizer{NewCursorWithContext(input, frontmatter, context)}
}
func NewTokenizerWithConfigAndContext(input string, frontmatter FrontmatterAllowed, config CursorConfig, context rustfmt.Context) *Tokenizer {
	return &Tokenizer{NewCursorWithConfigAndContext(input, frontmatter, config, context)}
}
func TokenizeIteratorWithConfig(input string, frontmatter FrontmatterAllowed, config CursorConfig) *Tokenizer {
	return NewTokenizerWithConfig(input, frontmatter, config)
}
func TokenizeIteratorWithContext(input string, frontmatter FrontmatterAllowed, context rustfmt.Context) *Tokenizer {
	return NewTokenizerWithContext(input, frontmatter, context)
}
func TokenizeIteratorWithConfigAndContext(input string, frontmatter FrontmatterAllowed, config CursorConfig, context rustfmt.Context) *Tokenizer {
	return NewTokenizerWithConfigAndContext(input, frontmatter, config, context)
}
func TokenizeWithConfig(input string, frontmatter FrontmatterAllowed, config CursorConfig) []Token {
	return TokenizeWithConfigAndContext(input, frontmatter, config, rustfmt.Context{})
}
func TokenizeWithContext(input string, frontmatter FrontmatterAllowed, context rustfmt.Context) []Token {
	return TokenizeWithConfigAndContext(input, frontmatter, DefaultCursorConfig(), context)
}
func TokenizeWithConfigAndContext(input string, frontmatter FrontmatterAllowed, config CursorConfig, context rustfmt.Context) []Token {
	it := NewTokenizerWithConfigAndContext(input, frontmatter, config, context)
	out := []Token{}
	for {
		token, ok := it.Next()
		if !ok {
			return out
		}
		out = append(out, token)
	}
}
func ValidateRawStrWithConfig(input string, prefixLen uint32, config CursorConfig) *RawStrError {
	return ValidateRawStrWithConfigAndContext(input, prefixLen, config, rustfmt.Context{})
}
func ValidateRawStrWithContext(input string, prefixLen uint32, context rustfmt.Context) *RawStrError {
	return ValidateRawStrWithConfigAndContext(input, prefixLen, DefaultCursorConfig(), context)
}
func ValidateRawStrWithConfigAndContext(input string, prefixLen uint32, config CursorConfig, context rustfmt.Context) *RawStrError {
	cursor := NewCursorWithConfigAndContext(input, FrontmatterNo, config, context)
	cursor.debugAssert(input != "")
	for i := uint32(0); i < prefixLen; i++ {
		if _, ok := cursor.Bump(); !ok {
			cursor.context.Panic("rustclexer: raw string prefix exceeds input")
		}
	}
	_, err := cursor.rawDoubleQuotedString(prefixLen)
	return err
}
