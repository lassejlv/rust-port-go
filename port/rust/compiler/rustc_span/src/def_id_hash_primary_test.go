// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"fmt"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type defIDFullRaw struct {
	events      []string
	fail, calls int
}

func (h *defIDFullRaw) record(s string) {
	h.events = append(h.events, s)
	n := h.calls
	h.calls++
	if n == h.fail {
		panic("hasher")
	}
}
func (h *defIDFullRaw) Write(b []byte)   { h.record(fmt.Sprintf("bytes:%x", b)) }
func (h *defIDFullRaw) Finish64() uint64 { panic("unexpected finish") }

type defIDFullTyped struct{ *defIDFullRaw }

func (h defIDFullTyped) WriteU8(v uint8) { h.record(fmt.Sprintf("u8:%d", v)) }

func (h defIDFullTyped) WriteU32(v uint32) { h.record(fmt.Sprintf("u32:%d", v)) }
func (h defIDFullTyped) WriteU64(v uint64) { h.record(fmt.Sprintf("u64:%d", v)) }

func defIDFullHashInput[T interface{ HashWithHasher(stablehash.Hasher) }](value T, q defIDRequest, h stablehash.Hasher, slice func([]T, stablehash.Hasher)) {
	switch q.Method {
	case "single":
		value.HashWithHasher(h)
	case "slice":
		slice([]T{value, value}, h)
	case "empty":
		slice(nil, h)
	default:
		panic("unknown hash input")
	}
}
func defIDFullHashApply(q defIDRequest, h stablehash.Hasher) {
	id := DefId{Krate: CrateNumFromU32(q.Crate), Index: DefIndexFromU32(q.Index)}
	local := LocalDefId{LocalDefIndex: id.Index}
	path := DefPathHash{Fingerprint: datastructures.FingerprintFromU64(q.First, q.Second)}
	switch q.Kind {
	case "controls":
		defIDFullHashInput(datastructures.StableHashControls{HashSpans: q.Exe}, q, h, datastructures.StableHashControlsHashSliceWithHasher)
	case "crate":
		defIDFullHashInput(id.Krate, q, h, CrateNumHashSliceWithHasher)
	case "index":
		defIDFullHashInput(id.Index, q, h, DefIndexHashSliceWithHasher)
	case "id":
		defIDFullHashInput(id, q, h, DefIdHashSliceWithHasher)
	case "local":
		defIDFullHashInput(local, q, h, LocalDefIdHashSliceWithHasher)
	case "mod":
		defIDFullHashInput(NewModIdUnchecked(id), q, h, ModIdHashSliceWithHasher)
	case "localmod":
		defIDFullHashInput(NewLocalModIdUnchecked(local), q, h, LocalModIdHashSliceWithHasher)
	case "path":
		defIDFullHashInput(path, q, h, DefPathHashHashSliceWithHasher)
	case "stable":
		defIDFullHashInput(path.StableCrateId(), q, h, StableCrateIdHashSliceWithHasher)
	default:
		panic("unknown hash kind")
	}
}
func defIDFullHash(q defIDRequest) any {
	if q.Name == "unhash" {
		h := datastructures.NewUnhasherWithContext(rustfmt.Context{}, q.Debug)
		h.WriteU64(q.A)
		p, failed := defIDCatch(func() any {
			if q.Kind == "path" && q.Override {
				op := stablehash.ExtendedHasherOps[datastructures.Unhasher, struct{}]{
					Write: func(r stablehash.Ref[datastructures.Unhasher], b []byte) {
						r.Value.SetRuntimeContext(r.Context)
						r.Value.Write(b)
					},
					WriteU64: func(r stablehash.Ref[datastructures.Unhasher], n uint64) {
						r.Value.SetRuntimeContext(r.Context)
						r.Value.WriteU64(n)
					},
					Finish64: func(r stablehash.Ref[datastructures.Unhasher]) uint64 { return r.Value.Finish64() },
				}
				r := op.Bind(stablehash.Ref[datastructures.Unhasher]{Value: h})
				value := DefPathHash{Fingerprint: datastructures.FingerprintFromU64(q.First, q.Second)}
				switch q.Method {
				case "single":
					DefPathHashHashWithHasherRef(value, r)
				case "slice":
					DefPathHashHashSliceWithHasherRef([]DefPathHash{value, value}, r)
				case "empty":
					DefPathHashHashSliceWithHasherRef(nil, r)
				}
			} else {
				defIDFullHashApply(q, h)
			}
			return nil
		})
		if !failed {
			p = nil
		}
		return map[string]any{"panic": p, "value": h.Finish64()}
	}
	h := &defIDFullRaw{events: []string{}, fail: q.Fail}
	var hasher stablehash.Hasher = h
	if q.Name == "typed" {
		hasher = defIDFullTyped{h}
	}
	p, failed := defIDCatch(func() any { defIDFullHashApply(q, hasher); return nil })
	if !failed {
		p = nil
	}
	return map[string]any{"panic": p, "events": h.events, "calls": h.calls}
}
