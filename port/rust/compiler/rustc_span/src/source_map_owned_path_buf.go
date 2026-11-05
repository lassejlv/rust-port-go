// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped SDK1.101 PathBuf mutation; see tools/port-rust-owned-path.
package rustcspan

import (
	"strings"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
)

type RustPathBufCell struct{ os *RustOsStringCell }
type rustPathBufOwner struct{ cell *RustPathBufCell }
type RustPathBuf struct{ owner *rustPathBufOwner }

func NewRustPathBuf() *RustPathBuf {
	return NewRustPathBufWithCapacity(0, RustPathNative, rustfmt.Context{})
}
func NewRustPathBufWithCapacity(capacity uint, flavor RustPathFlavor, context rustfmt.Context) *RustPathBuf {
	return RustPathBufFromOsString(NewRustOsStringWithCapacity(capacity, flavor, context), context)
}
func RustPathBufFromPath(path RustPath, context rustfmt.Context) *RustPathBuf {
	return RustPathBufFromOsString(RustOsStringFromPath(path, context), context)
}
func RustPathBufFromOsString(value *RustOsString, context rustfmt.Context) *RustPathBuf {
	return &RustPathBuf{owner: &rustPathBufOwner{cell: &RustPathBufCell{os: value.TakeCellWithContext(context)}}}
}
func RustPathBufFromOwnedString(value *ruststr.OwnedString, flavor RustPathFlavor, context rustfmt.Context) *RustPathBuf {
	return RustPathBufFromOsString(RustOsStringFromOwnedString(value, flavor, context), context)
}
func (p *RustPathBufCell) borrow(context rustfmt.Context) *RustOsStringCell {
	if p == nil || p.os == nil {
		context.Panic("rustcspan: consumed PathBuf cell")
	}
	p.os.borrow(context)
	return p.os
}
func (p *RustPathBufCell) AsPath() RustPath                 { return p.borrow(rustfmt.Context{}).AsPath() }
func (p *RustPathBufCell) AsMutOsString() *RustOsStringCell { return p.borrow(rustfmt.Context{}) }
func (p *RustPathBufCell) Len() uint                        { return p.borrow(rustfmt.Context{}).Len() }
func (p *RustPathBufCell) Capacity() uint                   { return p.borrow(rustfmt.Context{}).Capacity() }
func (p *RustPathBufCell) BufferIdentity() *byte            { return p.borrow(rustfmt.Context{}).BufferIdentity() }
func (p *RustPathBufCell) Reserve(additional uint)          { p.borrow(rustfmt.Context{}).Reserve(additional) }
func (p *RustPathBufCell) ReserveExact(additional uint) {
	p.borrow(rustfmt.Context{}).ReserveExact(additional)
}
func (p *RustPathBufCell) Clear()              { p.borrow(rustfmt.Context{}).Clear() }
func (p *RustPathBufCell) Push(value RustPath) { p.PushWithContext(value, rustfmt.Context{}) }
func (p *RustPathBufCell) PushWithContext(value RustPath, context rustfmt.Context) {
	os := p.borrow(context)
	path := os.AsPath()
	path.compatible(value)
	needSep := len(path.raw) > 0 && !path.sep(path.raw[len(path.raw)-1])
	comps := path.Components()
	if comps.hasPrefix && comps.prefixLen() == len(path.raw) && comps.prefix.isDrive() {
		needSep = false
	}
	if _, hasPrefix := value.parsePrefix(); value.IsAbsolute() || hasPrefix {
		os.Clear()
	} else if comps.hasPrefix && comps.prefix.verbatim() && value.raw != "" {
		// This source branch creates a NEW empty OsString, then pushes each
		// component/separator separately. It discards the old spare capacity.
		var all []RustPathComponent
		for {
			c, ok := comps.Next()
			if !ok {
				break
			}
			all = append(all, c)
		}
		more := value.Components()
		for {
			c, ok := more.Next()
			if !ok {
				break
			}
			switch c.Kind {
			case RustPathRootDir:
				all = append(all[:1], c)
			case RustPathCurDir:
			case RustPathParentDir:
				if len(all) > 0 && all[len(all)-1].Kind == RustPathNormal {
					all = all[:len(all)-1]
				}
			default:
				all = append(all, c)
			}
		}
		result := NewRustOsStringWithCapacity(0, path.flavor, context)
		cell := result.Borrow()
		needSep = false
		for _, c := range all {
			if needSep && c.Kind != RustPathRootDir {
				cell.PushWithContext(path.view(path.separator()), context)
			}
			cell.PushWithContext(c.Path, context)
			switch c.Kind {
			case RustPathRootDir:
				needSep = false
			case RustPathPrefix:
				needSep = !c.prefix.isDrive() && c.prefix.length() > 0
			default:
				needSep = true
			}
		}
		os.CloseWithContext(context)
		p.os = result.TakeCellWithContext(context)
		return
	} else if value.HasRoot() {
		os.truncateEncoded(uint(comps.prefixRemaining()))
	} else if needSep {
		os.PushWithContext(path.view(path.separator()), context)
	}
	os.PushWithContext(value, context)
}
func (p *RustPathBufCell) Pop() bool {
	os := p.borrow(rustfmt.Context{})
	comps := os.AsPath().Components()
	c, ok := comps.NextBack()
	if !ok || c.Kind == RustPathPrefix || c.Kind == RustPathRootDir {
		return false
	}
	os.truncateEncoded(uint(len(comps.AsPath().raw)))
	return true
}
func (p *RustPathBufCell) SetFileName(value RustPath) {
	p.SetFileNameWithContext(value, rustfmt.Context{})
}
func (p *RustPathBufCell) SetFileNameWithContext(value RustPath, context rustfmt.Context) {
	path := p.AsPath()
	path.compatible(value)
	if path.FileName().IsSome() {
		p.Pop()
	}
	p.PushWithContext(value, context)
}
func (p *RustPathBufCell) validateExtension(extension RustPath, context rustfmt.Context) {
	path := p.AsPath()
	path.compatible(extension)
	for i := 0; i < len(extension.raw); i++ {
		if path.sep(extension.raw[i]) {
			context.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: context.SourcePanicLocation(OwnedPathExtensionPanicSite), CanUnwind: true}, func(processing rustfmt.Context) any {
				var result strings.Builder
				formatter := &rustfmt.Formatter{Writer: &result, Context: processing, Options: rustfmt.DefaultOptions()}
				_ = formatter.WriteString("extension cannot contain path separators: ")
				_ = extension.WriteDebugFormatter(formatter)
				return result.String()
			})
		}
	}
}

