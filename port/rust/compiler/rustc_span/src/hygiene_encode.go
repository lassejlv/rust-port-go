// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type HygieneEncodeContext struct {
	serializedCtxts *hashmap.Set[SyntaxContext]
	latestCtxts     []SyntaxContext
	serializedExpns *hashmap.Set[ExpnId]
	latestExpns     []ExpnId
}

func hygieneCtxtKeyOps() hashmap.KeyOps[SyntaxContext] {
	return hashmap.KeyOps[SyntaxContext]{Hash: func(v *SyntaxContext, h hashmap.Hasher, _ hashmap.Context) { v.HashWithHasher(h) }, Equal: func(a, b *SyntaxContext, _ hashmap.Context) bool { return *a == *b }, TrivialClone: true}
}
func NewHygieneEncodeContext() HygieneEncodeContext {
	profile := hashmap.DefaultProfile()
	fx := hashmap.FxBuilder{PointerBits: 64}
	return HygieneEncodeContext{serializedCtxts: hashmap.NewSet(hashmap.Layout{Size: 4, Align: 4}, profile, hygieneCtxtKeyOps(), fx), serializedExpns: hashmap.NewSet(hashmap.Layout{Size: 8, Align: 4}, profile, hygieneExpnKeyOps(), fx)}
}
func DefaultHygieneEncodeContext() HygieneEncodeContext { return NewHygieneEncodeContext() }
func (c *HygieneEncodeContext) ScheduleExpnDataForEncoding(id ExpnId) {
	c.ScheduleExpnDataForEncodingWithContext(id, rustfmt.Context{})
}
func (c *HygieneEncodeContext) ScheduleExpnDataForEncodingWithContext(id ExpnId, ctx rustfmt.Context) {
	if c.serializedExpns.Insert(id, ctx) {
		c.latestExpns = append(c.latestExpns, id)
	}
}
func (c *HygieneEncodeContext) EnqueueSyntaxContextForEncoding(ctxt SyntaxContext) {
	c.EnqueueSyntaxContextForEncodingWithContext(ctxt, rustfmt.Context{})
}
func (c *HygieneEncodeContext) EnqueueSyntaxContextForEncodingWithContext(ctxt SyntaxContext, ctx rustfmt.Context) {
	if c.serializedCtxts.Insert(ctxt, ctx) {
		c.latestCtxts = append(c.latestCtxts, ctxt)
	}
}
func (c *HygieneEncodeContext) CloseWithContext(ctx rustfmt.Context) {
	if c == nil {
		return
	}
	hygieneDropFields(ctx, func(ctx rustfmt.Context) {
		if c.serializedCtxts != nil {
			set := c.serializedCtxts
			c.serializedCtxts = nil
			set.Close(ctx)
		}
	}, func(rustfmt.Context) { c.latestCtxts = nil }, func(ctx rustfmt.Context) {
		if c.serializedExpns != nil {
			set := c.serializedExpns
			c.serializedExpns = nil
			set.Close(ctx)
		}
	}, func(rustfmt.Context) { c.latestExpns = nil })
}
func (c *HygieneEncodeContext) Close() { c.CloseWithContext(rustfmt.Context{}) }

type HygieneContextEncoder[E, T any] struct {
	Env  *E
	Call func(*E, *T, SyntaxContext, *SyntaxContextKey, rustfmt.Context)
	Drop func(*E, rustfmt.Context)
}

func (e HygieneContextEncoder[E, T]) Borrow() HygieneContextEncoder[E, T] { e.Drop = nil; return e }

type HygieneExpansionEncoder[E, T any] struct {
	Env  *E
	Call func(*E, *T, ExpnId, Option[*ExpnData], ExpnHash, rustfmt.Context)
	Drop func(*E, rustfmt.Context)
}

func (e HygieneExpansionEncoder[E, T]) Borrow() HygieneExpansionEncoder[E, T] { e.Drop = nil; return e }
func EncodeHygiene[T any](session *SessionGlobals, cell *HygieneEncodeCell, encoder *T, encodeCtxt func(*T, SyntaxContext, *SyntaxContextKey), encodeExpn func(*T, ExpnId, Option[*ExpnData], ExpnHash)) {
	EncodeHygieneOwned(session, cell, encoder, HygieneContextEncoder[struct{}, T]{Call: func(_ *struct{}, e *T, c SyntaxContext, key *SyntaxContextKey, _ rustfmt.Context) {
		encodeCtxt(e, c, key)
	}}, HygieneExpansionEncoder[struct{}, T]{Call: func(_ *struct{}, e *T, id ExpnId, data Option[*ExpnData], hash ExpnHash, _ rustfmt.Context) {
		encodeExpn(e, id, data, hash)
	}}, rustfmt.Context{})
}

type hygieneRoundFields struct {
	count         uint
	first, second *HygieneEncodeRef
}

