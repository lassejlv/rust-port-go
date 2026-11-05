// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: rustc_ast/src/util/comments.rs, blob e12818623d8ec3e429475d723c617d08fbf976f5.
package astutil

import (
	rustcast "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_ast/src"
	rustcspan "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	unicodecore "github.com/lassejlv/rust-port-go/port/rust/library/core/src/unicode"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
	"strconv"
	"strings"
	"unicode/utf8"
)

type CommentStyle struct{ tag uint8 }

func Isolated() CommentStyle                                { return CommentStyle{} }
func Trailing() CommentStyle                                { return CommentStyle{tag: 1} }
func Mixed() CommentStyle                                   { return CommentStyle{tag: 2} }
func BlankLine() CommentStyle                               { return CommentStyle{tag: 3} }
func (value CommentStyle) Clone() CommentStyle              { return value }
func (value *CommentStyle) CloneFrom(source CommentStyle)   { *value = source }
func (value CommentStyle) Equal(other CommentStyle) bool    { return value == other }
func (value CommentStyle) NotEqual(other CommentStyle) bool { return !value.Equal(other) }
func (value CommentStyle) WriteDebugFormatter(formatter *rustfmt.Formatter) error {
	names := [...]string{"Isolated", "Trailing", "Mixed", "BlankLine"}
	if value.tag >= uint8(len(names)) {
		formatter.Context.Panic("rustcast: invalid Go CommentStyle variant")
	}
	return formatter.WriteString(names[value.tag])
}
func (value CommentStyle) RustDebug() string { return rustfmt.String(value.WriteDebugFormatter) }

// Comment owns the actual Vec<String> fields. Each Lines element is a mutable
// owning String header, including its buffer capacity. Copying Go descriptors
// is not Rust Clone or Copy: a moved field must not be reused, and borrowed
// headers/text cannot outlive mutation, vector reallocation, move or Drop.
type Comment struct {
	Style    CommentStyle
	Lines    []ruststr.StringCell
	Pos      rustcspan.BytePos
	consumed bool
}

// CommentStringElementLayout supplies the selected source Vec<String> element
// metadata (three usize words), not the physical layout of a Go descriptor.
// The native reference proves the 64-bit profile; the 32-bit result is logical.
func CommentStringElementLayout() (size, alignment uint) {
	word := uint(strconv.IntSize / 8)
	return 3 * word, word
}

func (value *Comment) check(context rustfmt.Context) {
	if value == nil || value.consumed {
		context.Panic("rustcast: use of consumed Comment")
	}
}
func (value *Comment) Clone() Comment { return value.CloneWithContext(rustfmt.Context{}) }
func (value *Comment) CloneWithContext(context rustfmt.Context) Comment {
	value.check(context)
	lines := make([]ruststr.StringCell, len(value.Lines))
	for index := range value.Lines {
		lines[index] = *value.Lines[index].CloneWithContext(context).TakeCellWithContext(context)
	}
	return Comment{Style: value.Style.Clone(), Lines: lines, Pos: value.Pos}
}
func (value *Comment) CloneFrom(source *Comment) {
	value.CloneFromWithContext(source, rustfmt.Context{})
}
func (value *Comment) CloneFromWithContext(source *Comment, context rustfmt.Context) {
	value.check(context)
	// This derives Clone's default clone_from, not Vec/String clone_from's
	// capacity-reusing specialization: prepare the whole clone before Drop.
	next := source.CloneWithContext(context)
	value.CloseWithContext(context)
	*value = next
}
func (value *Comment) MoveWithContext(context rustfmt.Context) Comment {
	value.check(context)
	next := Comment{Style: value.Style, Lines: value.Lines, Pos: value.Pos}
	value.Lines = nil
	value.consumed = true
	return next
}
func (value *Comment) Close() { value.CloseWithContext(rustfmt.Context{}) }
func (value *Comment) CloseWithContext(context rustfmt.Context) {
	value.check(context)
	for index := range value.Lines {
		value.Lines[index].CloseWithContext(context)
	}
	value.Lines = nil
	value.consumed = true
}

// CommentText is a copying Go text convenience. Comment is the source primary:
// this convenience does not represent source String capacities or mutation.
type CommentText struct {
	Style CommentStyle
	Lines []string
	Pos   rustcspan.BytePos
}

