// SPDX-License-Identifier: MIT OR Apache-2.0
// Real Unix std Path/OsStr native observations; borrowed-cell identity retained.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

func sourcePathAlias(address unsafe.Pointer, length int, base []byte) string {
	if length == 0 {
		return "empty"
	}
	lo := uintptr(unsafe.Pointer(unsafe.SliceData(base)))
	p := uintptr(address)
	if p >= lo && p-lo <= uintptr(len(base)) && uintptr(length) <= uintptr(len(base))-(p-lo) {
		return fmt.Sprint(p - lo)
	}
	return "other"
}

type sourcePathHasher struct {
	events []string
	base   []byte
}

func (h *sourcePathHasher) Finish() uint64   { return 0 }
func (h *sourcePathHasher) Finish64() uint64 { return 0 }
func (h *sourcePathHasher) Write(b []byte) {
	h.events = append(h.events, fmt.Sprintf("bytes:%x:%s", b, sourcePathAlias(unsafe.Pointer(unsafe.SliceData(b)), len(b), h.base)))
}
func (h *sourcePathHasher) WriteUsize(v uint) {
	h.events = append(h.events, fmt.Sprintf("usize:%d", v))
}
func (h *sourcePathHasher) WriteIsize(v int) { h.events = append(h.events, fmt.Sprintf("isize:%d", v)) }
func (h *sourcePathHasher) WriteU8(v uint8)  { h.events = append(h.events, fmt.Sprintf("u8:%d", v)) }
func (h *sourcePathHasher) WriteLengthPrefix(v uint) {
	h.events = append(h.events, fmt.Sprintf("length:%d", v))
}

type sourcePathSink struct {
	events      []string
	output      strings.Builder
	count, fail int
	base        []byte
}

