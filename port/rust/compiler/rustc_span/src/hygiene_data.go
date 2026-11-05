// SPDX-License-Identifier: MIT OR Apache-2.0
// Source hygiene storage and context-chain algorithms. Source/API inventory
// and native comparisons live in tools/port-rust-hygiene.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

type SyntaxContextKey struct {
	Parent       SyntaxContext
	Expansion    ExpnId
	Transparency Transparency
}
type syntaxContextData struct {
	outerExpn                           ExpnId
	outerTransparency                   Transparency
	parent, opaque, opaqueAndSemiopaque SyntaxContext
	dollarCrateName                     Symbol
}

func rootSyntaxContextData() syntaxContextData {
	return syntaxContextData{outerExpn: RootExpnId(), outerTransparency: Opaque, dollarCrateName: KwDollarCrate()}
}
func (d syntaxContextData) key() SyntaxContextKey {
	return SyntaxContextKey{d.parent, d.outerExpn, d.outerTransparency}
}

type HygieneData struct {
	localExpnData            *index.IndexVec[LocalExpnId, Option[ExpnData]]
	localExpnHashes          *index.IndexVec[LocalExpnId, ExpnHash]
	foreignExpnData          *hashmap.Map[ExpnId, ExpnData]
	foreignExpnHashes        *hashmap.Map[ExpnId, ExpnHash]
	expnHashToExpnId         *hashmap.Map[ExpnHash, ExpnId]
	syntaxContextData        *index.IndexVec[SyntaxContext, syntaxContextData]
	firstNonDetSyntaxContext Option[SyntaxContext]
	firstNonDetLocalExpnId   Option[LocalExpnId]
	syntaxContextMap         *hashmap.Map[SyntaxContextKey, SyntaxContext]
	expnDataDisambiguators   *hashmap.Map[rustchashes.Hash64, uint32]
	config                   HygieneConfig
	// Temporary operation context, installed under the exclusive hygiene lock.
	// This never captures a construction-time Scope for subsequent operations.
	operationContext *rustfmt.Context
}

// The source UnhashMap really builds Unhasher, including Fingerprint's concrete
// type specialization. Wrapping the hasher would change that dispatch.
type hygieneUnhashBuilder struct{ debug bool }

