// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual lock/freeze operations consumed by the span runtime, following
// rustc_data_structures/src/sync.rs and sync/{lock,freeze}.rs at the Rust pin.
// This supporting used-contract adapter grants no surrounding-file credit.
package rustcspan

import (
	"sync"
	"sync/atomic"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// spanPanicContext is the cold failure-site runtime witness. Production uses
// rustfmt.Context; a witness must panic or abort and must never return. Keeping
// this structural lets tests observe ordering at the actual raise site.
type spanPanicContext interface{ Panic(any) }

type LockMode uint8

// The original lock uses track_caller. Each explicit operation supplies the
// actual selected caller's location; the lock does not infer a source filename.
const SyncLockReentryPanicSite = "rustc_data_structures::sync::Lock::lock.reentry"

const (
	LockNoSync LockMode = iota
	LockSync
)

var spanDynThreadSafeMode atomic.Uint32

func SetDynThreadSafeMode(enabled bool) { SetDynThreadSafeModeWithContext(enabled, rustfmt.Context{}) }
func SetDynThreadSafeModeWithContext(enabled bool, context rustfmt.Context) {
	value := uint32(1)
	if enabled {
		value = 2
	}
	if spanDynThreadSafeMode.CompareAndSwap(0, value) || spanDynThreadSafeMode.Load() == value {
		return
	}
	context.Panic("assertion failed: previous.is_ok() || previous == Err(set)")
}
func IsDynThreadSafe() bool { return IsDynThreadSafeWithContext(rustfmt.Context{}) }
func IsDynThreadSafeWithContext(context rustfmt.Context) bool {
	switch spanDynThreadSafeMode.Load() {
	case 1:
		return false
	case 2:
		return true
	default:
		context.Panic("uninitialized dyn_thread_safe mode!")
		panic("unreachable")
	}
}
func MightBeDynThreadSafe() bool { return spanDynThreadSafeMode.Load() != 1 }
func currentSpanLockMode() LockMode {
	if MightBeDynThreadSafe() {
		return LockSync
	}
	return LockNoSync
}

// spanMutex keeps the existing packed-span interner storage address stable
// while applying the same constructor-time mode and failure-site behavior as
// Lock. Its Mutex-style API preserves the earlier encoding inspection helpers.
type spanMutex struct {
	mu     sync.Mutex
	noSync bool
}

func newSpanMutex() spanMutex { return spanMutex{noSync: !MightBeDynThreadSafe()} }
func (m *spanMutex) Lock()    { m.LockWithContext(rustfmt.Context{}) }
func (m *spanMutex) LockWithContext(context spanPanicContext) {
	if m.noSync {
		if !m.mu.TryLock() {
			if current, ok := context.(rustfmt.Context); ok {
				current.PanicAt(rustfmt.SourcePanicInfo{Payload: "lock was already held", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: current.SourcePanicLocation(SyncLockReentryPanicSite)})
			}
			// Structural cold-site witnesses retain the actual-site inspection
			// hook used by the interner lock-order tests.
			context.Panic("lock was already held")
		}
	} else {
		m.mu.Lock()
	}
}
func (m *spanMutex) TryLock() bool { return m.mu.TryLock() }
func (m *spanMutex) Unlock()       { m.mu.Unlock() }

type spanGuardLease struct {
	closed atomic.Bool
	unlock func()
}

func (g *spanGuardLease) close() {
	if g != nil && !g.closed.Swap(true) {
		g.unlock()
	}
}
func (g *spanGuardLease) check(context rustfmt.Context) {
	if g == nil || g.closed.Load() {
		context.Panic("rustcspan: consumed lock guard")
	}
}

// Lock snapshots the source dynamic-synchronization mode at construction.
// NoSync is single-thread-only and fails instead of waiting on reentry; Sync
// blocks. Guards own unlock operations and borrow the actual value cell. Locks
// and guards must not be copied; explicit Close supplies source Drop timing.
type Lock[T any] struct {
	value  T
	mode   LockMode
	mu     sync.Mutex
	closed atomic.Bool
	drop   func(*T, rustfmt.Context)
}

func NewLock[T any](value T) *Lock[T] { return NewLockWithMode(value, currentSpanLockMode()) }
func NewLockWithMode[T any](value T, mode LockMode) *Lock[T] {
	if mode > LockSync {
		panic("invalid source lock mode")
	}
	return &Lock[T]{value: value, mode: mode}
}
func NewLockWithDrop[T any](value T, drop func(*T, rustfmt.Context)) *Lock[T] {
	lock := NewLock(value)
	lock.drop = drop
	return lock
}
func (l *Lock[T]) check(context rustfmt.Context) {
	if l == nil || l.closed.Load() {
		context.Panic("rustcspan: consumed lock")
	}
}
func (l *Lock[T]) Lock() *LockGuard[T] { return l.LockWithContext(rustfmt.Context{}) }
func (l *Lock[T]) LockWithContext(context rustfmt.Context) *LockGuard[T] {
	l.check(context)
	if l.mode == LockNoSync {
		if !l.mu.TryLock() {
			context.PanicAt(rustfmt.SourcePanicInfo{Payload: "lock was already held", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: context.SourcePanicLocation(SyncLockReentryPanicSite)})
		}
	} else {
		l.mu.Lock()
	}
	return &LockGuard[T]{data: &l.value, lease: &spanGuardLease{unlock: l.mu.Unlock}}
}
func (l *Lock[T]) TryLock() (*LockGuard[T], bool) {
	l.check(rustfmt.Context{})
	if !l.mu.TryLock() {
		return nil, false
	}
	return &LockGuard[T]{data: &l.value, lease: &spanGuardLease{unlock: l.mu.Unlock}}, true
}
func (l *Lock[T]) Borrow() *LockGuard[T]    { return l.Lock() }
func (l *Lock[T]) BorrowMut() *LockGuard[T] { return l.Lock() }
func (l *Lock[T]) BorrowWithContext(context rustfmt.Context) *LockGuard[T] {
	return l.LockWithContext(context)
}
func (l *Lock[T]) BorrowMutWithContext(context rustfmt.Context) *LockGuard[T] {
	return l.LockWithContext(context)
}
func (l *Lock[T]) GetMut() *T   { l.check(rustfmt.Context{}); return &l.value }
func (l *Lock[T]) IntoInner() T { l.check(rustfmt.Context{}); l.closed.Store(true); return l.value }
func (l *Lock[T]) Close()       { l.CloseWithContext(rustfmt.Context{}) }
func (l *Lock[T]) CloseWithContext(context rustfmt.Context) {
	if l == nil || l.closed.Swap(true) {
		return
	}
	if l.drop != nil {
		l.drop(&l.value, context)
	}
}

type LockGuard[T any] struct {
	data  *T
	lease *spanGuardLease
}

func (g *LockGuard[T]) Get() *T { g.lease.check(rustfmt.Context{}); return g.data }
func (g *LockGuard[T]) Close() {
	if g != nil {
		g.lease.close()
	}
}

// RwLock always synchronizes, including when Lock's dynamic mode is NoSync.
// That distinction follows sync.rs's fixed ERROR_CHECKING=false branch.
type RwLock[T any] struct {
	value  T
	mu     sync.RWMutex
	closed atomic.Bool
	drop   func(*T, rustfmt.Context)
}

func NewRwLock[T any](value T) *RwLock[T] { return &RwLock[T]{value: value} }
func NewRwLockWithDrop[T any](value T, drop func(*T, rustfmt.Context)) *RwLock[T] {
	return &RwLock[T]{value: value, drop: drop}
}
func (l *RwLock[T]) check(context rustfmt.Context) {
	if l == nil || l.closed.Load() {
		context.Panic("rustcspan: consumed RwLock")
	}
}
func (l *RwLock[T]) Read() *RwReadGuard[T] { return l.ReadWithContext(rustfmt.Context{}) }
func (l *RwLock[T]) ReadWithContext(context rustfmt.Context) *RwReadGuard[T] {
	l.check(context)
	l.mu.RLock()
	return &RwReadGuard[T]{data: &l.value, lease: &spanGuardLease{unlock: l.mu.RUnlock}}
}
func (l *RwLock[T]) Write() *RwWriteGuard[T] { return l.WriteWithContext(rustfmt.Context{}) }
func (l *RwLock[T]) WriteWithContext(context rustfmt.Context) *RwWriteGuard[T] {
	l.check(context)
	l.mu.Lock()
	return &RwWriteGuard[T]{data: &l.value, lease: &spanGuardLease{unlock: l.mu.Unlock}}
}
func (l *RwLock[T]) TryWrite() (*RwWriteGuard[T], bool) {
	l.check(rustfmt.Context{})
	if !l.mu.TryLock() {
		return nil, false
	}
	return &RwWriteGuard[T]{data: &l.value, lease: &spanGuardLease{unlock: l.mu.Unlock}}, true
}
func (l *RwLock[T]) Borrow() *RwReadGuard[T]     { return l.Read() }
func (l *RwLock[T]) BorrowMut() *RwWriteGuard[T] { return l.Write() }
func (l *RwLock[T]) GetMut() *T                  { l.check(rustfmt.Context{}); return &l.value }
func (l *RwLock[T]) IntoInner() T                { l.check(rustfmt.Context{}); l.closed.Store(true); return l.value }
func (l *RwLock[T]) Close()                      { l.CloseWithContext(rustfmt.Context{}) }
func (l *RwLock[T]) CloseWithContext(context rustfmt.Context) {
	if l == nil || l.closed.Swap(true) {
		return
	}
	if l.drop != nil {
		l.drop(&l.value, context)
	}
}

type RwReadGuard[T any] struct {
	data  *T
	lease *spanGuardLease
}
type RwWriteGuard[T any] struct {
	data  *T
	lease *spanGuardLease
}

func (g *RwReadGuard[T]) Get() *T { g.lease.check(rustfmt.Context{}); return g.data }
func (g *RwReadGuard[T]) Close() {
	if g != nil {
		g.lease.close()
	}
}
func (g *RwWriteGuard[T]) Get() *T { g.lease.check(rustfmt.Context{}); return g.data }
func (g *RwWriteGuard[T]) Close() {
	if g != nil {
		g.lease.close()
	}
}
func MapRwReadGuard[T, U any](guard *RwReadGuard[T], mapValue func(*T) *U) *RwReadGuard[U] {
	return MapRwReadGuardWithContext(guard, func(value *T, _ rustfmt.Context) *U { return mapValue(value) }, rustfmt.Context{})
}
func MapRwReadGuardWithContext[T, U any](guard *RwReadGuard[T], mapValue func(*T, rustfmt.Context) *U, context rustfmt.Context) (result *RwReadGuard[U]) {
	defer func() {
		if failure := recover(); failure != nil {
			if context.ProcessingPanic {
				context.ResumeUnwind(failure)
			}
			guard.Close()
			context.ResumeUnwind(failure)
		}
	}()
	data := mapValue(guard.Get(), context)
	lease := guard.lease
	guard.lease = nil
	return &RwReadGuard[U]{data: data, lease: lease}
}

// FreezeLock permits mutation until a one-way freeze, then exposes immutable
// lock-free borrows. TryWrite blocks on the RwLock and returns false only when
// frozen; it is not a nonblocking lock attempt. Borrow lifetimes, exclusive
// guard access and no use after owner destruction are source preconditions.
type FreezeLock[T any] struct {
	value  T
	frozen atomic.Bool
	mu     sync.RWMutex
	closed atomic.Bool
	drop   func(*T, rustfmt.Context)
}

func NewFreezeLock[T any](value T) *FreezeLock[T]    { return NewFreezeLockWithState(value, false) }
func FrozenFreezeLock[T any](value T) *FreezeLock[T] { return NewFreezeLockWithState(value, true) }
func NewFreezeLockWithState[T any](value T, frozen bool) *FreezeLock[T] {
	l := &FreezeLock[T]{value: value}
	l.frozen.Store(frozen)
	return l
}
func NewFreezeLockWithDrop[T any](value T, frozen bool, drop func(*T, rustfmt.Context)) *FreezeLock[T] {
	l := NewFreezeLockWithState(value, frozen)
	l.drop = drop
	return l
}
func (l *FreezeLock[T]) check(context rustfmt.Context) {
	if l == nil || l.closed.Load() {
		context.Panic("rustcspan: consumed FreezeLock")
	}
}
func (l *FreezeLock[T]) IsFrozen() bool { l.check(rustfmt.Context{}); return l.frozen.Load() }
func (l *FreezeLock[T]) Get() (*T, bool) {
	l.check(rustfmt.Context{})
	if l.frozen.Load() {
		return &l.value, true
	}
	return nil, false
}
func (l *FreezeLock[T]) Read() *FreezeReadGuard[T] { return l.ReadWithContext(rustfmt.Context{}) }
func (l *FreezeLock[T]) ReadWithContext(context rustfmt.Context) *FreezeReadGuard[T] {
	l.check(context)
	var lease *spanGuardLease
	if !l.frozen.Load() {
		l.mu.RLock()
		lease = &spanGuardLease{unlock: l.mu.RUnlock}
	}
	return &FreezeReadGuard[T]{data: &l.value, lease: lease}
}
func (l *FreezeLock[T]) Borrow() *FreezeReadGuard[T] { return l.Read() }
func (l *FreezeLock[T]) BorrowWithContext(context rustfmt.Context) *FreezeReadGuard[T] {
	return l.ReadWithContext(context)
}
func (l *FreezeLock[T]) TryWrite() (*FreezeWriteGuard[T], bool) {
	return l.TryWriteWithContext(rustfmt.Context{})
}
func (l *FreezeLock[T]) TryWriteWithContext(context rustfmt.Context) (*FreezeWriteGuard[T], bool) {
	l.check(context)
	l.mu.Lock()
	if l.frozen.Load() {
		l.mu.Unlock()
		return nil, false
	}
	return &FreezeWriteGuard[T]{data: &l.value, frozen: &l.frozen, lease: &spanGuardLease{unlock: l.mu.Unlock}}, true
}
func (l *FreezeLock[T]) Write() *FreezeWriteGuard[T] { return l.WriteWithContext(rustfmt.Context{}) }
func (l *FreezeLock[T]) WriteWithContext(context rustfmt.Context) *FreezeWriteGuard[T] {
	g, ok := l.TryWriteWithContext(context)
	if !ok {
		context.Panic("data should not be frozen if we're still attempting to mutate it")
	}
	return g
}
func (l *FreezeLock[T]) Freeze() *T { return l.FreezeWithContext(rustfmt.Context{}) }
func (l *FreezeLock[T]) FreezeWithContext(context rustfmt.Context) *T {
	l.check(context)
	if !l.frozen.Load() {
		l.mu.Lock()
		l.frozen.Store(true)
		l.mu.Unlock()
	}
	return &l.value
}
func (l *FreezeLock[T]) CloneWithContext(cloneValue func(*T, rustfmt.Context) T, context rustfmt.Context) *FreezeLock[T] {
	g := l.ReadWithContext(context)
	defer g.Close()
	value := cloneValue(g.Get(), context)
	copy := NewFreezeLockWithState(value, l.IsFrozen())
	copy.drop = l.drop
	return copy
}
func (l *FreezeLock[T]) Clone(cloneValue func(*T) T) *FreezeLock[T] {
	return l.CloneWithContext(func(value *T, _ rustfmt.Context) T { return cloneValue(value) }, rustfmt.Context{})
}
func (l *FreezeLock[T]) Close() { l.CloseWithContext(rustfmt.Context{}) }
func (l *FreezeLock[T]) CloseWithContext(context rustfmt.Context) {
	if l == nil || l.closed.Swap(true) {
		return
	}
	if l.drop != nil {
		l.drop(&l.value, context)
	}
}

type FreezeReadGuard[T any] struct {
	data   *T
	lease  *spanGuardLease
	closed bool
}
type FreezeWriteGuard[T any] struct {
	data   *T
	lease  *spanGuardLease
	frozen *atomic.Bool
}

func (g *FreezeReadGuard[T]) Get() *T {
	if g == nil || g.closed {
		panic("rustcspan: consumed freeze read guard")
	}
	return g.data
}
func (g *FreezeReadGuard[T]) Close() {
	if g != nil && !g.closed {
		g.closed = true
		g.lease.close()
	}
}
func (g *FreezeWriteGuard[T]) Get() *T { g.lease.check(rustfmt.Context{}); return g.data }
func (g *FreezeWriteGuard[T]) Close() {
	if g != nil {
		g.lease.close()
	}
}
func (g *FreezeWriteGuard[T]) Freeze() *T {
	g.lease.check(rustfmt.Context{})
	g.frozen.Store(true)
	g.lease.close()
	return g.data
}
