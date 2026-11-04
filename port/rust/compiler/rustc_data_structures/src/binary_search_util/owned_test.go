// SPDX-License-Identifier: MIT OR Apache-2.0
// Independent replay of unchanged-source ownership, borrowing and ZST proofs.
package binarysearchutil

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

type ownedSearchRequest struct {
	ID       string            `json:"id"`
	ZST      bool              `json:"zst"`
	Embedded bool              `json:"embedded"`
	Length   string            `json:"length"`
	ZSTKey   int64             `json:"zstKey"`
	Values   []int64           `json:"values"`
	Query    int64             `json:"query"`
	Context  uint8             `json:"context"`
	Fail     map[string]uint64 `json:"fail"`
}
type ownedSearchView struct {
	Start  string `json:"start"`
	Length string `json:"length"`
	Alias  bool   `json:"alias"`
}
type ownedSearchSnapshot struct {
	Extracts, Keys, KeyDrops, FnDrops, QueryDrops, QueryTicks uint64
	Operators                                                 [3]uint64
	Data                                                      [][3]uint64
}
type ownedSearchResponse struct {
	ID       string               `json:"id"`
	Status   string               `json:"status"`
	Message  string               `json:"message"`
	View     *ownedSearchView     `json:"view"`
	Trace    []string             `json:"trace"`
	Snapshot *ownedSearchSnapshot `json:"snapshot"`
}
type ownedSearchRecord struct {
	Request  ownedSearchRequest  `json:"request"`
	Expected ownedSearchResponse `json:"expected"`
}
type ownedSearchManifest struct {
	Cases       int                             `json:"cases"`
	PointerBits int                             `json:"pointer_bits"`
	RawSHA      string                          `json:"raw_sha256"`
	FixtureSHA  string                          `json:"fixture_sha256"`
	Legacy      []struct{ Path, SHA256 string } `json:"legacy_preserved"`
}
type ownedSearchMeta struct{ extracts, operators, returned uint64 }

// Nonzero neighboring storage makes a lost slice pointer observable.
var ownedSearchEmbeddedUnitData struct {
	Values [4]struct{}
	Anchor byte
}

type ownedSearchElement struct {
	value    int64
	position uint
	meta     *ownedSearchMeta
}
type ownedSearchKey struct {
	value     int64
	id, ticks uint64
	borrowed  *ownedSearchKey
	data      *ownedSearchMeta
	shared    *ownedSearchState
}
type ownedSearchState struct {
	request                           ownedSearchRequest
	trace                             []string
	extracts, keys, keyDrops, fnDrops uint64
	operators                         [3]uint64
	query                             *ownedSearchKey
	environment                       any
	metadata                          []*ownedSearchMeta
	emit                              func(string)
}

func (state *ownedSearchState) event(text string) {
	if len(state.trace) >= 2048 {
		panic("Go owned oracle event bound exceeded")
	}
	state.trace = append(state.trace, text)
	if state.emit != nil {
		state.emit(text)
	}
}

type ownedSearchExtractor[E any] struct {
	state    *ownedSearchState
	calls    uint64
	describe func(*E) (int64, string, *ownedSearchMeta, bool)
}

func (extractor *ownedSearchExtractor[E]) Extract(element *E, context SearchContext) ExtractedKey[ownedSearchKey] {
	state := extractor.state
	value, label, meta, dataSame := extractor.describe(element)
	state.extracts++
	extractor.calls++
	envSame := state.environment == nil || state.environment == extractor
	state.environment = extractor
	if meta != nil {
		meta.extracts++
	}
	state.event(fmt.Sprintf("extract:%d:%s:%d:%t:%t:%t", state.extracts, label, extractor.calls, envSame, dataSame, context.Panicking()))
	if state.request.Fail["extract"] == state.extracts {
		context.Panic("extract")
	}
	state.keys++
	key := ownedSearchKey{value: value, id: state.keys, data: meta, shared: state}
	return ExtractedKey[ownedSearchKey]{Value: key, Drop: func(key *ownedSearchKey, context SearchContext) {
		state.keyDrops++
		state.event(fmt.Sprintf("key-drop:%d:%d:%d:%t:%t", key.id, state.keyDrops, key.ticks, key == key.borrowed, context.Panicking()))
		if state.request.Fail["keyDrop"] == state.keyDrops {
			context.Panic("key-drop")
		}
	}}
}
func (extractor *ownedSearchExtractor[E]) Close(context SearchContext) {
	state := extractor.state
	state.fnDrops++
	same := state.environment == nil || state.environment == extractor
	state.event(fmt.Sprintf("fn-drop:%d:%d:%t:%t", state.extracts, extractor.calls, same, context.Panicking()))
	if state.request.Fail["fnDrop"] != 0 {
		context.Panic("fn-drop")
	}
}
func ownedSearchOperator(operator int) func(*ownedSearchKey, *ownedSearchKey, SearchContext) bool {
	return func(key, query *ownedSearchKey, context SearchContext) bool {
		state := key.shared
		key.ticks++
		query.ticks++
		if key.data != nil {
			key.data.operators++
		}
		state.operators[operator]++
		name := []string{"lt", "ne", "le"}[operator]
		same := key.borrowed == nil || key == key.borrowed
		key.borrowed = key
		state.event(fmt.Sprintf("op:%s:%d:%d:%d:%d:%t:%t:%t", name, state.operators[operator], key.id, key.ticks, query.ticks, query == state.query, same, context.Panicking()))
		if state.request.Fail[name] == state.operators[operator] {
			context.Panic(name)
		}
		switch operator {
		case 0:
			return key.value < query.value
		case 1:
			return key.value != query.value
		default:
			return key.value <= query.value
		}
	}
}

