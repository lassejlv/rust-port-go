// SPDX-License-Identifier: MIT OR Apache-2.0
package indexmapsubject

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	im "github.com/lassejlv/rust-port-go/port/support/rustindexmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type IteratorRequest struct {
	N       uint
	Kind    string
	Steps   []IteratorStep
	Default bool
}
type IteratorStep struct {
	Op      string
	N, Mode uint
	Delta   int64
}
type iterCommon interface {
	Len(...im.Context) uint
	SizeHint(...im.Context) rustvec.Hint
	Count(...im.Context) uint
	WriteDebugFormatter(*rustfmt.Formatter) error
	Close()
}
type pairIterator interface {
	iterCommon
	Next(...im.Context) (*uint64, *int64, bool)
	NextBack(...im.Context) (*uint64, *int64, bool)
	Nth(uint, ...im.Context) (*uint64, *int64, bool)
	NthBack(uint, ...im.Context) (*uint64, *int64, bool)
	Last(...im.Context) (*uint64, *int64, bool)
}
type keyIterator interface {
	iterCommon
	Next(...im.Context) (*uint64, bool)
	NextBack(...im.Context) (*uint64, bool)
	Nth(uint, ...im.Context) (*uint64, bool)
	NthBack(uint, ...im.Context) (*uint64, bool)
	Last(...im.Context) (*uint64, bool)
}
type valueIterator interface {
	iterCommon
	Next(...im.Context) (*int64, bool)
	NextBack(...im.Context) (*int64, bool)
	Nth(uint, ...im.Context) (*int64, bool)
	NthBack(uint, ...im.Context) (*int64, bool)
	Last(...im.Context) (*int64, bool)
}
type iterView struct {
	base      iterCommon
	next      func(IteratorStep) any
	clone     func() iterView
	cloneFrom func(iterView)
	slice     func() any
	mutable   bool
	intoSlice func(int64) any
}

