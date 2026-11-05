// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"fmt"
	"strings"

	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	rv "github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// This is the actual compiler-generated FnOnce capture environment in the
// authored native caller: Env is the first capture, followed by ExpnData.
type hygieneOwnedDecodeCell struct {
	env      *rv.Owned[hygieneOwnedEnv]
	data     sp.ExpnData
	liveData bool
}

// Native call_once moves ExpnData into its return place before normal Env
// destruction. Its body-unwind edges instead drop Env with unwind-terminate,
// then the still-live ExpnData. Dropping an unused closure uses ordinary field
// drop glue. An explicit consumed-self witness expresses these distinct edges.
type hygieneDecodeOnce struct {
	state hygieneOwnedDecodeCell
	live  bool
}

func (f *hygieneDecodeOnce) Call(id sp.ExpnId, c rf.Context) (result sp.DecodedExpnData) {
	if !f.live {
		panic("consumed authored decode closure")
	}
	f.live = false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			u := c
			u.Unwinding = true
			cleanup := func(drop func(rf.Context)) {
				defer func() {
					if recover() != nil {
						u.AbortCleanup()
					}
				}()
				drop(u)
			}
			cleanup(f.state.env.DropWithContext)
			if f.state.liveData {
				f.state.liveData = false
				cleanup(f.state.data.CloseWithContext)
			}
			c.ResumeUnwind(p)
		}
		f.state.env.DropWithContext(c)
	}()
	f.state.env.Borrow().call(c, "decode", fmt.Sprintf("%d/%d", id.Krate.AsU32(), id.LocalID.AsU32()))
	f.state.liveData = false
	return sp.DecodedExpnData{Data: f.state.data, Hash: hygieneCodecHash(9, 10)}
}
func (f *hygieneDecodeOnce) Close(c rf.Context) {
	if !f.live {
		return
	}
	f.live = false
	defer hygieneOwnedClose(c, func(c rf.Context) {
		if f.state.liveData {
			f.state.liveData = false
			f.state.data.CloseWithContext(c)
		}
	})
	f.state.env.DropWithContext(c)
}

type hygieneEraseHcx struct{ hygieneCoreHcx }

func (hygieneEraseHcx) StableHashSpan(span ds.RawSpan, h *ds.StableHasher) {
	*h = *ds.NewStableHasher()
	h.WriteU64(span.Value)
}

