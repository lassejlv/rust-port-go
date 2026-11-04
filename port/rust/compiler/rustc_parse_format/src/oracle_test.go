package parseformat

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

type oracleAction struct {
	Op    string `json:"op"`
	Value any    `json:"value,omitempty"`
}
type oracleRequest struct {
	Input           string         `json:"input"`
	Mode            string         `json:"mode"`
	Style           *uint          `json:"style"`
	Snippet         *string        `json:"snippet"`
	AppendedNewline bool           `json:"appended_newline"`
	Checked         bool           `json:"checked"`
	Initial         map[string]any `json:"initial,omitempty"`
	Actions         []oracleAction `json:"actions"`
}

func oracleRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}
func oraclePtr[T any](value T) *T { return &value }
func oracleQuoted(value string) string {
	var result strings.Builder
	result.WriteByte('"')
	for _, c := range value {
		switch c {
		case '\x00':
			result.WriteString(`\0`)
		case '\n':
			result.WriteString(`\n`)
		case '\r':
			result.WriteString(`\r`)
		case '\t':
			result.WriteString(`\t`)
		case '\\':
			result.WriteString(`\\`)
		case '"':
			result.WriteString(`\"`)
		default:
			if c < 0x20 || c == 0x7f {
				fmt.Fprintf(&result, `\u{%x}`, c)
			} else {
				result.WriteRune(c)
			}
		}
	}
	result.WriteByte('"')
	return result.String()
}
func oracleActions(input string) []oracleAction {
	count := utf8.RuneCountInString(input) + 3
	if count > 14 {
		count = 14
	}
	actions := []oracleAction{{Op: "peek"}, {Op: "peek_ahead"}}
	for range count {
		actions = append(actions, oracleAction{Op: "next"})
	}
	return append(actions, oracleAction{Op: "peek"}, oracleAction{Op: "peek_ahead"}, oracleAction{Op: "next"})
}
func deterministicOracleRequests(t *testing.T) []oracleRequest {
	t.Helper()
	fixed := []string{"", "text", "{{", "}}", "}{", "{", "}", "{{}}", "{{{}}}", "{name}", "{0}", "{  }", "{:.*}", "{0:.*}", "{name:0$}", "{r#type}", "{_}", "{foo.bar}", "{foo.0}", "{foo.bar.baz}", "{foo?}", "{foo?:x}", "{x=}", "{x,+}", "{x+}", "{x>?}", "{x:?x}", "{x:?#}", "{x:?X}", "{x:}>", "{65536}", "{99999999999999999999999999999999999}", "{:65536.999999x}", "{π}", "{\U00010d50}", "{a\u0301}", "{\u0301}", "{a\x00}", "{a\u0085}", "{a\u200b}", "line\n  \t{name}\n\tend", "🦀{name}é", "{:🦀^9.2?}"}
	literalData, err := os.ReadFile("upstream_test_literals.json")
	if err != nil {
		t.Fatal(err)
	}
	var literals struct {
		Tests []struct {
			Literals []struct{ Kind, Bytes string }
		}
	}
	if err := json.Unmarshal(literalData, &literals); err != nil {
		t.Fatal(err)
	}
	for _, test := range literals.Tests {
		for _, literal := range test.Literals {
			if literal.Kind == "string" {
				value, err := hex.DecodeString(literal.Bytes)
				if err != nil {
					t.Fatal(err)
				}
				fixed = append(fixed, string(value))
			}
		}
	}
	var requests []oracleRequest
	addRequest := func(input, mode string, checked bool, style *uint, snippet *string, appended bool, initial map[string]any, actions []oracleAction) {
		if actions == nil {
			actions = oracleActions(input)
		}
		requests = append(requests, oracleRequest{input, mode, style, snippet, appended, checked, initial, actions})
	}
	modes := []string{"Format", "InlineAsm", "Diagnostic"}
	for _, input := range fixed {
		for _, mode := range modes {
			for _, checked := range []bool{true, false} {
				addRequest(input, mode, checked, nil, nil, false, nil, nil)
				addRequest(input, mode, checked, nil, oraclePtr(oracleQuoted(input)), false, nil, nil)
			}
		}
	}
	for _, base := range []string{"", "{name}", "\n  {0:🦀^8}\n", "é🦀\x00{}", "{{{name}}}"} {
		for hashes := uint(0); hashes < 4; hashes++ {
			for _, appended := range []bool{false, true} {
				for _, mode := range modes {
					for _, checked := range []bool{true, false} {
						input := base
						if appended {
							input += "\n"
						}
						snippet := "r" + strings.Repeat("#", int(hashes)) + "\"" + base + "\"" + strings.Repeat("#", int(hashes))
						addRequest(input, mode, checked, oraclePtr(hashes), oraclePtr(snippet), appended, nil, nil)
					}
				}
			}
		}
	}
	snippets := []string{"", "\"", "\"\"", "\"a", "\"b\"", "\"\\x41\"", "\"\\u{1f980}\"", "\"a\\\n   b\"", "\"a\\\n \n  b\"", "\"\\q\"", "r\"\"", "r#\"é\"#", "r##\"{}\"##", "unrelated"}
	for _, input := range []string{"", "a", "A", "é", "🦀", "ab", "{}", "a\n"} {
		for _, snippet := range snippets {
			for _, style := range []*uint{nil, oraclePtr(uint(0)), oraclePtr(uint(1)), oraclePtr(uint(3))} {
				for _, checked := range []bool{true, false} {
					appended := len(requests)%2 == 0
					addRequest(input, "Format", checked, style, oraclePtr(snippet), appended, nil, nil)
				}
			}
		}
	}
	max := ^uint(0)
	for _, style := range []uint{max, max - 1, max - 2, max / 2, max/2 + 1} {
		for _, input := range []string{"", "x", "{}", "é"} {
			for _, checked := range []bool{true, false} {
				for _, snippet := range []*string{nil, oraclePtr(""), oraclePtr("r\"\"")} {
					addRequest(input, "Format", checked, oraclePtr(style), snippet, false, nil, nil)
				}
			}
		}
	}
	for _, input := range []string{"{}", "{:.*}", "{0:.*}", "{}{}", "}{x?}\n", "\n \t{x}\n"} {
		for _, checked := range []bool{true, false} {
			addRequest(input, "Format", checked, nil, nil, false, map[string]any{"curarg": max, "is_source_literal": true}, nil)
			actions := []oracleAction{{Op: "seed_errors"}, {Op: "set_arg_places", Value: [][]uint{{9, 2}}}, {Op: "set_line_spans", Value: [][]uint{{max, 0}}}, {Op: "set_source", Value: true}, {Op: "next"}, {Op: "set_curarg", Value: max}, {Op: "next"}, {Op: "peek"}, {Op: "set_source", Value: false}, {Op: "next"}, {Op: "clear_errors"}, {Op: "set_source", Value: true}, {Op: "next"}, {Op: "set_errors", Value: []any{oracleSeedError()}}, {Op: "set_line_spans", Value: [][]uint{{1, uint(len(input))}}}, {Op: "next"}, {Op: "next"}}
			addRequest(input, "Format", checked, nil, oraclePtr(oracleQuoted(input)), false, nil, actions)
		}
	}
	for _, checked := range []bool{true, false} {
		for _, input := range []string{"", "x", "{}", "a\nb"} {
			addRequest(input, "Format", checked, nil, nil, false, map[string]any{"is_source_literal": true, "errors": []any{oracleSeedError()}, "arg_places": [][]uint{{3, 1}}, "line_spans": [][]uint{{7, 8}}}, nil)
		}
	}
	seed := uint64(0x428a6f13de957bc0)
	next := func() uint64 { seed ^= seed << 13; seed ^= seed >> 7; seed ^= seed << 17; return seed }
	alphabet := []rune("{}:$.*?#0123456789<>^+-_abcrxX,=?\"\\\n\r\t πé🦀\x00\u0301\u0085\u00a0\u200b\u2028\U00010d50")
	for range 1800 {
		count := int(next() % 33)
		var text strings.Builder
		for range count {
			text.WriteRune(alphabet[next()%uint64(len(alphabet))])
		}
		input := text.String()
		mode := modes[next()%uint64(len(modes))]
		checked := next()%2 == 0
		appended := next()%2 == 0
		var style *uint
		var snippet *string
		switch next() % 5 {
		case 1:
			snippet = oraclePtr(oracleQuoted(input))
			if appended {
				input += "\n"
			}
		case 2:
			hashes := uint(next() % 4)
			style = oraclePtr(hashes)
			snippet = oraclePtr("r" + strings.Repeat("#", int(hashes)) + "\"" + input + "\"" + strings.Repeat("#", int(hashes)))
			if appended {
				input += "\n"
			}
		case 3:
			snippet = oraclePtr("\"\\q\"")
		case 4:
			snippet = oraclePtr("\"")
		}
		var initial map[string]any
		if next()%8 == 0 {
			initial = map[string]any{"curarg": uint(next() % 65540), "is_source_literal": next()%2 == 0}
		}
		addRequest(input, mode, checked, style, snippet, appended, initial, nil)
	}
	return requests
}
func TestDumpOracleRequests(t *testing.T) {
	path, enabled := os.LookupEnv("FERRUGO_PARSE_FORMAT_DUMP_REQUESTS")
	if !enabled {
		t.Skip("development-only deterministic Rust request dump")
	}
	if path == "" {
		t.Fatal("empty request dump path")
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	requests := deterministicOracleRequests(t)
	for _, request := range requests {
		if err := encoder.Encode(request); err != nil {
			file.Close()
			t.Fatal(err)
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("dumped %d deterministic parser-state requests", len(requests))
}

func oracleRange(value Range) any { return []uint{value.Start, value.End} }
func oracleRanges(values []Range) []any {
	result := make([]any, 0, len(values))
	for _, value := range values {
		result = append(result, oracleRange(value))
	}
	return result
}
func oracleOption[T any](option Option[T], convert func(T) any) any {
	if !option.Present {
		return nil
	}
	return convert(option.Value)
}
func oraclePeek(value Option[InputChar]) any {
	return oracleOption(value, func(value InputChar) any {
		return map[string]any{"span": oracleRange(value.Span), "index": value.Index, "char": uint32(value.Char)}
	})
}
func oraclePosition(value Position) any {
	if value.Kind == ArgumentNamed {
		return map[string]any{"kind": "ArgumentNamed", "name": value.Name, "index_value": nil}
	}
	kind := "ArgumentIs"
	if value.Kind == ArgumentImplicitlyIs {
		kind = "ArgumentImplicitlyIs"
	}
	return map[string]any{"kind": kind, "index": value.Index, "index_value": value.IndexValue().Value}
}
func oracleCount(value Count) any {
	switch value.Kind {
	case CountIs:
		return map[string]any{"kind": "CountIs", "value": value.Value}
	case CountIsName:
		return map[string]any{"kind": "CountIsName", "name": value.Name, "span": oracleRange(value.Span)}
	case CountIsParam:
		return map[string]any{"kind": "CountIsParam", "index": value.Index}
	case CountIsStar:
		return map[string]any{"kind": "CountIsStar", "index": value.Index}
	default:
		return map[string]any{"kind": "CountImplied"}
	}
}
func oracleFormat(value FormatSpec) any {
	alignment := map[Alignment]string{AlignUnknown: "AlignUnknown", AlignLeft: "AlignLeft", AlignRight: "AlignRight", AlignCenter: "AlignCenter"}[value.Align]
	return map[string]any{
		"fill": oracleOption(value.Fill, func(v rune) any { return uint32(v) }), "fill_span": oracleOption(value.FillSpan, oracleRange), "align": alignment,
		"sign": oracleOption(value.Sign, func(v Sign) any {
			if v == Plus {
				return "Plus"
			}
			return "Minus"
		}), "alternate": value.Alternate, "zero_pad": value.ZeroPad,
		"debug_hex": oracleOption(value.DebugHex, func(v DebugHex) any {
			if v == Lower {
				return "Lower"
			}
			return "Upper"
		}), "precision": oracleCount(value.Precision), "precision_span": oracleOption(value.PrecisionSpan, oracleRange),
		"width": oracleCount(value.Width), "width_span": oracleOption(value.WidthSpan, oracleRange), "ty": value.Ty, "ty_span": oracleOption(value.TySpan, oracleRange),
	}
}
func oraclePiece(value Piece) any {
	if value.Kind == Lit {
		return map[string]any{"kind": "Lit", "text": value.Text, "debug": value.RustDebug()}
	}
	argument := value.Argument
	return map[string]any{"kind": "NextArgument", "debug": value.RustDebug(), "argument": map[string]any{"position": oraclePosition(argument.Position), "position_span": oracleRange(argument.PositionSpan), "format": oracleFormat(argument.Format), "is_identifier": argument.IsIdentifier()}}
}
func oracleSuggestion(value Suggestion) any {
	switch value.Kind {
	case UsePositional:
		return map[string]any{"kind": "UsePositional"}
	case RemoveRawIdent:
		return map[string]any{"kind": "RemoveRawIdent", "span": oracleRange(value.Span)}
	case ReorderFormatParameter:
		return map[string]any{"kind": "ReorderFormatParameter", "span": oracleRange(value.Span), "replacement": value.Replacement}
	case AddMissingColon:
		return map[string]any{"kind": "AddMissingColon", "span": oracleRange(value.Span)}
	case UseRustDebugPrintingMacro:
		return map[string]any{"kind": "UseRustDebugPrintingMacro"}
	default:
		return map[string]any{"kind": "None"}
	}
}
func oracleError(value ParseError) any {
	return map[string]any{"description": value.Description, "note": oracleOption(value.Note, func(v string) any { return v }), "label": value.Label, "span": oracleRange(value.Span), "secondary_label": oracleOption(value.SecondaryLabel, func(v SecondaryLabel) any { return map[string]any{"label": v.Label, "span": oracleRange(v.Span)} }), "suggestion": oracleSuggestion(value.Suggestion)}
}
func oracleState(parser *Parser) any {
	errors := make([]any, 0, len(parser.Errors))
	for _, value := range parser.Errors {
		errors = append(errors, oracleError(value))
	}
	return map[string]any{"errors": errors, "curarg": parser.Curarg, "arg_places": oracleRanges(parser.ArgPlaces), "is_source_literal": parser.IsSourceLiteral, "line_spans": oracleRanges(parser.LineSpans), "peek": oraclePeek(parser.Peek()), "peek_ahead": oraclePeek(parser.PeekAhead())}
}
func oracleSeedError() any {
	return oracleError(ParseError{Description: "seed-description", Note: Some("seed-note"), Label: "seed-label", Span: Range{3, 5}, SecondaryLabel: Some(SecondaryLabel{"seed-secondary", Range{6, 7}}), Suggestion: Suggestion{Kind: RemoveRawIdent, Span: Range{8, 9}}})
}
func oracleUint(value any) uint {
	switch v := value.(type) {
	case json.Number:
		result, err := strconv.ParseUint(v.String(), 10, strconv.IntSize)
		if err != nil {
			panic(err)
		}
		return uint(result)
	case uint:
		return v
	case uint64:
		return uint(v)
	case int:
		if v < 0 {
			panic("negative native integer")
		}
		return uint(v)
	default:
		panic(fmt.Sprintf("invalid native integer %T", value))
	}
}
func oracleReadRange(value any) Range {
	items := value.([]any)
	return Range{oracleUint(items[0]), oracleUint(items[1])}
}
func oracleReadRanges(value any) []Range {
	items := value.([]any)
	result := make([]Range, 0, len(items))
	for _, item := range items {
		result = append(result, oracleReadRange(item))
	}
	return result
}
func oracleReadSuggestion(value any) Suggestion {
	object := value.(map[string]any)
	kind := object["kind"].(string)
	result := Suggestion{Kind: map[string]SuggestionKind{"None": SuggestionNone, "UsePositional": UsePositional, "RemoveRawIdent": RemoveRawIdent, "ReorderFormatParameter": ReorderFormatParameter, "AddMissingColon": AddMissingColon, "UseRustDebugPrintingMacro": UseRustDebugPrintingMacro}[kind]}
	if span, ok := object["span"]; ok {
		result.Span = oracleReadRange(span)
	}
	if replacement, ok := object["replacement"]; ok {
		result.Replacement = replacement.(string)
	}
	return result
}
func oracleReadErrors(value any) []ParseError {
	values := value.([]any)
	result := make([]ParseError, 0, len(values))
	for _, value := range values {
		object := value.(map[string]any)
		item := ParseError{Description: object["description"].(string), Label: object["label"].(string), Span: oracleReadRange(object["span"]), Suggestion: oracleReadSuggestion(object["suggestion"])}
		if note := object["note"]; note != nil {
			item.Note = Some(note.(string))
		}
		if secondary := object["secondary_label"]; secondary != nil {
			label := secondary.(map[string]any)
			item.SecondaryLabel = Some(SecondaryLabel{label["label"].(string), oracleReadRange(label["span"])})
		}
		result = append(result, item)
	}
	return result
}
func oraclePatch(parser *Parser, initial map[string]any) {
	if value, ok := initial["curarg"]; ok {
		parser.Curarg = oracleUint(value)
	}
	if value, ok := initial["is_source_literal"]; ok {
		parser.IsSourceLiteral = value.(bool)
	}
	if value, ok := initial["errors"]; ok {
		parser.Errors = oracleReadErrors(value)
	}
	if value, ok := initial["arg_places"]; ok {
		parser.ArgPlaces = oracleReadRanges(value)
	}
	if value, ok := initial["line_spans"]; ok {
		parser.LineSpans = oracleReadRanges(value)
	}
}
func oraclePanicKind(value any) any {
	if value == nil {
		return nil
	}
	message := fmt.Sprint(value)
	switch {
	case strings.Contains(message, "attempt to add with overflow"):
		return "add_overflow"
	case strings.Contains(message, "attempt to subtract with overflow"):
		return "sub_overflow"
	case strings.Contains(message, "char boundary"):
		return "char_boundary"
	case strings.Contains(message, "bounds") || strings.Contains(message, "slice"):
		return "bounds"
	default:
		return "other"
	}
}
func oracleCatch(action func()) (kind any) {
	defer func() { kind = oraclePanicKind(recover()) }()
	action()
	return nil
}
func executeOracleRequest(request oracleRequest) any {
	var parser *Parser
	mode := map[string]ParseMode{"Format": Format, "InlineAsm": InlineAsm, "Diagnostic": Diagnostic}[request.Mode]
	overflow := WrappingOverflow
	if request.Checked {
		overflow = CheckedOverflow
	}
	style := Option[uint]{}
	if request.Style != nil {
		style = Some(*request.Style)
	}
	snippet := Option[string]{}
	if request.Snippet != nil {
		snippet = Some(*request.Snippet)
	}
	panicKind := oracleCatch(func() {
		parser = NewParserWithOverflowMode(request.Input, style, snippet, request.AppendedNewline, mode, overflow)
	})
	response := map[string]any{"constructor_panic": panicKind, "initial": nil, "configured": nil, "steps": []any{}}
	if panicKind != nil {
		return response
	}
	response["initial"] = oracleState(parser)
	oraclePatch(parser, request.Initial)
	response["configured"] = oracleState(parser)
	steps := make([]any, 0, len(request.Actions))
	for _, action := range request.Actions {
		var piece, value any
		panicKind := oracleCatch(func() {
			switch action.Op {
			case "next":
				if next, ok := parser.Next(); ok {
					piece = oraclePiece(next)
				}
			case "peek":
				value = oraclePeek(parser.Peek())
			case "peek_ahead":
				value = oraclePeek(parser.PeekAhead())
			case "set_curarg":
				parser.Curarg = oracleUint(action.Value)
			case "set_source":
				parser.IsSourceLiteral = action.Value.(bool)
			case "set_errors":
				parser.Errors = oracleReadErrors(action.Value)
			case "set_arg_places":
				parser.ArgPlaces = oracleReadRanges(action.Value)
			case "set_line_spans":
				parser.LineSpans = oracleReadRanges(action.Value)
			case "clear_errors":
				parser.Errors = parser.Errors[:0]
			case "seed_errors":
				parser.Errors = append(parser.Errors, ParseError{Description: "seed-description", Note: Some("seed-note"), Label: "seed-label", Span: Range{3, 5}, SecondaryLabel: Some(SecondaryLabel{"seed-secondary", Range{6, 7}}), Suggestion: Suggestion{Kind: RemoveRawIdent, Span: Range{8, 9}}})
			default:
				panic("unknown oracle action")
			}
		})
		steps = append(steps, map[string]any{"op": action.Op, "panic": panicKind, "piece": piece, "value": value, "state": oracleState(parser)})
	}
	response["steps"] = steps
	return response
}
func oracleDecode(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	return decoder.Decode(target)
}
func TestOriginalParserOracle(t *testing.T) {
	path := filepath.Join(oracleRoot(t), "tools", "port-rust-parse-format", "cases.jsonl.gz")
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	scanner := bufio.NewScanner(compressed)
	scanner.Buffer(make([]byte, 65536), 16*1024*1024)
	count := 0
	for scanner.Scan() {
		count++
		var fixture struct {
			Request  json.RawMessage `json:"request"`
			Response json.RawMessage `json:"response"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &fixture); err != nil {
			t.Fatal(err)
		}
		var request oracleRequest
		if err := oracleDecode(fixture.Request, &request); err != nil {
			t.Fatal(err)
		}
		actualData, err := json.Marshal(executeOracleRequest(request))
		if err != nil {
			t.Fatal(err)
		}
		var actual, expected any
		if err := oracleDecode(actualData, &actual); err != nil {
			t.Fatal(err)
		}
		if err := oracleDecode(fixture.Response, &expected); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("original parser case %d differs\nrequest=%s\nactual=%s\nexpected=%s", count, fixture.Request, actualData, fixture.Response)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no original parser oracle cases")
	}
	t.Logf("matched %d complete original parser state scripts", count)
}
