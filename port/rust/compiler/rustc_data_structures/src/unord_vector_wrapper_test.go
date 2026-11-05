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
	"path/filepath"
	"strings"
	"testing"

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type vectorWrapperCase struct {
	Mode               string
	Kind, Length, Fail int
	Bag, Resume        bool
	Outcome, Stdout    string
}
type vectorWrapperManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
}
type vectorWrapperRunner struct {
	spec      vectorWrapperCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
	path      string
	adapter   bool
}

func (r *vectorWrapperRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *vectorWrapperRunner) emit(index int, value byte, c rustfmt.Context, write func(byte)) {
	r.event("emit:%d:%d:%t", index, value, c.Panicking())
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			unwind := c
			unwind.Unwinding = true
			r.event("local:%d:%t", index, unwind.Panicking())
			c.ResumeUnwind(p)
		}
		r.event("local:%d:%t", index, c.Panicking())
	}()
	if index == r.spec.Fail {
		if r.spec.Resume {
			c.ResumeUnwind("emit_u8")
		}
		location := r.locations["emit_u8"]
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "emit_u8", Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
	}
	write(value)
}

type vectorWrappedMem struct {
	*rustcserialize.MemEncoder
	runner *vectorWrapperRunner
	calls  int
}

func (e *vectorWrappedMem) EmitU8(value byte) {
	index := e.calls
	e.calls++
	e.runner.emit(index, value, e.RuntimeContext(), e.MemEncoder.EmitU8)
}

type vectorWrappedFile struct {
	*rustcserialize.FileEncoder
	runner *vectorWrapperRunner
	calls  int
}

