// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped owning String/Vec<u8> methods from SDK1.99.1, source commit
// 58cf8f9c35b355199727627d07ae34a4e2846d67. No complete std String file credit.
package ruststr

import (
	"os"
	"unicode/utf8"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const OwnedStringCapacityPanicSite = "rust.alloc.string.capacity_overflow"

type ownedStringState struct {
	bytes []byte
	live  bool
}

// StringCell is the actual mutable String header cell. Copies are aliases,
// not Clone. Borrowed views obey Rust's lifetimes; they cannot be retained
// across mutation, allocation, owner move or destruction. This scoped surface
// preserves text, length/capacity, lawful push/reserve and buffer transfer.
type StringCell struct{ state *ownedStringState }
type ownedStringOwner struct{ cell *StringCell }
type OwnedString struct{ owner *ownedStringOwner }

func NewOwnedString() *OwnedString { return NewOwnedStringWithCapacity(0, rustfmt.Context{}) }
func NewOwnedStringWithCapacity(capacity uint, context rustfmt.Context) *OwnedString {
	return &OwnedString{owner: &ownedStringOwner{cell: &StringCell{state: &ownedStringState{bytes: ownedStringAllocate(capacity, context), live: true}}}}
}
func OwnedStringFromGo(value string, context rustfmt.Context) *OwnedString {
	if !utf8.ValidString(value) {
		context.Panic("ruststr: String input must be valid UTF-8")
	}
	result := NewOwnedStringWithCapacity(uint(len(value)), context)
	result.Borrow().PushStringWithContext(value, context)
	return result
}
func ownedStringAllocate(capacity uint, context rustfmt.Context) (result []byte) {
	if capacity > uint(^uint(0)>>1) {
		ownedStringCapacityOverflow(context)
	}
	defer func() {
		if recover() != nil {
			os.Exit(134)
		}
	}()
	return make([]byte, 0, int(capacity))
}
func ownedStringCapacityOverflow(context rustfmt.Context) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: "capacity overflow", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(OwnedStringCapacityPanicSite), CanUnwind: true})
}
func (s *StringCell) borrow(context rustfmt.Context) *ownedStringState {
	if s == nil || s.state == nil || !s.state.live {
		context.Panic("ruststr: consumed String cell")
	}
	return s.state
}
func (s *StringCell) Len() uint      { return uint(len(s.borrow(rustfmt.Context{}).bytes)) }
func (s *StringCell) Capacity() uint { return uint(cap(s.borrow(rustfmt.Context{}).bytes)) }
func (s *StringCell) AsStr() string  { return s.AsStrWithContext(rustfmt.Context{}) }
func (s *StringCell) AsStrWithContext(context rustfmt.Context) string {
	data := s.borrow(context).bytes
	return unsafe.String(unsafe.SliceData(data), len(data))
}
func (s *StringCell) Reserve(additional uint) { s.ReserveWithContext(additional, rustfmt.Context{}) }
func (s *StringCell) ReserveWithContext(additional uint, context rustfmt.Context) {
	data := s.borrow(context).bytes
	if additional <= uint(cap(data)-len(data)) {
		return
	}
	if additional > ^uint(0)-uint(len(data)) {
		ownedStringCapacityOverflow(context)
	}
	required := uint(len(data)) + additional
	capacity := uint(cap(data)) * 2
	if required > capacity {
		capacity = required
	}
	if capacity < 8 {
		capacity = 8
	}
	next := ownedStringAllocate(capacity, context)
	next = append(next, data...)
	s.state.bytes = next
}
func (s *StringCell) ReserveExactWithContext(additional uint, context rustfmt.Context) {
	data := s.borrow(context).bytes
	if additional <= uint(cap(data)-len(data)) {
		return
	}
	if additional > ^uint(0)-uint(len(data)) {
		ownedStringCapacityOverflow(context)
	}
	next := ownedStringAllocate(uint(len(data))+additional, context)
	s.state.bytes = append(next, data...)
}
func (s *StringCell) PushString(value string) { s.PushStringWithContext(value, rustfmt.Context{}) }
func (s *StringCell) PushStringWithContext(value string, context rustfmt.Context) {
	if !utf8.ValidString(value) {
		context.Panic("ruststr: push_str input must be valid UTF-8")
	}
	s.ReserveWithContext(uint(len(value)), context)
	s.state.bytes = append(s.state.bytes, value...)
}
func (s *StringCell) PushCharWithContext(value rune, context rustfmt.Context) {
	if value < 0 || value > utf8.MaxRune || value >= 0xd800 && value <= 0xdfff {
		context.Panic("ruststr: invalid char")
	}
	var bytes [4]byte
	n := utf8.EncodeRune(bytes[:], value)
	s.ReserveWithContext(uint(n), context)
	s.state.bytes = append(s.state.bytes, bytes[:n]...)
}
func (s *StringCell) MoveWithContext(context rustfmt.Context) *StringCell {
	state := s.borrow(context)
	next := &StringCell{state: &ownedStringState{bytes: state.bytes, live: true}}
	state.bytes = nil
	state.live = false
	return next
}
func (s *StringCell) CloseWithContext(context rustfmt.Context) {
	state := s.borrow(context)
	state.bytes = nil
	state.live = false
}
func (s *OwnedString) Borrow() *StringCell { return s.BorrowWithContext(rustfmt.Context{}) }
func (s *OwnedString) BorrowWithContext(context rustfmt.Context) *StringCell {
	if s == nil || s.owner == nil || s.owner.cell == nil {
		context.Panic("ruststr: consumed String owner")
	}
	s.owner.cell.borrow(context)
	return s.owner.cell
}
func (s *OwnedString) MoveWithContext(context rustfmt.Context) *OwnedString {
	cell := s.BorrowWithContext(context)
	s.owner.cell = nil
	return &OwnedString{owner: &ownedStringOwner{cell: cell}}
}

