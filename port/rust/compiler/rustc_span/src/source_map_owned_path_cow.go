// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped SDK1.101 Cow<Path>/Cow<str>; see tools/port-rust-owned-path.
package rustcspan

import (
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
)

type rustCowPathState struct {
	borrowed RustPath
	owned    *RustPathBuf
	live     bool
}
type RustCowPath struct{ state *rustCowPathState }

func BorrowedRustCowPath(path RustPath) *RustCowPath {
	return &RustCowPath{state: &rustCowPathState{borrowed: path, live: true}}
}
func OwnedRustCowPath(path *RustPathBuf, context rustfmt.Context) *RustCowPath {
	return &RustCowPath{state: &rustCowPathState{owned: path.MoveWithContext(context), live: true}}
}
func (c *RustCowPath) borrow(context rustfmt.Context) *rustCowPathState {
	if c == nil || c.state == nil || !c.state.live {
		context.Panic("rustcspan: consumed Cow<Path>")
	}
	return c.state
}
func (c *RustCowPath) IsOwned() bool    { return c.borrow(rustfmt.Context{}).owned != nil }
func (c *RustCowPath) IsBorrowed() bool { return !c.IsOwned() }
func (c *RustCowPath) AsPath() RustPath {
	state := c.borrow(rustfmt.Context{})
	if state.owned != nil {
		return state.owned.AsPath()
	}
	return state.borrowed
}
func (c *RustCowPath) ToMutWithContext(context rustfmt.Context) *RustPathBufCell {
	state := c.borrow(context)
	if state.owned == nil {
		state.owned = state.borrowed.ToOwnedPathBuf(context)
		state.borrowed = RustPath{}
	}
	return state.owned.BorrowWithContext(context)
}
func (c *RustCowPath) IntoOwnedWithContext(context rustfmt.Context) *RustPathBuf {
	state := c.borrow(context)
	var result *RustPathBuf
	if state.owned == nil {
		result = state.borrowed.ToOwnedPathBuf(context)
	} else {
		result = state.owned.MoveWithContext(context)
	}
	state.owned = nil
	state.borrowed = RustPath{}
	state.live = false
	return result
}
func (c *RustCowPath) MoveWithContext(context rustfmt.Context) *RustCowPath {
	state := c.borrow(context)
	next := *state
	state.owned = nil
	state.borrowed = RustPath{}
	state.live = false
	return &RustCowPath{state: &next}
}
func (c *RustCowPath) CloneWithContext(context rustfmt.Context) *RustCowPath {
	state := c.borrow(context)
	if state.owned == nil {
		return BorrowedRustCowPath(state.borrowed)
	}
	return OwnedRustCowPath(state.owned.AsPath().ToOwnedPathBuf(context), context)
}
func (c *RustCowPath) CloneFromWithContext(source *RustCowPath, context rustfmt.Context) {
	dest, other := c.borrow(context), source.borrow(context)
	if dest == other {
		context.Panic("rustcspan: aliased mutable Cow<Path> clone_from")
	}
	if dest.owned != nil && other.owned != nil {
		dest.owned.Borrow().AsMutOsString().cloneIntoFromPath(other.owned.AsPath(), context)
		return
	}
	next := source.CloneWithContext(context)
	if dest.owned != nil {
		dest.owned.CloseWithContext(context)
	}
	*dest = *next.state
	next.state.live = false
	next.state.owned = nil
}
func (c *RustCowPath) CloseWithContext(context rustfmt.Context) {
	state := c.borrow(context)
	if state.owned != nil {
		state.owned.CloseWithContext(context)
	}
	state.owned = nil
	state.borrowed = RustPath{}
	state.live = false
}
func (c *RustCowPath) Close() { c.CloseWithContext(rustfmt.Context{}) }

type rustCowStringState struct {
	borrowed string
	owned    *ruststr.OwnedString
	live     bool
}

// RustCowString owns its actual String on the Owned branch. CowString remains
// the legacy immutable convenience view and cannot substitute for this owner.
type RustCowString struct{ state *rustCowStringState }

