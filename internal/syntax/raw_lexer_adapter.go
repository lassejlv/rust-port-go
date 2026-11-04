package syntax

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	literalescaper "github.com/lassejlv/rust-port-go/port/dependencies/rustc-literal-escaper/v0_0_8/src"
	rustclexer "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_lexer/src"
)

// lexRawSource adapts the source-ported Rust lexer to the bootstrap parser's
// tokens. The raw lexer determines source boundaries; this layer handles the
// supported compound punctuation, literal values and explicit frontend limits.
// The caller must check UTF-8 so invalid bytes receive source diagnostics.
func lexRawSource(src string) ([]token, error) {
	l := &lexer{src: src, line: 1, column: 1}
	if _, present := rustclexer.StripShebang(src); present {
		return nil, sourceError(l.pos(), "shebangs are not supported")
	}
	cursor := rustclexer.NewCursor(src, rustclexer.FrontmatterYes)
	var out []token
	for {
		raw := cursor.AdvanceToken()
		start := l.pos()
		begin := l.offset
		end := begin + int(raw.Len)
		spelling := src[begin:end]
		switch raw.Kind.Tag {
		case rustclexer.Eof:
			return append(out, token{kind: "eof", span: start}), nil
		case rustclexer.Whitespace, rustclexer.LineComment:
			// Doc comments retain the existing bootstrap treatment as comments.
			// Attribute expansion and documentation semantics are not implemented.
		case rustclexer.BlockComment:
			if !raw.Kind.Terminated {
				return nil, sourceError(start, "unterminated block comment")
			}
		case rustclexer.Frontmatter:
			return nil, sourceError(start, "frontmatter is not supported")
		case rustclexer.Ident:
			// Field names currently enter generated C identifiers verbatim, so the
			// bootstrap frontend retains its ASCII identifier boundary.
			identifierPosition := *l
			for _, r := range spelling {
				if r > 0x7f {
					return nil, sourceError(identifierPosition.pos(), "unsupported character %q: Unicode identifiers are not supported", r)
				}
				identifierPosition.advance()
			}
			out = append(out, token{kind: "ident", text: spelling, span: start})
		case rustclexer.InvalidIdent:
			return nil, sourceError(start, "invalid identifier containing emoji")
		case rustclexer.RawIdent:
			return nil, sourceError(start, "raw identifiers are not supported")
		case rustclexer.ForcedKeywordIdent:
			return nil, sourceError(start, "forced keyword identifiers are not supported")
		case rustclexer.UnknownPrefix:
			return nil, sourceError(start, "unknown literal prefix %q", spelling)
		case rustclexer.UnknownPrefixLifetime, rustclexer.RawLifetime, rustclexer.Lifetime:
			return nil, sourceError(start, "unsupported character %q: lifetimes are not supported", '\'')
		case rustclexer.GuardedStrPrefix:
			return nil, sourceError(start, "guarded strings are not supported")
		case rustclexer.Literal:
			switch raw.Kind.Literal.Tag {
			case rustclexer.Int:
				if err := validateInteger(spelling); err != nil {
					return nil, sourceError(start, "%v", err)
				}
				out = append(out, token{kind: "int", text: spelling, span: start})
			case rustclexer.Str:
				// Limit decoding to the raw token's body. Malformed escapes must
				// never consume a following token while searching for a delimiter.
				literalEnd := begin + int(raw.Kind.SuffixStart)
				value, err := decodeNormalString(*l, literalEnd, raw.Kind.Literal.Terminated)
				if err != nil {
					return nil, err
				}
				if raw.Kind.SuffixStart != raw.Len {
					suffixPosition := literalPositionAt(*l, literalEnd)
					return nil, sourceError(suffixPosition.pos(), "string literal suffixes are not supported")
				}
				out = append(out, token{kind: "string", text: value, span: start})
			case rustclexer.Float:
				return nil, sourceError(start, "floating-point literals are not supported")
			case rustclexer.Char:
				return nil, sourceError(start, "character literals are not supported")
			case rustclexer.Byte:
				return nil, sourceError(start, "byte literals are not supported")
			case rustclexer.ByteStr:
				return nil, sourceError(start, "byte strings are not supported")
			case rustclexer.CStr:
				return nil, sourceError(start, "C strings are not supported")
			case rustclexer.RawStr, rustclexer.RawByteStr, rustclexer.RawCStr:
				return nil, sourceError(start, "raw strings are not supported")
			}
		default:
			punctuation := spelling
			if raw.Kind.Tag == rustclexer.Unknown || !strings.Contains("(){}[]#.;,:+-*/%=<>!&|^", punctuation) {
				r, _ := utf8.DecodeRuneInString(spelling)
				return nil, sourceError(start, "unsupported character %q", r)
			}
			if compound := rawCompoundPunctuation(src[begin:]); compound != "" {
				// Only byte-adjacent punctuation is joined. Raw comments and
				// whitespace remain token boundaries even though they are skipped.
				for consumed := int(raw.Len); consumed < len(compound); {
					part := cursor.AdvanceToken()
					consumed += int(part.Len)
				}
				punctuation = compound
				end = begin + len(compound)
			}
			out = append(out, token{kind: punctuation, text: punctuation, span: start})
		}
		for l.offset < end {
			l.advance()
		}
	}
}

