// SPDX-License-Identifier: MIT OR Apache-2.0
// Generic Ord/Eq behavior of the complete pinned Cargo graph.rs source port.
package util

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func vectorGraphOptions(release bool) GraphOptions[[]int, int] {
	return GraphOptions[[]int, int]{
		Compare: slices.Compare[[]int], CloneNode: slices.Clone[[]int],
		EqualEdge:   func(a, b int) bool { return a == b },
		DisplayNode: vectorGraphKey, DisableDebugAssertions: release,
	}
}
func vectorGraphKey(value []int) string { return "[" + graphList(value) + "]" }

type graphRecordKey struct {
	Parts []int
	Tag   int
}

func recordGraphKey(value graphRecordKey) string {
	return vectorGraphKey(value.Parts) + "#" + strconv.Itoa(value.Tag)
}
func recordGraphOptions(release bool) GraphOptions[graphRecordKey, int] {
	return GraphOptions[graphRecordKey, int]{
		Compare: func(a, b graphRecordKey) int {
			if compared := slices.Compare(a.Parts, b.Parts); compared != 0 {
				return compared
			}
			return cmp.Compare(a.Tag, b.Tag)
		},
		CloneNode: func(value graphRecordKey) graphRecordKey { return graphRecordKey{slices.Clone(value.Parts), value.Tag} },
		EqualEdge: func(a, b int) bool { return a == b }, DisplayNode: recordGraphKey, DisableDebugAssertions: release,
	}
}

func TestGraphNonComparableNodeKeys(t *testing.T) {
	graph := NewGraph(vectorGraphOptions(false))
	graph.Add(nil)
	graph.Add([]int{}) // Independent empty vectors identify the same Rust key.
	if graph.Len() != 1 || !graph.Contains([]int{}) {
		t.Fatal("nil/allocated empty slice keys differ")
	}
	first := []int{1}
	graph.Add(first)
	graph.Add([]int{1})
	pointer := graph.Link([]int{}, []int{1})
	*pointer = 41
	if graph.Len() != 2 || graph.Link(nil, []int{1}) != pointer || *graph.Edge([]int{}, []int{1}) != 41 {
		t.Fatal("equal-by-value slice keys did not reuse original edge")
	}
	for value := 2; value < 100; value++ {
		graph.Add([]int{value})
		graph.Link([]int{}, []int{value})
	}
	if graph.Edge(nil, []int{1}) != pointer {
		t.Fatal("ordered-entry movement changed edge identity")
	}
	if !graph.ContainsBy(func(node []int) int { return slices.Compare(node, []int{1}) }) {
		t.Fatal("slice borrowed query missing")
	}
	if !graph.IsPathFromTo([]int{}, []int{1}) || graph.IsPathFromTo([]int{1}, []int{}) {
		t.Fatal("slice reachability mismatch")
	}
	path := graph.PathToBottom([]int{})
	if len(path) != 2 || len(path[0].Node) != 0 || !slices.Equal(path[1].Node, []int{1}) || path[1].Edge != pointer {
		t.Fatal("slice shortest path mismatch")
	}
	reversed := graph.Reversed()
	if reversed.Edge([]int{1}, nil) == nil || *reversed.Edge([]int{1}, []int{}) != 41 {
		t.Fatal("slice reverse lost keys or values")
	}
	clone := graph.Clone()
	if !graph.Equal(clone) {
		t.Fatal("slice clone differs")
	}
	*clone.Link([]int{}, []int{1}) = 52
	if graph.Equal(clone) || *pointer != 41 {
		t.Fatal("slice clone edge is shared")
	}
	sourceKeys, cloneKeys := slices.Collect(graph.Iter()), slices.Collect(clone.Iter())
	if &sourceKeys[1][0] == &cloneKeys[1][0] {
		t.Fatal("CloneNode did not clone slice storage")
	}
	sorted := graph.Sort()
	if len(sorted[0]) == 0 {
		t.Fatal("expected a leaf first")
	}
	sorted[0][0] = -77
	if !graph.Contains([]int{1}) {
		t.Fatal("Sort returned an uncloned slice key")
	}

	records := NewGraph(recordGraphOptions(false))
	records.Add(graphRecordKey{[]int{1}, 7})
	records.Add(graphRecordKey{[]int{1}, 7})
	records.Add(graphRecordKey{[]int{1}, 8})
	*records.Link(graphRecordKey{[]int{1}, 7}, graphRecordKey{[]int{1}, 8}) = 99
	if records.Len() != 2 || !records.Contains(graphRecordKey{[]int{1}, 7}) || !records.IsPathFromTo(graphRecordKey{[]int{1}, 7}, graphRecordKey{[]int{1}, 8}) {
		t.Fatal("slice-bearing record keys did not use their complete ordering")
	}
	if !records.Equal(records.Clone()) || *records.Reversed().Edge(graphRecordKey{[]int{1}, 8}, graphRecordKey{[]int{1}, 7}) != 99 {
		t.Fatal("record clone/reverse mismatch")
	}
}

