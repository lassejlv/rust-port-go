// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: semver 1.0.28 src/error.rs. The private error variants remain private;
// Display, manual Debug, and the std-only error source adapter retain behavior.
package semver

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

type errorTag uint8

const (
	errorEmpty errorTag = iota
	errorUnexpectedEnd
	errorUnexpected
	errorUnexpectedAfter
	errorExpectedComma
	errorLeadingZero
	errorOverflow
	errorEmptySegment
	errorIllegalCharacter
	errorWildcardNotOnly
	errorUnexpectedAfterWildcard
	errorExcessiveComparators
)

type parsePosition uint8

const (
	positionMajor parsePosition = iota
	positionMinor
	positionPatch
	positionPre
	positionBuild
)

type errorKind struct {
	tag errorTag
	pos parsePosition
	ch  rune
}

func (pos parsePosition) String() string {
	switch pos {
	case positionMajor:
		return "major version number"
	case positionMinor:
		return "minor version number"
	case positionPatch:
		return "patch version number"
	case positionPre:
		return "pre-release identifier"
	case positionBuild:
		return "build metadata"
	default:
		panic("semver: invalid private parser position")
	}
}
func (err *Error) Error() string {
	var out strings.Builder
	_ = err.WriteDisplay(&out, FormatOptions{})
	return out.String()
}
func (err *Error) WriteDisplay(writer io.Writer, _ FormatOptions) error {
	kind := err.kind
	write := func(texts ...string) error {
		for _, text := range texts {
			if failure := writeDisplay(writer, text); failure != nil {
				return failure
			}
		}
		return nil
	}
	switch kind.tag {
	case errorEmpty:
		return write("empty string, expected a semver version")
	case errorUnexpectedEnd:
		return write("unexpected end of input while parsing ", kind.pos.String())
	case errorUnexpected:
		if failure := write("unexpected character "); failure != nil {
			return failure
		}
		if failure := writeQuotedChar(writer, kind.ch); failure != nil {
			return failure
		}
		return write(" while parsing ", kind.pos.String())
	case errorUnexpectedAfter:
		if failure := write("unexpected character "); failure != nil {
			return failure
		}
		if failure := writeQuotedChar(writer, kind.ch); failure != nil {
			return failure
		}
		return write(" after ", kind.pos.String())
	case errorExpectedComma:
		if failure := write("expected comma after ", kind.pos.String(), ", found "); failure != nil {
			return failure
		}
		return writeQuotedChar(writer, kind.ch)
	case errorLeadingZero:
		return write("invalid leading zero in ", kind.pos.String())
	case errorOverflow:
		return write("value of ", kind.pos.String(), " exceeds u64::MAX")
	case errorEmptySegment:
		return write("empty identifier segment in ", kind.pos.String())
	case errorIllegalCharacter:
		return write("unexpected character in ", kind.pos.String())
	case errorWildcardNotOnly:
		if failure := write("wildcard req ("); failure != nil {
			return failure
		}
		if failure := writeChar(writer, kind.ch); failure != nil {
			return failure
		}
		return write(") must be the only comparator in the version req")
	case errorUnexpectedAfterWildcard:
		return write("unexpected character after wildcard in version req")
	case errorExcessiveComparators:
		return write("excessive number of version comparators")
	default:
		panic("semver: invalid private error kind")
	}
}
func writeQuotedChar(writer io.Writer, ch rune) error {
	if ch == 0 {
		return writeDisplay(writer, `'\0'`)
	}
	if failure := writeChar(writer, '\''); failure != nil {
		return failure
	}
	text := quotedChar(ch)
	payload := text[1 : len(text)-1]
	var failure error
	if strings.HasPrefix(payload, "\\") {
		failure = writeDisplay(writer, payload)
	} else {
		failure = writeChar(writer, ch)
	}
	if failure != nil {
		return failure
	}
	return writeChar(writer, '\'')
}

func quotedChar(ch rune) string {
	var escaped string
	switch ch {
	case 0:
		escaped = `\0`
	case '\t':
		escaped = `\t`
	case '\n':
		escaped = `\n`
	case '\r':
		escaped = `\r`
	case '\\':
		escaped = `\\`
	case '\'':
		escaped = `\'`
	default:
		index := sort.Search(len(debugEscapedRanges), func(index int) bool { return debugEscapedRanges[index][1] >= ch })
		if index < len(debugEscapedRanges) && debugEscapedRanges[index][0] <= ch {
			escaped = fmt.Sprintf(`\u{%x}`, ch)
		} else {
			escaped = string(ch)
		}
	}
	return "'" + escaped + "'"
}
