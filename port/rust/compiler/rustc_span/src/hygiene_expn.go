// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

func FreshEmptyLocalExpnId(session *SessionGlobals) LocalExpnId {
	return FreshEmptyLocalExpnIdWithContext(session, rustfmt.Context{})
}
func FreshEmptyLocalExpnIdWithContext(session *SessionGlobals, ctx rustfmt.Context) LocalExpnId {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	id := LocalExpnIdFromUsizeWithContext(d.localExpnData.Len(), ctx)
	d.localExpnData.Push(None[ExpnData]())
	other := LocalExpnIdFromUsizeWithContext(d.localExpnHashes.Len(), ctx)
	d.localExpnHashes.Push(ExpnHash{})
	if d.config.DebugAssertions && id != other {
		hygieneEqualAssertion(ctx, HygienePanicSite(220, 13), id.WriteDebugFormatter, other.WriteDebugFormatter)
	}
	return id
}
func (id ExpnId) ExpnHash(session *SessionGlobals) ExpnHash {
	return id.ExpnHashWithContext(session, rustfmt.Context{})
}
func (id ExpnId) ExpnHashWithContext(session *SessionGlobals, ctx rustfmt.Context) ExpnHash {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().expnHash(id, ctx)
}
func ExpnIdFromHash(session *SessionGlobals, hash ExpnHash) Option[ExpnId] {
	return ExpnIdFromHashWithContext(session, hash, rustfmt.Context{})
}
func ExpnIdFromHashWithContext(session *SessionGlobals, hash ExpnHash, ctx rustfmt.Context) Option[ExpnId] {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	if value, ok := g.Get().expnHashToExpnId.Get(&hash, ctx); ok {
		return Some(*value)
	}
	return None[ExpnId]()
}
func (id ExpnId) ExpnData(session *SessionGlobals) ExpnData {
	return id.ExpnDataWithContext(session, rustfmt.Context{})
}
func (id ExpnId) ExpnDataWithContext(session *SessionGlobals, ctx rustfmt.Context) ExpnData {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().expnData(id, ctx).CloneWithContext(ctx)
}
func (id LocalExpnId) ExpnData(session *SessionGlobals) ExpnData {
	return id.ExpnDataWithContext(session, rustfmt.Context{})
}
func (id LocalExpnId) ExpnDataWithContext(session *SessionGlobals, ctx rustfmt.Context) ExpnData {
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().localData(id, ctx).CloneWithContext(ctx)
}
func (id ExpnId) IsDescendantOf(session *SessionGlobals, ancestor ExpnId) bool {
	return id.IsDescendantOfWithContext(session, ancestor, rustfmt.Context{})
}
func (id ExpnId) IsDescendantOfWithContext(session *SessionGlobals, ancestor ExpnId, ctx rustfmt.Context) bool {
	if ancestor == RootExpnId() || ancestor == id {
		return true
	}
	if ancestor.Krate != id.Krate {
		return false
	}
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	return g.Get().isDescendantOf(id, ancestor, ctx)
}
func (id LocalExpnId) IsDescendantOf(session *SessionGlobals, ancestor LocalExpnId) bool {
	return id.ToExpnId().IsDescendantOf(session, ancestor.ToExpnId())
}
func (id LocalExpnId) IsDescendantOfWithContext(session *SessionGlobals, ancestor LocalExpnId, ctx rustfmt.Context) bool {
	return id.ToExpnId().IsDescendantOfWithContext(session, ancestor.ToExpnId(), ctx)
}
func (id ExpnId) OuterExpnIsDescendantOf(session *SessionGlobals, ctxt SyntaxContext) bool {
	return id.OuterExpnIsDescendantOfWithContext(session, ctxt, rustfmt.Context{})
}
func (id ExpnId) OuterExpnIsDescendantOfWithContext(session *SessionGlobals, ctxt SyntaxContext, ctx rustfmt.Context) bool {
	if ctxt.IsRoot() {
		return true
	}
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	d := g.Get()
	return d.isDescendantOf(id, d.outerExpn(ctxt, ctx), ctx)
}
func (id ExpnId) ExpansionCause(session *SessionGlobals) Option[Span] {
	return id.ExpansionCauseWithContext(session, rustfmt.Context{})
}
func (id ExpnId) ExpansionCauseWithContext(session *SessionGlobals, ctx rustfmt.Context) Option[Span] {
	last := None[Span]()
	for id != RootExpnId() {
		stop := false
		func() {
			data := id.ExpnDataWithContext(session, ctx)
			defer hygieneDropOnReturn(data.CloseWithContext, ctx)
			if data.Kind.Equal(ExpnKind{Tag: ExpnMacro, MacroKind: BangMacro, Name: SymInclude()}) {
				stop = true
				return
			}
			id = data.CallSite.Ctxt().OuterExpnWithContext(session, ctx)
			last = Some(data.CallSite)
		}()
		if stop {
			break
		}
	}
	return last
}
func (id LocalExpnId) ExpansionCause(session *SessionGlobals) Option[Span] {
	return id.ToExpnId().ExpansionCause(session)
}
func (id LocalExpnId) ExpansionCauseWithContext(session *SessionGlobals, ctx rustfmt.Context) Option[Span] {
	return id.ToExpnId().ExpansionCauseWithContext(session, ctx)
}