func (b hygieneUnhashBuilder) Build(ctx hashmap.Context) *hashmap.OwnedHasher {
	return hashmap.NewOwnedHasher(datastructures.NewUnhasherWithContext(ctx, b.debug), nil)
}
func (b hygieneUnhashBuilder) Clone(hashmap.Context) hashmap.Builder          { return b }
func (b hygieneUnhashBuilder) CloneFrom(_ hashmap.Builder, _ hashmap.Context) {}
func (b hygieneUnhashBuilder) Close(hashmap.Context)                          {}
func hygieneExpnKeyOps() hashmap.KeyOps[ExpnId] {
	return hashmap.KeyOps[ExpnId]{Hash: func(v *ExpnId, h hashmap.Hasher, _ hashmap.Context) {
		hashmap.HashU32(h, v.Krate.AsU32())
		hashmap.HashU32(h, v.LocalID.AsU32())
	}, Equal: func(a, b *ExpnId, _ hashmap.Context) bool { return *a == *b }, TrivialClone: true}
}
func hygieneContextKeyOps() hashmap.KeyOps[SyntaxContextKey] {
	return hashmap.KeyOps[SyntaxContextKey]{Hash: func(v *SyntaxContextKey, h hashmap.Hasher, _ hashmap.Context) {
		hashmap.HashU32(h, v.Parent.AsU32())
		hashmap.HashU32(h, v.Expansion.Krate.AsU32())
		hashmap.HashU32(h, v.Expansion.LocalID.AsU32())
		hashmap.HashIsize(h, int(v.Transparency))
	}, Equal: func(a, b *SyntaxContextKey, _ hashmap.Context) bool { return *a == *b }, TrivialClone: true}
}
func hygieneHashKeyOps() hashmap.KeyOps[ExpnHash] {
	return hashmap.KeyOps[ExpnHash]{Hash: func(v *ExpnHash, h hashmap.Hasher, _ hashmap.Context) {
		if unhash, ok := h.(*datastructures.Unhasher); ok {
			v.Hash(unhash)
			return
		}
		a, b := v.fingerprint.Split()
		hashmap.HashU64(h, a.AsU64())
		hashmap.HashU64(h, b.AsU64())
	}, Equal: func(a, b *ExpnHash, _ hashmap.Context) bool { return *a == *b }, TrivialClone: true}
}
func hygieneHash64KeyOps() hashmap.KeyOps[rustchashes.Hash64] {
	return hashmap.KeyOps[rustchashes.Hash64]{Hash: func(v *rustchashes.Hash64, h hashmap.Hasher, _ hashmap.Context) { hashmap.HashU64(h, v.AsU64()) }, Equal: func(a, b *rustchashes.Hash64, _ hashmap.Context) bool { return *a == *b }, TrivialClone: true}
}
func newHygieneData(edition Edition) HygieneData {
	return newHygieneDataWithConfig(edition, DefaultHygieneConfig())
}
func newHygieneDataWithConfig(edition Edition, config HygieneConfig) HygieneData {
	operationContext := &rustfmt.Context{}
	profile := hashmap.DefaultProfile()
	fx := hashmap.FxBuilder{PointerBits: profile.PointerBits}
	unhash := hygieneUnhashBuilder{config.DebugAssertions}
	dataOps := hashmap.ValueOps[ExpnData]{Clone: func(v *ExpnData, c hashmap.Context) ExpnData { return v.CloneWithContext(c) }, Drop: func(v *ExpnData, c hashmap.Context) { v.CloseWithContext(c) }}
	root := Some(DefaultExpnData(ExpnKind{Tag: ExpnRoot}, DummySpan(), edition, Some(CrateDefId().ToDefId()), None[ModId]()))
	d := HygieneData{
		localExpnData: index.IndexVecFromElemN(root, 1, LocalExpnIdIdx(), index.ValueOps[Option[ExpnData]]{CloneRef: func(v *Option[ExpnData]) Option[ExpnData] {
			if v.present {
				return Some(v.value.CloneWithContext(*operationContext))
			}
			return None[ExpnData]()
		}, DropWithContext: func(v *Option[ExpnData], c index.IndexDropContext) {
			if v.present {
				ctx := *operationContext
				ctx.Unwinding = ctx.Unwinding || c.Unwinding
				v.value.CloseWithContext(ctx)
			}
		}, RustSize: 80}),
		localExpnHashes:        index.IndexVecFromElemN(ExpnHash{}, 1, LocalExpnIdIdx(), index.ValueOps[ExpnHash]{Copy: true, TrivialClone: true, RustSize: 16}),
		foreignExpnData:        hashmap.NewMap(hashmap.Layout{Size: 88, Align: 8}, profile, hygieneExpnKeyOps(), dataOps, fx),
		foreignExpnHashes:      hashmap.NewMap(hashmap.Layout{Size: 24, Align: 8}, profile, hygieneExpnKeyOps(), hashmap.ValueOps[ExpnHash]{TrivialClone: true}, fx),
		expnHashToExpnId:       hashmap.NewMap(hashmap.Layout{Size: 24, Align: 8}, profile, hygieneHashKeyOps(), hashmap.ValueOps[ExpnId]{TrivialClone: true}, unhash),
		syntaxContextData:      index.IndexVecFromElemN(rootSyntaxContextData(), 1, SyntaxContextIdx(), index.ValueOps[syntaxContextData]{Copy: true, TrivialClone: true, RustSize: 28}),
		syntaxContextMap:       hashmap.NewMap(hashmap.Layout{Size: 20, Align: 4}, profile, hygieneContextKeyOps(), hashmap.ValueOps[SyntaxContext]{TrivialClone: true}, fx),
		expnDataDisambiguators: hashmap.NewMap(hashmap.Layout{Size: 16, Align: 8}, profile, hygieneHash64KeyOps(), hashmap.ValueOps[uint32]{TrivialClone: true}, unhash), config: config, operationContext: operationContext,
	}
	d.expnHashToExpnId.Insert(ExpnHash{}, RootExpnId(), rustfmt.Context{})
	d.syntaxContextMap.Insert(rootSyntaxContextData().key(), RootSyntaxContext(), rustfmt.Context{})
	return d
}

type hygieneGuard struct {
	guard    *LockGuard[HygieneData]
	previous rustfmt.Context
	closed   bool
}

