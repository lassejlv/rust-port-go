// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped SDK1.101 PathBuf/OsString storage used by pinned rustc_span.
// Exact source selection and native witnesses: tools/port-rust-owned-path.
// This is not a complete port of std's path/ffi/container source files.
package rustcspan

import (
	"os"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
)

const OwnedPathCapacityPanicSite = "rust.alloc.path.capacity_overflow"
const OwnedPathExtensionPanicSite = "rust.std.path.invalid_extension"

type rustOsStringState struct {
	bytes  []byte
	flavor RustPathFlavor
	live   bool
}

// RustOsStringCell is the actual mutable OsString header. Copies alias it.
// Borrowed views follow Rust lifetimes: no view may survive mutation, move or
// destruction. Owning handles must be explicitly moved, cloned or closed.
type RustOsStringCell struct{ state *rustOsStringState }
type rustOsStringOwner struct{ cell *RustOsStringCell }
type RustOsString struct{ owner *rustOsStringOwner }

func ownedPathCapacityOverflow(context rustfmt.Context) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: "capacity overflow", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(OwnedPathCapacityPanicSite), CanUnwind: true})
}
func ownedPathAllocate(capacity uint, context rustfmt.Context) (result []byte) {
	if capacity > uint(^uint(0)>>1) {
		ownedPathCapacityOverflow(context)
	}
	defer func() {
		if recover() != nil {
			os.Exit(134)
		}
	}()
	return make([]byte, 0, int(capacity))
}
func ownedOsString(data []byte, flavor RustPathFlavor) *RustOsString {
	return &RustOsString{owner: &rustOsStringOwner{cell: &RustOsStringCell{state: &rustOsStringState{bytes: data, flavor: flavor, live: true}}}}
}
func NewRustOsString() *RustOsString {
	return NewRustOsStringWithCapacity(0, RustPathNative, rustfmt.Context{})
}
func NewRustOsStringWithCapacity(capacity uint, flavor RustPathFlavor, context rustfmt.Context) *RustOsString {
	return ownedOsString(ownedPathAllocate(capacity, context), flavor)
}
func RustOsStringFromPath(path RustPath, context rustfmt.Context) *RustOsString {
	result := NewRustOsStringWithCapacity(uint(len(path.raw)), path.flavor, context)
	result.Borrow().state.bytes = append(result.Borrow().state.bytes, path.raw...)
	return result
}

// RustOsStringFromOwnedString consumes the actual String buffer.
func RustOsStringFromOwnedString(value *ruststr.OwnedString, flavor RustPathFlavor, context rustfmt.Context) *RustOsString {
	return ownedOsString(value.TakeUTF8BufferWithContext(context), flavor)
}

// RustOsStringFromUnixBuffer consumes an owning Vec<u8> descriptor. All byte
// values are valid. The caller relinquishes the descriptor and mutable aliases.
func RustOsStringFromUnixBuffer(data []byte) *RustOsString { return ownedOsString(data, RustPathUnix) }

