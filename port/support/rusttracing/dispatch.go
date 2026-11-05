// SPDX-License-Identifier: MIT OR Apache-2.0
package rusttracing

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"sync"
	"sync/atomic"
)

type Interest uint8

const (
	Never Interest = iota
	Sometimes
	Always
)

func combine(a, b Interest) Interest {
	if a == b {
		return a
	}
	return Sometimes
}

// SubscriberOps is the complete subscriber operation dictionary exercised by
// the selected tracing macros/instrumentation. All operations borrow the actual
// owned S cell; optional methods implement the genuine trait defaults.
type SubscriberOps[S any] struct {
	OnRegisterDispatch func(*S, *Dispatch, Context)
	RegisterCallsite   func(*S, *Metadata, Context) Interest
	Enabled            func(*S, *Metadata, Context) bool
	MaxLevelHint       func(*S, Context) (Level, bool)
	NewSpan            func(*S, *Attributes, Context) ID
	Record             func(*S, ID, []FieldValue, Context)
	RecordFollowsFrom  func(*S, ID, ID, Context)
	EventEnabled       func(*S, *EventRecord, Context) bool
	Event              func(*S, *EventRecord, Context)
	Enter              func(*S, ID, Context)
	Exit               func(*S, ID, Context)
	CloneSpan          func(*S, ID, Context) ID
	TryClose           func(*S, ID, Context) bool
	DropSpan           func(*S, ID, Context)
	CurrentSpan        func(*S, Context) Current
}
type subscriber interface {
	onRegister(*Dispatch, Context)
	register(*Metadata, Context) Interest
	enabled(*Metadata, Context) bool
	maxLevel(Context) (Level, bool)
	newSpan(*Attributes, Context) ID
	record(ID, []FieldValue, Context)
	follows(ID, ID, Context)
	event(*EventRecord, Context)
	enter(ID, Context)
	exit(ID, Context)
	cloneSpan(ID, Context) ID
	tryClose(ID, Context) bool
	current(Context) Current
	close(Context)
}
type subscriberCell[S any] struct {
	state *rustvec.Owned[S]
	ops   SubscriberOps[S]
}

func (s *subscriberCell[S]) onRegister(d *Dispatch, c Context) {
	if s.ops.OnRegisterDispatch != nil {
		s.ops.OnRegisterDispatch(s.state.Borrow(), d, c)
	}
}
func (s *subscriberCell[S]) register(m *Metadata, c Context) Interest {
	if s.ops.RegisterCallsite != nil {
		return s.ops.RegisterCallsite(s.state.Borrow(), m, c)
	}
	if s.enabled(m, c) {
		return Always
	}
	return Never
}
func (s *subscriberCell[S]) enabled(m *Metadata, c Context) bool {
	return s.ops.Enabled(s.state.Borrow(), m, c)
}
func (s *subscriberCell[S]) maxLevel(c Context) (Level, bool) {
	if s.ops.MaxLevelHint != nil {
		return s.ops.MaxLevelHint(s.state.Borrow(), c)
	}
	return 0, false
}
func (s *subscriberCell[S]) newSpan(a *Attributes, c Context) ID {
	return s.ops.NewSpan(s.state.Borrow(), a, c)
}
func (s *subscriberCell[S]) record(id ID, v []FieldValue, c Context) {
	s.ops.Record(s.state.Borrow(), id, v, c)
}
func (s *subscriberCell[S]) follows(a, b ID, c Context) {
	s.ops.RecordFollowsFrom(s.state.Borrow(), a, b, c)
}
func (s *subscriberCell[S]) event(e *EventRecord, c Context) {
	if s.ops.EventEnabled == nil || s.ops.EventEnabled(s.state.Borrow(), e, c) {
		s.ops.Event(s.state.Borrow(), e, c)
	}
}
func (s *subscriberCell[S]) enter(id ID, c Context) { s.ops.Enter(s.state.Borrow(), id, c) }
func (s *subscriberCell[S]) exit(id ID, c Context)  { s.ops.Exit(s.state.Borrow(), id, c) }
func (s *subscriberCell[S]) cloneSpan(id ID, c Context) ID {
	if s.ops.CloneSpan != nil {
		return s.ops.CloneSpan(s.state.Borrow(), id, c)
	}
	return id
}
func (s *subscriberCell[S]) tryClose(id ID, c Context) bool {
	if s.ops.TryClose != nil {
		return s.ops.TryClose(s.state.Borrow(), id, c)
	}
	if s.ops.DropSpan != nil {
		s.ops.DropSpan(s.state.Borrow(), id, c)
	}
	return false
}
func (s *subscriberCell[S]) current(c Context) Current {
	if s.ops.CurrentSpan != nil {
		return s.ops.CurrentSpan(s.state.Borrow(), c)
	}
	return Current{Kind: UnknownCurrent}
}
func (s *subscriberCell[S]) close(c Context) { s.state.DropWithContext(c.Panic) }

