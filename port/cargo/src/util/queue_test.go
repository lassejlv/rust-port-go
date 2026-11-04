package util

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

func queueReceive[T any](t *testing.T, values <-chan T) T {
	t.Helper()
	select {
	case value := <-values:
		return value
	case <-time.After(5 * time.Second):
		t.Fatal("queue operation did not complete")
		var zero T
		return zero
	}
}

func queueBlocked[T any](t *testing.T, done <-chan T) {
	t.Helper()
	select {
	case value := <-done:
		t.Fatalf("operation completed while the queue was full: %v", value)
	case <-time.After(20 * time.Millisecond):
	}
}

// Inspect registration only to force test interleavings; assertions concern
// public queue operations, rather than reproducing the implementation.
func queueWaitForPoppers[T any](t *testing.T, queue *Queue[T], count int) {
	t.Helper()
	queueLockWithPoppers(t, queue, count)
	queue.mu.Unlock()
}

// Return with the mutex held so the caller can act on the observed registration
// without a scheduling gap between observing it and acquiring the mutex.
func queueLockWithPoppers[T any](t *testing.T, queue *Queue[T], count int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		queue.mu.Lock()
		found := 0
		for waiter := queue.popHead; waiter != nil; waiter = waiter.next {
			found++
		}
		if found == count {
			return
		}
		queue.mu.Unlock()
		runtime.Gosched()
	}
	t.Fatalf("did not register %d waiting consumers", count)
}

func TestQueueFIFOAndUnboundedPush(t *testing.T) {
	queue := NewQueue[int](2)
	if value, ok := queue.Pop(Duration{}); ok || value != 0 {
		t.Fatalf("empty pop = %d, %v", value, ok)
	}
	// Grow, wrap and grow again while checking against an independent FIFO.
	var expected []int
	seed := uint64(0x7310acbf)
	for step := 0; step < 20_000; step++ {
		seed = seed*6364136223846793005 + 1442695040888963407
		switch seed >> 61 {
		case 0, 1, 2, 3, 4:
			queue.Push(step) // Deliberately exceeds the bound of two.
			expected = append(expected, step)
		case 5, 6:
			value, ok := queue.Pop(Duration{})
			if len(expected) == 0 {
				if ok {
					t.Fatalf("step %d: popped %d from empty queue", step, value)
				}
			} else {
				if !ok || value != expected[0] {
					t.Fatalf("step %d: pop = %d,%v; want %d,true", step, value, ok, expected[0])
				}
				expected = expected[1:]
			}
		case 7:
			if got := queue.TryPopAll(); len(got) != len(expected) || !reflect.DeepEqual(got, append([]int{}, expected...)) {
				t.Fatalf("step %d: drain = %v; want %v", step, got, expected)
			}
			expected = nil
		}
	}
	if got := queue.TryPopAll(); !reflect.DeepEqual(got, append([]int{}, expected...)) {
		t.Fatalf("final drain = %v; want %v", got, expected)
	}
}

func TestQueueOptionalNilAndReleasedReferences(t *testing.T) {
	queue := NewQueue[*int](1)
	queue.Push(nil)
	if value, ok := queue.Pop(Duration{}); !ok || value != nil {
		t.Fatalf("queued nil = %v,%v", value, ok)
	}
	for value := 0; value < 30; value++ {
		copy := value
		queue.Push(&copy)
	}
	queue.Pop(Duration{})
	if got := queue.TryPopAll(); len(got) != 29 || *got[0] != 1 || *got[28] != 29 {
		t.Fatalf("pointer FIFO drain = %v", got)
	}
	for _, retained := range queue.items {
		if retained != nil {
			t.Fatal("an item reference was retained after draining")
		}
	}
}

func TestQueueBoundedCapacityAfterUnboundedPush(t *testing.T) {
	queue := NewQueue[int](2)
	for value := 1; value <= 4; value++ {
		queue.Push(value)
	}
	done := make(chan int, 2)
	for value := 5; value <= 6; value++ {
		go func(value int) { queue.PushBounded(value); done <- value }(value)
	}
	queueBlocked(t, done)
	for want := 1; want <= 2; want++ {
		if value, ok := queue.Pop(Duration{}); !ok || value != want {
			t.Fatalf("pop = %d,%v; want %d,true", value, ok, want)
		}
	}
	queueBlocked(t, done) // Equal to the bound is still full.
	queue.Pop(Duration{})
	first := queueReceive(t, done)
	queueBlocked(t, done) // One free slot wakes exactly one producer.
	if value, ok := queue.Pop(Duration{}); !ok || value != 4 {
		t.Fatalf("fourth FIFO item = %d,%v", value, ok)
	}
	second := queueReceive(t, done)
	if first == second || first+second != 11 {
		t.Fatalf("producer completions = %d,%d", first, second)
	}
	if got := queue.TryPopAll(); !reflect.DeepEqual(got, []int{first, second}) {
		t.Fatalf("bounded FIFO drain = %v; want [%d %d]", got, first, second)
	}
}

