// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual RefCell operations consumed by HygieneEncodeContext; no std-file credit.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// HygieneEncodeCell is the source RefCell<HygieneEncodeContext>. Ref.Get is a
// shared borrow, Mut.Get is exclusive; Go callers must respect those source
// borrow/lifetime contracts and may not retain either pointer after Close.
type HygieneEncodeCell struct {
	value  HygieneEncodeContext
	borrow int64
	closed bool
}

func NewHygieneEncodeCell() *HygieneEncodeCell {
	return &HygieneEncodeCell{value: NewHygieneEncodeContext()}
}
func HygieneEncodeCellFrom(value HygieneEncodeContext) *HygieneEncodeCell {
	return &HygieneEncodeCell{value: value}
}

type HygieneEncodeRef struct {
	cell            *HygieneEncodeCell
	mutable, closed bool
}

func (c *HygieneEncodeCell) Borrow() *HygieneEncodeRef { return c.BorrowWithContext(rustfmt.Context{}) }
func (c *HygieneEncodeCell) BorrowWithContext(ctx rustfmt.Context) *HygieneEncodeRef {
	return c.borrowWithSite(ctx, "rustc_span::hygiene::RefCell.borrow.caller")
}
func (c *HygieneEncodeCell) borrowWithSite(ctx rustfmt.Context, site string) *HygieneEncodeRef {
	if c == nil || c.closed {
		ctx.Panic("hygiene: consumed RefCell")
	}
	if c.borrow < 0 || c.borrow == int64(^uint64(0)>>1) {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "RefCell already mutably borrowed", Kind: rustfmt.PanicOwnedString, CanUnwind: true, Location: ctx.SourcePanicLocation(site)})
	}
	c.borrow++
	return &HygieneEncodeRef{cell: c}
}
func (c *HygieneEncodeCell) BorrowMut() *HygieneEncodeRef {
	return c.BorrowMutWithContext(rustfmt.Context{})
}
func (c *HygieneEncodeCell) BorrowMutWithContext(ctx rustfmt.Context) *HygieneEncodeRef {
	return c.borrowMutWithSite(ctx, "rustc_span::hygiene::RefCell.borrow_mut.caller")
}
func (c *HygieneEncodeCell) borrowMutWithSite(ctx rustfmt.Context, site string) *HygieneEncodeRef {
	if c == nil || c.closed {
		ctx.Panic("hygiene: consumed RefCell")
	}
	if c.borrow != 0 {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "RefCell already borrowed", Kind: rustfmt.PanicOwnedString, CanUnwind: true, Location: ctx.SourcePanicLocation(site)})
	}
	c.borrow = -1
	return &HygieneEncodeRef{cell: c, mutable: true}
}
func (r *HygieneEncodeRef) Get() *HygieneEncodeContext {
	if r == nil || r.closed {
		panic("hygiene: use of a consumed RefCell guard")
	}
	return &r.cell.value
}
func (r *HygieneEncodeRef) Close() {
	if r == nil || r.closed {
		return
	}
	r.closed = true
	if r.mutable {
		r.cell.borrow++
	} else {
		r.cell.borrow--
	}
}
func (c *HygieneEncodeCell) Close() { c.CloseWithContext(rustfmt.Context{}) }
func (c *HygieneEncodeCell) CloseWithContext(ctx rustfmt.Context) {
	if c == nil || c.closed {
		return
	}
	c.closed = true
	c.value.CloseWithContext(ctx)
}