// encodedViewEnd computes the offset of a borrowed subslice without searching
// for equal text elsewhere in the path. Even repeated names preserve identity.
func encodedViewEnd(whole, part string) uint {
	return uint(uintptr(unsafe.Pointer(unsafe.StringData(part))) + uintptr(len(part)) - uintptr(unsafe.Pointer(unsafe.StringData(whole))))
}
func (p *RustPathBufCell) SetExtension(value RustPath) bool {
	return p.SetExtensionWithContext(value, rustfmt.Context{})
}
func (p *RustPathBufCell) SetExtensionWithContext(value RustPath, context rustfmt.Context) bool {
	p.validateExtension(value, context)
	path := p.AsPath()
	file := path.FileName()
	if !file.IsSome() {
		return false
	}
	stem := file.value.raw
	if dot := strings.LastIndexByte(stem, '.'); dot > 0 {
		stem = stem[:dot]
	}
	os := p.borrow(context)
	os.truncateEncoded(encodedViewEnd(path.raw, stem))
	if len(value.raw) > 0 {
		os.ReserveExactWithContext(uint(len(value.raw)+1), context)
		os.PushWithContext(path.view("."), context)
		os.appendEncoded(value.EncodedBytes(), context)
	}
	return true
}
func (p *RustPathBufCell) AddExtension(value RustPath) bool {
	return p.AddExtensionWithContext(value, rustfmt.Context{})
}
func (p *RustPathBufCell) AddExtensionWithContext(value RustPath, context rustfmt.Context) bool {
	p.validateExtension(value, context)
	path := p.AsPath()
	file := path.FileName()
	if !file.IsSome() {
		return false
	}
	if len(value.raw) > 0 {
		os := p.borrow(context)
		os.truncateEncoded(encodedViewEnd(path.raw, file.value.raw))
		os.ReserveExactWithContext(uint(len(value.raw)+1), context)
		os.PushWithContext(path.view("."), context)
		os.appendEncoded(value.EncodedBytes(), context)
	}
	return true
}
func (p *RustPathBufCell) CloneWithContext(context rustfmt.Context) *RustPathBuf {
	return RustPathBufFromOsString(p.borrow(context).CloneWithContext(context), context)
}
func (p *RustPathBufCell) CloneFromWithContext(other *RustPathBufCell, context rustfmt.Context) {
	p.borrow(context).CloneFromWithContext(other.borrow(context), context)
}
func (p *RustPathBufCell) MoveWithContext(context rustfmt.Context) *RustPathBufCell {
	os := p.borrow(context)
	p.os = nil
	return &RustPathBufCell{os: os.MoveWithContext(context)}
}
func (p *RustPathBufCell) CloseWithContext(context rustfmt.Context) {
	p.borrow(context).CloseWithContext(context)
	p.os = nil
}
func (p *RustPathBuf) Borrow() *RustPathBufCell { return p.BorrowWithContext(rustfmt.Context{}) }
func (p *RustPathBuf) BorrowWithContext(context rustfmt.Context) *RustPathBufCell {
	if p == nil || p.owner == nil || p.owner.cell == nil {
		context.Panic("rustcspan: consumed PathBuf owner")
	}
	p.owner.cell.borrow(context)
	return p.owner.cell
}
func (p *RustPathBuf) AsPath() RustPath { return p.Borrow().AsPath() }
func (p *RustPathBuf) TakeCellWithContext(context rustfmt.Context) *RustPathBufCell {
	cell := p.BorrowWithContext(context)
	p.owner.cell = nil
	return cell
}
func (p *RustPathBuf) MoveWithContext(context rustfmt.Context) *RustPathBuf {
	return &RustPathBuf{owner: &rustPathBufOwner{cell: p.TakeCellWithContext(context)}}
}
func (p *RustPathBuf) CloneWithContext(context rustfmt.Context) *RustPathBuf {
	return p.BorrowWithContext(context).CloneWithContext(context)
}
func (p *RustPathBuf) IntoOsStringWithContext(context rustfmt.Context) *RustOsString {
	cell := p.TakeCellWithContext(context)
	os := cell.borrow(context)
	cell.os = nil
	return &RustOsString{owner: &rustOsStringOwner{cell: os}}
}
func (p *RustPathBuf) CloseWithContext(context rustfmt.Context) {
	p.TakeCellWithContext(context).CloseWithContext(context)
}
func (p *RustPathBuf) Close() { p.CloseWithContext(rustfmt.Context{}) }

// ToOwnedPathBuf and JoinOwned implement Path's owned results. Join clones
// only the source length, then performs the real sequence of push operations.
func (p RustPath) ToOwnedPathBuf(context rustfmt.Context) *RustPathBuf {
	return RustPathBufFromPath(p, context)
}
func (p RustPath) JoinOwned(other RustPath, context rustfmt.Context) *RustPathBuf {
	result := p.ToOwnedPathBuf(context)
	result.Borrow().PushWithContext(other, context)
	return result
}
