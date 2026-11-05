// SPDX-License-Identifier: MIT OR Apache-2.0
package rustvec

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func TestScopedConsumerProcessingWorker(t *testing.T) {
	if os.Getenv("FERRUGO_SCOPED_CONSUMER_PROCESSING") == "" {
		return
	}
	it := NewScopedIteratorFromOwned(NewOwned(struct{}{}, ValueOps[struct{}]{Drop: func(*struct{}, Context) { fmt.Println("iterator-cleanup") }}), IteratorOps[struct{}, uint]{Next: func(Ref[struct{}]) (uint, bool) { return 0, false }}, ScopedIteratorOps[struct{}, uint]{ForEachScoped: func(_ *Owned[struct{}], consumer BorrowedConsumer[uint]) {
		consumer.Call(1, Context{ProcessingPanic: true, Unwinding: true})
	}})
	it.ForEachScopedWithContext(Context{}, NewBorrowedConsumer(func(uint, Context) { fmt.Println("body"); defer fmt.Println("host-local"); panic("raw host callback") }))
	fmt.Println("after")
}
func TestScopedConsumerDynamicProcessingStopsOuterCleanup(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^TestScopedConsumerProcessingWorker$")
	cmd.Env = append(os.Environ(), "FERRUGO_SCOPED_CONSUMER_PROCESSING=1")
	output, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 134 || strings.TrimSpace(string(output)) != "body\nhost-local" {
		t.Fatalf("dynamic processing context escaped to outer cleanup: %v /%s", err, output)
	}
}

type scopedTestState struct {
	next  uint
	drops *atomic.Uint64
}

func TestScopedConsumerClonedStateRecursiveAndConcurrent(t *testing.T) {
	nested := Context{Unwinding: true, Runtime: &rustfmt.RuntimeMetadata{Value: "nested consumer"}}
	var drops, clones atomic.Uint64
	ops := ValueOps[scopedTestState]{Drop: func(_ *scopedTestState, c Context) {
		drops.Add(1)
		if c.Runtime == nil || c.Runtime.Value != "iterator caller" {
			t.Error("iterator Drop used consumer/construction metadata")
		}
	}}
	legacy := IteratorOps[scopedTestState, uint]{Next: func(r Ref[scopedTestState]) (uint, bool) {
		t.Error("selected scoped body was bypassed")
		return 0, false
	}}
	scoped := ScopedIteratorOps[scopedTestState, uint]{ForEachScoped: func(arg *Owned[scopedTestState], consumer BorrowedConsumer[uint]) {
		if arg.Context().Runtime == nil || arg.Context().Runtime.Value != "iterator caller" {
			t.Error("callback did not receive active owner scope")
		}
		for arg.Borrow().next < 2 {
			value := arg.Borrow().next
			arg.Borrow().next++
			consumer.Call(value, nested)
		}
	}}
	original := NewOwned(scopedTestState{drops: &drops}, ops)
	// Concrete source Clone is explicit; it clones the actual state before each
	// constructor, rather than copying an iterator descriptor/consumer bridge.
	cloneState := func(source *Owned[scopedTestState]) *Owned[scopedTestState] {
		clones.Add(1)
		return NewOwned(*source.Borrow(), ops)
	}
	cloned := cloneState(original)
	first := NewScopedIteratorFromOwned(original, legacy, scoped)
	second := NewScopedIteratorFromOwned(cloned, legacy, scoped)
	caller := Context{Runtime: &rustfmt.RuntimeMetadata{Value: "iterator caller"}}
	seen := []string{}
	first.ForEachScopedWithContext(caller, NewBorrowedConsumer(func(value uint, c Context) {
		if c != nested {
			t.Error("outer consumer lost its current context")
		}
		seen = append(seen, "first")
		if value == 0 {
			second.ForEachScopedWithContext(caller, NewBorrowedConsumer(func(value uint, c Context) {
				if c != nested {
					t.Error("recursive distinct consumer lost context")
				}
				seen = append(seen, "second")
			}))
		}
	}))
	if !reflect.DeepEqual(seen, []string{"first", "second", "second", "first"}) || drops.Load() != 2 || clones.Load() != 1 {
		t.Fatalf("shared clone bridge or incorrect ownership: %v drops%d clones%d", seen, drops.Load(), clones.Load())
	}
	var consumed atomic.Uint64
	var group sync.WaitGroup
	for n := 0; n < 12; n++ {
		// Independent source iterator owners and distinct FnMut borrows permit
		// concurrency; the shared atomic is actual source interior mutability.
		it := NewScopedIteratorFromOwned(NewOwned(scopedTestState{drops: &drops}, ops), legacy, scoped)
		group.Add(1)
		go func(it Iterator[uint]) {
			defer group.Done()
			it.ForEachScopedWithContext(caller, NewBorrowedConsumer(func(value uint, c Context) {
				if c != nested {
					t.Error("distinct concurrent bridge lost scope")
				}
				consumed.Add(1)
			}))
		}(it)
	}
	group.Wait()
	if consumed.Load() != 24 || drops.Load() != 14 {
		t.Fatal("independent state/consumer ownership was lost")
	}
}

