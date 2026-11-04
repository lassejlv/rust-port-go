// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"strconv"
	"strings"
	"testing"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type svhFormatSpec struct {
	Debug   bool
	Options rustfmt.Options
}
type svhOracleManifest struct {
	Records int             `json:"records"`
	SHA     string          `json:"fixture_sha256"`
	Formats []svhFormatSpec `json:"formats"`
}
type svhWriter struct {
	fail, calls int
	text        strings.Builder
	events      []string
}

func (w *svhWriter) record(kind, text string) error {
	w.events = append(w.events, kind+":"+hex.EncodeToString([]byte(text)))
	n := w.calls
	w.calls++
	if n == w.fail {
		return errors.New("injected fmt::Error")
	}
	w.text.WriteString(text)
	return nil
}
func (w *svhWriter) WriteString(text string) (int, error) {
	if err := w.record("S", text); err != nil {
		return 0, err
	}
	return len(text), nil
}
func (w *svhWriter) Write(bytes []byte) (int, error) { return w.WriteString(string(bytes)) }

type svhCharWriter struct{ *svhWriter }

func (w *svhCharWriter) WriteChar(c rune) error { return w.record("C", string(c)) }

type svhRawHasher struct {
	value  uint64
	events []string
}

func (h *svhRawHasher) Write(bytes []byte) {
	h.events = append(h.events, "B:"+hex.EncodeToString(bytes))
	for _, b := range bytes {
		h.value = bits.RotateLeft64(h.value, 5) ^ uint64(b)
	}
}
func (h *svhRawHasher) Finish64() uint64 { return h.value }

type svhTypedHasher struct{ svhRawHasher }

func (h *svhTypedHasher) WriteU64(n uint64) {
	h.events = append(h.events, fmt.Sprintf("U:%016x", n))
	h.value = bits.RotateLeft64(h.value, 7) ^ n
}

type svhWrappedUnhasher struct{ *Unhasher }

func svhDictionaryHash[H any](values []Svh, cell *H, write func(*H, []byte), typed func(*H, uint64), finish func(*H) uint64) {
	ops := stablehash.ExtendedHasherOps[H, uint64]{
		Write: func(ref stablehash.Ref[H], bytes []byte) {
			if ref.Value != cell {
				panic("copied hasher state")
			}
			write(ref.Value, bytes)
		},
		Finish64: func(ref stablehash.Ref[H]) uint64 { return finish(ref.Value) },
	}
	if typed != nil {
		ops.WriteU64 = func(ref stablehash.Ref[H], n uint64) {
			if ref.Value != cell {
				panic("copied hasher state")
			}
			typed(ref.Value, n)
		}
	}
	SvhHashSliceWithHasherRef(values, ops.Bind(stablehash.Ref[H]{Value: cell}))
}

type svhEncoder struct{ data []byte }

func (e *svhEncoder) EmitRawBytes(b []byte) { e.data = append(e.data, b...) }

type svhDecoder struct{ data []byte }

func (d *svhDecoder) ReadRawBytes(n int) []byte { out := d.data[:n]; d.data = d.data[n:]; return out }

