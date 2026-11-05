// SPDX-License-Identifier: MIT OR Apache-2.0
// Ident, IdentPrinter and MacroRulesNormalizedIdent from symbol.rs.
package rustcspan

import (
	"runtime"
	"strings"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Ident.Equal/Hash use Name and hygiene context only. Builtin Go equality also
// includes positions; use CanonicalKey for host maps that model source keys.
type Ident struct {
	Name Symbol
	Span Span
}

// IdentKey is a Go-only immutable comparable view of the source Eq/Hash key.
// It adds no upstream public API or source-file credit.
type IdentKey struct {
	name    Symbol
	context SyntaxContext
}

func (value Ident) CanonicalKey() IdentKey { return IdentKey{value.Name, value.Span.Ctxt()} }

func NewIdent(session *SessionGlobals, name Symbol, span Span, config ...SpanEncodingConfig) Ident {
	_, file, line, _ := runtime.Caller(1)
	return NewIdentAtWithContext(session, name, span, SourceLocation{File: file, Line: uint32(line), Host: true}, rustfmt.Context{}, config...)
}
func NewIdentWithContext(session *SessionGlobals, name Symbol, span Span, context rustfmt.Context, config ...SpanEncodingConfig) Ident {
	_, file, line, _ := runtime.Caller(1)
	return NewIdentAtWithContext(session, name, span, SourceLocation{File: file, Line: uint32(line), Host: true}, context, config...)
}

// NewIdentAt is the primary track_caller witness. Direct translated callers
// supply their source location; the convenience constructors above mark host
// captures explicitly instead of claiming an exact Rust caller location.
func NewIdentAt(session *SessionGlobals, name Symbol, span Span, location SourceLocation, config ...SpanEncodingConfig) Ident {
	return NewIdentAtWithContext(session, name, span, location, rustfmt.Context{}, config...)
}
func NewIdentAtWithContext(session *SessionGlobals, name Symbol, span Span, location SourceLocation, context rustfmt.Context, config ...SpanEncodingConfig) Ident {
	if spanEncodingConfig(session, config...).DebugAssertions && name == Sym_empty() {
		SpanBugAtWithContext(session, span, "called `Ident::new` with empty symbol", location, context)
	}
	return Ident{Name: name, Span: span}
}

const (
	PanicIdentDummySpanSite  = "rustc_span.symbol.ident.with_dummy_span"
	PanicIdentFromSpanSite   = "rustc_span.symbol.ident.from_str_and_span"
	PanicIdentSpanPosSite    = "rustc_span.symbol.ident.with_span_pos"
	PanicIdentQuoteSite      = "rustc_span.symbol.ident.without_first_quote"
	PanicIdentMacros20Site   = "rustc_span.symbol.ident.normalize_to_macros_2_0"
	PanicIdentMacroRulesSite = "rustc_span.symbol.ident.normalize_to_macro_rules"
)

// NewIdentAtLocationCell is the primary borrowed track_caller witness. Unlike
// the scalar At convenience, the exact immutable location cell reaches both
// diagnostic and panic-hook callbacks without conversion or reconstruction.
func NewIdentAtLocationCell(session *SessionGlobals, name Symbol, span Span, cell *rustfmt.SourceLocation, config ...SpanEncodingConfig) Ident {
	return NewIdentAtLocationCellWithContext(session, name, span, cell, rustfmt.Context{}, config...)
}
func NewIdentAtLocationCellWithContext(session *SessionGlobals, name Symbol, span Span, cell *rustfmt.SourceLocation, context rustfmt.Context, config ...SpanEncodingConfig) Ident {
	if spanEncodingConfig(session, config...).DebugAssertions && name == Sym_empty() {
		SpanBugAtLocationCellWithContext(session, span, "called `Ident::new` with empty symbol", cell, context)
	}
	return Ident{Name: name, Span: span}
}

func identAtSymbolCallsite(session *SessionGlobals, name Symbol, span Span, site string, line, column uint32, context rustfmt.Context) Ident {
	// The logical projection remains useful to diagnostic consumers, but only
	// an explicit selected-build profile supplies an exact native panic site.
	location := SourceLocation{File: "compiler/rustc_span/src/symbol.rs", Line: line, Column: column, Host: true}
	if actual := context.SourcePanicLocation(site); actual != nil {
		return NewIdentAtLocationCellWithContext(session, name, span, actual, context)
	}
	return NewIdentAtWithContext(session, name, span, location, context)
}
func NewIdentWithDummySpan(session *SessionGlobals, name Symbol) Ident {
	return NewIdentWithDummySpanWithContext(session, name, rustfmt.Context{})
}
func NewIdentWithDummySpanWithContext(session *SessionGlobals, name Symbol, context rustfmt.Context) Ident {
	return identAtSymbolCallsite(session, name, DummySpan(), PanicIdentDummySpanSite, 2495, 9, context)
}
func DummyIdent(session *SessionGlobals) Ident { return NewIdentWithDummySpan(session, SymDummy()) }
func IdentFromStr(session *SessionGlobals, text string) Ident {
	return NewIdentWithDummySpan(session, InternSymbol(session, text))
}
func IdentFromStrWithContext(session *SessionGlobals, text string, context rustfmt.Context) Ident {
	return identAtSymbolCallsite(session, InternSymbolWithContext(session, text, context), DummySpan(), PanicIdentDummySpanSite, 2495, 9, context)
}
func IdentFromStrAndSpan(session *SessionGlobals, text string, span Span) Ident {
	return IdentFromStrAndSpanWithContext(session, text, span, rustfmt.Context{})
}
func IdentFromStrAndSpanWithContext(session *SessionGlobals, text string, span Span, context rustfmt.Context) Ident {
	return identAtSymbolCallsite(session, InternSymbolWithContext(session, text, context), span, PanicIdentFromSpanSite, 2515, 9, context)
}
func (value Ident) Clone() Ident           { return value }
func (value *Ident) CloneFrom(other Ident) { *value = other }
func (value Ident) WithSpanPos(session *SessionGlobals, span Span) Ident {
	return value.WithSpanPosWithContext(session, span, rustfmt.Context{})
}
func (value Ident) WithSpanPosWithContext(session *SessionGlobals, span Span, context rustfmt.Context) Ident {
	ctxt := value.Span.Ctxt()
	positioned := span.mapCtxtWithContext(session, func(SyntaxContext) SyntaxContext { return ctxt }, spanEncodingConfig(session), context)
	return identAtSymbolCallsite(session, value.Name, positioned, PanicIdentSpanPosSite, 2520, 9, context)
}
func (value Ident) WithoutFirstQuote(session *SessionGlobals) Ident {
	return value.WithoutFirstQuoteWithContext(session, rustfmt.Context{})
}
func (value Ident) WithoutFirstQuoteWithContext(session *SessionGlobals, context rustfmt.Context) Ident {
	if text, present := strings.CutPrefix(value.AsStrWithContext(session, context), "'"); present {
		return identAtSymbolCallsite(session, InternSymbolWithContext(session, text, context), value.Span, PanicIdentQuoteSite, 2528, 34, context)
	}
	return value
}
func (value Ident) NormalizeToMacros20(session *SessionGlobals) Ident {
	return value.NormalizeToMacros20WithContext(session, rustfmt.Context{})
}
func (value Ident) NormalizeToMacros20WithContext(session *SessionGlobals, context rustfmt.Context) Ident {
	span := value.Span.mapCtxtWithContext(session, func(ctxt SyntaxContext) SyntaxContext { return ctxt.NormalizeToMacros20WithContext(session, context) }, spanEncodingConfig(session), context)
	return identAtSymbolCallsite(session, value.Name, span, PanicIdentMacros20Site, 2537, 9, context)
}
func (value Ident) NormalizeToMacroRules(session *SessionGlobals) Ident {
	return value.NormalizeToMacroRulesWithContext(session, rustfmt.Context{})
}
func (value Ident) NormalizeToMacroRulesWithContext(session *SessionGlobals, context rustfmt.Context) Ident {
	span := value.Span.mapCtxtWithContext(session, func(ctxt SyntaxContext) SyntaxContext { return ctxt.NormalizeToMacroRulesWithContext(session, context) }, spanEncodingConfig(session), context)
	return identAtSymbolCallsite(session, value.Name, span, PanicIdentMacroRulesSite, 2547, 9, context)
}
func (value Ident) AsStr(session *SessionGlobals) string { return value.Name.AsStr(session) }
func (value Ident) AsStrWithContext(session *SessionGlobals, context rustfmt.Context) string {
	return value.Name.AsStrWithContext(session, context)
}
func (value Ident) Equal(other Ident) bool {
	return value.Name == other.Name && value.Span.Ctxt() == other.Span.Ctxt()
}
func (value Ident) NotEqual(other Ident) bool { return !value.Equal(other) }
func (value Ident) Hash(hasher stablehash.Hasher) {
	value.Name.Hash(hasher)
	stablehash.HashU32(hasher, value.Span.Ctxt().AsU32())
}
func (value Ident) StableHash(session *SessionGlobals, context datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	value.StableHashWithContext(session, context, hasher, rustfmt.Context{})
}
func (value Ident) StableHashWithContext(session *SessionGlobals, context datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ambient rustfmt.Context) {
	value.Name.StableHashWithContext(session, context, hasher, ambient)
	datastructures.StableHashSpanWithContext(context, value.Span.ToRawSpan(), hasher, ambient)
}
func (value Ident) IsSpecial() bool { return value.Name.isSpecial() }
func (value Ident) IsUsedKeyword(session *SessionGlobals) bool {
	return value.IsUsedKeywordWithContext(session, rustfmt.Context{})
}
func (value Ident) IsUsedKeywordWithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.Name.isUsedKeywordAlways() || value.Name.isUsedKeywordConditional(func() Edition { return value.Span.Ctxt().EditionWithContext(session, context) })
}
func (value Ident) IsUnusedKeyword(session *SessionGlobals) bool {
	return value.IsUnusedKeywordWithContext(session, rustfmt.Context{})
}
func (value Ident) IsUnusedKeywordWithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.Name.isUnusedKeywordAlways() || value.Name.isUnusedKeywordConditional(func() Edition { return value.Span.Ctxt().EditionWithContext(session, context) })
}
func (value Ident) IsReserved(session *SessionGlobals) bool {
	return value.IsReservedWithContext(session, rustfmt.Context{})
}
func (value Ident) IsReservedWithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.Name.IsReserved(func() Edition { return value.Span.Ctxt().EditionWithContext(session, context) })
}
func (value Ident) IsPathSegmentKeyword() bool { return value.Name.IsPathSegmentKeyword() }
func (value Ident) IsRawGuess(session *SessionGlobals) bool {
	return value.isRawGuessWithContext(session, rustfmt.Context{})
}
func (value Ident) IsRawGuessWithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.isRawGuessWithContext(session, context)
}
func (value Ident) isRawGuessWithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.Name.CanBeRaw() && value.IsReservedWithContext(session, context)
}
func (value Ident) IsReservedLifetime(session *SessionGlobals) bool {
	return value.isReservedLifetimeWithContext(session, rustfmt.Context{})
}
func (value Ident) IsReservedLifetimeWithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.isReservedLifetimeWithContext(session, context)
}
func (value Ident) isReservedLifetimeWithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.IsReservedWithContext(session, context) && value.Name != KwUnderscore() && value.Name != KwStatic()
}
func (value Ident) IsRawLifetimeGuess(session *SessionGlobals) bool {
	return value.isRawLifetimeGuessWithContext(session, rustfmt.Context{})
}
func (value Ident) IsRawLifetimeGuessWithContext(session *SessionGlobals, context rustfmt.Context) bool {
	return value.isRawLifetimeGuessWithContext(session, context)
}
func (value Ident) isRawLifetimeGuessWithContext(session *SessionGlobals, context rustfmt.Context) bool {
	if value.Name.AsStrWithContext(session, context) == "'" {
		return false
	}
	withoutQuote := value.WithoutFirstQuoteWithContext(session, context)
	return withoutQuote.Name != value.Name && withoutQuote.Name.CanBeRaw() && withoutQuote.isReservedLifetimeWithContext(session, context)
}
func (value Ident) GuessPrintMode(session *SessionGlobals) IdentPrintMode {
	return value.GuessPrintModeWithContext(session, rustfmt.Context{})
}
func (value Ident) GuessPrintModeWithContext(session *SessionGlobals, context rustfmt.Context) IdentPrintMode {
	if value.isRawLifetimeGuessWithContext(session, context) {
		return IdentRawLifetime
	}
	if value.isRawGuessWithContext(session, context) {
		return IdentRawIdent
	}
	return IdentNormal
}
func (value Ident) IsNumeric(session *SessionGlobals) bool {
	text := value.AsStr(session)
	for i := 0; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return true
}
func (value Symbol) ToIdentString(session *SessionGlobals) string {
	return value.ToIdentStringWithContext(session, rustfmt.Context{})
}
func (value Symbol) ToIdentStringWithContext(session *SessionGlobals, context rustfmt.Context) string {
	if value == Sym_empty() {
		return ""
	}
	ident := identAtSymbolCallsite(session, value, DummySpan(), PanicIdentDummySpanSite, 2495, 9, context)
	var text strings.Builder
	f := rustfmt.Formatter{Writer: &text, Options: rustfmt.DefaultOptions(), Context: context}
	if err := ident.WriteDisplayFormatter(session, &f); err != nil {
		symbolToStringError(context)
	}
	return text.String()
}

