// SPDX-License-Identifier: MIT OR Apache-2.0
// Span text/extension operations from the pinned source_map.rs.
package rustcspan

import (
	"strings"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Rust char::is_whitespace uses White_Space, independently of rustc_lexer's
// Pattern_White_Space. These exact scalar ranges are stable at source Unicode18.
func sourceMapWhitespace(c rune) bool {
	return c >= '\t' && c <= '\r' || c == ' ' || c == '\u0085' || c == '\u00a0' || c == '\u1680' || c >= '\u2000' && c <= '\u200a' || c == '\u2028' || c == '\u2029' || c == '\u202f' || c == '\u205f' || c == '\u3000'
}
func (sm *SourceMap) IndentationBefore(session *SessionGlobals, sp Span) Option[string] {
	return sm.IndentationBeforeWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) IndentationBeforeWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) Option[string] {
	text, failure := SpanToSourceWithContext(sm, session, sp, func(src string, begin, _ uint) (string, *SpanSnippetError) {
		before := sourceMapStringSlice(src, 0, begin, context, SourceMapPanicIndentSlice)
		if index := strings.LastIndexByte(before, '\n'); index >= 0 {
			before = before[index+1:]
		}
		for index, char := range before {
			if !sourceMapWhitespace(char) {
				return strings.Clone(before[:index]), nil
			}
		}
		return strings.Clone(before), nil
	}, context)
	if failure != nil {
		return None[string]()
	}
	return Some(text)
}
func (sm *SourceMap) SpanToMargin(session *SessionGlobals, sp Span) Option[uint] {
	return sm.SpanToMarginWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanToMarginWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) Option[uint] {
	if indent, ok := sm.IndentationBeforeWithContext(session, sp, context).Get(); ok {
		return Some(uint(len(indent)))
	}
	return None[uint]()
}
func (sm *SourceMap) IsLineBeforeSpanEmpty(session *SessionGlobals, sp Span) bool {
	return sm.IsLineBeforeSpanEmptyWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) IsLineBeforeSpanEmptyWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) bool {
	text, failure := sm.SpanToPrevSourceWithContext(session, sp, context)
	if failure != nil {
		return false
	}
	if index := strings.LastIndexByte(text, '\n'); index >= 0 {
		text = text[index+1:]
	}
	return strings.TrimLeftFunc(text, sourceMapWhitespace) == ""
}

func (sm *SourceMap) SpanExtendToPrevChar(session *SessionGlobals, sp Span, char rune, acceptNewlines bool) Span {
	return sm.SpanExtendToPrevCharWithContext(session, sp, char, acceptNewlines, rustfmt.Context{})
}
func (sm *SourceMap) SpanExtendToPrevCharWithContext(session *SessionGlobals, sp Span, char rune, acceptNewlines bool, context rustfmt.Context) Span {
	if !utf8.ValidRune(char) {
		context.Panic("Rust char must be a valid scalar")
	}
	if text, failure := sm.SpanToPrevSourceWithContext(session, sp, context); failure == nil {
		if index := strings.LastIndex(text, string(char)); index >= 0 {
			text = text[index+utf8.RuneLen(char):]
		}
		if text != "" && (acceptNewlines || !strings.ContainsRune(text, '\n')) {
			lo := BytePos(sourceMapU32Sub(uint32(sourceMapSpanLo(sp, session, sm.config, context)), uint32(len(text)), sm.config, context, "rustc_span::source_map::span_extend_to_prev_char.subtract"))
			return sourceMapSpanWithLo(sp, session, lo, sm.config, context)
		}
	}
	return sp
}
func (sm *SourceMap) SpanExtendToPrevCharBefore(session *SessionGlobals, sp Span, char rune, acceptNewlines bool) Span {
	return sm.SpanExtendToPrevCharBeforeWithContext(session, sp, char, acceptNewlines, rustfmt.Context{})
}
func (sm *SourceMap) SpanExtendToPrevCharBeforeWithContext(session *SessionGlobals, sp Span, char rune, acceptNewlines bool, context rustfmt.Context) Span {
	if !utf8.ValidRune(char) {
		context.Panic("Rust char must be a valid scalar")
	}
	if text, failure := sm.SpanToPrevSourceWithContext(session, sp, context); failure == nil {
		if index := strings.LastIndex(text, string(char)); index >= 0 {
			text = text[index+utf8.RuneLen(char):]
		}
		if acceptNewlines || !strings.ContainsRune(text, '\n') {
			lo := BytePos(sourceMapU32Sub(sourceMapU32Sub(uint32(sourceMapSpanLo(sp, session, sm.config, context)), uint32(len(text)), sm.config, context, "rustc_span::source_map::span_extend_to_prev_char_before.subtract_length"), 1, sm.config, context, "rustc_span::source_map::span_extend_to_prev_char_before.subtract_one"))
			return sourceMapSpanWithLo(sp, session, lo, sm.config, context)
		}
	}
	return sp
}
func (sm *SourceMap) SpanExtendToPrevStr(session *SessionGlobals, sp Span, pattern string, acceptNewlines, includeWhitespace bool) Option[Span] {
	return sm.SpanExtendToPrevStrWithContext(session, sp, pattern, acceptNewlines, includeWhitespace, rustfmt.Context{})
}
func (sm *SourceMap) SpanExtendToPrevStrWithContext(session *SessionGlobals, sp Span, pattern string, acceptNewlines, includeWhitespace bool, context rustfmt.Context) Option[Span] {
	requireSpanUTF8(pattern, context)
	text, failure := sm.SpanToPrevSourceWithContext(session, sp, context)
	if failure != nil {
		return None[Span]()
	}
	for _, whitespace := range []string{" ", "\t", "\n"} {
		pat := pattern + whitespace
		if index := strings.LastIndex(text, pat); index >= 0 {
			after := index + len(pat) - 1
			if includeWhitespace {
				for offset, char := range text[after:] {
					if !sourceMapWhitespace(char) {
						after += offset
						break
					}
				}
			}
			length := len(text) - after
			if acceptNewlines || !strings.ContainsRune(strings.TrimLeftFunc(text[after:], sourceMapWhitespace), '\n') {
				lo := BytePos(sourceMapU32Sub(uint32(sourceMapSpanLo(sp, session, sm.config, context)), uint32(length), sm.config, context, "rustc_span::source_map::span_extend_to_prev_str.subtract"))
				return Some(sourceMapSpanWithLo(sp, session, lo, sm.config, context))
			}
		}
	}
	return None[Span]()
}