func RegisterLocalExpnId(session *SessionGlobals, data ExpnData, hash ExpnHash) ExpnId {
	return RegisterLocalExpnIdWithContext(session, data, hash, rustfmt.Context{})
}
func RegisterLocalExpnIdWithContext(session *SessionGlobals, data ExpnData, hash ExpnHash, ctx rustfmt.Context) ExpnId {
	live := true
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if live {
			data.CloseWithContext(c)
		}
	}, ctx)
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	// The source FnOnce owns its captured input while inside HygieneData::with.
	// If it fails before moving the data into storage, Drop occurs before the
	// temporary hygiene lock guard is released.
	capturedLive := true
	live = false
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if capturedLive {
			data.CloseWithContext(c)
		}
	}, ctx)
	d := g.Get()
	id := LocalExpnIdFromUsizeWithContext(d.localExpnData.Len(), ctx)
	d.localExpnData.Push(Some(data))
	capturedLive = false
	other := LocalExpnIdFromUsizeWithContext(d.localExpnHashes.Len(), ctx)
	d.localExpnHashes.Push(hash)
	if d.config.DebugAssertions && id != other {
		hygieneEqualAssertion(ctx, HygienePanicSite(1525, 9), id.WriteDebugFormatter, other.WriteDebugFormatter)
	}
	_, old := d.expnHashToExpnId.Insert(hash, id.ToExpnId(), ctx)
	if d.config.DebugAssertions && old {
		hygieneStaticPanic(ctx, 1530, 9, "assertion failed: _old_id.is_none()")
	}
	return id.ToExpnId()
}
func RegisterExpnId(session *SessionGlobals, crate CrateNum, index ExpnIndex, data ExpnData, hash ExpnHash) ExpnId {
	return RegisterExpnIdWithContext(session, crate, index, data, hash, rustfmt.Context{})
}
func RegisterExpnIdWithContext(session *SessionGlobals, crate CrateNum, index ExpnIndex, data ExpnData, hash ExpnHash, ctx rustfmt.Context) ExpnId {
	live := true
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if live {
			data.CloseWithContext(c)
		}
	}, ctx)
	// This check precedes the hygiene lock in the source.
	config := sessionHygieneConfig(session)
	if config.DebugAssertions && data.Parent != RootExpnId() && crate != data.Parent.Krate {
		hygieneStaticPanic(ctx, 1542, 5, "assertion failed: data.parent == ExpnId::root() || krate == data.parent.krate")
	}
	id := ExpnId{crate, index}
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	// The source FnOnce owns its captured input while inside HygieneData::with.
	// If it fails before moving the data into storage, Drop occurs before the
	// temporary hygiene lock guard is released.
	capturedLive := true
	live = false
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if capturedLive {
			data.CloseWithContext(c)
		}
	}, ctx)
	d := g.Get()
	oldData, hadData := d.foreignExpnData.Insert(id, data, ctx)
	capturedLive = false
	// _old_data remains live until the closure exits, including assertion panic.
	if hadData {
		defer hygieneDropOnReturn(oldData.CloseWithContext, ctx)
	}
	oldHash, hadHash := d.foreignExpnHashes.Insert(id, hash, ctx)
	if config.DebugAssertions && hadHash && oldHash != hash {
		hygieneStaticPanic(ctx, 1547, 9, "assertion failed: _old_hash.is_none() || _old_hash == Some(hash)")
	}
	oldID, hadID := d.expnHashToExpnId.Insert(hash, id, ctx)
	if config.DebugAssertions && hadID && oldID != id {
		hygieneStaticPanic(ctx, 1549, 9, "assertion failed: _old_id.is_none() || _old_id == Some(expn_id)")
	}
	return id
}