// RustOsStringFromUTF16 follows Wtf8Buf::from_wide: allocate input unit count,
// then append one decoded code point at a time (which determines growth).
func RustOsStringFromUTF16(units []uint16, context rustfmt.Context) *RustOsString {
	result := NewRustOsStringWithCapacity(uint(len(units)), RustPathWindows, context)
	cell := result.Borrow()
	for i := 0; i < len(units); i++ {
		ch := rune(units[i])
		if ch >= 0xd800 && ch <= 0xdbff && i+1 < len(units) && units[i+1] >= 0xdc00 && units[i+1] <= 0xdfff {
			ch = utf16.DecodeRune(ch, rune(units[i+1]))
			i++
		}
		var bytes [4]byte
		n := 0
		if ch >= 0xd800 && ch <= 0xdfff {
			bytes[0] = byte(0xe0 | ch>>12)
			bytes[1] = byte(0x80 | (ch>>6)&0x3f)
			bytes[2] = byte(0x80 | ch&0x3f)
			n = 3
		} else {
			n = utf8.EncodeRune(bytes[:], ch)
		}
		cell.appendEncoded(bytes[:n], context)
	}
	return result
}
func (s *RustOsStringCell) borrow(context rustfmt.Context) *rustOsStringState {
	if s == nil || s.state == nil || !s.state.live {
		context.Panic("rustcspan: consumed OsString cell")
	}
	return s.state
}
func (s *RustOsStringCell) Len() uint      { return uint(len(s.borrow(rustfmt.Context{}).bytes)) }
func (s *RustOsStringCell) Capacity() uint { return uint(cap(s.borrow(rustfmt.Context{}).bytes)) }
func (s *RustOsStringCell) AsPath() RustPath {
	state := s.borrow(rustfmt.Context{})
	return RustPath{raw: unsafe.String(ruststr.OwnedByteBufferIdentity(state.bytes), len(state.bytes)), flavor: state.flavor}
}
func (s *RustOsStringCell) Reserve(additional uint) {
	s.ReserveWithContext(additional, rustfmt.Context{})
}
func (s *RustOsStringCell) ReserveExact(additional uint) {
	s.ReserveExactWithContext(additional, rustfmt.Context{})
}
func (s *RustOsStringCell) ReserveWithContext(additional uint, context rustfmt.Context) {
	s.reserve(additional, false, context)
}
func (s *RustOsStringCell) ReserveExactWithContext(additional uint, context rustfmt.Context) {
	s.reserve(additional, true, context)
}
func (s *RustOsStringCell) reserve(additional uint, exact bool, context rustfmt.Context) {
	data := s.borrow(context).bytes
	if additional <= uint(cap(data)-len(data)) {
		return
	}
	if additional > uint(^uint(0)>>1)-uint(len(data)) {
		ownedPathCapacityOverflow(context)
	}
	required := uint(len(data)) + additional
	capacity := required
	if !exact {
		if doubled := uint(cap(data)) * 2; doubled > capacity {
			capacity = doubled
		}
		if capacity < 8 {
			capacity = 8
		}
	}
	next := ownedPathAllocate(capacity, context)
	s.state.bytes = append(next, data...)
}
func (s *RustOsStringCell) appendEncoded(data []byte, context rustfmt.Context) {
	s.ReserveWithContext(uint(len(data)), context)
	s.state.bytes = append(s.state.bytes, data...)
}
func (s *RustOsStringCell) Clear() {
	state := s.borrow(rustfmt.Context{})
	state.bytes = state.bytes[:0]
}
func (s *RustOsStringCell) truncateEncoded(length uint) {
	state := s.borrow(rustfmt.Context{})
	if length < uint(len(state.bytes)) {
		state.bytes = state.bytes[:length]
	}
}

// Push is OsString::push; it adds no path separator. Windows joins newly
// adjacent surrogate halves before reserving, as Wtf8Buf::push_wtf8 does.
func (s *RustOsStringCell) Push(value RustPath) { s.PushWithContext(value, rustfmt.Context{}) }
func (s *RustOsStringCell) PushWithContext(value RustPath, context rustfmt.Context) {
	path := s.AsPath()
	path.compatible(value)
	if path.Flavor() == RustPathWindows && len(path.raw) >= 3 && len(value.raw) >= 3 {
		lead, nLead := pathSurrogateAt(path.raw[len(path.raw)-3:])
		trail, nTrail := pathSurrogateAt(value.raw[:3])
		if nLead != 0 && nTrail != 0 && lead <= 0xdbff && trail >= 0xdc00 {
			s.truncateEncoded(uint(len(path.raw) - 3))
			s.ReserveWithContext(uint(4+len(value.raw)-3), context)
			var bytes [4]byte
			n := utf8.EncodeRune(bytes[:], utf16.DecodeRune(rune(lead), rune(trail)))
			s.appendEncoded(bytes[:n], context)
			s.appendEncoded(value.EncodedBytes()[3:], context)
			return
		}
	}
	s.appendEncoded(value.EncodedBytes(), context)
}
func (s *RustOsStringCell) ShrinkToWithContext(minimum uint, context rustfmt.Context) {
	data := s.borrow(context).bytes
	if minimum >= uint(cap(data)) {
		return
	}
	if minimum < uint(len(data)) {
		minimum = uint(len(data))
	}
	if minimum == uint(cap(data)) {
		return
	}
	next := ownedPathAllocate(minimum, context)
	s.state.bytes = append(next, data...)
}
func (s *RustOsStringCell) ShrinkToFit() { s.ShrinkToWithContext(s.Len(), rustfmt.Context{}) }
func (s *RustOsStringCell) MakeASCIILowercase() {
	for i, ch := range s.borrow(rustfmt.Context{}).bytes {
		if ch >= 'A' && ch <= 'Z' {
			s.state.bytes[i] = ch + ('a' - 'A')
		}
	}
}
func (s *RustOsStringCell) MakeASCIIUppercase() {
	for i, ch := range s.borrow(rustfmt.Context{}).bytes {
		if ch >= 'a' && ch <= 'z' {
			s.state.bytes[i] = ch - ('a' - 'A')
		}
	}
}
func (s *RustOsStringCell) CloneWithContext(context rustfmt.Context) *RustOsString {
	return RustOsStringFromPath(s.AsPath(), context)
}
func (s *RustOsStringCell) CloneFromWithContext(other *RustOsStringCell, context rustfmt.Context) {
	s.borrow(context)
	other.borrow(context)
	if s.state == other.state {
		context.Panic("rustcspan: aliased mutable OsString clone_from")
	}
	s.AsPath().compatible(other.AsPath())
	if s.AsPath().Flavor() == RustPathWindows {
		// Wtf8Buf derives Clone, but has no clone_from override. Its default
		// Clone::clone_from replaces self with source.clone(), discarding
		// destination capacity. Cow<Path>'s clone_into is a different path.
		next := ownedPathAllocate(uint(len(other.state.bytes)), context)
		s.state.bytes = append(next, other.state.bytes...)
		return
	}
	s.Clear()
	s.appendEncoded(other.state.bytes, context)
}