type dispatchState struct {
	mu         sync.Mutex
	refs       uint
	subscriber subscriber
	runtime    *Runtime
}

func (s *dispatchState) upgrade() *Dispatch {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.refs == 0 {
		return nil
	}
	s.refs++
	return &Dispatch{state: s, live: true}
}
func (s *dispatchState) alive() bool { s.mu.Lock(); defer s.mu.Unlock(); return s.refs != 0 }

// Dispatch is an owning Arc-like handle. Assignment is not a source Clone;
// callers use Clone/Close for explicit source lifetime and destructor timing.
type Dispatch struct {
	state *dispatchState
	live  bool
}

func (d *Dispatch) Clone() *Dispatch {
	if d == nil {
		return nil
	}
	d.check(rustfmt.Context{})
	return d.state.upgrade()
}

type WeakDispatch struct {
	state              *dispatchState
	live               bool
	globalNoSubscriber bool
}

func (d *Dispatch) Downgrade(ctx rustfmt.Context) *WeakDispatch {
	if d == nil {
		return &WeakDispatch{live: true, globalNoSubscriber: true}
	}
	d.check(ctx)
	return &WeakDispatch{state: d.state, live: true}
}
func (d *WeakDispatch) Clone(ctx rustfmt.Context) *WeakDispatch {
	if d == nil || !d.live {
		ctx.Panic("rusttracing: consumed WeakDispatch")
	}
	return &WeakDispatch{state: d.state, live: true, globalNoSubscriber: d.globalNoSubscriber}
}
func (d *WeakDispatch) Upgrade(ctx rustfmt.Context) (*Dispatch, bool) {
	if d == nil || !d.live {
		ctx.Panic("rusttracing: consumed WeakDispatch")
	}
	if d.globalNoSubscriber {
		return nil, true
	}
	strong := d.state.upgrade()
	return strong, strong != nil
}

// Nil Dispatch is the source's global NoSubscriber, not an absent current
// dispatcher. In particular its downgraded handle always upgrades successfully
// (returning nil,true), since the real NoSubscriber has static storage.
type NoSubscriber struct{}

var noSubscriber NoSubscriber

func DowncastSubscriber[S any](d *Dispatch, ctx rustfmt.Context) (*S, bool) {
	if d == nil {
		value, ok := any(&noSubscriber).(*S)
		return value, ok
	}
	d.check(ctx)
	value, ok := d.state.subscriber.(*subscriberCell[S])
	if !ok {
		return nil, false
	}
	return value.state.Borrow(), true
}

