// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func hygieneU32KeyOps() hashmap.KeyOps[uint32] {
	return hashmap.KeyOps[uint32]{Hash: func(v *uint32, h hashmap.Hasher, _ hashmap.Context) { hashmap.HashU32(h, *v) }, Equal: func(a, b *uint32, _ hashmap.Context) bool { return *a == *b }, TrivialClone: true}
}
func newHygieneRemapTable() *hashmap.Map[uint32, uint32] {
	return hashmap.NewMap(hashmap.Layout{Size: 8, Align: 4}, hashmap.DefaultProfile(), hygieneU32KeyOps(), hashmap.ValueOps[uint32]{TrivialClone: true}, hashmap.FxBuilder{PointerBits: 64})
}

type HygieneRemappingResults[TKey, TRemappedKey any] struct {
	remapping        *hashmap.Map[uint32, uint32]
	firstNonDetIndex uint32
}

func DefaultHygieneRemappingResults[K, O any]() HygieneRemappingResults[K, O] {
	return HygieneRemappingResults[K, O]{remapping: newHygieneRemapTable()}
}
func (r *HygieneRemappingResults[K, O]) CloseWithContext(ctx rustfmt.Context) {
	if r.remapping != nil {
		table := r.remapping
		r.remapping = nil
		table.Close(ctx)
	}
}
func (r *HygieneRemappingResults[K, O]) Close() { r.CloseWithContext(rustfmt.Context{}) }

// The two conversion dictionaries are the source Into<u32>/From<u32>
// operations. K and O have no Go comparability restriction; source Copy/Eq/Hash
// laws remain their dictionaries' contracts. Hash/Eq are not spuriously called.
func (r *HygieneRemappingResults[K, O]) MapID(id K, into func(K) uint32, from func(uint32) O) O {
	return r.MapIDWithContext(id, func(id K, _ rustfmt.Context) uint32 { return into(id) }, func(id uint32, _ rustfmt.Context) O { return from(id) }, rustfmt.Context{})
}
func (r *HygieneRemappingResults[K, O]) MapIDWithContext(id K, into func(K, rustfmt.Context) uint32, from func(uint32, rustfmt.Context) O, ctx rustfmt.Context) O {
	raw := into(id, ctx)
	remapped := raw
	if raw >= r.firstNonDetIndex && r.remapping != nil {
		if value, ok := r.remapping.Get(&raw, ctx); ok {
			remapped = *value
		}
	}
	return from(remapped, ctx)
}

// HygieneRemapper supplies the four required trait methods on a genuinely
// borrowed implementor. The operations carry ambient panic state explicitly.
type HygieneRemapper[T any] interface {
	StableHash(T, datastructures.StableHashCtxt, *datastructures.StableHasher, rustfmt.Context)
	Len() uint
	FirstNonDetIndexMut() *Option[T]
	FirstNonDetIndex(rustfmt.Context) uint32
}
type HygieneCommitOverride interface{ CommitEndOfDeterminism(rustfmt.Context) }
type hygieneRemapperContextLen interface{ LenWithContext(rustfmt.Context) uint }
type hygieneRemapperContextSlot[T any] interface {
	FirstNonDetIndexMutWithContext(rustfmt.Context) *Option[T]
}

func hygieneRemapperLen[T any](r HygieneRemapper[T], ctx rustfmt.Context) uint {
	if operation, ok := r.(hygieneRemapperContextLen); ok {
		return operation.LenWithContext(ctx)
	}
	return r.Len()
}
func hygieneRemapperSlot[T any](r HygieneRemapper[T], ctx rustfmt.Context) *Option[T] {
	if operation, ok := r.(hygieneRemapperContextSlot[T]); ok {
		return operation.FirstNonDetIndexMutWithContext(ctx)
	}
	return r.FirstNonDetIndexMut()
}

// Commit requires only From<u32>; its T may own fields and implement Drop.
// These are borrowed static type operations, not an owned callback capture.
// A nil Drop selects a source value with trivial destruction.
type HygieneCommitValueOps[T any] struct {
	FromU32 func(uint32, rustfmt.Context) T
	Drop    func(*T, rustfmt.Context)
}
type hygieneCommitValueDropper[T any] interface {
	DropHygieneCommitValue(*T, rustfmt.Context)
}

func CommitHygieneEndOfDeterminism[T any](remapper HygieneRemapper[T], from func(uint32, rustfmt.Context) T, ctx rustfmt.Context) {
	if override, ok := remapper.(HygieneCommitOverride); ok {
		override.CommitEndOfDeterminism(ctx)
		return
	}
	CommitHygieneEndOfDeterminismDefault(remapper, from, ctx)
}