func BorrowedRustCowString(value string) *RustCowString {
	if !utf8.ValidString(value) {
		panic("rustcspan: Cow<str> input requires valid UTF-8")
	}
	return &RustCowString{state: &rustCowStringState{borrowed: value, live: true}}
}
func OwnedRustCowString(value *ruststr.OwnedString, context rustfmt.Context) *RustCowString {
	return &RustCowString{state: &rustCowStringState{owned: value.MoveWithContext(context), live: true}}
}
func (c *RustCowString) borrow(context rustfmt.Context) *rustCowStringState {
	if c == nil || c.state == nil || !c.state.live {
		context.Panic("rustcspan: consumed Cow<str>")
	}
	return c.state
}
func (c *RustCowString) IsOwned() bool    { return c.borrow(rustfmt.Context{}).owned != nil }
func (c *RustCowString) IsBorrowed() bool { return !c.IsOwned() }
func (c *RustCowString) String() string {
	state := c.borrow(rustfmt.Context{})
	if state.owned != nil {
		return state.owned.Borrow().AsStr()
	}
	return state.borrowed
}
func (c *RustCowString) ToMutWithContext(context rustfmt.Context) *ruststr.StringCell {
	state := c.borrow(context)
	if state.owned == nil {
		state.owned = ruststr.OwnedStringFromGo(state.borrowed, context)
		state.borrowed = ""
	}
	return state.owned.BorrowWithContext(context)
}
func (c *RustCowString) IntoOwnedWithContext(context rustfmt.Context) *ruststr.OwnedString {
	state := c.borrow(context)
	var result *ruststr.OwnedString
	if state.owned == nil {
		result = ruststr.OwnedStringFromGo(state.borrowed, context)
	} else {
		result = state.owned.MoveWithContext(context)
	}
	state.owned = nil
	state.borrowed = ""
	state.live = false
	return result
}
func (c *RustCowString) MoveWithContext(context rustfmt.Context) *RustCowString {
	state := c.borrow(context)
	next := *state
	state.owned = nil
	state.borrowed = ""
	state.live = false
	return &RustCowString{state: &next}
}
func (c *RustCowString) CloneWithContext(context rustfmt.Context) *RustCowString {
	state := c.borrow(context)
	if state.owned == nil {
		return BorrowedRustCowString(state.borrowed)
	}
	return OwnedRustCowString(ruststr.OwnedStringFromGo(state.owned.Borrow().AsStr(), context), context)
}
func (c *RustCowString) CloneFromWithContext(source *RustCowString, context rustfmt.Context) {
	dest, other := c.borrow(context), source.borrow(context)
	if dest == other {
		context.Panic("rustcspan: aliased mutable Cow<str> clone_from")
	}
	if dest.owned != nil && other.owned != nil {
		dest.owned.Borrow().CloneFromWithContext(other.owned.Borrow(), context)
		return
	}
	next := source.CloneWithContext(context)
	if dest.owned != nil {
		dest.owned.CloseWithContext(context)
	}
	*dest = *next.state
	next.state.live = false
	next.state.owned = nil
}
func (c *RustCowString) CloseWithContext(context rustfmt.Context) {
	state := c.borrow(context)
	if state.owned != nil {
		state.owned.CloseWithContext(context)
	}
	state.owned = nil
	state.borrowed = ""
	state.live = false
}
func (c *RustCowString) Close() { c.CloseWithContext(rustfmt.Context{}) }

// ToStringLossyOwned returns the true borrowed/owned Cow alternative. Unix
// follows UTF8Chunks and String's source push order, preserving its capacity.
// Windows starts with capacity equal to encoded length; each surrogate becomes
// one three-byte replacement, so its exact capacity is retained.
func (p RustPath) ToStringLossyOwned(context rustfmt.Context) *RustCowString {
	if len(p.raw) == 0 && p.Flavor() == RustPathUnix {
		// Unix's UTF8Chunks returns no first chunk, so it returns the static
		// empty str rather than preserving an empty allocated slice's pointer.
		return BorrowedRustCowString("")
	}
	if utf8.ValidString(p.raw) {
		return BorrowedRustCowString(p.raw)
	}
	result := ruststr.NewOwnedStringWithCapacity(uint(len(p.raw)), context)
	cell := result.Borrow()
	for i := 0; i < len(p.raw); {
		start := i
		for i < len(p.raw) {
			ch, n := utf8.DecodeRuneInString(p.raw[i:])
			if ch == utf8.RuneError && n == 1 {
				break
			}
			i += n
		}
		cell.PushStringWithContext(p.raw[start:i], context)
		if i == len(p.raw) {
			break
		}
		n := rustInvalidUTF8Length(p.raw[i:])
		if p.Flavor() == RustPathWindows {
			if _, size := pathSurrogateAt(p.raw[i:]); size != 0 {
				n = size
			}
		}
		cell.PushStringWithContext("\ufffd", context)
		i += n
	}
	return OwnedRustCowString(result, context)
}
