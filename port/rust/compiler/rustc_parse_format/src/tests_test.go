// SPDX-License-Identifier: MIT OR Apache-2.0
// Full test-file port of rust-lang/rust compiler/rustc_parse_format/src/tests.rs
// at 2d20b42b883dcfc014f3c2c56081eeb6091c2179, Git blob
// a2e03a514fa5fcc8304933ef59e02aa4400c4181 (607 lines, 16,799 bytes).
// Source SHA-256: 8ad525beb126768ecdb887c4b4a2774db4e419aa4c4f7677b8a545c7405fc531.
// All 28 tests, 43 parser inputs and 90 test-body string literals are retained.
// See upstream_test_literals.json for the decoded source-literal audit.
package parseformat

import (
	"encoding/hex"
	"encoding/json"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Rust's Piece enum constructors and Box are represented by tagged Go values.
func upstreamLit(text string) Piece        { return Piece{Kind: Lit, Text: text} }
func upstreamNext(argument Argument) Piece { return Piece{Kind: NextArgument, Argument: &argument} }
func upstreamCollect(parser *Parser) []Piece {
	var pieces []Piece
	for {
		piece, ok := parser.Next()
		if !ok {
			return pieces
		}
		pieces = append(pieces, piece)
	}
}
func upstreamErrorsEmpty(errors []ParseError) bool { return len(errors) == 0 }
func upstreamAssert(t *testing.T, condition bool) {
	t.Helper()
	if !condition {
		t.Fatal("upstream assertion failed")
	}
}
func upstreamEqual(t *testing.T, actual, expected any) {
	t.Helper()
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("upstream assertion:\nactual %#v\nexpected %#v", actual, expected)
	}
}
func upstreamSliceEqual[T any](t *testing.T, actual, expected []T) {
	t.Helper()
	// Rust Vec/slice equality treats two empty sequences alike, independently
	// of Go's distinction between nil and an allocated empty slice.
	if len(actual) != len(expected) {
		t.Fatalf("upstream slice lengths: actual %d, expected %d", len(actual), len(expected))
	}
	for index := range actual {
		if !reflect.DeepEqual(actual[index], expected[index]) {
			t.Fatalf("upstream slice element %d:\nactual %#v\nexpected %#v", index, actual[index], expected[index])
		}
	}
}

// Upstream: same (lines 5-9). Collect stops on the first None, as Rust does.
func upstreamSame(t *testing.T, format string, expected []Piece) {
	t.Helper()
	parser := NewParser(format, Option[uint]{}, Option[string]{}, false, Format)
	upstreamSliceEqual(t, upstreamCollect(parser), expected)
}

// Upstream: fmtdflt (lines 11-27); every original default field is explicit.
func fmtdflt() FormatSpec {
	return FormatSpec{
		Fill: Option[rune]{}, FillSpan: Option[Range]{}, Align: AlignUnknown,
		Sign: Option[Sign]{}, Alternate: false, ZeroPad: false, DebugHex: Option[DebugHex]{},
		Precision: Count{Kind: CountImplied}, Width: Count{Kind: CountImplied},
		PrecisionSpan: Option[Range]{}, WidthSpan: Option[Range]{}, Ty: "", TySpan: Option[Range]{},
	}
}

// Upstream: musterr (lines 29-33). It advances once, not through the full input.
func upstreamMusterr(t *testing.T, input string) {
	t.Helper()
	parser := NewParser(input, Option[uint]{}, Option[string]{}, false, Format)
	parser.Next()
	upstreamAssert(t, !upstreamErrorsEmpty(parser.Errors))
}

// These diagnostic tests use FormatSpec { ty, ty_span, ..Default::default() }.
// The Rust Default has the same fields and values as the explicit fmtdflt above.
func upstreamFormatType(ty string, span Option[Range]) FormatSpec {
	value := fmtdflt()
	value.Ty = ty
	value.TySpan = span
	return value
}
func upstreamOnlyArgument(t *testing.T, pieces []Piece) *Argument {
	t.Helper()
	if len(pieces) != 1 || pieces[0].Kind != NextArgument || pieces[0].Argument == nil {
		t.Fatalf("expected exactly one NextArgument, got %#v", pieces)
	}
	return pieces[0].Argument
}