// The from-only adapter uses the remapper's DropHygieneCommitValue dictionary,
// when supplied; otherwise it is the trivial-destruction convenience. The
// complete arbitrary-T primary is CommitHygieneEndOfDeterminismWithOps.
func CommitHygieneEndOfDeterminismDefault[T any](remapper HygieneRemapper[T], from func(uint32, rustfmt.Context) T, ctx rustfmt.Context) {
	operations := HygieneCommitValueOps[T]{FromU32: from}
	if dictionary, ok := remapper.(hygieneCommitValueDropper[T]); ok {
		operations.Drop = dictionary.DropHygieneCommitValue
	}
	CommitHygieneEndOfDeterminismDefaultWithOps(remapper, operations, ctx)
}
func CommitHygieneEndOfDeterminismWithOps[T any](remapper HygieneRemapper[T], operations HygieneCommitValueOps[T], ctx rustfmt.Context) {
	if override, ok := remapper.(HygieneCommitOverride); ok {
		override.CommitEndOfDeterminism(ctx)
		return
	}
	CommitHygieneEndOfDeterminismDefaultWithOps(remapper, operations, ctx)
}
func CommitHygieneEndOfDeterminismDefaultWithOps[T any](remapper HygieneRemapper[T], operations HygieneCommitValueOps[T], ctx rustfmt.Context) {
	length := uint32(hygieneRemapperLen(remapper, ctx))
	slot := hygieneRemapperSlot(remapper, ctx)
	replacement := operations.FromU32(length, ctx)
	previous := *slot
	*slot = Some(replacement)
	// Actual original MIR installs Some(T) before testing prev. The moved
	// previous Option is destroyed on ordinary return or the assertion's
	// cleanup edge; a failing Drop on that cleanup edge aborts immediately.
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if previous.present && operations.Drop != nil {
			operations.Drop(&previous.value, c)
		}
	}, ctx)
	if previous.present {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "this function can be called only once", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation("rustc_span::hygiene::HygieneRemapper.commit_end_of_determinism.repeat")})
	}
}

// HygieneRemappingOverride is the source generic default-method dispatch for
// this selected output type. Distinct output-type dictionaries can override
// independently, without pretending Go has generic interface methods.
type HygieneRemappingOverride[K, O any] interface {
	RemappingResults(datastructures.StableHashCtxt, rustfmt.Context) HygieneRemappingResults[K, O]
}

func ComputeHygieneRemappingResults[K, O any](remapper HygieneRemapper[K], from func(uint32, rustfmt.Context) K, hcx datastructures.StableHashCtxt, ctx rustfmt.Context) HygieneRemappingResults[K, O] {
	if override, ok := remapper.(HygieneRemappingOverride[K, O]); ok {
		return override.RemappingResults(hcx, ctx)
	}
	return ComputeHygieneRemappingResultsDefault[K, O](remapper, from, hcx, ctx)
}
func ComputeHygieneRemappingResultsDefault[K, O any](remapper HygieneRemapper[K], from func(uint32, rustfmt.Context) K, hcx datastructures.StableHashCtxt, ctx rustfmt.Context) HygieneRemappingResults[K, O] {
	start := remapper.FirstNonDetIndex(ctx)
	end := uint32(hygieneRemapperLen(remapper, ctx))
	keys := []uint32{}
	for key := start; key < end; key++ {
		keys = append(keys, key)
	}
	// The source FnMut owns only borrowed self/hcx references. Its complete
	// environment has trivial Drop; the associated From operation is static
	// dictionary metadata, rather than an extra destructible capture.
	type keyEnvironment struct {
		remapper HygieneRemapper[K]
		hcx      datastructures.StableHashCtxt
		from     func(uint32, rustfmt.Context) K
	}
	keyFunction := rustsort.NewKeyFunction(rustvec.Own(keyEnvironment{remapper, hcx, from}), func(env *keyEnvironment, key *uint32, c rustfmt.Context) *rustvec.Owned[uint64] {
		h := datastructures.NewStableHasher()
		h.SetContext(c)
		env.remapper.StableHash(env.from(*key, c), env.hcx, h, c)
		// Hasher::finish borrows this hasher and returns u64. It is distinct
		// from StableHasher's consuming generic 128-bit finish operation.
		return rustvec.Own(h.Finish64())
	})
	rustsort.Cached(keys, keyFunction, rustsort.CachedConfig{KeyType: rustsort.Type{Size: 8, Align: 8, Freeze: true, Copy: true}, PointerBits: 64}, rustsort.CachedKeyOps[uint64]{PartialCompare: func(a, b *uint64, _ rustfmt.Context) (rustsort.Ordering, bool) {
		if *a < *b {
			return rustsort.LessThan, true
		}
		if *a > *b {
			return rustsort.GreaterThan, true
		}
		return rustsort.Equal, true
	}}, ctx)
	result := HygieneRemappingResults[K, O]{remapping: newHygieneRemapTable(), firstNonDetIndex: start}
	for i, key := range keys {
		to := start + uint32(i)
		if key != to {
			result.remapping.Insert(key, to, ctx)
		}
	}
	return result
}

