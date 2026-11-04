package cargoutilschemas

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type nameFormatState struct {
	Fill         uint32
	Align, Flags byte
	HasWidth     bool
	Width        uint64
	HasPrecision bool
	Precision    uint64
}
type nameFormatWrite struct {
	Kind byte
	Text string
}

func nameFormatStateOf(f *rustfmt.Formatter) nameFormatState {
	o := f.Options
	flags := byte(0)
	if o.Alternate {
		flags |= 1
	}
	if o.SignPlus {
		flags |= 2
	}
	if o.SignMinus {
		flags |= 4
	}
	if o.ZeroPad {
		flags |= 8
	}
	return nameFormatState{uint32(o.Fill), byte(o.Align), flags, o.HasWidth, uint64(o.Width), o.HasPrecision, uint64(o.Precision)}
}
func nameFormatReadState(r nameFixtureReader) nameFormatState {
	state := nameFormatState{Fill: r.u32(), Align: r.byte(), Flags: r.byte()}
	u64 := func() uint64 {
		var n uint64
		if err := binary.Read(r, binary.LittleEndian, &n); err != nil {
			r.t.Fatal(err)
		}
		return n
	}
	state.HasWidth = r.byte() != 0
	state.Width = u64()
	state.HasPrecision = r.byte() != 0
	state.Precision = u64()
	return state
}

var errNameFormatWrite = errors.New("name formatter write failure")

type nameFormatSink struct {
	Writes   []nameFormatWrite
	Accepted strings.Builder
	Chars    bool
	Fail     uint32
}

func (s *nameFormatSink) record(kind byte, text string) error {
	index := uint32(len(s.Writes))
	s.Writes = append(s.Writes, nameFormatWrite{kind, text})
	if index == s.Fail {
		return errNameFormatWrite
	}
	s.Accepted.WriteString(text)
	return nil
}
func (s *nameFormatSink) Write(text []byte) (int, error) { return s.WriteString(string(text)) }
func (s *nameFormatSink) WriteString(text string) (int, error) {
	if err := s.record(0, text); err != nil {
		return 0, err
	}
	return len(text), nil
}
func (s *nameFormatSink) WriteChar(ch rune) error {
	kind := byte(0)
	if s.Chars {
		kind = 1
	}
	return s.record(kind, string(ch))
}
func nameFormatOptions(pattern byte) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	width := func(n uint) { o.Width = n; o.HasWidth = true }
	switch pattern {
	case 0:
	case 1:
		o.Alternate = true
	case 2:
		o.DebugLowerHex = true
	case 3:
		o.DebugUpperHex = true
	case 4:
		o.Alternate = true
		o.DebugLowerHex = true
	case 5:
		o.SignPlus = true
		o.ZeroPad = true
		width(8)
	case 6:
		o.DebugLowerHex = true
		o.ZeroPad = true
		width(8)
	case 7:
		o.Fill = '*'
		o.Align = rustfmt.AlignRight
		width(12)
	case 8:
		o.Fill = '*'
		o.Align = rustfmt.AlignLeft
		width(12)
	case 9:
		o.Fill = '*'
		o.Align = rustfmt.AlignCenter
		width(13)
	case 10:
		o.HasPrecision = true
		o.Precision = 2
	case 11:
		o.Alternate = true
		o.SignPlus = true
		o.ZeroPad = true
		o.DebugLowerHex = true
		width(8)
	case 12:
		o.SignMinus = true
		o.ZeroPad = true
		width(8)
	case 13:
		o.Fill = '🦀'
		o.Align = rustfmt.AlignCenter
		width(12)
	case 14:
		o.Fill = '0'
		o.Align = rustfmt.AlignRight
		width(12)
	case 15:
		o.Fill = '\n'
		o.Align = rustfmt.AlignRight
		width(8)
	case 16:
		o.Fill = 0
		o.Align = rustfmt.AlignRight
		width(8)
	case 17:
		o.Alternate = true
		o.ZeroPad = true
		width(8)
	case 18:
		o.Alternate = true
		o.HasPrecision = true
		o.Precision = 2
	default:
		panic("unknown original formatter pattern")
	}
	return o
}

