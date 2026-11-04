// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

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
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func unordTraitConfigs() (UnordSetConfig[uint64], UnordMapConfig[uint64, uint64]) {
	keys := unordTestKeys()
	keys.Debug = func(v *uint64, f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(*v, 64, f) }
	return UnordSetConfig[uint64]{Layout: rusthashmap.Layout{Size: 8, Align: 8}, Profile: rusthashmap.DefaultProfile(), Values: keys}, UnordMapConfig[uint64, uint64]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: keys, Values: rusthashmap.ValueOps[uint64]{TrivialClone: true, Equal: func(a, b *uint64, _ rusthashmap.Context) bool { return *a == *b }, Debug: keys.Debug}}
}
func unordTraitCollections(length, order int) (*UnordSet[uint64], *UnordMap[uint64, uint64]) {
	values := make([]uint64, length)
	for i := range values {
		values[i] = uint64(i*17 + 3)
	}
	if order == 1 {
		for i, j := 0, length-1; i < j; i, j = i+1, j-1 {
			values[i], values[j] = values[j], values[i]
		}
	} else if order == 2 {
		values = append(append([]uint64{}, values[length/2:]...), values[:length/2]...)
	}
	pairs := make([]rusthashmap.Pair[uint64, uint64], length)
	for i, value := range values {
		pairs[i] = rusthashmap.Pair[uint64, uint64]{Key: value, Value: value + 1000}
	}
	setConfig, mapConfig := unordTraitConfigs()
	set := UnordSetFromIterator(rustvec.NewVecIntoIter(values, rustvec.ElementOps[uint64]{}).IntoIterator(), setConfig)
	m := UnordMapFromIterator(rustvec.NewVecIntoIter(pairs, rustvec.ElementOps[rusthashmap.Pair[uint64, uint64]]{}).IntoIterator(), mapConfig)
	return set, m
}

var unordWriterError = errors.New("original Unord fmt writer error")

type unordTraitSink struct {
	calls []string
	fail  int
}

func (s *unordTraitSink) record(kind, value string) error {
	index := len(s.calls)
	s.calls = append(s.calls, kind+hex.EncodeToString([]byte(value)))
	if index == s.fail {
		return unordWriterError
	}
	return nil
}
func (s *unordTraitSink) Write(value []byte) (int, error) { return s.WriteString(string(value)) }
func (s *unordTraitSink) WriteString(value string) (int, error) {
	if err := s.record("S", value); err != nil {
		return 0, err
	}
	return len(value), nil
}
func (s *unordTraitSink) WriteChar(value rune) error { return s.record("C", string(value)) }
func unordTraitOptions(pattern int) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	switch pattern {
	case 0:
	case 1:
		o.Alternate = true
	case 2:
		o.Width, o.HasWidth, o.ZeroPad = 8, true, true
	case 3:
		o.Width, o.HasWidth, o.ZeroPad, o.SignPlus, o.DebugLowerHex = 10, true, true, true, true
	case 4:
		o.Fill, o.Align, o.Width, o.HasWidth, o.Precision, o.HasPrecision = '💫', rustfmt.AlignRight, 12, true, 3, true
	case 5:
		o.Width, o.HasWidth, o.ZeroPad, o.Alternate, o.DebugUpperHex = 10, true, true, true, true
	default:
		panic("bad Unord formatter pattern")
	}
	return o
}
func unordTraitAfter(f *rustfmt.Formatter) string {
	align := [...]string{"None", "Some(Left)", "Some(Right)", "Some(Center)"}[f.Options.Align]
	return fmt.Sprintf("%x,%s", f.Options.Fill, align)
}