func TestQueueDrainWakesAllBoundedProducers(t *testing.T) {
	const bound = 12
	queue := NewQueue[int](bound)
	for value := 0; value < bound; value++ {
		queue.Push(value)
	}
	done := make(chan int, bound)
	for value := 0; value < bound; value++ {
		go func(value int) { queue.PushBounded(value + 100); done <- value }(value)
	}
	queueBlocked(t, done)
	if got := queue.TryPopAll(); len(got) != bound || got[0] != 0 || got[bound-1] != bound-1 {
		t.Fatalf("initial drain = %v", got)
	}
	for range bound {
		queueReceive(t, done)
	}
	got := queue.TryPopAll()
	sort.Ints(got)
	for value := 0; value < bound; value++ {
		if len(got) != bound || got[value] != value+100 {
			t.Fatalf("producers after drain = %v", got)
		}
	}
}

func TestQueueMultipleWaitingConsumers(t *testing.T) {
	const consumers = 20
	queue := NewQueue[int](1)
	results := make(chan int, consumers)
	for range consumers {
		go func() {
			value, ok := queue.Pop(DurationFromGo(5 * time.Second))
			if !ok {
				value = -1
			}
			results <- value
		}()
	}
	queueWaitForPoppers(t, queue, consumers)
	queue.Push(0)
	if got := queueReceive(t, results); got != 0 {
		t.Fatalf("first consumer = %d", got)
	}
	queueWaitForPoppers(t, queue, consumers-1)
	for value := 1; value < consumers; value++ {
		queue.Push(value)
	}
	seen := make([]bool, consumers)
	for range consumers - 1 {
		value := queueReceive(t, results)
		if value <= 0 || value >= consumers || seen[value] {
			t.Fatalf("missing, duplicate or timed-out consumer: %d", value)
		}
		seen[value] = true
	}
	queueWaitForPoppers(t, queue, 0)
}

func TestQueueConcurrentProducersAndConsumers(t *testing.T) {
	const producers, consumers, perProducer = 8, 8, 1000
	queue := NewQueue[int](7)
	results := make(chan int, producers*perProducer)
	var workers sync.WaitGroup
	for producer := 0; producer < producers; producer++ {
		workers.Add(1)
		go func(producer int) {
			defer workers.Done()
			for item := 0; item < perProducer; item++ {
				value := producer*perProducer + item
				if item%17 == 0 {
					queue.Push(value)
				} else {
					queue.PushBounded(value)
				}
			}
		}(producer)
	}
	for range consumers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range producers * perProducer / consumers {
				value, ok := queue.Pop(DurationFromGo(5 * time.Second))
				if !ok {
					results <- -1
					return
				}
				results <- value
			}
		}()
	}
	finished := make(chan struct{})
	go func() { workers.Wait(); close(finished) }()
	queueReceive(t, finished)
	close(results)
	seen := make([]bool, producers*perProducer)
	count := 0
	for value := range results {
		if value < 0 || value >= len(seen) || seen[value] {
			t.Fatalf("invalid, duplicate or timed-out item: %d", value)
		}
		seen[value], count = true, count+1
	}
	if count != len(seen) || len(queue.TryPopAll()) != 0 {
		t.Fatalf("consumed %d items; want %d with an empty queue", count, len(seen))
	}
}

func TestQueueTimeoutAndSpuriousWakeups(t *testing.T) {
	queue := NewQueue[int](1)
	finished := make(chan time.Duration, 1)
	go func() {
		started := time.Now()
		_, ok := queue.Pop(DurationFromGo(80 * time.Millisecond))
		if ok {
			finished <- -1
			return
		}
		finished <- time.Since(started)
	}()
	queueWaitForPoppers(t, queue, 1)
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case elapsed := <-finished:
			if elapsed < 75*time.Millisecond || elapsed > time.Second {
				t.Fatalf("timeout with repeated empty notifications took %v", elapsed)
			}
			queueWaitForPoppers(t, queue, 0)
			return
		case <-ticker.C:
			queue.mu.Lock()
			queue.wakePopperLocked() // Legal condition-variable spurious notification.
			queue.mu.Unlock()
		}
	}
}

func TestQueueTimeoutReacquisitionChecksPredicateFirst(t *testing.T) {
	queue := NewQueue[int](1)
	result := make(chan int, 1)
	go func() {
		value, ok := queue.Pop(DurationFromGo(30 * time.Millisecond))
		if !ok {
			value = -1
		}
		result <- value
	}()
	queueLockWithPoppers(t, queue, 1)
	time.Sleep(50 * time.Millisecond) // Expire the timer while holding its mutex.
	queue.pushLocked(37)
	queue.mu.Unlock()
	if got := queueReceive(t, result); got != 37 {
		t.Fatalf("timed-out mutex reacquisition = %d; want queued item 37", got)
	}
}

