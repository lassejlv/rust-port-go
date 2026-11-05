package util

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	imrc "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type graphOwnedExpected struct {
	Abort  bool     `json:"abort"`
	Result string   `json:"result"`
	Len    uint     `json:"len"`
	After  string   `json:"after"`
	Stored string   `json:"stored"`
	Alive  string   `json:"alive"`
	Trace  []string `json:"trace"`
	Stderr string   `json:"stderr"`
}

// Execute the three unchanged source test cases against the primary interface
// as well as their existing exact file-mapped value-convenience translations.
func TestGraphOwnedOriginalSourceCases(t *testing.T) {
	traits := GraphTraits[int, struct{}]{Node: imrc.Ops[int]{Clone: func(n rustvec.Ref[int]) *rustvec.Owned[int] { return rustvec.Own(*n.Value) }}, Edge: imrc.Ops[struct{}]{Clone: func(rustvec.Ref[struct{}]) *rustvec.Owned[struct{}] { return rustvec.Own(struct{}{}) }}, CompareNode: func(a, b rustvec.Ref[int]) int {
		if *a.Value < *b.Value {
			return -1
		}
		if *a.Value > *b.Value {
			return 1
		}
		return 0
	}, LessNode: func(a, b rustvec.Ref[int]) bool { return *a.Value < *b.Value }, EqualNode: func(a, b rustvec.Ref[int]) bool { return *a.Value == *b.Value }, EqualEdge: func(rustvec.Ref[struct{}], rustvec.Ref[struct{}]) bool { return true }, DefaultEdge: func(rustfmt.Context) *rustvec.Owned[struct{}] { return rustvec.Own(struct{}{}) }}
	first := NewOwnedGraph(traits)
	defer first.Close()
	for _, e := range [][2]int{{0, 3}, {1, 0}, {2, 0}, {2, 1}} {
		first.Link(rustvec.Own(e[0]), rustvec.Own(e[1]))
	}
	start := 2
	path := first.PathToBottom(&start)
	if len(path) != 3 || *path[0].Node != 2 || path[0].Node != &start || path[0].Edge != nil || *path[1].Node != 0 || *path[2].Node != 3 || path[1].Edge != first.Edge(path[0].Node, path[1].Node) || path[2].Edge != first.Edge(path[1].Node, path[2].Node) {
		t.Fatal("original path_to_case")
	}
	self := NewOwnedGraph(traits)
	defer self.Close()
	self.Link(rustvec.Own(0), rustvec.Own(0))
	zero := 0
	path = self.PathToBottom(&zero)
	if len(path) != 1 || *path[0].Node != 0 || path[0].Edge != self.Edge(&zero, &zero) {
		t.Fatal("original path_to_self")
	}
	reverse := NewOwnedGraph(traits)
	defer reverse.Close()
	reverse.Link(rustvec.Own(0), rustvec.Own(1))
	reverse.Link(rustvec.Own(0), rustvec.Own(2))
	expected := NewOwnedGraph(traits)
	defer expected.Close()
	expected.Add(rustvec.Own(0))
	expected.Link(rustvec.Own(1), rustvec.Own(0))
	expected.Link(rustvec.Own(2), rustvec.Own(0))
	actual := reverse.Reversed()
	defer actual.Close()
	if !actual.Equal(expected) {
		t.Fatal("original reverse")
	}
}

type graphOwnedOracleRecord struct {
	Request  string             `json:"request"`
	Hooks    bool               `json:"hooks"`
	Behavior string             `json:"behavior"`
	Want     graphOwnedExpected `json:"want"`
	Profiles []string           `json:"profiles"`
}
type graphOwnedOracleProfile struct {
	Name    string                   `json:"name"`
	Debug   bool                     `json:"debug_assertions"`
	Checked bool                     `json:"overflow_checks"`
	Sites   map[string]graphHookSite `json:"panic_sites"`
}
type graphOwnedOracleManifest struct {
	Rows         int                       `json:"retained_rows"`
	Observations int                       `json:"native_observations"`
	RawBytes     int                       `json:"raw_bytes"`
	RawSHA       string                    `json:"raw_sha256"`
	FixtureSHA   string                    `json:"fixture_sha256"`
	Profiles     []graphOwnedOracleProfile `json:"profiles"`
}

