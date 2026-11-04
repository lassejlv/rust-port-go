// SPDX-License-Identifier: MIT OR Apache-2.0
// Full port of rust-lang/cargo src/util/graph.rs at
// 24de4721844c7a4d839072bab6fb66d41c9bc1f4, Git blob
// 74cdc51a0c36f21ce64f291540be571c05b12d63 (303 lines, 8,926 bytes).
// Source SHA-256: 238123cc4c8300921008db5bcec1bbdf0946a023e0acafe9997e6917b5fada51.
// See tools/port-cargo-graph/PORTING.md for adapters and validation.
package util

import (
	"fmt"
	"io"
	"iter"
	"os"
	"slices"
	"strings"
)

// GraphOptions supplies the Rust Ord, Default, Clone, Eq and Display behavior.
// Compare is required and must define a total order consistent with Go ==.
// EdgeDefault defaults to Go's zero E. CloneNode and CloneEdge default to value
// assignment: supply adapters for reference-bearing values whose Rust Clone
// semantics require independent data. EqualEdge is required only by Equal.
// DisplayNode defaults to fmt.Sprint. Adapters must remain fixed for a graph.
//
// Rust enables the path invariants in debug builds. They are enabled by default
// here; DisableDebugAssertions selects the upstream release-build behavior.
type GraphOptions[N comparable, E any] struct {
	Compare                func(N, N) int
	EdgeDefault            func() E
	CloneNode              func(N) N
	CloneEdge              func(E) E
	EqualEdge              func(E, E) bool
	DisplayNode            func(N) string
	DisableDebugAssertions bool
}

// Graph stores sorted nodes and sorted outgoing edges, matching im_rc::OrdMap.
// Link adds its source node but deliberately does not add its child node.
// Graph must be cloned with Clone rather than copied by assignment. Operations
// and iterator use must not overlap mutations, as required by Rust's borrows.
type Graph[N comparable, E any] struct {
	nodes   map[N]map[N]*graphEdge[E]
	options GraphOptions[N, E]
}

// Nonzero storage keeps each edge's address distinct even when E has size zero.
type graphEdge[E any] struct {
	value E
	id    byte
}

// GraphPathEntry records a node and the edge from the previous path node.
// A cycle can have a non-nil first Edge, exactly as in the original algorithm.
// Edge pointers borrow graph storage and must not be written except through a
// Link call; the original edge()/path methods return shared Rust references.
type GraphPathEntry[N comparable, E any] struct {
	Node N
	Edge *E
}

const (
	GraphMissingNodePanic = "cargo graph: sort encountered a child that is not a node"
	GraphPathEdgePanic    = "cargo graph: path edge does not match graph edge"
	GraphPathEndPanic     = "The last element in the path should not have outgoing edges"
)

// NewGraph implements Graph::new and its equivalent Default constructor.
func NewGraph[N comparable, E any](options GraphOptions[N, E]) *Graph[N, E] {
	if options.Compare == nil {
		panic("cargo graph: node comparison adapter is required")
	}
	return &Graph[N, E]{nodes: make(map[N]map[N]*graphEdge[E]), options: options}
}

func (graph *Graph[N, E]) cloneNode(node N) N {
	if graph.options.CloneNode != nil {
		return graph.options.CloneNode(node)
	}
	return node
}
func (graph *Graph[N, E]) cloneEdge(edge E) E {
	if graph.options.CloneEdge != nil {
		return graph.options.CloneEdge(edge)
	}
	return edge
}
func (graph *Graph[N, E]) keys(nodes map[N]*graphEdge[E]) []N {
	keys := make([]N, 0, len(nodes))
	for node := range nodes {
		keys = append(keys, node)
	}
	slices.SortFunc(keys, graph.options.Compare)
	return keys
}
func (graph *Graph[N, E]) nodeKeys() []N {
	keys := make([]N, 0, len(graph.nodes))
	for node := range graph.nodes {
		keys = append(keys, node)
	}
	slices.SortFunc(keys, graph.options.Compare)
	return keys
}

func (graph *Graph[N, E]) Add(node N) {
	if _, exists := graph.nodes[node]; !exists {
		graph.nodes[node] = make(map[N]*graphEdge[E])
	}
}

// Link returns the existing edge or inserts EdgeDefault(), adding only node.
func (graph *Graph[N, E]) Link(node, child N) *E {
	graph.Add(node)
	edges := graph.nodes[node]
	if edge, exists := edges[child]; exists {
		return &edge.value
	}
	edge := new(graphEdge[E])
	if graph.options.EdgeDefault != nil {
		edge.value = graph.options.EdgeDefault()
	}
	edges[child] = edge
	return &edge.value
}

