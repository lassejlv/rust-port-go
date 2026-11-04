package syntax

import (
	"strings"
	"unicode/utf8"

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
				decoder := *l
				decoder.src = src[:begin+int(raw.Kind.SuffixStart)]
				value, err := decoder.stringLiteral()
				if err != nil {
					return nil, err
				}
				if raw.Kind.SuffixStart != raw.Len {
					return nil, sourceError(decoder.pos(), "string literal suffixes are not supported")
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