func TestOriginalUnordTraits(t *testing.T) {
	root := "../../../../../tools/port-rust-unord/traits/"
	b, err := os.ReadFile(root + "manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases     int               `json:"cases"`
		Raw       string            `json:"raw_sha256"`
		Fixture   string            `json:"fixture_sha256"`
		Oracle    string            `json:"oracle_sha256"`
		Generator string            `json:"generator_sha256"`
		Reference map[string]string `json:"reference_tools"`
	}
	if err := json.Unmarshal(b, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { hash := sha256.Sum256(b); return hex.EncodeToString(hash[:]) }
	bindings := map[string]string{root + "oracle.rs": manifest.Oracle, root + "generate.py": manifest.Generator}
	for name, hash := range manifest.Reference {
		bindings["../../../../../tools/port-rust-span-reference/"+name] = hash
	}
	for path, want := range bindings {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("Unord original proof binding differs: %s: %v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/unord_traits.tsv.gz")
	if err != nil || digest(compressed) != manifest.Fixture || manifest.Cases != 5662 {
		t.Fatalf("Unord trait fixture/coverage differs: %v", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 32<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("Unord trait raw fixture differs: %v", err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	var set *UnordSet[uint64]
	var seed, state uint64
	setConfig, _ := unordTraitConfigs()
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count := 0
	for scanner.Scan() {
		want := scanner.Text()
		fields := strings.Split(want, "\t")
		number := func(index int) int {
			v, err := strconv.Atoi(fields[index])
			if err != nil {
				t.Fatal(err)
			}
			return v
		}
		var got string
		switch fields[0] {
		case "S":
			if uint64(number(1)) != seed {
				if set != nil {
					set.Close()
				}
				seed = uint64(number(1))
				state = seed
				set = NewUnordSet(setConfig)
			}
			step := number(2)
			bits := unordTestRandom(&state)
			value := bits >> 8 % 31
			result := ""
			switch bits % 7 {
			case 0, 1:
				result = strconv.FormatBool(set.Insert(value))
			case 2:
				result = strconv.FormatBool(set.Remove(&value))
			case 3:
				result = strconv.FormatBool(set.Contains(&value))
			case 4:
				result = "None"
				if value, ok := set.GetOnly(); ok {
					result = fmt.Sprintf("Some(%d)", *value)
				}
			case 5:
				clone := set.Clone()
				set.CloneFrom(clone)
				result = fmt.Sprintf("%t:%t", set.Equal(clone), set.NotEqual(clone))
				clone.Close()
			case 6:
				if step%19 == 0 {
					set.Clear()
				}
				result = "clear"
			}
			got = fmt.Sprintf("S\t%d\t%d\t%s\t%d\t%t\t%s", seed, step, result, set.Len(), set.IsEmpty(), hex.EncodeToString([]byte(set.RustDebug())))
		case "H":
			kind, length, order, fail := number(1), number(2), number(3), number(4)
			s, m := unordTraitCollections(length, order)
			hasher := NewStableHasher()
			hasher.WriteU64(0xfeed0123456789ab)
			var events []string
			operation := func(value *uint64, _ StableHashCtxt, h *StableHasher, c rusthashmap.Context) {
				index := len(events)
				events = append(events, strconv.FormatUint(*value, 10))
				h.WriteU64(*value ^ 0xc0ffee1234567890)
				if index == fail {
					c.Panic("tracked_hash")
				}
				h.WriteU8(uint8(*value))
			}
			panicked := false
			func() {
				defer func() { panicked = recover() != nil }()
				if kind == 0 {
					s.StableHash(nil, hasher, operation)
				} else {
					m.StableHash(nil, hasher, operation, operation)
				}
			}()
			hash := hasher.FinishHash()
			got = fmt.Sprintf("H\t%d\t%d\t%d\t%d\t%t\t%d\t%d\t%s", kind, length, order, fail, panicked, hash[0], hash[1], strings.Join(events, ","))
			s.Close()
			m.Close()
		case "D":
			kind, length, pattern, fail := number(1), number(2), number(3), number(4)
			s, m := unordTraitCollections(length, 0)
			for index := 0; index < length; index++ {
				key := uint64(index*17 + 3)
				value, _ := m.GetMut(&key)
				*value = uint64(index + 1000)
			}
			sink := &unordTraitSink{fail: fail}
			f := &rustfmt.Formatter{Options: unordTraitOptions(pattern), Writer: sink}
			var err error
			if kind == 0 {
				err = s.WriteDebugFormatter(f)
			} else {
				err = m.WriteDebugFormatter(f)
			}
			if err != nil && err != unordWriterError {
				t.Fatal("Unord changed writer error identity")
			}
			got = fmt.Sprintf("D\t%d\t%d\t%d\t%d\t%t\t%s\t%s", kind, length, pattern, fail, err != nil, unordTraitAfter(f), strings.Join(sink.calls, ","))
			s.Close()
			m.Close()
		default:
			t.Fatal("unknown original Unord trait record")
		}
		if got != want {
			t.Fatalf("original Unord trait record %d differs\n got: %s\nwant: %s", count, got, want)
		}
		count++
	}
	if set != nil {
		set.Close()
	}
	if err := scanner.Err(); err != nil || count != manifest.Cases {
		t.Fatalf("Unord trait coverage differs: %d: %v", count, err)
	}
}