func TestOriginalRestrictedNamesFormatting(t *testing.T) {
	r := nameFixtureReader{bytes.NewReader(nameFixture(t, "testdata/restricted_names_formats.bin.gz")), t}
	r.magic("RNCF")
	size := r.u32()
	if uint64(size) > uint64(r.Len()) {
		t.Fatal("short formatter request stream")
	}
	request := make([]byte, size)
	if _, err := io.ReadFull(r, request); err != nil {
		t.Fatal(err)
	}
	q := nameFixtureReader{bytes.NewReader(request), t}
	q.magic("RNF1")
	count := q.u32()
	r.magic("RNG1")
	if r.u32() != count || count != 31664 {
		t.Fatal("original formatter corpus count differs")
	}
	writes, failures := 0, 0
	for index := uint32(0); index < count; index++ {
		op, name, what := q.byte(), q.text(), q.text()
		debug, pattern, chars, reuse, fail := q.byte() != 0, q.byte(), q.byte() != 0, q.byte() != 0, q.u32()
		firstOK, reuseOK, ok := r.byte() != 0, r.byte() != 0, r.byte() != 0
		states := [3]nameFormatState{nameFormatReadState(r), nameFormatReadState(r), nameFormatReadState(r)}
		n := r.u32()
		if n > 4096 {
			t.Fatal("too many original writer calls")
		}
		wanted := make([]nameFormatWrite, n)
		for i := range wanted {
			wanted[i] = nameFormatWrite{r.byte(), r.text()}
		}
		accepted := r.text()
		var failure error
		switch op {
		case 0:
			failure = ValidatePackageName(name)
		case 1:
			failure = ValidateRegistryName(name)
		case 2:
			failure = ValidateName(name, what)
		case 3:
			failure = ValidateProfileName(name)
		case 4:
			failure = ValidateFeatureName(name)
		case 5:
			failure = ValidatePathBaseName(name)
		default:
			t.Fatal("unknown validator")
		}
		var value *NameValidationError
		if !errors.As(failure, &value) {
			t.Fatalf("format input%d did not produce validation error", index)
		}
		sink := &nameFormatSink{Chars: chars, Fail: fail, Writes: make([]nameFormatWrite, 0)}
		f := &rustfmt.Formatter{Writer: sink, Options: nameFormatOptions(pattern)}
		actualStates := [3]nameFormatState{nameFormatStateOf(f)}
		var first error
		if debug {
			first = value.WriteDebugFormatter(f)
		} else {
			first = value.WriteDisplayFormatter(f)
		}
		actualStates[1] = nameFormatStateOf(f)
		var second error
		if reuse {
			second = rustfmt.FormatDebugUint(26, uint(strconv.IntSize), f)
		}
		actualStates[2] = nameFormatStateOf(f)
		if (first == nil) != firstOK || (second == nil) != reuseOK || (first == nil && second == nil) != ok || actualStates != states || !reflect.DeepEqual(sink.Writes, wanted) || sink.Accepted.String() != accepted {
			t.Fatalf("formatter case%d op%d input%q pattern%d debug%v chars%v fail%d:\nGo status%v/%v states%+v writes%+v accepted%q\nRust status%v/%v states%+v writes%+v accepted%q", index, op, name, pattern, debug, chars, fail, first, second, actualStates, sink.Writes, sink.Accepted.String(), firstOK, reuseOK, states, wanted, accepted)
		}
		writes += len(wanted)
		if !ok {
			failures++
		}
	}
	if r.Len() != 0 || q.Len() != 0 {
		t.Fatal("trailing formatter corpus bytes")
	}
	t.Logf("%d original formatter scripts,%d attempted C/S writes,%d failing scripts", count, writes, failures)
}

func TestNameErrorGoFormattingBridge(t *testing.T) {
	err := ValidatePackageName("")
	if actual := fmt.Sprintf("%30.2v", err); actual != "package name cannot be empty" {
		t.Fatalf("outer Go padding/precision changed source Display:%q", actual)
	}
	if actual := fmt.Sprintf("%#v", err); actual != "NameValidationError(Empty(\"package name\"))" {
		t.Fatalf("compact Rust Debug bridge:%q", actual)
	}
}
