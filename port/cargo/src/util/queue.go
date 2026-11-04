// Port of Cargo src/util/queue.rs.
// Upstream: rust-lang/cargo@24de4721844c7a4d839072bab6fb66d41c9bc1f4
// Source SHA-256: f1ca01db50fd9b0534af5b6b1160b6e47de86b4770ef59191a54346ede72eb2a
// Licensed under MIT OR Apache-2.0; see ../../LICENSE-MIT and LICENSE-APACHE.
package util

import (
	"sync"
	"time"
)

// Queue is a threadsafe FIFO shared by any number of producers and consumers.
// Push never blocks on capacity; PushBounded waits until the length is below
// the bound. A Queue must not be copied after first use. The zero value has a
// zero bound, so its PushBounded calls wait forever, just as NewQueue(0) does.
type Queue[T any] struct {
	mu        sync.Mutex
	items     []T
	head      int
	count     int
	bound     uint
	boundedCV *sync.Cond
	popHead   *queuePopWaiter
	popTail   *queuePopWaiter
}

// queuePopWaiter implements one timed condition-variable wait without spawning
// a goroutine. All list operations and channel closes occur under Queue.mu.
type queuePopWaiter struct {
	ready      chan struct{}
	prev, next *queuePopWaiter
	queued     bool
}

// NewQueue creates a queue with the given bound. A zero bound deliberately
// prevents every PushBounded from completing; it does not restrict Push.
func NewQueue[T any](bound uint) *Queue[T] {
	queue := &Queue[T]{bound: bound}
	queue.boundedCV = sync.NewCond(&queue.mu)
	return queue
}

// Push adds an item regardless of the queue's capacity.
func (queue *Queue[T]) Push(item T) {
	queue.mu.Lock()
	queue.pushLocked(item)
	queue.wakePopperLocked()
	queue.mu.Unlock()
}

// PushBounded adds an item, waiting while the queue length is at least its bound.
// As in Cargo, this wait has no cancellation or timeout operation.
func (queue *Queue[T]) PushBounded(item T) {
	queue.mu.Lock()
	queue.initBoundedLocked()
	for uint(queue.count) >= queue.bound {
		queue.boundedCV.Wait()
	}
	queue.pushLocked(item)
	queue.wakePopperLocked()
	queue.mu.Unlock()
}

// Pop removes the oldest item, waiting at most timeout when the queue is empty.
// An already-queued item is returned even for a zero timeout. The boolean is
// false on timeout, and the returned value is then the zero value of T.
func (queue *Queue[T]) Pop(timeout Duration) (T, bool) {
	queue.mu.Lock()
	remaining := timeout
	checkpoint := time.Now()
	for {
		if queue.count != 0 {
			item := queue.popLocked()
			if uint(queue.count) < queue.bound && queue.boundedCV != nil {
				// Cargo assumes waiting threads cannot be canceled.
				queue.boundedCV.Signal()
			}
			queue.mu.Unlock()
			return item, true
		}
		now := time.Now()
		remaining = remaining.subtractElapsed(now.Sub(checkpoint))
		checkpoint = now
		if remaining.isZero() {
			queue.mu.Unlock()
			var zero T
			return zero, false
		}

		waiter := &queuePopWaiter{ready: make(chan struct{})}
		queue.addPopperLocked(waiter)
		timer := time.NewTimer(remaining.waitInterval())
		queue.mu.Unlock()
		select {
		case <-waiter.ready:
		case <-timer.C:
		}
		timer.Stop()
		queue.mu.Lock()
		queue.removePopperLocked(waiter)
		// Rust checks the predicate before declaring timeout, including when
		// an item arrived while the timed-out consumer reacquired the mutex.
		// Another consumer may also have taken a signaled item meanwhile.
	}
}

// TryPopAll drains the queue in FIFO order without waiting for an item and wakes
// every bounded producer, including when the queue was already empty.
func (queue *Queue[T]) TryPopAll() []T {
	queue.mu.Lock()
	items := make([]T, queue.count)
	for index := range items {
		items[index] = queue.popLocked()
	}
	if queue.boundedCV != nil {
		queue.boundedCV.Broadcast()
	}
	queue.mu.Unlock()
	return items
}

func (queue *Queue[T]) initBoundedLocked() {
	if queue.boundedCV == nil {
		queue.boundedCV = sync.NewCond(&queue.mu)
	}
}

func (queue *Queue[T]) pushLocked(item T) {
	if queue.count == len(queue.items) {
		capacity := max(4, len(queue.items)*2)
		items := make([]T, capacity)
		for index := 0; index < queue.count; index++ {
			items[index] = queue.items[(queue.head+index)%len(queue.items)]
		}
		queue.items = items
		queue.head = 0
	}
	queue.items[(queue.head+queue.count)%len(queue.items)] = item
	queue.count++
}

func (queue *Queue[T]) popLocked() T {
	item := queue.items[queue.head]
	var zero T
	queue.items[queue.head] = zero // Release references as VecDeque::pop_front does.
	queue.head = (queue.head + 1) % len(queue.items)
	queue.count--
	if queue.count == 0 {
		queue.head = 0
	}
	return item
}

func (queue *Queue[T]) addPopperLocked(waiter *queuePopWaiter) {
	waiter.prev, waiter.queued = queue.popTail, true
	if queue.popTail == nil {
		queue.popHead = waiter
	} else {
		queue.popTail.next = waiter
	}
	queue.popTail = waiter
}

func (queue *Queue[T]) removePopperLocked(waiter *queuePopWaiter) {
	if !waiter.queued {
		return
	}
	if waiter.prev == nil {
		queue.popHead = waiter.next
	} else {
		waiter.prev.next = waiter.next
	}
	if waiter.next == nil {
		queue.popTail = waiter.prev
	} else {
		waiter.next.prev = waiter.prev
	}
	waiter.prev, waiter.next, waiter.queued = nil, nil, false
}

func (queue *Queue[T]) wakePopperLocked() {
	if waiter := queue.popHead; waiter != nil {
		queue.removePopperLocked(waiter)
		close(waiter.ready)
	}
}