func hygieneFailureBuild(operation string, s *hygieneOwnedCore, construction rf.Context) func(rf.Context) {
	switch operation {
	case "fresh_hash_duplicate", "set_hash_duplicate":
		data := s.data(construction)
		hasher := ds.NewStableHasher()
		data.StableHash(s.session, hygieneCoreHcx{}, hasher)
		hash := hygieneCodecHash(100, rustchashes.Hash64FromStableHash(rustchashes.StableHasherHash(hasher.FinishHash())).AsU64())
		sp.RegisterExpnId(s.session, sp.CrateNumFromU32(3), sp.ExpnIndexFromU32(17), data.CloneWithContext(construction), hash)
		id := sp.RootLocalExpnId()
		if operation == "set_hash_duplicate" {
			id = sp.FreshEmptyLocalExpnId(s.session)
		}
		env := s.newEnv(construction)
		return func(c rf.Context) {
			if operation == "set_hash_duplicate" {
				sp.SetExpnDataOwned(s.session, id, data, env, hygieneOwnedOps(), c)
				fmt.Println("returned:set_hash_duplicate")
			} else {
				id := sp.FreshLocalExpnIdOwned(s.session, data, env, hygieneOwnedOps(), c)
				fmt.Printf("returned:fresh_hash_duplicate:%d\n", id.AsU32())
			}
		}
	case "clamp_index":
		return func(c rf.Context) {
			_ = sp.ExpnIndexFromU32(17).ClampWithContext(sp.ExpnIndexFromU32(99), sp.ExpnIndexFromU32(1), c)
		}
	case "clamp_macro":
		return func(c rf.Context) { _ = sp.AttrMacro.ClampWithContext(sp.DeriveMacro, sp.BangMacro, c) }
	case "scope_missing":
		env := s.newEnv(construction)
		s.session = nil
		return func(c rf.Context) {
			defer hygieneOwnedClose(c, env.DropWithContext)
			_ = sp.FreshEmptyLocalExpnIdWithContext(nil, c)
			fmt.Println("returned:scope_missing")
		}
	case "collision_forced":
		data := s.data(construction)
		_ = sp.FreshLocalExpnId(s.session, data.CloneWithContext(construction), hygieneEraseHcx{})
		env := s.newEnv(construction)
		return func(c rf.Context) {
			ops := hygieneOwnedOps()
			ops.StableHashSpan = func(r rv.Ref[hygieneOwnedEnv], span ds.RawSpan, h *ds.StableHasher) {
				r.Value.call(r.Context, "span", fmt.Sprint(span.Value))
				*h = *ds.NewStableHasher()
				h.SetContext(r.Context)
				h.WriteU64(span.Value)
			}
			id := sp.FreshLocalExpnIdOwned(s.session, data, env, ops, c)
			fmt.Printf("returned:collision_forced:%d\n", id.AsU32())
		}
	case "fresh_parent_missing", "set_parent_missing":
		data := s.data(construction)
		data.Parent = sp.ExpnId{Krate: sp.LocalCrate(), LocalID: sp.ExpnIndexFromU32(17)}
		env := s.newEnv(construction)
		id := sp.RootLocalExpnId()
		if operation == "set_parent_missing" {
			id = sp.FreshEmptyLocalExpnId(s.session)
		}
		return func(c rf.Context) {
			if operation == "set_parent_missing" {
				sp.SetExpnDataOwned(s.session, id, data, env, hygieneOwnedOps(), c)
			} else {
				_ = sp.FreshLocalExpnIdOwned(s.session, data, env, hygieneOwnedOps(), c)
			}
			fmt.Println("returned:parent_missing")
		}
	case "context_apply_parent", "context_alloc_self":
		id := sp.FreshEmptyLocalExpnId(s.session)
		raw := uint32(17)
		if operation == "context_alloc_self" {
			raw = 1
		}
		ctxt := sp.SyntaxContextFromU32(raw)
		return func(c rf.Context) {
			_ = ctxt.ApplyMarkWithContext(s.session, id.ToExpnId(), sp.Opaque, c)
			fmt.Println("returned:context")
		}
	case "context_dollar":
		return func(c rf.Context) {
			span := sp.DummySpan().WithCtxt(s.session, sp.SyntaxContextFromU32(17))
			printer := sp.NewIdentPrinter(sp.KwDollarCrate(), sp.IdentNormal, sp.Some(span))
			var text strings.Builder
			f := rf.Formatter{Writer: &text, Options: rf.DefaultOptions(), Context: c}
			if err := printer.WriteDisplayFormatter(s.session, &f); err != nil {
				panic(err)
			}
			fmt.Println("returned:context")
		}
	case "register_local_duplicate":
		hash := hygieneCodecHash(9, 9)
		sp.RegisterLocalExpnId(s.session, s.data(construction), hash)
		data := s.data(construction)
		return func(c rf.Context) {
			id := sp.RegisterLocalExpnIdWithContext(s.session, data, hash, c)
			fmt.Printf("returned:register:%d:%d\n", id.Krate.AsU32(), id.LocalID.AsU32())
		}
	case "register_foreign_parent":
		data := s.data(construction)
		data.Parent = sp.ExpnId{Krate: sp.CrateNumFromU32(2), LocalID: sp.ExpnIndexFromU32(17)}
		return func(c rf.Context) {
			id := sp.RegisterExpnIdWithContext(s.session, sp.CrateNumFromU32(3), sp.ExpnIndexFromU32(17), data, hygieneCodecHash(9, 9), c)
			fmt.Printf("returned:register:%d:%d\n", id.Krate.AsU32(), id.LocalID.AsU32())
		}
	case "register_foreign_hash", "register_foreign_id":
		krate := sp.CrateNumFromU32(3)
		sp.RegisterExpnId(s.session, krate, sp.ExpnIndexFromU32(17), s.data(construction), hygieneCodecHash(9, 9))
		data := s.data(construction)
		return func(c rf.Context) {
			index, last := uint32(18), uint64(9)
			if operation == "register_foreign_hash" {
				index, last = 17, 10
			}
			id := sp.RegisterExpnIdWithContext(s.session, krate, sp.ExpnIndexFromU32(index), data, hygieneCodecHash(9, last), c)
			fmt.Printf("returned:register:%d:%d\n", id.Krate.AsU32(), id.LocalID.AsU32())
		}
	case "decode_root_owned", "decode_cached_owned", "decode_new_owned", "decode_local_owned", "decode_parent_owned":
		raw, crate := uint32(17), uint32(3)
		if operation == "decode_root_owned" {
			raw = 0
		}
		if operation == "decode_local_owned" {
			crate = 0
		}
		krate := sp.CrateNumFromU32(crate)
		if operation == "decode_cached_owned" {
			sp.RegisterExpnId(s.session, krate, sp.ExpnIndexFromU32(raw), s.data(construction), hygieneCodecHash(9, 9))
		}
		data := s.data(construction)
		if operation == "decode_parent_owned" {
			data.Parent = sp.ExpnId{Krate: sp.CrateNumFromU32(2), LocalID: sp.ExpnIndexFromU32(17)}
		}
		env := s.newEnv(construction)
		closure := &hygieneDecodeOnce{state: hygieneOwnedDecodeCell{env, data, true}, live: true}
		return func(c rf.Context) {
			id := sp.DecodeHygieneExpnIdOwned(s.session, krate, raw, closure, c)
			fmt.Printf("returned:decode:%d:%d\n", id.Krate.AsU32(), id.LocalID.AsU32())
		}
	case "lookup_local_empty", "lookup_expn_empty", "lookup_outer_empty", "lookup_debug_empty":
		id := sp.FreshEmptyLocalExpnId(s.session)
		ctxt := sp.RootSyntaxContext()
		if operation == "lookup_outer_empty" {
			ctxt = ctxt.ApplyMark(s.session, id.ToExpnId(), sp.Opaque)
		}
		return func(c rf.Context) {
			switch operation {
			case "lookup_local_empty":
				data := id.ExpnDataWithContext(s.session, c)
				data.CloseWithContext(c)
			case "lookup_expn_empty":
				data := id.ToExpnId().ExpnDataWithContext(s.session, c)
				data.CloseWithContext(c)
			case "lookup_outer_empty":
				data := ctxt.OuterExpnDataWithContext(s.session, c)
				data.CloseWithContext(c)
			default:
				_ = sp.DebugHygieneDataWithContext(s.session, false, c)
			}
			fmt.Println("returned:lookup")
		}
	case "lookup_local_missing", "lookup_expn_missing", "lookup_hash_local_missing", "lookup_foreign_data_missing", "lookup_foreign_hash_missing", "lookup_expect_foreign":
		return func(c rf.Context) {
			local := sp.LocalExpnIdFromU32(17)
			foreign := sp.ExpnId{Krate: sp.CrateNumFromU32(3), LocalID: sp.ExpnIndexFromU32(17)}
			switch operation {
			case "lookup_local_missing":
				data := local.ExpnDataWithContext(s.session, c)
				data.CloseWithContext(c)
			case "lookup_expn_missing":
				data := local.ToExpnId().ExpnDataWithContext(s.session, c)
				data.CloseWithContext(c)
			case "lookup_hash_local_missing":
				_ = local.ToExpnId().ExpnHashWithContext(s.session, c)
			case "lookup_foreign_data_missing":
				data := foreign.ExpnDataWithContext(s.session, c)
				data.CloseWithContext(c)
			case "lookup_foreign_hash_missing":
				_ = foreign.ExpnHashWithContext(s.session, c)
			default:
				_ = foreign.ExpectLocalWithContext(c)
			}
			fmt.Println("returned:lookup")
		}
	case "set_missing":
		data := s.data(construction)
		env := s.newEnv(construction)
		return func(c rf.Context) {
			sp.SetExpnDataOwned(s.session, sp.LocalExpnIdFromU32(17), data, env, hygieneOwnedOps(), c)
			fmt.Println("returned:set_missing")
		}
	}
	if strings.HasPrefix(operation, "context_") {
		return func(c rf.Context) {
			ctxt, root := sp.SyntaxContextFromU32(17), sp.RootSyntaxContext()
			id := sp.RootExpnId()
			switch operation {
			case "context_outer":
				_ = ctxt.OuterExpnWithContext(s.session, c)
			case "context_outer_data":
				data := ctxt.OuterExpnDataWithContext(s.session, c)
				data.CloseWithContext(c)
			case "context_edition":
				_ = ctxt.EditionWithContext(s.session, c)
			case "context_marks":
				_ = ctxt.MarksWithContext(s.session, c)
			case "context_remove":
				_ = ctxt.RemoveMarkWithContext(s.session, c)
			case "context_normalize":
				_ = ctxt.NormalizeToMacros20WithContext(s.session, c)
			case "context_normalize_rules":
				_ = ctxt.NormalizeToMacroRulesWithContext(s.session, c)
			case "context_adjust":
				_ = ctxt.AdjustWithContext(s.session, id, c)
			case "context_normalize_adjust":
				_ = ctxt.NormalizeToMacros20AndAdjustWithContext(s.session, id, c)
			case "context_glob":
				_ = root.GlobAdjustWithContext(s.session, id, sp.DummySpan().WithCtxt(s.session, ctxt), c)
			case "context_reverse":
				_ = ctxt.ReverseGlobAdjustWithContext(s.session, id, sp.DummySpan(), c)
			case "context_equal":
				_ = ctxt.HygienicEqualWithContext(s.session, root, id, c)
			default:
				_ = ctxt.ApplyMarkWithContext(s.session, id, sp.Opaque, c)
			}
			fmt.Println("returned:context")
		}
	}
	if strings.HasPrefix(operation, "index_") {
		return func(c rf.Context) { hygieneIndexFailureRun(operation, s, c) }
	}
	if strings.HasPrefix(operation, "syntax_") || strings.HasPrefix(operation, "encode_") || strings.HasPrefix(operation, "codec_") || strings.HasPrefix(operation, "raw_hash_") {
		return hygienePipelineBuild(operation, s, construction)
	}
	if strings.HasPrefix(operation, "debug_") || strings.HasPrefix(operation, "already_set_debug") {
		return hygieneDebugPanicBuild(operation, s, construction)
	}
	panic("unknown hygiene failure operation")
}