func rawCompoundPunctuation(rest string) string {
	for _, op := range [...]string{"<<=", ">>=", "..=", "..", "::", "->", "==", "!=", "<=", ">=", "&&", "||", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<", ">>"} {
		if strings.HasPrefix(rest, op) {
			return op
		}
	}
	return ""
}

// normalizedLiteral keeps byte offsets into the original body. For every CRLF
// pair its normalized LF byte marks one removed byte; boundaries after that LF
// shift by one. Unpaired CR bytes remain for the escaper to diagnose or to skip
// as ASCII continuation whitespace, following Rust's behavior.
type normalizedLiteral struct {
	text string
	crlf []int
}

func normalizeLiteralCRLF(body string) normalizedLiteral {
	if !strings.Contains(body, "\r\n") {
		return normalizedLiteral{text: body}
	}
	var text strings.Builder
	text.Grow(len(body))
	var crlf []int
	for offset := 0; offset < len(body); offset++ {
		if body[offset] == '\r' && offset+1 < len(body) && body[offset+1] == '\n' {
			crlf = append(crlf, text.Len())
			text.WriteByte('\n')
			offset++
		} else {
			text.WriteByte(body[offset])
		}
	}
	return normalizedLiteral{text: text.String(), crlf: crlf}
}

func (literal normalizedLiteral) sourceOffset(offset int) int {
	return offset + sort.SearchInts(literal.crlf, offset)
}

// Literal errors retain the complete original byte/scalar range internally.
// The bootstrap diagnostic still prints its existing start line and column.
type literalEscapeError struct {
	kind       literalescaper.EscapeError
	start, end Span
	message    string
}

func (failure *literalEscapeError) Error() string {
	return sourceError(failure.start, "%s", failure.message).Error()
}

func literalPositionAt(position lexer, offset int) lexer {
	for position.offset < offset {
		position.advance()
	}
	return position
}

func decodeNormalString(position lexer, literalEnd int, terminated bool) (string, error) {
	start := position.pos()
	bodyStart, bodyEnd := position.offset+1, literalEnd
	if terminated {
		bodyEnd-- // The raw lexer has already identified the closing quote.
	}
	body := normalizeLiteralCRLF(position.src[bodyStart:bodyEnd])
	var value strings.Builder
	var failure *literalEscapeError
	literalescaper.UnescapeStr(body.text, func(span literalescaper.Range, result literalescaper.Result[rune]) {
		if failure != nil {
			return
		}
		if result.IsOK() {
			value.WriteRune(result.Value)
			return
		}
		if !result.Error.IsFatal() {
			// The bootstrap compiler does not yet expose a warning channel.
			return
		}
		errorStart := literalPositionAt(position, bodyStart+body.sourceOffset(span.Start))
		errorEnd := literalPositionAt(errorStart, bodyStart+body.sourceOffset(span.End))
		failure = &literalEscapeError{
			kind:  result.Error,
			start: errorStart.pos(), end: errorEnd.pos(),
			message: literalEscapeMessage(result.Error, body.text[span.Start:span.End]),
		}
	})
	if failure != nil {
		if !terminated && failure.kind == literalescaper.LoneSlash {
			return "", sourceError(start, "unterminated string literal")
		}
		return "", failure
	}
	if !terminated {
		return "", sourceError(start, "unterminated string literal")
	}
	return value.String(), nil
}

// Keep the frontend's diagnostic vocabulary while the complete source port
// decides escape validity, consumption, continuation handling and scalar values.
func literalEscapeMessage(kind literalescaper.EscapeError, spelling string) string {
	switch kind {
	case literalescaper.LoneSlash:
		return "unterminated string literal"
	case literalescaper.InvalidEscape:
		escaped, _ := utf8.DecodeRuneInString(strings.TrimPrefix(spelling, "\\"))
		if escaped == '\r' {
			return "bare CR is not a valid string escape"
		}
		return fmt.Sprintf("unsupported string escape \\%c", escaped)
	case literalescaper.BareCarriageReturn:
		return "bare CR is not allowed in a string; use \\r"
	case literalescaper.TooShortHexEscape:
		return "expected two hexadecimal digits after \\x"
	case literalescaper.InvalidCharInHexEscape, literalescaper.OutOfRangeHexEscape:
		return "\\x escape must be an ASCII byte"
	case literalescaper.NoBraceInUnicodeEscape:
		return "expected { after \\u"
	case literalescaper.UnclosedUnicodeEscape:
		return "unterminated Unicode escape"
	case literalescaper.LeadingUnderscoreUnicodeEscape:
		return "invalid Unicode escape: first character must be a hexadecimal digit"
	case literalescaper.InvalidCharInUnicodeEscape, literalescaper.EmptyUnicodeEscape,
		literalescaper.OverlongUnicodeEscape, literalescaper.LoneSurrogateUnicodeEscape,
		literalescaper.OutOfRangeUnicodeEscape:
		return "invalid Unicode escape"
	case literalescaper.EscapeOnlyChar:
		return "unescaped quote in string literal"
	default:
		return "invalid string escape"
	}
}
