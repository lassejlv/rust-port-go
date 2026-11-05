// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"fmt"
	"strconv"
	"strings"

	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	rv "github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type hygienePipelineDecoder struct {
	*hygieneCodec
	context  rf.Context
	failRead bool
}
type hygienePipelineBoolDecoder struct{ *hygienePipelineDecoder }

func (d hygienePipelineBoolDecoder) ReadBool() bool { return d.take("bool").(bool) }

type hygieneBadRawDecoder struct{ *hygienePipelineDecoder }

func (d hygieneBadRawDecoder) ReadRawBytes(n uint) []byte {
	return d.take(fmt.Sprintf("raw:%d", n)).([]byte)
}

func (d *hygienePipelineDecoder) RuntimeContext() rf.Context     { return d.context }
func (d *hygienePipelineDecoder) SetRuntimeContext(c rf.Context) { d.context = c }
func (d *hygienePipelineDecoder) ReadU32() uint32 {
	if d.failRead {
		d.events = append(d.events, "u32")
		d.context.PanicAt(rf.SourcePanicInfo{Payload: "decoder", Kind: rf.PanicStaticStr, CanUnwind: true, Location: d.context.SourcePanicLocation("owned_decoder")})
	}
	return d.hygieneCodec.ReadU32()
}
func hygienePipelineBuild(operation string, s *hygieneOwnedCore, construction rf.Context) func(rf.Context) {
	if strings.HasPrefix(operation, "raw_hash_") {
		n, err := strconv.Atoi(strings.TrimPrefix(operation, "raw_hash_"))
		if err != nil {
			panic(err)
		}
		return func(c rf.Context) {
			d := hygieneBadRawDecoder{&hygienePipelineDecoder{hygieneCodec: &hygieneCodec{session: s.session, atoms: []hygieneCodecAtom{{"raw", make([]byte, n)}}, fail: -1}, context: c}}
			defer hygieneOwnedClose(c, func(c rf.Context) {
				fmt.Printf("raw_state:%d:%s:%t\n", d.position, strings.Join(d.events, ","), c.Panicking())
			})
			_ = sp.DecodeExpnHashWithContext(d, c)
			fmt.Println("returned:raw")
		}
	}
	if strings.HasPrefix(operation, "codec_") {
		n, err := strconv.Atoi(strings.TrimPrefix(operation, "codec_"))
		if err != nil {
			panic(err)
		}
		return func(c rf.Context) {
			ty, atoms := hygieneInvalidCodecInput(s.session, n%32, n >= 32)
			d := &hygienePipelineDecoder{hygieneCodec: &hygieneCodec{session: s.session, atoms: atoms, fail: -1}, context: c}
			defer hygieneOwnedClose(c, func(c rf.Context) {
				fmt.Printf("codec_state:%d:%s:%t\n", d.position, strings.Join(d.events, ","), c.Panicking())
			})
			var decoder sp.SpanDecoder = d
			if n >= 32 {
				decoder = hygienePipelineBoolDecoder{d}
			}
			_ = decodeHygieneCodec(s.session, ty, decoder)
			fmt.Println("returned:codec")
		}
	}
	if strings.HasPrefix(operation, "syntax_") {
		cache := sp.NewHygieneDecodeContext(s.profile)
		raw := uint32(17)
		if operation == "syntax_root_owned" {
			raw = 0
		} else if operation == "syntax_bound_owned" {
			raw = 0x20000000
		}
		parent := sp.RootSyntaxContext()
		if operation == "syntax_parent_owned" {
			parent = sp.SyntaxContextFromU32(999)
		}
		expn := sp.ExpnId{Krate: sp.CrateNumFromU32(3), LocalID: sp.ExpnIndexFromU32(17)}
		decoder := func(raw uint32, fail bool) *hygienePipelineDecoder {
			return &hygienePipelineDecoder{hygieneCodec: &hygieneCodec{session: s.session, atoms: []hygieneCodecAtom{{"u32", raw}}, fail: -1}, context: construction, failRead: fail}
		}
		if operation == "syntax_cached_owned" {
			d := decoder(raw, false)
			_ = sp.DecodeHygieneSyntaxContextOwned(s.session, d, cache, hashmap.PlainOnce(func(_ sp.HygieneSyntaxDecodeArgs[*hygienePipelineDecoder], _ rf.Context) sp.SyntaxContextKey {
				return sp.SyntaxContextKey{Parent: sp.RootSyntaxContext(), Expansion: expn, Transparency: sp.Opaque}
			}), construction)
		}
		d := decoder(raw, operation == "syntax_reader_owned")
		env := s.newEnv(construction)
		return func(c rf.Context) {
			defer cache.CloseWithContext(c)
			defer hygieneOwnedClose(c, func(c rf.Context) {
				fmt.Printf("decoder_state:%d:%s:%t:%d:%d\n", d.position, strings.Join(d.events, ","), c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
			})
			closure := hashmap.NewOnce(env, func(e *hygieneOwnedEnv, args sp.HygieneSyntaxDecodeArgs[*hygienePipelineDecoder], c rf.Context) sp.SyntaxContextKey {
				if args.Decoder != d {
					panic("decoder reference changed")
				}
				e.call(c, "syntax_decode", fmt.Sprintf("%d/%d", args.Context.AsU32(), args.Decoder.Position()))
				return sp.SyntaxContextKey{Parent: parent, Expansion: expn, Transparency: sp.Opaque}
			})
			ctxt := sp.DecodeHygieneSyntaxContextOwned(s.session, d, cache, closure, c)
			fmt.Printf("returned:syntax:%d:%d\n", ctxt.AsU32(), d.Position())
		}
	}
	id := sp.FreshLocalExpnId(s.session, s.data(construction), hygieneCoreHcx{}).ToExpnId()
	ctxt := sp.RootSyntaxContext().ApplyMark(s.session, id, sp.Opaque)
	cell := sp.NewHygieneEncodeCell()
	guard := cell.BorrowMut()
	switch operation {
	case "encode_empty_owned":
	case "encode_bad_context_owned":
		guard.Get().EnqueueSyntaxContextForEncoding(sp.SyntaxContextFromU32(17))
	case "encode_bad_expn_owned":
		guard.Get().ScheduleExpnDataForEncoding(sp.LocalExpnIdFromU32(17).ToExpnId())
	case "encode_empty_expn_owned":
		guard.Get().ScheduleExpnDataForEncoding(sp.FreshEmptyLocalExpnId(s.session).ToExpnId())
	case "encode_foreign_missing_owned":
		guard.Get().ScheduleExpnDataForEncoding(sp.ExpnId{Krate: sp.CrateNumFromU32(3), LocalID: sp.ExpnIndexFromU32(17)})
	default:
		guard.Get().EnqueueSyntaxContextForEncoding(ctxt)
		guard.Get().ScheduleExpnDataForEncoding(id)
	}
	guard.Close()
	ce, ee := s.newEnv(construction), s.newEnv(construction)
	owner := func(label string, env *rv.Owned[hygieneOwnedEnv], c rf.Context) {
		if !env.IsLive() {
			return
		}
		fmt.Printf("encoder_owner:%s:%t:%d:%d\n", label, c.Panicking(), hygieneOwnedRuntimeID(c), s.refs())
		env.DropWithContext(c)
	}
	return func(c rf.Context) {
		defer cell.CloseWithContext(c)
		if operation == "encode_locked_owned" {
			guard := cell.BorrowMutWithContext(c)
			defer guard.Close()
		}
		if operation == "encode_shared_owned" {
			guard := cell.BorrowWithContext(c)
			defer guard.Close()
		}
		defer func() {
			p := recover()
			if p != nil {
				if c.ProcessingPanic {
					c.ResumeUnwind(p)
				}
				u := c
				u.Unwinding = true
				for _, field := range []struct {
					label string
					env   *rv.Owned[hygieneOwnedEnv]
				}{{"ctxt", ce}, {"expn", ee}} {
					func() {
						defer func() {
							if recover() != nil {
								u.AbortCleanup()
							}
						}()
						owner(field.label, field.env, u)
					}()
				}
				c.ResumeUnwind(p)
			}
		}()
		cfn := sp.HygieneContextEncoder[hygieneOwnedEnv, struct{}]{Env: ce.Borrow(), Call: func(e *hygieneOwnedEnv, _ *struct{}, ctxt sp.SyntaxContext, key *sp.SyntaxContextKey, c rf.Context) {
			e.call(c, "encode_ctxt", fmt.Sprintf("%d/%d/%d", ctxt.AsU32(), key.Parent.AsU32(), key.Expansion.LocalID.AsU32()))
		}, Drop: func(_ *hygieneOwnedEnv, c rf.Context) { owner("ctxt", ce, c) }}
		efn := sp.HygieneExpansionEncoder[hygieneOwnedEnv, struct{}]{Env: ee.Borrow(), Call: func(e *hygieneOwnedEnv, _ *struct{}, id sp.ExpnId, data sp.Option[*sp.ExpnData], hash sp.ExpnHash, c rf.Context) {
			v, present := data.Get()
			refs := uint(0)
			if present {
				if features, yes := v.AllowInternalUnstable.Get(); yes {
					refs = features.StrongCount()
				}
			}
			e.call(c, "encode_expn", fmt.Sprintf("%d/%d/%t/%d/%d", id.Krate.AsU32(), id.LocalID.AsU32(), present, hash.LocalHash().AsU64(), refs))
		}, Drop: func(_ *hygieneOwnedEnv, c rf.Context) { owner("expn", ee, c) }}
		var encoder struct{}
		if operation == "encode_borrowed" {
			sp.EncodeHygieneOwned(s.session, cell, &encoder, cfn.Borrow(), efn.Borrow(), c)
			fmt.Println("returned:encode_borrowed")
			owner("expn", ee, c)
			owner("ctxt", ce, c)
		} else {
			sp.EncodeHygieneOwned(s.session, cell, &encoder, cfn, efn, c)
			fmt.Println("returned:encode_owned")
		}
	}
}
