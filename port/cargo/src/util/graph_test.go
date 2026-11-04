// SPDX-License-Identifier: MIT OR Apache-2.0
// Tests for the full Cargo src/util/graph.rs port at
// 24de4721844c7a4d839072bab6fb66d41c9bc1f4. The three upstream tests are
// preserved first; retained oracle cases execute the unchanged source.
package util

import (
	"bytes"
	"cmp"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func unitGraphOptions() GraphOptions[int, struct{}] {
	return GraphOptions[int, struct{}]{Compare: cmp.Compare[int], EqualEdge: func(_, _ struct{}) bool { return true }}
}

// Upstream: path_to_case, lines 202-213.
func TestGraphUpstreamPathToCase(t *testing.T) {
	graph := NewGraph(unitGraphOptions())
	graph.Link(0, 3)
	graph.Link(1, 0)
	graph.Link(2, 0)
	graph.Link(2, 1)
	path := graph.PathToBottom(2)
	if len(path) != 3 || path[0].Node != 2 || path[0].Edge != nil || path[1].Node != 0 || path[1].Edge == nil || path[2].Node != 3 || path[2].Edge == nil {
		t.Fatalf("path_to_bottom(2) = %#v, want [(2,None),(0,Some(())),(3,Some(()))]", path)
	}
}

// Upstream: path_to_self, lines 215-221 (Cargo issue #12941).
func TestGraphUpstreamPathToSelf(t *testing.T) {
	graph := NewGraph(unitGraphOptions())
	graph.Link(0, 0)
	path := graph.PathToBottom(0)
	if len(path) != 1 || path[0].Node != 0 || path[0].Edge == nil {
		t.Fatalf("path_to_bottom(0) = %#v, want [(0,Some(()))]", path)
	}
}

// Upstream: reverse, lines 223-234.
func TestGraphUpstreamReverse(t *testing.T) {
	graph := NewGraph(unitGraphOptions())
	graph.Link(0, 1)
	graph.Link(0, 2)
	expected := NewGraph(unitGraphOptions())
	expected.Add(0)
	expected.Link(1, 0)
	expected.Link(2, 0)
	if !graph.Reversed().Equal(expected) {
		t.Fatalf("reversed graph = %s, want %s", graph.Reversed(), expected)
	}
}

func intGraphOptions(release bool) GraphOptions[int, int] {
	return GraphOptions[int, int]{Compare: cmp.Compare[int], EqualEdge: func(a, b int) bool { return a == b }, DisableDebugAssertions: release}
}

func TestGraphDefaultEdgesOrderAndBorrowedLookup(t *testing.T) {
	calls := 0
	options := intGraphOptions(false)
	options.EdgeDefault = func() int { calls++; return 17 }
	graph := NewGraph(options)
	if graph.Len() != 0 || len(graph.Sort()) != 0 || graph.Debug() != "Graph {\n}" {
		t.Fatal("new/default graph is not empty")
	}
	edge := graph.Link(8, 3)
	if *edge != 17 || graph.Contains(3) || !graph.Contains(8) || graph.Len() != 1 || graph.Edge(0, 3) != nil {
		t.Fatalf("Link must add only its source and default its edge: %s", graph)
	}
	*edge = 29
	graph.Add(8)
	if repeated := graph.Link(8, 3); repeated != edge || *repeated != 29 || calls != 1 {
		t.Fatal("Add/Link replaced an existing adjacency or edge")
	}
	clone := graph.Clone()
	if calls != 1 || *clone.Edge(8, 3) != 29 {
		t.Fatal("Clone must not call Default")
	}
	units := NewGraph(unitGraphOptions())
	if units.Link(0, 1) == units.Link(0, 2) {
		t.Fatal("distinct unit edges lost pointer identity")
	}
	graph.Add(3)
	graph.Add(-2)
	graph.Link(8, -2)
	if got := slices.Collect(graph.Iter()); !slices.Equal(got, []int{-2, 3, 8}) {
		t.Fatalf("Iter order = %v", got)
	}
	var children []int
	for child := range graph.Edges(8) {
		children = append(children, child)
	}
	missingEdges := 0
	for range graph.Edges(-999) {
		missingEdges++
	}
	if !slices.Equal(children, []int{-2, 3}) || missingEdges != 0 {
		t.Fatalf("Edges order = %v", children)
	}
	// Rust's Borrow<Q> allows, for example, String keys and str lookups.
	stringsGraph := NewGraph(GraphOptions[string, int]{Compare: strings.Compare})
	stringsGraph.Add("alpha")
	stringsGraph.Add("zeta")
	if !stringsGraph.ContainsBy(func(node string) int { return strings.Compare(node, "zeta") }) || stringsGraph.ContainsBy(func(node string) int { return strings.Compare(node, "missing") }) {
		t.Fatal("borrowed lookup mismatch")
	}
	// Iterator short-circuiting must honor yield(false).
	count := 0
	for range graph.Iter() {
		count++
		break
	}
	for range graph.Edges(8) {
		count++
		break
	}
	if count != 2 {
		t.Fatalf("iterator early stops = %d", count)
	}
}

func TestGraphMissingNodesCyclesAndOrdering(t *testing.T) {
	graph := NewGraph(intGraphOptions(false))
	graph.Link(0, 1)
	graph.Link(0, 9) // Missing 9 is popped before 1, terminating search early.
	graph.Link(1, 2)
	graph.Add(2)
	if graph.IsPathFromTo(0, 2) || !graph.IsPathFromTo(0, 9) || graph.IsPathFromTo(2, 2) {
		t.Fatal("IsPathFromTo did not preserve edge-only/while-let semantics")
	}
	if got := graphCatch(func() string { graph.Sort(); return "" }); got != "panic missing_node" {
		t.Fatalf("missing child sort = %q", got)
	}
	graph.Add(9)
	if !graph.IsPathFromTo(0, 2) || !slices.Equal(graph.Sort(), []int{2, 1, 9, 0}) {
		t.Fatalf("completed graph traversal = %v", graph.Sort())
	}
	cycle := NewGraph(intGraphOptions(false))
	cycle.Link(0, 1)
	cycle.Link(1, 0)
	if !slices.Equal(cycle.Sort(), []int{1, 0}) || !cycle.IsPathFromTo(0, 0) {
		t.Fatal("marked DFS/cycle reachability mismatch")
	}
	if got := graphCatch(func() string { return graphPathString(cycle.PathToBottom(0)) }); got != "panic path_edge" {
		t.Fatalf("reciprocal-edge debug assertion = %q", got)
	}
	release := cycle.Clone()
	release.options.DisableDebugAssertions = true
	if got := graphPathString(release.PathToBottom(0)); got != "1:0,0:0" {
		t.Fatalf("release cyclic backlink-removal path = %s", got)
	}
	// A missing starting node is itself a path endpoint.
	if got := graphPathString(graph.PathToBottom(-1)); got != "-1:none" {
		t.Fatalf("absent-node path = %s", got)
	}
}

func TestGraphCloneEqualityReverseAndDisplayAdapters(t *testing.T) {
	type payload struct{ Values []int }
	options := GraphOptions[string, payload]{
		Compare:     func(a, b string) int { return strings.Compare(b, a) },
		EdgeDefault: func() payload { return payload{[]int{7}} },
		CloneEdge:   func(value payload) payload { return payload{slices.Clone(value.Values)} },
		EqualEdge:   func(a, b payload) bool { return slices.Equal(a.Values, b.Values) },
		DisplayNode: func(node string) string { return "<" + node + ">" },
	}
	graph := NewGraph(options)
	graph.Add("z")
	graph.Link("b", "a")
	copy := graph.Clone()
	if !graph.Equal(copy) {
		t.Fatal("clone differs")
	}
	copy.Edge("b", "a").Values[0] = 99
	copy.Add("new")
	if graph.Equal(copy) || graph.Edge("b", "a").Values[0] != 7 || graph.Contains("new") {
		t.Fatal("clone mutation affected original graph")
	}
	reversed := graph.Reversed()
	if !reversed.Contains("a") || !reversed.Contains("b") || !reversed.Contains("z") || reversed.Edge("a", "b").Values[0] != 7 {
		t.Fatalf("reverse lost isolated/source/child nodes or edge values: %s", reversed)
	}
	reversed.Edge("a", "b").Values[0] = 42
	if graph.Edge("b", "a").Values[0] != 7 {
		t.Fatal("Reversed did not Clone E")
	}
	want := "Graph {\n  - <z>\n  - <b>\n    - <a>\n}"
	if graph.Debug() != want || fmt.Sprintf("%#v", graph) != want || graph.String() != want {
		t.Fatalf("Debug/Display/order = %q", graph.Debug())
	}
	var printer bytes.Buffer
	if err := graph.PrintForTest(&printer); err != nil {
		t.Fatal(err)
	}
	if got, want := printer.String(), "Graph for tests = Graph {\n  - 0\n  - 1\n    - 2\n}\n"; got != want {
		t.Fatalf("normalized printer = %q, want %q", got, want)
	}
	if copy.Equal(graph) {
		t.Fatal("asymmetric equality mismatch")
	}
}

type graphOracleCase struct {
	Nodes []int    `json:"nodes"`
	Links [][3]int `json:"links"`
	Max   int      `json:"query_max"`
}
type graphOracleFixture struct {
	Version       int    `json:"format"`
	InputSHA256   string `json:"input_sha256"`
	Cases         int    `json:"cases"`
	DebugOutput   string `json:"debug_stdout"`
	DebugError    string `json:"debug_stderr"`
	ReleaseOutput string `json:"release_stdout"`
	ReleaseError  string `json:"release_stderr"`
}

// graphOracleCases is deliberately independent of the implementation. It
// exhausts all directed three-node graphs, with and without explicit node
// entries, and adds deterministic larger graphs, duplicate edges and weights.
func graphOracleCases() []graphOracleCase {
	cases := []graphOracleCase{
		{Max: 0},
		{Nodes: []int{4, 1, -1}, Max: 4},
		{Links: [][3]int{{0, 3, 13}, {1, 0, 10}, {2, 0, 20}, {2, 1, 21}}, Max: 4},
		{Links: [][3]int{{0, 0, 7}}, Max: 1},
		{Links: [][3]int{{0, 1, 1}, {1, 0, 2}}, Max: 2},
		{Nodes: []int{2}, Links: [][3]int{{0, 1, 1}, {0, 9, 9}, {1, 2, 2}}, Max: 3},
		{Links: [][3]int{{0, 1, 4}, {0, 1, 11}, {1, 2, -8}, {2, 3, 0}}, Max: 4},
		{Links: [][3]int{{0, 2, 1}, {1, 0, 2}, {2, 1, 3}, {1, 3, 4}, {3, 3, 5}}, Max: 4},
	}
	for mask := 0; mask < 1<<9; mask++ {
		for _, explicit := range []bool{false, true} {
			item := graphOracleCase{Max: 3}
			if explicit {
				item.Nodes = []int{2, 0, 1}
			}
			for bit := 0; bit < 9; bit++ {
				if mask&(1<<bit) != 0 {
					item.Links = append(item.Links, [3]int{bit / 3, bit % 3, bit*11 - 39})
				}
			}
			cases = append(cases, item)
		}
	}
	seed := uint64(0x73f9ba21d8c645e1)
	next := func() uint64 { seed ^= seed << 13; seed ^= seed >> 7; seed ^= seed << 17; return seed }
	for range 384 {
		count := int(next()%8) + 1
		item := graphOracleCase{Max: count}
		for node := 0; node < count; node++ {
			if next()%2 != 0 {
				item.Nodes = append(item.Nodes, node)
			}
		}
		links := int(next() % uint64(count*count+1))
		for range links {
			item.Links = append(item.Links, [3]int{int(next() % uint64(count)), int(next() % uint64(count+1)), int(next()%201) - 100})
		}
		cases = append(cases, item)
	}
	return cases
}
func graphOracleInput(cases []graphOracleCase) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "CARGO_GRAPH_V1 %d\n", len(cases))
	for _, item := range cases {
		fmt.Fprintf(&out, "%d %d", item.Max, len(item.Nodes))
		for _, node := range item.Nodes {
			fmt.Fprintf(&out, " %d", node)
		}
		fmt.Fprintf(&out, " %d", len(item.Links))
		for _, link := range item.Links {
			fmt.Fprintf(&out, " %d %d %d", link[0], link[1], link[2])
		}
		out.WriteByte('\n')
	}
	return []byte(out.String())
}
func graphList(nodes []int) string {
	values := make([]string, len(nodes))
	for index, node := range nodes {
		values[index] = strconv.Itoa(node)
	}
	return strings.Join(values, ",")
}
func graphEdgesString(graph *Graph[int, int]) string {
	var values []string
	for node := range graph.Iter() {
		for child, edge := range graph.Edges(node) {
			values = append(values, fmt.Sprintf("%d>%d:%d", node, child, *edge))
		}
	}
	return strings.Join(values, ",")
}
func graphPathString(path []GraphPathEntry[int, int]) string {
	values := make([]string, len(path))
	for index, entry := range path {
		edge := "none"
		if entry.Edge != nil {
			edge = strconv.Itoa(*entry.Edge)
		}
		values[index] = fmt.Sprintf("%d:%s", entry.Node, edge)
	}
	return strings.Join(values, ",")
}
func graphCatch(operation func() string) (result string) {
	defer func() {
		if failure := recover(); failure != nil {
			kind := "other"
			switch failure {
			case GraphMissingNodePanic:
				kind = "missing_node"
			case GraphPathEdgePanic:
				kind = "path_edge"
			case GraphPathEndPanic:
				kind = "path_end"
			}
			result = "panic " + kind
		}
	}()
	return "ok " + operation()
}
func graphGoOracle(t *testing.T, cases []graphOracleCase, release bool) (string, string) {
	t.Helper()
	stderr, err := os.CreateTemp(t.TempDir(), "graph-stderr-")
	if err != nil {
		t.Fatal(err)
	}
	originalStderr := os.Stderr
	os.Stderr = stderr
	defer func() { os.Stderr = originalStderr; stderr.Close() }()
	var out strings.Builder
	for index, item := range cases {
		graph := NewGraph(intGraphOptions(release))
		for _, node := range item.Nodes {
			graph.Add(node)
		}
		for _, link := range item.Links {
			*graph.Link(link[0], link[1]) = link[2]
		}
		fmt.Fprintf(&out, "case %d\nlen %d\nnodes %s\nedges %s\n", index, graph.Len(), graphList(slices.Collect(graph.Iter())), graphEdgesString(graph))
		sorted := graphCatch(func() string { return graphList(graph.Sort()) })
		fmt.Fprintf(&out, "sort %s\ndebug\n%s\n", sorted, graph.Debug())
		if !release {
			if err := graph.PrintForTest(stderr); err != nil {
				t.Fatal(err)
			}
		}
		for from := -1; from <= item.Max; from++ {
			fmt.Fprintf(&out, "contains %d %t\n", from, graph.Contains(from))
			for to := -1; to <= item.Max; to++ {
				fmt.Fprintf(&out, "reachable %d>%d %t\n", from, to, graph.IsPathFromTo(from, to))
				edge := "none"
				if value := graph.Edge(from, to); value != nil {
					edge = strconv.Itoa(*value)
				}
				fmt.Fprintf(&out, "edge %d>%d %s\n", from, to, edge)
			}
			bottom := graphCatch(func() string { return graphPathString(graph.PathToBottom(from)) })
			top := graphCatch(func() string { return graphPathString(graph.PathToTop(from)) })
			fmt.Fprintf(&out, "bottom %d %s\ntop %d %s\n", from, bottom, from, top)
		}
		reversed := graph.Reversed()
		fmt.Fprintf(&out, "reversed\n%s\nreversed_edges %s\n", reversed.Debug(), graphEdgesString(reversed))
		cloned := graph.Clone()
		fmt.Fprintf(&out, "clone_equal %t\n", graph.Equal(cloned))
		first := true
		for node := range graph.Iter() {
			for child := range graph.Edges(node) {
				if first {
					*cloned.Link(node, child) += 7
					first = false
				}
			}
		}
		cloned.Add(-1000)
		*cloned.Link(-1000, -999) = 999
		fmt.Fprintf(&out, "changed_equal %t\noriginal_edges %s\nclone_nodes %s\nclone_edges %s\n", graph.Equal(cloned), graphEdgesString(graph), graphList(slices.Collect(cloned.Iter())), graphEdgesString(cloned))
	}
	if _, err := stderr.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	errors, err := io.ReadAll(stderr)
	if err != nil {
		t.Fatal(err)
	}
	return out.String(), string(errors)
}
func graphCompareOutput(t *testing.T, label, actual, expected string) {
	t.Helper()
	if actual == expected {
		return
	}
	position := 0
	for position < min(len(actual), len(expected)) && actual[position] == expected[position] {
		position++
	}
	start := max(0, position-120)
	t.Fatalf("%s differs at byte %d:\nGo %q\nRust %q", label, position, actual[start:min(len(actual), position+240)], expected[start:min(len(expected), position+240)])
}

