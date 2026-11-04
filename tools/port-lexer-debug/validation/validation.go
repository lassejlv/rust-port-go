// SPDX-License-Identifier: MIT OR Apache-2.0
// Shared validation replay, never part of production execution.
package validation

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type Value interface {
	FormatDebug(*rustfmt.Formatter) error
}

func debugPattern(pattern int) rustfmt.Options {
	value := rustfmt.DefaultOptions()
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
func debugState(f *rustfmt.Formatter) string {
	return fmt.Sprintf("%d:%d", f.Options.Fill, f.Options.Align)
}

func Replay(t *testing.T, directory string, values []Value) {
	t.Helper()
	CheckEvidence(t, directory, "debug")
	if strconv.IntSize != 64 {
		t.Skip("original formatter corpus uses64-bit usize")
	}
	compressed, e := os.ReadFile(filepath.Join(directory, "debug.tsv.gz"))
	if e != nil {
		t.Fatal(e)
	}
	metadata, e := os.ReadFile(filepath.Join(directory, "debug-manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var info struct {
		Records    int    `json:"records"`
		Raw        string `json:"raw_sha256"`
		Compressed string `json:"fixture_sha256"`
	}
	if e = json.Unmarshal(metadata, &info); e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(compressed)
	if hex.EncodeToString(sum[:]) != info.Compressed {
		t.Fatal("native fixture compressed digest changed")
	}
	reader, e := gzip.NewReader(bytes.NewReader(compressed))
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	digest := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(reader, digest))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	line := 0
	for scanner.Scan() {
		line++
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) != 13 {
			t.Fatalf("line%d fields%d", line, len(parts))
		}
		number := func(i int) int {
			n, e := strconv.Atoi(parts[i])
			if e != nil {
				t.Fatal(e)
			}
			return n
		}
		id, pattern, fail := number(0), number(1), number(4)
		if id < 0 || id >= len(values) {
			t.Fatalf("invalid case%d", id)
		}
		chars, reuse := parts[2] == "true", parts[3] == "true"
		sink := &debugTraceSink{fail: fail}
		var writer io.Writer = sink
		if chars {
			writer = debugCharSink{sink}
		}
		f := rustfmt.Formatter{Writer: writer, Options: debugPattern(pattern)}
		before := debugState(&f)
		first := values[id].FormatDebug(&f)
		after := debugState(&f)
		var second error
		if reuse {
			second = rustfmt.FormatDebugUint(26, 64, &f)
		}
		for _, e := range []error{first, second} {
			if e != nil && e != debugWriteFailure {
				t.Fatalf("line%d error identity lost:%v", line, e)
			}
		}
		actual := []string{strconv.FormatBool(first == nil && second == nil), strconv.FormatBool(first == nil), strconv.FormatBool(second == nil), before, after, debugState(&f), hex.EncodeToString([]byte(sink.accepted.String())), strings.Join(sink.events, "|")}
		for i, text := range actual {
			if text != parts[i+5] {
				t.Fatalf("line%d case%d pattern%d chars%v reuse%v fail%d field%d\nGo:%s\nRust:%s", line, id, pattern, chars, reuse, fail, i, text, parts[i+5])
			}
		}
	}
	if e = scanner.Err(); e != nil {
		t.Fatal(e)
	}
	if line != info.Records || hex.EncodeToString(digest.Sum(nil)) != info.Raw {
		t.Fatalf("changed native corpus:%d", line)
	}
	t.Logf("compared%d native formatter observations", line)
}