func TestGraphNodeNeDefaultsToEq(t *testing.T) {
	var compared [][2]int
	options := vectorGraphOptions(false)
	options.EqualNode = func(left, right []int) bool {
		compared = append(compared, [2]int{left[0], right[0]})
		return slices.Equal(left, right)
	}
	first, second := NewGraph(options), NewGraph(options)
	*first.Link([]int{0}, []int{1}) = 7
	*second.Link([]int{0}, []int{1}) = 7
	path := first.PathToBottom([]int{0})
	if len(path) != 2 || !slices.Equal(compared, [][2]int{{1, 0}}) {
		t.Fatalf("default Ne did not dispatch overridden Eq: %v", compared)
	}
	compared = nil
	if !first.Equal(second) || !slices.Equal(compared, [][2]int{{0, 0}, {1, 1}}) {
		t.Fatalf("map tuple Ne did not dispatch default Eq: %v", compared)
	}
	// An interface containing a slice must never reach native Go equality or hashing.
	boxed := NewGraph(GraphOptions[any, int]{Compare: func(a, b any) int { return slices.Compare(a.([]int), b.([]int)) }})
	boxed.Add([]int{1})
	boxed.Add([]int{1})
	if boxed.Len() != 1 || !boxed.Contains([]int{1}) {
		t.Fatal("dynamic slice key did not use ordering")
	}
}

func graphGenericList[N any](nodes []N, key func(N) string) string {
	values := make([]string, len(nodes))
	for index, node := range nodes {
		values[index] = key(node)
	}
	return strings.Join(values, "|")
}
func graphGenericEdges[N any](graph *Graph[N, int], key func(N) string) string {
	var values []string
	for from := range graph.Iter() {
		for to, edge := range graph.Edges(from) {
			values = append(values, fmt.Sprintf("%s>%s:%d", key(from), key(to), *edge))
		}
	}
	return strings.Join(values, "|")
}
func graphGenericPath[N any](path []GraphPathEntry[N, int], key func(N) string) string {
	values := make([]string, len(path))
	for index, entry := range path {
		edge := "none"
		if entry.Edge != nil {
			edge = strconv.Itoa(*entry.Edge)
		}
		values[index] = key(entry.Node) + ":" + edge
	}
	return strings.Join(values, "|")
}
func graphGenericCase[N any](out *strings.Builder, printer io.Writer, keys []N, options GraphOptions[N, int], key func(N) string) *Graph[N, int] {
	graph := NewGraph(options)
	graph.Add(graph.cloneNode(keys[4]))
	graph.Add(graph.cloneNode(keys[0]))
	graph.Add(graph.cloneNode(keys[1]))
	for _, link := range [][3]int{{0, 1, 10}, {1, 2, 20}, {1, 3, 21}, {3, 4, 30}} {
		*graph.Link(graph.cloneNode(keys[link[0]]), graph.cloneNode(keys[link[1]])) = link[2]
	}
	before := graph.Edge(keys[1], keys[2])
	repeated := graph.Link(graph.cloneNode(keys[1]), graph.cloneNode(keys[2]))
	fmt.Fprintf(out, "len %d\niter %s\nedges %s\n", graph.Len(), graphGenericList(slices.Collect(graph.Iter()), key), graphGenericEdges(graph, key))
	fmt.Fprintf(out, "repeat_identity %t\nchild_only %t\n", before == repeated, graph.Contains(keys[2]))
	fmt.Fprintf(out, "sort_initial %s\n", graphCatch(func() string { return graphGenericList(graph.Sort(), key) }))
	reversed := graph.Reversed()
	fmt.Fprintf(out, "reversed_iter %s\nreversed_edges %s\n", graphGenericList(slices.Collect(reversed.Iter()), key), graphGenericEdges(reversed, key))
	graph.Add(graph.cloneNode(keys[2]))
	graph.Add(graph.cloneNode(keys[1]))
	fmt.Fprintf(out, "sort_complete %s\n", graphCatch(func() string { return graphGenericList(graph.Sort(), key) }))
	bottom := graphCatch(func() string { return graphGenericPath(graph.PathToBottom(keys[0]), key) })
	top := graphCatch(func() string { return graphGenericPath(graph.PathToTop(keys[4]), key) })
	fmt.Fprintf(out, "bottom %s\ntop %s\n", bottom, top)
	fmt.Fprintf(out, "reachable %t %t\n", graph.IsPathFromTo(keys[0], keys[4]), graph.IsPathFromTo(keys[4], keys[0]))
	copy := graph.Clone()
	fmt.Fprintf(out, "clone_equal %t\n", graph.Equal(copy))
	*copy.Link(graph.cloneNode(keys[1]), graph.cloneNode(keys[2])) = 99
	fmt.Fprintf(out, "changed_equal %t\noriginal_edge %d\ncloned_edge %d\n", graph.Equal(copy), *graph.Edge(keys[1], keys[2]), *copy.Edge(keys[1], keys[2]))
	if !options.DisableDebugAssertions {
		_ = graph.PrintForTest(printer)
	}
	return graph
}

