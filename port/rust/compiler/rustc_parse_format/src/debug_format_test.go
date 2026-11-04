package parseformat

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

//go:embed debug_testdata/native.tsv.gz
var originalDebugCorpus []byte

type debugValue interface{ WriteDebugFormatter(*DebugFormatter) error }

func debugSpec(fill rune) FormatSpec {
	return FormatSpec{Fill: Some(fill), FillSpan: Some(Range{26, ^uint(0)}), Align: AlignCenter, Sign: Some(Minus), Alternate: true, ZeroPad: false, DebugHex: Some(Upper), Precision: Count{Kind: CountIsName, Name: "n\n\"\\\x00\u0301", Span: Range{26, 42}}, PrecisionSpan: Some(Range{1, 9}), Width: Count{Kind: CountIs, Value: 65535}, WidthSpan: Some(Range{0, 26}), Ty: "x\n🦀\u0301\x00", TySpan: Some(Range{26, 42})}
}
func debugArgument(rich bool) Argument {
	value := Argument{Position: Position{Kind: ArgumentNamed, Name: "name"}, PositionSpan: Range{26, 42}}
	if rich {
		value.Position = Position{Kind: ArgumentIs, Index: 26}
		value.Format = debugSpec('\'')
	}
	return value
}
func debugValues() []debugValue {
	plain, rich := debugArgument(false), debugArgument(true)
	return []debugValue{
		Format, InlineAsm, Diagnostic, AlignUnknown, AlignLeft, AlignRight, AlignCenter, Plus, Minus, Lower, Upper,
		Position{Kind: ArgumentImplicitlyIs}, Position{Kind: ArgumentIs, Index: 26}, Position{Kind: ArgumentIs, Index: ^uint(0)},
		Position{Kind: ArgumentNamed}, Position{Kind: ArgumentNamed, Name: "plain"}, Position{Kind: ArgumentNamed, Name: "a\n\"'\\\x00"}, Position{Kind: ArgumentNamed, Name: "\u0301x🦀\u0301"},
		Count{Kind: CountImplied}, Count{Kind: CountIs}, Count{Kind: CountIs, Value: 65535}, Count{Kind: CountIsName}, Count{Kind: CountIsName, Name: "n\n", Span: Range{26, ^uint(0)}}, Count{Kind: CountIsParam, Index: 26}, Count{Kind: CountIsStar, Index: ^uint(0)},
		FormatSpec{}, debugSpec('🦀'), debugSpec(0), debugSpec('\n'), debugSpec('\''), debugSpec('\u0301'), plain, rich,
		Piece{Kind: Lit}, Piece{Kind: Lit, Text: "plain"}, Piece{Kind: Lit, Text: "a\n\"'\\\x00"}, Piece{Kind: Lit, Text: "\u0301x🦀\u0301"}, Piece{Kind: NextArgument, Argument: &plain}, Piece{Kind: NextArgument, Argument: &rich},
		Count{Kind: CountIsName, Name: "range", Span: Range{0, 26}},
	}
}
func debugPattern(pattern int) DebugOptions {
	value := DefaultDebugOptions()
	width := func(n uint) { value.Width = n; value.HasWidth = true }
	switch pattern {
	case 0:
	case 1:
		value.Alternate = true
	case 2:
		value.DebugLowerHex = true
	case 3:
		value.DebugUpperHex = true
	case 4:
		value.Alternate = true
		value.DebugLowerHex = true
	case 5:
		value.SignPlus = true
		value.ZeroPad = true
		width(8)
	case 6:
		value.DebugLowerHex = true
		value.ZeroPad = true
		width(8)
	case 7:
		value.Fill = '*'
		value.Align = rustfmt.AlignRight
		width(12)
	case 8:
		value.Fill = '*'
		value.Align = rustfmt.AlignLeft
		width(12)
	case 9:
		value.Fill = '*'
		value.Align = rustfmt.AlignCenter
		width(13)
	case 10:
		value.HasPrecision = true
		value.Precision = 2
	case 11:
		value.Alternate = true
		value.SignPlus = true
		value.ZeroPad = true
		value.DebugLowerHex = true
		width(8)
	case 12:
		value.SignMinus = true
		value.ZeroPad = true
		width(8)
	case 13:
		value.Fill = '🦀'
		value.Align = rustfmt.AlignCenter
		width(12)
	case 14:
		value.Fill = '0'
		value.Align = rustfmt.AlignRight
		width(12)
	case 15:
		value.Fill = '\n'
		value.Align = rustfmt.AlignRight
		width(8)
	case 16:
		value.Fill = 0
		value.Align = rustfmt.AlignRight
		width(8)
	case 17:
		value.Alternate = true
		value.ZeroPad = true
		width(8)
	case 18:
		value.Alternate = true
		value.HasPrecision = true
		value.Precision = 2
	default:
		panic("invalid pattern")
	}
	return value
}

