package datastructures

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func baseNFormatValues() []BaseNString {
	var result []BaseNString
	for _, base := range []uint{2, 3, 10, 36, 62, 64} {
		for _, n := range []BaseNU128{{}, {Lo: 1}, {Lo: 35}, {Lo: 63}, {Lo: 64}, {Lo: ^uint64(0)}, {Hi: ^uint64(0), Lo: ^uint64(0)}} {
			result = append(result, BaseNToBase128(n, base), BaseNToBaseFixedLen128(n, base))
		}
	}
	for _, pair := range [][2]uint{{7, 0}, {42, 1}, {7, 5}, {7, 128}} {
		result = append(result, baseNWithLength(BaseNToBase128(BaseNU128{Lo: uint64(pair[0])}, 10), pair[1]))
	}
	return result
}

func baseNFormatOptions(pattern int) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	switch pattern {
	case 1:
		o.Alternate = true
	case 2:
		o.DebugLowerHex = true
	case 3:
		o.DebugUpperHex = true
	case 4:
		o.Alternate, o.DebugLowerHex = true, true
	case 5:
		o.SignPlus, o.ZeroPad, o.HasWidth, o.Width = true, true, true, 8
	case 6:
		o.DebugLowerHex, o.ZeroPad, o.HasWidth, o.Width = true, true, true, 8
	case 7:
		o.Fill, o.Align, o.HasWidth, o.Width = '*', rustfmt.AlignRight, true, 12
	case 8:
		o.Fill, o.Align, o.HasWidth, o.Width = '*', rustfmt.AlignLeft, true, 12
	case 9:
		o.Fill, o.Align, o.HasWidth, o.Width = '*', rustfmt.AlignCenter, true, 13
	case 10:
		o.HasPrecision, o.Precision = true, 2
	case 11:
		o.Alternate, o.SignPlus, o.ZeroPad, o.DebugLowerHex, o.HasWidth, o.Width = true, true, true, true, true, 8
	case 12:
		o.SignMinus, o.ZeroPad, o.HasWidth, o.Width = true, true, true, 8
	case 13:
		o.Fill, o.Align, o.HasWidth, o.Width = '🦀', rustfmt.AlignCenter, true, 12
	case 14:
		o.Fill, o.Align, o.HasWidth, o.Width = '0', rustfmt.AlignRight, true, 12
	case 15:
		o.Fill, o.Align, o.HasWidth, o.Width = '\n', rustfmt.AlignRight, true, 8
	case 16:
		o.Fill, o.Align, o.HasWidth, o.Width = '\x00', rustfmt.AlignRight, true, 8
	case 17:
		o.Alternate, o.ZeroPad, o.HasWidth, o.Width = true, true, true, 8
	case 18:
		o.Alternate, o.HasPrecision, o.Precision = true, true, 2
	}
	return o
}

func baseNFormatterState(o rustfmt.Options) string {
	width, precision := "-", "-"
	if o.HasWidth {
		width = strconv.FormatUint(uint64(o.Width), 10)
	}
	if o.HasPrecision {
		precision = strconv.FormatUint(uint64(o.Precision), 10)
	}
	flags := uint32(0)
	for i, value := range []bool{o.SignPlus, o.SignMinus, o.Alternate, o.ZeroPad, o.DebugLowerHex, o.DebugUpperHex} {
		if value {
			flags |= 1 << i
		}
	}
	return fmt.Sprintf("%d:%d:%s:%s:%d", o.Fill, o.Align, width, precision, flags)
}

type baseNFormatSink struct {
	chars    bool
	fail     int
	expected *byte
	calls    []string
	accepted strings.Builder
}

var baseNFormatError = errors.New("injected formatter failure")

func (s *baseNFormatSink) WriteString(text string) (int, error) {
	s.calls = append(s.calls, "S:"+hex.EncodeToString([]byte(text))+":"+strconv.FormatBool(unsafe.StringData(text) == s.expected))
	if len(s.calls)-1 == s.fail {
		return 0, baseNFormatError
	}
	return s.accepted.WriteString(text)
}
func (s *baseNFormatSink) Write(bytes []byte) (int, error) { return s.WriteString(string(bytes)) }
func (s *baseNFormatSink) WriteChar(c rune) error {
	if s.chars {
		s.calls = append(s.calls, fmt.Sprintf("C:%c", c))
		return baseNFormatError
	}
	_, err := s.WriteString(string(c))
	return err
}

func TestBaseNFullBorrowedDisplayAgainstNative(t *testing.T) {
	file, err := os.Open("testdata/base_n_formats.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	values := baseNFormatValues()
	scanner := bufio.NewScanner(io.LimitReader(z, 32<<20))
	records := 0
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 13 {
			t.Fatal("native formatter schema")
		}
		number := func(at int) int {
			n, err := strconv.Atoi(fields[at])
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
		boolean := func(at int) bool {
			n, err := strconv.ParseBool(fields[at])
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
		value := &values[number(0)]
		borrow := value.Deref()
		if unsafe.StringData(borrow) != unsafe.StringData(value.AsRefBorrowed()) || unsafe.StringData(borrow) != unsafe.StringData(value.Deref()) {
			t.Fatal("source borrows lost identity")
		}
		sink := &baseNFormatSink{chars: boolean(2), fail: number(4), expected: unsafe.StringData(borrow)}
		opts := baseNFormatOptions(number(1))
		f := &rustfmt.Formatter{Writer: sink, Options: opts}
		before := baseNFormatterState(f.Options)
		first := value.WriteDisplayFormatter(f) == nil
		after := baseNFormatterState(f.Options)
		second := true
		if boolean(3) {
			second = value.WriteDisplayFormatter(f) == nil
		}
		final := baseNFormatterState(f.Options)
		got := []string{strconv.FormatBool(first && second), strconv.FormatBool(first), strconv.FormatBool(second), before, after, final, hex.EncodeToString([]byte(sink.accepted.String())), strings.Join(sink.calls, "|")}
		if strings.Join(got, "\t") != strings.Join(fields[5:], "\t") {
			t.Fatalf("native record %d:\nactual %q\nexpected %q", records, got, fields[5:])
		}
		if f.Options != opts {
			t.Fatal("Display mutated caller formatter state")
		}
		records++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if records != 26752 {
		t.Fatalf("native formatter record count %d", records)
	}
}