func traceHygieneEncodeRound(cell *HygieneEncodeCell, ctx rustfmt.Context) {
	rusttracing.EventOwnedFields(rusttracing.ScopeFromContext(ctx), hygieneSelectedTraceCallsite(ctx, hygieneTraceEncodeRound), ctx, func(ctx rustfmt.Context) rusttracing.OwnedFieldValues[hygieneRoundFields] {
		fields := hygieneRoundFields{}
		fields.first = cell.borrowWithSite(ctx, "rustc_span::hygiene::encode.trace_round.borrow")
		fields.count = uint(len(fields.first.Get().latestCtxts))
		complete := false
		defer func() {
			if p := recover(); p != nil {
				if !complete {
					fields.first.Close()
				}
				ctx.ResumeUnwind(p)
			}
		}()
		fields.second = cell.borrowWithSite(ctx, "rustc_span::hygiene::encode.trace_round.borrow")
		env := rustvec.NewOwned(fields, rustvec.ValueOps[hygieneRoundFields]{Drop: func(v *hygieneRoundFields, _ rustfmt.Context) { v.second.Close(); v.first.Close() }})
		values := env.Borrow()
		value := rusttracing.MessageField(func(f *rustfmt.Formatter) error {
			return hygieneMessage(f, []string{"encode_hygiene: Serializing a round of ", " SyntaxContextData: ", ""}, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(uint64(values.count), 64, f) }, func(f *rustfmt.Formatter) error {
				pretty := f.Options.Alternate
				b := rustfmt.BeginList(f.Writer, pretty)
				for _, ctxt := range values.second.Get().latestCtxts {
					b.Entry(f.Value(pretty, ctxt.WriteDebugFormatter))
				}
				return b.Finish()
			})
		})
		complete = true
		return rusttracing.OwnedFieldValues[hygieneRoundFields]{Env: env, Values: []rusttracing.FieldValue{{Name: "message", Value: value}}}
	})
}
func EncodeHygieneOwned[T, E1, E2 any](session *SessionGlobals, cell *HygieneEncodeCell, encoder *T, encodeCtxt HygieneContextEncoder[E1, T], encodeExpn HygieneExpansionEncoder[E2, T], ctx rustfmt.Context) {
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if encodeCtxt.Drop != nil {
			encodeCtxt.Drop(encodeCtxt.Env, c)
		}
	}, ctx)
	defer hygieneDropOnReturn(func(c rustfmt.Context) {
		if encodeExpn.Drop != nil {
			encodeExpn.Drop(encodeExpn.Env, c)
		}
	}, ctx)
	for {
		pending := func() bool {
			g := cell.borrowWithSite(ctx, HygienePanicSite(1463, 33))
			defer g.Close()
			d := g.Get()
			return len(d.latestCtxts) != 0 || len(d.latestExpns) != 0
		}()
		if !pending {
			break
		}
		traceHygieneEncodeRound(cell, ctx)
		latestContexts := func() []SyntaxContext {
			g := cell.borrowMutWithSite(ctx, HygienePanicSite(1476, 59))
			defer g.Close()
			d := g.Get()
			latest := d.latestCtxts
			d.latestCtxts = nil
			return latest
		}()
		for _, ctxt := range latestContexts {
			key := func() SyntaxContextKey {
				g := hygieneBorrow(session, ctx)
				defer g.Close()
				return g.Get().contextData(ctxt, ctx, hygienePoint(1479, 76)).key()
			}()
			encodeCtxt.Call(encodeCtxt.Env, encoder, ctxt, &key, ctx)
		}
		latestExpansions := func() []ExpnId {
			g := cell.BorrowMutWithContext(ctx)
			defer g.Close()
			d := g.Get()
			latest := d.latestExpns
			d.latestExpns = nil
			return latest
		}()
		for _, id := range latestExpansions {
			func() {
				var data Option[ExpnData]
				var hash ExpnHash
				func() {
					g := hygieneBorrow(session, ctx)
					defer g.Close()
					d := g.Get()
					transferred := false
					defer hygieneDropOnReturn(func(c rustfmt.Context) {
						if !transferred && data.present {
							data.value.CloseWithContext(c)
						}
					}, ctx)
					if local, yes := id.AsLocal().Get(); yes {
						data = Some(d.localData(local, ctx).CloneWithContext(ctx))
					}
					hash = d.expnHash(id, ctx)
					transferred = true
				}()
				defer hygieneDropOnReturn(func(c rustfmt.Context) {
					if data.present {
						data.value.CloseWithContext(c)
					}
				}, ctx)
				borrowed := None[*ExpnData]()
				if data.present {
					borrowed = Some(&data.value)
				}
				encodeExpn.Call(encodeExpn.Env, encoder, id, borrowed, hash, ctx)
			}()
		}
	}
	hygieneTraceLiteral(ctx, hygieneTraceEncodeDone, "encode_hygiene: Done serializing SyntaxContextData")
}