var debugWriteFailure = errors.New("caller writer failure")

type debugTraceSink struct {
	events   []string
	accepted strings.Builder
	fail     int
}

func (sink *debugTraceSink) event(kind byte, text string) error {
	index := len(sink.events)
	sink.events = append(sink.events, string(kind)+":"+hex.EncodeToString([]byte(text)))
	if index == sink.fail {
		return debugWriteFailure
	}
	sink.accepted.WriteString(text)
	return nil
}
func (sink *debugTraceSink) WriteString(text string) (int, error) {
	if err := sink.event('S', text); err != nil {
		return 0, err
	}
	return len(text), nil
}
func (sink *debugTraceSink) Write(text []byte) (int, error) { return sink.WriteString(string(text)) }

type debugCharSink struct{ *debugTraceSink }

func (sink debugCharSink) WriteChar(c rune) error { return sink.event('C', string(c)) }
func debugState(f *DebugFormatter) string {
	return fmt.Sprintf("%d:%d", f.Options.Fill, f.Options.Align)
}
func TestOriginalParseFormatDebug(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("native corpus records64-bit usize; architecture-neutral API remains compiled")
	}
	compressed, err := gzip.NewReader(bytes.NewReader(originalDebugCorpus))
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	digest := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(compressed, digest))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	values := debugValues()
	line := 0
	for scanner.Scan() {
		line++
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) != 13 {
			t.Fatalf("line%d: fields%d", line, len(parts))
		}
		number := func(i int) int {
			n, e := strconv.Atoi(parts[i])
			if e != nil {
				t.Fatal(e)
			}
			return n
		}
		caseID, pattern, fail := number(0), number(1), number(4)
		chars, reuse := parts[2] == "true", parts[3] == "true"
		sink := &debugTraceSink{fail: fail}
		var writer io.Writer = sink
		if chars {
			writer = debugCharSink{sink}
		}
		f := DebugFormatter{Writer: writer, Options: debugPattern(pattern)}
		before := debugState(&f)
		first := values[caseID].WriteDebugFormatter(&f)
		after := debugState(&f)
		var second error
		if reuse {
			second = rustfmt.FormatDebugUint(26, strconv.IntSize, &f)
		}
		for _, e := range []error{first, second} {
			if e != nil && e != debugWriteFailure {
				t.Fatalf("line%d: caller error identity lost: %v", line, e)
			}
		}
		actual := []string{strconv.FormatBool(first == nil && second == nil), strconv.FormatBool(first == nil), strconv.FormatBool(second == nil), before, after, debugState(&f), hex.EncodeToString([]byte(sink.accepted.String())), strings.Join(sink.events, "|")}
		for i, text := range actual {
			if text != parts[i+5] {
				t.Fatalf("line%d case%d pattern%d chars%v reuse%v fail%d field%d\nGo:%s\nRust:%s", line, caseID, pattern, chars, reuse, fail, i, text, parts[i+5])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if line != 156460 || hex.EncodeToString(digest.Sum(nil)) != "c353d189d5004da84fd30fb9ae9e12a5f4fd39a31caf6a7d3e0704a4369b41f0" {
		t.Fatalf("changed or truncated original Debug corpus:%d", line)
	}
}
func TestDebugConvenienceUsesPrimaryFormatter(t *testing.T) {
	options := DefaultDebugOptions()
	options.Alternate = true
	options.DebugLowerHex = true
	value := debugArgument(true)
	var out strings.Builder
	if err := value.WriteDebug(&out, options); err != nil {
		t.Fatal(err)
	}
	if out.String() != value.RustDebugWith(options) {
		t.Fatal("convenience Debug disagrees")
	}
	if value.RustDebug() != value.RustDebugWith(DefaultDebugOptions()) {
		t.Fatal("default Debug disagrees")
	}
}