// Upstream: simple (lines 35-43).
func TestUpstreamSimple(t *testing.T) {
	upstreamSame(t, "asdf", []Piece{
		upstreamLit("asdf"),
	})
	upstreamSame(t, "a{{b", []Piece{
		upstreamLit("a"),
		upstreamLit("{b"),
	})
	upstreamSame(t, "a}}b", []Piece{
		upstreamLit("a"),
		upstreamLit("}b"),
	})
	upstreamSame(t, "a}}", []Piece{
		upstreamLit("a"),
		upstreamLit("}"),
	})
	upstreamSame(t, "}}", []Piece{
		upstreamLit("}"),
	})
	upstreamSame(t, "\\}}", []Piece{
		upstreamLit("\\"),
		upstreamLit("}"),
	})
}

// Upstream: invalid01 (lines 44-47).
func TestUpstreamInvalid01(t *testing.T) {
	upstreamMusterr(t, "{")
}

// Upstream: invalid02 (lines 48-51).
func TestUpstreamInvalid02(t *testing.T) {
	upstreamMusterr(t, "}")
}

// Upstream: invalid04 (lines 52-55).
func TestUpstreamInvalid04(t *testing.T) {
	upstreamMusterr(t, "{3a}")
}

// Upstream: invalid05 (lines 56-59).
func TestUpstreamInvalid05(t *testing.T) {
	upstreamMusterr(t, "{:|}")
}

// Upstream: invalid06 (lines 60-63).
func TestUpstreamInvalid06(t *testing.T) {
	upstreamMusterr(t, "{:>>>}")
}

// Upstream: invalid_position (lines 65-68).
func TestUpstreamInvalidPosition(t *testing.T) {
	upstreamMusterr(t, "{18446744073709551616}")
}

// Upstream: invalid_width (lines 70-73).
func TestUpstreamInvalidWidth(t *testing.T) {
	upstreamMusterr(t, "{:18446744073709551616}")
}

// Upstream: invalid_precision (lines 75-78).
func TestUpstreamInvalidPrecision(t *testing.T) {
	upstreamMusterr(t, "{:.18446744073709551616}")
}

// Upstream: format_empty (lines 80-90).
func TestUpstreamFormatEmpty(t *testing.T) {
	upstreamSame(t, "{}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 2, End: 2},
			Format:       fmtdflt(),
		}),
	})
}

// Upstream: format_tab_empty (lines 91-107).
func TestUpstreamFormatTabEmpty(t *testing.T) {
	fmtPre := "\"\\t{}\""
	format := "\t{}"
	parser := NewParser(format, Option[uint]{}, Some(fmtPre), false, Format)
	upstreamSliceEqual(t, upstreamCollect(parser), []Piece{
		upstreamLit("\t"),
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 4, End: 4},
			Format:       fmtdflt(),
		}),
	})
}

// Upstream: format_open_brace_tab (lines 108-115).
func TestUpstreamFormatOpenBraceTab(t *testing.T) {
	fmtPre := "\"{\\t\""
	format := "{\t"
	parser := NewParser(format, Option[uint]{}, Some(fmtPre), false, Format)
	_ = upstreamCollect(parser)
	upstreamEqual(t, parser.Errors[0].Span, Range{Start: 4, End: 4})
}

// Upstream: format_position (lines 116-126).
func TestUpstreamFormatPosition(t *testing.T) {
	upstreamSame(t, "{3}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentIs, Index: 3},
			PositionSpan: Range{Start: 2, End: 3},
			Format:       fmtdflt(),
		}),
	})
}

// Upstream: format_position_nothing_else (lines 127-137).
func TestUpstreamFormatPositionNothingElse(t *testing.T) {
	upstreamSame(t, "{3:}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentIs, Index: 3},
			PositionSpan: Range{Start: 2, End: 3},
			Format:       fmtdflt(),
		}),
	})
}

// Upstream: format_named (lines 138-148).
func TestUpstreamFormatNamed(t *testing.T) {
	upstreamSame(t, "{name}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentNamed, Name: "name"},
			PositionSpan: Range{Start: 2, End: 6},
			Format:       fmtdflt(),
		}),
	})
}

// Upstream: format_named_space_nothing (lines 149-167).
func TestUpstreamFormatNamedSpaceNothing(t *testing.T) {
	upstreamSame(t, "{name} {}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentNamed, Name: "name"},
			PositionSpan: Range{Start: 2, End: 6},
			Format:       fmtdflt(),
		}),
		upstreamLit(" "),
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 9, End: 9},
			Format:       fmtdflt(),
		}),
	})
}

// Upstream: format_raw (lines 168-184).
func TestUpstreamFormatRaw(t *testing.T) {
	snippet := "r#\"assertion `left {op} right` failed\"#"
	source := "assertion `left {op} right` failed"
	parser := NewParser(source, Some(uint(1)), Some(snippet), true, Format)
	expected := []Piece{
		upstreamLit("assertion `left "),
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentNamed, Name: "op"},
			PositionSpan: Range{Start: 20, End: 22},
			Format:       fmtdflt(),
		}),
		upstreamLit(" right` failed"),
	}
	upstreamSliceEqual(t, upstreamCollect(parser), expected)
}