func TestQueueFullRustDurationWakesEarly(t *testing.T) {
	queue := NewQueue[int](1)
	result := make(chan int, 1)
	go func() {
		value, ok := queue.Pop(NewDuration(math.MaxUint64, 999_999_999))
		if !ok {
			value = -1
		}
		result <- value
	}()
	queueWaitForPoppers(t, queue, 1)
	queue.Push(99)
	if got := queueReceive(t, result); got != 99 {
		t.Fatalf("full-duration pop = %d", got)
	}
}

func TestQueueTimeoutsRemoveWaitersAndDoNotLeakGoroutines(t *testing.T) {
	queue := NewQueue[int](1)
	baseline := runtime.NumGoroutine()
	var workers sync.WaitGroup
	for range 200 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			queue.Pop(DurationFromGo(time.Millisecond))
		}()
	}
	workers.Wait()
	queueWaitForPoppers(t, queue, 0)
	deadline := time.Now().Add(time.Second)
	for runtime.NumGoroutine() > baseline+2 && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if got := runtime.NumGoroutine(); got > baseline+2 {
		t.Fatalf("goroutines after 200 timeouts = %d; baseline %d", got, baseline)
	}
	queue.Push(7)
	if value, ok := queue.Pop(Duration{}); !ok || value != 7 {
		t.Fatalf("push after timeouts = %d,%v", value, ok)
	}
}

func TestQueueZeroBound(t *testing.T) {
	if os.Getenv("FERRUGO_QUEUE_ZERO_BOUND_TEST") == "1" {
		var queue Queue[int] // Also exercise zero-value initialization.
		done := make(chan struct{}, 1)
		go func() { queue.PushBounded(99); done <- struct{}{} }()
		for value := 0; value < 3; value++ {
			queue.Push(value)
		}
		if value, ok := queue.Pop(Duration{}); !ok || value != 0 {
			t.Fatal("zero-bound queue restricted unbounded push")
		}
		if got := queue.TryPopAll(); !reflect.DeepEqual(got, []int{1, 2}) {
			t.Fatalf("zero-bound drain = %v", got)
		}
		queueBlocked(t, done)
		// A zero-bound producer is intentionally uncancelable in Cargo too.
		// Isolate it in a subprocess instead of leaking it into the test suite.
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestQueueZeroBound$")
	command.Env = append(os.Environ(), "FERRUGO_QUEUE_ZERO_BOUND_TEST=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("zero-bound subprocess: %v\n%s", err, output)
	}
}

func TestQueueRustDuration(t *testing.T) {
	cases := []struct {
		seconds uint64
		nanos   uint32
		wantSec uint64
		wantNS  uint32
	}{
		{0, 0, 0, 0}, {0, 1, 0, 1}, {1, 1_500_000_001, 2, 500_000_001},
		{0, math.MaxUint32, 4, 294_967_295},
		{math.MaxUint64, 999_999_999, math.MaxUint64, 999_999_999},
	}
	for _, test := range cases {
		got := NewDuration(test.seconds, test.nanos)
		if got.Seconds() != test.wantSec || got.Nanoseconds() != test.wantNS {
			t.Fatalf("NewDuration(%d,%d) = %d,%d", test.seconds, test.nanos, got.Seconds(), got.Nanoseconds())
		}
	}
	for _, value := range []time.Duration{0, 1, time.Second + 37, time.Duration(math.MaxInt64)} {
		got := DurationFromGo(value)
		if got.Seconds() != uint64(value/time.Second) || got.Nanoseconds() != uint32(value%time.Second) {
			t.Fatalf("Go adapter changed %v", value)
		}
	}
	for _, operation := range []func(){
		func() { NewDuration(math.MaxUint64, 1_000_000_000) },
		func() { NewDuration(math.MaxUint64-3, math.MaxUint32) },
		func() { DurationFromGo(-1) },
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Error("invalid duration did not panic")
				}
			}()
			operation()
		}()
	}
}

// The original-source comparison is opt-in so ordinary Go tests do not require
// Rust or an upstream checkout. CI can also run the same tool directly -check.
func TestQueuePinnedUpstreamOracle(t *testing.T) {
	if os.Getenv("FERRUGO_QUEUE_ORACLE") != "1" {
		t.Skip("set FERRUGO_QUEUE_ORACLE=1 to compare the unchanged pinned Cargo queue")
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	goTool, explicit := os.LookupEnv("FERRUGO_GO")
	if !explicit {
		goTool = filepath.Join(root, ".toolchain/go/bin/go")
		if info, err := os.Stat(goTool); err != nil || info.IsDir() {
			goTool = "go"
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, goTool, "run", "./tools/port-cargo-queue", "-check")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("pinned queue oracle: %v\n%s", err, output)
	} else {
		t.Log(strings.TrimSpace(string(output)))
	}
}