func svhReplay(request string, formats []svhFormatSpec) string {
	p := strings.Split(request, "\t")
	lo, err := strconv.ParseUint(p[1], 16, 64)
	if err != nil {
		panic(err)
	}
	hi, err := strconv.ParseUint(p[2], 16, 64)
	if err != nil {
		panic(err)
	}
	fp := FingerprintFromU64(lo, hi)
	value := NewSvh(fp)
	switch p[0] {
	case "V":
		other := NewSvh(FingerprintFromU64(hi, lo))
		copied := value.Clone()
		copied.CloneFrom(other)
		e := &svhEncoder{}
		value.Encode(e)
		d := &svhDecoder{data: e.data}
		decoded := DecodeSvh(d)
		h := NewStableHasher()
		h.Write([]byte("seeded-svh"))
		value.StableHash(nil, h)
		digest := h.FinishHash()
		stable := FingerprintFromU64(digest[0], digest[1])
		n := value.AsU128()
		f := value.AsFingerprint().asU128()
		return fmt.Sprintf("%016x%016x\t%s\t%016x%016x\t%t\t%t\t%t\t%t\t%s\t%s", n.Hi, n.Lo, value.ToHex(), f.Hi, f.Lo, value.Equal(other), value.NotEqual(other), copied.Equal(other), decoded.Equal(value), hex.EncodeToString(e.data), stable.ToHex())
	case "H":
		n, _ := strconv.Atoi(p[4])
		values := []Svh{value, NewSvh(FingerprintFromU64(hi, lo))}
		if p[3] == "raw" {
			h := &svhRawHasher{value: 7}
			SvhHashSlice(values[:n], h)
			dictionary := &svhRawHasher{value: 7}
			svhDictionaryHash(values[:n], dictionary, (*svhRawHasher).Write, nil, (*svhRawHasher).Finish64)
			if strings.Join(h.events, "|") != strings.Join(dictionary.events, "|") || h.value != dictionary.value {
				panic("raw Hasher default dictionary differs")
			}
			return fmt.Sprintf("%s\t%016x", strings.Join(h.events, "|"), h.Finish64())
		}
		h := &svhTypedHasher{svhRawHasher: svhRawHasher{value: 7}}
		SvhHashSlice(values[:n], h)
		dictionary := &svhTypedHasher{svhRawHasher: svhRawHasher{value: 7}}
		svhDictionaryHash(values[:n], dictionary, (*svhTypedHasher).Write, (*svhTypedHasher).WriteU64, (*svhTypedHasher).Finish64)
		if strings.Join(h.events, "|") != strings.Join(dictionary.events, "|") || h.value != dictionary.value {
			panic("typed Hasher dictionary differs")
		}
		return fmt.Sprintf("%s\t%016x", strings.Join(h.events, "|"), h.Finish64())
	case "U":
		seed, _ := strconv.ParseUint(p[4], 10, 64)
		n, _ := strconv.Atoi(p[5])
		debug := p[6] == "1"
		h := NewUnhasherWithDebugAssertions(debug)
		h.WriteU64(seed)
		values := []Svh{value, NewSvh(FingerprintFromU64(hi, lo))}
		ok := true
		func() {
			defer func() {
				if recover() != nil {
					ok = false
				}
			}()
			if p[3] == "direct" {
				SvhHashSlice(values[:n], h)
			} else {
				SvhHashSlice(values[:n], &svhWrappedUnhasher{h})
			}
		}()
		dictionary := NewUnhasherWithDebugAssertions(debug)
		dictionary.WriteU64(seed)
		dictionaryOK := true
		func() {
			defer func() {
				if recover() != nil {
					dictionaryOK = false
				}
			}()
			if p[3] == "direct" {
				svhDictionaryHash(values[:n], dictionary, func(h *Unhasher, b []byte) { h.Write(b) }, func(h *Unhasher, n uint64) { h.WriteU64(n) }, func(h *Unhasher) uint64 { return h.Finish64() })
			} else if p[3] == "reference" {
				pointer := dictionary
				svhDictionaryHash(values[:n], &pointer, func(h **Unhasher, b []byte) { (*h).Write(b) }, func(h **Unhasher, n uint64) { (*h).WriteU64(n) }, func(h **Unhasher) uint64 { return (*h).Finish64() })
			} else {
				wrapped := &svhWrappedUnhasher{dictionary}
				svhDictionaryHash(values[:n], wrapped, (*svhWrappedUnhasher).Write, (*svhWrappedUnhasher).WriteU64, (*svhWrappedUnhasher).Finish64)
			}
		}()
		if ok != dictionaryOK || h.Finish64() != dictionary.Finish64() {
			panic("concrete dictionary specialization differs")
		}
		return fmt.Sprintf("%t\t%016x", ok, h.Finish64())
	case "F":
		index, _ := strconv.Atoi(p[4])
		fail, _ := strconv.Atoi(p[5])
		spec := formats[index]
		writer := &svhWriter{fail: fail}
		f := &rustfmt.Formatter{Writer: writer, Options: spec.Options}
		if p[6] == "1" {
			f.Writer = &svhCharWriter{writer}
		}
		format := func() error {
			if p[3] == "F" {
				return fp.WriteDebugFormatter(f)
			}
			if spec.Debug {
				return value.WriteDebugFormatter(f)
			}
			return value.WriteDisplayFormatter(f)
		}
		first := format()
		after := []string{fmt.Sprintf("%t:%d:%d", first == nil, f.Options.Fill, f.Options.Align)}
		ok := first == nil
		if p[7] == "1" {
			second := format()
			after = append(after, fmt.Sprintf("%t:%d:%d", second == nil, f.Options.Fill, f.Options.Align))
			ok = ok && second == nil
		}
		return fmt.Sprintf("%t\t%s\t%s\t%s", ok, hex.EncodeToString([]byte(writer.text.String())), strings.Join(writer.events, "|"), strings.Join(after, "|"))
	default:
		panic("unknown Svh operation")
	}
}

func TestOriginalSvh(t *testing.T) {
	data, err := os.ReadFile("../../../../../tools/port-rust-svh/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest svhOracleManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	compressed, err := os.ReadFile("testdata/svh.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != manifest.SHA {
		t.Fatal("native fixture checksum differs")
	}
	file, err := os.Open("testdata/svh.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	r, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 8192), 1<<20)
	count := 0
	for scanner.Scan() {
		p := strings.SplitN(scanner.Text(), "\t|\t", 2)
		if len(p) != 2 {
			t.Fatal("native record schema")
		}
		if got := svhReplay(p[0], manifest.Formats); got != p[1] {
			t.Fatalf("Svh native row%d %s\ngot %s\nwant %s", count, p[0], got, p[1])
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != manifest.Records {
		t.Fatal("native record count differs")
	}
	t.Logf("%d unchanged-source Svh records match", count)
}