func (s *sourcePathSink) Write(b []byte) (int, error) { return s.WriteString(string(b)) }
func (s *sourcePathSink) WriteString(value string) (int, error) {
	s.count++
	s.events = append(s.events, fmt.Sprintf("S:%x:%s", value, sourcePathAlias(unsafe.Pointer(unsafe.StringData(value)), len(value), s.base)))
	if s.count == s.fail {
		return 0, fmt.Errorf("sink failure")
	}
	return s.output.WriteString(value)
}
func (s *sourcePathSink) WriteChar(value rune) error {
	s.count++
	s.events = append(s.events, fmt.Sprintf("C:%x", value))
	if s.count == s.fail {
		return fmt.Errorf("sink failure")
	}
	_, err := s.output.WriteRune(value)
	return err
}
func sourcePathComponent(c RustPathComponent) string {
	return fmt.Sprintf("%d:%x", c.Kind, c.Path.EncodedString())
}
func sourcePathOracleRun(p []string) string {
	pb, err := hex.DecodeString(p[2])
	if err != nil {
		panic(err)
	}
	qb, err := hex.DecodeString(p[3])
	if err != nil {
		panic(err)
	}
	return sourcePathOracleRunViews(p, BorrowedRustPathFromUnixBytes(pb), BorrowedRustPathFromUnixBytes(qb))
}
func sourcePathOracleRunViews(p []string, path, q RustPath) string {
	pb, qb := path.EncodedBytes(), q.EncodedBytes()
	switch p[1] {
	case "view":
		name := "none"
		if n, ok := path.FileName().Get(); ok {
			name = hex.EncodeToString(n.EncodedBytes())
		}
		parts := []string{}
		c := path.Components()
		for {
			v, ok := c.Next()
			if !ok {
				break
			}
			parts = append(parts, sourcePathComponent(v))
		}
		return fmt.Sprintf("%t:%t:%s:%s", path.HasRoot(), path.IsAbsolute(), name, strings.Join(parts, ","))
	case "iter":
		c := path.Components()
		out := []string{}
		for _, step := range p[4] {
			switch step {
			case 'F':
				v, ok := c.Next()
				if !ok {
					out = append(out, "none")
				} else {
					out = append(out, sourcePathComponent(v))
				}
			case 'B':
				v, ok := c.NextBack()
				if !ok {
					out = append(out, "none")
				} else {
					out = append(out, sourcePathComponent(v))
				}
			case 'A':
				out = append(out, fmt.Sprintf("path:%x", c.AsPath().EncodedString()))
			case 'C':
				clone := c.Clone()
				parts := []string{}
				for {
					v, ok := clone.Next()
					if !ok {
						break
					}
					parts = append(parts, sourcePathComponent(v))
				}
				out = append(out, "clone:"+strings.Join(parts, ","))
			default:
				panic("iterator op")
			}
		}
		return strings.Join(out, "|")
	case "hash", "oshash":
		h := &sourcePathHasher{base: pb}
		if p[1] == "hash" {
			path.Hash(h)
		} else {
			hashRustOsStr(path, h)
		}
		return strings.Join(h.events, "|")
	case "lossy":
		v := path.ToStringLossy()
		return fmt.Sprintf("%t:%x", v.IsOwned(), v.String())
	case "compare":
		n := path.Compare(q)
		if n < 0 {
			return "Less"
		}
		if n > 0 {
			return "Greater"
		}
		return "Equal"
	case "strip":
		value, ok := path.StripPrefix(q)
		if !ok {
			return "none"
		}
		return hex.EncodeToString(value.EncodedBytes())
	case "join":
		value := path.Join(q)
		raw := value.EncodedString()
		pointer := unsafe.Pointer(unsafe.StringData(raw))
		return fmt.Sprintf("%x:%s:%s", raw, sourcePathAlias(pointer, len(raw), pb), sourcePathAlias(pointer, len(raw), qb))
	case "clone":
		value := path.Clone()
		raw := value.EncodedString()
		return fmt.Sprintf("%x:%s", raw, sourcePathAlias(unsafe.Pointer(unsafe.StringData(raw)), len(raw), pb))
	case "debug":
		style, _ := strconv.Atoi(p[5])
		fail, _ := strconv.Atoi(p[6])
		sink := &sourcePathSink{base: pb, fail: fail}
		f := &rustfmt.Formatter{Writer: sink, Options: sourceMapFormatOptions(style)}
		err := path.WriteDebugFormatter(f)
		return fmt.Sprintf("%t:%x:%s", err == nil, sink.output.String(), strings.Join(sink.events, "|"))
	default:
		panic("path oracle operation")
	}
}
func TestSourceMapUnixPathNativeOracle(t *testing.T) {
	f, err := os.Open("testdata/source_map_path_unix.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scan := bufio.NewScanner(z)
	scan.Buffer(make([]byte, 65536), 4<<20)
	n := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 8 {
			t.Fatal("invalid path native fixture")
		}
		got := sourcePathOracleRun(p)
		want := sourceMapOracleString(p[7])
		if got != want {
			t.Fatalf("id%s op%s p%s q%s script%s style%s fail%s got%q want%q", p[0], p[1], p[2], p[3], p[4], p[5], p[6], got, want)
		}
		n++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d original Unix std Path/OsStr observations", n)
}

// The retained expected bytes were observed from actual Windows-target Rust
// and Go PE binaries under Wine on Linux. Replaying the portable Windows
// grammar here is useful on every host; it grants no native Windows I/O claim.
func TestSourceMapWindowsEmulatedPathOracle(t *testing.T) {
	f, err := os.Open("testdata/source_map_path_windows_wine.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	wide := func(text string) RustPath {
		b, err := hex.DecodeString(text)
		if err != nil || len(b)%2 != 0 {
			t.Fatal("invalid Windows UTF16 fixture")
		}
		w := make([]uint16, len(b)/2)
		for i := range w {
			w[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
		}
		return RustPathFromUTF16(w)
	}
	scan := bufio.NewScanner(z)
	scan.Buffer(make([]byte, 65536), 4<<20)
	count := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 8 {
			t.Fatal("invalid Windows path fixture")
		}
		got := sourcePathOracleRunViews(p, wide(p[2]), wide(p[3]))
		want := sourceMapOracleString(p[7])
		if got != want {
			t.Fatalf("id%s op%s p%s q%s script%s style%s fail%s got%q want%q", p[0], p[1], p[2], p[3], p[4], p[5], p[6], got, want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d Windows Path/OsStr emulated observations", count)
}
