// SPDX-License-Identifier: MIT OR Apache-2.0
// Primary owned/borrowed interfaces for the complete pinned Cargo graph.rs.
package util

import (
	"io"
	"os"

	imrc "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src"
	ord "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src/ord"
	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststdio"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// GraphTraits are borrowed static Rust type/trait dictionaries. Inputs and
// clone/default results must describe these same type destructors. Callbacks
// receive actual value cells, permitting lawful interior state changes that do
// not alter the ordering identity of a stored key. No extra owned Fn is hidden
// in the dictionary. Translated panic sites use the explicit Context.Panic;
// raw Go callback-local defers retain Go timing before reaching a boundary.
type GraphTraits[N, E any] struct {
	Node        imrc.Ops[N]
	Edge        imrc.Ops[E]
	CompareNode func(rustvec.Ref[N], rustvec.Ref[N]) int
	// LessNode is the selected PartialOrd::lt, which may differ observably
	// from calling Ord::cmp even when both return a lawful ordering.
	LessNode     func(rustvec.Ref[N], rustvec.Ref[N]) bool
	EqualNode    func(rustvec.Ref[N], rustvec.Ref[N]) bool
	NotEqualNode func(rustvec.Ref[N], rustvec.Ref[N]) bool
	EqualEdge    func(rustvec.Ref[E], rustvec.Ref[E]) bool
	NotEqualEdge func(rustvec.Ref[E], rustvec.Ref[E]) bool
	DefaultEdge  func(rustfmt.Context) *rustvec.Owned[E]
	DisplayNode  func(rustvec.Ref[N], *rustfmt.Formatter) error
}
type OwnedGraphConfig struct {
	DebugAssertions bool
	OverflowChecks  bool
	Stderr          io.Writer
	// StderrBinding supplies the complete selected source-thread print
	// operation, including capture and any pre-held/reentrant stderr lease.
	// A nil binding uses the shared uncaptured host runtime. Stderr remains
	// the raw-writer convenience for isolated backend tests and embeddings.
	StderrBinding *ruststdio.Binding
	// StderrErrorDisplay is the selected Rust IO error's Display witness for
	// a nonstandard stderr backend. The default handles native errno errors.
	StderrErrorDisplay func(error, rustfmt.Context) string
	// UnixErrorBinding supplies the actual libc operation when cgo is disabled
	// or a caller uses a selected thread-local locale. The default uses cgo.
	UnixErrorBinding *rusterrno.Binding
}

func DefaultOwnedGraphConfig() OwnedGraphConfig {
	return OwnedGraphConfig{DebugAssertions: true, OverflowChecks: true, Stderr: os.Stderr}
}

func (config OwnedGraphConfig) printStderr(statement ruststdio.Statement, context rustfmt.Context) error {
	if config.StderrBinding != nil {
		if config.StderrBinding.PrintStderr == nil {
			context.Panic("cargo graph: incomplete stderr binding")
		}
		return config.StderrBinding.PrintStderr(statement, context)
	}
	if config.Stderr == nil || config.Stderr == os.Stderr {
		return ruststdio.PrintStderr(statement, context)
	}
	runtime := ruststdio.NewRuntime(func(bytes []byte, _ rustfmt.Context) (int, error) {
		return config.Stderr.Write(bytes)
	})
	return runtime.PrintStderr(ruststdio.NewThreadScope(), statement, context)
}

// OwnedGraph follows the pinned im-rc Rc copy-on-write representation. Clone
// shares its tree; element cloning occurs only where source mutation needs it.
// Close is the explicit source Drop. Copy this handle only through Clone.
// All returned cells/iterators/paths are borrows: obey Rust's shared/mutable
// lifetimes, including the caller-owned query in a returned path.
type OwnedGraph[N, E any] struct {
	nodes  ord.Map[N, ord.Map[N, E]]
	traits GraphTraits[N, E]
	config OwnedGraphConfig
	live   bool
}

func NewOwnedGraph[N, E any](traits GraphTraits[N, E]) *OwnedGraph[N, E] {
	return NewOwnedGraphWithConfig(traits, DefaultOwnedGraphConfig())
}
func NewOwnedGraphWithConfig[N, E any](traits GraphTraits[N, E], config OwnedGraphConfig) *OwnedGraph[N, E] {
	if traits.CompareNode == nil || traits.LessNode == nil || traits.EqualNode == nil || traits.Node.Clone == nil || traits.Edge.Clone == nil || traits.DefaultEdge == nil {
		panic("cargo graph: incomplete source trait dictionary")
	}
	if config.Stderr == nil {
		config.Stderr = os.Stderr
	}
	g := &OwnedGraph[N, E]{traits: traits, config: config, live: true}
	g.nodes = ord.New(g.outerTraits())
	return g
}
func (g *OwnedGraph[N, E]) check(ctx rustfmt.Context) {
	if g == nil || !g.live {
		ctx.Panic("cargo graph: use of consumed graph")
	}
}
func (g *OwnedGraph[N, E]) neNode(a, b rustvec.Ref[N]) bool {
	if g.traits.NotEqualNode != nil {
		return g.traits.NotEqualNode(a, b)
	}
	return !g.traits.EqualNode(a, b)
}
func (g *OwnedGraph[N, E]) neEdge(a, b rustvec.Ref[E]) bool {
	if g.traits.NotEqualEdge != nil {
		return g.traits.NotEqualEdge(a, b)
	}
	if g.traits.EqualEdge == nil {
		a.Context.Panic("cargo graph: missing edge PartialEq dictionary")
	}
	return !g.traits.EqualEdge(a, b)
}
func (g *OwnedGraph[N, E]) innerTraits() ord.Traits[N, E] {
	return ord.Traits[N, E]{Key: g.traits.Node, Value: g.traits.Edge, Compare: g.traits.CompareNode, NotEqualKey: g.neNode, NotEqualValue: g.neEdge, CheckedArithmetic: g.config.OverflowChecks}
}
func (g *OwnedGraph[N, E]) mapOps() imrc.Ops[ord.Map[N, E]] {
	drop := func(m *ord.Map[N, E], ctx rustfmt.Context) { m.Close(ctx) }
	return imrc.Ops[ord.Map[N, E]]{Drop: drop, Clone: func(ref rustvec.Ref[ord.Map[N, E]]) *rustvec.Owned[ord.Map[N, E]] {
		return rustvec.NewOwned(ref.Value.Clone(ref.Context), rustvec.ValueOps[ord.Map[N, E]]{Drop: drop})
	}}
}
func (g *OwnedGraph[N, E]) outerTraits() ord.Traits[N, ord.Map[N, E]] {
	return ord.Traits[N, ord.Map[N, E]]{Key: g.traits.Node, Value: g.mapOps(), Compare: g.traits.CompareNode, NotEqualKey: g.neNode, NotEqualValue: func(a, b rustvec.Ref[ord.Map[N, E]]) bool { return !a.Value.Equal(b.Value, a.Context) }, CheckedArithmetic: g.config.OverflowChecks}
}
func (g *OwnedGraph[N, E]) defaultMap(ctx rustfmt.Context) *rustvec.Owned[ord.Map[N, E]] {
	ops := g.mapOps()
	return rustvec.NewOwned(ord.New(g.innerTraits()), rustvec.ValueOps[ord.Map[N, E]]{Drop: ops.Drop})
}
func (g *OwnedGraph[N, E]) OwnNode(value N) *rustvec.Owned[N] {
	return rustvec.NewOwned(value, rustvec.ValueOps[N]{Drop: g.traits.Node.Drop})
}
func (g *OwnedGraph[N, E]) OwnEdge(value E) *rustvec.Owned[E] {
	return rustvec.NewOwned(value, rustvec.ValueOps[E]{Drop: g.traits.Edge.Drop})
}
func (g *OwnedGraph[N, E]) Close() { g.CloseWithContext(rustfmt.Context{}) }
func (g *OwnedGraph[N, E]) CloseWithContext(ctx rustfmt.Context) {
	if g == nil || !g.live {
		return
	}
	g.live = false
	g.nodes.Close(ctx)
}
func (g *OwnedGraph[N, E]) Clone() *OwnedGraph[N, E] { return g.CloneWithContext(rustfmt.Context{}) }
func (g *OwnedGraph[N, E]) CloneWithContext(ctx rustfmt.Context) *OwnedGraph[N, E] {
	g.check(ctx)
	return &OwnedGraph[N, E]{nodes: g.nodes.Clone(ctx), traits: g.traits, config: g.config, live: true}
}
func (g *OwnedGraph[N, E]) CloneFrom(other *OwnedGraph[N, E]) {
	g.CloneFromWithContext(other, rustfmt.Context{})
}
func (g *OwnedGraph[N, E]) CloneFromWithContext(other *OwnedGraph[N, E], ctx rustfmt.Context) {
	g.check(ctx)
	other.check(ctx)
	replacement := other.CloneWithContext(ctx)
	imrc.Scope(ctx, func() { g.CloseWithContext(ctx) }, func(rustfmt.Context) { *g = *replacement; replacement.live = false })
}
func (g *OwnedGraph[N, E]) Len() uint                  { g.check(rustfmt.Context{}); return g.nodes.Len() }
func (g *OwnedGraph[N, E]) Add(node *rustvec.Owned[N]) { g.AddWithContext(node, rustfmt.Context{}) }
func (g *OwnedGraph[N, E]) AddWithContext(node *rustvec.Owned[N], ctx rustfmt.Context) {
	g.check(ctx)
	rustvec.ConsumeValue(node, ctx, func(node *rustvec.Owned[N]) struct{} {
		g.nodes.EntryOrInsert(imrc.MoveIn(node, g.traits.Node, ctx), g.defaultMap, ctx)
		return struct{}{}
	})
}

// GraphEdgeMut is an exclusive E place. Value borrows the actual cell; Assign
// implements owned assignment with the original type's Drop glue. End this
// exclusive borrow before using other graph operations.
type GraphEdgeMut[E any] struct{ cell *imrc.Cell[E] }

func (e GraphEdgeMut[E]) Value() *E                               { return e.ValueWithContext(rustfmt.Context{}) }
func (e GraphEdgeMut[E]) ValueWithContext(ctx rustfmt.Context) *E { return e.cell.Ref(ctx).Value }
func (e GraphEdgeMut[E]) Assign(value *rustvec.Owned[E]) {
	e.AssignWithContext(value, rustfmt.Context{})
}
func (e GraphEdgeMut[E]) AssignWithContext(value *rustvec.Owned[E], ctx rustfmt.Context) {
	e.cell.Assign(value, ctx)
}
func (g *OwnedGraph[N, E]) Link(node, child *rustvec.Owned[N]) GraphEdgeMut[E] {
	return g.LinkWithContext(node, child, rustfmt.Context{})
}
func (g *OwnedGraph[N, E]) LinkWithContext(node, child *rustvec.Owned[N], ctx rustfmt.Context) GraphEdgeMut[E] {
	g.check(ctx)
	return rustvec.ConsumeValue(node, ctx, func(node *rustvec.Owned[N]) GraphEdgeMut[E] {
		return rustvec.ConsumeValue(child, ctx, func(child *rustvec.Owned[N]) GraphEdgeMut[E] {
			adjacent := g.nodes.EntryOrInsert(imrc.MoveIn(node, g.traits.Node, ctx), g.defaultMap, ctx)
			return GraphEdgeMut[E]{cell: adjacent.Ref(ctx).Value.EntryOrInsert(imrc.MoveIn(child, g.traits.Node, ctx), g.traits.DefaultEdge, ctx)}
		})
	})
}
func (g *OwnedGraph[N, E]) Contains(node *N) bool {
	return g.ContainsWithContext(node, rustfmt.Context{})
}
func (g *OwnedGraph[N, E]) ContainsWithContext(node *N, ctx rustfmt.Context) bool {
	g.check(ctx)
	return g.nodes.Get(rustvec.Ref[N]{Value: node, Context: ctx}, ctx) != nil
}

// OwnedGraphContainsBorrowed is the full Borrow<Q> query bridge. Comparison
// borrows a stored N and the caller's query; it never consumes either value.
func OwnedGraphContainsBorrowed[N, E, Q any](g *OwnedGraph[N, E], query *Q, borrow func(rustvec.Ref[N]) *Q, compare func(*Q, *Q, rustfmt.Context) int, ctx rustfmt.Context) bool {
	g.check(ctx)
	return g.nodes.GetQuery(func(node rustvec.Ref[N]) int { return compare(borrow(node), query, ctx) }, ctx) != nil
}
func (g *OwnedGraph[N, E]) Edge(from, to *N) *E {
	return g.EdgeWithContext(from, to, rustfmt.Context{})
}
func (g *OwnedGraph[N, E]) EdgeWithContext(from, to *N, ctx rustfmt.Context) *E {
	g.check(ctx)
	adjacent := g.nodes.Get(rustvec.Ref[N]{Value: from, Context: ctx}, ctx)
	if adjacent == nil {
		return nil
	}
	edge := adjacent.Ref(ctx).Value.Get(rustvec.Ref[N]{Value: to, Context: ctx}, ctx)
	if edge == nil {
		return nil
	}
	return edge.Ref(ctx).Value
}

type GraphNodeIter[N, E any] struct{ inner *ord.Iter[N, ord.Map[N, E]] }

func (g *OwnedGraph[N, E]) Iter() *GraphNodeIter[N, E] { return g.IterWithContext(rustfmt.Context{}) }
func (g *OwnedGraph[N, E]) IterWithContext(ctx rustfmt.Context) *GraphNodeIter[N, E] {
	g.check(ctx)
	return &GraphNodeIter[N, E]{inner: g.nodes.Iter(ctx)}
}
func (it *GraphNodeIter[N, E]) Next() (*N, bool) { return it.NextWithContext(rustfmt.Context{}) }
func (it *GraphNodeIter[N, E]) NextWithContext(ctx rustfmt.Context) (*N, bool) {
	key, _, ok := it.inner.Next(ctx)
	if !ok {
		return nil, false
	}
	return key.Ref(ctx).Value, true
}
func (it *GraphNodeIter[N, E]) SizeHint() rustvec.Hint {
	lower, upper := it.inner.SizeHint()
	return rustvec.Hint{Lower: lower, Upper: *upper, HasUpper: true}
}
func (it *GraphNodeIter[N, E]) IntoIterator() rustvec.Iterator[*N] {
	return rustvec.NewIterator(it, rustvec.IteratorOps[*GraphNodeIter[N, E], *N]{Next: func(ref rustvec.Ref[*GraphNodeIter[N, E]]) (*N, bool) {
		return (*ref.Value).NextWithContext(ref.Context)
	}, SizeHint: func(ref rustvec.Ref[*GraphNodeIter[N, E]]) rustvec.Hint { return (*ref.Value).SizeHint() }})
}

type GraphEdgeBorrow[N, E any] struct {
	Node *N
	Edge *E
}
type GraphEdgesIter[N, E any] struct {
	adjacent      *ord.Map[N, E]
	inner         *ord.Iter[N, E]
	started, done bool
}

func (g *OwnedGraph[N, E]) Edges(from *N) *GraphEdgesIter[N, E] {
	return g.EdgesWithContext(from, rustfmt.Context{})
}
func (g *OwnedGraph[N, E]) EdgesWithContext(from *N, ctx rustfmt.Context) *GraphEdgesIter[N, E] {
	g.check(ctx)
	entry := g.nodes.Get(rustvec.Ref[N]{Value: from, Context: ctx}, ctx)
	out := &GraphEdgesIter[N, E]{}
	if entry == nil {
		out.done = true
	} else {
		out.adjacent = entry.Ref(ctx).Value
	}
	return out
}
func (it *GraphEdgesIter[N, E]) Next() (GraphEdgeBorrow[N, E], bool) {
	return it.NextWithContext(rustfmt.Context{})
}
func (it *GraphEdgesIter[N, E]) NextWithContext(ctx rustfmt.Context) (GraphEdgeBorrow[N, E], bool) {
	if it.done {
		return GraphEdgeBorrow[N, E]{}, false
	}
	if !it.started {
		it.inner = it.adjacent.Iter(ctx)
		it.started = true
	}
	key, value, ok := it.inner.Next(ctx)
	if !ok {
		it.done = true
		return GraphEdgeBorrow[N, E]{}, false
	}
	return GraphEdgeBorrow[N, E]{Node: key.Ref(ctx).Value, Edge: value.Ref(ctx).Value}, true
}
func (it *GraphEdgesIter[N, E]) SizeHint() rustvec.Hint {
	if it.done {
		return rustvec.Hint{HasUpper: true}
	}
	if it.started {
		lower, upper := it.inner.SizeHint()
		return rustvec.Hint{Lower: lower, Upper: *upper, HasUpper: true}
	}
	return rustvec.Hint{}
}
func (it *GraphEdgesIter[N, E]) IntoIterator() rustvec.Iterator[GraphEdgeBorrow[N, E]] {
	return rustvec.NewIterator(it, rustvec.IteratorOps[*GraphEdgesIter[N, E], GraphEdgeBorrow[N, E]]{Next: func(ref rustvec.Ref[*GraphEdgesIter[N, E]]) (GraphEdgeBorrow[N, E], bool) {
		return (*ref.Value).NextWithContext(ref.Context)
	}, SizeHint: func(ref rustvec.Ref[*GraphEdgesIter[N, E]]) rustvec.Hint { return (*ref.Value).SizeHint() }})
}
func (g *OwnedGraph[N, E]) Equal(other *OwnedGraph[N, E]) bool {
	return g.EqualWithContext(other, rustfmt.Context{})
}
func (g *OwnedGraph[N, E]) EqualWithContext(other *OwnedGraph[N, E], ctx rustfmt.Context) bool {
	g.check(ctx)
	other.check(ctx)
	return g.nodes.Equal(&other.nodes, ctx)
}

func (g *OwnedGraph[N, E]) Reversed() *OwnedGraph[N, E] {
	return g.ReversedWithContext(rustfmt.Context{})
}
func (g *OwnedGraph[N, E]) ReversedWithContext(ctx rustfmt.Context) (result *OwnedGraph[N, E]) {
	g.check(ctx)
	ret := NewOwnedGraphWithConfig(g.traits, g.config)
	remaining := ret
	imrc.Scope(ctx, func() {
		it := g.IterWithContext(ctx)
		for {
			node, ok := it.NextWithContext(ctx)
			if !ok {
				break
			}
			ret.AddWithContext(g.traits.Node.Clone(rustvec.Ref[N]{Value: node, Context: ctx}), ctx)
			edges := g.EdgesWithContext(node, ctx)
			for {
				edge, ok := edges.NextWithContext(ctx)
				if !ok {
					break
				}
				// Rust assignment evaluates RHS before evaluating its place.
				value := g.traits.Edge.Clone(rustvec.Ref[E]{Value: edge.Edge, Context: ctx})
				rustvec.ConsumeValue(value, ctx, func(value *rustvec.Owned[E]) struct{} {
					child := g.traits.Node.Clone(rustvec.Ref[N]{Value: edge.Node, Context: ctx})
					rustvec.ConsumeValue(child, ctx, func(child *rustvec.Owned[N]) struct{} {
						parent := g.traits.Node.Clone(rustvec.Ref[N]{Value: node, Context: ctx})
						ret.LinkWithContext(child, parent, ctx).AssignWithContext(value, ctx)
						return struct{}{}
					})
					return struct{}{}
				})
			}
		}
		result = ret
		remaining = nil
	}, func(ctx rustfmt.Context) { remaining.CloseWithContext(ctx) })
	return
}

// WriteDebugFormatter translates the source's writes directly. Nested Display
// gets a fresh default Formatter, as each {} formatting argument does in Rust.
// Its writer/context are borrowed; errors stop before any later iterator/call.
func (g *OwnedGraph[N, E]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	g.check(f.Context)
	if err := f.WriteString("Graph {\n"); err != nil {
		return err
	}
	it := g.nodes.Iter(f.Context)
	for {
		node, adjacent, ok := it.Next(f.Context)
		if !ok {
			break
		}
		if err := g.writeNode(node.Ref(f.Context), "  - ", f); err != nil {
			return err
		}
		edges := adjacent.Ref(f.Context).Value.Iter(f.Context)
		for {
			child, _, ok := edges.Next(f.Context)
			if !ok {
				break
			}
			if err := g.writeNode(child.Ref(f.Context), "    - ", f); err != nil {
				return err
			}
		}
	}
	return f.WriteString("}")
}
func (g *OwnedGraph[N, E]) writeNode(node rustvec.Ref[N], prefix string, f *rustfmt.Formatter) error {
	if err := f.WriteString(prefix); err != nil {
		return err
	}
	if g.traits.DisplayNode == nil {
		f.Context.Panic("cargo graph: missing Display dictionary")
	}
	child := rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}
	if err := g.traits.DisplayNode(node, &child); err != nil {
		return err
	}
	return f.WriteString("\n")
}
