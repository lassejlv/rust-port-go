// SPDX-License-Identifier: MIT OR Apache-2.0
package rustfmt

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

func nonExhaustiveOptions(pattern int) Options {
	o := DefaultOptions()
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
			o.Fill, o.Align = '*', AlignRight
		}
		if pattern == 14 || pattern == 15 {
			o.Fill, o.Align = '\n', AlignRight
		}
	case 6:
		o.Fill, o.Align, o.Width, o.HasWidth = '*', AlignLeft, 30, true
	case 7:
		o.Fill, o.Align, o.Width, o.HasWidth = '_', AlignCenter, 30, true
	case 12:
		o.Precision, o.HasPrecision = 0, true
	case 13:
		o.Fill, o.Align, o.Width, o.HasWidth, o.Precision, o.HasPrecision = 'é', AlignRight, 30, true, 3, true
	default:
		panic("unknown native formatter pattern")
	}
	return o
}

func nonExhaustiveValue(kind int, events *[]string, f *Formatter) error {
	*events = append(*events, fmt.Sprintf("D:%d", kind))
	switch kind % 3 {
	case 0:
		return FormatDebugUint(^uint64(0), 64, f)
	case 1:
		if err := f.WriteChar('\n'); err != nil {
			return err
		}
		_ = f.WriteString("attempt")
		if err := f.WriteChar('é'); err != nil {
			return err
		}
		return f.WriteString("x\ny")
	default:
		b := BeginStruct(f.Writer, "Child", f.Options.Alternate)
		b.Field("n", f.Value(f.Options.Alternate, func(child *Formatter) error { return FormatDebugUint(15, 64, child) }))
		return b.FinishNonExhaustive()
	}
}

func nonExhaustiveRecord(count int, events *[]string, f *Formatter) error {
	b := BeginStruct(f.Writer, "Record", f.Options.Alternate)
	for i := 0; i < count; i++ {
		*events = append(*events, fmt.Sprintf("E:%d", i))
		name := "field"
		if i == 0 {
			name = "number"
		}
		b.Field(name, f.Value(f.Options.Alternate, func(child *Formatter) error { return nonExhaustiveValue(i, events, child) }))
	}
	return b.FinishNonExhaustive()
}

func TestOriginalDebugStructNonExhaustive(t *testing.T) {
	data, err := os.ReadFile("testdata/non_exhaustive.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile("testdata/non_exhaustive_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases     int    `json:"cases"`
		Fixture   string `json:"fixture_sha256"`
		Raw       string `json:"raw_sha256"`
		Source    string `json:"source_sha256"`
		Generator string `json:"generator_sha256"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, expected := range map[string]string{
		"testdata/non_exhaustive.rs":          manifest.Source,
		"testdata/non_exhaustive_generate.py": manifest.Generator,
	} {
		body, err := os.ReadFile(path)
		if err != nil || digest(body) != expected {
			t.Fatalf("native proof source differs: %s: %v", path, err)
		}
	}
	if digest(data) != manifest.Fixture || manifest.Cases != 20992 {
		t.Fatal("native proof fixture or coverage differs")
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(r, 32<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("native proof body differs: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	cases := 0
	for scanner.Scan() {
		cases++
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 7 {
			t.Fatalf("case %d: malformed fixture", cases)
		}
		pattern, _ := strconv.Atoi(fields[0])
		count, _ := strconv.Atoi(fields[1])
		chars, _ := strconv.ParseBool(fields[2])
		fail, _ := strconv.Atoi(fields[3])
		reuse, _ := strconv.ParseBool(fields[4])
		var events []string
		sink := &nativeSink{log: &events, chars: chars, fail: fail + 1}
		f := &Formatter{Writer: sink, Options: nonExhaustiveOptions(pattern)}
		format := func(n int) error {
			err := nonExhaustiveRecord(n, &events, f)
			events = append(events, fmt.Sprintf("R:%t:%d:%d", err == nil, f.Options.Fill, f.Options.Align))
			return err
		}
		err := format(count)
		if reuse {
			err = format(1)
		}
		if strconv.FormatBool(err == nil) != fields[5] || strings.Join(events, "|") != fields[6] {
			t.Fatalf("case %d (%s):\n got %t %s\nwant %s %s", cases, strings.Join(fields[:5], "\t"), err == nil, strings.Join(events, "|"), fields[5], fields[6])
		}
	}
	if err := scanner.Err(); err != nil || cases != manifest.Cases {
		t.Fatalf("native proof incomplete: %d: %v", cases, err)
	}
}