// A session's build profile is immutable, so selecting it does not borrow its
// hygiene Lock (source checks can occur before or while that lock is held).
// Cargo's SessionGlobals construction initializes this companion field.
func sessionHygieneConfig(session *SessionGlobals) HygieneConfig {
	if session == nil {
		return DefaultHygieneConfig()
	}
	return HygieneConfig{session.spanConfig.DebugAssertions, session.spanConfig.OverflowChecks}
}

func hygieneAssertControls(hcx datastructures.StableHashCtxt, message string, ctx rustfmt.Context) {
	if typed, ok := hcx.(interface{ AssertDefaultStableHashControlsWithContext(string, rustfmt.Context) }); ok {
		typed.AssertDefaultStableHashControlsWithContext(message, ctx)
		return
	}
	defer func() {
		if p := recover(); p != nil {
			ctx.ResumeUnwind(p)
		}
	}()
	hcx.AssertDefaultStableHashControls(message)
}
func (id ExpnId) stableHashInternal(session *SessionGlobals, data *HygieneData, hcx datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ctx rustfmt.Context) {
	hasher.SetContext(ctx)
	hygieneAssertControls(hcx, "ExpnId", ctx)
	hash := datastructures.ZeroFingerprint()
	if id != RootExpnId() {
		if data != nil {
			hash = data.expnHash(id, ctx).fingerprint
		} else {
			hash = id.ExpnHashWithContext(session, ctx).fingerprint
		}
	}
	hash.StableHash(hcx, hasher)
}
func (id ExpnId) StableHash(session *SessionGlobals, hcx datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	id.StableHashWithContext(session, hcx, hasher, rustfmt.Context{})
}
func (id ExpnId) StableHashWithContext(session *SessionGlobals, hcx datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ctx rustfmt.Context) {
	id.stableHashInternal(session, nil, hcx, hasher, ctx)
}
func (id LocalExpnId) StableHash(session *SessionGlobals, hcx datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	id.ToExpnId().StableHash(session, hcx, hasher)
}
func (id LocalExpnId) StableHashWithContext(session *SessionGlobals, hcx datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ctx rustfmt.Context) {
	id.ToExpnId().StableHashWithContext(session, hcx, hasher, ctx)
}
func (c SyntaxContext) stableHashInternal(session *SessionGlobals, data *HygieneData, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
	h.SetContext(ctx)
	if c.IsRoot() {
		h.WriteU8(1)
		return
	}
	h.WriteU8(0)
	hygieneAssertControls(hcx, "ExpnId", ctx)
	var hash ExpnHash
	var t Transparency
	if data != nil {
		mark := data.outerMark(c, ctx)
		hash = data.expnHash(mark.Expansion, ctx)
		t = mark.Transparency
	} else {
		func() {
			g := hygieneBorrow(session, ctx)
			defer g.Close()
			d := g.Get()
			mark := d.outerMark(c, ctx)
			hash = d.expnHash(mark.Expansion, ctx)
			t = mark.Transparency
		}()
	}
	hash.fingerprint.StableHash(hcx, h)
	t.StableHash(hcx, h)
}
func (c SyntaxContext) StableHash(session *SessionGlobals, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	c.StableHashWithContext(session, hcx, h, rustfmt.Context{})
}
func (c SyntaxContext) StableHashWithContext(session *SessionGlobals, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
	c.stableHashInternal(session, nil, hcx, h, ctx)
}

