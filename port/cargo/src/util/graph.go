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
// Compare is required and defines the total order and stored-key identity;
// nodes need neither Go equality nor hashing. EqualNode and NotEqualNode adapt
// direct Rust == and != expressions, independently of ordered-container lookup.
// EqualNode defaults to Compare(a,b)==0; NotEqualNode defaults to !EqualNode.
// The adapters must obey Rust's Ord/Eq laws.
//
// EdgeDefault defaults to Go's zero E. CloneNode and CloneEdge default to value
// assignment: supply adapters for reference-bearing values whose Rust Clone
// semantics require independent data. Equal requires EqualEdge or NotEqualEdge;
// map equality uses Rust's !=, with NotEqualEdge defaulting to !EqualEdge.
// DisplayNode defaults to fmt.Sprint. Adapters must remain fixed for a graph.
//
// Rust enables the path invariants in debug builds. They are enabled by default
// here; DisableDebugAssertions selects the upstream release-build behavior.
type GraphOptions[N any, E any] struct {
	Compare                func(N, N) int
	EdgeDefault            func() E
	CloneNode              func(N) N
	CloneEdge              func(E) E
	EqualEdge              func(E, E) bool
	DisplayNode            func(N) string
	DisableDebugAssertions bool
	EqualNode              func(N, N) bool
	NotEqualNode           func(N, N) bool
	NotEqualEdge           func(E, E) bool
}

// Graph stores sorted nodes and sorted outgoing edges, matching im_rc::OrdMap.
// Link adds its source node but deliberately does not add its child node.
// Graph must be cloned with Clone rather than copied by assignment. Operations
// and iterator use must not overlap mutations, as required by Rust's borrows.
// Keys must not be mutated while stored, including through referenced data.
type Graph[N any, E any] struct {
	nodes   graphOrdered[N, *graphOrdered[N, *graphEdge[E]]]
	options GraphOptions[N, E]
}

// graphOrdered replaces ordered maps/sets without restricting their key type.
// Entry values containing references are separately allocated where pointers
// escape, so moving the entry slice never changes an edge's address.
type graphOrdered[K any, V any] struct {
	entries []graphOrderedEntry[K, V]
}
type graphOrderedEntry[K any, V any] struct {
	key   K
	value V
}