func (value *CommentText) Clone() CommentText {
	lines := make([]string, len(value.Lines))
	for index, line := range value.Lines {
		lines[index] = strings.Clone(line)
	}
	return CommentText{Style: value.Style.Clone(), Lines: lines, Pos: value.Pos}
}
func (value *CommentText) CloneFrom(source *CommentText) { *value = source.Clone() }
func (value *Comment) CopyTextWithContext(context rustfmt.Context) CommentText {
	value.check(context)
	lines := make([]string, len(value.Lines))
	for index := range value.Lines {
		lines[index] = strings.Clone(value.Lines[index].AsStrWithContext(context))
	}
	return CommentText{Style: value.Style, Lines: lines, Pos: value.Pos}
}

// MayHaveDocLinks is the conservative source test, not a bracket parser.
func MayHaveDocLinks(text string) bool { return strings.ContainsRune(text, '[') }

func allCommentStars(text string) bool {
	for _, char := range text {
		if char != '*' {
			return false
		}
	}
	return true
}
func commentVerticalTrim(lines []string) (int, int, bool) {
	i, j := 0, len(lines)
	if len(lines) != 0 && allCommentStars(lines[0]) {
		i++
	}
	if j > i && lines[j-1] != "" && allCommentStars(lines[j-1]) {
		j--
	}
	return i, j, i != 0 || j != len(lines)
}
func commentWhitespace(char rune) bool {
	return char == ' ' || '\t' <= char && char <= '\r' || char > 0x7f && unicodecore.WhiteSpaceLookup(char)
}
func commentTrimStart(text string) string {
	for len(text) != 0 {
		char, width := utf8.DecodeRuneInString(text)
		if !commentWhitespace(char) {
			break
		}
		text = text[width:]
	}
	return text
}
func commentWhitespaceOnly(text string) bool {
	for _, char := range text {
		if !commentWhitespace(char) {
			return false
		}
	}
	return true
}
func commentHorizontalTrim(lines []string, kind rustcast.CommentKind) (string, bool) {
	i, first := ^uint(0), true
	if kind.Equal(rustcast.CommentKindBlock()) {
		start, end := 0, len(lines)
		if len(lines) != 0 && !strings.HasPrefix(commentTrimStart(lines[0]), "*") {
			start = 1
		}
		for start < end && commentWhitespaceOnly(lines[start]) {
			start++
		}
		for end > start && commentWhitespaceOnly(lines[end-1]) {
			end--
		}
		lines = lines[start:end]
	}
	for _, line := range lines {
		j := uint(0)
		for _, char := range line {
			if j > i || char != '*' && char != ' ' && char != '\t' {
				return "", false
			}
			if char == '*' {
				if first {
					i, first = j, false
				} else if i != j {
					return "", false
				}
				break
			}
			j++
		}
		if i >= uint(len(line)) {
			return "", false
		}
	}
	if len(lines) == 0 {
		return "", false
	}
	// All characters before i were proven ASCII spaces/tabs, so scalar and
	// byte offsets agree at this original Rust string slice boundary.
	return lines[0][:i], true
}

// commentLines is str::lines: split LF, strip CR only immediately before LF,
// and omit the final terminator's empty segment. Lone CR remains in a line.
func commentLines(text string) []string {
	lines := []string{}
	for {
		end := strings.IndexByte(text, '\n')
		if end < 0 {
			if text != "" {
				lines = append(lines, text)
			}
			return lines
		}
		line := text[:end]
		if strings.HasSuffix(line, "\r") {
			line = line[:len(line)-1]
		}
		lines = append(lines, line)
		text = text[end+1:]
	}
}
func BeautifyDocString(session *rustcspan.SessionGlobals, data rustcspan.Symbol, kind rustcast.CommentKind) rustcspan.Symbol {
	return BeautifyDocStringWithContext(session, data, kind, rustfmt.Context{})
}
func BeautifyDocStringWithContext(session *rustcspan.SessionGlobals, data rustcspan.Symbol, kind rustcast.CommentKind, context rustfmt.Context) rustcspan.Symbol {
	text := data.AsStrWithContext(session, context)
	if strings.ContainsRune(text, '\n') {
		lines := commentLines(text)
		i, j, changes := commentVerticalTrim(lines)
		if changes {
			lines = lines[i:j]
		}
		if horizontal, found := commentHorizontalTrim(lines, kind); found {
			changes = true
			for index, line := range lines {
				if strings.HasPrefix(line, horizontal) {
					line = line[len(horizontal):]
					if kind.Equal(rustcast.CommentKindBlock()) && (line == "*" || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "**")) {
						line = line[1:]
					}
					lines[index] = line
				}
			}
		}
		if changes {
			return rustcspan.InternSymbolWithContext(session, strings.Join(lines, "\n"), context)
		}
	}
	return data
}