// Owned predicates carry all captured fields and their complete destructor.
// ExtendWhile is FnMut(char); ExtendPrevWhile has the source Fn(char) shared
// receiver law. Neither may retain a borrow of the call's environment cell.
func SpanExtendWhileOwned[E any](sm *SourceMap, session *SessionGlobals, sp Span, environment *SpanOwned[E], predicate func(*E, rune, rustfmt.Context) bool, context rustfmt.Context) (Span, *SpanSnippetError) {
	env := environment.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, env.DropWithContext)
	return SpanToSourceWithContext(sm, session, sp, func(text string, _, end uint) (Span, *SpanSnippetError) {
		after := sourceMapStringSlice(text, end, uint(len(text)), context, SourceMapPanicExtendSlice)
		length := uint(len(after))
		for index, char := range after {
			match := false
			spanRunContext(context, func() { match = predicate(env.Borrow(), char, context) })
			if !match {
				length = uint(index)
				break
			}
		}
		hi := sourceMapSpanHi(sp, session, sm.config, context).AddWithContext(BytePos(uint32(length)), sm.config, context)
		return sourceMapSpanWithHi(sp, session, hi, sm.config, context), nil
	}, context)
}
func SpanExtendPrevWhileOwned[E any](sm *SourceMap, session *SessionGlobals, sp Span, environment *SpanOwned[E], predicate func(*E, rune, rustfmt.Context) bool, context rustfmt.Context) (Span, *SpanSnippetError) {
	env := environment.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, env.DropWithContext)
	return SpanToSourceWithContext(sm, session, sp, func(text string, begin, _ uint) (Span, *SpanSnippetError) {
		before := sourceMapStringSlice(text, 0, begin, context, SourceMapPanicExtendPrevSlice)
		length := begin
		for end := len(before); end > 0; {
			char, size := utf8.DecodeLastRuneInString(before[:end])
			index := end - size
			match := false
			spanRunContext(context, func() { match = predicate(env.Borrow(), char, context) })
			if !match {
				length = begin - uint(index) - uint(size)
				break
			}
			end = index
		}
		lo := BytePos(sourceMapU32Sub(uint32(sourceMapSpanLo(sp, session, sm.config, context)), uint32(length), sm.config, context, "rustc_span::source_map::span_extend_to_prev_str.subtract"))
		return sourceMapSpanWithLo(sp, session, lo, sm.config, context), nil
	}, context)
}
func (sm *SourceMap) SpanExtendWhile(session *SessionGlobals, sp Span, predicate func(rune) bool) (Span, *SpanSnippetError) {
	return sm.SpanExtendWhileWithContext(session, sp, predicate, rustfmt.Context{})
}
func (sm *SourceMap) SpanExtendWhileWithContext(session *SessionGlobals, sp Span, predicate func(rune) bool, context rustfmt.Context) (Span, *SpanSnippetError) {
	return SpanExtendWhileOwned(sm, session, sp, NewSpanOwned(struct{}{}, nil), func(_ *struct{}, char rune, _ rustfmt.Context) bool { return predicate(char) }, context)
}
func (sm *SourceMap) SpanExtendPrevWhile(session *SessionGlobals, sp Span, predicate func(rune) bool) (Span, *SpanSnippetError) {
	return sm.SpanExtendPrevWhileWithContext(session, sp, predicate, rustfmt.Context{})
}
func (sm *SourceMap) SpanExtendPrevWhileWithContext(session *SessionGlobals, sp Span, predicate func(rune) bool, context rustfmt.Context) (Span, *SpanSnippetError) {
	return SpanExtendPrevWhileOwned(sm, session, sp, NewSpanOwned(struct{}{}, nil), func(_ *struct{}, char rune, _ rustfmt.Context) bool { return predicate(char) }, context)
}
func (sm *SourceMap) SpanExtendWhileWhitespace(session *SessionGlobals, sp Span) Span {
	return sm.SpanExtendWhileWhitespaceWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanExtendWhileWhitespaceWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) Span {
	if result, failure := sm.SpanExtendWhileWithContext(session, sp, sourceMapWhitespace, context); failure == nil {
		return result
	}
	return sp
}
func (sm *SourceMap) SpanExtendToNextChar(session *SessionGlobals, sp Span, char rune, acceptNewlines bool) Span {
	return sm.SpanExtendToNextCharWithContext(session, sp, char, acceptNewlines, rustfmt.Context{})
}
func (sm *SourceMap) SpanExtendToNextCharWithContext(session *SessionGlobals, sp Span, char rune, acceptNewlines bool, context rustfmt.Context) Span {
	if !utf8.ValidRune(char) {
		context.Panic("Rust char must be a valid scalar")
	}
	if text, failure := sm.SpanToNextSourceWithContext(session, sp, context); failure == nil {
		if index := strings.IndexRune(text, char); index >= 0 {
			text = text[:index]
		}
		if text != "" && (acceptNewlines || !strings.ContainsRune(text, '\n')) {
			hi := BytePos(sourceMapU32Add(uint32(sourceMapSpanHi(sp, session, sm.config, context)), uint32(len(text)), sm.config, context, "rustc_span::source_map::span_extend_to_next_char.add"))
			return sourceMapSpanWithHi(sp, session, hi, sm.config, context)
		}
	}
	return sp
}
func (sm *SourceMap) SpanExtendToLine(session *SessionGlobals, sp Span) Span {
	return sm.SpanExtendToLineWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanExtendToLineWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) Span {
	return sm.SpanExtendToPrevCharWithContext(session, sm.SpanExtendToNextCharWithContext(session, sp, '\n', true, context), '\n', true, context)
}
func (sm *SourceMap) SpanUntilChar(session *SessionGlobals, sp Span, char rune) Span {
	return sm.SpanUntilCharWithContext(session, sp, char, rustfmt.Context{})
}
func (sm *SourceMap) SpanUntilCharWithContext(session *SessionGlobals, sp Span, char rune, context rustfmt.Context) Span {
	if !utf8.ValidRune(char) {
		context.Panic("Rust char must be a valid scalar")
	}
	if text, failure := sm.SpanToSnippetWithContext(session, sp, context); failure == nil {
		if index := strings.IndexRune(text, char); index >= 0 {
			text = text[:index]
		}
		text = strings.TrimRightFunc(text, sourceMapWhitespace)
		if text != "" && !strings.ContainsRune(text, '\n') {
			hi := BytePos(sourceMapU32Add(uint32(sourceMapSpanLo(sp, session, sm.config, context)), uint32(len(text)), sm.config, context, "rustc_span::source_map::span_until_char.add"))
			return sourceMapSpanWithHi(sp, session, hi, sm.config, context)
		}
	}
	return sp
}
func (sm *SourceMap) SpanThroughChar(session *SessionGlobals, sp Span, char rune) Span {
	return sm.SpanThroughCharWithContext(session, sp, char, rustfmt.Context{})
}
func (sm *SourceMap) SpanThroughCharWithContext(session *SessionGlobals, sp Span, char rune, context rustfmt.Context) Span {
	if !utf8.ValidRune(char) {
		context.Panic("Rust char must be a valid scalar")
	}
	if text, failure := sm.SpanToSnippetWithContext(session, sp, context); failure == nil {
		if index := strings.IndexRune(text, char); index >= 0 {
			hi := BytePos(sourceMapU32Add(uint32(sourceMapSpanLo(sp, session, sm.config, context)), uint32(index+utf8.RuneLen(char)), sm.config, context, "rustc_span::source_map::span_through_char.add"))
			return sourceMapSpanWithHi(sp, session, hi, sm.config, context)
		}
	}
	return sp
}

