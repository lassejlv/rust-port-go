// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type bagContextConfig struct {
	Length             int
	Body, Token, Moved uint64
	Read               int
	Resume             bool
}
type bagContextCase struct {
	Mode, Action, Outcome, Stdout string
	Config                        bagContextConfig
}
type bagContextManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
}
type bagContextToken struct {
	id   uint64
	live bool
}
type bagContextValue struct {
	id    uint64
	token *bagContextToken
}
type bagContextRunner struct {
	spec      bagContextCase
	locations map[string]rustfmt.SourceLocation
	writer    io.Writer
	moved     []*bagContextToken
}

func (r *bagContextRunner) event(format string, args ...any) {
	fmt.Fprintf(r.writer, format+"\n", args...)
}
func bagContextScope(c rustfmt.Context) string {
	if c.Runtime == nil || c.Runtime.Value == nil {
		return "none"
	}
	return c.Runtime.Value.(string)
}
func bagContextMask(mask, id uint64) bool { return id < 64 && mask&(1<<id) != 0 }
func (r *bagContextRunner) raise(message string, c rustfmt.Context) {
	if r.spec.Config.Resume {
		c.ResumeUnwind(message)
	}
	location := r.locations[message]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: message, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func (r *bagContextRunner) newValue(id uint64, c rustfmt.Context) bagContextValue {
	r.event("new:%d:%s:%t", id, bagContextScope(c), c.Panicking())
	return bagContextValue{id, &bagContextToken{id: id, live: true}}
}
func (r *bagContextRunner) dropToken(value *bagContextToken, c rustfmt.Context) {
	if !value.live {
		c.Panic("test witness token was already moved/dropped")
	}
	value.live = false
	r.event("token:%d:%s:%t", value.id, bagContextScope(c), c.Panicking())
	if bagContextMask(r.spec.Config.Token, value.id) {
		r.raise("token", c)
	}
}
func (r *bagContextRunner) dropValue(value *bagContextValue, c rustfmt.Context) {
	fieldsDone := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !fieldsDone && value.token != nil {
				unwind := c
				unwind.Unwinding = true
				token := value.token
				value.token = nil
				bagItemsUnwind(unwind, func() { r.dropToken(token, unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	r.event("drop:%d:%s:%t", value.id, bagContextScope(c), c.Panicking())
	if bagContextMask(r.spec.Config.Moved, value.id) && value.token != nil {
		token := value.token
		value.token = nil
		r.event("move:%d:%t", value.id, c.Panicking())
		r.moved = append(r.moved, token)
	}
	if bagContextMask(r.spec.Config.Body, value.id) {
		r.raise("drop", c)
	}
	fieldsDone = true
	if value.token != nil {
		token := value.token
		value.token = nil
		r.dropToken(token, c)
	}
}

type bagContextReader struct {
	rustcserialize.Decoder
	runner  *bagContextRunner
	next    int
	context rustfmt.Context
}

func (r *bagContextReader) RuntimeContext() rustfmt.Context     { return r.context }
func (r *bagContextReader) SetRuntimeContext(c rustfmt.Context) { r.context = c }
func (r *bagContextReader) ReadUsize() uint {
	r.runner.event("length:%d:%s:%t", r.runner.spec.Config.Length, bagContextScope(r.context), r.context.Panicking())
	return uint(r.runner.spec.Config.Length)
}
func (r *bagContextReader) ReadU64() uint64 {
	id := r.next
	r.next++
	r.runner.event("read:%d:%s:%t", id, bagContextScope(r.context), r.context.Panicking())
	if id == r.runner.spec.Config.Read {
		r.runner.raise("read", r.context)
	}
	return uint64(id)
}
func (r *bagContextRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	config := UnordBagConfig[bagContextValue]{Layout: rusthashmap.Layout{Size: 24, Align: 8}, Profile: rustcindex.IndexStorageConfig{OverflowChecks: true, DebugAssertions: true}, Values: UnordBagValueOps[bagContextValue]{Drop: r.dropValue}}
	var bag *UnordBag[bagContextValue]
	if r.spec.Action == "get_only" {
		a := c
		meta := *c.Runtime
		meta.Value = "A"
		a.Runtime = &meta
		bag = NewUnordBag(config, a)
		for id := 0; id < r.spec.Config.Length; id++ {
			bag.Push(r.newValue(uint64(id), a), a)
		}
	}
	reader := &bagContextReader{runner: r}
	opaque, err := rustcserialize.NewMemDecoder(append([]byte{5, 42}, []byte(rustcserialize.MagicEndBytes)...), 0)
	if err != nil {
		panic(err)
	}
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		if r.spec.Action == "get_only" {
			item, ok := bag.IntoItems(c).GetOnly(c)
			if ok {
				r.event("returned:%d", item.Borrow().id)
				item.DropWithContext(c)
			} else {
				r.event("returned:none")
			}
			return
		}
		if r.spec.Action == "decode_bytes" {
			rustcserialize.WithRuntimeContext(opaque, c, func(decoder *rustcserialize.MemDecoder) struct{} {
				value := DecodeUnordByteBag(decoder, rustcindex.IndexStorageConfig{OverflowChecks: true, DebugAssertions: true})
				r.event("decoded:%d", value.Len())
				value.Close(c)
				return struct{}{}
			})
			return
		}
		key := func(d rustcserialize.Decoder, c rustfmt.Context) *rustvec.Owned[bagContextValue] {
			value := r.newValue(d.ReadU64(), c)
			return rustvec.NewOwned(value, rustvec.ValueOps[bagContextValue]{Drop: r.dropValue})
		}
		keys := rusthashmap.KeyOps[bagContextValue]{Hash: func(value *bagContextValue, h rusthashmap.Hasher, _ rustfmt.Context) {
			rusthashmap.HashU64(h, value.id)
		}, Equal: func(a, b *bagContextValue, _ rustfmt.Context) bool { return a.id == b.id }, Drop: r.dropValue}
		values := rusthashmap.ValueOps[bagContextValue]{Drop: r.dropValue}
		rustcserialize.WithRuntimeContext(reader, c, func(reader *bagContextReader) struct{} {
			switch r.spec.Action {
			case "decode_vec":
				v := rustcserialize.DecodeVector(reader, rustcserialize.VectorDecodeOps[bagContextValue, *rustcindex.IndexVec[uint, bagContextValue]]{Element: key, Collect: func(iterator rustvec.Iterator[bagContextValue], c rustfmt.Context) *rustcindex.IndexVec[uint, bagContextValue] {
					return collectDecodedVector(iterator, config, c)
				}})
				r.event("decoded:%d", v.Len())
				v.CloseWithRuntimeContext(c)
			case "decode_bag":
				v := DecodeUnordBag(reader, config, key, nil)
				r.event("decoded:%d", v.Len())
				v.Close(c)
			case "decode_map":
				v := DecodeUnordMap(reader, UnordMapConfig[bagContextValue, bagContextValue]{Layout: rusthashmap.Layout{Size: 48, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: keys, Values: values}, key, key)
				r.event("decoded:%d", v.Len())
				v.Close(c)
			case "decode_set":
				v := DecodeUnordSet(reader, UnordSetConfig[bagContextValue]{Layout: rusthashmap.Layout{Size: 24, Align: 8}, Profile: rusthashmap.DefaultProfile(), Values: keys}, key)
				r.event("decoded:%d", v.Len())
				v.Close(c)
			default:
				panic("unknown source bag context action")
			}
			return struct{}{}
		})
	}()
	position := reader.next
	if r.spec.Action == "decode_bytes" {
		position = int(opaque.Position())
	}
	r.event("caught:%s:%d:%t", hex.EncodeToString([]byte(message)), position, c.Panicking())
	moved := r.moved
	r.moved = nil
	for _, token := range moved {
		r.dropToken(token, c)
	}
}
func (r *bagContextRunner) run() {
	c := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "B"}}
	cleanupLocation := r.locations["panic in a destructor during cleanup"]
	cleanup := rustfmt.SourcePanicInfo{Payload: "panic in a destructor during cleanup", Kind: rustfmt.PanicStaticStr, Location: &cleanupLocation, CanUnwind: false, ForceNoBacktrace: true}
	c = rustfmt.WithPanicOperations(c, &rustfmt.PanicOperations{CleanupAbort: &cleanup, Locations: map[string]rustfmt.SourceLocation{rustcserialize.MemDecoderExhaustedPanicSite: r.locations["MemDecoder exhausted"]}, Hook: func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
		location := rustfmt.SourceLocation{}
		if info.Location != nil {
			location = *info.Location
		}
		r.event("HOOK:%s:%s:%t:%t:%t:%s:%d:%d", info.Kind, hex.EncodeToString([]byte(fmt.Sprint(info.Payload))), c.Panicking(), info.CanUnwind, info.ForceNoBacktrace, hex.EncodeToString([]byte(location.File)), location.Line, location.Column)
		r.event("hook_local:%t", c.Panicking())
		if r.spec.Mode == "processing" {
			r.body(c)
		}
	}})
	if r.spec.Mode == "normal" {
		r.body(c)
		return
	}
	outer := r.locations["outer."+r.spec.Mode]
	func() {
		defer func() {
			if recover() != nil && r.spec.Mode == "unwind" {
				unwind := c
				unwind.Unwinding = true
				r.body(unwind)
			}
		}()
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: &outer, CanUnwind: true})
	}()
	r.event("outer_end:false")
}
func bagContextDigest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func bagLoadContextManifest(t *testing.T) bagContextManifest {
	t.Helper()
	b, e := os.ReadFile("../../../../../tools/port-rust-unord/bag/context_manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m bagContextManifest
	if e = json.Unmarshal(b, &m); e != nil {
		t.Fatal(e)
	}
	for p, h := range m.Files {
		b, e := os.ReadFile("../../../../../" + p)
		if e != nil || bagContextDigest(b) != h {
			t.Fatalf("bag context source binding %s: %v", p, e)
		}
	}
	return m
}
func TestOriginalUnordBagContext(t *testing.T) {
	m := bagLoadContextManifest(t)
	b, e := os.ReadFile("testdata/unord_bag_context.jsonl.gz")
	if e != nil || bagContextDigest(b) != m.Fixture {
		t.Fatal("bag context fixture", e)
	}
	z, e := gzip.NewReader(bytes.NewReader(b))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := io.ReadAll(io.LimitReader(z, 12<<20))
	if e != nil {
		t.Fatal(e)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if bagContextDigest(raw) != m.Raw {
		t.Fatal("bag context raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	count, aborts := 0, 0
	for scanner.Scan() {
		var c bagContextCase
		if e = json.Unmarshal(scanner.Bytes(), &c); e != nil {
			t.Fatal(e)
		}
		got := ""
		if c.Outcome == "abort" {
			aborts++
			data, _ := json.Marshal(c)
			command := exec.Command(os.Args[0], "-test.run=^TestUnordBagContextChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_BAG_CONTEXT="+string(data))
			out, e := command.Output()
			exit, ok := e.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("bag context %d abort %v: %s", count, e, out)
			}
			got = string(out)
		} else {
			var out strings.Builder
			runner := bagContextRunner{spec: c, locations: m.Locations, writer: &out}
			runner.run()
			got = out.String()
		}
		if got != c.Stdout {
			t.Fatalf("bag context %d %#v\nGOT:\n%sWANT:\n%s", count, c, got, c.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 1872 || m.Cases != 1872 || aborts != 602 || m.Aborts != 602 {
		t.Fatal("bag context coverage", count, aborts, scanner.Err())
	}
}
func TestUnordBagContextChild(t *testing.T) {
	value := os.Getenv("FERRUGO_UNORD_BAG_CONTEXT")
	if value == "" {
		return
	}
	m := bagLoadContextManifest(t)
	var c bagContextCase
	if e := json.Unmarshal([]byte(value), &c); e != nil {
		t.Fatal(e)
	}
	runner := bagContextRunner{spec: c, locations: m.Locations, writer: os.Stdout}
	runner.run()
	os.Exit(0)
}