func (v ExpnKind) StableHash(session *SessionGlobals, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	v.StableHashWithContext(session, hcx, h, rustfmt.Context{})
}
func (v ExpnKind) StableHashWithContext(session *SessionGlobals, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
	h.SetContext(ctx)
	h.WriteU8(uint8(v.Tag))
	switch v.Tag {
	case ExpnRoot:
	case ExpnMacro:
		v.MacroKind.StableHash(hcx, h)
		v.Name.StableHashWithContext(session, hcx, h, ctx)
	case ExpnAstPass:
		v.AstPass.StableHash(hcx, h)
	case ExpnDesugaring:
		v.Desugaring.StableHash(hcx, h)
	default:
		ctx.Panic("invalid Go-only ExpnKind")
	}
}
func (v *ExpnData) StableHash(session *SessionGlobals, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	v.StableHashWithContext(session, hcx, h, rustfmt.Context{})
}
func (v *ExpnData) StableHashWithContext(session *SessionGlobals, hcx datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
	h.SetContext(ctx)
	v.Kind.StableHashWithContext(session, hcx, h, ctx)
	v.Parent.StableHashWithContext(session, hcx, h, ctx)
	datastructures.StableHashSpanWithContext(hcx, v.CallSite.ToRawSpan(), h, ctx)
	h.WriteU32(v.disambiguator)
	datastructures.StableHashSpanWithContext(hcx, v.DefSite.ToRawSpan(), h, ctx)
	if shared, ok := v.AllowInternalUnstable.Get(); ok {
		h.WriteU8(1)
		symbols := shared.BorrowWithContext(ctx)
		h.WriteUsize(uint(len(*symbols)))
		for _, symbol := range *symbols {
			symbol.StableHashWithContext(session, hcx, h, ctx)
		}
	} else {
		h.WriteU8(0)
	}
	v.Edition.StableHash(hcx, h)
	if id, ok := v.MacroDefID.Get(); ok {
		h.WriteU8(1)
		id.StableHashWithContext(hcx, h, ctx)
	} else {
		h.WriteU8(0)
	}
	if id, ok := v.ParentModule.Get(); ok {
		h.WriteU8(1)
		id.StableHashWithContext(hcx, h, ctx)
	} else {
		h.WriteU8(0)
	}
	for _, value := range [...]bool{v.AllowInternalUnsafe, v.LocalInnerMacros, v.CollapseDebuginfo, v.DiagnosticOpaque} {
		if value {
			h.WriteU8(1)
		} else {
			h.WriteU8(0)
		}
	}
}
func (v *ExpnData) hashExpn(session *SessionGlobals, hcx datastructures.StableHashCtxt, ctx rustfmt.Context) rustchashes.Hash64 {
	h := datastructures.NewStableHasher()
	v.StableHashWithContext(session, hcx, h, ctx)
	return rustchashes.Hash64FromStableHash(rustchashes.StableHasherHash(h.FinishHash()))
}
func updateDisambiguator(session *SessionGlobals, data *ExpnData, hcx datastructures.StableHashCtxt, ctx rustfmt.Context) ExpnHash {
	if data.disambiguator != 0 {
		hygieneFormattedPanic(ctx, func(f *rustfmt.Formatter) error {
			if err := f.WriteString("assertion `left == right` failed: Already set disambiguator for ExpnData: "); err != nil {
				return err
			}
			if err := data.WriteDebugFormatter(session, f); err != nil {
				return err
			}
			if err := f.WriteString("\n  left: "); err != nil {
				return err
			}
			if err := rustfmt.FormatDebugUint(uint64(data.disambiguator), 32, f); err != nil {
				return err
			}
			return f.WriteString("\n right: 0")
		})
	}
	hygieneAssertControls(hcx, "ExpnData (disambiguator)", ctx)
	hash := data.hashExpn(session, hcx, ctx)
	var disambiguator uint32
	func() {
		g := hygieneBorrow(session, ctx)
		defer g.Close()
		d := g.Get()
		entry := d.expnDataDisambiguators.Entry(hash, ctx)
		disambig := entry.OrInsert(0, ctx)
		disambiguator = *disambig
		if d.config.OverflowChecks && *disambig == ^uint32(0) {
			hygieneStaticPanic(ctx, 1665, 9, "attempt to add with overflow")
		}
		*disambig++
	}()
	if disambiguator != 0 {
		hygieneTraceMessage(ctx, hygieneTraceDisambiguator, func(f *rustfmt.Formatter) error {
			return hygieneMessage(f, []string{"Set disambiguator for expn_data=", " expn_hash=", ""}, func(f *rustfmt.Formatter) error { return data.WriteDebugFormatter(session, f) }, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(hash.AsU64(), 64, f) })
		})
		data.disambiguator = disambiguator
		hash = data.hashExpn(session, hcx, ctx)
		if sessionHygieneConfig(session).DebugAssertions {
			func() {
				g := hygieneBorrow(session, ctx)
				defer g.Close()
				if value, ok := g.Get().expnDataDisambiguators.Get(&hash, ctx); ok {
					hygieneFormattedPanicAt(ctx, HygienePanicSite(1678, 13), func(f *rustfmt.Formatter) error {
						if err := f.WriteString("assertion `left == right` failed: Hash collision after disambiguator update!\n  left: Some("); err != nil {
							return err
						}
						if err := rustfmt.FormatDebugUint(uint64(*value), 32, f); err != nil {
							return err
						}
						return f.WriteString(")\n right: None")
					})
				}
			}()
		}
	}
	return newExpnHash(LocalCrate().AsDefId().ToStableHashKeyWithContext(hcx, ctx).StableCrateId(), hash)
}
func FreshLocalExpnId(session *SessionGlobals, data ExpnData, hcx datastructures.StableHashCtxt) LocalExpnId {
	return FreshLocalExpnIdWithContext(session, data, hcx, rustfmt.Context{})
}
func FreshLocalExpnIdWithContext(session *SessionGlobals, data ExpnData, hcx datastructures.StableHashCtxt, ctx rustfmt.Context) LocalExpnId {
	return freshLocalExpnId(session, data, func(data *ExpnData) ExpnHash { return updateDisambiguator(session, data, hcx, ctx) }, nil, ctx)
}
func freshLocalExpnId(session *SessionGlobals, data ExpnData, hashData func(*ExpnData) ExpnHash, dropContext func(rustfmt.Context), ctx rustfmt.Context) LocalExpnId {
	live := true
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if live {
			data.CloseWithContext(c)
		}
	}, ctx)
	if dropContext != nil {
		defer hygieneDropOnReturn(dropContext, ctx)
	}
	if sessionHygieneConfig(session).DebugAssertions && data.Parent.Krate != LocalCrate() {
		hygieneParentLocalAssertion(data.Parent.Krate, ctx, "rustc_span::hygiene::fresh.parent_local")
	}
	hash := hashData(&data)
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	// The source FnOnce owns its captured input while inside HygieneData::with.
	// If it fails before moving the data into storage, Drop occurs before the
	// temporary hygiene lock guard is released.
	capturedLive := true
	live = false
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if capturedLive {
			data.CloseWithContext(c)
		}
	}, ctx)
	d := g.Get()
	id := LocalExpnIdFromUsizeWithContext(d.localExpnData.Len(), ctx)
	d.localExpnData.Push(Some(data))
	capturedLive = false
	other := LocalExpnIdFromUsizeWithContext(d.localExpnHashes.Len(), ctx)
	d.localExpnHashes.Push(hash)
	if d.config.DebugAssertions && id != other {
		hygieneEqualAssertion(ctx, HygienePanicSite(231, 13), id.WriteDebugFormatter, other.WriteDebugFormatter)
	}
	_, had := d.expnHashToExpnId.Insert(hash, id.ToExpnId(), ctx)
	if d.config.DebugAssertions && had {
		hygieneStaticPanic(ctx, 233, 13, "assertion failed: _old_id.is_none()")
	}
	return id
}

