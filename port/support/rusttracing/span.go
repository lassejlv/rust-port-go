// SPDX-License-Identifier: MIT OR Apache-2.0
package rusttracing

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// OwnedFieldValues owns the temporary values/guards evaluated by one source
// macro invocation. The Values borrow Env's actual cells through Debug/Display
// witnesses. Source destruction order belongs in Env's full Drop dictionary.
type OwnedFieldValues[E any] struct {
	Values []FieldValue
	Env    *rustvec.Owned[E]
}

func EventOwnedFields[E any](scope *Scope, site *Callsite, ctx rustfmt.Context, fields func(rustfmt.Context) OwnedFieldValues[E]) {
	c := callContext(scope, ctx)
	scope, ctx = c.Scope, c.Panic
	if !site.enabled(scope, ctx) {
		return
	}
	owned := fields(ctx)
	defer dropOnReturn(owned.Env.DropWithContext, ctx)
	event := EventRecord{&site.Metadata, Parent{Kind: ContextualParent}, owned.Values, site}
	current(scope, ctx, func(d *Dispatch) {
		if d != nil {
			d.state.subscriber.event(&event, callContext(scope, ctx))
		}
	})
}
func NewSpanOwnedFields[E any](scope *Scope, site *Callsite, ctx rustfmt.Context, fields func(rustfmt.Context) OwnedFieldValues[E]) *Span {
	c := callContext(scope, ctx)
	scope, ctx = c.Scope, c.Panic
	span := &Span{live: true}
	if !site.enabled(scope, ctx) {
		return span
	}
	owned := fields(ctx)
	defer dropOnReturn(owned.Env.DropWithContext, ctx)
	attrs := Attributes{&site.Metadata, Parent{Kind: ContextualParent}, owned.Values, site}
	current(scope, ctx, func(d *Dispatch) {
		span.id = ID{0xDEAD}
		if d != nil {
			span.id = d.state.subscriber.newSpan(&attrs, callContext(scope, ctx))
			span.dispatch = d.Clone()
		}
		span.site = site
		span.enabled = true
	})
	return span
}

func Event(scope *Scope, site *Callsite, ctx rustfmt.Context, fields func() []FieldValue) {
	EventWithParent(scope, site, Parent{Kind: ContextualParent}, ctx, fields)
}
func EventWithParent(scope *Scope, site *Callsite, parent Parent, ctx rustfmt.Context, fields func() []FieldValue) {
	c := callContext(scope, ctx)
	scope, ctx = c.Scope, c.Panic
	if !site.enabled(scope, ctx) {
		return
	}
	values := fields()
	event := EventRecord{&site.Metadata, parent, values, site}
	current(scope, ctx, func(d *Dispatch) {
		if d != nil {
			d.state.subscriber.event(&event, callContext(scope, ctx))
		}
	})
}

// Span owns only its subscriber dispatch and ID. It does not retain the
// creating caller's TLS Scope, so a span can be used under another caller's
// explicit scope just as a Rust tracing Span can be sent to another thread.
type Span struct {
	dispatch      *Dispatch
	id            ID
	site          *Callsite
	enabled, live bool
}

