// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
)

func TestEditionMethodsAndOrdering(t *testing.T) {
	values := AllEditions()
	if len(values) != 5 || DefaultEdition != Edition2015 || LatestStableEdition != Edition2024 {
		t.Fatal("edition constants changed")
	}
	if EditionNameList != "<2015|2018|2021|2024|future>" || EditionNameListStable != "<2015|2018|2021|2024>" {
		t.Fatal("edition name lists changed")
	}
	for index, value := range values {
		name := [...]string{"2015", "2018", "2021", "2024", "future"}[index]
		debug := [...]string{"Edition2015", "Edition2018", "Edition2021", "Edition2024", "EditionFuture"}[index]
		lint := [...]string{"rust_2015_compatibility", "rust_2018_compatibility", "rust_2021_compatibility", "rust_2024_compatibility", "edition_future_compatibility"}[index]
		if value.AsStr() != name || value.String() != name || value.DebugString() != debug || value.LintName() != lint {
			t.Fatalf("edition %d names changed", index)
		}
		parsed, ok := EditionFromStr(name)
		if !ok || parsed != value || value.Clone() != value {
			t.Fatalf("edition %s round trip/copy failed", name)
		}
		copy := EditionFuture
		copy.CloneFrom(value)
		if copy != value {
			t.Fatal("CloneFrom did not replace its receiver")
		}
		if value.IsStable() != (index < 4) || value.IsRust2015() != (index == 0) || value.AtLeastRust2018() != (index >= 1) || value.AtLeastRust2021() != (index >= 2) || value.AtLeastRust2024() != (index >= 3) || value.AtLeastEditionFuture() != (index >= 4) {
			t.Fatalf("edition %s predicates changed", name)
		}
		for otherIndex, other := range values {
			want := 0
			if index < otherIndex {
				want = -1
			} else if index > otherIndex {
				want = 1
			}
			if value.Compare(other) != want || (value == other) != (index == otherIndex) || (value < other) != (index < otherIndex) {
				t.Fatal("derived edition equality/ordering changed")
			}
		}
	}
	values[0] = EditionFuture
	if AllEditions()[0] != Edition2015 {
		t.Fatal("caller changed the immutable edition list")
	}
}

func TestEditionParsingIsExact(t *testing.T) {
	for _, text := range []string{"", "2014", "2016", "2027", "Future", "FUTURE", " 2015", "2015 ", "02015", "+2015", "-2015", "2015\x00", "２０１５", "٢٠١٥", "future\n", "Edition2015", "2015.0"} {
		if value, ok := EditionFromStr(text); ok {
			t.Fatalf("accepted invalid edition %q as %v", text, value)
		}
	}
}

func TestEditionDisplayAndDebugFormatting(t *testing.T) {
	for _, edition := range AllEditions() {
		for _, format := range []string{"%v", "%s", "%20v", "%-20s", "%.1v", "%020.1s", "%+v", "%#s"} {
			if got := fmt.Sprintf(format, edition); got != edition.AsStr() {
				t.Errorf("%q edition = %q; want %q", format, got, edition.AsStr())
			}
		}
		for _, format := range []string{"%#v", "%#20v", "%#.1v", "%+#020.1v"} {
			if got := fmt.Sprintf(format, edition); got != edition.DebugString() {
				t.Errorf("%q edition = %q; want %q", format, got, edition.DebugString())
			}
		}
		var display, debug bytes.Buffer
		if err := edition.DisplayTo(&display); err != nil {
			t.Fatal(err)
		}
		if err := edition.DebugTo(&debug); err != nil {
			t.Fatal(err)
		}
		if display.String() != edition.AsStr() || debug.String() != edition.DebugString() {
			t.Fatal("writer formatting changed")
		}
	}
}

type editionFailingWriter struct{ err error }

func (writer editionFailingWriter) Write([]byte) (int, error) { return 0, writer.err }

type editionShortWriter struct{}

func (editionShortWriter) Write([]byte) (int, error) { return 0, nil }

func TestEditionWriterErrors(t *testing.T) {
	sentinel := errors.New("writer rejected edition")
	for _, write := range []func(io.Writer) error{Edition2024.DisplayTo, Edition2024.DebugTo} {
		if err := write(editionFailingWriter{sentinel}); err != sentinel {
			t.Fatalf("formatter lost writer error: %v", err)
		}
		if err := write(editionShortWriter{}); err != io.ErrShortWrite {
			t.Fatalf("short write error = %v", err)
		}
	}
}

func TestEditionInvalidGoDiscriminantIsRejected(t *testing.T) {
	for _, operation := range []func(){func() { Edition(255).AsStr() }, func() { Edition(5).LintName() }, func() { Edition(6).IsStable() }, func() { Edition(7).Clone() }, func() { Edition2015.Compare(Edition(8)) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid Go-only discriminant was accepted")
				}
			}()
			operation()
		}()
	}
}

type editionOperationTrace struct {
	entries    []string
	panicValue any
}

