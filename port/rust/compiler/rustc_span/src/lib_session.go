// SPDX-License-Identifier: MIT OR Apache-2.0
// Session and metavariable storage from pinned rustc_span/src/lib.rs. Explicit
// scope handles adapt scoped TLS without pretending that Go has Rust TLS.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

func NewSessionGlobals(edition Edition, extraSymbols []string, inputs Option[SourceMapInputs], config ...SpanEncodingConfig) *SessionGlobals {
	return NewSessionGlobalsWithContext(edition, extraSymbols, inputs, rustfmt.Context{}, config...)
}
func NewSessionGlobalsWithContext(edition Edition, extraSymbols []string, inputs Option[SourceMapInputs], context rustfmt.Context, config ...SpanEncodingConfig) *SessionGlobals {
	profile := spanEncodingConfig(nil, config...)
	session := &SessionGlobals{spanConfig: profile}
	finished := false
	inputsConsumed := false
	defer func() {
		payload := recover()
		if payload != nil && context.ProcessingPanic {
			context.ResumeUnwind(payload)
		}
		if !finished {
			dropContext := context
			if payload != nil {
				dropContext.Unwinding = true
			}
			cleanup := func() {
				spanDropFields(dropContext, func(ctx rustfmt.Context) { session.CloseWithContext(ctx) }, func(ctx rustfmt.Context) {
					if !inputsConsumed {
						if sourceInputs, ok := inputs.Get(); ok {
							sourceInputs.FileLoader.CloseWithContext(ctx)
						}
					}
				})
			}
			if payload != nil {
				spanCleanupDuringUnwindWithContext(dropContext, cleanup)
			} else {
				cleanup()
			}
		}
		if payload != nil {
			context.ResumeUnwind(payload)
		}
	}()
	session.symbolInterner = newSymbolInternerWithContext(extraSymbols, context)
	session.spanMu = newSpanMutex()
	session.metavarSpans = NewMetavarSpansMap()
	session.hygieneData = NewLockWithDrop(newHygieneDataWithConfig(edition, HygieneConfig{DebugAssertions: profile.DebugAssertions, OverflowChecks: profile.OverflowChecks}), func(value *HygieneData, context rustfmt.Context) { value.CloseWithContext(context) })
	if sourceInputs, ok := inputs.Get(); ok {
		inputsConsumed = true
		session.sourceMap = Some(NewSharedSourceMapWithContext(context, sourceInputs, profile))
	}
	finished = true
	return session
}

// spanDropFields preserves declaration-order field cleanup after an earlier
// destructor starts unwinding. Each callback borrows the actual owned field.
func spanDropFields(context rustfmt.Context, fields ...func(rustfmt.Context)) {
	for index, field := range fields {
		func() {
			defer func() {
				if payload := recover(); payload != nil {
					if context.ProcessingPanic {
						context.ResumeUnwind(payload)
					}
					unwind := context
					unwind.Unwinding = true
					for _, remaining := range fields[index+1:] {
						spanCleanupDuringUnwindWithContext(unwind, func() { remaining(unwind) })
					}
					context.ResumeUnwind(payload)
				}
			}()
			field(context)
		}()
	}
}
func (session *SessionGlobals) Close() { session.CloseWithContext(rustfmt.Context{}) }
func (session *SessionGlobals) CloseWithContext(context rustfmt.Context) {
	if session == nil {
		return
	}
	spanDropFields(context,
		func(ctx rustfmt.Context) {
			if session.symbolInterner != nil {
				session.symbolInterner.CloseWithContext(ctx)
			}
		},
		func(ctx rustfmt.Context) {
			if session.metavarSpans != nil {
				session.metavarSpans.entries.CloseWithContext(ctx)
			}
		},
		func(ctx rustfmt.Context) {
			if session.hygieneData != nil {
				session.hygieneData.CloseWithContext(ctx)
			}
		},
		func(ctx rustfmt.Context) {
			if sourceMap, ok := session.sourceMap.Get(); ok {
				sourceMap.CloseWithContext(ctx)
			}
		})
}

type metavarSpanValue struct {
	span Span
	read bool
}

