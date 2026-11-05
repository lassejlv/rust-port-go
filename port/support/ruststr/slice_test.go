package ruststr

import (
	"bufio"
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
	"unicode/utf8"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type slicingManifest struct {
	Cases, Panics int
	FixtureSHA    string `json:"fixture_sha256"`
	RawSHA        string `json:"raw_sha256"`
	Locations     map[string]rustfmt.SourceLocation
}

func TestOriginalSDKStr(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("retained original SDK profile uses 64-bit usize")
	}
	manifestBytes, err := os.ReadFile("testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest slicingManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	compressed, err := os.ReadFile("testdata/cases.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) > 64<<20 || fmt.Sprintf("%x", sha256.Sum256(compressed)) != manifest.FixtureSHA {
		t.Fatal("original fixture checksum/bound")
	}
	stream, err := gzip.NewReader(strings.NewReader(string(compressed)))
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	hash := sha256.New()
	scan := bufio.NewScanner(io.TeeReader(io.LimitReader(stream, 64<<20+1), hash))
	scan.Buffer(make([]byte, 4096), 1<<20)
	count, panics := 0, 0
	for scan.Scan() {
		count++
		fields := strings.Split(scan.Text(), "\t")
		if len(fields) != 6 {
			t.Fatalf("line%d: invalid fixture shape", count)
		}
		bytes, err := hex.DecodeString(fields[0])
		if err != nil || !utf8.Valid(bytes) {
			t.Fatalf("line%d: invalid Rust str", count)
		}
		text := string(bytes)
		begin, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		end, err := strconv.ParseUint(fields[3], 10, 64)
		if err != nil {
			t.Fatal(err)
		}
		var hook string
		ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: manifest.Locations, Hook: func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
			if info.Location == nil {
				t.Fatal("missing explicit original caller location")
			}
			payload, ok := info.Payload.(string)
			if !ok {
				t.Fatal("unexpected panic payload type")
			}
			hook = fmt.Sprintf("%s:%s:%s:%d:%d:%t:%t:%t", info.Kind, hex.EncodeToString([]byte(payload)), hex.EncodeToString([]byte(info.Location.File)), info.Location.Line, info.Location.Column, info.CanUnwind, info.ForceNoBacktrace, c.Panicking())
		}})
		result := "panic"
		func() {
			defer func() {
				if p := recover(); p != nil {
					panics++
				}
			}()
			var value string
			ok := true
			switch fields[1] {
			case "range":
				value = Slice(text, uint(begin), uint(end), ctx, "range")
			case "from":
				value = SliceFrom(text, uint(begin), ctx, "from")
			case "to":
				value = SliceTo(text, uint(end), ctx, "to")
			case "inclusive":
				value = SliceInclusive(text, Inclusive(uint(begin), uint(end)), ctx, "inclusive")
			case "to_inclusive":
				value = SliceToInclusive(text, uint(end), ctx, "to_inclusive")
			case "exhausted":
				value = SliceInclusive(text, ExhaustedInclusive(uint(begin)), ctx, "exhausted")
			case "get":
				value, ok = Get(text, uint(begin), uint(end))
			case "get_inclusive":
				value, ok = GetInclusive(text, Inclusive(uint(begin), uint(end)))
			case "get_exhausted":
				value, ok = GetInclusive(text, ExhaustedInclusive(uint(begin)))
			case "boundaries":
				result = fmt.Sprintf("ok:%t:%d:%d", IsCharBoundary(text, uint(begin)), FloorCharBoundary(text, uint(begin)), CeilCharBoundary(text, uint(begin)))
				return
			default:
				t.Fatal(fields[1])
			}
			if ok {
				result = "ok:some:" + hex.EncodeToString([]byte(value))
			} else {
				result = "ok:none"
			}
		}()
		if result != fields[4] || hook != fields[5] {
			t.Fatalf("line%d %v\nGo: %s %s\nRust: %s %s", count, fields[:4], result, hook, fields[4], fields[5])
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", hash.Sum(nil)) != manifest.RawSHA || count != manifest.Cases || panics != manifest.Panics {
		t.Fatal("original corpus count/checksum mismatch", count, panics)
	}
}

func TestCheckedAndBorrowedSlice(t *testing.T) {
	for _, text := range []string{"", "ascii", "aé🦀z", "\x00"} {
		value, ok := Checked(text)
		if !ok || value.String() != text {
			t.Fatal("valid Rust str rejected")
		}
	}
	for _, text := range []string{"\xff", "\xc0\x80", "\xed\xa0\x80", "\xf4\x90\x80\x80", "\xe2\x82"} {
		if _, ok := Checked(text); ok {
			t.Fatal("invalid UTF8 accepted")
		}
	}
	text := strings.Clone("aé🦀z")
	value := BorrowValid(text).Slice(1, 7, rustfmt.Context{}, "unused")
	if value.String() != "é🦀" || unsafe.StringData(value.String()) != (*byte)(unsafe.Add(unsafe.Pointer(unsafe.StringData(text)), 1)) {
		t.Fatal("successful slice must borrow original backing bytes")
	}
	var location *rustfmt.SourceLocation
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, _ rustfmt.Context) { location = info.Location }})
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("invalid slice did not panic")
			}
		}()
		Slice("é", 1, 2, ctx, "absent")
	}()
	if location != nil {
		t.Fatal("default context invented a Rust source location")
	}
}

func FuzzBoundariesAndSlices(f *testing.F) {
	for _, text := range []string{"", "ascii", "aé🦀z", "\x00\n\u0301"} {
		f.Add(text, uint64(0), uint64(1))
	}
	f.Fuzz(func(t *testing.T, text string, rawBegin, rawEnd uint64) {
		if !utf8.ValidString(text) {
			return
		}
		n := uint64(len(text))
		begin, end := uint(rawBegin%(n+2)), uint(rawEnd%(n+2))
		boundary := func(index uint) bool {
			if index > uint(len(text)) {
				return false
			}
			return index == uint(len(text)) || utf8.ValidString(text[:index])
		}
		if IsCharBoundary(text, begin) != boundary(begin) {
			t.Fatal("boundary predicate")
		}
		floor, ceil := begin, begin
		if floor > uint(len(text)) {
			floor = uint(len(text))
		}
		if ceil > uint(len(text)) {
			ceil = uint(len(text))
		}
		for !boundary(floor) {
			floor--
		}
		for !boundary(ceil) {
			ceil++
		}
		if FloorCharBoundary(text, begin) != floor || CeilCharBoundary(text, begin) != ceil {
			t.Fatal("nearest boundaries")
		}
		value, ok := Get(text, begin, end)
		valid := begin <= end && boundary(begin) && boundary(end)
		if ok != valid || ok && value != text[begin:end] {
			t.Fatal("checked range")
		}
	})
}