func graphOperatorProbe(out *strings.Builder, release bool) {
	var events []string
	panicOperator := ""
	event := func(kind string, a, b int) {
		events = append(events, fmt.Sprintf("%s:%d:%d", kind, a, b))
		if panicOperator == kind {
			panic("operator_" + kind)
		}
	}
	options := intGraphOptions(release)
	options.EqualNode = func(a, b int) bool { event("eq", a, b); return a == b }
	options.NotEqualNode = func(a, b int) bool { event("ne", a, b); return a != b }
	makeGraph := func() *Graph[int, int] {
		g := NewGraph(options)
		*g.Link(0, 1) = 7
		*g.Link(0, 2) = 8
		g.Link(1, 1)
		g.Link(2, 2)
		return g
	}
	graph := makeGraph()
	events = nil
	fmt.Fprintf(out, "reach %t\n", graph.IsPathFromTo(0, 2))
	fmt.Fprintf(out, "reach_trace %s\n", strings.Join(events, ","))
	events = nil
	fmt.Fprintf(out, "bottom %s\n", graphPathString(graph.PathToBottom(0)))
	fmt.Fprintf(out, "bottom_trace %s\n", strings.Join(events, ","))
	events = nil
	other := makeGraph()
	events = nil
	fmt.Fprintf(out, "equal %t\n", graph.Equal(other))
	fmt.Fprintf(out, "equal_trace %s\n", strings.Join(events, ","))
	type operatorEdge struct{ value int }
	edgeOptions := GraphOptions[int, operatorEdge]{Compare: cmp.Compare[int], EqualNode: options.EqualNode, NotEqualNode: options.NotEqualNode,
		EqualEdge:    func(a, b operatorEdge) bool { event("edge_eq", a.value, b.value); return a.value == b.value },
		NotEqualEdge: func(a, b operatorEdge) bool { event("edge_ne", a.value, b.value); return a.value != b.value }}
	first, second := NewGraph(edgeOptions), NewGraph(edgeOptions)
	*first.Link(0, 1) = operatorEdge{7}
	*second.Link(0, 1) = operatorEdge{7}
	events = nil
	fmt.Fprintf(out, "edge_equal %t\n", first.Equal(second))
	fmt.Fprintf(out, "edge_equal_trace %s\n", strings.Join(events, ","))
	events = nil
	failed := func(operation func()) (panicked bool) {
		defer func() { panicked = recover() != nil }()
		operation()
		return false
	}
	panicOperator = "eq"
	panicked := failed(func() { graph.IsPathFromTo(0, 2) })
	panicOperator = ""
	fmt.Fprintf(out, "eq_panic %t %s\n", panicked, strings.Join(events, ","))
	events = nil
	panicOperator = "ne"
	panicked = failed(func() { graph.PathToBottom(0) })
	panicOperator = ""
	fmt.Fprintf(out, "ne_panic %t %s\n", panicked, strings.Join(events, ","))
	events = nil
	panicOperator = "edge_eq"
	noEqCall := first.Equal(second)
	panicOperator = ""
	fmt.Fprintf(out, "edge_eq_bypassed %t %s\n", noEqCall, strings.Join(events, ","))
	events = nil
	panicOperator = "edge_ne"
	panicked = failed(func() { first.Equal(second) })
	panicOperator = ""
	fmt.Fprintf(out, "edge_ne_panic %t %s\n", panicked, strings.Join(events, ","))
}

