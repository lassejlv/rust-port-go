// SPDX-License-Identifier: MIT OR Apache-2.0
package rustfmt

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

var errNativeWriter = errors.New("native builder writer failure")

type nativeSink struct {
	log         *[]string
	calls, fail int
	chars       bool
}

func (s *nativeSink) record(kind string, text string) error {
	*s.log = append(*s.log, kind+":"+hex.EncodeToString([]byte(text)))
	s.calls++
	if s.calls == s.fail {
		return errNativeWriter
	}
	return nil
}
func (s *nativeSink) Write(text []byte) (int, error) { return s.WriteString(string(text)) }
func (s *nativeSink) WriteString(text string) (int, error) {
	if err := s.record("S", text); err != nil {
		return 0, err
	}
	return len(text), nil
}
func (s *nativeSink) WriteChar(char rune) error {
	kind := "S"
	if s.chars {
		kind = "C"
	}
	return s.record(kind, string(char))
}

func nativeValue(log *[]string, id int, nested, pretty, evaluate bool) Value {
	if evaluate {
		*log = append(*log, fmt.Sprintf("E:%d", id))
	}
	return func(writer io.Writer) error {
		*log = append(*log, fmt.Sprintf("D:%d", id))
		if id >= 2000 {
			if err := WriteChar(writer, '\n'); err != nil {
				return err
			}
			_ = WriteChar(writer, '!')
			return WriteChar(writer, '?')
		}
		if id >= 1000 {
			if err := WriteChar(writer, '\n'); err != nil {
				return err
			}
			_ = WriteString(writer, "attempt")
			return WriteString(writer, "retry")
		}
		if nested {
			return WriteMap(writer, pretty, []Entry{{nativeValue(log, id+10, false, pretty, false), nativeValue(log, id+20, false, pretty, false)}})
		}
		if err := WriteString(writer, ""); err != nil {
			return err
		}
		char := '!'
		if id%2 == 0 {
			char = 'é'
		}
		if err := WriteChar(writer, char); err != nil {
			return err
		}
		if err := WriteChar(writer, '\n'); err != nil {
			return err
		}
		return WriteString(writer, "x\ny")
	}
}
func runNative(kind, count int, pretty, chars bool, fail int, batch bool) (error, []string) {
	var log []string
	sink := &nativeSink{log: &log, chars: chars, fail: fail}
	value := func(id int, nested bool) Value { return nativeValue(&log, id, nested, pretty, !batch) }
	var err error
	switch kind {
	case 0, 5, 6, 7:
		identifier := func(id int) int {
			if kind == 6 {
				return 1000 + id
			}
			if kind == 7 {
				return 2000 + id
			}
			return id
		}
		if batch {
			var fields []Field
			for id := 0; id < count; id++ {
				name := ""
				if id == 0 {
					name = "first"
				}
				fields = append(fields, Field{name, value(identifier(id), kind == 5)})
			}
			err = WriteStruct(sink, "Record", pretty, fields)
		} else {
			builder := BeginStruct(sink, "Record", pretty)
			for id := 0; id < count; id++ {
				name := ""
				if id == 0 {
					name = "first"
				}
				builder.Field(name, value(identifier(id), kind == 5))
			}
			err = builder.Finish()
		}
	case 1, 2:
		name := ""
		if kind == 1 {
			name = "Tuple"
		}
		if batch {
			var fields []Value
			for id := 0; id < count; id++ {
				fields = append(fields, value(id, false))
			}
			err = WriteTuple(sink, name, pretty, fields)
		} else {
			builder := BeginTuple(sink, name, pretty)
			for id := 0; id < count; id++ {
				builder.Field(value(id, false))
			}
			err = builder.Finish()
		}
	case 3:
		if batch {
			var values []Value
			for id := 0; id < count; id++ {
				values = append(values, value(id, false))
			}
			err = WriteList(sink, pretty, values)
		} else {
			builder := BeginList(sink, pretty)
			for id := 0; id < count; id++ {
				builder.Entry(value(id, false))
			}
			err = builder.Finish()
		}
	case 4:
		if batch {
			var entries []Entry
			for id := 0; id < count; id++ {
				entries = append(entries, Entry{value(id*2, false), value(id*2+1, false)})
			}
			err = WriteMap(sink, pretty, entries)
		} else {
			builder := BeginMap(sink, pretty)
			for id := 0; id < count; id++ {
				builder.Entry(value(id*2, false), value(id*2+1, false))
			}
			err = builder.Finish()
		}
	default:
		panic("unknown reference kind")
	}
	return err, log
}