// Upstream: format_type (lines 185-209).
func TestUpstreamFormatType(t *testing.T) {
	upstreamSame(t, "{3:x}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentIs, Index: 3},
			PositionSpan: Range{Start: 2, End: 3},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountImplied},
				Width:         Count{Kind: CountImplied},
				PrecisionSpan: Option[Range]{},
				WidthSpan:     Option[Range]{},
				Ty:            "x",
				TySpan:        Option[Range]{},
			},
		}),
	})
}

// Upstream: format_align_fill (lines 210-278).
func TestUpstreamFormatAlignFill(t *testing.T) {
	upstreamSame(t, "{3:>}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentIs, Index: 3},
			PositionSpan: Range{Start: 2, End: 3},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignRight,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountImplied},
				Width:         Count{Kind: CountImplied},
				PrecisionSpan: Option[Range]{},
				WidthSpan:     Option[Range]{},
				Ty:            "",
				TySpan:        Option[Range]{},
			},
		}),
	})
	upstreamSame(t, "{3:0<}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentIs, Index: 3},
			PositionSpan: Range{Start: 2, End: 3},
			Format: FormatSpec{
				Fill:          Some('0'),
				FillSpan:      Some(Range{Start: 4, End: 5}),
				Align:         AlignLeft,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountImplied},
				Width:         Count{Kind: CountImplied},
				PrecisionSpan: Option[Range]{},
				WidthSpan:     Option[Range]{},
				Ty:            "",
				TySpan:        Option[Range]{},
			},
		}),
	})
	upstreamSame(t, "{3:*<abcd}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentIs, Index: 3},
			PositionSpan: Range{Start: 2, End: 3},
			Format: FormatSpec{
				Fill:          Some('*'),
				FillSpan:      Some(Range{Start: 4, End: 5}),
				Align:         AlignLeft,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountImplied},
				Width:         Count{Kind: CountImplied},
				PrecisionSpan: Option[Range]{},
				WidthSpan:     Option[Range]{},
				Ty:            "abcd",
				TySpan:        Some(Range{Start: 6, End: 10}),
			},
		}),
	})
}

// Upstream: format_counts (lines 279-435).
func TestUpstreamFormatCounts(t *testing.T) {
	upstreamSame(t, "{:10x}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 2, End: 2},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountImplied},
				PrecisionSpan: Option[Range]{},
				Width:         Count{Kind: CountIs, Value: 10},
				WidthSpan:     Some(Range{Start: 3, End: 5}),
				Ty:            "x",
				TySpan:        Option[Range]{},
			},
		}),
	})
	upstreamSame(t, "{:10$.10x}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 2, End: 2},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountIs, Value: 10},
				PrecisionSpan: Some(Range{Start: 6, End: 9}),
				Width:         Count{Kind: CountIsParam, Index: 10},
				WidthSpan:     Some(Range{Start: 3, End: 6}),
				Ty:            "x",
				TySpan:        Option[Range]{},
			},
		}),
	})
	upstreamSame(t, "{1:0$.10x}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentIs, Index: 1},
			PositionSpan: Range{Start: 2, End: 3},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountIs, Value: 10},
				PrecisionSpan: Some(Range{Start: 6, End: 9}),
				Width:         Count{Kind: CountIsParam, Index: 0},
				WidthSpan:     Some(Range{Start: 4, End: 6}),
				Ty:            "x",
				TySpan:        Option[Range]{},
			},
		}),
	})
	upstreamSame(t, "{:.*x}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 1},
			PositionSpan: Range{Start: 2, End: 2},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountIsStar, Index: 0},
				PrecisionSpan: Some(Range{Start: 3, End: 5}),
				Width:         Count{Kind: CountImplied},
				WidthSpan:     Option[Range]{},
				Ty:            "x",
				TySpan:        Option[Range]{},
			},
		}),
	})
	upstreamSame(t, "{:.10$x}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 2, End: 2},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountIsParam, Index: 10},
				Width:         Count{Kind: CountImplied},
				PrecisionSpan: Some(Range{Start: 3, End: 7}),
				WidthSpan:     Option[Range]{},
				Ty:            "x",
				TySpan:        Option[Range]{},
			},
		}),
	})
	upstreamSame(t, "{:a$.b$?}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 2, End: 2},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountIsName, Name: "b", Span: Range{Start: 6, End: 7}},
				PrecisionSpan: Some(Range{Start: 5, End: 8}),
				Width:         Count{Kind: CountIsName, Name: "a", Span: Range{Start: 3, End: 4}},
				WidthSpan:     Some(Range{Start: 3, End: 5}),
				Ty:            "?",
				TySpan:        Option[Range]{},
			},
		}),
	})
	upstreamSame(t, "{:.4}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 2, End: 2},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountIs, Value: 4},
				PrecisionSpan: Some(Range{Start: 3, End: 5}),
				Width:         Count{Kind: CountImplied},
				WidthSpan:     Option[Range]{},
				Ty:            "",
				TySpan:        Option[Range]{},
			},
		}),
	})
}