func (ordered *graphOrdered[K, V]) position(key K, compare func(K, K) int) (int, bool) {
	low, high := 0, len(ordered.entries)
	for low < high {
		middle := low + (high-low)/2
		switch comparison := compare(ordered.entries[middle].key, key); {
		case comparison < 0:
			low = middle + 1
		case comparison > 0:
			high = middle
		default:
			return middle, true
		}
	}
	return low, false
}
func (ordered *graphOrdered[K, V]) get(key K, compare func(K, K) int) (V, bool) {
	index, exists := ordered.position(key, compare)
	if exists {
		return ordered.entries[index].value, true
	}
	var zero V
	return zero, false
}
func (ordered *graphOrdered[K, V]) insertAt(index int, key K, value V) {
	ordered.entries = append(ordered.entries, graphOrderedEntry[K, V]{})
	copy(ordered.entries[index+1:], ordered.entries[index:])
	ordered.entries[index] = graphOrderedEntry[K, V]{key, value}
}
func (ordered *graphOrdered[K, V]) insertIfAbsent(key K, value V, compare func(K, K) int) bool {
	index, exists := ordered.position(key, compare)
	if exists {
		return false
	}
	ordered.insertAt(index, key, value)
	return true
}
func (ordered *graphOrdered[K, V]) remove(key K, compare func(K, K) int) (V, bool) {
	index, exists := ordered.position(key, compare)
	if !exists {
		var zero V
		return zero, false
	}
	value := ordered.entries[index].value
	copy(ordered.entries[index:], ordered.entries[index+1:])
	last := len(ordered.entries) - 1
	ordered.entries[last] = graphOrderedEntry[K, V]{}
	ordered.entries = ordered.entries[:last]
	return value, true
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
type GraphPathEntry[N any, E any] struct {
	Node N
	Edge *E
}

const (
	GraphMissingNodePanic = "cargo graph: sort encountered a child that is not a node"
	GraphPathEdgePanic    = "cargo graph: path edge does not match graph edge"
	GraphPathEndPanic     = "The last element in the path should not have outgoing edges"
)

// NewGraph implements Graph::new and its equivalent Default constructor.
func NewGraph[N any, E any](options GraphOptions[N, E]) *Graph[N, E] {
	if options.Compare == nil {
		panic("cargo graph: node comparison adapter is required")
	}
	return &Graph[N, E]{options: options}
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
func (graph *Graph[N, E]) equalNode(left, right N) bool {
	if graph.options.EqualNode != nil {
		return graph.options.EqualNode(left, right)
	}
	return graph.options.Compare(left, right) == 0
}
func (graph *Graph[N, E]) notEqualNode(left, right N) bool {
	if graph.options.NotEqualNode != nil {
		return graph.options.NotEqualNode(left, right)
	}
	return !graph.equalNode(left, right)
}
func (graph *Graph[N, E]) ensureNode(node N) *graphOrdered[N, *graphEdge[E]] {
	index, exists := graph.nodes.position(node, graph.options.Compare)
	if exists {
		return graph.nodes.entries[index].value
	}
	edges := new(graphOrdered[N, *graphEdge[E]])
	graph.nodes.insertAt(index, node, edges)
	return edges
}

func (graph *Graph[N, E]) Add(node N) { graph.ensureNode(node) }

// Link returns the existing edge or inserts EdgeDefault(), adding only node.
func (graph *Graph[N, E]) Link(node, child N) *E {
	edges := graph.ensureNode(node)
	index, exists := edges.position(child, graph.options.Compare)
	if exists {
		return &edges.entries[index].value.value
	}
	edge := new(graphEdge[E])
	if graph.options.EdgeDefault != nil {
		edge.value = graph.options.EdgeDefault()
	}
	edges.insertAt(index, child, edge)
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
	_, exists := graph.nodes.position(node, graph.options.Compare)
	return exists
}

// ContainsBy adapts Rust's Borrow<Q> lookup without requiring Q to have type N.
// compareNodeToQuery must compare each node's borrowed value to the query using
// the same ordering as Compare. It returns negative/zero/positive in that order.
func (graph *Graph[N, E]) ContainsBy(compareNodeToQuery func(N) int) bool {
	_, exists := slices.BinarySearchFunc(graph.nodes.entries, struct{}{}, func(entry graphOrderedEntry[N, *graphOrdered[N, *graphEdge[E]]], _ struct{}) int {
		return compareNodeToQuery(entry.key)
	})
	return exists
}

// Edge returns nil when either the source or edge is absent.
func (graph *Graph[N, E]) Edge(from, to N) *E {
	edges, exists := graph.nodes.get(from, graph.options.Compare)
	if !exists {
		return nil
	}
	edge, exists := edges.get(to, graph.options.Compare)
	if !exists {
		return nil
	}
	return &edge.value
}

// Edges yields outgoing child nodes and borrowed values in node order.
func (graph *Graph[N, E]) Edges(from N) iter.Seq2[N, *E] {
	return func(yield func(N, *E) bool) {
		edges, exists := graph.nodes.get(from, graph.options.Compare)
		if !exists {
			return
		}
		for _, entry := range edges.entries {
			if !yield(entry.key, &entry.value.value) {
				return
			}
		}
	}
}

// Sort implements Cargo's marked depth-first postorder, including on cycles.
// It panics if an edge points to a child that has not been added as a node.
func (graph *Graph[N, E]) Sort() []N {
	result := make([]N, 0, len(graph.nodes.entries))
	marks := new(graphOrdered[N, struct{}])
	for node := range graph.Iter() {
		graph.sortInnerVisit(node, &result, marks)
	}
	return result
}

func (graph *Graph[N, E]) sortInnerVisit(node N, destination *[]N, marks *graphOrdered[N, struct{}]) {
	if !marks.insertIfAbsent(graph.cloneNode(node), struct{}{}, graph.options.Compare) {
		return
	}
	edges, exists := graph.nodes.get(node, graph.options.Compare)
	if !exists {
		panic(GraphMissingNodePanic)
	}
	for _, child := range edges.entries {
		graph.sortInnerVisit(child.key, destination, marks)
	}
	*destination = append(*destination, graph.cloneNode(node))
}

func (graph *Graph[N, E]) Iter() iter.Seq[N] {
	return func(yield func(N) bool) {
		for _, entry := range graph.nodes.entries {
			if !yield(entry.key) {
				return
			}
		}
	}
}

func (graph *Graph[N, E]) Len() int { return len(graph.nodes.entries) }

// IsPathFromTo checks edges, so a node has no path to itself without a cycle.
// Popping a child with no node entry ends the entire search, preserving the
// original while-let condition even if other stack entries remain pending.
func (graph *Graph[N, E]) IsPathFromTo(from, to N) bool {
	stack := []N{from}
	seen := new(graphOrdered[N, struct{}])
	seen.insertIfAbsent(from, struct{}{}, graph.options.Compare)
	for len(stack) != 0 {
		last := len(stack) - 1
		node := stack[last]
		stack = stack[:last]
		edges, exists := graph.nodes.get(node, graph.options.Compare)
		if !exists {
			break
		}
		for _, child := range edges.entries {
			if graph.equalNode(child.key, to) {
				return true
			}
			if seen.insertIfAbsent(child.key, struct{}{}, graph.options.Compare) {
				stack = append(stack, child.key)
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
			for parent := range graph.Iter() {
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
	backLinks := new(graphOrdered[N, backLink])
	queue := []N{start}
	last := start
	for head := 0; head < len(queue); head++ {
		node := queue[head]
		last = node
		outEdges := true
		for child, edge := range edges(node) {
			outEdges = false
			if backLinks.insertIfAbsent(child, backLink{node, edge}, graph.options.Compare) {
				queue = append(queue, child)
			}
		}
		if outEdges {
			break
		}
	}

	result := make([]GraphPathEntry[N, E], 0)
	next := last
	for {
		link, exists := backLinks.remove(next, graph.options.Compare)
		if !exists {
			break
		}
		result = append(result, GraphPathEntry[N, E]{next, link.edge})
		next = link.parent
	}
	allDifferent := true
	for _, entry := range result {
		if !graph.notEqualNode(entry.Node, next) {
			allDifferent = false
			break
		}
	}
	if allDifferent {
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
			contains := false
			for _, entry := range result {
				if graph.equalNode(entry.Node, child) {
					contains = true
					break
				}
			}
			if !contains {
				_ = graph.PrintForTest(os.Stderr)
				panic(GraphPathEndPanic)
			}
		}
	}
	return result
}

// Clone eagerly copies the ordered storage and applies its Clone adapters.
// It replaces im_rc's copy-on-write representation with independent Go storage.
func (graph *Graph[N, E]) Clone() *Graph[N, E] {
	result := NewGraph(graph.options)
	for _, node := range graph.nodes.entries {
		edges := new(graphOrdered[N, *graphEdge[E]])
		for _, child := range node.value.entries {
			edges.entries = append(edges.entries, graphOrderedEntry[N, *graphEdge[E]]{
				graph.cloneNode(child.key), &graphEdge[E]{value: graph.cloneEdge(child.value.value)},
			})
		}
		result.nodes.entries = append(result.nodes.entries, graphOrderedEntry[N, *graphOrdered[N, *graphEdge[E]]]{graph.cloneNode(node.key), edges})
	}
	return result
}

// Equal implements im_rc ordered-map PartialEq/Eq. Its diff compares key
// ordering and then tuple !=, dispatching node and edge Ne independently of Eq.
func (graph *Graph[N, E]) Equal(other *Graph[N, E]) bool {
	if graph.options.EqualEdge == nil && graph.options.NotEqualEdge == nil {
		panic("cargo graph: edge equality adapter is required")
	}
	if graph.Len() != other.Len() {
		return false
	}
	for index, node := range graph.nodes.entries {
		otherNode := other.nodes.entries[index]
		if graph.options.Compare(node.key, otherNode.key) != 0 || graph.notEqualNode(node.key, otherNode.key) {
			return false
		}
		if len(node.value.entries) != len(otherNode.value.entries) {
			return false
		}
		for index, child := range node.value.entries {
			otherChild := otherNode.value.entries[index]
			if graph.options.Compare(child.key, otherChild.key) != 0 || graph.notEqualNode(child.key, otherChild.key) {
				return false
			}
			different := false
			if graph.options.NotEqualEdge != nil {
				different = graph.options.NotEqualEdge(child.value.value, otherChild.value.value)
			} else {
				different = !graph.options.EqualEdge(child.value.value, otherChild.value.value)
			}
			if different {
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
	names := new(graphOrdered[N, struct{}])
	for node := range graph.Iter() {
		names.insertIfAbsent(node, struct{}{}, graph.options.Compare)
	}
	for _, node := range graph.nodes.entries {
		for _, child := range node.value.entries {
			names.insertIfAbsent(child.key, struct{}{}, graph.options.Compare)
		}
	}
	var out strings.Builder
	out.WriteString("Graph for tests = Graph {\n")
	for node := range graph.Iter() {
		index, _ := names.position(node, graph.options.Compare)
		fmt.Fprintf(&out, "  - %d\n", index)
		for child := range graph.Edges(node) {
			index, _ := names.position(child, graph.options.Compare)
			fmt.Fprintf(&out, "    - %d\n", index)
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
