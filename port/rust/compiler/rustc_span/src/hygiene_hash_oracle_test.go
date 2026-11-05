// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
)

type hygieneHashState struct {
	events []string
	value  uint64
	fail   int
}

func (s *hygieneHashState) record(event string) {
	n := len(s.events)
	s.events = append(s.events, event)
	if n == s.fail {
		panic("hasher")
	}
	for _, b := range []byte(event) {
		s.value = s.value*17 + uint64(b)
	}
}

type hygieneRawHasher struct{ state *hygieneHashState }

func (h hygieneRawHasher) Write(b []byte)   { h.state.record("bytes:" + hex.EncodeToString(b)) }
func (h hygieneRawHasher) Finish64() uint64 { return h.state.value }

type hygieneTypedHasher struct{ hygieneRawHasher }

func (h hygieneTypedHasher) WriteU8(v uint8)   { h.state.record(fmt.Sprintf("u8:%d", v)) }
func (h hygieneTypedHasher) WriteU32(v uint32) { h.state.record(fmt.Sprintf("u32:%d", v)) }
func (h hygieneTypedHasher) WriteU64(v uint64) { h.state.record(fmt.Sprintf("u64:%d", v)) }
func (h hygieneTypedHasher) WriteIsize(v int)  { h.state.record(fmt.Sprintf("isize:%d", v)) }

type hygieneUnhasherWrapper struct{ *ds.Unhasher }

