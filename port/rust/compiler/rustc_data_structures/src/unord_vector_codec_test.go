// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

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

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

type vectorCodecSink struct {
	rustcserialize.Encoder
	events     *[]string
	next, fail int
}

func (s *vectorCodecSink) put(value string) {
	*s.events = append(*s.events, value)
	index := s.next
	s.next++
	if index == s.fail {
		panic("emit")
	}
}
func (s *vectorCodecSink) EmitUsize(value uint)      { s.put(fmt.Sprintf("L:%d", value)) }
func (s *vectorCodecSink) EmitU64(value uint64)      { s.put(fmt.Sprintf("U:%d", value)) }
func (s *vectorCodecSink) EmitU8(value uint8)        { s.put(fmt.Sprintf("B:%d", value)) }
func (s *vectorCodecSink) EmitRawBytes(value []byte) { s.put("R:" + hex.EncodeToString(value)) }
func vectorCodecGeneric(kind, length, fail int) (panicked bool, next int, events string) {
	var trace []string
	sink := &vectorCodecSink{events: &trace, fail: fail}
	profile := rustcindex.IndexStorageConfig{OverflowChecks: true, DebugAssertions: true}
	if kind < 2 {
		config := UnordBagConfig[uint64]{Layout: rusthashmap.Layout{Size: 8, Align: 8}, Profile: profile, Values: UnordBagValueOps[uint64]{Drop: func(value *uint64, c rustfmt.Context) {
			trace = append(trace, fmt.Sprintf("D:%d:%t", *value, c.Panicking()))
		}}}
		ops := rustcserialize.VectorEncodeOps[uint64]{Element: func(value *uint64, e rustcserialize.Encoder, _ rustfmt.Context) {
			trace = append(trace, fmt.Sprintf("E:%d", *value))
			e.EmitU64(*value*17 + 3)
		}}
		if kind == 0 {
			v := rustcindex.NewIndexVecWithConfig(rustcindex.UsizeIdx(), config.indexOps(), profile)
			for i := 0; i < length; i++ {
				v.Push(uint64(i))
			}
			func() {
				defer func() { panicked = recover() != nil }()
				rustcserialize.EncodeVector(sink, rustcserialize.VectorSlice[uint64]{Length: v.Len(), At: v.Raw.At, AsGoSlice: v.Raw.AsGoSlice}, ops)
			}()
			v.Close()
		} else {
			bag := NewUnordBag(config)
			for i := 0; i < length; i++ {
				bag.Push(uint64(i))
			}
			func() { defer func() { panicked = recover() != nil }(); bag.Encode(sink, ops) }()
			bag.Close()
		}
	} else {
		config := UnordByteBagConfig(profile)
		if kind == 2 {
			v := rustcindex.NewIndexVecWithConfig(rustcindex.UsizeIdx(), config.indexOps(), profile)
			for i := 0; i < length; i++ {
				v.Push(uint8(i))
			}
			func() {
				defer func() { panicked = recover() != nil }()
				rustcserialize.EncodeVector(sink, rustcserialize.VectorSlice[uint8]{Length: v.Len(), At: v.Raw.At, AsGoSlice: v.Raw.AsGoSlice}, rustcserialize.ByteVectorEncodeOps(sink))
			}()
			v.Close()
		} else {
			bag := NewUnordBag(config)
			for i := 0; i < length; i++ {
				bag.Push(uint8(i))
			}
			func() { defer func() { panicked = recover() != nil }(); EncodeUnordByteBag(bag, sink) }()
			bag.Close()
		}
	}
	return panicked, sink.next, strings.Join(trace, "\n")
}
func TestOriginalUnordVectorCodec(t *testing.T) {
	b, e := os.ReadFile("../../../../../tools/port-rust-unord/codec/vector_manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m struct {
		Cases    int
		Raw      string `json:"raw_sha256"`
		Fixture  string `json:"fixture_sha256"`
		Files    map[string]string
		Location rustfmt.SourceLocation
	}
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	digest := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	for p, h := range m.Files {
		b, e := os.ReadFile("../../../../../" + p)
		if e != nil || digest(b) != h {
			t.Fatalf("vector codec binding %s: %v", p, e)
		}
	}
	fixture, e := os.ReadFile("testdata/unord_vector_codec.tsv.gz")
	if e != nil || digest(fixture) != m.Fixture {
		t.Fatal("vector codec fixture", e)
	}
	z, e := gzip.NewReader(bytes.NewReader(fixture))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := io.ReadAll(io.LimitReader(z, 4<<20))
	if e != nil {
		t.Fatal(e)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if digest(raw) != m.Raw {
		t.Fatal("vector codec raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	count := 0
	counts := map[string]int{}
	profile := rustcindex.IndexStorageConfig{OverflowChecks: true, DebugAssertions: true}
	for scanner.Scan() {
		want := scanner.Text()
		f := strings.Split(want, "\t")
		integer := func(i int) int {
			v, e := strconv.Atoi(f[i])
			if e != nil {
				t.Fatal(e)
			}
			return v
		}
		got := ""
		switch f[0] {
		case "G":
			kind, length, fail := integer(1), integer(2), integer(3)
			panicked, next, events := vectorCodecGeneric(kind, length, fail)
			got = fmt.Sprintf("G\t%d\t%d\t%d\t%t\t%d\t%s", kind, length, fail, panicked, next, hex.EncodeToString([]byte(events)))
		case "O":
			kind, length, seed := integer(1), integer(2), integer(3)
			values := make([]byte, length)
			for i := range values {
				values[i] = byte((i*73 + seed) % 256)
			}
			encoder := rustcserialize.NewMemEncoder()
			if kind == 0 {
				rustcserialize.EncodeVector(encoder, rustcserialize.VectorSlice[uint8]{Length: uint(length), At: func(i uint) *byte { return &values[i] }, AsGoSlice: func() []byte { return values }}, rustcserialize.ByteVectorEncodeOps(encoder))
			} else {
				bag := NewUnordBag(UnordByteBagConfig(profile))
				for _, value := range values {
					bag.Push(value)
				}
				EncodeUnordByteBag(bag, encoder)
				bag.Close()
			}
			encoded := encoder.Finish()
			capacity := cap(encoded)
			header := len(encoded) - length
			input := make([]byte, len(encoded)+len(rustcserialize.MagicEndBytes))
			copy(input, encoded)
			copy(input[len(encoded):], rustcserialize.MagicEndBytes)
			decoder, e := rustcserialize.NewMemDecoder(input, 0)
			if e != nil {
				t.Fatal(e)
			}
			decoded := DecodeUnordByteBag(decoder, profile)
			before := hex.EncodeToString(decoded.inner.Raw.AsGoSlice())
			decodedCapacity := decoded.inner.Raw.Cap()
			if length > 0 {
				*decoded.inner.Raw.At(0) ^= 255
			}
			independent := length == 0 || input[header] == values[0]
			got = fmt.Sprintf("O\t%d\t%d\t%d\t%s\t%d\t%d\t%d\t%t\t%s", kind, length, seed, hex.EncodeToString(encoded), capacity, decoder.Position(), decodedCapacity, independent, before)
			decoded.Close()
		case "X":
			declared, e := strconv.ParseUint(f[1], 10, 64)
			if e != nil {
				t.Fatal(e)
			}
			available := integer(2)
			header := rustcserialize.NewMemEncoder()
			header.EmitUsize(uint(declared))
			data := header.Finish()
			for i := 0; i < available; i++ {
				data = append(data, byte(i))
			}
			data = append(data, []byte(rustcserialize.MagicEndBytes)...)
			decoder, e := rustcserialize.NewMemDecoder(data, 0)
			if e != nil {
				t.Fatal(e)
			}
			observed := ""
			c := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: map[string]rustfmt.SourceLocation{rustcserialize.MemDecoderExhaustedPanicSite: m.Location}, Hook: func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
				location := rustfmt.SourceLocation{}
				if info.Location != nil {
					location = *info.Location
				}
				observed = fmt.Sprintf("%s:%s:%d:%d:%t:%t:%t", info.Kind, hex.EncodeToString([]byte(location.File)), location.Line, location.Column, c.Panicking(), info.CanUnwind, info.ForceNoBacktrace)
			}})
			failed := false
			func() {
				defer func() { failed = recover() != nil }()
				rustcserialize.WithRuntimeContext(decoder, c, func(d *rustcserialize.MemDecoder) struct{} {
					decoded := DecodeUnordByteBag(d, profile)
					decoded.Close(c)
					return struct{}{}
				})
			}()
			got = fmt.Sprintf("X\t%d\t%d\t%t\t%d\t%s", declared, available, failed, decoder.Position(), hex.EncodeToString([]byte(observed)))
		default:
			t.Fatal("unknown vector codec source record")
		}
		if got != want {
			t.Fatalf("vector codec record %d\nGOT %s\nWANT %s", count, got, want)
		}
		counts[f[0]]++
		count++
	}
	if scanner.Err() != nil || count != 292 || m.Cases != 292 || counts["G"] != 236 || counts["O"] != 36 || counts["X"] != 20 {
		t.Fatal("vector codec coverage", count, counts, scanner.Err())
	}
}
