// SPDX-License-Identifier: MPL-2.0
// Required no-pool/no-specialisation operations from im-rc 15.1.0
// src/nodes/btree.rs and config.rs. Unused map/set/range/removal APIs are not
// represented as a complete dependency-file port.
package imrcbtree

import (
	imrc "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

const NodeSize = 64 // config::OrdChunkSize = U64
const Median = (NodeSize + 1) >> 1

type ValueOps[A any] struct {
	CheckedArithmetic bool
	Value             imrc.Ops[A]
	Compare           func(rustvec.Ref[A], rustvec.Ref[A]) int
	NotEqual          func(rustvec.Ref[A], rustvec.Ref[A]) bool
}
type Node[A any] struct {
	Keys     []*imrc.Cell[A]
	Children []*imrc.Rc[Node[A]]
	Ops      ValueOps[A]
}

func New[A any](ops ValueOps[A]) *imrc.Rc[Node[A]] {
	return Wrap(Node[A]{Children: make([]*imrc.Rc[Node[A]], 1), Ops: ops})
}
func Wrap[A any](value Node[A]) *imrc.Rc[Node[A]] {
	ops := imrc.Ops[Node[A]]{
		Clone: func(ref rustvec.Ref[Node[A]]) *rustvec.Owned[Node[A]] {
			cloned := ref.Value.clone(ref.Context)
			return rustvec.NewOwned(cloned, rustvec.ValueOps[Node[A]]{Drop: func(n *Node[A], ctx rustfmt.Context) { n.Drop(ctx) }})
		},
		Drop: func(n *Node[A], ctx rustfmt.Context) { n.Drop(ctx) },
	}
	return imrc.NewRc(imrc.NewCell(value, ops))
}
func (n *Node[A]) clone(ctx rustfmt.Context) (out Node[A]) {
	out.Ops = n.Ops
	complete := false
	imrc.Scope(ctx, func() {
		for _, key := range n.Keys {
			out.Keys = append(out.Keys, key.Clone(ctx))
		}
		for _, child := range n.Children {
			if child == nil {
				out.Children = append(out.Children, nil)
			} else {
				out.Children = append(out.Children, child.Clone(ctx))
			}
		}
		complete = true
	}, func(ctx rustfmt.Context) {
		if !complete {
			out.Drop(ctx)
		}
	})
	return
}
func (n *Node[A]) Drop(ctx rustfmt.Context) {
	keys, children := n.Keys, n.Children
	n.Keys = nil
	n.Children = nil
	imrc.Scope(ctx, func() { imrc.DropCells(keys, ctx) }, func(ctx rustfmt.Context) {
		imrc.DropSequence(children, ctx, func(child *imrc.Rc[Node[A]], ctx rustfmt.Context) { child.Drop(ctx) })
	})
}

// BinarySearch follows the std slice::binary_search_by used by this crate's
// non-specialised BTreeValue implementation, including the final comparison.
func BinarySearch(length int, compare func(int) int) (int, bool) {
	if length == 0 {
		return 0, false
	}
	base, size := 0, length
	for size > 1 {
		half := size / 2
		mid := base + half
		if compare(mid) <= 0 {
			base = mid
		}
		size -= half
	}
	c := compare(base)
	if c == 0 {
		return base, true
	}
	if c < 0 {
		return base + 1, false
	}
	return base, false
}
func (n *Node[A]) search(value *imrc.Cell[A], ctx rustfmt.Context) (int, bool) {
	return BinarySearch(len(n.Keys), func(i int) int { return n.Ops.Compare(n.Keys[i].Ref(ctx), value.Ref(ctx)) })
}
func (n *Node[A]) Lookup(compare func(*A) int, ctx rustfmt.Context) *imrc.Cell[A] {
	if len(n.Keys) == 0 {
		return nil
	}
	index, found := BinarySearch(len(n.Keys), func(i int) int { return compare(n.Keys[i].Ref(ctx).Value) })
	if found {
		return n.Keys[index]
	}
	if child := n.Children[index]; child != nil {
		return child.Ref(ctx).Value.Lookup(compare, ctx)
	}
	return nil
}
func (n *Node[A]) LookupMut(compare func(*A) int, ctx rustfmt.Context) *imrc.Cell[A] {
	if len(n.Keys) == 0 {
		return nil
	}
	index, found := BinarySearch(len(n.Keys), func(i int) int { return compare(n.Keys[i].Ref(ctx).Value) })
	if found {
		return n.Keys[index]
	}
	if n.Children[index] != nil {
		return imrc.MakeMut(&n.Children[index], ctx).LookupMut(compare, ctx)
	}
	return nil
}

type Insert[A any] struct {
	Kind        uint8 // 0 Added, 1 Replaced, 2 Split
	Old         *imrc.Cell[A]
	Left, Right *imrc.Rc[Node[A]]
	Median      *imrc.Cell[A]
}

func insertAt[T any](slice []T, index int, value T) []T {
	var zero T
	slice = append(slice, zero)
	copy(slice[index+1:], slice[index:])
	slice[index] = value
	return slice
}
func (n *Node[A]) setChild(index int, child *imrc.Rc[Node[A]], ctx rustfmt.Context) {
	old := n.Children[index]
	n.Children[index] = child
	old.Drop(ctx)
}
func (n *Node[A]) split(value *imrc.Cell[A], left, right *imrc.Rc[Node[A]], ctx rustfmt.Context) (result Insert[A]) {
	remaining := value
	localLeft, localRight := left, right
	imrc.Scope(ctx, func() {
		index, found := n.search(value, ctx)
		if found {
			ctx.Panic("im-rc: split search unexpectedly found key")
		}
		n.setChild(index, left, ctx)
		localLeft = nil
		keys := insertAt(n.Keys, index, value)
		remaining = nil
		children := insertAt(n.Children, index+1, right)
		localRight = nil
		result = Insert[A]{Kind: 2, Left: Wrap(Node[A]{Keys: append([]*imrc.Cell[A](nil), keys[:Median]...), Children: append([]*imrc.Rc[Node[A]](nil), children[:Median+1]...), Ops: n.Ops}), Median: keys[Median], Right: Wrap(Node[A]{Keys: append([]*imrc.Cell[A](nil), keys[Median+1:]...), Children: append([]*imrc.Rc[Node[A]](nil), children[Median+1:]...), Ops: n.Ops})}
		n.Keys = nil
		n.Children = nil
	}, func(ctx rustfmt.Context) {
		imrc.Scope(ctx, func() { localRight.Drop(ctx) }, func(ctx rustfmt.Context) {
			imrc.Scope(ctx, func() { localLeft.Drop(ctx) }, func(ctx rustfmt.Context) { remaining.Drop(ctx) })
		})
	})
	return
}
func (n *Node[A]) Insert(value *imrc.Cell[A], ctx rustfmt.Context) (result Insert[A]) {
	remaining := value
	imrc.Scope(ctx, func() {
		if len(n.Keys) == 0 {
			n.Keys = append(n.Keys, value)
			remaining = nil
			n.Children = append(n.Children, nil)
			return
		}
		index, found := n.search(value, ctx)
		if found {
			result.Kind = 1
			result.Old = n.Keys[index]
			n.Keys[index] = value
			remaining = nil
			return
		}
		hasRoom := len(n.Keys) < NodeSize
		if n.Children[index] == nil {
			if hasRoom {
				n.Keys = insertAt(n.Keys, index, value)
				remaining = nil
				n.Children = insertAt(n.Children, index+1, nil)
				return
			}
			remaining = nil
			result = n.split(value, nil, nil, ctx)
			return
		}
		child := imrc.MakeMut(&n.Children[index], ctx)
		inserted := child.Insert(value.Clone(ctx), ctx)
		if inserted.Kind != 2 {
			result = inserted
			return
		}
		if hasRoom {
			n.setChild(index, inserted.Left, ctx)
			n.Keys = insertAt(n.Keys, index, inserted.Median)
			n.Children = insertAt(n.Children, index+1, inserted.Right)
			return
		}
		result = n.split(inserted.Median, inserted.Left, inserted.Right, ctx)
	}, func(ctx rustfmt.Context) { remaining.Drop(ctx) })
	return
}

type pathEntry[A any] struct {
	node  *Node[A]
	index int
}
type Iter[A any] struct {
	front, back []pathEntry[A]
	ops         ValueOps[A]
	remaining   uint
}

func NewIter[A any](root *Node[A], size uint, ctx rustfmt.Context) *Iter[A] {
	it := &Iter[A]{ops: root.Ops, remaining: size}
	if len(root.Keys) == 0 {
		return it
	}
	for n := root; ; n = n.Children[0].Ref(ctx).Value {
		if len(n.Keys) == 0 {
			it.front = nil
			break
		}
		it.front = append(it.front, pathEntry[A]{n, 0})
		if n.Children[0] == nil {
			break
		}
	}
	for n := root; ; {
		if len(n.Keys) == 0 {
			it.back = nil
			break
		}
		end := len(n.Children) - 1
		if n.Children[end] == nil {
			if end == 0 && n.Ops.CheckedArithmetic {
				ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "attempt to subtract with overflow", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation("im-rc.btree.path_last.sub"), CanUnwind: true})
			}
			it.back = append(it.back, pathEntry[A]{n, end - 1})
			break
		}
		it.back = append(it.back, pathEntry[A]{n, end})
		n = n.Children[end].Ref(ctx).Value
	}
	return it
}
func pathGet[A any](path []pathEntry[A]) *imrc.Cell[A] {
	if len(path) == 0 {
		return nil
	}
	entry := path[len(path)-1]
	return entry.node.Keys[entry.index]
}
func stepForward[A any](path *[]pathEntry[A], ctx rustfmt.Context) {
	if len(*path) == 0 {
		return
	}
	entry := (*path)[len(*path)-1]
	*path = (*path)[:len(*path)-1]
	index := entry.index + 1
	if child := entry.node.Children[index]; child != nil {
		*path = append(*path, pathEntry[A]{entry.node, index})
		n := child.Ref(ctx).Value
		for {
			*path = append(*path, pathEntry[A]{n, 0})
			if n.Children[0] == nil {
				return
			}
			n = n.Children[0].Ref(ctx).Value
		}
	}
	if index < len(entry.node.Keys) {
		*path = append(*path, pathEntry[A]{entry.node, index})
		return
	}
	for len(*path) != 0 {
		last := (*path)[len(*path)-1]
		*path = (*path)[:len(*path)-1]
		if last.index < len(last.node.Keys) {
			*path = append(*path, last)
			return
		}
	}
}
func (it *Iter[A]) Next(ctx rustfmt.Context) *imrc.Cell[A] {
	value, last := pathGet(it.front), pathGet(it.back)
	if value == nil || last == nil || it.ops.Compare(value.Ref(ctx), last.Ref(ctx)) > 0 {
		return nil
	}
	stepForward(&it.front, ctx)
	if it.remaining == 0 && it.ops.CheckedArithmetic {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "attempt to subtract with overflow", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation("im-rc.btree.iter.remaining_sub"), CanUnwind: true})
	}
	it.remaining--
	return value
}
func (it *Iter[A]) SizeHint() (uint, *uint) { return 0, nil }