// GetDefault borrows the exact current dispatcher with tracing's reentrancy
// suppression. Callback state is a borrowed operation witness; use its explicit
// Context for nested tracing and keep returned subscriber references in scope.
func GetDefault(scope *Scope, ctx rustfmt.Context, call func(*Dispatch, Context)) {
	c := callContext(scope, ctx)
	current(c.Scope, c.Panic, func(d *Dispatch) { call(d, c) })
}
func (d *WeakDispatch) Close() {
	if d != nil {
		d.live = false
		d.state = nil
	}
}
func (d *Dispatch) check(ctx rustfmt.Context) {
	if d == nil || !d.live || d.state == nil {
		ctx.Panic("rusttracing: consumed Dispatch")
	}
}
func (d *Dispatch) Close(ctx rustfmt.Context) { d.CloseWithScope(nil, ctx) }
func (d *Dispatch) CloseWithScope(scope *Scope, ctx rustfmt.Context) {
	if d == nil || !d.live {
		return
	}
	d.live = false
	s := d.state
	s.mu.Lock()
	s.refs--
	last := s.refs == 0
	sub := s.subscriber
	if last {
		s.subscriber = nil
	}
	s.mu.Unlock()
	if last {
		sub.close(callContext(scope, ctx))
	}
}
func cleanupOnPanic(ctx rustfmt.Context, cleanup func()) {
	defer func() {
		if recover() != nil {
			ctx.AbortCleanup()
		}
	}()
	cleanup()
}
func dropOnReturn(drop func(rustfmt.Context), ctx rustfmt.Context) {
	finishDrop(drop, ctx, recover())
}
func finishDrop(drop func(rustfmt.Context), ctx rustfmt.Context, p any) {
	if p != nil {
		if ctx.ProcessingPanic {
			ctx.ResumeUnwind(p)
		}
		ctx.Unwinding = true
		cleanupOnPanic(ctx, func() { drop(ctx) })
		ctx.ResumeUnwind(p)
	}
	drop(ctx)
}
func NewDispatchOwned[S any](runtime *Runtime, state *rustvec.Owned[S], ops SubscriberOps[S], context Context) *Dispatch {
	context = callContext(context.Scope, context.Panic)
	if runtime == nil {
		runtime = DefaultRuntime()
	}
	ctx := context.Panic
	if ops.Enabled == nil || ops.NewSpan == nil || ops.Record == nil || ops.RecordFollowsFrom == nil || ops.Event == nil || ops.Enter == nil || ops.Exit == nil {
		ctx.Panic("rusttracing: missing required Subscriber operation")
	}
	moved := state.MoveWithContext(ctx)
	d := &Dispatch{state: &dispatchState{refs: 1, subscriber: &subscriberCell[S]{moved, ops}, runtime: runtime}, live: true}
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if ctx.ProcessingPanic {
				ctx.ResumeUnwind(p)
			}
			if !complete {
				unwind := ctx
				unwind.Unwinding = true
				cleanupOnPanic(unwind, func() { d.CloseWithScope(context.Scope, unwind) })
			}
			ctx.ResumeUnwind(p)
		}
	}()
	runtime.register(d, context)
	complete = true
	return d
}

// Scope is one caller's tracing TLS witness. It is exclusively borrowed by
// synchronous calls and may not be shared concurrently. Dispatch may be shared.
// No goroutine IDs or process-global Go panic/TLS policies are used.
type Scope struct {
	runtime  *Runtime
	current  *Dispatch
	canEnter bool
	live     bool
	borrows  uint
}

func NewScope(dispatch *Dispatch) *Scope {
	r := DefaultRuntime()
	if dispatch != nil {
		dispatch.check(rustfmt.Context{})
		r = dispatch.state.runtime
	}
	s := &Scope{runtime: r, canEnter: true, live: true}
	if dispatch != nil {
		s.current = dispatch.Clone()
	}
	return s
}
func (s *Scope) Close(ctx rustfmt.Context) {
	if s == nil || !s.live {
		return
	}
	s.live = false
	previous := s.current
	s.current = nil
	previous.CloseWithScope(s, ctx)
}

type DefaultGuard struct {
	scope    *Scope
	previous *Dispatch
	live     bool
}

// These source sites use explicit selected-build location cells; filenames are
// never inferred from the host Cargo cache or from a Go stack frame.
const (
	PanicSiteSetDefaultBorrow   = "tracing_core::dispatcher::State::set_default.replace"
	PanicSiteDefaultGuardBorrow = "tracing_core::dispatcher::DefaultGuard::drop.replace"
)

func panicDefaultBorrow(ctx rustfmt.Context, site string) {
	ctx.PanicAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Payload: "RefCell already borrowed", Location: ctx.SourcePanicLocation(site), CanUnwind: true})
}

func (s *Scope) SetDefault(dispatch *Dispatch, ctx rustfmt.Context) *DefaultGuard {
	if s == nil || !s.live {
		ctx.Panic("rusttracing: consumed Scope")
	}
	var next *Dispatch
	if dispatch != nil {
		dispatch.check(ctx)
		if dispatch.state.runtime != s.runtime {
			ctx.Panic("rusttracing: mixed registry domains")
		}
		next = dispatch.Clone()
	}
	// State::set_default enables entry before RefCell::replace can fail.
	s.canEnter = true
	if s.borrows != 0 {
		defer dropOnReturn(func(c rustfmt.Context) { next.CloseWithScope(s, c) }, ctx)
		panicDefaultBorrow(ctx, PanicSiteSetDefaultBorrow)
	}
	g := &DefaultGuard{scope: s, previous: s.current, live: true}
	s.current = next
	s.canEnter = true
	return g
}
func (g *DefaultGuard) Close(ctx rustfmt.Context) {
	if g == nil || !g.live {
		return
	}
	g.live = false
	if g.scope.borrows != 0 {
		previous := g.previous
		g.previous = nil
		defer dropOnReturn(func(c rustfmt.Context) { previous.CloseWithScope(g.scope, c) }, ctx)
		panicDefaultBorrow(ctx, PanicSiteDefaultGuardBorrow)
	}
	previous := g.scope.current
	g.scope.current = g.previous
	g.previous = nil
	previous.CloseWithScope(g.scope, ctx)
}
func current(scope *Scope, ctx rustfmt.Context, call func(*Dispatch)) {
	if scope == nil {
		scope = ScopeFromContext(ctx)
	}
	if scope == nil || !scope.live || !scope.canEnter {
		call(nil)
		return
	}
	scope.canEnter = false
	scope.borrows++
	defer func() { scope.borrows--; scope.canEnter = true }()
	call(scope.current)
}