func (e *vectorWrappedFile) EmitU8(value byte) {
	index := e.calls
	e.calls++
	e.runner.emit(index, value, e.RuntimeContext(), e.FileEncoder.EmitU8)
}
func (r *vectorWrapperRunner) encode(e rustcserialize.Encoder, c rustfmt.Context) {
	values := make([]byte, r.spec.Length)
	for i := range values {
		values[i] = byte((i*73 + 42) % 256)
	}
	ops := rustcserialize.ByteVectorEncodeOps(e)
	if r.adapter {
		ops = rustcserialize.OpaqueByteVectorEncodeOps()
	}
	if r.spec.Bag {
		bag := NewUnordBag(UnordByteBagConfig(rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}), c)
		for _, value := range values {
			bag.Push(value, c)
		}
		owner := rustvec.NewOwned(bag, rustvec.ValueOps[*UnordBag[uint8]]{Drop: func(bag **UnordBag[uint8], c rustfmt.Context) { (*bag).Close(c) }})
		rustvec.ConsumeValue(owner, c, func(*rustvec.Owned[*UnordBag[uint8]]) struct{} {
			bag.Encode(e, ops)
			return struct{}{}
		})
	} else {
		rustcserialize.EncodeVector(e, rustcserialize.VectorSlice[uint8]{Length: uint(len(values)), At: func(index uint) *byte { return &values[index] }, AsGoSlice: func() []byte { return values }}, ops)
	}
}
func (r *vectorWrapperRunner) caught(body func()) (message string) {
	defer func() {
		if p := recover(); p != nil {
			message = fmt.Sprint(p)
		}
	}()
	body()
	return ""
}
func (r *vectorWrapperRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	if r.spec.Kind < 2 {
		inner := rustcserialize.NewMemEncoder()
		inner.SetRuntimeContext(c)
		var e rustcserialize.Encoder = inner
		wrapped := &vectorWrappedMem{MemEncoder: inner, runner: r}
		if r.spec.Kind == 1 || r.adapter {
			e = wrapped
		}
		message = r.caught(func() { r.encode(e, c) })
		position := inner.Position()
		data := inner.Finish()
		r.event("state:mem:%d:%d:%d:%s", wrapped.calls, position, cap(data), hex.EncodeToString(data))
	} else {
		inner, err := rustcserialize.NewFileEncoder(r.path)
		if err != nil {
			panic(err)
		}
		inner.SetRuntimeContext(c)
		var e rustcserialize.Encoder = inner
		wrapped := &vectorWrappedFile{FileEncoder: inner, runner: r}
		if r.spec.Kind == 3 || r.adapter {
			e = wrapped
		}
		message = r.caught(func() { r.encode(e, c) })
		position := inner.Position()
		if _, err = inner.Finish(); err != nil {
			panic(err)
		}
		inner.CloseWithContext(c)
		data, err := os.ReadFile(r.path)
		if err != nil {
			panic(err)
		}
		r.event("state:file:%d:%d:%s", wrapped.calls, position, hex.EncodeToString(data))
	}
	r.event("caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *vectorWrapperRunner) run() {
	c := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
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
func vectorWrapperDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadVectorWrapperManifest(t *testing.T) vectorWrapperManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/codec/wrapper_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata vectorWrapperManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for path, want := range metadata.Files {
		data, err := os.ReadFile("../../../../../" + path)
		if err != nil || vectorWrapperDigest(data) != want {
			t.Fatalf("wrapper source %s: %v", path, err)
		}
	}
	return metadata
}
func TestOriginalUnordVectorWrapper(t *testing.T) {
	metadata := loadVectorWrapperManifest(t)
	data, err := os.ReadFile("testdata/unord_vector_wrapper.jsonl.gz")
	if err != nil || vectorWrapperDigest(data) != metadata.Fixture {
		t.Fatal("wrapper fixture", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 4<<20))
	if err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if vectorWrapperDigest(raw) != metadata.Raw {
		t.Fatal("wrapper raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 128<<10)
	count, aborts := 0, 0
	path := filepath.Join(t.TempDir(), "bytes")
	for scanner.Scan() {
		var spec vectorWrapperCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordVectorWrapperChild$")
			command.Env = append(os.Environ(), "FERRUGO_VECTOR_WRAPPER="+scanner.Text(), "FERRUGO_VECTOR_WRAPPER_PATH="+path, "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("wrapper abort %d: %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := vectorWrapperRunner{spec: spec, locations: metadata.Locations, out: &output, path: path}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("wrapper case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		// These extra Go adapters represent the SAME original concrete source Self,
		// unlike the distinct Wrapper<E> cases. Selection is explicit, never inherited.
		if spec.Kind%2 == 0 {
			var output strings.Builder
			runner := vectorWrapperRunner{spec: spec, locations: metadata.Locations, out: &output, path: path, adapter: true}
			runner.run()
			if output.String() != spec.Stdout {
				t.Fatalf("explicit source-Self strategy case%d\nGOT\n%sWANT\n%s", count, output.String(), spec.Stdout)
			}
		}
		count++
	}
	if scanner.Err() != nil || count != 528 || metadata.Cases != 528 || aborts != 32 || metadata.Aborts != 32 {
		t.Fatal("wrapper coverage", count, aborts, scanner.Err())
	}
}
func TestUnordVectorWrapperChild(t *testing.T) {
	data := os.Getenv("FERRUGO_VECTOR_WRAPPER")
	if data == "" {
		return
	}
	metadata := loadVectorWrapperManifest(t)
	var spec vectorWrapperCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := vectorWrapperRunner{spec: spec, locations: metadata.Locations, out: os.Stdout, path: os.Getenv("FERRUGO_VECTOR_WRAPPER_PATH")}
	runner.run()
	os.Exit(0)
}
func TestByteVectorSpecializationDoesNotPromote(t *testing.T) {
	type alias = rustcserialize.MemEncoder
	original := rustcserialize.NewMemEncoder()
	if rustcserialize.ByteVectorEncodeOps((*alias)(original)).Override == nil {
		t.Fatal("source aliases preserve exact type")
	}
	if rustcserialize.ByteVectorEncodeOps(&vectorWrappedMem{MemEncoder: original}).Override != nil {
		t.Fatal("unrelated Mem wrapper inherited specialization")
	}
	if rustcserialize.ByteVectorEncodeOps(&vectorWrappedFile{}).Override != nil {
		t.Fatal("unrelated File wrapper inherited specialization")
	}
	if rustcserialize.OpaqueByteVectorEncodeOps().Override == nil {
		t.Fatal("explicit actual-source-Self specialization missing")
	}
}