// MetavarSpansMap uses the actual scoped Fx/SwissTable and UnordMap operations.
type MetavarSpansMap struct {
	entries *FreezeLock[*datastructures.UnordMap[Span, metavarSpanValue]]
}

func NewMetavarSpansMap() *MetavarSpansMap {
	mapValue := datastructures.NewUnordMap(metavarInputMapConfig())
	return &MetavarSpansMap{entries: NewFreezeLockWithDrop(mapValue, false, func(value **datastructures.UnordMap[Span, metavarSpanValue], ctx rustfmt.Context) {
		(*value).Close(ctx)
	})}
}
func (value *MetavarSpansMap) Insert(span, varSpan Span) bool {
	return value.InsertWithContext(span, varSpan, rustfmt.Context{})
}
func (value *MetavarSpansMap) InsertWithContext(span, varSpan Span, context rustfmt.Context) bool {
	guard := value.entries.WriteWithContext(context)
	defer guard.Close()
	entries := *guard.Get()
	_, occupied := entries.TryInsert(span, metavarSpanValue{span: varSpan}, context)
	if occupied == nil {
		return true
	}
	defer occupied.Close(context)
	return occupied.Entry.Get(context).span == varSpan
}
func (value *MetavarSpansMap) Get(span Span) Option[Span] {
	return value.GetWithContext(span, rustfmt.Context{})
}
func (value *MetavarSpansMap) GetWithContext(span Span, context rustfmt.Context) Option[Span] {
	if guard, ok := value.entries.TryWriteWithContext(context); ok {
		defer guard.Close()
		entries := *guard.Get()
		if old, ok := entries.GetMut(&span, context); ok {
			old.read = true
			return Some(old.span)
		}
		return None[Span]()
	}
	guard := value.entries.ReadWithContext(context)
	defer guard.Close()
	if old, ok := (*guard.Get()).Get(&span, context); ok && old.read {
		return Some(old.span)
	}
	return None[Span]()
}
func (value *MetavarSpansMap) FreezeAndGetReadSpans() *datastructures.UnordMap[Span, Span] {
	return value.FreezeAndGetReadSpansWithContext(rustfmt.Context{})
}
func (value *MetavarSpansMap) FreezeAndGetReadSpansWithContext(context rustfmt.Context) *datastructures.UnordMap[Span, Span] {
	entries := value.entries.FreezeWithContext(context)
	return datastructures.CollectUnordMapFilterMap((*entries).Items(context), func(key *Span, value *metavarSpanValue, _ rusthashmap.Context) (rusthashmap.Pair[Span, Span], bool) {
		if !value.read {
			return rusthashmap.Pair[Span, Span]{}, false
		}
		return rusthashmap.Pair[Span, Span]{Key: *key, Value: value.span}, true
	}, metavarOutputMapConfig(), context)
}

func metavarSpanKeys() rusthashmap.KeyOps[Span] {
	return rusthashmap.KeyOps[Span]{Hash: func(value *Span, h rusthashmap.Hasher, _ rusthashmap.Context) { value.HashWithHasher(h) }, Equal: func(a, b *Span, _ rusthashmap.Context) bool { return *a == *b }, TrivialClone: true}
}
func metavarInputMapConfig() datastructures.UnordMapConfig[Span, metavarSpanValue] {
	return datastructures.UnordMapConfig[Span, metavarSpanValue]{Layout: rusthashmap.Layout{Size: 20, Align: 4}, Profile: rusthashmap.DefaultProfile(), Keys: metavarSpanKeys(), Values: rusthashmap.ValueOps[metavarSpanValue]{Equal: func(a, b *metavarSpanValue, _ rusthashmap.Context) bool { return *a == *b }, TrivialClone: true}}
}
func metavarOutputMapConfig() datastructures.UnordMapConfig[Span, Span] {
	return datastructures.UnordMapConfig[Span, Span]{Layout: rusthashmap.Layout{Size: 16, Align: 4}, Profile: rusthashmap.DefaultProfile(), Keys: metavarSpanKeys(), Values: rusthashmap.ValueOps[Span]{Equal: func(a, b *Span, _ rusthashmap.Context) bool { return *a == *b }, TrivialClone: true}}
}