func graphGenericGoOracle(t *testing.T, release bool) (string, string) {
	t.Helper()
	printer, err := os.CreateTemp(t.TempDir(), "generic-graph-stderr-")
	if err != nil {
		t.Fatal(err)
	}
	defer printer.Close()
	originalStderr := os.Stderr
	os.Stderr = printer
	defer func() { os.Stderr = originalStderr }()
	var out strings.Builder
	vectors := [][]int{{}, {1}, {1, 0}, {2, 3}, {4}}
	out.WriteString("vec_keys\n")
	graph := graphGenericCase(&out, printer, vectors, vectorGraphOptions(release), vectorGraphKey)
	fmt.Fprintf(&out, "borrowed %t %t\n", graph.ContainsBy(func(node []int) int { return slices.Compare(node, []int{1}) }), graph.ContainsBy(func(node []int) int { return slices.Compare(node, []int{9}) }))
	records := make([]graphRecordKey, len(vectors))
	for index, parts := range vectors {
		records[index] = graphRecordKey{parts, index}
	}
	out.WriteString("record_keys\n")
	recordGraph := graphGenericCase(&out, printer, records, recordGraphOptions(release), recordGraphKey)
	fmt.Fprintf(&out, "record_debug\n%s\n", recordGraph.Debug())
	recordGraph.Add(graphRecordKey{[]int{}, 9})
	fmt.Fprintf(&out, "record_field_identity %d\n", recordGraph.Len())
	out.WriteString("operators\n")
	graphOperatorProbe(&out, release)
	if _, err := printer.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	errors, err := io.ReadAll(printer)
	if err != nil {
		t.Fatal(err)
	}
	return out.String(), string(errors)
}

func TestGraphGenericPinnedOracle(t *testing.T) {
	debug, release := os.Getenv("FERRUGO_GRAPH_ORACLE_DEBUG"), os.Getenv("FERRUGO_GRAPH_ORACLE_RELEASE")
	write := os.Getenv("FERRUGO_GRAPH_WRITE") == "1"
	if (debug == "") != (release == "") {
		t.Fatal("both graph oracle profiles are required")
	}
	if write && debug == "" {
		t.Fatal("cannot write generic graph evidence without the Rust source oracle")
	}
	type result struct{ profile, output, errors string }
	var results []result
	for index, path := range []string{debug, release} {
		profile := "debug"
		if index == 1 {
			profile = "release"
		}
		actual, errors := graphGenericGoOracle(t, index == 1)
		var expected, expectedErrors []byte
		if path != "" {
			cmd := exec.Command(path, "generic")
			var out, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &out, &stderr
			if err := cmd.Run(); err != nil {
				t.Fatalf("generic original-source graph oracle: %v\n%s", err, stderr.String())
			}
			expected, expectedErrors = out.Bytes(), stderr.Bytes()
		}
		for stream, reference := range map[string][]byte{"stdout": expected, "stderr": expectedErrors} {
			path := filepath.Join("testdata", "graph_generic_"+profile+"."+stream)
			if !write {
				retained, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if debug != "" && !bytes.Equal(retained, reference) {
					t.Fatalf("retained generic %s %s differs from pinned source", profile, stream)
				}
				if stream == "stdout" {
					expected = retained
				} else {
					expectedErrors = retained
				}
			}
		}
		graphCompareOutput(t, "generic "+profile+" stdout", actual, string(expected))
		graphCompareOutput(t, "generic "+profile+" stderr", errors, string(expectedErrors))
		results = append(results, result{profile, string(expected), string(expectedErrors)})
	}
	if write {
		for _, result := range results {
			for stream, value := range map[string]string{"stdout": result.output, "stderr": result.errors} {
				if err := os.WriteFile(filepath.Join("testdata", "graph_generic_"+result.profile+"."+stream), []byte(value), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	t.Log("matched vector/record keys and direct Eq/Ne operator traces in both profiles")
}
