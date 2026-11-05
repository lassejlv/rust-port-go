// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

type itemVectorRunner struct{ itemSortRunner }

func (r *itemVectorRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		config := UnordBagConfig[itemSortValue]{Layout: rusthashmap.Layout{Size: 8, Align: 4}, Profile: rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}, Values: UnordBagValueOps[itemSortValue]{Drop: r.itemDrop}}
		bag := NewUnordBag(config, c)
		for i := 0; i < r.spec.Request.Length; i++ {
			bag.Push(itemSortValue{key: int32((i*7 + 3) % 5), id: uint32(i)}, c)
		}
		items := bag.IntoItems(c).General(c)
		selected := UnordOwnedVectorCollector(config, rustsort.Type{Size: 8, Align: 4, Freeze: true, Copy: false})
		compare := UnordStableCompare[itemSortValue]{CanUseUnstableSort: r.spec.Unstable, Compare: func(a, b *itemSortValue, c rustfmt.Context) rustsort.Ordering { return r.compare("stable", a, b, c) }}
		var result *rustcindex.IndexVec[uint, itemSortValue]
		switch r.spec.Request.Action {
		case 0:
			result = IntoSortedUnordItems(items, nil, selected, r.keyOps(), c)
		case 1:
			result = IntoSortedUnordItemsStableOrd(items, selected, compare, c)
		case 2:
			result = IntoSortedUnordItemsStableOrdByKey(items, r.projection(), selected, compare, c)
		default:
			panic("unexpected vector sort action")
		}
		r.event("returned:%s:%d", itemSortIDs(result), result.Raw.Cap())
		result.CloseWithRuntimeContext(c)
	}()
	r.event("caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *itemVectorRunner) run() {
	cleanupLocation := r.locations["panic in a destructor during cleanup"]
	cleanup := rustfmt.SourcePanicInfo{Payload: "panic in a destructor during cleanup", Kind: rustfmt.PanicStaticStr, Location: &cleanupLocation, CanUnwind: false, ForceNoBacktrace: true}
	c := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{CleanupAbort: &cleanup, Hook: func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
		location := rustfmt.SourceLocation{}
		if info.Location != nil {
			location = *info.Location
		}
		r.event("HOOK:%s:%s:%t:%t:%t:%s:%d:%d", info.Kind, hex.EncodeToString([]byte(fmt.Sprint(info.Payload))), c.Panicking(), info.CanUnwind, info.ForceNoBacktrace, hex.EncodeToString([]byte(location.File)), location.Line, location.Column)
		if r.spec.Mode == "processing" {
			r.body(c)
		}
	}})
	if r.spec.Mode == "normal" {
		r.body(c)
		return
	}
	func() {
		defer func() {
			if recover() != nil && r.spec.Mode == "unwind" {
				unwind := c
				unwind.Unwinding = true
				r.body(unwind)
			}
		}()
		location := r.locations["outer."+r.spec.Mode]
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
	}()
	r.event("outer_end:false")
}
func loadItemVectorManifest(t *testing.T) itemSortManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/item-sort/vector_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m itemSortManifest
	if err = json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{m.Files, m.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || itemSortDigest(data) != want {
				t.Fatalf("vector binding %s %v", path, err)
			}
		}
	}
	return m
}
func TestOriginalUnordItemsVector(t *testing.T) {
	metadata := loadItemVectorManifest(t)
	data, err := os.ReadFile("testdata/unord_items_vector.jsonl.gz")
	if err != nil || itemSortDigest(data) != metadata.Fixture {
		t.Fatal("sort fixture", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 16<<20))
	if err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if itemSortDigest(raw) != metadata.Raw {
		t.Fatal("sort raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec itemSortCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsVectorChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_VECTOR="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("sort abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := itemVectorRunner{itemSortRunner: itemSortRunner{spec: spec, locations: metadata.Locations, out: &output}}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("sort case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 864 || metadata.Cases != 864 || aborts != metadata.Aborts {
		t.Fatal("sort coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsVectorChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_ITEMS_VECTOR")
	if data == "" {
		return
	}
	metadata := loadItemVectorManifest(t)
	var spec itemSortCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := itemVectorRunner{itemSortRunner: itemSortRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}}
	runner.run()
	os.Exit(0)
}