func NewSpan(scope *Scope, site *Callsite, ctx rustfmt.Context, fields func() []FieldValue) *Span {
	return NewSpanWithParent(scope, site, Parent{Kind: ContextualParent}, ctx, fields)
}
func NewSpanWithParent(scope *Scope, site *Callsite, parent Parent, ctx rustfmt.Context, fields func() []FieldValue) *Span {
	c := callContext(scope, ctx)
	scope, ctx = c.Scope, c.Panic
	span := &Span{live: true}
	if !site.enabled(scope, ctx) {
		return span
	}
	values := fields()
	attrs := Attributes{&site.Metadata, parent, values, site}
	current(scope, ctx, func(d *Dispatch) {
		span.id = ID{0xDEAD}
		if d != nil {
			span.id = d.state.subscriber.newSpan(&attrs, callContext(scope, ctx))
			d.check(ctx)
			span.dispatch = d.Clone()
		}
		span.site = site
		span.enabled = true
	})
	return span
}
func (s *Span) IsDisabled() bool { return s == nil || !s.enabled }
func (s *Span) Metadata() *Metadata {
	if s == nil || s.site == nil {
		return nil
	}
	return &s.site.Metadata
}
func (s *Span) ID() (ID, bool) {
	if s == nil || !s.enabled {
		return ID{}, false
	}
	return s.id, true
}
func (s *Span) check(ctx rustfmt.Context) {
	if s == nil || !s.live {
		ctx.Panic("rusttracing: consumed Span")
	}
}
func (s *Span) Clone(ctx rustfmt.Context) *Span { return s.CloneWithScope(nil, ctx) }
func (s *Span) CloneWithScope(scope *Scope, ctx rustfmt.Context) *Span {
	s.check(ctx)
	clone := &Span{site: s.site, id: s.id, enabled: s.enabled, live: true}
	if s.dispatch != nil {
		clone.id = s.dispatch.state.subscriber.cloneSpan(s.id, callContext(scope, ctx))
		clone.dispatch = s.dispatch.Clone()
	}
	return clone
}
func (s *Span) Record(ctx rustfmt.Context, fields []FieldValue) { s.RecordWithScope(nil, ctx, fields) }
func (s *Span) RecordWithScope(scope *Scope, ctx rustfmt.Context, fields []FieldValue) {
	s.check(ctx)
	if !s.enabled || s.dispatch == nil {
		return
	}
	values := make([]FieldValue, 0, len(fields))
	for _, value := range fields {
		field := value.Field
		if field == nil {
			field = s.site.Field(value.Name)
		}
		if field != nil && field.Callsite == s.site {
			value.Field = field
			values = append(values, value)
		}
	}
	s.dispatch.state.subscriber.record(s.id, values, callContext(scope, ctx))
}
func (s *Span) RecordFieldWithScope(scope *Scope, ctx rustfmt.Context, name string, value Value) {
	s.check(ctx)
	if !s.enabled || s.site.Field(name) == nil {
		return
	}
	s.RecordWithScope(scope, ctx, []FieldValue{{Name: name, Value: value}})
}
func (s *Span) FollowsFromWithScope(scope *Scope, ctx rustfmt.Context, other ID) {
	s.check(ctx)
	if s.enabled && s.dispatch != nil {
		s.dispatch.state.subscriber.follows(s.id, other, callContext(scope, ctx))
	}
}
func (s *Span) Close(ctx rustfmt.Context) { s.CloseWithScope(nil, ctx) }
func (s *Span) CloseWithScope(scope *Scope, ctx rustfmt.Context) {
	if s == nil || !s.live {
		return
	}
	s.live = false
	if s.dispatch == nil {
		return
	}
	dispatch := s.dispatch
	s.dispatch = nil
	// Inner's owning Dispatch field is dropped even if Span::drop's
	// subscriber.try_close call panics.
	defer dropOnReturn(func(c rustfmt.Context) { dispatch.CloseWithScope(scope, c) }, ctx)
	dispatch.state.subscriber.tryClose(s.id, callContext(scope, ctx))
}
func (s *Span) DeferCloseWithScope(scope *Scope, ctx rustfmt.Context) {
	finishDrop(func(c rustfmt.Context) { s.CloseWithScope(scope, c) }, ctx, recover())
}

type Entered struct {
	span *Span
	live bool
}

func (s *Span) Enter(ctx rustfmt.Context) *Entered { return s.EnterWithScope(nil, ctx) }
func (s *Span) EnterWithScope(scope *Scope, ctx rustfmt.Context) *Entered {
	s.check(ctx)
	if s.enabled && s.dispatch != nil {
		s.dispatch.state.subscriber.enter(s.id, callContext(scope, ctx))
	}
	return &Entered{s, true}
}
func (e *Entered) Close(ctx rustfmt.Context) { e.CloseWithScope(nil, ctx) }
func (e *Entered) CloseWithScope(scope *Scope, ctx rustfmt.Context) {
	if e == nil || !e.live {
		return
	}
	e.live = false
	s := e.span
	if s.enabled && s.dispatch != nil {
		s.dispatch.state.subscriber.exit(s.id, callContext(scope, ctx))
	}
}
func (e *Entered) DeferCloseWithScope(scope *Scope, ctx rustfmt.Context) {
	finishDrop(func(c rustfmt.Context) { e.CloseWithScope(scope, c) }, ctx, recover())
}
func CurrentSpan(scope *Scope, ctx rustfmt.Context) Current {
	result := Current{Kind: UnknownCurrent}
	current(scope, ctx, func(d *Dispatch) {
		if d != nil {
			result = d.state.subscriber.current(callContext(scope, ctx))
		}
	})
	return result
}
