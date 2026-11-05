package rustcspan

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestSharedActualCellAndDestruction(t *testing.T) {
	var events []string
	first := NewShared(7, func(value *int, context rustfmt.Context) {
		events = append(events, fmt.Sprintf("drop:%d:%v", *value, context.Panicking()))
	})
	second := first.Clone()
	if first.StrongCount() != 2 || !first.PointerEqual(second) {
		t.Fatal("clone did not share actual cell/count")
	}
	if _, ok := first.GetMut(); ok {
		t.Fatal("shared value exposed an exclusive borrow")
	}
	first.Close()
	if second.StrongCount() != 1 || len(events) != 0 {
		t.Fatal("nonfinal owner destroyed the cell")
	}
	value, ok := second.GetMut()
	if !ok {
		t.Fatal("unique shared value lacks exclusive access")
	}
	*value = 9
	second.CloseWithContext(rustfmt.Context{Unwinding: true})
	second.Close()
	if !reflect.DeepEqual(events, []string{"drop:9:true"}) {
		t.Fatalf("destructor did not borrow actual final cell: %v", events)
	}
}
func TestSharedConcurrentOwnerCounts(t *testing.T) {
	var drops int
	owner := NewShared(1, func(*int, rustfmt.Context) { drops++ })
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 1000; j++ {
				copy := owner.Clone()
				if *copy.Borrow() != 1 {
					panic("wrong shared value")
				}
				copy.Close()
			}
		}()
	}
	workers.Wait()
	if owner.StrongCount() != 1 || drops != 0 {
		t.Fatal("reference counting lost a live owner")
	}
	owner.Close()
	if drops != 1 {
		t.Fatal("last owner did not destroy value exactly once")
	}
}
func TestLockNoSyncAndUnwind(t *testing.T) {
	lock := NewLockWithMode(1, LockNoSync)
	guard := lock.Lock()
	*guard.Get() = 2
	if other, ok := lock.TryLock(); ok {
		other.Close()
		t.Fatal("reentrant try-lock succeeded")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("NoSync reentry did not fail")
			}
		}()
		lock.Lock()
	}()
	guard.Close()
	again := lock.Lock()
	if *again.Get() != 2 {
		t.Fatal("lock guard borrowed a copy")
	}
	again.Close()
}
func TestFreezeCloneAndIrreversibleMutation(t *testing.T) {
	lock := NewFreezeLock([]int{1, 2})
	if _, ok := lock.Get(); ok {
		t.Fatal("unfrozen value was exposed without a guard")
	}
	guard := lock.Write()
	(*guard.Get())[0] = 9
	frozen := guard.Freeze()
	guard.Close()
	if !lock.IsFrozen() || (*frozen)[0] != 9 {
		t.Fatal("write guard failed to publish frozen value")
	}
	if guard, ok := lock.TryWrite(); ok {
		guard.Close()
		t.Fatal("frozen value admitted mutation")
	}
	copy := lock.Clone(func(value *[]int) []int { return append([]int(nil), (*value)...) })
	if !copy.IsFrozen() {
		t.Fatal("clone lost frozen state")
	}
	if data, ok := copy.Get(); !ok || !reflect.DeepEqual(*data, []int{9, 2}) {
		t.Fatal("clone lost value")
	}
}
func TestFreezeTryWriteBlocksUntilReadGuardCloses(t *testing.T) {
	lock := NewFreezeLock(1)
	read := lock.Read()
	started := make(chan struct{})
	finished := make(chan bool, 1)
	go func() {
		close(started)
		guard, ok := lock.TryWrite()
		if ok {
			*guard.Get() = 2
			guard.Close()
		}
		finished <- ok
	}()
	<-started
	select {
	case <-finished:
		t.Fatal("TryWrite behaved like a nonblocking lock attempt")
	case <-time.After(10 * time.Millisecond):
	}
	read.Close()
	select {
	case ok := <-finished:
		if !ok {
			t.Fatal("unfrozen value rejected writer")
		}
	case <-time.After(time.Second):
		t.Fatal("read guard did not release writer")
	}
}
func TestMappedReadGuardTransfersUnlockOwnership(t *testing.T) {
	lock := NewRwLock(struct{ Value int }{3})
	original := lock.Read()
	mapped := MapRwReadGuard(original, func(value *struct{ Value int }) *int { return &value.Value })
	original.Close()
	if *mapped.Get() != 3 {
		t.Fatal("mapped guard did not borrow field")
	}
	if write, ok := lock.TryWrite(); ok {
		write.Close()
		t.Fatal("moved source guard released mapped guard's lock")
	}
	mapped.Close()
	write, ok := lock.TryWrite()
	if !ok {
		t.Fatal("mapped guard leaked read lock")
	}
	write.Close()
}
