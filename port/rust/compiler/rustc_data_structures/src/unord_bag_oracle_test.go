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
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

type bagProbe struct{ value, id uint64 }

func bagProbeCore(kind, length int) (string, string) {
	var events []string
	ops := UnordBagValueOps[bagProbe]{Clone: func(p *bagProbe, _ rustfmt.Context) bagProbe {
		events = append(events, fmt.Sprintf("clone:%d", p.id))
		return bagProbe{p.value, p.id + 100}
	}, Drop: func(p *bagProbe, c rustfmt.Context) {
		events = append(events, fmt.Sprintf("drop:%d:%t", p.id, c.Panicking()))
	}, Equal: func(a, b *bagProbe, _ rustfmt.Context) bool {
		events = append(events, fmt.Sprintf("eq:%d:%d", a.id, b.id))
		return a.value == b.value
	}, NotEqual: func(a, b *bagProbe, _ rustfmt.Context) bool {
		events = append(events, fmt.Sprintf("ne:%d:%d", a.id, b.id))
		return a.value != b.value
	}}
	config := UnordBagConfig[bagProbe]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rustcindex.IndexStorageConfig{OverflowChecks: true, DebugAssertions: true}, Values: ops}
	makeBag := func(n int, offset uint64) *UnordBag[bagProbe] {
		b := NewUnordBag(config)
		for i := 0; i < n; i++ {
			b.Push(bagProbe{uint64(i*17 + 3), uint64(i) + offset})
		}
		return b
	}
	result := ""
	switch kind {
	case 0:
		b := makeBag(length, 0)
		clone := b.Clone()
		result = fmt.Sprintf("%t:%t", b.Equal(clone), b.NotEqual(clone))
		clone.Close()
		b.Close()
	case 1:
		source, destination := makeBag(length, 0), makeBag(1, 999)
		destination.CloneFrom(source)
		result = fmt.Sprintf("%t:%t", destination.Equal(source), destination.NotEqual(source))
		destination.Close()
		source.Close()
	case 2:
		item, ok := makeBag(length, 0).IntoItems().GetOnly()
		result = "none"
		if ok {
			result = strconv.FormatUint(item.Borrow().id, 10)
			item.Drop()
		}
	case 3:
		b := makeBag(length, 0)
		result = strconv.FormatUint(uint64(b.Items().Count()), 10)
		b.Close()
	case 4:
		a, b := makeBag(length, 0), makeBag(length+1, 100)
		result = fmt.Sprintf("%t:%t", a.Equal(b), a.NotEqual(b))
		a.Close()
		b.Close()
	case 5:
		input := rustcindex.IndexVecWithCapacityAndConfig(uint(length+12), rustcindex.UsizeIdx(), config.indexOps(), config.Profile)
		for i := 0; i < length; i++ {
			input.Push(bagProbe{uint64(i*17 + 3), uint64(i)})
		}
		b := UnordBagFromIterator(input.IntoIter().IntoIterator(), config)
		result = strconv.FormatUint(uint64(b.Len()), 10)
		b.Close()
	case 6:
		b := DefaultUnordBag(config)
		result = strconv.FormatUint(uint64(b.Len()), 10)
		b.Close()
	case 7:
		b := makeBag(length, 0)
		item, ok := b.Items().GetOnly()
		result = "none"
		if ok {
			result = strconv.FormatUint(item.id, 10)
		}
		b.Close()
	case 8:
		result = strconv.FormatUint(uint64(makeBag(length, 0).IntoItems().Count()), 10)
	default:
		panic("invalid source Bag core kind")
	}
	return result, strings.Join(events, "\n")
}
func bagNumbers(length, order int) *UnordBag[uint64] {
	config := UnordBagConfig[uint64]{Layout: rusthashmap.Layout{Size: 8, Align: 8}, Profile: rustcindex.IndexStorageConfig{OverflowChecks: true, DebugAssertions: true}, Values: UnordBagValueOps[uint64]{TrivialClone: true, Equal: func(a, b *uint64, _ rustfmt.Context) bool { return *a == *b }, Debug: func(value *uint64, f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(*value, 64, f) }}}
	values := make([]uint64, length)
	for i := range values {
		values[i] = uint64(i*17 + 3)
	}
	if order == 1 {
		for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
			values[i], values[j] = values[j], values[i]
		}
	} else if order == 2 {
		values = append(append([]uint64{}, values[length/2:]...), values[:length/2]...)
	}
	b := NewUnordBag(config)
	for _, value := range values {
		b.Push(value)
	}
	return b
}
func TestOriginalUnordBag(t *testing.T) {
	b, e := os.ReadFile("../../../../../tools/port-rust-unord/bag/manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m struct {
		Cases   int
		Counts  map[string]int
		Raw     string `json:"raw_sha256"`
		Fixture string `json:"fixture_sha256"`
		Files   map[string]string
	}
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	digest := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	for p, h := range m.Files {
		b, e := os.ReadFile("../../../../../" + p)
		if e != nil || digest(b) != h {
			t.Fatalf("Bag binding changed %s: %v", p, e)
		}
	}
	fixture, e := os.ReadFile("testdata/unord_bag.tsv.gz")
	if e != nil || digest(fixture) != m.Fixture {
		t.Fatal("Bag fixture digest", e)
	}
	z, e := gzip.NewReader(bytes.NewReader(fixture))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := io.ReadAll(io.LimitReader(z, 8<<20))
	if e != nil {
		t.Fatal(e)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if digest(raw) != m.Raw {
		t.Fatal("Bag raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	count := 0
	counts := map[string]int{}
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
		case "C":
			kind, length := integer(1), integer(2)
			result, events := bagProbeCore(kind, length)
			got = fmt.Sprintf("C\t%d\t%d\t%s\t%s", kind, length, result, hex.EncodeToString([]byte(events)))
		case "H":
			length, order, fail := integer(1), integer(2), integer(3)
			bag := bagNumbers(length, order)
			hasher := NewStableHasher()
			hasher.WriteU64(0xfeed0123456789ab)
			var events []string
			panicked := false
			func() {
				defer func() { panicked = recover() != nil }()
				bag.StableHash(nil, hasher, func(value *uint64, _ StableHashCtxt, target *StableHasher, c rustfmt.Context) {
					index := len(events)
					events = append(events, strconv.FormatUint(*value, 10))
					target.WriteU64(*value ^ 0xc0ffee1234567890)
					if index == fail {
						c.Panic("tracked_hash")
					}
					target.WriteU8(uint8(*value))
				})
			}()
			hash := hasher.FinishHash()
			got = fmt.Sprintf("H\t%d\t%d\t%d\t%t\t%d\t%d\t%s", length, order, fail, panicked, hash[0], hash[1], strings.Join(events, ","))
			bag.Close()
		case "D":
			length, pattern, fail := integer(1), integer(2), integer(3)
			bag := bagNumbers(length, 0)
			sink := &unordTraitSink{fail: fail}
			formatter := &rustfmt.Formatter{Options: unordTraitOptions(pattern), Writer: sink}
			err := bag.WriteDebugFormatter(formatter)
			if err != nil && err != unordWriterError {
				t.Fatal("Bag writer error identity")
			}
			got = fmt.Sprintf("D\t%d\t%d\t%d\t%t\t%s\t%s", length, pattern, fail, err != nil, unordTraitAfter(formatter), strings.Join(sink.calls, ","))
			bag.Close()
		default:
			t.Fatal("unknown Bag source record")
		}
		if got != want {
			t.Fatalf("Bag record %d\nGOT %s\nWANT %s", count, got, want)
		}
		counts[f[0]]++
		count++
	}
	if scanner.Err() != nil || count != 1260 || m.Cases != 1260 || counts["C"] != 36 || counts["H"] != 375 || counts["D"] != 849 {
		t.Fatal("Bag coverage", count, counts, scanner.Err())
	}
}
