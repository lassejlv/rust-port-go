// SPDX-License-Identifier: MIT OR Apache-2.0
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

	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// Independent native witnesses exercise Diff's unequal tree-shape and shared
// prefix paths. All callbacks/cells retain the original non-Copy probe model.
func graphOwnedEqualityReplay(line string, debug, checked bool) string {
	s := graphOwnedProbe{request: graphOwnedParse(line), checked: checked}
	setup := graphProbeContext("setup")
	g := s.build(setup, debug)
	assign := func(h *OwnedGraph[graphProbeKey, graphProbeEdge], a, b, id int32, scope string) {
		ctx := graphProbeContext(scope)
		value := s.ownEdge(s.edge(id))
		rustvec.ConsumeValue(value, ctx, func(value *rustvec.Owned[graphProbeEdge]) struct{} {
			h.LinkWithContext(s.ownKey(s.key(a)), s.ownKey(s.key(b)), ctx).AssignWithContext(value, ctx)
			return struct{}{}
		})
	}
	if s.request.scenario == 5 {
		for i := uint64(1); i < s.request.size; i++ {
			assign(g, 0, int32(i), int32(i)+10, "setup")
		}
	}
	var h *OwnedGraph[graphProbeKey, graphProbeEdge]
	if s.request.op == 4 || s.request.op == 5 || s.request.op >= 8 {
		h = g.CloneWithContext(setup)
	} else {
		h = NewOwnedGraphWithConfig(s.traits(), g.config)
		values := make([]uint64, s.request.size)
		for i := range values {
			values[i] = uint64(i)
			if s.request.op == 0 || s.request.op == 1 {
				values[i]++
			}
		}
		if s.request.op == 2 {
			for i, j := 0, len(values)-1; i < j; i, j = i+1, j-1 {
				values[i], values[j] = values[j], values[i]
			}
		}
		if (s.request.op == 6 || s.request.op == 7) && len(values) > 0 {
			middle := len(values) / 2
			values = append(append([]uint64{}, values[middle:]...), values[:middle]...)
		}
		for _, id := range values {
			h.AddWithContext(s.ownKey(s.key(int32(id))), setup)
		}
	}
	s.trace = nil
	s.events = 0
	ctx := graphProbeContext("op")
	result := "panic"
	graphProbeCatch(func() {
		if s.request.op == 4 || s.request.op == 5 {
			n := s.request.size / 2
			if s.request.op == 4 {
				n = s.request.size - 1
			}
			assign(h, int32(n), int32(n), 99, "op")
		}
		if s.request.op >= 8 {
			n := int32(s.request.size) - 1
			id := int32(99)
			if s.request.op == 9 {
				id = n + 10
			}
			assign(h, 0, n, id, "op")
		}
		equal := false
		if s.request.op == 1 || s.request.op == 3 || s.request.op == 7 {
			equal = h.EqualWithContext(g, ctx)
		} else {
			equal = g.EqualWithContext(h, ctx)
		}
		result = fmt.Sprintf("equal:%t", equal)
	})
	length := g.Len()
	after, stored := "ok", "ok"
	afterCtx := graphProbeContext("after")
	if graphProbeCatch(func() { g.CloseWithContext(afterCtx) }) != nil {
		after = "panic"
	}
	if graphProbeCatch(func() { h.CloseWithContext(afterCtx) }) != nil {
		stored = "panic"
	}
	var alive []string
	for _, info := range s.registry {
		if info.live {
			alive = append(alive, strconv.FormatUint(uint64(info.serial), 10))
		}
	}
	return fmt.Sprintf("%x\t%d\t%s\t%s\t%s\t%x", result, length, after, stored, strings.Join(alive, ","), strings.Join(s.trace, "|"))
}

func TestGraphOwnedOriginalEqualityShapes(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("explicit native64 equality profile; no32 runtime claim")
	}
	data, err := os.ReadFile("../../../../tools/port-cargo-graph/contracts/equality-validation.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest graphOwnedOracleManifest
	if err = json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	compressed, err := os.ReadFile("testdata/graph_equality.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) > 32<<20 || fmt.Sprintf("%x", sha256.Sum256(compressed)) != manifest.FixtureSHA {
		t.Fatal("equality fixture bound/hash differs")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(io.LimitReader(z, (96<<20)+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 96<<20 || len(raw) != manifest.RawBytes || fmt.Sprintf("%x", sha256.Sum256(raw)) != manifest.RawSHA {
		t.Fatal("equality raw bound/hash differs")
	}
	profiles := map[string]graphOwnedOracleProfile{}
	for _, p := range manifest.Profiles {
		profiles[p.Name] = p
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 65536), 8<<20)
	rows, observations := 0, 0
	for scanner.Scan() {
		var r graphOwnedOracleRecord
		if err = json.Unmarshal(scanner.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		if r.Want.Abort || r.Want.Stderr != "" || r.Hooks {
			t.Fatal("unexpected equality envelope")
		}
		rows++
		want := fmt.Sprintf("%x\t%d\t%s\t%s\t%s\t%x", r.Want.Result, r.Want.Len, r.Want.After, r.Want.Stored, r.Want.Alive, strings.Join(r.Want.Trace, "|"))
		for _, name := range r.Profiles {
			p, ok := profiles[name]
			if !ok {
				t.Fatal("unknown equality profile")
			}
			observations++
			got := graphOwnedEqualityReplay(r.Request, p.Debug, p.Checked)
			graphOwnedAssertObservation(t, name+" equality "+r.Request, got, want)
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if rows != manifest.Rows || observations != manifest.Observations {
		t.Fatalf("equality coverage rows%d/%d observations%d/%d", rows, manifest.Rows, observations, manifest.Observations)
	}
}

func TestGraphOwnedEqualityLiveRegeneration(t *testing.T) {
	value, requested := os.LookupEnv("FERRUGO_CARGO_GRAPH_CONTRACTS")
	if !requested {
		t.Skip("set FERRUGO_CARGO_GRAPH_CONTRACTS=1 for unchanged-source equality proof")
	}
	if value != "1" {
		t.Fatal("FERRUGO_CARGO_GRAPH_CONTRACTS must be1")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatal(err)
	}
	deadline, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(deadline, python, filepath.Join(root, "tools/port-cargo-graph/contracts/generate_equality.py"), "--check")
	command.Dir = root
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("equality generator: %v\n%s", err, out)
	}
}