func pairRead(it pairIterator, mut bool) iterView {
	view := iterView{base: it, mutable: mut}
	view.next = func(op IteratorStep) any {
		var k *uint64
		var v *int64
		var ok bool
		switch op.Op {
		case "next":
			k, v, ok = it.Next()
		case "back":
			k, v, ok = it.NextBack()
		case "nth":
			k, v, ok = it.Nth(op.N)
		case "nth_back":
			k, v, ok = it.NthBack(op.N)
		case "last":
			k, v, ok = it.Last()
		}
		if !ok {
			return nil
		}
		if mut {
			*v += op.Delta
		}
		return []any{*k, *v}
	}
	return view
}
func keyRead(it keyIterator) iterView {
	view := iterView{base: it}
	view.next = func(op IteratorStep) any {
		var k *uint64
		var ok bool
		switch op.Op {
		case "next":
			k, ok = it.Next()
		case "back":
			k, ok = it.NextBack()
		case "nth":
			k, ok = it.Nth(op.N)
		case "nth_back":
			k, ok = it.NthBack(op.N)
		case "last":
			k, ok = it.Last()
		}
		if !ok {
			return nil
		}
		return *k
	}
	return view
}
func valueRead(it valueIterator, mut bool) iterView {
	view := iterView{base: it, mutable: mut}
	view.next = func(op IteratorStep) any {
		var v *int64
		var ok bool
		switch op.Op {
		case "next":
			v, ok = it.Next()
		case "back":
			v, ok = it.NextBack()
		case "nth":
			v, ok = it.Nth(op.N)
		case "nth_back":
			v, ok = it.NthBack(op.N)
		case "last":
			v, ok = it.Last()
		}
		if !ok {
			return nil
		}
		if mut {
			*v += op.Delta
		}
		return *v
	}
	return view
}
func sharedPairs(it *im.Iter[uint64, int64]) iterView {
	view := pairRead(it, false)
	view.clone = func() iterView { return sharedPairs(it.Clone()) }
	view.cloneFrom = func(other iterView) { it.CloneFrom(other.base.(*im.Iter[uint64, int64])) }
	view.slice = func() any {
		s := it.AsSlice()
		out := []any{}
		for i := uint(0); i < s.Len(); i++ {
			k, v, _ := s.GetIndex(i)
			out = append(out, []any{*k, *v})
		}
		return out
	}
	return view
}
func sharedKeys(it *im.Keys[uint64, int64]) iterView {
	view := keyRead(it)
	view.clone = func() iterView { return sharedKeys(it.Clone()) }
	view.cloneFrom = func(other iterView) { it.CloneFrom(other.base.(*im.Keys[uint64, int64])) }
	return view
}
func sharedValues(it *im.Values[uint64, int64]) iterView {
	view := valueRead(it, false)
	view.clone = func() iterView { return sharedValues(it.Clone()) }
	view.cloneFrom = func(other iterView) { it.CloneFrom(other.base.(*im.Values[uint64, int64])) }
	return view
}
func iterDebug(it iterCommon, mode uint) string {
	return rustfmt.String(func(f *rustfmt.Formatter) error {
		switch mode {
		case 1:
			f.Options.Alternate = true
		case 2:
			f.Options.DebugLowerHex = true
		case 3:
			f.Options.SignPlus = true
			f.Options.ZeroPad = true
			f.Options.HasWidth = true
			f.Options.Width = 8
			f.Options.DebugUpperHex = true
		}
		return it.WriteDebugFormatter(f)
	})
}
func RunIterator(q IteratorRequest) any {
	config := im.Config[uint64, int64]{BucketLayout: im.Layout{Size: 24, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true, Keys: im.KeyOps[uint64]{KeyOps: rusthashmap.KeyOps[uint64]{Hash: func(k *uint64, h rusthashmap.Hasher, _ im.Context) { rusthashmap.HashU64(h, *k) }, Equal: func(a, b *uint64, _ im.Context) bool { return *a == *b }, TrivialClone: true, Debug: func(k *uint64, f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(*k, 64, f) }}}, Values: im.ValueOps[int64]{ValueOps: rusthashmap.ValueOps[int64]{TrivialClone: true, Debug: func(v *int64, f *rustfmt.Formatter) error { return rustfmt.FormatDebugInt(*v, 64, f) }}}}
	m := im.WithHasher(config, rusthashmap.FxBuilder{PointerBits: 64})
	defer m.Close()
	for i := uint(0); i < q.N; i++ {
		m.Insert(uint64(i)*17+1, -int64(i)*9-1)
	}
	var view iterView
	switch q.Kind {
	case "iter":
		it := m.Iter()
		if q.Default {
			it.Close()
			it = im.DefaultIter(config)
		}
		view = sharedPairs(it)
	case "keys":
		it := m.Keys()
		if q.Default {
			it.Close()
			it = im.DefaultKeys(config)
		}
		view = sharedKeys(it)
	case "values":
		it := m.Values()
		if q.Default {
			it.Close()
			it = im.DefaultValues(config)
		}
		view = sharedValues(it)
	case "iter_mut":
		it := m.IterMut()
		if q.Default {
			it.Close()
			it = im.DefaultIterMut(config)
		}
		view = pairRead(it, true)
		view.slice = func() any {
			s := it.AsSlice()
			out := []any{}
			for i := uint(0); i < s.Len(); i++ {
				k, v, _ := s.GetIndex(i)
				out = append(out, []any{*k, *v})
			}
			return out
		}
		view.intoSlice = func(delta int64) any {
			s := it.IntoSlice()
			out := []any{}
			for i := uint(0); i < s.Len(); i++ {
				k, v, _ := s.GetIndexMut(i)
				*v += delta
				out = append(out, []any{*k, *v})
			}
			return out
		}
	case "values_mut":
		it := m.ValuesMut()
		if q.Default {
			it.Close()
			it = im.DefaultValuesMut(config)
		}
		view = valueRead(it, true)
	default:
		panic("bad kind")
	}
	defer view.base.Close()
	rows := []any{}
	for _, op := range q.Steps {
		var value any
		done := false
		switch op.Op {
		case "next", "back", "nth", "nth_back":
			value = view.next(op)
		case "count":
			if view.mutable {
				value = view.base.Count()
				done = true
			} else {
				cloned := view.clone()
				value = cloned.base.Count()
			}
		case "last":
			if view.mutable {
				value = view.next(op)
				done = true
			} else {
				cloned := view.clone()
				value = cloned.next(op)
			}
		case "clone_from":
			other := view.clone()
			other.next(IteratorStep{Op: "nth", N: op.N})
			view.cloneFrom(other)
			other.base.Close()
		case "slice":
			value = view.slice()
		case "into_slice":
			value = view.intoSlice(op.Delta)
			done = true
		case "debug":
			value = iterDebug(view.base, op.Mode)
		}
		if done {
			rows = append(rows, map[string]any{"value": value, "len": nil, "hint": nil, "debug": nil})
			break
		}
		hint := view.base.SizeHint()
		rows = append(rows, map[string]any{"value": value, "len": view.base.Len(), "hint": []any{hint.Lower, hint.Upper}, "debug": iterDebug(view.base, 0)})
	}
	view.base.Close()
	entries := []any{}
	s := m.AsSlice()
	for i := uint(0); i < s.Len(); i++ {
		k, v, _ := s.GetIndex(i)
		entries = append(entries, []any{*k, *v})
	}
	return map[string]any{"rows": rows, "entries": entries}
}
