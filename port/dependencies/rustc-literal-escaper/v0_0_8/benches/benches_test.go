// Port of all 27 workloads in rustc-literal-escaper 0.0.8 benches/benches.rs.
package literalescaper_test

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	esc "github.com/lassejlv/rust-port-go/port/dependencies/rustc-literal-escaper/v0_0_8/src"
)

const benchLen = 10_000

type benchEvent struct {
	start, end int
	kind       byte
	value      rune
}
type fragment struct {
	input string
	value rune
	high  bool
}
type workload struct {
	name     string
	op       byte
	input    string
	expected []benchEvent
}

func streamEvents(op byte, input string) []benchEvent {
	var output []benchEvent
	appendEvent := func(span esc.Range, kind byte, value rune, err esc.EscapeError) {
		if err != esc.NoError {
			kind = 0
			value = rune(err)
		}
		output = append(output, benchEvent{span.Start, span.End, kind, value})
	}
	switch op {
	case 2, 5:
		callback := func(span esc.Range, result esc.Result[rune]) { appendEvent(span, 1, result.Value, result.Error) }
		if op == 2 {
			esc.UnescapeStr(input, callback)
		} else {
			esc.CheckRawStr(input, callback)
		}
	case 3, 6:
		callback := func(span esc.Range, result esc.Result[byte]) { appendEvent(span, 2, rune(result.Value), result.Error) }
		if op == 3 {
			esc.UnescapeByteStr(input, callback)
		} else {
			esc.CheckRawByteStr(input, callback)
		}
	case 4:
		esc.UnescapeCStr(input, func(span esc.Range, result esc.Result[esc.MixedUnit]) {
			if result.Error != esc.NoError {
				appendEvent(span, 0, 0, result.Error)
				return
			}
			if c, ok := result.Value.Char(); ok {
				appendEvent(span, 3, c.Get(), esc.NoError)
			} else {
				b, _ := result.Value.HighByte()
				appendEvent(span, 4, rune(b.Get()), esc.NoError)
			}
		})
	case 7:
		esc.CheckRawCStr(input, func(span esc.Range, result esc.Result[esc.NonZeroChar]) {
			if result.Error != esc.NoError {
				appendEvent(span, 0, 0, result.Error)
			} else {
				appendEvent(span, 5, result.Value.Get(), esc.NoError)
			}
		})
	default:
		panic("invalid benchmark operation")
	}
	return output
}

func makeWorkload(name string, op byte, fragments ...fragment) workload {
	var input strings.Builder
	var expected []benchEvent
	for _, fragment := range fragments {
		start := input.Len()
		input.WriteString(fragment.input)
		kind := byte(1)
		if op == 3 || op == 6 {
			kind = 2
		}
		if op == 4 {
			kind = 3
			if fragment.high {
				kind = 4
			}
		}
		if op == 7 {
			kind = 5
		}
		expected = append(expected, benchEvent{start, input.Len(), kind, fragment.value})
	}
	return workload{name, op, input.String(), expected}
}
func plainWorkload(name string, op byte, input string) workload {
	var fragments []fragment
	for start, c := range input {
		fragments = append(fragments, fragment{input[start : start+utf8.RuneLen(c)], c, false})
	}
	return makeWorkload(name, op, fragments...)
}

