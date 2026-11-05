// SPDX-License-Identifier: MIT OR Apache-2.0
package indexmapsubject

import (
	"errors"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"strings"
)

type EntryFormatRequest struct {
	Kind   string
	Format uint
	Fail   int
	Chars  bool
}
type entryFormatSink struct {
	calls []any
	out   strings.Builder
	fail  int
	chars bool
}

func (s *entryFormatSink) step(kind, text string) (int, error) {
	n := len(s.calls)
	s.calls = append(s.calls, map[string]any{"Kind": kind, "Text": text})
	if s.fail >= 0 && n == s.fail {
		return 0, errors.New("fmt::Error")
	}
	return s.out.WriteString(text)
}
func (s *entryFormatSink) Write(value []byte) (int, error)       { return s.step("S", string(value)) }
func (s *entryFormatSink) WriteString(value string) (int, error) { return s.step("S", value) }
func (s *entryFormatSink) WriteChar(value rune) error {
	kind := "S"
	if s.chars {
		kind = "C"
	}
	_, err := s.step(kind, string(value))
	return err
}
func RunEntryFormat(q EntryFormatRequest) any {
	log := []string{}
	config := im.Config[uint64, int32]{BucketLayout: im.Layout{Size: 24, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true, Keys: im.KeyOps[uint64]{KeyOps: rusthashmap.KeyOps[uint64]{Hash: func(k *uint64, h rusthashmap.Hasher, _ im.Context) { rusthashmap.HashU64(h, *k) }, Equal: func(a, b *uint64, _ im.Context) bool { return *a == *b }, Debug: func(k *uint64, f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(*k, 64, f) }}}, Values: im.ValueOps[int32]{ValueOps: rusthashmap.ValueOps[int32]{Debug: func(v *int32, f *rustfmt.Formatter) error { return rustfmt.FormatDebugInt(int64(*v), 32, f) }}}}
	m := im.WithCapacityAndHasher(0, config, &builder{&log, 0, ^uint64(0)})
	defer m.Close()
	m.Insert(10, -26)
	m.Insert(20, 33)
	c := im.Context{}
	var value interface {
		WriteDebugFormatter(*rustfmt.Formatter) error
	}
	switch q.Kind {
	case "entry_occupied":
		e := m.Entry(10)
		defer e.Close(c)
		value = e
	case "entry_vacant":
		e := m.Entry(7)
		defer e.Close(c)
		value = e
	case "occupied":
		e, _ := m.Entry(10).TakeOccupied(c)
		defer e.Close(c)
		value = e
	case "vacant":
		e, _ := m.Entry(7).TakeVacant(c)
		defer e.Close(c)
		value = e
	case "indexed":
		e, _ := m.GetIndexEntry(1)
		defer e.Close(c)
		value = e
	default:
		panic("bad kind")
	}
	options := rustfmt.DefaultOptions()
	switch q.Format {
	case 0:
	case 1:
		options.Alternate = true
	case 2:
		options.DebugLowerHex = true
	case 3:
		options.DebugUpperHex = true
	case 4:
		options.SignPlus = true
		options.ZeroPad = true
		options.DebugLowerHex = true
		options.Width = 8
		options.HasWidth = true
	case 5:
		options.SignPlus = true
		options.Alternate = true
		options.ZeroPad = true
		options.DebugUpperHex = true
		options.Width = 8
		options.HasWidth = true
	case 6:
		options.Fill = '*'
		options.Align = rustfmt.AlignCenter
		options.Width = 9
		options.HasWidth = true
	case 7:
		options.Fill = '0'
		options.Align = rustfmt.AlignRight
		options.Width = 12
		options.HasWidth = true
	case 8:
		options.Fill = '\n'
		options.Align = rustfmt.AlignCenter
		options.Alternate = true
		options.Width = 12
		options.HasWidth = true
	case 9:
		options.Precision = 3
		options.HasPrecision = true
	case 10:
		options.SignPlus = true
	case 11:
		options.SignMinus = true
	default:
		panic("bad format")
	}
	sink := &entryFormatSink{calls: []any{}, fail: q.Fail, chars: q.Chars}
	f := rustfmt.Formatter{Writer: sink, Options: options}
	err := value.WriteDebugFormatter(&f)
	var align any
	switch f.Options.Align {
	case rustfmt.AlignLeft:
		align = "Left"
	case rustfmt.AlignRight:
		align = "Right"
	case rustfmt.AlignCenter:
		align = "Center"
	}
	return map[string]any{"calls": sink.calls, "out": sink.out.String(), "failed": err != nil, "post": map[string]any{"Fill": string(f.Options.Fill), "Align": align}}
}