// Remaining is the internal counter read by the outer OrdMap::Iter wrapper.
func (it *Iter[A]) Remaining() uint { return it.remaining }

type diffItem[A any] struct {
	node  *Node[A]
	value *imrc.Cell[A]
}
type Diff[A any] struct {
	old, new []diffItem[A]
	ops      ValueOps[A]
}

func NewDiff[A any](old, new *Node[A]) *Diff[A] {
	d := &Diff[A]{ops: old.Ops}
	if len(old.Keys) != 0 {
		d.old = append(d.old, diffItem[A]{node: old})
	}
	if len(new.Keys) != 0 {
		d.new = append(d.new, diffItem[A]{node: new})
	}
	return d
}
func diffPop[A any](stack *[]diffItem[A]) (diffItem[A], bool) {
	if len(*stack) == 0 {
		return diffItem[A]{}, false
	}
	value := (*stack)[len(*stack)-1]
	*stack = (*stack)[:len(*stack)-1]
	return value, true
}
func diffPush[A any](stack *[]diffItem[A], n *Node[A], ctx rustfmt.Context) {
	for i := len(n.Keys); i > 0; i-- {
		if child := n.Children[i]; child != nil {
			*stack = append(*stack, diffItem[A]{node: child.Ref(ctx).Value})
		}
		*stack = append(*stack, diffItem[A]{value: n.Keys[i-1]})
	}
	if child := n.Children[0]; child != nil {
		*stack = append(*stack, diffItem[A]{node: child.Ref(ctx).Value})
	}
}

