// SPDX-License-Identifier: MIT OR Apache-2.0
// IN PROGRESS full-file port from compiler/rustc_span/src/lib.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. The implementation is split across
// lib_*.go helpers. Sessions initialize the real symbol/hygiene/span/source-map
// runtime. Full-file completion remains pending the remaining dependency and
// original-source validation gates; this header grants no completion credit.
package rustcspan

// Option uses a canonical None representation, preserving the source's absent
// parent without exposing an inactive payload that could change Eq/Hash keys.
type Option[T any] struct {
	value   T
	present bool
}

func Some[T any](value T) Option[T]    { return Option[T]{value: value, present: true} }
func None[T any]() Option[T]           { return Option[T]{} }
func (value Option[T]) Get() (T, bool) { return value.value, value.present }
func (value Option[T]) IsSome() bool   { return value.present }

type BytePos uint32

func BytePosFromU32(value uint32) BytePos  { return BytePos(value) }
func BytePosFromUsize(value uint) BytePos  { return BytePos(uint32(value)) }
func (value BytePos) AsU32() uint32        { return uint32(value) }
func (value BytePos) AsUsize() uint        { return uint(value) }
func (value BytePos) Clone() BytePos       { return value }
func (value BytePos) Hash(h spanU32Hasher) { h.WriteU32(uint32(value)) }

type SpanData struct {
	Lo, Hi BytePos
	Ctxt   SyntaxContext
	Parent Option[LocalDefId]
}

func (value SpanData) Clone() SpanData           { return value }
func (value SpanData) Equal(other SpanData) bool { return value == other }
func (value SpanData) Span(session *SessionGlobals, config ...SpanEncodingConfig) Span {
	return NewSpan(session, value.Lo, value.Hi, value.Ctxt, value.Parent, config...)
}
func (value SpanData) WithLo(session *SessionGlobals, lo BytePos, config ...SpanEncodingConfig) Span {
	return NewSpan(session, lo, value.Hi, value.Ctxt, value.Parent, config...)
}
func (value SpanData) WithHi(session *SessionGlobals, hi BytePos, config ...SpanEncodingConfig) Span {
	return NewSpan(session, value.Lo, hi, value.Ctxt, value.Parent, config...)
}
func (value SpanData) withCtxt(session *SessionGlobals, ctxt SyntaxContext, config ...SpanEncodingConfig) Span {
	return NewOrderedSpan(session, value.Lo, value.Hi, ctxt, value.Parent, config...)
}
func (value SpanData) withParent(session *SessionGlobals, parent Option[LocalDefId], config ...SpanEncodingConfig) Span {
	return NewOrderedSpan(session, value.Lo, value.Hi, value.Ctxt, parent, config...)
}
func (value SpanData) IsDummy() bool { return value.Lo == 0 && value.Hi == 0 }
func (value SpanData) Contains(other SpanData) bool {
	return value.Lo <= other.Lo && other.Hi <= value.Hi
}

type SpanDataHasher interface {
	WriteU32(uint32)
	WriteIsize(int)
}

func (value SpanData) Hash(h SpanDataHasher) {
	value.Lo.Hash(h)
	value.Hi.Hash(h)
	value.Ctxt.Hash(h)
	hashSpanParent(h, value.Parent)
}
func hashSpanParent(h SpanDataHasher, parent Option[LocalDefId]) {
	if value, ok := parent.Get(); ok {
		h.WriteIsize(1)
		value.Hash(h)
	} else {
		h.WriteIsize(0)
	}
}

// SpanEncodingConfig selects both independent original Rust build properties.
// DebugStrictAdd wraps when DebugAssertions is false; when true its ordinary
// addition still follows OverflowChecks, exactly as the original trait body.
type SpanEncodingConfig struct{ DebugAssertions, OverflowChecks bool }

func DefaultSpanEncodingConfig() SpanEncodingConfig { return SpanEncodingConfig{true, true} }

// SessionGlobals owns the real session components. Explicit handles/scopes
// adapt scoped TLS; spans/symbols/contexts retain their originating-session
// lifetime contract. Nil is permitted only on operations avoiding sessions.
type SessionGlobals struct {
	symbolInterner *symbolInterner
	hygieneData    *Lock[HygieneData]
	metavarSpans   *MetavarSpansMap
	sourceMap      Option[Shared[SourceMap]]
	spanMu         spanMutex
	spanInterner   SpanInterner
	spanConfig     SpanEncodingConfig
	spanSourceMap  SpanDiagnosticSourceMap
}

func NewSpanEncodingSession(config SpanEncodingConfig) *SessionGlobals {
	return NewSessionGlobals(DefaultEdition, nil, None[SourceMapInputs](), config)
}

// SpanDiagnosticSourceMap is the earlier borrowed-operation convenience kept
// for compatibility. The primary full constructor installs the actual shared
// SourceMap; no synthetic filename/location fallback is supplied.
// Returned strings must be valid UTF-8, matching the source Rust String domain.
type SpanDiagnosticSourceMap interface {
	SpanToDiagnosticString(*SessionGlobals, Span) string
}

func NewSpanEncodingSessionWithSourceMap(config SpanEncodingConfig, sourceMap SpanDiagnosticSourceMap) *SessionGlobals {
	session := NewSpanEncodingSession(config)
	session.spanSourceMap = sourceMap
	return session
}
func spanEncodingConfig(session *SessionGlobals, explicit ...SpanEncodingConfig) SpanEncodingConfig {
	if len(explicit) > 1 {
		panic("span_encoding: multiple build configurations")
	}
	if len(explicit) == 1 {
		return explicit[0]
	}
	if session == nil {
		return DefaultSpanEncodingConfig()
	}
	return session.spanConfig
}

func (value Span) Lo(session *SessionGlobals, config ...SpanEncodingConfig) BytePos {
	return value.Data(session, config...).Lo
}
func (value Span) Hi(session *SessionGlobals, config ...SpanEncodingConfig) BytePos {
	return value.Data(session, config...).Hi
}
func (value Span) LoHi(session *SessionGlobals, config ...SpanEncodingConfig) (BytePos, BytePos) {
	data := value.Data(session, config...)
	return data.Lo, data.Hi
}
func (value Span) WithLo(session *SessionGlobals, lo BytePos, config ...SpanEncodingConfig) Span {
	profile := spanEncodingConfig(session, config...)
	return value.Data(session, profile).WithLo(session, lo, profile)
}
func (value Span) WithHi(session *SessionGlobals, hi BytePos, config ...SpanEncodingConfig) Span {
	profile := spanEncodingConfig(session, config...)
	return value.Data(session, profile).WithHi(session, hi, profile)
}
func (value Span) WithCtxt(session *SessionGlobals, ctxt SyntaxContext, config ...SpanEncodingConfig) Span {
	return value.MapCtxt(session, func(SyntaxContext) SyntaxContext { return ctxt }, config...)
}
