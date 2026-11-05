// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	serialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

type HygieneDecodeContext struct {
	remappedCtxts *Lock[*index.OptionIndexVec[DecodedSyntaxContext, SyntaxContext]]
}

func NewHygieneDecodeContext(config ...HygieneConfig) *HygieneDecodeContext {
	profile := hygieneConfiguration(config)
	values := index.NewOptionIndexVecWithLayout(DecodedSyntaxContextIdx(), index.ValueOps[SyntaxContext]{Copy: true, TrivialClone: true, RustSize: 4}, index.OptionLayout{RustSize: 4})
	values.Config = index.IndexStorageConfig{DebugAssertions: profile.DebugAssertions, OverflowChecks: profile.OverflowChecks}
	return &HygieneDecodeContext{NewLockWithDrop(values, func(v **index.OptionIndexVec[DecodedSyntaxContext, SyntaxContext], ctx rustfmt.Context) {
		(*v).CloseWithContext(index.IndexDropContext{Unwinding: ctx.Unwinding})
	})}
}
func DefaultHygieneDecodeContext(config ...HygieneConfig) *HygieneDecodeContext {
	return NewHygieneDecodeContext(config...)
}
func (c *HygieneDecodeContext) Close() { c.CloseWithContext(rustfmt.Context{}) }
func (c *HygieneDecodeContext) CloseWithContext(ctx rustfmt.Context) {
	if c != nil && c.remappedCtxts != nil {
		lock := c.remappedCtxts
		c.remappedCtxts = nil
		lock.CloseWithContext(ctx)
	}
}

type DecodedExpnData struct {
	Data ExpnData
	Hash ExpnHash
}

func DecodeHygieneExpnId(session *SessionGlobals, crate CrateNum, raw uint32, decode func(ExpnId) (ExpnData, ExpnHash)) ExpnId {
	return DecodeHygieneExpnIdOwned(session, crate, raw, hashmap.PlainOnce(func(id ExpnId, _ rustfmt.Context) DecodedExpnData {
		data, hash := decode(id)
		return DecodedExpnData{data, hash}
	}), rustfmt.Context{})
}
func DecodeHygieneExpnIdOwned(session *SessionGlobals, crate CrateNum, raw uint32, decode hashmap.Once[ExpnId, DecodedExpnData], ctx rustfmt.Context) ExpnId {
	defer hygieneDropOnReturn(decode.Close, ctx)
	if raw == 0 {
		hygieneTraceLiteral(ctx, hygieneTraceDecodeRootExpn, "decode_expn_id: deserialized root")
		return RootExpnId()
	}
	index := ExpnIndexFromU32WithContext(raw, ctx)
	if sessionHygieneConfig(session).DebugAssertions && crate == LocalCrate() {
		hygieneNotEqualAssertion(ctx, HygienePanicSite(1568, 5), crate.WriteDebugFormatter, LocalCrate().WriteDebugFormatter)
	}
	id := ExpnId{crate, index}
	cached := func() bool {
		g := hygieneBorrow(session, ctx)
		defer g.Close()
		return g.Get().foreignExpnData.ContainsKey(&id, ctx)
	}()
	if cached {
		return id
	}
	decoded := decode.Call(id, ctx)
	return RegisterExpnIdWithContext(session, crate, index, decoded.Data, decoded.Hash, ctx)
}

// D is the actual borrowed decoder witness (normally *D); the exact same
// reference is passed into decode_data, outside both the cache and hygiene
// locks, so recursively decoding parent contexts remains possible.
type HygieneSyntaxDecodeArgs[D serialize.Decoder] struct {
	Decoder D
	Context DecodedSyntaxContext
}

func DecodeHygieneSyntaxContext[D serialize.Decoder](session *SessionGlobals, decoder D, cache *HygieneDecodeContext, decode func(D, DecodedSyntaxContext) SyntaxContextKey) SyntaxContext {
	return DecodeHygieneSyntaxContextOwned(session, decoder, cache, hashmap.PlainOnce(func(args HygieneSyntaxDecodeArgs[D], _ rustfmt.Context) SyntaxContextKey {
		return decode(args.Decoder, args.Context)
	}), spanCodecContext(decoder))
}
func DecodeHygieneSyntaxContextOwned[D serialize.Decoder](session *SessionGlobals, decoder D, cache *HygieneDecodeContext, decode hashmap.Once[HygieneSyntaxDecodeArgs[D], SyntaxContextKey], ctx rustfmt.Context) SyntaxContext {
	// Keep the exact D reference borrowed through read, callback and argument
	// destruction. Opaque/context-cell dictionaries bind the current operation
	// without retaining the decoder's construction-time runtime metadata.
	if cell, ok := any(decoder).(serialize.RuntimeContextCell); ok {
		return serialize.WithRuntimeContext(cell, ctx, func(serialize.RuntimeContextCell) SyntaxContext {
			return decodeHygieneSyntaxContextOwned(session, decoder, cache, decode, ctx)
		})
	}
	return decodeHygieneSyntaxContextOwned(session, decoder, cache, decode, ctx)
}
func decodeHygieneSyntaxContextOwned[D serialize.Decoder](session *SessionGlobals, decoder D, cache *HygieneDecodeContext, decode hashmap.Once[HygieneSyntaxDecodeArgs[D], SyntaxContextKey], ctx rustfmt.Context) SyntaxContext {
	defer hygieneDropOnReturn(decode.Close, ctx)
	decoded := DecodeDecodedSyntaxContextWithContext(decoder, ctx)
	if decoded.isRoot() {
		hygieneTraceLiteral(ctx, hygieneTraceDecodeRootCtxt, "decode_syntax_context: deserialized root")
		return RootSyntaxContext()
	}
	var cached Option[SyntaxContext]
	func() {
		guard := cache.remappedCtxts.LockWithContext(ctx)
		defer guard.Close()
		values := *guard.Get()
		if value, ok := values.Get(decoded); ok && value.Present {
			cached = Some(value.Value)
		}
	}()
	if cached.present {
		return cached.value
	}
	key := decode.Call(HygieneSyntaxDecodeArgs[D]{decoder, decoded}, ctx)
	remapped := func() SyntaxContext {
		guard := hygieneBorrow(session, ctx)
		defer guard.Close()
		return guard.Get().allocCtxt(session, key.Parent, key.Expansion, key.Transparency, ctx)
	}()
	guard := cache.remappedCtxts.LockWithContext(ctx)
	defer guard.Close()
	(*guard.Get()).Insert(decoded, remapped)
	return remapped
}
