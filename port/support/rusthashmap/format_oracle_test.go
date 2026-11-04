// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

var errMapWriter = errors.New("original map writer failure")

type mapFormatSink struct {
	events       *[]string
	writes, fail int
	chars        bool
}

func (s *mapFormatSink) record(kind, text string) error {
	*s.events = append(*s.events, kind+":"+hex.EncodeToString([]byte(text)))
	n := s.writes
	s.writes++
	if n == s.fail {
		return errMapWriter
	}
	return nil
}
func (s *mapFormatSink) Write(p []byte) (int, error) { return s.WriteString(string(p)) }
func (s *mapFormatSink) WriteString(text string) (int, error) {
	if err := s.record("S", text); err != nil {
		return 0, err
	}
	return len(text), nil
}
func (s *mapFormatSink) WriteChar(c rune) error {
	kind := "S"
	if s.chars {
		kind = "C"
	}
	return s.record(kind, string(c))
}
func mapFormatOptions(pattern int) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	switch pattern {
	case 0:
	case 1:
		o.Alternate = true
	case 2, 3, 4, 5, 8, 9, 10, 11, 14, 15:
		o.Width, o.HasWidth, o.ZeroPad = 30, true, true
		o.Alternate = pattern == 3 || pattern == 9 || pattern == 11 || pattern == 15
		o.SignPlus = pattern == 4 || pattern == 5 || pattern == 14 || pattern == 15
		o.DebugLowerHex = pattern == 8 || pattern == 9
		o.DebugUpperHex = pattern == 10 || pattern == 11
		if pattern == 5 {
			o.Fill, o.Align = '*', rustfmt.AlignRight
		}
		if pattern == 14 || pattern == 15 {
			o.Fill, o.Align = '\n', rustfmt.AlignRight
		}
	case 6:
		o.Fill, o.Align, o.Width, o.HasWidth = '*', rustfmt.AlignLeft, 30, true
	case 7:
		o.Fill, o.Align, o.Width, o.HasWidth = '_', rustfmt.AlignCenter, 30, true
	case 12:
		o.Precision, o.HasPrecision = 0, true
	case 13:
		o.Fill, o.Align, o.Width, o.HasWidth, o.Precision, o.HasPrecision = 'é', rustfmt.AlignRight, 30, true, 3, true
	default:
		panic("unknown format pattern")
	}
	return o
}
func mapFormatTarget(kind int) (func(*rustfmt.Formatter) error, func()) {
	c := Context{}
	keys := KeyOps[uint64]{Hash: func(v *uint64, h Hasher, c Context) { HashU64(h, *v) }, Equal: func(a, b *uint64, c Context) bool { return *a == *b }, TrivialClone: true, Debug: func(v *uint64, f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(*v, 64, f) }}
	values := ValueOps[uint64]{TrivialClone: true, Debug: keys.Debug}
	m := NewMap[uint64, uint64](Layout{16, 8}, DefaultProfile(), keys, values, FxBuilder{64})
	m.Insert(1, ^uint64(0), c)
	m.Insert(^uint64(0), 1, c)
	m.Insert(16, 2, c)
	set := NewSet(Layout{8, 8}, DefaultProfile(), keys, FxBuilder{64})
	set.Insert(1, c)
	set.Insert(^uint64(0), c)
	set.Insert(16, c)
	closeBase := func() { set.Close(c); m.Close(c) }
	switch kind {
	case 0:
		return m.WriteDebugFormatter, closeBase
	case 1:
		return set.WriteDebugFormatter, closeBase
	case 2, 3:
		key := uint64(1)
		if kind == 3 {
			key = 2
		}
		entry := m.Entry(key, c)
		return entry.WriteDebugFormatter, func() { entry.Close(c); closeBase() }
	case 4:
		entry, _ := m.Entry(1, c).TakeOccupied(c)
		return entry.WriteDebugFormatter, func() { entry.Close(c); closeBase() }
	case 5:
		entry, _ := m.Entry(2, c).TakeVacant(c)
		return entry.WriteDebugFormatter, func() { entry.Close(c); closeBase() }
	case 6:
		_, occupied := m.TryInsert(1, 999, c)
		return occupied.WriteDebugFormatter, func() { occupied.Close(c); closeBase() }
	default:
		panic("unknown format target")
	}
}

func TestOriginalMapSetEntryFormatting(t *testing.T) {
	metadata, err := os.ReadFile("../../../tools/port-rust-unord/table/format_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases     int    `json:"cases"`
		Fixture   string `json:"fixture_sha256"`
		Raw       string `json:"raw_sha256"`
		Oracle    string `json:"oracle_sha256"`
		Generator string `json:"generator_sha256"`
		Support   string `json:"support_generator_sha256"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{"../../../tools/port-rust-unord/table/format_oracle.rs": manifest.Oracle, "../../../tools/port-rust-unord/table/format_generate.py": manifest.Generator, "../../../tools/port-rust-unord/table/generate.py": manifest.Support} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("format reference binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/format.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if digest(compressed) != manifest.Fixture || manifest.Cases != 43160 {
		t.Fatal("format reference fixture/coverage differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(reader, 64<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("format reference body differs:%v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	cases := 0
	for scanner.Scan() {
		cases++
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 7 {
			t.Fatalf("malformed format case%d", cases)
		}
		kind, _ := strconv.Atoi(fields[0])
		pattern, _ := strconv.Atoi(fields[1])
		chars, _ := strconv.ParseBool(fields[2])
		fail, _ := strconv.Atoi(fields[3])
		reuse, _ := strconv.ParseBool(fields[4])
		var events []string
		sink := &mapFormatSink{events: &events, fail: fail, chars: chars}
		f := &rustfmt.Formatter{Options: mapFormatOptions(pattern), Writer: sink}
		format, closeTarget := mapFormatTarget(kind)
		call := func() error {
			result := format(f)
			if result != nil && result != errMapWriter {
				t.Fatalf("format writer error identity changed:%v", result)
			}
			events = append(events, fmt.Sprintf("R:%t:%d:%d", result == nil, f.Options.Fill, f.Options.Align))
			return result
		}
		result := call()
		if reuse {
			result = call()
		}
		closeTarget()
		if strconv.FormatBool(result == nil) != fields[5] || strings.Join(events, "|") != fields[6] {
			t.Fatalf("format case%d (%s):\ngot%t %s\nwant%s %s", cases, strings.Join(fields[:5], "\t"), result == nil, strings.Join(events, "|"), fields[5], fields[6])
		}
	}
	if err := scanner.Err(); err != nil || cases != manifest.Cases {
		t.Fatalf("format reference incomplete:%d:%v", cases, err)
	}
}