// SpanTakeWhileOwned consumes the predicate. Its char pointer is a shared,
// read-only borrow of the current temporary and expires at callback return.
// Providers may copy its value/observe alias identity, but cannot mutate it or
// retain the reference beyond that borrow. E is the FnMut environment.
func SpanTakeWhileOwned[E any](sm *SourceMap, session *SessionGlobals, sp Span, environment *SpanOwned[E], predicate func(*E, *rune, rustfmt.Context) bool, context rustfmt.Context) Span {
	env := environment.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, env.DropWithContext)
	text, failure := sm.SpanToSnippetWithContext(session, sp, context)
	if failure != nil {
		return sp
	}
	offset := func() uint {
		defer spanCloseOwnedCallback(context, env.DropWithContext)
		length := uint(0)
		var cell rune
		for _, char := range text {
			// Iterator::next reuses its current item place between calls.
			// The borrow expires on each predicate return; pointer-integer
			// alias observations still see that same temporary cell.
			cell = char
			match := false
			spanRunContext(context, func() { match = predicate(env.Borrow(), &cell, context) })
			if !match {
				break
			}
			length = spanCharPosAdd(length, uint(utf8.RuneLen(char)), sm.config, context)
		}
		return length
	}()
	hi := BytePos(sourceMapU32Add(uint32(sourceMapSpanLo(sp, session, sm.config, context)), uint32(offset), sm.config, context, "rustc_span::source_map::span_take_while.add"))
	return sourceMapSpanWithHi(sp, session, hi, sm.config, context)
}
func (sm *SourceMap) SpanTakeWhile(session *SessionGlobals, sp Span, predicate func(*rune) bool) Span {
	return sm.SpanTakeWhileWithContext(session, sp, predicate, rustfmt.Context{})
}
func (sm *SourceMap) SpanTakeWhileWithContext(session *SessionGlobals, sp Span, predicate func(*rune) bool, context rustfmt.Context) Span {
	return SpanTakeWhileOwned(sm, session, sp, NewSpanOwned(struct{}{}, nil), func(_ *struct{}, char *rune, _ rustfmt.Context) bool { return predicate(char) }, context)
}
func (sm *SourceMap) SpanUntilNonWhitespace(session *SessionGlobals, sp Span) Span {
	return sm.SpanUntilNonWhitespaceWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanUntilNonWhitespaceWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) Span {
	found := false
	return sm.SpanTakeWhileWithContext(session, sp, func(char *rune) bool {
		if !found && sourceMapWhitespace(*char) {
			found = true
		}
		return !found || sourceMapWhitespace(*char)
	}, context)
}
func (sm *SourceMap) SpanUntilWhitespace(session *SessionGlobals, sp Span) Span {
	return sm.SpanUntilWhitespaceWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanUntilWhitespaceWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) Span {
	return sm.SpanTakeWhileWithContext(session, sp, func(char *rune) bool { return !sourceMapWhitespace(*char) }, context)
}
func (sm *SourceMap) GuessHeadSpan(session *SessionGlobals, sp Span) Span {
	return sm.SpanUntilChar(session, sp, '{')
}
func (sm *SourceMap) GuessHeadSpanWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) Span {
	return sm.SpanUntilCharWithContext(session, sp, '{', context)
}

