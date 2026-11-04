package compiler

import (
	"fmt"
	"unicode/utf8"

	parseformat "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_parse_format/src"
)

// formatPlan is the checked print/println program. Argument indices address the
// value arguments, excluding the format literal. The generator consumes this
// plan without interpreting the format string again.
type formatPlan struct {
	pieces    []formatPiece
	arguments int
	newline   bool
}

type formatPiece struct {
	literal    string
	argument   int
	isArgument bool
}

// planFormat uses the complete upstream format parser for syntax, then enforces
// the compiler's deliberately smaller formatting profile: bare sequential {}
// arguments and escaped braces. Spans here refer to decoded string bytes; the
// caller reports errors at the original literal's source-start span.
func planFormat(format string) (formatPlan, error) {
	if !utf8.ValidString(format) {
		return formatPlan{}, fmt.Errorf("invalid format string: input is not valid UTF-8")
	}
	p := parseformat.NewParser(format, parseformat.Option[uint]{}, parseformat.Option[string]{}, false, parseformat.Format)
	var plan formatPlan
	unsupported := false
	for current := p.Peek(); current.Present; current = p.Peek() {
		start := current.Value.Index
		piece, ok := p.Next()
		if !ok {
			// The source iterator can resume after an unmatched closing brace.
			// Continue so all syntax is validated before applying our profile.
			continue
		}
		if piece.Kind == parseformat.Lit {
			plan.pieces = append(plan.pieces, formatPiece{literal: piece.Text})
			continue
		}
		end := uint(len(format))
		if next := p.Peek(); next.Present {
			end = next.Value.Index
		}
		argument := piece.Argument
		// The byte cursors come from the parser; comparing the parsed argument
		// lexeme also excludes valid but unsupported {:} and whitespace forms.
		if format[start:end] != "{}" || argument.Position.Kind != parseformat.ArgumentImplicitlyIs || argument.Position.Index != uint(plan.arguments) || !argument.Format.Equal(parseformat.FormatSpec{}) {
			unsupported = true
		}
		plan.pieces = append(plan.pieces, formatPiece{argument: plan.arguments, isArgument: true})
		plan.arguments++
	}
	if len(p.Errors) != 0 {
		if p.Errors[0].Description == "unmatched `}` found" {
			return formatPlan{}, fmt.Errorf("invalid format string: unmatched } in format string")
		}
		return formatPlan{}, fmt.Errorf("invalid format string: %s", p.Errors[0].Description)
	}
	if unsupported {
		return formatPlan{}, fmt.Errorf("unsupported format string: only {} format placeholders and escaped braces are supported")
	}
	return plan, nil
}