type SyntaxContextRemapper struct {
	data    *HygieneData
	session *SessionGlobals
}

func (r SyntaxContextRemapper) Len() uint { return r.data.syntaxContextData.Len() }
func (r SyntaxContextRemapper) FirstNonDetIndexMut() *Option[SyntaxContext] {
	return &r.data.firstNonDetSyntaxContext
}
func (r SyntaxContextRemapper) FirstNonDetIndex(ctx rustfmt.Context) uint32 {
	value, ok := r.data.firstNonDetSyntaxContext.Get()
	if !ok {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "must be set at this point", Kind: rustfmt.PanicOwnedString, CanUnwind: true, Location: ctx.SourcePanicLocation("rustc_span::hygiene::SyntaxContextRemapper.first_non_det_index.unset")})
	}
	return value.AsU32()
}
func (r SyntaxContextRemapper) StableHash(ctxt SyntaxContext, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
	ctxt.stableHashInternal(r.session, r.data, hcx, h, ctx)
}
func (r SyntaxContextRemapper) CommitEndOfDeterminism(ctx rustfmt.Context) {
	CommitHygieneEndOfDeterminismDefault[SyntaxContext](r, SyntaxContextFromU32WithContext, ctx)
}

type LocalExpnIdRemapper struct {
	data    *HygieneData
	session *SessionGlobals
}

func (r LocalExpnIdRemapper) Len() uint { return r.data.localExpnData.Len() }
func (r LocalExpnIdRemapper) FirstNonDetIndexMut() *Option[LocalExpnId] {
	return &r.data.firstNonDetLocalExpnId
}
func (r LocalExpnIdRemapper) FirstNonDetIndex(ctx rustfmt.Context) uint32 {
	value, ok := r.data.firstNonDetLocalExpnId.Get()
	if !ok {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "must be set at this point", Kind: rustfmt.PanicOwnedString, CanUnwind: true, Location: ctx.SourcePanicLocation("rustc_span::hygiene::LocalExpnIdRemapper.first_non_det_index.unset")})
	}
	return value.AsU32()
}
func (r LocalExpnIdRemapper) StableHash(id LocalExpnId, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
	id.ToExpnId().stableHashInternal(r.session, r.data, hcx, h, ctx)
}
func (r LocalExpnIdRemapper) CommitEndOfDeterminism(ctx rustfmt.Context) {
	CommitHygieneEndOfDeterminismDefault[LocalExpnId](r, LocalExpnIdFromU32WithContext, ctx)
}
func WithSyntaxContextRemapper[R any](session *SessionGlobals, callback func(SyntaxContextRemapper) R) R {
	return WithSyntaxContextRemapperOwned(session, hashmap.PlainOnce(func(r SyntaxContextRemapper, _ rustfmt.Context) R { return callback(r) }), rustfmt.Context{})
}
func WithSyntaxContextRemapperOwned[R any](session *SessionGlobals, callback hashmap.Once[SyntaxContextRemapper, R], ctx rustfmt.Context) R {
	defer hygieneDropOnReturn(callback.Close, ctx)
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return callback.Call(SyntaxContextRemapper{g.Get(), session}, ctx)
}
func WithLocalExpnIdRemapper[R any](session *SessionGlobals, callback func(LocalExpnIdRemapper) R) R {
	return WithLocalExpnIdRemapperOwned(session, hashmap.PlainOnce(func(r LocalExpnIdRemapper, _ rustfmt.Context) R { return callback(r) }), rustfmt.Context{})
}
func WithLocalExpnIdRemapperOwned[R any](session *SessionGlobals, callback hashmap.Once[LocalExpnIdRemapper, R], ctx rustfmt.Context) R {
	defer hygieneDropOnReturn(callback.Close, ctx)
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return callback.Call(LocalExpnIdRemapper{g.Get(), session}, ctx)
}