type Runtime struct {
	dispatchMu       sync.RWMutex
	dispatchPoisoned bool
	dispatchers      []*dispatchState
	justOne          atomic.Bool
	siteMu           sync.Mutex
	sites            []*Callsite
	maxLevel         atomic.Uint32
}

func NewRuntime() *Runtime { r := &Runtime{}; r.justOne.Store(true); return r }

var defaultRuntime = NewRuntime()

func DefaultRuntime() *Runtime { return defaultRuntime }

const (
	PanicSiteRegistryWritePoison = "tracing_core::callsite::Dispatchers::register_dispatch.write_unwrap"
	PanicSiteRegistryReadPoison  = "tracing_core::callsite::Dispatchers::rebuilder.read_unwrap"
)

func panicRegistryPoison(ctx rustfmt.Context, site string) {
	ctx.PanicAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Payload: "called `Result::unwrap()` on an `Err` value: PoisonError { .. }", Location: ctx.SourcePanicLocation(site), CanUnwind: true})
}

// std::sync::RwLock poisons only a writer that began outside panic and then
// unwinds. A reader never poisons it. The failed Result::unwrap still owns its
// acquired guard while the hook runs; the deferred operation releases it later.
func (r *Runtime) finishRegistryWrite(ctx rustfmt.Context, enteredPanicking bool) {
	p := recover()
	if p != nil && ctx.ProcessingPanic {
		ctx.ResumeUnwind(p)
	}
	if p != nil && !enteredPanicking {
		r.dispatchPoisoned = true
	}
	r.dispatchMu.Unlock()
	if p != nil {
		ctx.ResumeUnwind(p)
	}
}
func (r *Runtime) finishRegistryRead(ctx rustfmt.Context) {
	p := recover()
	if p != nil && ctx.ProcessingPanic {
		ctx.ResumeUnwind(p)
	}
	r.dispatchMu.RUnlock()
	if p != nil {
		ctx.ResumeUnwind(p)
	}
}
func (r *Runtime) register(d *Dispatch, ctx Context) {
	r.dispatchMu.Lock()
	defer r.finishRegistryWrite(ctx.Panic, ctx.Panic.Panicking())
	if r.dispatchPoisoned {
		panicRegistryPoison(ctx.Panic, PanicSiteRegistryWritePoison)
	}
	r.retainRegistered(ctx)
	r.dispatchers = append(r.dispatchers, d.state)
	r.justOne.Store(len(r.dispatchers) <= 1)
	d.state.subscriber.onRegister(d, ctx)
	r.rebuildWith(func(f func(*Dispatch)) { r.eachRegistered(ctx, f) }, ctx)
}
func (r *Runtime) retainRegistered(ctx Context) {
	// Vec::retain's predicate really upgrades and destroys each temporary
	// Dispatch. A concurrent last owner can therefore transfer subscriber Drop
	// to this thread, while the registry write lock is held.
	input := r.dispatchers
	kept, processed := 0, 0
	defer func() {
		if failure := recover(); failure != nil {
			copy(input[kept:], input[processed:])
			r.dispatchers = input[:kept+len(input)-processed]
			ctx.Panic.ResumeUnwind(failure)
		}
	}()
	for processed < len(input) {
		state := input[processed]
		temporary := state.upgrade()
		if temporary != nil {
			temporary.CloseWithScope(ctx.Scope, ctx.Panic)
			input[kept] = state
			kept++
		}
		processed++
	}
	r.dispatchers = input[:kept]
}
func (r *Runtime) eachRegistered(ctx Context, f func(*Dispatch)) {
	for _, state := range r.dispatchers {
		if d := state.upgrade(); d != nil {
			func() {
				defer dropOnReturn(func(c rustfmt.Context) { d.CloseWithScope(ctx.Scope, c) }, ctx.Panic)
				f(d)
			}()
		}
	}
}
func (r *Runtime) withRebuilder(ctx Context, call func(func(func(*Dispatch)))) {
	if r.justOne.Load() {
		call(func(f func(*Dispatch)) { current(ctx.Scope, ctx.Panic, f) })
		return
	}
	r.dispatchMu.RLock()
	defer r.finishRegistryRead(ctx.Panic)
	if r.dispatchPoisoned {
		panicRegistryPoison(ctx.Panic, PanicSiteRegistryReadPoison)
	}
	call(func(f func(*Dispatch)) { r.eachRegistered(ctx, f) })
}
func (r *Runtime) RebuildInterest(scope *Scope, ctx rustfmt.Context) {
	r.withRebuilder(callContext(scope, ctx), func(each func(func(*Dispatch))) { r.rebuildWith(each, callContext(scope, ctx)) })
}
func (r *Runtime) rebuildWith(each func(func(*Dispatch)), ctx Context) {
	max := Off
	each(func(d *Dispatch) {
		hint := Trace
		if d != nil {
			value, present := d.state.subscriber.maxLevel(ctx)
			hint = value
			if !present {
				hint = Trace
			}
		}
		if hint > max {
			max = hint
		}
	})
	r.siteMu.Lock()
	sites := append([]*Callsite(nil), r.sites...)
	r.siteMu.Unlock()
	for i := len(sites) - 1; i >= 0; i-- {
		sites[i].rebuild(each, ctx)
	}
	r.maxLevel.Store(uint32(max))
}