// Upstream: format_flags (lines 436-482).
func TestUpstreamFormatFlags(t *testing.T) {
	upstreamSame(t, "{:-}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 2, End: 2},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Some(Minus),
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountImplied},
				Width:         Count{Kind: CountImplied},
				PrecisionSpan: Option[Range]{},
				WidthSpan:     Option[Range]{},
				Ty:            "",
				TySpan:        Option[Range]{},
			},
		}),
	})
	upstreamSame(t, "{:+#}", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 2, End: 2},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Some(Plus),
				Alternate:     true,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountImplied},
				Width:         Count{Kind: CountImplied},
				PrecisionSpan: Option[Range]{},
				WidthSpan:     Option[Range]{},
				Ty:            "",
				TySpan:        Option[Range]{},
			},
		}),
	})
}

// Upstream: format_mixture (lines 483-511).
func TestUpstreamFormatMixture(t *testing.T) {
	upstreamSame(t, "abcd {3:x} efg", []Piece{
		upstreamLit("abcd "),
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentIs, Index: 3},
			PositionSpan: Range{Start: 7, End: 8},
			Format: FormatSpec{
				Fill:          Option[rune]{},
				FillSpan:      Option[Range]{},
				Align:         AlignUnknown,
				Sign:          Option[Sign]{},
				Alternate:     false,
				ZeroPad:       false,
				DebugHex:      Option[DebugHex]{},
				Precision:     Count{Kind: CountImplied},
				Width:         Count{Kind: CountImplied},
				PrecisionSpan: Option[Range]{},
				WidthSpan:     Option[Range]{},
				Ty:            "x",
				TySpan:        Option[Range]{},
			},
		}),
		upstreamLit(" efg"),
	})
}

// Upstream: format_whitespace (lines 512-530).
func TestUpstreamFormatWhitespace(t *testing.T) {
	upstreamSame(t, "{ }", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 2, End: 3},
			Format:       fmtdflt(),
		}),
	})
	upstreamSame(t, "{  }", []Piece{
		upstreamNext(Argument{
			Position:     Position{Kind: ArgumentImplicitlyIs, Index: 0},
			PositionSpan: Range{Start: 2, End: 4},
			Format:       fmtdflt(),
		}),
	})
}

// Upstream: asm_linespans (lines 531-546).
func TestUpstreamAsmLinespans(t *testing.T) {
	asmPre := "r\"\n        .intel_syntax noprefix\n        nop\""
	assembly := "\n        .intel_syntax noprefix\n        nop"
	parser := NewParser(assembly, Some(uint(0)), Some(asmPre), false, InlineAsm)
	upstreamAssert(t, parser.IsSourceLiteral)
	upstreamSliceEqual(t, upstreamCollect(parser), []Piece{
		upstreamLit("\n        .intel_syntax noprefix\n        nop"),
	})
	upstreamSliceEqual(t, parser.LineSpans, []Range{
		Range{Start: 2, End: 2},
		Range{Start: 11, End: 33},
		Range{Start: 42, End: 45},
	})
}

// Upstream: asm_concat (lines 547-555).
func TestUpstreamAsmConcat(t *testing.T) {
	asmPre := "concat!(\"invalid\", \"_\", \"instruction\")"
	assembly := "invalid_instruction"
	parser := NewParser(assembly, Option[uint]{}, Some(asmPre), false, InlineAsm)
	upstreamAssert(t, !parser.IsSourceLiteral)
	upstreamSliceEqual(t, upstreamCollect(parser), []Piece{upstreamLit(assembly)})
	upstreamSliceEqual(t, parser.LineSpans, []Range{})
}