// Reversed returns a graph obtained by reversing all edges, cloning values.
func (graph *Graph[N, E]) Reversed() *Graph[N, E] {
	result := NewGraph(graph.options)
	for node := range graph.Iter() {
		result.Add(graph.cloneNode(node))
		for child, edge := range graph.Edges(node) {
			*result.Link(graph.cloneNode(child), graph.cloneNode(node)) = graph.cloneEdge(*edge)
		}
	}
	return result
}

func (graph *Graph[N, E]) Contains(node N) bool {
	_, exists := graph.nodes[node]
	return exists
}

// ContainsBy adapts Rust's Borrow<Q> lookup without requiring Q to have type N.
// compareNodeToQuery must compare each node's borrowed value to the query using
// the same ordering as Compare. It returns negative/zero/positive in that order.
func (graph *Graph[N, E]) ContainsBy(compareNodeToQuery func(N) int) bool {
	_, exists := slices.BinarySearchFunc(graph.nodeKeys(), struct{}{}, func(node N, _ struct{}) int {
		return compareNodeToQuery(node)
	})
	return exists
}

// Edge returns nil when either the source or edge is absent.
func (graph *Graph[N, E]) Edge(from, to N) *E {
	if edge := graph.nodes[from][to]; edge != nil {
		return &edge.value
	}
	return nil
}

// Edges yields outgoing child nodes and borrowed values in node order.
func (graph *Graph[N, E]) Edges(from N) iter.Seq2[N, *E] {
	return func(yield func(N, *E) bool) {
		edges := graph.nodes[from]
		for _, node := range graph.keys(edges) {
			if !yield(node, &edges[node].value) {
				return
			}
		}
	}
}

// Sort implements Cargo's marked depth-first postorder, including on cycles.
// It panics if an edge points to a child that has not been added as a node.
func (graph *Graph[N, E]) Sort() []N {
	result := make([]N, 0, len(graph.nodes))
	marks := make(map[N]bool)
	for _, node := range graph.nodeKeys() {
		graph.sortInnerVisit(node, &result, marks)
	}
	return result
}

func (graph *Graph[N, E]) sortInnerVisit(node N, destination *[]N, marks map[N]bool) {
	if marks[node] {
		return
	}
	marks[node] = true
	edges, exists := graph.nodes[node]
	if !exists {
		panic(GraphMissingNodePanic)
	}
	for _, child := range graph.keys(edges) {
		graph.sortInnerVisit(child, destination, marks)
	}
	*destination = append(*destination, graph.cloneNode(node))
}

func (graph *Graph[N, E]) Iter() iter.Seq[N] {
	return func(yield func(N) bool) {
		for _, node := range graph.nodeKeys() {
			if !yield(node) {
				return
			}
		}
	}
}

func (graph *Graph[N, E]) Len() int { return len(graph.nodes) }

// IsPathFromTo checks edges, so a node has no path to itself without a cycle.
// Popping a child with no node entry ends the entire search, preserving the
// original while-let condition even if other stack entries remain pending.
func (graph *Graph[N, E]) IsPathFromTo(from, to N) bool {
	stack := []N{from}
	seen := map[N]bool{from: true}
	for len(stack) != 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		edges, exists := graph.nodes[node]
		if !exists {
			break
		}
		for _, child := range graph.keys(edges) {
			if child == to {
				return true
			}
			if !seen[child] {
				seen[child] = true
				stack = append(stack, child)
			}
		}
	}
	return false
}

// PathToBottom finds one shortest path towards a leaf. Cyclic graphs retain
// the upstream queue/backlink-removal behavior and debug assertion failures.
func (graph *Graph[N, E]) PathToBottom(node N) []GraphPathEntry[N, E] {
	return graph.pathTo(node, graph.Edges)
}

// PathToTop finds one shortest path following incoming edges towards a root.
func (graph *Graph[N, E]) PathToTop(node N) []GraphPathEntry[N, E] {
	return graph.pathTo(node, func(node N) iter.Seq2[N, *E] {
		return func(yield func(N, *E) bool) {
			for _, parent := range graph.nodeKeys() {
				if edge := graph.Edge(parent, node); edge != nil && !yield(parent, edge) {
					return
				}
			}
		}
	})
}