type Callsite struct {
	Metadata     Metadata
	runtime      *Runtime
	fields       []Field
	registration atomic.Uint32
	interest     atomic.Uint32
}

func NewCallsite(metadata Metadata) *Callsite { return NewCallsiteIn(DefaultRuntime(), metadata) }
func NewCallsiteIn(runtime *Runtime, metadata Metadata) *Callsite {
	if runtime == nil {
		runtime = DefaultRuntime()
	}
	c := &Callsite{runtime: runtime, Metadata: metadata}
	c.interest.Store(255)
	for i, name := range metadata.Fields {
		c.fields = append(c.fields, Field{name, uint(i), c})
	}
	return c
}
func (c *Callsite) Field(name string) *Field {
	for i := range c.fields {
		if c.fields[i].Name == name {
			return &c.fields[i]
		}
	}
	return nil
}
func (c *Callsite) rebuild(each func(func(*Dispatch)), ctx Context) {
	seen := false
	interest := Never
	each(func(d *Dispatch) {
		next := Never
		if d != nil {
			next = d.state.subscriber.register(&c.Metadata, ctx)
		}
		if !seen {
			interest = next
			seen = true
		} else {
			interest = combine(interest, next)
		}
	})
	c.interest.Store(uint32(interest))
}
func (c *Callsite) Interest(scope *Scope, ctx rustfmt.Context) Interest {
	if value := c.interest.Load(); value <= uint32(Always) {
		return Interest(value)
	}
	if c.registration.CompareAndSwap(0, 1) {
		c.runtime.siteMu.Lock()
		c.runtime.sites = append(c.runtime.sites, c)
		c.runtime.siteMu.Unlock()
		c.runtime.withRebuilder(callContext(scope, ctx), func(each func(func(*Dispatch))) { c.rebuild(each, callContext(scope, ctx)) })
		c.registration.Store(2)
	} else if c.registration.Load() != 2 {
		return Sometimes
	}
	if value := c.interest.Load(); value <= uint32(Always) {
		return Interest(value)
	}
	return Sometimes
}
func (c *Callsite) enabled(scope *Scope, ctx rustfmt.Context) bool {
	if uint32(c.Metadata.Level) > c.runtime.maxLevel.Load() {
		return false
	}
	interest := c.Interest(scope, ctx)
	if interest == Never {
		return false
	}
	if interest == Always {
		return true
	}
	enabled := false
	current(scope, ctx, func(d *Dispatch) {
		if d != nil {
			enabled = d.state.subscriber.enabled(&c.Metadata, callContext(scope, ctx))
		}
	})
	return enabled
}