// TakeCell transfers ownership to an enclosing concrete source value. The
// enclosing value must later move/drop this cell, not copy its Go descriptor.
func (s *OwnedString) TakeCellWithContext(context rustfmt.Context) *StringCell {
	cell := s.BorrowWithContext(context)
	s.owner.cell = nil
	return cell
}
func OwnedStringFromMovedCell(cell *StringCell, context rustfmt.Context) *OwnedString {
	return &OwnedString{owner: &ownedStringOwner{cell: cell.MoveWithContext(context)}}
}
func (s *OwnedString) CloseWithContext(context rustfmt.Context) {
	cell := s.TakeCellWithContext(context)
	cell.CloseWithContext(context)
}
func (s *OwnedString) Close() { s.CloseWithContext(rustfmt.Context{}) }

// StringWriter borrows the actual cell only for this formatting operation;
// current Context is supplied at the call site, never captured by the String.
type StringWriter struct {
	Cell    *StringCell
	Context rustfmt.Context
}

func (w *StringWriter) Write(p []byte) (int, error) {
	w.Cell.PushStringWithContext(string(p), w.Context)
	return len(p), nil
}
func (w *StringWriter) WriteString(p string) (int, error) {
	w.Cell.PushStringWithContext(p, w.Context)
	return len(p), nil
}
func (w *StringWriter) WriteChar(c rune) error { w.Cell.PushCharWithContext(c, w.Context); return nil }

// EstimatedFormatCapacitySDK199 specializes the actual Arguments template scan
// after the caller has counted its literal bytes. The three inputs are static
// template facts, not an estimate inferred from the formatted output.
func EstimatedFormatCapacitySDK199(literalBytes uint, startsWithPlaceholder, plainLiteral bool) uint {
	if plainLiteral {
		return literalBytes
	}
	if startsWithPlaceholder && literalBytes < 16 {
		return 0
	}
	return literalBytes * 2
}

// Clone uses the actual u8 slice specialization: a distinct buffer whose
// capacity is the source LENGTH, not its spare capacity.
func (s *StringCell) CloneWithContext(context rustfmt.Context) *OwnedString {
	return OwnedStringFromGo(s.AsStrWithContext(context), context)
}

// CloneFrom follows slice::SpecCloneIntoVec<u8>: clear the destination then
// extend_from_slice. Existing capacity is reused; necessary growth is amortized.
func (s *StringCell) CloneFromWithContext(source *StringCell, context rustfmt.Context) {
	data := source.borrow(context).bytes
	target := s.borrow(context)
	if source.state == s.state {
		context.Panic("ruststr: aliased mutable String clone_from")
	}
	target.bytes = target.bytes[:0]
	s.ReserveWithContext(uint(len(data)), context)
	s.state.bytes = append(s.state.bytes, data...)
}
func (s *OwnedString) CloneWithContext(context rustfmt.Context) *OwnedString {
	return s.BorrowWithContext(context).CloneWithContext(context)
}

var ownedStringEmptyBuffer byte

// BufferIdentity is a within-process identity witness, not a Rust address/ABI.
// All unallocated empty u8 buffers share the source dangling-pointer identity;
// empty strings WITH capacity retain the identity of their real allocation.
func (s *StringCell) BufferIdentity() *byte {
	data := s.borrow(rustfmt.Context{}).bytes
	if cap(data) == 0 {
		return &ownedStringEmptyBuffer
	}
	return unsafe.SliceData(data)
}