func TestNativeDebugBuilderProtocol(t *testing.T) {
	data, err := os.ReadFile("testdata/native.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile("testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases           int `json:"cases"`
		Raw, Compressed string
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(metadata, &fields); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fields["cases"], &manifest.Cases); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fields["raw_sha256"], &manifest.Raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fields["fixture_sha256"], &manifest.Compressed); err != nil {
		t.Fatal(err)
	}
	digest := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	if digest(data) != manifest.Compressed {
		t.Fatal("compressed native fixture hash differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if digest(raw) != manifest.Raw {
		t.Fatal("original std output hash differs")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	records := 0
	for scanner.Scan() {
		records++
		f := strings.Split(scanner.Text(), "\t")
		if len(f) != 7 {
			t.Fatal("malformed native record")
		}
		number := func(text string) int {
			v, err := strconv.Atoi(text)
			if err != nil {
				t.Fatal(err)
			}
			return v
		}
		boolean := func(text string) bool {
			v, err := strconv.ParseBool(text)
			if err != nil {
				t.Fatal(err)
			}
			return v
		}
		kind, count, pretty, chars, fail := number(f[0]), number(f[1]), boolean(f[2]), boolean(f[3]), number(f[4])
		for _, batch := range []bool{false, true} {
			want := strings.Split(f[6], ",")
			if batch {
				filtered := want[:0:0]
				for _, event := range want {
					if !strings.HasPrefix(event, "E:") {
						filtered = append(filtered, event)
					}
				}
				want = filtered
			}
			err, got := runNative(kind, count, pretty, chars, fail, batch)
			if (err != nil) != boolean(f[5]) || err != nil && !errors.Is(err, errNativeWriter) || strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("record%d kind%d count%d pretty%v chars%v fail%d batch%v:\nGo err%v %v\nRust err%s %v", records, kind, count, pretty, chars, fail, batch, err, got, f[5], want)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if records != manifest.Cases {
		t.Fatalf("records%d want%d", records, manifest.Cases)
	}
}

type shortWriter struct{}

func (shortWriter) Write([]byte) (int, error) { return 0, nil }
func TestShortWritesAndCallbackPanics(t *testing.T) {
	if err := WriteString(shortWriter{}, "not written"); err != io.ErrShortWrite {
		t.Fatalf("short write=%v", err)
	}
	sentinel := new(int)
	for _, pretty := range []bool{false, true} {
		func() {
			defer func() {
				if got := recover(); got != sentinel {
					t.Errorf("callback panic identity changed: %v", got)
				}
			}()
			WriteTuple(io.Discard, "T", pretty, []Value{func(io.Writer) error { panic(sentinel) }})
		}()
	}
}

func TestLiveNativeBuilderProtocol(t *testing.T) {
	compiler, requested := os.LookupEnv("FERRUGO_RUSTC")
	if !requested {
		t.Skip("set FERRUGO_RUSTC to reproduce the native std builder observations")
	}
	if compiler == "" {
		t.Fatal("FERRUGO_RUSTC explicitly empty")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "python3", "testdata/generate.py", "--rustc", compiler, "--check")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native std builders: %v\n%s", err, output)
	} else {
		t.Log(strings.TrimSpace(string(output)))
	}
}
