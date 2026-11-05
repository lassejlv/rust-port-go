// SPDX-License-Identifier: MIT OR Apache-2.0
package util

import (
	"errors"
	"io"
	"strings"
	"syscall"
	"unsafe"

	imrc "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src"
	btree "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src/nodes"
	ord "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src/ord"
	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// GraphOwnedNodes is the affine Vec<N> return witness of Graph::sort. Each N
// is independently owned; Close performs its exact element Drop sequence.
// Ref borrows the actual element cell. Take transfers one element explicitly.
type GraphOwnedNodes[N any] struct {
	values []*rustvec.Owned[N]
	live   bool
}

func (v *GraphOwnedNodes[N]) Len() int {
	if v == nil || !v.live {
		panic("cargo graph: consumed sort vector")
	}
	return len(v.values)
}
func (v *GraphOwnedNodes[N]) Ref(index int) *N {
	if v == nil || !v.live {
		panic("cargo graph: consumed sort vector")
	}
	return v.values[index].Borrow()
}
func (v *GraphOwnedNodes[N]) Take(index int, ctx rustfmt.Context) *rustvec.Owned[N] {
	if v == nil || !v.live {
		ctx.Panic("cargo graph: consumed sort vector")
	}
	value := v.values[index].MoveWithContext(ctx)
	v.values[index] = nil
	return value
}
func (v *GraphOwnedNodes[N]) Close() { v.CloseWithContext(rustfmt.Context{}) }
func (v *GraphOwnedNodes[N]) CloseWithContext(ctx rustfmt.Context) {
	if v == nil || !v.live {
		return
	}
	v.live = false
	values := v.values
	v.values = nil
	imrc.DropSequence(values, ctx, func(value *rustvec.Owned[N], ctx rustfmt.Context) { value.DropWithContext(ctx) })
}
func (g *OwnedGraph[N, E]) Sort() *GraphOwnedNodes[N] { return g.SortWithContext(rustfmt.Context{}) }
func (g *OwnedGraph[N, E]) SortWithContext(ctx rustfmt.Context) (result *GraphOwnedNodes[N]) {
	g.check(ctx)
	ret := &GraphOwnedNodes[N]{live: true}
	remaining := ret
	marks := graphBTree[*rustvec.Owned[N], struct{}]{ops: graphBTreeOps[*rustvec.Owned[N], struct{}]{
		compare: func(a, b *rustvec.Owned[N], ctx rustfmt.Context) int {
			return g.traits.CompareNode(rustvec.Ref[N]{Value: a.Borrow(), Context: ctx}, rustvec.Ref[N]{Value: b.Borrow(), Context: ctx})
		},
		moveKey: func(a *rustvec.Owned[N], ctx rustfmt.Context) *rustvec.Owned[N] { return a.MoveWithContext(ctx) }, dropKey: func(a *rustvec.Owned[N], ctx rustfmt.Context) { a.DropWithContext(ctx) },
	}}
	imrc.Scope(ctx, func() {
		imrc.Scope(ctx, func() {
			it := g.IterWithContext(ctx)
			for {
				node, ok := it.NextWithContext(ctx)
				if !ok {
					break
				}
				g.sortOwnedVisit(node, ret, &marks, ctx)
			}
			// Prepare the return place before marks' normal destructor. A panic
			// there leaks this prepared Vec, matching Rust's return cleanup.
			result = ret
			remaining = nil
		}, func(ctx rustfmt.Context) { marks.close(ctx) })
	}, func(ctx rustfmt.Context) { remaining.CloseWithContext(ctx) })
	return
}
func (g *OwnedGraph[N, E]) sortOwnedVisit(node *N, ret *GraphOwnedNodes[N], marks *graphBTree[*rustvec.Owned[N], struct{}], ctx rustfmt.Context) {
	if !marks.insertSet(g.traits.Node.Clone(rustvec.Ref[N]{Value: node, Context: ctx}), ctx) {
		return
	}
	adjacent := g.nodes.IndexAt(rustvec.Ref[N]{Value: node, Context: ctx}, ctx.SourcePanicLocation("cargo.graph.sort.index"), ctx)
	it := adjacent.Ref(ctx).Value.Iter(ctx)
	for {
		child, _, ok := it.Next(ctx)
		if !ok {
			break
		}
		g.sortOwnedVisit(child.Ref(ctx).Value, ret, marks, ctx)
	}
	ret.values = append(ret.values, g.traits.Node.Clone(rustvec.Ref[N]{Value: node, Context: ctx}))
}
func (g *OwnedGraph[N, E]) borrowedTree() graphBTree[*N, struct{}] {
	return graphBTree[*N, struct{}]{ops: graphBTreeOps[*N, struct{}]{compare: func(a, b *N, ctx rustfmt.Context) int {
		return g.traits.CompareNode(rustvec.Ref[N]{Value: a, Context: ctx}, rustvec.Ref[N]{Value: b, Context: ctx})
	}}}
}
func (g *OwnedGraph[N, E]) IsPathFromTo(from, to *N) bool {
	return g.IsPathFromToWithContext(from, to, rustfmt.Context{})
}
func (g *OwnedGraph[N, E]) IsPathFromToWithContext(from, to *N, ctx rustfmt.Context) bool {
	g.check(ctx)
	stack := []*N{from}
	seen := g.borrowedTree()
	seen.insertSet(from, ctx)
	for len(stack) != 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		adjacent := g.nodes.Get(rustvec.Ref[N]{Value: node, Context: ctx}, ctx)
		if adjacent == nil {
			break
		}
		it := adjacent.Ref(ctx).Value.Iter(ctx)
		for {
			child, _, ok := it.Next(ctx)
			if !ok {
				break
			}
			p := child.Ref(ctx).Value
			if g.traits.EqualNode(rustvec.Ref[N]{Value: p, Context: ctx}, rustvec.Ref[N]{Value: to, Context: ctx}) {
				return true
			}
			if seen.insertSet(p, ctx) {
				stack = append(stack, p)
			}
		}
	}
	return false
}