func (g *hygieneGuard) Get() *HygieneData { return g.guard.Get() }
func (g *hygieneGuard) Close() {
	if g.closed {
		return
	}
	g.closed = true
	*g.guard.Get().operationContext = g.previous
	g.guard.Close()
}
func hygieneBorrow(session *SessionGlobals, ctx rustfmt.Context) *hygieneGuard {
	if session == nil || session.hygieneData == nil {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "cannot access a scoped thread local variable without calling `set` first", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation(ScopedSessionPanicSite)})
	}
	guard := session.hygieneData.BorrowMutWithContext(ctx)
	cell := guard.Get().operationContext
	previous := *cell
	*cell = ctx
	return &hygieneGuard{guard: guard, previous: previous}
}
func hygieneAt[I, T any](v *index.IndexVec[I, T], key I, ctx rustfmt.Context, points ...hygieneCodePoint) *T {
	if value, ok := v.Get(key); ok {
		return value
	}
	point := hygieneCodePoint{}
	if len(points) > 0 {
		point = points[0]
	}
	hygieneBoundsPanic(ctx, point, v.Len(), v.Idx.Index(key))
	return nil
}
func (d *HygieneData) contextData(ctxt SyntaxContext, ctx rustfmt.Context, points ...hygieneCodePoint) *syntaxContextData {
	return hygieneAt(d.syntaxContextData, ctxt, ctx, points...)
}
func (d *HygieneData) expnHash(id ExpnId, ctx rustfmt.Context) ExpnHash {
	if local, ok := id.AsLocal().Get(); ok {
		return *hygieneAt(d.localExpnHashes, local, ctx, hygienePoint(561, 52))
	}
	if hash, ok := d.foreignExpnHashes.Get(&id, ctx); ok {
		return *hash
	}
	hygieneOwnedPanic(ctx, 562, 45, "no entry found for key")
	return ExpnHash{}
}
func (d *HygieneData) localData(id LocalExpnId, ctx rustfmt.Context) *ExpnData {
	value := hygieneAt(d.localExpnData, id, ctx, hygienePoint(568, 29))
	if value.present {
		return &value.value
	}
	hygieneOwnedPanic(ctx, 568, 48, "no expansion data for an expansion ID")
	return nil
}
func (d *HygieneData) expnData(id ExpnId, ctx rustfmt.Context) *ExpnData {
	if local, ok := id.AsLocal().Get(); ok {
		value := hygieneAt(d.localExpnData, local, ctx, hygienePoint(573, 33))
		if value.present {
			return &value.value
		}
		hygieneOwnedPanic(ctx, 573, 52, "no expansion data for an expansion ID")
	}
	if data, ok := d.foreignExpnData.Get(&id, ctx); ok {
		return data
	}
	hygieneOwnedPanic(ctx, 575, 36, "no entry found for key")
	return nil
}
func (d *HygieneData) isDescendantOf(id, ancestor ExpnId, ctx rustfmt.Context) bool {
	if ancestor == RootExpnId() {
		return true
	}
	if id.Krate != ancestor.Krate {
		return false
	}
	for {
		if id == ancestor {
			return true
		}
		if id == RootExpnId() {
			return false
		}
		id = d.expnData(id, ctx).Parent
	}
}
func (d *HygieneData) outerExpn(ctxt SyntaxContext, ctx rustfmt.Context) ExpnId {
	return d.contextData(ctxt, ctx, hygienePoint(611, 33)).outerExpn
}
func (d *HygieneData) normalizeMacros20(ctxt SyntaxContext, ctx rustfmt.Context) SyntaxContext {
	return d.contextData(ctxt, ctx, hygienePoint(600, 33)).opaque
}
func (d *HygieneData) normalizeMacroRules(ctxt SyntaxContext, ctx rustfmt.Context) SyntaxContext {
	return d.contextData(ctxt, ctx, hygienePoint(605, 33)).opaqueAndSemiopaque
}

type HygieneMark struct {
	Expansion    ExpnId
	Transparency Transparency
}