type ownedSearchOutcome struct {
	status, message string
	view            *ownedSearchView
}

func ownedSearchCatch(invoke func() *ownedSearchView) (out ownedSearchOutcome) {
	defer func() {
		if value := recover(); value != nil {
			out = ownedSearchOutcome{status: "panic", message: fmt.Sprint(value)}
		}
	}()
	out = ownedSearchOutcome{status: "ok", view: invoke()}
	return
}
func ownedSearchContextCall(state *ownedSearchState, invoke func(SearchContext) *ownedSearchView) ownedSearchOutcome {
	switch state.request.Context {
	case 0:
		return ownedSearchCatch(func() *ownedSearchView { return invoke(SearchContext{}) })
	case 3:
		return ownedSearchCatch(func() *ownedSearchView { return invoke(SearchContext{ProcessingPanic: true}) })
	default:
		var inner ownedSearchOutcome
		outer := ownedSearchCatch(func() *ownedSearchView {
			defer func() {
				original := recover()
				if original == nil {
					return
				}
				cleanup := ownedSearchCatch(func() *ownedSearchView {
					state.event("outer-enter:true")
					if state.request.Context == 1 {
						inner = ownedSearchCatch(func() *ownedSearchView { return invoke(SearchContext{Unwinding: true}) })
					} else {
						inner = ownedSearchOutcome{status: "ok", view: invoke(SearchContext{Unwinding: true})}
					}
					state.event("outer-inner:" + inner.status + ":" + inner.message)
					state.event("outer-return:true")
					return nil
				})
				if cleanup.status == "panic" {
					os.Exit(134)
				}
				panic(original)
			}()
			panic("outer")
		})
		state.event("outer-caught:" + outer.message)
		return inner
	}
}
func ownedSearchReplayTyped[E any](state *ownedSearchState, raw index.IndexRaw[E], slice []E, describe func(*E) (int64, string, *ownedSearchMeta, bool), position func(*E) uint, entry string) ownedSearchResponse {
	extractor := &ownedSearchExtractor[E]{state: state, describe: describe}
	operators := BorrowedKeyOperators[ownedSearchKey]{Less: ownedSearchOperator(0), NotEqual: ownedSearchOperator(1), LessOrEqual: ownedSearchOperator(2)}
	outcome := ownedSearchContextCall(state, func(context SearchContext) *ownedSearchView {
		var result index.IndexRaw[E]
		switch entry {
		case "slice":
			found := BinarySearchSliceOwned(slice, extractor, state.query, operators)
			result = index.IndexSliceFromRaw(found, index.UsizeIdx(), index.ValueOps[E]{}).Raw
		case "slice-context":
			found := BinarySearchSliceOwnedWithContext(slice, extractor, state.query, operators, context)
			result = index.IndexSliceFromRaw(found, index.UsizeIdx(), index.ValueOps[E]{}).Raw
		case "storage":
			result = BinarySearchStorageOwned(raw, extractor, state.query, operators)
		default:
			result = BinarySearchStorageOwnedWithContext(raw, extractor, state.query, operators, context)
		}
		view := &ownedSearchView{Start: "0", Length: strconv.FormatUint(uint64(result.Len()), 10)}
		if result.Len() != 0 {
			first := result.At(0)
			start := position(first)
			view.Start = strconv.FormatUint(uint64(start), 10)
			view.Alias = first == raw.At(start)
			_, _, meta, _ := describe(first)
			if meta != nil {
				meta.returned++
			}
		}
		return view
	})
	data := make([][3]uint64, len(state.metadata))
	for i, meta := range state.metadata {
		data[i] = [3]uint64{meta.extracts, meta.operators, meta.returned}
	}
	return ownedSearchResponse{ID: state.request.ID, Status: outcome.status, Message: outcome.message, View: outcome.view, Trace: state.trace, Snapshot: &ownedSearchSnapshot{Extracts: state.extracts, Keys: state.keys, KeyDrops: state.keyDrops, FnDrops: state.fnDrops, QueryDrops: 0, QueryTicks: state.query.ticks, Operators: state.operators, Data: data}}
}
func ownedSearchReplay(request ownedSearchRequest, entry string, emit func(string)) ownedSearchResponse {
	state := &ownedSearchState{request: request, emit: emit}
	query := ownedSearchKey{value: request.Query, shared: state}
	state.query = &query
	if request.ZST {
		length, err := strconv.ParseUint(request.Length, 10, strconv.IntSize)
		if err != nil {
			panic(err)
		}
		var values []struct{}
		var raw index.IndexRaw[struct{}]
		if request.Embedded {
			if length > 4 {
				panic("embedded unit slice length exceeds bound")
			}
			values = ownedSearchEmbeddedUnitData.Values[:int(length)]
			raw = index.IndexSliceFromRaw(values, index.UsizeIdx(), index.ValueOps[struct{}]{}).Raw
		} else {
			raw = index.IndexRawZeroSized(&struct{}{}, uint(length))
		}
		describe := func(value *struct{}) (int64, string, *ownedSearchMeta, bool) {
			return request.ZSTKey, "z", nil, value == raw.At(0)
		}
		return ownedSearchReplayTyped(state, raw, values, describe, func(*struct{}) uint { return 0 }, entry)
	}
	values := make([]ownedSearchElement, len(request.Values))
	state.metadata = make([]*ownedSearchMeta, len(values))
	for i, value := range request.Values {
		meta := new(ownedSearchMeta)
		state.metadata[i] = meta
		values[i] = ownedSearchElement{value: value, position: uint(i), meta: meta}
	}
	raw := index.IndexSliceFromRaw(values, index.UsizeIdx(), index.ValueOps[ownedSearchElement]{}).Raw
	describe := func(element *ownedSearchElement) (int64, string, *ownedSearchMeta, bool) {
		return element.value, strconv.FormatUint(uint64(element.position), 10), element.meta, element == raw.At(element.position)
	}
	return ownedSearchReplayTyped(state, raw, values, describe, func(value *ownedSearchElement) uint { return value.position }, entry)
}
func ownedSearchRecords(t *testing.T) ([]ownedSearchRecord, ownedSearchManifest) {
	t.Helper()
	var manifest ownedSearchManifest
	metadata, err := os.ReadFile("owned_oracle_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Cases < 1 || manifest.Cases > 4000 {
		t.Fatal("owned oracle case count exceeds bound")
	}
	compressed, err := os.ReadFile("testdata/owned_oracle.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != manifest.FixtureSHA {
		t.Fatal("owned fixture compressed hash differs")
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(gzipReader, (64<<20)+1))
	if err != nil {
		t.Fatal(err)
	}
	if err = gzipReader.Close(); err != nil {
		t.Fatal(err)
	}
	if len(raw) > 64<<20 {
		t.Fatal("owned oracle raw data exceeds bound")
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != manifest.RawSHA {
		t.Fatal("owned fixture raw hash differs")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 2<<20)
	records := make([]ownedSearchRecord, 0, manifest.Cases)
	for scanner.Scan() {
		var record ownedSearchRecord
		if err = json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(records) != manifest.Cases {
		t.Fatal("owned fixture case count differs")
	}
	return records, manifest
}
func TestBinarySearchOwnedAbortHelper(t *testing.T) {
	encoded, ok := os.LookupEnv("FERRUGO_BINARY_OWNED_ABORT")
	if !ok {
		t.Skip("isolated original-source abort replay")
	}
	var config struct {
		Request ownedSearchRequest
		Entry   string
	}
	if err := json.Unmarshal([]byte(encoded), &config); err != nil {
		t.Fatal(err)
	}
	ownedSearchReplay(config.Request, config.Entry, func(event string) { fmt.Fprintln(os.Stdout, event) })
	t.Fatal("expected source abort returned")
}
func TestBinarySearchOwnedOriginalOracle(t *testing.T) {
	records, manifest := ownedSearchRecords(t)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		if record.Request.ZST && !record.Request.Embedded && strconv.IntSize != manifest.PointerBits {
			continue
		} // host-specific full-usize trace; no target-runtime claim
		entries := []string{"storage-context"}
		if !record.Request.ZST || record.Request.Embedded {
			entries = append(entries, "slice-context")
		}
		if record.Request.Context == 0 {
			entries = append(entries, "storage")
			if !record.Request.ZST || record.Request.Embedded {
				entries = append(entries, "slice")
			}
		}
		for _, entry := range entries {
			if record.Expected.Status == "abort" {
				config := struct {
					Request ownedSearchRequest
					Entry   string
				}{record.Request, entry}
				encoded, err := json.Marshal(config)
				if err != nil {
					t.Fatal(err)
				}
				deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				command := exec.CommandContext(deadline, binary, "-test.run=^TestBinarySearchOwnedAbortHelper$")
				command.Env = append(os.Environ(), "FERRUGO_BINARY_OWNED_ABORT="+string(encoded))
				var stdout, stderr bytes.Buffer
				command.Stdout = &stdout
				command.Stderr = &stderr
				err = command.Run()
				cancel()
				var failure *exec.ExitError
				if !errors.As(err, &failure) || failure.ExitCode() != 134 {
					t.Fatalf("%s/%s must abort134: %v\n%s\n%s", record.Request.ID, entry, err, stdout.String(), stderr.String())
				}
				if got, want := strings.TrimSuffix(stdout.String(), "\n"), strings.Join(record.Expected.Trace, "\n"); got != want {
					t.Fatalf("%s/%s before-abort events differ:\ngot%s\nwant%s", record.Request.ID, entry, got, want)
				}
			} else {
				got := ownedSearchReplay(record.Request, entry, nil)
				if !reflect.DeepEqual(got, record.Expected) {
					actual, _ := json.Marshal(got)
					expected, _ := json.Marshal(record.Expected)
					t.Fatalf("%s/%s ownership differs:\ngot%s\nwant%s", record.Request.ID, entry, actual, expected)
				}
			}
		}
	}
}
func TestBinarySearchOwnedLegacyProbeParity(t *testing.T) {
	records, _ := ownedSearchRecords(t)
	for _, record := range records {
		request := record.Request
		if request.ZST || request.Context != 0 || len(request.Fail) != 0 {
			continue
		}
		values := make([]ownedSearchElement, len(request.Values))
		for i, value := range request.Values {
			values[i] = ownedSearchElement{value: value, position: uint(i)}
		}
		var got, want []string
		operators := KeyOperators[int64]{
			Less:        func(a, b int64) bool { got = append(got, "lt"); return a < b },
			NotEqual:    func(a, b int64) bool { got = append(got, "ne"); return a != b },
			LessOrEqual: func(a, b int64) bool { got = append(got, "le"); return a <= b },
		}
		result := BinarySearchSliceWithOperators(values, func(value *ownedSearchElement) int64 {
			got = append(got, "probe:"+strconv.FormatUint(uint64(value.position), 10))
			return value.value
		}, request.Query, operators)
		for _, event := range record.Expected.Trace {
			fields := strings.Split(event, ":")
			if fields[0] == "extract" {
				want = append(want, "probe:"+fields[2])
			} else if fields[0] == "op" {
				want = append(want, fields[1])
			}
		}
		if !reflect.DeepEqual(got, want) || strconv.FormatUint(uint64(len(result)), 10) != record.Expected.View.Length {
			t.Fatalf("%s legacy/owned probe order differs", request.ID)
		}
	}
}
func TestBinarySearchOwnedOracleRegeneration(t *testing.T) {
	if os.Getenv("FERRUGO_RUN_BINARY_OWNED_ORACLE") != "1" {
		t.Skip("set FERRUGO_RUN_BINARY_OWNED_ORACLE=1 for unchanged Rust regeneration")
	}
	root, err := filepath.Abs("../../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("python3", filepath.Join(root, "tools/port-rust-binary-search/owned/generate.py"), "--root", root, "--check")
	command.Dir = root
	command.Env = os.Environ()
	if os.Getenv("FERRUGO_GO") == "" {
		command.Env = append(command.Env, "FERRUGO_GO="+filepath.Join(runtime.GOROOT(), "bin", "go"))
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("owned original-source oracle: %v\n%s", err, output)
	}
}