func hygieneHashValue(n, count int, h stablehash.Hasher) {
	variant := n / 10
	raw := []uint32{0, 17, 0x1fffffff}[variant]
	wide := []uint32{0, 17, 0xffffff00}[variant]
	id := sp.ExpnId{Krate: sp.CrateNumFromU32([]uint32{0, 3, 0xffffff00}[variant]), LocalID: sp.ExpnIndexFromU32(wide)}
	switch n % 10 {
	case 0:
		v := sp.SyntaxContextFromU32(raw)
		sp.HygieneHashSlice([]sp.SyntaxContext{v, v}[:count], h)
	case 1:
		v := sp.ExpnIndexFromU32(wide)
		sp.HygieneHashSlice([]sp.ExpnIndex{v, v}[:count], h)
	case 2:
		v := sp.LocalExpnIdFromU32(wide)
		sp.HygieneHashSlice([]sp.LocalExpnId{v, v}[:count], h)
	case 3:
		v := sp.DecodedSyntaxContextFromU32(raw)
		sp.HygieneHashSlice([]sp.DecodedSyntaxContext{v, v}[:count], h)
	case 4:
		sp.HygieneHashSlice([]sp.ExpnId{id, id}[:count], h)
	case 5:
		p := [][2]uint64{{0, 0}, {111, 777}, {^uint64(0), 1 << 63}}[variant]
		v := hygieneCodecHash(p[0], p[1])
		sp.HygieneHashSlice([]sp.ExpnHash{v, v}[:count], h)
	case 6:
		v := sp.Transparency(variant)
		sp.HygieneHashSlice([]sp.Transparency{v, v}[:count], h)
	case 7:
		v := sp.MacroKind(variant)
		sp.HygieneHashSlice([]sp.MacroKind{v, v}[:count], h)
	case 8:
		v := sp.SyntaxContextKey{Parent: sp.SyntaxContextFromU32(raw), Expansion: id, Transparency: sp.Transparency(variant)}
		sp.HygieneHashSlice([]sp.SyntaxContextKey{v, v}[:count], h)
	case 9:
		v := sp.HygieneMark{Expansion: id, Transparency: sp.Transparency(variant)}
		sp.HygieneHashSlice([]sp.HygieneMark{v, v}[:count], h)
	}
}
func hygieneHashRefValue[H, Output any](n, count int, h stablehash.HasherRef[H, Output]) {
	if n%10 == 5 {
		p := [][2]uint64{{0, 0}, {111, 777}, {^uint64(0), 1 << 63}}[n/10]
		v := hygieneCodecHash(p[0], p[1])
		sp.ExpnHashHashSliceWithHasherRef([]sp.ExpnHash{v, v}[:count], h)
	} else {
		hygieneHashValue(n, count, h)
	}
}
func hygieneHashPanic(call func()) (failed bool) {
	defer func() {
		if recover() != nil {
			failed = true
		}
	}()
	call()
	return
}
func runHygieneHash(row hygieneOracleRow, dictionary bool) string {
	kind, count, seed, fail := row.Mode%5, row.Mode/5%3, uint64(row.Mode/15%2*7), row.Mode/30-1
	if kind < 2 {
		s := hygieneHashState{value: seed, fail: fail}
		var h stablehash.Hasher = hygieneRawHasher{&s}
		if kind == 1 {
			h = hygieneTypedHasher{hygieneRawHasher{&s}}
		}
		failed := hygieneHashPanic(func() {
			if !dictionary {
				hygieneHashValue(row.N, count, h)
				return
			}
			ops := stablehash.ExtendedHasherOps[hygieneHashState, struct{}]{Write: func(r stablehash.Ref[hygieneHashState], b []byte) { hygieneRawHasher{r.Value}.Write(b) }, Finish64: func(r stablehash.Ref[hygieneHashState]) uint64 { return r.Value.value }}
			if kind == 1 {
				ops.WriteU8 = func(r stablehash.Ref[hygieneHashState], v uint8) {
					hygieneTypedHasher{hygieneRawHasher{r.Value}}.WriteU8(v)
				}
				ops.WriteU32 = func(r stablehash.Ref[hygieneHashState], v uint32) {
					hygieneTypedHasher{hygieneRawHasher{r.Value}}.WriteU32(v)
				}
				ops.WriteU64 = func(r stablehash.Ref[hygieneHashState], v uint64) {
					hygieneTypedHasher{hygieneRawHasher{r.Value}}.WriteU64(v)
				}
				ops.WriteIsize = func(r stablehash.Ref[hygieneHashState], v int) {
					hygieneTypedHasher{hygieneRawHasher{r.Value}}.WriteIsize(v)
				}
			}
			hygieneHashRefValue(row.N, count, ops.Bind(stablehash.Ref[hygieneHashState]{Value: &s}))
		})
		return fmt.Sprintf("trace %t %d %s\n", failed, s.value, strings.Join(s.events, ","))
	}
	h := ds.NewUnhasherWithDebugAssertions(row.Extra != 0)
	h.WriteU64(seed)
	failed := hygieneHashPanic(func() {
		if kind == 4 {
			hygieneHashValue(row.N, count, hygieneUnhasherWrapper{h})
			return
		}
		if kind == 2 && !dictionary {
			hygieneHashValue(row.N, count, h)
			return
		}
		if kind == 2 {
			ops := stablehash.ExtendedHasherOps[ds.Unhasher, struct{}]{Write: func(r stablehash.Ref[ds.Unhasher], b []byte) { r.Value.Write(b) }, WriteU64: func(r stablehash.Ref[ds.Unhasher], v uint64) { r.Value.WriteU64(v) }, Finish64: func(r stablehash.Ref[ds.Unhasher]) uint64 { return r.Value.Finish64() }}
			hygieneHashRefValue(row.N, count, ops.Bind(stablehash.Ref[ds.Unhasher]{Value: h}))
			return
		}
		ops := stablehash.ExtendedHasherOps[*ds.Unhasher, struct{}]{Write: func(r stablehash.Ref[*ds.Unhasher], b []byte) { (*r.Value).Write(b) }, WriteU64: func(r stablehash.Ref[*ds.Unhasher], v uint64) { (*r.Value).WriteU64(v) }, Finish64: func(r stablehash.Ref[*ds.Unhasher]) uint64 { return (*r.Value).Finish64() }}
		hygieneHashRefValue(row.N, count, ops.Bind(stablehash.Ref[*ds.Unhasher]{Value: &h}))
	})
	return fmt.Sprintf("unhash %t %d\n", failed, h.Finish64())
}
func TestOriginalHygieneHash(t *testing.T) {
	readHygieneOracle(t, "hash", func(row hygieneOracleRow) string {
		actual := runHygieneHash(row, false)
		dictionary := runHygieneHash(row, true)
		if actual != dictionary {
			t.Fatalf("direct/dictionary mismatch n%d mode%d", row.N, row.Mode)
		}
		return actual
	})
}