type IdentPrintMode uint8

const (
	IdentNormal IdentPrintMode = iota
	IdentRawIdent
	IdentForcedKeywordIdent
	IdentRawLifetime
)

type IdentPrinter struct {
	symbol             Symbol
	mode               IdentPrintMode
	convertDollarCrate Option[Span]
}

func NewIdentPrinter(symbol Symbol, mode IdentPrintMode, convertDollarCrate Option[Span]) IdentPrinter {
	if mode > IdentRawLifetime {
		panic("rustcspan: invalid IdentPrintMode discriminant")
	}
	return IdentPrinter{symbol, mode, convertDollarCrate}
}
func IdentPrinterForASTIdent(ident Ident, mode IdentPrintMode) IdentPrinter {
	return NewIdentPrinter(ident.Name, mode, Some(ident.Span))
}
func (value IdentPrinter) WriteDisplayFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	context := f.Context
	defer spanDebugContextGuard(context)
	symbol := value.symbol
	switch value.mode {
	case IdentNormal:
		if span, present := value.convertDollarCrate.Get(); symbol == KwDollarCrate() && present {
			symbol = span.Ctxt().DollarCrateNameWithContext(session, context)
			if !symbol.IsPathSegmentKeyword() {
				if err := f.WriteString("::"); err != nil {
					return err
				}
			}
		}
	case IdentRawIdent:
		if err := f.WriteString("r#"); err != nil {
			return err
		}
	case IdentForcedKeywordIdent:
		if err := f.WriteString("k#"); err != nil {
			return err
		}
	case IdentRawLifetime:
		if err := f.WriteString("'r#"); err != nil {
			return err
		}
		text, present := strings.CutPrefix(value.symbol.AsStrWithContext(session, context), "'")
		if !present {
			symbolOwnedPanic(context, PanicIdentRawLifetimeSite, func(rustfmt.Context) any { return "only lifetime idents should be passed with RawLifetime mode" })
		}
		symbol = InternSymbolWithContext(session, text, context)
	default:
		context.Panic("rustcspan: invalid IdentPrintMode discriminant")
	}
	return symbol.WriteDisplayFormatter(session, f)
}
func (value Ident) WriteDisplayFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	context := f.Context
	defer spanDebugContextGuard(context)
	return NewIdentPrinter(value.Name, value.GuessPrintModeWithContext(session, context), None[Span]()).WriteDisplayFormatter(session, f)
}
func (value Ident) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	context := f.Context
	defer spanDebugContextGuard(context)
	if err := value.WriteDisplayFormatter(session, f); err != nil {
		return err
	}
	return value.Span.Ctxt().WriteDebugFormatter(f)
}
func (value Ident) DisplayString(session *SessionGlobals) string {
	return rustfmt.String(func(f *rustfmt.Formatter) error { return value.WriteDisplayFormatter(session, f) })
}
func (value Ident) RustDebug(session *SessionGlobals) string {
	return rustfmt.String(func(f *rustfmt.Formatter) error { return value.WriteDebugFormatter(session, f) })
}
func (value IdentPrinter) DisplayString(session *SessionGlobals) string {
	return rustfmt.String(func(f *rustfmt.Formatter) error { return value.WriteDisplayFormatter(session, f) })
}