func (s *RustOsStringCell) cloneIntoFromPath(value RustPath, context rustfmt.Context) {
	s.AsPath().compatible(value)
	// OsStr::clone_into specializes slice::clone_into on BOTH hosts, unlike
	// Windows Wtf8Buf's default Clone::clone_from implementation.
	s.Clear()
	s.appendEncoded(value.EncodedBytes(), context)
}
func (s *RustOsStringCell) MoveWithContext(context rustfmt.Context) *RustOsStringCell {
	state := s.borrow(context)
	next := &RustOsStringCell{state: &rustOsStringState{bytes: state.bytes, flavor: state.flavor, live: true}}
	state.bytes = nil
	state.live = false
	return next
}
func (s *RustOsStringCell) CloseWithContext(context rustfmt.Context) {
	state := s.borrow(context)
	state.bytes = nil
	state.live = false
}
func (s *RustOsStringCell) BufferIdentity() *byte {
	data := s.borrow(rustfmt.Context{}).bytes
	return ruststr.OwnedByteBufferIdentity(data)
}
func (s *RustOsString) Borrow() *RustOsStringCell { return s.BorrowWithContext(rustfmt.Context{}) }
func (s *RustOsString) BorrowWithContext(context rustfmt.Context) *RustOsStringCell {
	if s == nil || s.owner == nil || s.owner.cell == nil {
		context.Panic("rustcspan: consumed OsString owner")
	}
	s.owner.cell.borrow(context)
	return s.owner.cell
}
func (s *RustOsString) TakeCellWithContext(context rustfmt.Context) *RustOsStringCell {
	cell := s.BorrowWithContext(context)
	s.owner.cell = nil
	return cell
}
func (s *RustOsString) MoveWithContext(context rustfmt.Context) *RustOsString {
	return &RustOsString{owner: &rustOsStringOwner{cell: s.TakeCellWithContext(context)}}
}
func (s *RustOsString) CloneWithContext(context rustfmt.Context) *RustOsString {
	return s.BorrowWithContext(context).CloneWithContext(context)
}
func (s *RustOsString) AsPath() RustPath { return s.Borrow().AsPath() }
func (s *RustOsString) CloseWithContext(context rustfmt.Context) {
	s.TakeCellWithContext(context).CloseWithContext(context)
}
func (s *RustOsString) Close() { s.CloseWithContext(rustfmt.Context{}) }
func (s *RustOsString) IntoEncodedBufferWithContext(context rustfmt.Context) []byte {
	cell := s.TakeCellWithContext(context)
	state := cell.borrow(context)
	data := state.bytes
	state.bytes = nil
	state.live = false
	return data
}

// IntoString consumes self on BOTH Result branches. Exactly one result is
// nonnil; invalid Unicode returns the original allocation and capacity.
func (s *RustOsString) IntoStringWithContext(context rustfmt.Context) (*ruststr.OwnedString, *RustOsString) {
	cell := s.BorrowWithContext(context)
	if !utf8.Valid(cell.borrow(context).bytes) {
		return nil, s.MoveWithContext(context)
	}
	return ruststr.OwnedStringFromUTF8BufferWithContext(s.IntoEncodedBufferWithContext(context), context), nil
}