// FreshLocalExpnIdOwned consumes the actual stable-hash context cell. Source
// drops it at update_disambiguator's return, before inserting expansion data.
func FreshLocalExpnIdOwned[S any](session *SessionGlobals, data ExpnData, owned *rustvec.Owned[S], ops datastructures.StableHashCtxtOps[S], ambient ...rustfmt.Context) LocalExpnId {
	ctx := hygieneOwnedContext(owned.Context(), ambient)
	moved := owned.MoveWithContext(ctx)
	return freshLocalExpnId(session, data, func(data *ExpnData) ExpnHash {
		defer hygieneDropOnReturn(moved.DropWithContext, ctx)
		hcx := datastructures.NewStableHashCtxtRef(moved.Borrow(), ops, ctx)
		return updateDisambiguator(session, data, hcx, ctx)
	}, moved.DropWithContext, ctx)
}
func (id LocalExpnId) SetExpnData(session *SessionGlobals, data ExpnData, hcx datastructures.StableHashCtxt) {
	id.SetExpnDataWithContext(session, data, hcx, rustfmt.Context{})
}
func (id LocalExpnId) SetExpnDataWithContext(session *SessionGlobals, data ExpnData, hcx datastructures.StableHashCtxt, ctx rustfmt.Context) {
	setExpnData(session, id, data, func(data *ExpnData) ExpnHash { return updateDisambiguator(session, data, hcx, ctx) }, nil, ctx)
}
func setExpnData(session *SessionGlobals, id LocalExpnId, data ExpnData, hashData func(*ExpnData) ExpnHash, dropContext func(rustfmt.Context), ctx rustfmt.Context) {
	live := true
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if live {
			data.CloseWithContext(c)
		}
	}, ctx)
	if dropContext != nil {
		defer hygieneDropOnReturn(dropContext, ctx)
	}
	if sessionHygieneConfig(session).DebugAssertions && data.Parent.Krate != LocalCrate() {
		hygieneParentLocalAssertion(data.Parent.Krate, ctx, "rustc_span::hygiene::set_expn_data.parent_local")
	}
	hash := hashData(&data)
	g := hygieneBorrow(session, ctx)
	defer g.Close()
	// The source FnOnce owns its captured input while inside HygieneData::with.
	// If it fails before moving the data into storage, Drop occurs before the
	// temporary hygiene lock guard is released.
	capturedLive := true
	live = false
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if capturedLive {
			data.CloseWithContext(c)
		}
	}, ctx)
	d := g.Get()
	oldData := hygieneAt(d.localExpnData, id, ctx, hygienePoint(253, 58))
	if oldData.present {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "expansion data is reset for an expansion ID", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation("rustc_span::hygiene::set_expn_data.reset")})
	}
	*oldData = Some(data)
	capturedLive = false
	oldHash := hygieneAt(d.localExpnHashes, id, ctx)
	if d.config.DebugAssertions && !oldHash.IsRoot() {
		hygieneEqualAssertion(ctx, HygienePanicSite(256, 13), oldHash.fingerprint.WriteDebugFormatter, datastructures.ZeroFingerprint().WriteDebugFormatter)
	}
	*oldHash = hash
	_, had := d.expnHashToExpnId.Insert(hash, id.ToExpnId(), ctx)
	if d.config.DebugAssertions && had {
		hygieneStaticPanic(ctx, 259, 13, "assertion failed: _old_id.is_none()")
	}
}
func SetExpnDataOwned[S any](session *SessionGlobals, id LocalExpnId, data ExpnData, owned *rustvec.Owned[S], ops datastructures.StableHashCtxtOps[S], ambient ...rustfmt.Context) {
	ctx := hygieneOwnedContext(owned.Context(), ambient)
	moved := owned.MoveWithContext(ctx)
	setExpnData(session, id, data, func(data *ExpnData) ExpnHash {
		defer hygieneDropOnReturn(moved.DropWithContext, ctx)
		hcx := datastructures.NewStableHashCtxtRef(moved.Borrow(), ops, ctx)
		return updateDisambiguator(session, data, hcx, ctx)
	}, moved.DropWithContext, ctx)
}
