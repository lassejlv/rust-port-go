// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"reflect"
	"testing"
)

type borrowedHasherState struct {
	writes   [][]byte
	finishes int
}

func (h *borrowedHasherState) Write(bytes []byte) {
	h.writes = append(h.writes, append([]byte(nil), bytes...))
}
func (h *borrowedHasherState) Finish64() uint64 { h.finishes++; return uint64(len(h.writes)) }
func TestOwnedHasherBorrowUsesActualStateAndRejectsConsumedOwner(t *testing.T) {
	state := &borrowedHasherState{}
	drops := 0
	owner := NewOwnedHasher(state, func(actual Hasher, _ Context) {
		if actual != state {
			t.Fatal("destructor received copied hasher")
		}
		drops++
	})
	borrow := owner.Borrow(Context{})
	if borrow != state {
		t.Fatal("borrow did not expose actual hasher")
	}
	borrow.Write([]byte{3, 5, 8})
	if borrow.Finish64() != 1 || state.finishes != 1 || !reflect.DeepEqual(state.writes, [][]byte{{3, 5, 8}}) || drops != 0 {
		t.Fatal("borrow changed ownership or copied mutable state")
	}
	owner.Close(Context{})
	if drops != 1 {
		t.Fatal("borrow changed hasher destruction")
	}
	var value any
	func() { defer func() { value = recover() }(); owner.Borrow(Context{}) }()
	if value != "rusthashmap: borrow of consumed owned Hasher" {
		t.Fatalf("consumed borrow was accepted: %v", value)
	}
	owner.Close(Context{})
	if drops != 1 {
		t.Fatal("consumed borrow duplicated destruction")
	}
}