var benchmarkWorkloads = []workload{
	plainWorkload("bench_check_raw_str_ascii", 5, "a"),
	plainWorkload("bench_check_raw_str_non_ascii", 5, "🦀"),
	plainWorkload("bench_check_raw_str_unicode", 5, "a🦀🚀z"),
	plainWorkload("bench_check_raw_byte_str_ascii", 6, "a"),
	plainWorkload("bench_check_raw_c_str_ascii", 7, "a"),
	plainWorkload("bench_check_raw_c_str_non_ascii", 7, "🦀"),
	plainWorkload("bench_check_raw_c_str_unicode", 7, "a🦀🚀z"),
	plainWorkload("bench_unescape_str_ascii", 2, "a"),
	plainWorkload("bench_unescape_str_non_ascii", 2, "🦀"),
	plainWorkload("bench_unescape_str_unicode", 2, "a🦀🚀z"),
	makeWorkload("bench_unescape_str_ascii_escape", 2, fragment{`\n`, '\n', false}),
	makeWorkload("bench_unescape_str_hex_escape", 2, fragment{`\x22`, '"', false}),
	makeWorkload("bench_unescape_str_unicode_escape", 2, fragment{`\u{1f980}`, '🦀', false}, fragment{`\u{1f680}`, '🚀', false}),
	makeWorkload("bench_unescape_str_mixed_escape", 2, fragment{`\n`, '\n', false}, fragment{`\x22`, '"', false}, fragment{`\u{1f980}`, '🦀', false}, fragment{`\u{1f680}`, '🚀', false}),
	plainWorkload("bench_unescape_byte_str_ascii", 3, "a"),
	makeWorkload("bench_unescape_byte_str_ascii_escape", 3, fragment{`\n`, '\n', false}),
	makeWorkload("bench_unescape_byte_str_hex_escape", 3, fragment{`\xff`, 255, false}),
	makeWorkload("bench_unescape_byte_str_mixed_escape", 3, fragment{"a", 'a', false}, fragment{`\n`, '\n', false}, fragment{`\xff`, 255, false}, fragment{"z", 'z', false}),
	plainWorkload("bench_unescape_c_str_ascii", 4, "a"),
	plainWorkload("bench_unescape_c_str_non_ascii", 4, "🦀"),
	plainWorkload("bench_unescape_c_str_unicode", 4, "a🦀🚀z"),
	makeWorkload("bench_unescape_c_str_ascii_escape", 4, fragment{`\n`, '\n', false}),
	makeWorkload("bench_unescape_c_str_hex_escape_ascii", 4, fragment{`\x22`, '"', false}),
	makeWorkload("bench_unescape_c_str_hex_escape_byte", 4, fragment{`\xff`, 255, true}),
	makeWorkload("bench_unescape_c_str_unicode_escape", 4, fragment{`\u{1f980}`, '🦀', false}),
	makeWorkload("bench_unescape_c_str_mixed_escape", 4, fragment{`\n`, '\n', false}, fragment{`\x22`, '"', false}, fragment{`\u{1f980}`, '🦀', false}, fragment{`\u{1f680}`, '🚀', false}, fragment{`\xff`, 255, true}),
}

func checkWorkload(workload workload, input string) error {
	got := streamEvents(workload.op, input)
	if len(got) != benchLen*len(workload.expected) {
		return fmt.Errorf("got %d events, want %d", len(got), benchLen*len(workload.expected))
	}
	for i, event := range got {
		want := workload.expected[i%len(workload.expected)]
		offset := (i / len(workload.expected)) * len(workload.input)
		want.start += offset
		want.end += offset
		if event != want {
			return fmt.Errorf("event %d = %#v, want %#v", i, event, want)
		}
	}
	return nil
}
func skipInput() string { return "\\\n" + strings.Repeat(" ", benchLen-1) + "\n" }
func checkSkip(input string) error {
	got := streamEvents(2, input)
	want := benchEvent{0, benchLen + 2, 0, rune(esc.MultipleSkippedLinesWarning)}
	if len(got) != 1 || got[0] != want {
		return fmt.Errorf("skip output = %#v, want %#v", got, want)
	}
	return nil
}

func TestAllUpstreamBenchmarkWorkloads(t *testing.T) {
	if len(benchmarkWorkloads) != 26 {
		t.Fatal("missing upstream benchmark")
	}
	for _, workload := range benchmarkWorkloads {
		t.Run(workload.name, func(t *testing.T) {
			if err := checkWorkload(workload, strings.Repeat(workload.input, benchLen)); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("bench_skip_ascii_whitespace", func(t *testing.T) {
		if err := checkSkip(skipInput()); err != nil {
			t.Fatal(err)
		}
	})
}

func BenchmarkUpstreamWorkloads(b *testing.B) {
	for _, workload := range benchmarkWorkloads {
		b.Run(workload.name, func(b *testing.B) {
			input := strings.Repeat(workload.input, benchLen)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := checkWorkload(workload, input); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
	b.Run("bench_skip_ascii_whitespace", func(b *testing.B) {
		input := skipInput()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if err := checkSkip(input); err != nil {
				b.Fatal(err)
			}
		}
	})
}