func TestScopedConsumerIntoIteratorAndFixedMapTransfer(t *testing.T) {
	for _, runtime := range []*rustfmt.RuntimeMetadata{nil, {Value: "observer A"}, {Value: "observer B"}} {
		caller := Context{Runtime: runtime}
		current := caller
		current.Unwinding = true
		each, fold, drops := 0, 0, 0
		makeIterator := func() Iterator[uint] {
			state := NewOwned(uint(7), ValueOps[uint]{Drop: func(value *uint, c Context) {
				drops++
				if c != caller || *value != 7 {
					t.Fatal("transferred iterator Drop scope/state changed")
				}
			}})
			return NewScopedIteratorFromOwned(state, IteratorOps[uint, uint]{Next: func(Ref[uint]) (uint, bool) { return 0, false }, SizeHint: func(r Ref[uint]) Hint {
				if r.Context != caller {
					t.Fatal("hint used cached context")
				}
				return Hint{Lower: 1}
			}}, ScopedIteratorOps[uint, uint]{ForEachScoped: func(arg *Owned[uint], consumer BorrowedConsumer[uint]) { each++; consumer.Call(*arg.Borrow(), current) }, FoldUnitScoped: func(arg *Owned[uint], consumer BorrowedConsumer[uint]) { fold++; consumer.Call(*arg.Borrow(), current) }})
		}
		it := FromIterator(makeIterator()).IntoIter(caller)
		mapped := it.MapToUnitScopedWithContext(caller, ElementOps[uint]{})
		if it.IsLive() || mapped.SizeHint(caller).Lower != 1 {
			t.Fatal("fixed Map did not consume/forward actual iterator")
		}
		count := 0
		mapped.ForEachScopedWithContext(caller, NewBorrowedConsumer(func(value uint, c Context) {
			count++
			if value != 7 || c != current {
				t.Fatal("fixed Map lost current consumer scope")
			}
		}))
		mapped.CloseWithContext(caller)
		if count != 1 || each != 0 || fold != 1 || drops != 1 {
			t.Fatalf("wrong Map dispatch or duplicate Drop: %d/%d/%d/%d", count, each, fold, drops)
		}
		mapping := UnitMapping[uint]{context: caller, ops: ElementOps[uint]{Drop: func(value *uint, c Context) {
			if c != current || *value != 9 {
				t.Fatal("mapping value lost explicit call scope")
			}
		}}}
		value := mapping.OwnWithContext(9, current)
		moved := mapping.ApplyWithContext(value, current)
		if value.IsLive() || moved.Context() != current {
			t.Fatal("mapper retained construction guard")
		}
		moved.DropWithContext(current)
	}
}