// Upstream: diagnostic_format_flags (lines 557-576).
func TestUpstreamDiagnosticFormatFlags(t *testing.T) {
	lit := "{thing:blah}"
	parser := NewParser(lit, Option[uint]{}, Option[string]{}, false, Diagnostic)
	upstreamAssert(t, !parser.IsSourceLiteral)
	argument := upstreamOnlyArgument(t, upstreamCollect(parser))
	upstreamEqual(t, *argument, Argument{
		Position:     Position{Kind: ArgumentNamed, Name: "thing"},
		PositionSpan: Range{Start: 2, End: 7},
		Format:       upstreamFormatType("blah", Some(Range{Start: 7, End: 12})),
	})
	upstreamSliceEqual(t, parser.LineSpans, []Range{})
	upstreamAssert(t, upstreamErrorsEmpty(parser.Errors))
}

// Upstream: diagnostic_format_mod (lines 578-597).
func TestUpstreamDiagnosticFormatMod(t *testing.T) {
	lit := "{thing:+}"
	parser := NewParser(lit, Option[uint]{}, Option[string]{}, false, Diagnostic)
	upstreamAssert(t, !parser.IsSourceLiteral)
	argument := upstreamOnlyArgument(t, upstreamCollect(parser))
	upstreamEqual(t, *argument, Argument{
		Position:     Position{Kind: ArgumentNamed, Name: "thing"},
		PositionSpan: Range{Start: 2, End: 7},
		Format:       upstreamFormatType("+", Some(Range{Start: 7, End: 9})),
	})
	upstreamSliceEqual(t, parser.LineSpans, []Range{})
	upstreamAssert(t, upstreamErrorsEmpty(parser.Errors))
}

// Upstream: format_plus_sign_missing_colon_error (lines 598-607).
func TestUpstreamFormatPlusSignMissingColonError(t *testing.T) {
	parser := NewParser("{+}", Option[uint]{}, Option[string]{}, false, Format)
	_ = upstreamCollect(parser)
	upstreamAssert(t, !upstreamErrorsEmpty(parser.Errors))
	upstreamAssert(t, strings.Contains(parser.Errors[0].Description, "`+` sign flag must appear after `:`"))
	upstreamAssert(t, strings.Contains(parser.Errors[0].Label, "expected `:` before `+` sign flag"))
	upstreamEqual(t, parser.Errors[0].Span, Range{Start: 2, End: 3})
}

// This independent audit preserves each original test's declaration order and
// all decoded string/character/integer literals, including snippets and spans.
// It needs no upstream checkout or Rust toolchain during normal Go tests.
func TestPinnedUpstreamTestLiteralParity(t *testing.T) {
	var manifest struct {
		Tests []struct {
			RustName string `json:"rust_name"`
			GoName   string `json:"go_name"`
			Literals []struct {
				Kind  string `json:"kind"`
				Bytes string `json:"bytes"`
			} `json:"literals"`
		} `json:"tests"`
	}
	data, err := os.ReadFile("upstream_test_literals.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	file, err := goparser.ParseFile(token.NewFileSet(), "tests_test.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	functions := make(map[string]*ast.FuncDecl)
	var declarationOrder []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && strings.HasPrefix(function.Name.Name, "TestUpstream") {
			functions[function.Name.Name] = function
			declarationOrder = append(declarationOrder, function.Name.Name)
		}
	}
	var expectedOrder []string
	for _, source := range manifest.Tests {
		expectedOrder = append(expectedOrder, source.GoName)
		function := functions[source.GoName]
		if function == nil {
			t.Fatalf("missing upstream test %s", source.RustName)
		}
		type literal struct{ kind, bytes string }
		var actual []literal
		ast.Inspect(function.Body, func(node ast.Node) bool {
			value, ok := node.(*ast.BasicLit)
			if !ok {
				return true
			}
			item := literal{}
			switch value.Kind {
			case token.STRING, token.CHAR:
				decoded, err := strconv.Unquote(value.Value)
				if err != nil {
					t.Fatal(err)
				}
				item.kind = "string"
				if value.Kind == token.CHAR {
					item.kind = "char"
				}
				item.bytes = hex.EncodeToString([]byte(decoded))
			case token.INT:
				item.kind = "number"
				item.bytes = value.Value
			default:
				t.Fatalf("unexpected literal %s in %s", value.Value, source.GoName)
			}
			actual = append(actual, item)
			return true
		})
		var expected []literal
		for _, value := range source.Literals {
			expected = append(expected, literal{value.Kind, value.Bytes})
		}
		if !slices.Equal(actual, expected) {
			t.Fatalf("decoded literal sequence differs in upstream %s:\nGo %#v\nRust %#v", source.RustName, actual, expected)
		}
	}
	if !slices.Equal(declarationOrder, expectedOrder) {
		t.Fatalf("upstream test declaration order differs: %v", declarationOrder)
	}
}