// GraphBorrowedPathEntry retains the original graph/query reference, rather
// than copying N. Edge nil represents None; all remaining fields borrow E.
type GraphBorrowedPathEntry[N, E any] struct {
	Node *N
	Edge *E
}
type graphOwnedEdgeIterator[N, E any] interface {
	NextWithContext(rustfmt.Context) (GraphEdgeBorrow[N, E], bool)
}
type graphParentEdges[N, E any] struct {
	iter  *ord.Iter[N, ord.Map[N, E]]
	query *N
}

func (it *graphParentEdges[N, E]) NextWithContext(ctx rustfmt.Context) (GraphEdgeBorrow[N, E], bool) {
	for {
		node, adjacent, ok := it.iter.Next(ctx)
		if !ok {
			return GraphEdgeBorrow[N, E]{}, false
		}
		edge := adjacent.Ref(ctx).Value.Get(rustvec.Ref[N]{Value: it.query, Context: ctx}, ctx)
		if edge != nil {
			return GraphEdgeBorrow[N, E]{Node: node.Ref(ctx).Value, Edge: edge.Ref(ctx).Value}, true
		}
	}
}
func (g *OwnedGraph[N, E]) PathToBottom(node *N) []GraphBorrowedPathEntry[N, E] {
	return g.PathToBottomWithContext(node, rustfmt.Context{})
}
func (g *OwnedGraph[N, E]) PathToBottomWithContext(node *N, ctx rustfmt.Context) []GraphBorrowedPathEntry[N, E] {
	return g.ownedPathTo(node, func(n *N) graphOwnedEdgeIterator[N, E] { return g.EdgesWithContext(n, ctx) }, ctx)
}
func (g *OwnedGraph[N, E]) PathToTop(node *N) []GraphBorrowedPathEntry[N, E] {
	return g.PathToTopWithContext(node, rustfmt.Context{})
}
func (g *OwnedGraph[N, E]) PathToTopWithContext(node *N, ctx rustfmt.Context) []GraphBorrowedPathEntry[N, E] {
	return g.ownedPathTo(node, func(n *N) graphOwnedEdgeIterator[N, E] {
		return &graphParentEdges[N, E]{iter: g.nodes.Iter(ctx), query: n}
	}, ctx)
}
func (g *OwnedGraph[N, E]) ownedPathTo(start *N, edges func(*N) graphOwnedEdgeIterator[N, E], ctx rustfmt.Context) []GraphBorrowedPathEntry[N, E] {
	g.check(ctx)
	type backlink struct {
		parent *N
		edge   *E
	}
	links := graphBTree[*N, backlink]{ops: graphBTreeOps[*N, backlink]{compare: func(a, b *N, ctx rustfmt.Context) int {
		return g.traits.CompareNode(rustvec.Ref[N]{Value: a, Context: ctx}, rustvec.Ref[N]{Value: b, Context: ctx})
	}}}
	queue := []*N{start}
	last := start
	for head := 0; head < len(queue); head++ {
		node := queue[head]
		last = node
		outEdges := true
		it := edges(node)
		for {
			child, ok := it.NextWithContext(ctx)
			if !ok {
				break
			}
			outEdges = false
			links.entryOrInsert(child.Node, func() backlink { queue = append(queue, child.Node); return backlink{parent: node, edge: child.Edge} }, ctx)
		}
		if outEdges {
			break
		}
	}
	var result []GraphBorrowedPathEntry[N, E]
	next := last
	for {
		link, ok := links.remove(next, ctx)
		if !ok {
			break
		}
		result = append(result, GraphBorrowedPathEntry[N, E]{Node: next, Edge: link.edge})
		next = link.parent
	}
	all := true
	for _, entry := range result {
		if !g.neNode(rustvec.Ref[N]{Value: entry.Node, Context: ctx}, rustvec.Ref[N]{Value: next, Context: ctx}) {
			all = false
			break
		}
	}
	if all {
		result = append(result, GraphBorrowedPathEntry[N, E]{Node: next})
	}
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	if g.config.DebugAssertions {
		for i := 1; i < len(result); i++ {
			n2, n1 := result[i-1].Node, result[i].Node
			// Option::or evaluates its second operand even when the first is
			// Some. Both lookups and their callback failures are observable.
			a := g.EdgeWithContext(n1, n2, ctx)
			b := g.EdgeWithContext(n2, n1, ctx)
			edge := a
			if edge == nil {
				edge = b
			}
			if result[i].Edge == nil || edge == nil || edge != result[i].Edge {
				ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "assertion failed: std::ptr::eq(self.edge(n1, n2).or(self.edge(n2, n1)).unwrap(), *e12)", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation("cargo.graph.path_edge"), CanUnwind: true})
			}
		}
		last := result[len(result)-1].Node
		it := edges(last)
		for {
			child, ok := it.NextWithContext(ctx)
			if !ok {
				break
			}
			contained := false
			for _, entry := range result {
				if g.traits.EqualNode(rustvec.Ref[N]{Value: entry.Node, Context: ctx}, rustvec.Ref[N]{Value: child.Node, Context: ctx}) {
					contained = true
					break
				}
			}
			if !contained {
				g.printOwnedForTest(ctx)
				ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "internal error: entered unreachable code: The last element in the path should not have outgoing edges", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation("cargo.graph.path_end"), CanUnwind: true})
			}
		}
	}
	return result
}