func (sm *SourceMap) SpanWrappedByAngleOrParentheses(session *SessionGlobals, sp Span) bool {
	return sm.SpanWrappedByAngleOrParenthesesWithContext(session, sp, rustfmt.Context{})
}
func (sm *SourceMap) SpanWrappedByAngleOrParenthesesWithContext(session *SessionGlobals, sp Span, context rustfmt.Context) bool {
	wrapped, failure := SpanToSourceWithContext(sm, session, sp, func(src string, begin, end uint) (bool, *SpanSnippetError) {
		if !sourceMapStringRangeValid(src, begin, end) {
			return false, nil
		}
		rightAngle, rightParen := false, false
		for _, char := range src[end:] {
			if char == ' ' {
				continue
			}
			if char == '>' {
				rightAngle = true
			} else if char == ')' {
				rightParen = true
			} else {
				return false, nil
			}
			break
		}
		// Preserve the original byte-index/char-nth behavior, including the
		// no-iteration case when begin >= the prefix's scalar count.
		index := begin
		before := src[:begin]
		for {
			var char rune
			found := false
			nth := uint(0)
			for _, c := range before {
				if nth == index {
					char, found = c, true
					break
				}
				nth++
			}
			if !found {
				break
			}
			if char == ' ' {
				if index == 0 {
					return false, nil
				}
				index--
			} else if char == '<' {
				if !rightAngle {
					return false, nil
				}
				break
			} else if char == '(' {
				if !rightParen {
					return false, nil
				}
				break
			} else {
				return false, nil
			}
		}
		return true, nil
	}, context)
	return failure == nil && wrapped
}