func TestGraphOwnedRetainedOriginalContracts(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("explicit native64-bit oracle; no32-bit runtime claim")
	}
	metadata, err := os.ReadFile("../../../../tools/port-cargo-graph/contracts/owned-validation.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest graphOwnedOracleManifest
	if err = json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	profiles := map[string]graphOwnedOracleProfile{}
	for _, profile := range manifest.Profiles {
		profiles[profile.Name] = profile
	}
	compressed, err := os.ReadFile("testdata/graph_owned.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) > 64<<20 || fmt.Sprintf("%x", sha256.Sum256(compressed)) != manifest.FixtureSHA {
		t.Fatal("owned graph fixture size/hash differs")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(io.LimitReader(z, (192<<20)+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != manifest.RawBytes || len(raw) > 192<<20 || fmt.Sprintf("%x", sha256.Sum256(raw)) != manifest.RawSHA {
		t.Fatal("owned graph raw size/hash differs")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 65536), 8<<20)
	rows, observations := 0, 0
	for scanner.Scan() {
		var record graphOwnedOracleRecord
		if err = json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		rows++
		for _, name := range record.Profiles {
			profile, ok := profiles[name]
			if !ok {
				t.Fatal("unknown native profile")
			}
			observations++
			if record.Want.Abort {
				graphOwnedAssertAbortRecord(t, record, profile)
				continue
			}
			want := fmt.Sprintf("%x\t%d\t%s\t%s\t%s\t%x", record.Want.Result, record.Want.Len, record.Want.After, record.Want.Stored, record.Want.Alive, strings.Join(record.Want.Trace, "|"))
			got := graphOwnedReplayProfile(record.Request, profile.Debug, profile.Checked, false, record.Hooks, profile.Sites, record.Behavior)
			graphOwnedAssertObservation(t, name+" "+record.Request, got.observation, want)
			if got.stderr != record.Want.Stderr {
				t.Fatalf("%s %s source stderr differs\ngot %s\nwant%s", name, record.Request, got.stderr, record.Want.Stderr)
			}
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if rows != manifest.Rows || observations != manifest.Observations {
		t.Fatalf("native coverage rows%d/%d observations%d/%d", rows, manifest.Rows, observations, manifest.Observations)
	}
}
func graphOwnedAssertAbortRecord(t *testing.T, record graphOwnedOracleRecord, profile graphOwnedOracleProfile) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	sites, err := json.Marshal(profile.Sites)
	if err != nil {
		t.Fatal(err)
	}
	boolString := func(v bool) string {
		if v {
			return "1"
		}
		return "0"
	}
	deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(deadline, binary, "-test.run=^TestGraphOwnedAbortHelper$")
	command.Env = append(os.Environ(), "FERRUGO_GRAPH_OWNED_ABORT="+record.Request, "FERRUGO_GRAPH_DEBUG="+boolString(profile.Debug), "FERRUGO_GRAPH_CHECKED="+boolString(profile.Checked), "FERRUGO_GRAPH_HOOKS="+boolString(record.Hooks), "FERRUGO_GRAPH_HOOK_BEHAVIOR="+record.Behavior, "FERRUGO_GRAPH_HOOK_SITES="+string(sites))
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err = command.Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 134 {
		t.Fatalf("%s %s expected abort134: %v\n%s\n%s", profile.Name, record.Request, err, stdout.String(), stderr.String())
	}
	if got, want := strings.ReplaceAll(strings.TrimSuffix(stdout.String(), "\n"), "\n", "|"), strings.Join(record.Want.Trace, "|"); got != want {
		t.Fatalf("%s %s before-abort trace differs\ngot %s\nwant%s", profile.Name, record.Request, got, want)
	}
	if stderr.String() != record.Want.Stderr {
		t.Fatalf("%s %s before-abort stderr differs\ngot %s\nwant%s", profile.Name, record.Request, stderr.String(), record.Want.Stderr)
	}
}

func TestGraphOwnedLiveRegeneration(t *testing.T) {
	value, requested := os.LookupEnv("FERRUGO_CARGO_GRAPH_CONTRACTS")
	if !requested {
		t.Skip("set FERRUGO_CARGO_GRAPH_CONTRACTS=1 for original whole-source contracts")
	}
	if value != "1" {
		t.Fatal("FERRUGO_CARGO_GRAPH_CONTRACTS must be1 when explicitly requested")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{"generate_owned.py", "audit.py"} {
		deadline, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		command := exec.CommandContext(deadline, python, filepath.Join(root, "tools/port-cargo-graph/contracts", script), "--check")
		command.Dir = root
		output, err := command.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("%s: %v\n%s", script, err, output)
		}
	}
}