// std BTreeSet::from_iter stable-sorts and retains the last equal key. Its
// bulk tree builder and consuming iterator move only shared references here;
// they introduce no further user comparison, cloning or destruction calls.
func (g *OwnedGraph[N, E]) printOwnedForTest(ctx rustfmt.Context) {
	var inputs []*N
	it := g.nodes.Iter(ctx)
	for {
		node, _, ok := it.Next(ctx)
		if !ok {
			break
		}
		inputs = append(inputs, node.Ref(ctx).Value)
	}
	it = g.nodes.Iter(ctx)
	for {
		_, adjacent, ok := it.Next(ctx)
		if !ok {
			break
		}
		inner := adjacent.Ref(ctx).Value.Iter(ctx)
		for {
			node, _, ok := inner.Next(ctx)
			if !ok {
				break
			}
			inputs = append(inputs, node.Ref(ctx).Value)
		}
	}
	if len(inputs) != 0 {
		rustsort.Stable(inputs, rustsort.Type{Size: uint(unsafe.Sizeof((*N)(nil))), Align: uint(unsafe.Alignof((*N)(nil))), Freeze: true, Copy: true}, func(a, b **N, ctx rustfmt.Context) bool {
			return g.traits.LessNode(rustvec.Ref[N]{Value: *a, Context: ctx}, rustvec.Ref[N]{Value: *b, Context: ctx})
		}, ctx)
	}
	names := make([]*N, 0, len(inputs))
	for i, node := range inputs {
		if i+1 == len(inputs) || g.neNode(rustvec.Ref[N]{Value: node, Context: ctx}, rustvec.Ref[N]{Value: inputs[i+1], Context: ctx}) {
			names = append(names, node)
		}
	}
	name := func(node *N) uint {
		index, ok := btree.BinarySearch(len(names), func(i int) int {
			return g.traits.CompareNode(rustvec.Ref[N]{Value: names[i], Context: ctx}, rustvec.Ref[N]{Value: node, Context: ctx})
		})
		if !ok {
			ctx.Panic("cargo graph: normalized printer key missing")
		}
		return uint(index)
	}
	primitive := GraphTraits[uint, struct{}]{
		Node: imrc.Ops[uint]{Clone: func(n rustvec.Ref[uint]) *rustvec.Owned[uint] { return rustvec.Own(*n.Value) }}, Edge: imrc.Ops[struct{}]{Clone: func(rustvec.Ref[struct{}]) *rustvec.Owned[struct{}] { return rustvec.Own(struct{}{}) }},
		CompareNode: func(a, b rustvec.Ref[uint]) int {
			if *a.Value < *b.Value {
				return -1
			}
			if *a.Value > *b.Value {
				return 1
			}
			return 0
		}, LessNode: func(a, b rustvec.Ref[uint]) bool { return *a.Value < *b.Value }, EqualNode: func(a, b rustvec.Ref[uint]) bool { return *a.Value == *b.Value }, DefaultEdge: func(rustfmt.Context) *rustvec.Owned[struct{}] { return rustvec.Own(struct{}{}) },
		DisplayNode: func(n rustvec.Ref[uint], f *rustfmt.Formatter) error {
			// writeNode supplies fresh default flags for each {} argument;
			// the shared unsigned primitive therefore selects decimal here.
			return rustfmt.FormatDebugUint(uint64(*n.Value), uint(unsafe.Sizeof(uint(0)))*8, f)
		},
	}
	printed := NewOwnedGraphWithConfig(primitive, g.config)
	imrc.Scope(ctx, func() {
		it := g.nodes.Iter(ctx)
		for {
			node, _, ok := it.Next(ctx)
			if !ok {
				break
			}
			n1 := node.Ref(ctx).Value
			name1 := name(n1)
			printed.AddWithContext(rustvec.Own(name1), ctx)
			// Source indexing performs another lookup; reusing the outer
			// iterator's adjacency would lose comparison side effects.
			adjacent := g.nodes.IndexAt(rustvec.Ref[N]{Value: n1, Context: ctx}, ctx.SourcePanicLocation("cargo.graph.print.index"), ctx)
			inner := adjacent.Ref(ctx).Value.Iter(ctx)
			for {
				n2, _, ok := inner.Next(ctx)
				if !ok {
					break
				}
				name2 := name(n2.Ref(ctx).Value)
				printed.LinkWithContext(rustvec.Own(name1), rustvec.Own(name2), ctx).AssignWithContext(rustvec.Own(struct{}{}), ctx)
			}
		}
		err := g.config.printStderr(func(writer io.Writer, current rustfmt.Context) error {
			f := rustfmt.Formatter{Writer: writer, Context: current, Options: rustfmt.DefaultOptions()}
			f.Options.Alternate = true
			if err := f.WriteString("Graph for tests = "); err != nil {
				return err
			}
			if err := printed.WriteDebugFormatter(&f); err != nil {
				return err
			}
			return f.WriteString("\n")
		}, ctx)
		// Stderr::write_fmt's lease has ended before print_to raises the
		// formatted IO panic. Its hook may acquire stderr on another thread.
		if err != nil {
			g.stderrPanic(err, ctx)
		}
	}, func(ctx rustfmt.Context) { printed.CloseWithContext(ctx) })
}

func (g *OwnedGraph[N, E]) stderrPanic(err error, ctx rustfmt.Context) {
	ctx.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: ctx.SourcePanicLocation("rust.std.stdio.stderr.print_error"), CanUnwind: true}, func(processing rustfmt.Context) any {
		var display string
		if g.config.StderrErrorDisplay != nil {
			display = g.config.StderrErrorDisplay(err, processing)
		} else {
			var errno syscall.Errno
			if errors.As(err, &errno) {
				var text strings.Builder
				f := rustfmt.Formatter{Writer: &text, Context: processing, Options: rustfmt.DefaultOptions()}
				// The Rust raw OS error is i32. Use the actual host libc rather
				// than Go's errno table: unknown errors and locales differ.
				_ = rusterrno.FormatError(int32(errno), &f, g.config.UnixErrorBinding)
				_ = f.WriteString(" (os error ")
				_ = rustfmt.FormatDebugInt(int64(int32(errno)), 32, &f)
				_ = f.WriteString(")")
				display = text.String()
			} else {
				display = err.Error()
			}
		}
		return "failed printing to stderr: " + display
	})
}