func (trace *editionOperationTrace) WriteIsize(value int) {
	trace.entries = append(trace.entries, fmt.Sprintf("isize:%d", value))
	if trace.panicValue != nil {
		panic(trace.panicValue)
	}
}
func (trace *editionOperationTrace) WriteU8(value uint8) {
	trace.entries = append(trace.entries, fmt.Sprintf("u8:%d", value))
	if trace.panicValue != nil {
		panic(trace.panicValue)
	}
}
func (trace *editionOperationTrace) EmitU8(value uint8) {
	trace.entries = append(trace.entries, fmt.Sprintf("emit_u8:%d", value))
	if trace.panicValue != nil {
		panic(trace.panicValue)
	}
}
func (trace *editionOperationTrace) ReadU8() uint8 {
	trace.entries = append(trace.entries, "read_u8")
	if trace.panicValue != nil {
		panic(trace.panicValue)
	}
	return 4
}

func TestEditionDerivedOperationProtocols(t *testing.T) {
	for index, edition := range AllEditions() {
		trace := &editionOperationTrace{entries: []string{"prefix"}}
		edition.Hash(trace)
		edition.StableHash(struct{ unused string }{"context stays unused"}, trace)
		edition.Encode(trace)
		if decoded := DecodeEdition(trace); decoded != EditionFuture {
			t.Fatalf("custom decoder returned %v", decoded)
		}
		want := []string{"prefix", fmt.Sprintf("isize:%d", index), fmt.Sprintf("u8:%d", index), fmt.Sprintf("emit_u8:%d", index), "read_u8"}
		if !reflect.DeepEqual(trace.entries, want) {
			t.Fatalf("edition %s callback sequence = %v; want %v", edition.AsStr(), trace.entries, want)
		}
	}
	// Trait implementations preserve caller-owned mutations and unwinding;
	// there is no swallowed panic, extra write, or replacement error channel.
	for _, invoke := range []func(*editionOperationTrace){func(trace *editionOperationTrace) { Edition2018.Hash(trace) }, func(trace *editionOperationTrace) { Edition2018.StableHash(nil, trace) }, func(trace *editionOperationTrace) { Edition2018.Encode(trace) }, func(trace *editionOperationTrace) { DecodeEdition(trace) }} {
		sentinel := &struct{ message string }{"operation failed after side effect"}
		trace := &editionOperationTrace{panicValue: sentinel}
		func() {
			defer func() {
				if got := recover(); got != sentinel {
					t.Errorf("callback panic = %v; want original sentinel", got)
				}
			}()
			invoke(trace)
		}()
		if len(trace.entries) != 1 {
			t.Fatalf("callback side effects = %v", trace.entries)
		}
	}
}

func TestEditionActualMemoryByteProtocol(t *testing.T) {
	encoder := NewEditionMemoryEncoder()
	encoder.EmitU8(99) // A caller-owned prefix precedes the enum values.
	for _, edition := range AllEditions() {
		edition.Encode(encoder)
	}
	if encoder.Position() != 6 {
		t.Fatalf("encoded position = %d", encoder.Position())
	}
	data := encoder.Finish()
	if !bytes.Equal(data, []byte{99, 0, 1, 2, 3, 4}) {
		t.Fatalf("encoded bytes = %v", data)
	}
	blob := append(data, []byte(EditionBlobEndBytes)...)
	decoder, ok := NewEditionMemoryDecoder(blob, 1)
	if !ok || decoder.Len() != 6 || decoder.Position() != 1 || decoder.Remaining() != 5 {
		t.Fatal("memory decoder framing/position changed")
	}
	blob[1] = 255 // Go adapter snapshots the Rust-immutable input borrow.
	for _, want := range AllEditions() {
		if got := DecodeEdition(decoder); got != want {
			t.Fatalf("decoded %v; want %v", got, want)
		}
	}
	if decoder.Remaining() != 0 || decoder.Position() != 6 {
		t.Fatal("decoder did not consume precisely five tags")
	}
	if _, ok := NewEditionMemoryDecoder([]byte{0}, 0); ok {
		t.Fatal("accepted missing memory decoder footer")
	}
	if empty, ok := NewEditionMemoryDecoder([]byte(EditionBlobEndBytes), 0); !ok || empty.Len() != 0 {
		t.Fatal("empty framed decoder initialization changed")
	}
	for _, operation := range []func(){func() { decoder.ReadU8() }, func() { encoder.EmitU8(0) }, func() { encoder.Finish() }, func() { NewEditionMemoryDecoder([]byte(EditionBlobEndBytes), 1) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid memory protocol operation did not panic")
				}
			}()
			operation()
		}()
	}
}

func TestEditionAllByteTags(t *testing.T) {
	for value := 0; value < 256; value++ {
		blob := append([]byte{byte(value)}, []byte(EditionBlobEndBytes)...)
		decoder, ok := NewEditionMemoryDecoder(blob, 0)
		if !ok {
			t.Fatal("valid footer rejected")
		}
		func() {
			defer func() {
				got := recover()
				if value < 5 {
					if got != nil {
						t.Errorf("valid tag %d panicked: %v", value, got)
					}
				} else {
					want := fmt.Sprintf("invalid enum variant tag while decoding `Edition`, expected 0..5, actual %d", value)
					if got != want {
						t.Errorf("invalid tag %d panic = %v; want %s", value, got, want)
					}
				}
			}()
			decoded := DecodeEdition(decoder)
			if value < 5 && decoded != Edition(value) {
				t.Errorf("tag %d decoded %v", value, decoded)
			}
		}()
		if decoder.Position() != 1 {
			t.Errorf("tag %d consumed %d bytes", value, decoder.Position())
		}
	}
}
