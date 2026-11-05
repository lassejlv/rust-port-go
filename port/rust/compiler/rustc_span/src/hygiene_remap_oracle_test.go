// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"fmt"
	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"strings"
	"testing"
)

type hygieneRemapKey struct {
	raw      uint32
	metadata []byte
}
type hygieneRemapA struct{ raw uint32 }
type hygieneRemapB struct {
	raw      uint32
	metadata []byte
}
type hygieneRemapState struct {
	length uint
	first  sp.Option[hygieneRemapKey]
	fail   *uint32
	log    *[]string
}

func (s *hygieneRemapState) push(f string, a ...any) { *s.log = append(*s.log, fmt.Sprintf(f, a...)) }

type hygieneRemapDummy struct{}

func (hygieneRemapDummy) StableHashSpan(ds.RawSpan, *ds.StableHasher) { panic("unexpected context") }
func (hygieneRemapDummy) DefPathHash(ds.RawDefId) ds.Fingerprint      { panic("unexpected context") }
func (hygieneRemapDummy) StableHashControls() ds.StableHashControls   { panic("unexpected context") }
func (hygieneRemapDummy) AssertDefaultStableHashControls(string)      { panic("unexpected context") }
func hygieneRemapRequired[O any]() sp.HygieneRemapperOps[hygieneRemapState, hygieneRemapKey, O] {
	return sp.HygieneRemapperOps[hygieneRemapState, hygieneRemapKey, O]{StableHash: func(s *hygieneRemapState, k hygieneRemapKey, _ ds.StableHashCtxt, h *ds.StableHasher, _ rf.Context) {
		s.push("hash:%d", k.raw)
		if s.fail != nil && *s.fail == k.raw {
			panic("hash failed")
		}
		h.WriteU64(uint64(k.raw % 3))
	}, Len: func(s *hygieneRemapState, _ rf.Context) uint { s.push("len"); return s.length }, FirstNonDetIndexMut: func(s *hygieneRemapState, _ rf.Context) *sp.Option[hygieneRemapKey] {
		s.push("firstmut")
		return &s.first
	}, FirstNonDetIndex: func(s *hygieneRemapState, _ rf.Context) uint32 {
		s.push("first")
		v, ok := s.first.Get()
		if !ok {
			panic("not committed")
		}
		return v.raw
	}}
}
func hygieneRemapCaught(call func()) (ok bool) {
	ok = true
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	call()
	return
}
func runHygieneRemap(old, extra, mode int) string {
	log := []string{}
	s := hygieneRemapState{length: uint(old), log: &log}
	if mode&4 != 0 {
		v := uint32(old + 1)
		s.fail = &v
	}
	ctx := rf.Context{}
	from := func(raw uint32, _ rf.Context) hygieneRemapKey {
		s.push("keyfrom:%d", raw)
		return hygieneRemapKey{raw, []byte{1, 2}}
	}
	ao, bo := hygieneRemapRequired[hygieneRemapA](), hygieneRemapRequired[hygieneRemapB]()
	if mode&2 != 0 {
		ao.CommitEndOfDeterminism = func(s *hygieneRemapState, c rf.Context) {
			s.push("overridecommit")
			base := sp.BorrowHygieneRemapper(s, hygieneRemapRequired[hygieneRemapA](), from)
			sp.CommitHygieneEndOfDeterminismDefault[hygieneRemapKey](base, from, c)
		}
		ao.RemappingResults = func(s *hygieneRemapState, h ds.StableHashCtxt, c rf.Context) sp.HygieneRemappingResults[hygieneRemapKey, hygieneRemapA] {
			s.push("overrideremap:A")
			base := sp.BorrowHygieneRemapper(s, hygieneRemapRequired[hygieneRemapA](), from)
			return sp.ComputeHygieneRemappingResultsDefault[hygieneRemapKey, hygieneRemapA](base, from, h, c)
		}
		bo.RemappingResults = func(s *hygieneRemapState, h ds.StableHashCtxt, c rf.Context) sp.HygieneRemappingResults[hygieneRemapKey, hygieneRemapB] {
			s.push("overrideremap:B")
			base := sp.BorrowHygieneRemapper(s, hygieneRemapRequired[hygieneRemapB](), from)
			return sp.ComputeHygieneRemappingResultsDefault[hygieneRemapKey, hygieneRemapB](base, from, h, c)
		}
	}
	a, b := sp.BorrowHygieneRemapper(&s, ao, from), sp.BorrowHygieneRemapper(&s, bo, from)
	sp.CommitHygieneEndOfDeterminism[hygieneRemapKey](a, from, ctx)
	s.length = uint(old + extra)
	if mode&1 != 0 {
		ok := hygieneRemapCaught(func() { sp.CommitHygieneEndOfDeterminism[hygieneRemapKey](a, from, ctx) })
		s.push("commitagain:%t", ok)
	}
	boundary := a.FirstNonDetIndex(ctx)
	s.push("boundary:%d", boundary)
	into := func(k hygieneRemapKey, _ rf.Context) uint32 { s.push("into:%d", k.raw); return k.raw }
	if !hygieneRemapCaught(func() {
		r := sp.ComputeHygieneRemappingResults[hygieneRemapKey, hygieneRemapA](a, from, hygieneRemapDummy{}, ctx)
		defer r.Close()
		for i := 0; i < old+extra+3; i++ {
			value := r.MapIDWithContext(hygieneRemapKey{uint32(i), []byte{1, 2}}, into, func(raw uint32, _ rf.Context) hygieneRemapA { s.push("afrom:%d", raw); return hygieneRemapA{raw} }, ctx)
			s.push("aresult:%d:%d", i, value.raw)
		}
	}) {
		s.push("a:panic")
	}
	if !hygieneRemapCaught(func() {
		r := sp.ComputeHygieneRemappingResults[hygieneRemapKey, hygieneRemapB](b, from, hygieneRemapDummy{}, ctx)
		defer r.Close()
		for i := 0; i < old+extra+3; i++ {
			value := r.MapIDWithContext(hygieneRemapKey{uint32(i), []byte{1, 2}}, into, func(raw uint32, _ rf.Context) hygieneRemapB {
				s.push("bfrom:%d", raw)
				return hygieneRemapB{raw + 100, []byte{3, 4}}
			}, ctx)
			s.push("bresult:%d:%d", i, value.raw)
		}
	}) {
		s.push("b:panic")
	}
	return strings.Join(log, "\n") + "\n"
}

func TestOriginalHygieneRemapping(t *testing.T) {
	readHygieneOracle(t, "remap", func(row hygieneOracleRow) string { return runHygieneRemap(row.Old, row.Extra, row.Mode) })
}