type MacroRulesNormalizedIdent struct{ ident Ident }

func (value MacroRulesNormalizedIdent) CanonicalKey() IdentKey { return value.ident.CanonicalKey() }

func NewMacroRulesNormalizedIdent(session *SessionGlobals, ident Ident) MacroRulesNormalizedIdent {
	return NewMacroRulesNormalizedIdentWithContext(session, ident, rustfmt.Context{})
}
func NewMacroRulesNormalizedIdentWithContext(session *SessionGlobals, ident Ident, context rustfmt.Context) MacroRulesNormalizedIdent {
	return MacroRulesNormalizedIdent{ident.NormalizeToMacroRulesWithContext(session, context)}
}
func (value MacroRulesNormalizedIdent) Symbol() Symbol                             { return value.ident.Name }
func (value MacroRulesNormalizedIdent) Ident() Ident                               { return value.ident }
func (value MacroRulesNormalizedIdent) Clone() MacroRulesNormalizedIdent           { return value }
func (value *MacroRulesNormalizedIdent) CloneFrom(other MacroRulesNormalizedIdent) { *value = other }
func (value MacroRulesNormalizedIdent) Equal(other MacroRulesNormalizedIdent) bool {
	return value.ident.Equal(other.ident)
}
func (value MacroRulesNormalizedIdent) NotEqual(other MacroRulesNormalizedIdent) bool {
	return !value.Equal(other)
}
func (value MacroRulesNormalizedIdent) Hash(hasher stablehash.Hasher) {
	value.ident.Hash(hasher)
}
func (value MacroRulesNormalizedIdent) WriteDisplayFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	return value.ident.WriteDisplayFormatter(session, f)
}
func (value MacroRulesNormalizedIdent) WriteDebugFormatter(session *SessionGlobals, f *rustfmt.Formatter) error {
	return value.ident.WriteDebugFormatter(session, f)
}

// The original derives delegate fields in declaration order and construct the
// decoded struct directly; decoding does not call Ident::new's debug assertion.
type IdentEncoder = SpanEncoder
type IdentDecoder = SpanDecoder

func (value Ident) Encode(encoder IdentEncoder) {
	encoder.EncodeSymbol(value.Name)
	encoder.EncodeSpan(value.Span)
}
func DecodeIdent(decoder IdentDecoder) Ident {
	return Ident{Name: decoder.DecodeSymbol(), Span: decoder.DecodeSpan()}
}