// Different returns whether the first diff item exists. It preserves all lazy
// Compare/Ne calls and identical-subtree skips before that first difference.
func (d *Diff[A]) Different(ctx rustfmt.Context) bool {
	for {
		old, oldOK := diffPop(&d.old)
		new, newOK := diffPop(&d.new)
		if !oldOK && !newOK {
			return false
		}
		if !oldOK {
			if new.node != nil {
				diffPush(&d.new, new.node, ctx)
				continue
			}
			return true
		}
		if !newOK {
			if old.node != nil {
				diffPush(&d.old, old.node, ctx)
				continue
			}
			return true
		}
		if old.node != nil && new.node != nil {
			if old.node == new.node {
				continue
			}
			c := d.ops.Compare(old.node.Keys[0].Ref(ctx), new.node.Keys[0].Ref(ctx))
			if c <= 0 {
				diffPush(&d.old, old.node, ctx)
			} else {
				d.old = append(d.old, old)
			}
			if c >= 0 {
				diffPush(&d.new, new.node, ctx)
			} else {
				d.new = append(d.new, new)
			}
			continue
		}
		if old.node != nil {
			diffPush(&d.old, old.node, ctx)
			d.new = append(d.new, new)
			continue
		}
		if new.node != nil {
			d.old = append(d.old, old)
			diffPush(&d.new, new.node, ctx)
			continue
		}
		c := d.ops.Compare(old.value.Ref(ctx), new.value.Ref(ctx))
		if c == 0 {
			if d.ops.NotEqual(old.value.Ref(ctx), new.value.Ref(ctx)) {
				return true
			}
			continue
		}
		if c < 0 {
			d.new = append(d.new, new)
		} else {
			d.old = append(d.old, old)
		}
		return true
	}
}
