// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete port of compiler/rustc_index/src/interval/tests.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. See licenses in port/rust.
package rustcindex

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	oracle "github.com/lassejlv/rust-port-go/tools/port-rust-interval/validation"
)

func newU32(domain uint) *index.IntervalSet[uint32] {
	return index.NewIntervalSet(index.U32Idx(), domain, index.U32IntervalStep())
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root missing")
		}
		dir = parent
	}
}
func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func TestOriginalSourceOracle(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("committed original-source oracle pins a 64-bit usize profile")
	}
	dir := filepath.Join(repositoryRoot(t), "tools/port-rust-interval")
	var manifest struct {
		Format        int    `json:"format"`
		Revision      string `json:"rust_revision"`
		Records       int    `json:"records"`
		OriginalTests int    `json:"original_tests"`
		FixtureSHA256 string `json:"fixture_sha256"`
		RawSHA256     string `json:"raw_fixture_sha256"`
		OracleSHA256  string `json:"oracle_sha256"`
		Sources       []struct {
			Path   string `json:"path"`
			Blob   string `json:"git_blob"`
			SHA256 string `json:"sha256"`
		} `json:"sources"`
		Profiles []struct {
			OverflowChecks, DebugAssertions bool
			Records                         int
			RequestsSHA256, ResponsesSHA256 string
		} `json:"profiles"`
	}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Format != 1 || manifest.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || manifest.OriginalTests != 7 || len(manifest.Profiles) != 4 {
		t.Fatal("original-source provenance changed")
	}
	want := map[string][2]string{
		"compiler/rustc_index/src/interval.rs":       {"475c81e66ab09229137350775db615e39df24eee", "1f1e094a6d260ebd3ffd00309a324d4c2ab93620463c6fb6dec5ba22715da3bc"},
		"compiler/rustc_index/src/interval/tests.rs": {"cf3222e6c6572303939f620858a4d98a90ba2bbe", "a2e057843e58fbee1b3057fe09bd25f70c64953701d7d60883d3aa0a4f87a2e2"},
	}
	for _, source := range manifest.Sources {
		if expected, ok := want[source.Path]; ok {
			if source.Blob != expected[0] || source.SHA256 != expected[1] {
				t.Fatal("pinned source differs:", source.Path)
			}
			delete(want, source.Path)
		}
	}
	if len(want) != 0 {
		t.Fatal("missing original source provenance")
	}
	data, err = os.ReadFile(filepath.Join(dir, "oracle.rs"))
	if err != nil {
		t.Fatal(err)
	}
	if digest(data) != manifest.OracleSHA256 {
		t.Fatal("oracle source checksum differs")
	}
	compressed, err := os.ReadFile(filepath.Join(dir, "testdata/oracle.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if digest(compressed) != manifest.FixtureSHA256 {
		t.Fatal("compressed fixture checksum differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	data, err = io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if digest(data) != manifest.RawSHA256 {
		t.Fatal("raw fixture checksum differs")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	requests := oracle.Requests()
	count := 0
	for _, profile := range manifest.Profiles {
		if profile.Records != len(requests) {
			t.Fatal("request count changed; regenerate exact-source fixture")
		}
		var input, output bytes.Buffer
		for _, expected := range requests {
			var record oracle.Record
			if err := decoder.Decode(&record); err != nil {
				t.Fatal(err)
			}
			if record.OverflowChecks != profile.OverflowChecks || record.DebugAssertions != profile.DebugAssertions || record.Request.Protocol() != expected.Protocol() {
				t.Fatal("fixture request/profile order differs")
			}
			actual := oracle.Execute(record.Request, record.OverflowChecks, record.DebugAssertions)
			if actual != record.Response {
				t.Fatalf("original Rust differs, overflow=%v debug=%v request %s\nGo: %s\nRust: %s", record.OverflowChecks, record.DebugAssertions, record.Request.Protocol(), actual, record.Response)
			}
			input.WriteString(record.Request.Protocol())
			output.WriteString(record.Response)
			output.WriteByte('\n')
			count++
		}
		if digest(input.Bytes()) != profile.RequestsSHA256 || digest(output.Bytes()) != profile.ResponsesSHA256 {
			t.Fatal("profile stream checksum differs")
		}
	}
	var extra oracle.Record
	if err := decoder.Decode(&extra); err != io.EOF {
		t.Fatal("unexpected trailing fixture data", err)
	}
	if count != manifest.Records {
		t.Fatal("fixture total differs")
	}
	t.Logf("%d exact original-source records matched", count)
}
func TestLiveRustOracle(t *testing.T) {
	compiler, requested := os.LookupEnv("FERRUGO_RUSTC")
	if !requested {
		t.Skip("set FERRUGO_RUSTC to compile unchanged original source")
	}
	if compiler == "" {
		t.Fatal("FERRUGO_RUSTC explicitly empty")
	}
	root := repositoryRoot(t)
	cmd := exec.Command(filepath.Join(runtime.GOROOT(), "bin/go"), "run", "./tools/port-rust-interval", "-root", root, "-rustc", compiler, "-check")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("live original-source validation: %v\n%s", err, output)
	}
}

func TestCloneIsolationAndUnionResult(t *testing.T) {
	left, right := newU32(32), newU32(32)
	right.Insert(7)
	if left.Union(right) {
		t.Fatal("original optimized union returns false when copied other already contains old self")
	}
	if !left.Contains(7) {
		t.Fatal("union did not copy the other interval")
	}
	copy := left.Clone()
	copy.Insert(9)
	if left.Contains(9) {
		t.Fatal("clone aliases original intervals")
	}
	left.CloneFrom(copy)
	copy.Clear()
	if !left.Contains(9) {
		t.Fatal("clone_from aliases its source")
	}
	matrix := index.NewSparseIntervalMatrix(index.U32Idx(), func(a, b uint32) bool { return a == b }, index.U32Idx(), 32, index.U32IntervalStep())
	matrix.Insert(2, 7)
	other := matrix.Clone()
	other.Insert(2, 9)
	if matrix.Contains(2, 9) || !other.Contains(2, 9) {
		t.Fatal("matrix row clone aliases source")
	}
	row, _ := other.Row(0)
	if !row.IsEmpty() {
		t.Fatal("sparse growth omitted intermediate empty row")
	}
}
func TestStepDefaultsAndOperatorOverrides(t *testing.T) {
	type key struct {
		value    uint
		metadata []byte
	}
	var events []string
	step := index.IntervalStep[key]{
		PartialCompareFn: func(a, b key) index.IntervalOrdering {
			events = append(events, "compare")
			if a.value == 99 {
				return index.IntervalUnordered
			}
			if a.value < b.value {
				return index.IntervalLess
			}
			if a.value > b.value {
				return index.IntervalGreater
			}
			return index.IntervalEqual
		},
		ForwardCheckedFn: func(value key, n uint) (key, bool) {
			events = append(events, "forward-checked")
			value.value += n
			return value, value.value < 10
		},
		BackwardCheckedFn: func(value key, n uint) (key, bool) {
			events = append(events, "backward-checked")
			if n > value.value {
				return key{}, false
			}
			value.value -= n
			return value, true
		},
	}
	if step.LessEqual(key{99, nil}, key{1, nil}) || step.GreaterEqual(key{99, nil}, key{1, nil}) {
		t.Fatal("unordered comparison became ordered")
	}
	step.LessFn = func(key, key) bool { events = append(events, "less-override"); return true }
	if !step.Less(key{}, key{}) || events[len(events)-1] != "less-override" {
		t.Fatal("independent operator override ignored")
	}
	if value := step.ForwardUnchecked(key{1, []byte{1}}, 2); value.value != 3 || events[len(events)-1] != "forward-checked" {
		t.Fatal("unchecked default did not dispatch through forward default")
	}
	step.ForwardOverride = func(value key, n uint) key {
		events = append(events, "forward-override")
		value.value = n
		return value
	}
	if value := step.ForwardUnchecked(key{}, 7); value.value != 7 || events[len(events)-1] != "forward-override" {
		t.Fatal("unchecked default bypassed forward override")
	}
	if value := step.BackwardUnchecked(key{3, nil}, 2); value.value != 1 || events[len(events)-1] != "backward-checked" {
		t.Fatal("backward default dispatch differs")
	}
}
func TestRangeCloneAndExhaustion(t *testing.T) {
	var events []string
	idx := index.NewIdx(func(value uint) uint { events = append(events, "new"); return value }, func(value uint) uint { events = append(events, "index"); return value })
	set := index.NewIntervalSet(idx, 16, index.UsizeIntervalStep())
	var original index.IntervalRangeBounds[uint]
	original = index.IntervalRangeBounds[uint]{
		CloneFn: func() index.IntervalRangeBounds[uint] {
			events = append(events, "clone")
			return index.IntervalRangeBounds[uint]{StartBoundFn: func() index.IntervalBound[uint] {
				events = append(events, "clone-start")
				return index.IntervalBound[uint]{Kind: index.IntervalIncluded, Value: 3}
			}, EndBoundFn: func() index.IntervalBound[uint] { panic("must read original end") }}
		},
		StartBoundFn: func() index.IntervalBound[uint] { panic("must read cloned start") },
		EndBoundFn: func() index.IntervalBound[uint] {
			events = append(events, "original-end")
			return index.IntervalBound[uint]{Kind: index.IntervalIncluded, Value: 5}
		},
	}
	set.InsertRange(original)
	if strings.Join(events, ",") != "clone,clone-start,index,original-end,index" {
		t.Fatal("range callback order", events)
	}
	set.Clear()
	set.InsertRange(index.IntervalStandardInclusive(index.RangeInclusive[uint]{Start: 1, End: 0, Exhausted: true}))
	if !set.IsEmpty() {
		t.Fatal("exhausted inclusive range became nonempty")
	}
}

func FuzzIntervalState(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23})
	f.Add([]byte{5, 31, 0, 6, 0, 1, 11, 7, 31, 12, 7, 0})
	f.Fuzz(func(t *testing.T, data []byte) {
		sets := [2]*index.IntervalSet[uint32]{newU32(32), newU32(32)}
		var model [2][32]bool
		matrix := index.NewSparseIntervalMatrix(index.U32Idx(), func(a, b uint32) bool { return a == b }, index.U32Idx(), 32, index.U32IntervalStep())
		var rows [8][32]bool
		rowCount := uint32(0)
		for position := 0; position+3 < len(data) && position < 384; position += 4 {
			action := data[position] % 14
			target := data[position+1] % 2
			a, b := uint32(data[position+2]%32), uint32(data[position+3]%32)
			set := sets[target]
			switch action {
			case 0:
				expected := !model[target][a]
				if set.Insert(a) != expected {
					t.Fatal("insert flag")
				}
				model[target][a] = true
			case 1:
				set.InsertRange(index.IntervalClosed(a, b))
				if a <= b {
					for value := a; value <= b; value++ {
						model[target][value] = true
					}
				}
			case 2:
				last := a
				for value := uint32(0); value < 32; value++ {
					if model[target][value] && value > last {
						last = value
					}
				}
				set.Append(last)
				model[target][last] = true
			case 3:
				last := a
				for value := uint32(0); value < 32; value++ {
					if model[target][value] && value > last {
						last = value
					}
				}
				end := last
				if b > end {
					end = b
				}
				set.AppendRange(index.IntervalClosed(last, end))
				for value := last; value <= end; value++ {
					model[target][value] = true
				}
			case 4:
				set.Clear()
				model[target] = [32]bool{}
			case 5:
				set.InsertAll()
				for value := range model[target] {
					model[target][value] = true
				}
			case 6:
				set.Union(sets[1-target])
				for value := range model[target] {
					model[target][value] = model[target][value] || model[1-target][value]
				}
			case 7:
				want := true
				for value := range model[target] {
					if model[1-target][value] && !model[target][value] {
						want = false
					}
				}
				if set.Superset(sets[1-target]) != want {
					t.Fatal("superset")
				}
			case 8:
				want := true
				for value := range model[target] {
					if model[1-target][value] && model[target][value] {
						want = false
					}
				}
				if set.Disjoint(sets[1-target]) != want {
					t.Fatal("disjoint")
				}
			case 9:
				got, ok := set.FirstUnsetIn(index.IntervalClosed(a, b))
				var want uint32
				present := false
				if a <= b {
					for value := a; value <= b; value++ {
						if !model[target][value] {
							want, present = value, true
							break
						}
					}
				}
				if ok != present || (ok && got != want) {
					t.Fatal("first unset")
				}
			case 10:
				got, ok := set.LastSetIn(index.IntervalClosed(a, b))
				var want uint32
				present := false
				if a <= b {
					for value := a; value <= b; value++ {
						if model[target][value] {
							want, present = value, true
						}
					}
				}
				if ok != present || (ok && got != want) {
					t.Fatal("last set")
				}
			case 11:
				row := a % 8
				matrix.Insert(row, b)
				rows[row][b] = true
				if row >= rowCount {
					rowCount = row + 1
				}
			case 12:
				read, write := a%8, b%8
				matrix.UnionRows(read, write)
				if read != write && read < rowCount {
					if write >= rowCount {
						rowCount = write + 1
					}
					for value := range rows[write] {
						rows[write][value] = rows[write][value] || rows[read][value]
					}
				}
			case 13:
				row := a % 8
				matrix.ClearRow(row)
				rows[row] = [32]bool{}
			}
			for i, values := range model {
				for value, want := range values {
					if sets[i].Contains(uint32(value)) != want {
						t.Fatal("set model membership")
					}
				}
			}
			for row := uint32(0); row < 8; row++ {
				for value, want := range rows[row] {
					if matrix.Contains(row, uint32(value)) != want {
						t.Fatal("matrix model membership")
					}
				}
			}
		}
	})
}
func newUsize(domain uint) *index.IntervalSet[uint] {
	return index.NewIntervalSet(index.UsizeIdx(), domain, index.UsizeIntervalStep())
}
func points[I any](set *index.IntervalSet[I]) []I {
	var out []I
	for value := range set.Iter().Seq() {
		out = append(out, value)
	}
	return out
}
func expect[I any](t *testing.T, set *index.IntervalSet[I], want []I) {
	t.Helper()
	if got := points(set); !reflect.DeepEqual(got, want) {
		t.Fatalf("points: got %v, want %v", got, want)
	}
}
func TestInsertCollapses(t *testing.T) {
	set := newU32(10000)
	set.InsertRange(index.IntervalClosed(uint32(9831), uint32(9837)))
	set.InsertRange(index.IntervalClosed(uint32(43), uint32(9830)))
	iterator := set.IterIntervals()
	value, ok := iterator.Next()
	if !ok || value.Start != 43 || value.Last != 9837 {
		t.Fatal(value, ok)
	}
	if _, ok := iterator.Next(); ok {
		t.Fatal("extra interval")
	}
}
func TestContains(t *testing.T) {
	set := newU32(300)
	set.Insert(0)
	if !set.Contains(0) {
		t.Fatal("missing zero")
	}
	set.InsertRange(index.IntervalHalfOpen(uint32(0), uint32(10)))
	if !set.Contains(9) || set.Contains(10) {
		t.Fatal("half-open membership")
	}
	set.InsertRange(index.IntervalHalfOpen(uint32(10), uint32(11)))
	if !set.Contains(10) {
		t.Fatal("missing ten")
	}
}
func TestInsert(t *testing.T) {
	for i := uint(0); i < 30; i++ {
		set := newUsize(300)
		for j := i; j < 30; j++ {
			set.Insert(j)
			for k := i; k < j; k++ {
				if !set.Contains(k) {
					t.Fatal(i, j, k)
				}
			}
		}
	}
	set := newU32(300)
	set.InsertRange(index.IntervalHalfOpen(uint32(0), uint32(1)))
	if !set.Contains(0) || set.Contains(1) {
		t.Fatal("initial membership")
	}
	set.InsertRange(index.IntervalHalfOpen(uint32(1), uint32(1)))
	if !set.Contains(0) || set.Contains(1) {
		t.Fatal("empty insertion changed membership")
	}
	set = newU32(300)
	set.InsertRange(index.IntervalHalfOpen(uint32(4), uint32(5)))
	set.InsertRange(index.IntervalHalfOpen(uint32(5), uint32(10)))
	expect(t, set, []uint32{4, 5, 6, 7, 8, 9})
	set.InsertRange(index.IntervalHalfOpen(uint32(3), uint32(7)))
	expect(t, set, []uint32{3, 4, 5, 6, 7, 8, 9})
	for _, test := range []struct {
		start  uint32
		end    uint32
		values []uint32
	}{
		{3, 5, []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{0, 3, []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{0, 10, []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{5, 13, []uint32{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}},
	} {
		set = newU32(300)
		set.InsertRange(index.IntervalHalfOpen(uint32(0), uint32(10)))
		set.InsertRange(index.IntervalHalfOpen(test.start, test.end))
		expect(t, set, test.values)
	}
}
func TestInsertRange(t *testing.T) {
	check := func(start, end uint, inclusive bool) {
		t.Helper()
		bounds := index.IntervalHalfOpen(start, end)
		limit := end
		if inclusive {
			bounds = index.IntervalClosed(start, end)
			limit++
		}
		set := newUsize(300)
		for repeat := 0; repeat < 2; repeat++ {
			set.InsertRange(bounds)
			for _, value := range points(set) {
				if value < start || value >= limit {
					t.Fatalf("unexpected %d in %d..%d", value, start, limit)
				}
			}
			for value := start; value < limit; value++ {
				if !set.Contains(value) {
					t.Fatalf("missing %d in %d..%d", value, start, limit)
				}
			}
		}
	}
	for _, inclusive := range []bool{false, true} {
		for _, bounds := range [][2]uint{{10, 10}, {10, 100}, {10, 30}, {0, 5}, {0, 250}, {200, 250}} {
			check(bounds[0], bounds[1], inclusive)
		}
	}
	for i := uint(0); i < 30; i++ {
		for j := i; j < 30; j++ {
			check(i, j, false)
			check(i, j, true)
		}
	}
}
func TestInsertRangeDual(t *testing.T) {
	set := newU32(300)
	set.InsertRange(index.IntervalHalfOpen(uint32(0), uint32(3)))
	expect(t, set, []uint32{0, 1, 2})
	set.InsertRange(index.IntervalHalfOpen(uint32(5), uint32(7)))
	expect(t, set, []uint32{0, 1, 2, 5, 6})
	set.InsertRange(index.IntervalHalfOpen(uint32(3), uint32(4)))
	expect(t, set, []uint32{0, 1, 2, 3, 5, 6})
	set.InsertRange(index.IntervalHalfOpen(uint32(3), uint32(5)))
	expect(t, set, []uint32{0, 1, 2, 3, 4, 5, 6})
}
func TestLastSetBeforeAdjacent(t *testing.T) {
	set := newU32(300)
	set.InsertRange(index.IntervalHalfOpen(uint32(0), uint32(3)))
	set.InsertRange(index.IntervalHalfOpen(uint32(3), uint32(5)))
	check := func() {
		for _, test := range [][3]uint32{{0, 3, 2}, {0, 5, 4}, {3, 5, 4}} {
			got, ok := set.LastSetIn(index.IntervalHalfOpen(test[0], test[1]))
			if !ok || got != test[2] {
				t.Fatal(test, got, ok)
			}
		}
	}
	check()
	set.InsertRange(index.IntervalHalfOpen(uint32(2), uint32(5)))
	check()
}
func TestLastSetIn(t *testing.T) {
	cmp := func(set *index.IntervalSet[uint], bounds index.IntervalRangeBounds[uint]) {
		t.Helper()
		var want uint
		present := false
		start, end := bounds.StartBound(), bounds.EndBound()
		for _, value := range points(set) {
			left := start.Kind == index.IntervalUnbounded || (start.Kind == index.IntervalIncluded && value >= start.Value) || (start.Kind == index.IntervalExcluded && value > start.Value)
			right := end.Kind == index.IntervalUnbounded || (end.Kind == index.IntervalIncluded && value <= end.Value) || (end.Kind == index.IntervalExcluded && value < end.Value)
			if left && right {
				want, present = value, true
			}
		}
		got, ok := set.LastSetIn(bounds)
		if ok != present || (ok && got != want) {
			t.Fatalf("last set: got %d/%v want %d/%v", got, ok, want, present)
		}
	}
	set := newUsize(300)
	cmp(set, index.IntervalClosed(uint(50), uint(50)))
	set.Insert(64)
	cmp(set, index.IntervalClosed(uint(64), uint(64)))
	set.Insert(63)
	cmp(set, index.IntervalClosed(uint(0), uint(63)))
	cmp(set, index.IntervalClosed(uint(0), uint(5)))
	cmp(set, index.IntervalHalfOpen(uint(10), uint(100)))
	set.Insert(100)
	cmp(set, index.IntervalHalfOpen(uint(100), uint(110)))
	cmp(set, index.IntervalHalfOpen(uint(99), uint(100)))
	cmp(set, index.IntervalClosed(uint(99), uint(100)))
	for i := uint(0); i <= 30; i++ {
		for j := i; j <= 30; j++ {
			for k := uint(0); k < 30; k++ {
				set := newUsize(100)
				for repeat := 0; repeat < 2; repeat++ {
					cmp(set, index.IntervalTo(j))
					cmp(set, index.IntervalFrom(i))
					cmp(set, index.IntervalHalfOpen(i, j))
					cmp(set, index.IntervalClosed(i, j))
					if repeat == 0 {
						set.Insert(k)
					}
				}
			}
		}
	}
}
