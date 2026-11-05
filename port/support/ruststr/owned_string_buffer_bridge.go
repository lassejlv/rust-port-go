// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped String/Vec<u8> ownership bridge for the owning OsString port.
package ruststr

import (
	"unicode/utf8"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// OwnedByteBufferIdentity observes an owning u8 buffer descriptor. Unallocated
// empty buffers use the same dangling identity as StringCell::BufferIdentity.
func OwnedByteBufferIdentity(data []byte) *byte {
	if cap(data) == 0 {
		return &ownedStringEmptyBuffer
	}
	return unsafe.SliceData(data)
}

// TakeUTF8BufferWithContext consumes this String and its cell. The returned
// slice is its owning Vec<u8> descriptor, including spare capacity. The caller
// must transfer it once, and must not retain aliases after that transfer.
func (s *OwnedString) TakeUTF8BufferWithContext(context rustfmt.Context) []byte {
	cell := s.TakeCellWithContext(context)
	state := cell.borrow(context)
	data := state.bytes
	state.bytes = nil
	state.live = false
	return data
}

// OwnedStringFromUTF8BufferWithContext consumes an owning Vec<u8> descriptor
// containing valid UTF-8, without copying or changing capacity. The caller
// relinquishes every mutable alias. An invalid input violates this bridge's
// precondition; OsString::into_string checks before calling this constructor.
func OwnedStringFromUTF8BufferWithContext(data []byte, context rustfmt.Context) *OwnedString {
	if !utf8.Valid(data) {
		context.Panic("ruststr: owning UTF-8 buffer contains invalid UTF-8")
	}
	return &OwnedString{owner: &ownedStringOwner{cell: &StringCell{state: &ownedStringState{bytes: data, live: true}}}}
}