func (graph *Graph[N, E]) pathTo(start N, edges func(N) iter.Seq2[N, *E]) []GraphPathEntry[N, E] {
	type backLink struct {
		parent N
		edge   *E
	}
	backLinks := make(map[N]backLink)
	queue := []N{start}
	last := start
	for head := 0; head < len(queue); head++ {
		node := queue[head]
		last = node
		outEdges := true
		for child, edge := range edges(node) {
			outEdges = false
			if _, exists := backLinks[child]; !exists {
				queue = append(queue, child)
				backLinks[child] = backLink{node, edge}
			}
		}
		if outEdges {
			break
		}
	}

	result := make([]GraphPathEntry[N, E], 0)
	next := last
	for {
		link, exists := backLinks[next]
		if !exists {
			break
		}
		delete(backLinks, next)
		result = append(result, GraphPathEntry[N, E]{next, link.edge})
		next = link.parent
	}
	if !slices.ContainsFunc(result, func(entry GraphPathEntry[N, E]) bool { return entry.Node == next }) {
		result = append(result, GraphPathEntry[N, E]{next, nil})
	}
	slices.Reverse(result)

	if !graph.options.DisableDebugAssertions {
		for index := 1; index < len(result); index++ {
			node2, node1 := result[index-1].Node, result[index].Node
			edge := graph.Edge(node1, node2)
			if edge == nil {
				edge = graph.Edge(node2, node1)
			}
			if result[index].Edge == nil || edge == nil || edge != result[index].Edge {
				panic(GraphPathEdgePanic)
			}
		}
		last := result[len(result)-1].Node
		for child := range edges(last) {
			if !slices.ContainsFunc(result, func(entry GraphPathEntry[N, E]) bool { return entry.Node == child }) {
				_ = graph.PrintForTest(os.Stderr)
				panic(GraphPathEndPanic)
			}
		}
	}
	return result
}

// Clone copies the adjacency maps and applies the Clone adapters to their data.
// It replaces im_rc's copy-on-write representation with independent Go storage.
func (graph *Graph[N, E]) Clone() *Graph[N, E] {
	result := NewGraph(graph.options)
	for node := range graph.Iter() {
		copyNode := graph.cloneNode(node)
		edges := make(map[N]*graphEdge[E])
		for child, edge := range graph.Edges(node) {
			edges[graph.cloneNode(child)] = &graphEdge[E]{value: graph.cloneEdge(*edge)}
		}
		result.nodes[copyNode] = edges
	}
	return result
}

// Equal implements PartialEq/Eq using the supplied E equality adapter.
func (graph *Graph[N, E]) Equal(other *Graph[N, E]) bool {
	if graph.options.EqualEdge == nil {
		panic("cargo graph: edge equality adapter is required")
	}
	if len(graph.nodes) != len(other.nodes) {
		return false
	}
	for node, edges := range graph.nodes {
		otherEdges, exists := other.nodes[node]
		if !exists || len(edges) != len(otherEdges) {
			return false
		}
		for child, edge := range edges {
			otherEdge, exists := otherEdges[child]
			if !exists || !graph.options.EqualEdge(edge.value, otherEdge.value) {
				return false
			}
		}
	}
	return true
}

// Debug reproduces Cargo's fmt::Debug output, using Display for the nodes.
func (graph *Graph[N, E]) Debug() string {
	var out strings.Builder
	out.WriteString("Graph {\n")
	display := graph.options.DisplayNode
	if display == nil {
		display = func(node N) string { return fmt.Sprint(node) }
	}
	for node := range graph.Iter() {
		fmt.Fprintf(&out, "  - %s\n", display(node))
		for child := range graph.Edges(node) {
			fmt.Fprintf(&out, "    - %s\n", display(child))
		}
	}
	out.WriteByte('}')
	return out.String()
}

func (graph *Graph[N, E]) String() string { return graph.Debug() }
func (graph *Graph[N, E]) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, graph.Debug())
}

// TestString reproduces the debug-only print_for_test normalization: all nodes
// and child-only keys receive consecutive integers in their original order.
func (graph *Graph[N, E]) TestString() string {
	names := make(map[N]bool)
	for node, edges := range graph.nodes {
		names[node] = true
		for child := range edges {
			names[child] = true
		}
	}
	ordered := make([]N, 0, len(names))
	for node := range names {
		ordered = append(ordered, node)
	}
	slices.SortFunc(ordered, graph.options.Compare)
	indices := make(map[N]int)
	for index, node := range ordered {
		indices[node] = index
	}
	var out strings.Builder
	out.WriteString("Graph for tests = Graph {\n")
	for node := range graph.Iter() {
		fmt.Fprintf(&out, "  - %d\n", indices[node])
		for child := range graph.Edges(node) {
			fmt.Fprintf(&out, "    - %d\n", indices[child])
		}
	}
	out.WriteString("}\n")
	return out.String()
}

// PrintForTest directs the upstream stderr representation to the given writer.
// Path debug assertions call this with os.Stderr, matching eprintln!.
func (graph *Graph[N, E]) PrintForTest(writer io.Writer) error {
	_, err := io.WriteString(writer, graph.TestString())
	return err
}