func (d *HygieneData) outerMark(ctxt SyntaxContext, ctx rustfmt.Context) HygieneMark {
	v := d.contextData(ctxt, ctx, hygienePoint(617, 45))
	return HygieneMark{v.outerExpn, v.outerTransparency}
}
func (d *HygieneData) removeMark(ctxt *SyntaxContext, ctx rustfmt.Context) HygieneMark {
	mark := d.outerMark(*ctxt, ctx)
	*ctxt = d.contextData(*ctxt, ctx).parent
	return mark
}
func (d *HygieneData) marks(ctxt SyntaxContext, ctx rustfmt.Context) []HygieneMark {
	marks := []HygieneMark{}
	for !ctxt.IsRoot() {
		hygieneTraceMessage(ctx, hygieneTraceMarks, func(f *rustfmt.Formatter) error {
			return hygieneMessage(f, []string{"marks: getting parent of ", ""}, ctxt.WriteDebugFormatter)
		})
		marks = append(marks, d.outerMark(ctxt, ctx))
		ctxt = d.contextData(ctxt, ctx).parent
	}
	for i, j := 0, len(marks)-1; i < j; i, j = i+1, j-1 {
		marks[i], marks[j] = marks[j], marks[i]
	}
	return marks
}
func (d *HygieneData) adjust(ctxt *SyntaxContext, id ExpnId, ctx rustfmt.Context) Option[ExpnId] {
	scope := None[ExpnId]()
	for !d.isDescendantOf(id, d.outerExpn(*ctxt, ctx), ctx) {
		scope = Some(d.removeMark(ctxt, ctx).Expansion)
	}
	return scope
}
func (d *HygieneData) applyMark(session *SessionGlobals, ctxt SyntaxContext, id ExpnId, transparency Transparency, ctx rustfmt.Context) SyntaxContext {
	if id == RootExpnId() {
		hygieneNotEqualAssertion(ctx, HygienePanicSite(694, 9), id.WriteDebugFormatter, RootExpnId().WriteDebugFormatter)
	}
	if transparency == Opaque {
		return d.allocCtxt(session, ctxt, id, transparency, ctx)
	}
	callCtxt := d.expnData(id, ctx).CallSite.Ctxt()
	if transparency == SemiOpaque {
		callCtxt = d.normalizeMacros20(callCtxt, ctx)
	} else {
		callCtxt = d.normalizeMacroRules(callCtxt, ctx)
	}
	if callCtxt.IsRoot() {
		return d.allocCtxt(session, ctxt, id, transparency, ctx)
	}
	for _, mark := range d.marks(ctxt, ctx) {
		callCtxt = d.allocCtxt(session, callCtxt, mark.Expansion, mark.Transparency, ctx)
	}
	return d.allocCtxt(session, callCtxt, id, transparency, ctx)
}
func (d *HygieneData) allocCtxt(session *SessionGlobals, parent SyntaxContext, id ExpnId, transparency Transparency, ctx rustfmt.Context) SyntaxContext {
	key := SyntaxContextKey{parent, id, transparency}
	if cached, ok := d.syntaxContextMap.Get(&key, ctx); ok {
		return *cached
	}
	// IndexVec validates the new typed index before mutating its backing Vec.
	ctxt := SyntaxContextFromUsizeWithContext(d.syntaxContextData.Len(), ctx)
	dummy := rootSyntaxContextData()
	dummy.dollarCrateName = SymDummy()
	d.syntaxContextData.Push(dummy)
	d.syntaxContextMap.Insert(key, ctxt, ctx)
	parentData := d.contextData(parent, ctx, hygienePoint(751, 52))
	if parentData.dollarCrateName == SymDummy() {
		hygieneNotEqualAssertion(ctx, HygienePanicSite(752, 9), func(f *rustfmt.Formatter) error { return parentData.dollarCrateName.WriteDebugFormatter(session, f) }, func(f *rustfmt.Formatter) error { return SymDummy().WriteDebugFormatter(session, f) })
	}
	opaque, semi := parentData.opaque, parentData.opaqueAndSemiopaque
	switch transparency {
	case Transparent:
	case SemiOpaque:
		semi = d.allocCtxt(session, semi, id, transparency, ctx)
	case Opaque:
		opaque = d.allocCtxt(session, opaque, id, transparency, ctx)
		semi = d.allocCtxt(session, semi, id, transparency, ctx)
	default:
		ctx.Panic("invalid Go-only Transparency")
	}
	*d.contextData(ctxt, ctx) = syntaxContextData{id, transparency, parent, opaque, semi, KwDollarCrate()}
	return ctxt
}