// TestGraphPinnedOracle always replays retained pinned-source evidence. The
// regeneration tool additionally supplies debug/release original-source native
// binaries; Rust/Cargo are never dependencies of normal Go tests or production.
func TestGraphPinnedOracle(t *testing.T) {
	cases := graphOracleCases()
	input := graphOracleInput(cases)
	hash := sha256.Sum256(input)
	fixture := graphOracleFixture{Version: 1, Cases: len(cases), InputSHA256: hex.EncodeToString(hash[:])}
	debug, release := os.Getenv("FERRUGO_GRAPH_ORACLE_DEBUG"), os.Getenv("FERRUGO_GRAPH_ORACLE_RELEASE")
	if (debug == "") != (release == "") {
		t.Fatal("both debug and release graph oracles are required")
	}
	if debug != "" {
		for index, path := range []string{debug, release} {
			cmd := exec.Command(path)
			cmd.Stdin = bytes.NewReader(input)
			var output, errors bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &errors
			if err := cmd.Run(); err != nil {
				t.Fatalf("original-source graph oracle: %v\n%s", err, errors.String())
			}
			if index == 0 {
				fixture.DebugOutput, fixture.DebugError = output.String(), errors.String()
			} else {
				fixture.ReleaseOutput, fixture.ReleaseError = output.String(), errors.String()
			}
		}
	}
	path := filepath.Join("testdata", "graph_cases.json.gz")
	writeFixture := os.Getenv("FERRUGO_GRAPH_WRITE") == "1"
	if writeFixture && debug == "" {
		t.Fatal("cannot generate fixture without original Rust oracle")
	}
	if !writeFixture {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer file.Close()
		reader, err := gzip.NewReader(file)
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		var retained graphOracleFixture
		if err := json.NewDecoder(reader).Decode(&retained); err != nil {
			t.Fatal(err)
		}
		if retained.Version != fixture.Version || retained.Cases != fixture.Cases || retained.InputSHA256 != fixture.InputSHA256 {
			t.Fatal("retained corpus provenance differs")
		}
		if debug != "" && !reflect.DeepEqual(fixture, retained) {
			t.Fatal("retained graph oracle results differ from pinned source")
		}
		fixture = retained
	}
	actual, errors := graphGoOracle(t, cases, false)
	graphCompareOutput(t, "debug stdout", actual, fixture.DebugOutput)
	graphCompareOutput(t, "debug stderr/test printer", errors, fixture.DebugError)
	actual, errors = graphGoOracle(t, cases, true)
	graphCompareOutput(t, "release stdout", actual, fixture.ReleaseOutput)
	graphCompareOutput(t, "release stderr", errors, fixture.ReleaseError)
	if writeFixture {
		if debug == "" {
			t.Fatal("cannot generate fixture without original Rust oracle")
		}
		data, err := json.Marshal(fixture)
		if err != nil {
			t.Fatal(err)
		}
		var compressed bytes.Buffer
		writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(data); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, compressed.Bytes(), 0644); err != nil {
			t.Fatal(err)
		}

	}
	t.Logf("matched %d original-source graph cases in debug and release modes", len(cases))
}
