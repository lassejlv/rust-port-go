package datastructures

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
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// These witnesses translate only the oracle's external non-Copy implementation.
// The operation under test is the production ToBaseN dictionary/owned adapter.
type baseNOwnedRequest struct {
	kind, width, fixed, base, hi, lo, mask, disposition, mode uint64
}

func baseNOwnedParse(line string) baseNOwnedRequest {
	p := strings.Split(line, "\t")
	if len(p) != 10 || p[9] != "v1" {
		panic("owned request schema")
	}
	var n [9]uint64
	for i := range n {
		var err error
		n[i], err = strconv.ParseUint(p[i], 10, 64)
		if err != nil {
			panic(err)
		}
	}
	return baseNOwnedRequest{n[0], n[1], n[2], n[3], n[4], n[5], n[6], n[7], n[8]}
}

type baseNOwnedRecord struct{ Profile, Request, Status, Observation string }
type baseNOwnedInput struct {
	counter, a, b uint32
	n             BaseNU128
}
type baseNOwnedState struct {
	request  baseNOwnedRequest
	trace    []string
	snapshot [6]uint32
	cell     *baseNOwnedInput
	stored   *rustvec.Owned[baseNOwnedInput]
	emit     bool
	phase    string
}

func (s *baseNOwnedState) log(ctx rustfmt.Context, message string) {
	scope, ok := ctx.Runtime.Value.(string)
	if !ok {
		panic("missing explicit runtime scope")
	}
	line := fmt.Sprintf("%s:%t:%s", message, ctx.Panicking(), scope)
	s.trace = append(s.trace, line)
	if s.emit {
		fmt.Fprintln(os.Stdout, line)
	}
}
func baseNOwnedCatch(body func()) (p any) { defer func() { p = recover() }(); body(); return nil }
func baseNOwnedAbortOnPanic(body func()) {
	defer func() {
		if recover() != nil {
			os.Exit(134)
		}
	}()
	body()
}

// Ambient Unwinding is independent of a panic raised by this scope. A source
// destructor can catch the latter; a second escaping cleanup panic must abort.
func baseNOwnedScope(ctx rustfmt.Context, body func(), cleanup func(rustfmt.Context)) {
	p := baseNOwnedCatch(body)
	if p != nil {
		ctx.Unwinding = true
		baseNOwnedAbortOnPanic(func() { cleanup(ctx) })
		panic(p)
	}
	cleanup(ctx)
}
func (s *baseNOwnedState) local(kind string, ctx rustfmt.Context) {
	baseNOwnedScope(ctx, func() {
		s.log(ctx, "local:"+kind)
		if s.request.mask&2 != 0 {
			ctx.Panic("local")
		}
	}, func(ctx rustfmt.Context) { s.log(ctx, "local-probe") })
}
func (s *baseNOwnedState) field(value *baseNOwnedInput, which int, ctx rustfmt.Context) {
	n, bit, label := value.a, uint64(8), "a"
	if which == 1 {
		n, bit, label = value.b, 16, "b"
	}
	baseNOwnedScope(ctx, func() {
		s.log(ctx, fmt.Sprintf("field:%s:%d", label, n))
		s.snapshot[1+which] = n
		s.snapshot[4+which]++
		if s.request.mask&bit != 0 {
			ctx.Panic("field")
		}
	}, func(ctx rustfmt.Context) { s.log(ctx, label+"-probe") })
}
func (s *baseNOwnedState) drop(value *baseNOwnedInput, ctx rustfmt.Context) {
	if value != s.cell {
		panic("Drop received copied cell")
	}
	p := baseNOwnedCatch(func() {
		baseNOwnedScope(ctx, func() {
			s.log(ctx, fmt.Sprintf("self:%d:%d:%d", value.counter, value.a, value.b))
			value.counter++
			value.a += 3
			value.b += 5
			s.snapshot[0], s.snapshot[1], s.snapshot[2] = value.counter, value.a, value.b
			s.snapshot[3]++
			if s.request.mask&4 != 0 {
				ctx.Panic("self")
			}
		}, func(ctx rustfmt.Context) { s.log(ctx, "self-probe") })
	})
	if p != nil {
		ctx.Unwinding = true
		// Rust's Self::drop unwind landing pad drops the fields in order; a
		// field panic terminates before reaching the following field.
		baseNOwnedAbortOnPanic(func() { s.field(value, 0, ctx); s.field(value, 1, ctx) })
		panic(p)
	}
	baseNOwnedScope(ctx, func() { s.field(value, 0, ctx) }, func(ctx rustfmt.Context) { s.field(value, 1, ctx) })
}
func (s *baseNOwnedState) enter(value *baseNOwnedInput, kind string, ctx rustfmt.Context) {
	if value != s.cell {
		panic("conversion received copied cell")
	}
	s.log(ctx, fmt.Sprintf("body:%s:%d:%d:%d", kind, value.counter, value.a, value.b))
	value.counter += 10
	value.a += 100
	value.b += 1000
	s.snapshot[0], s.snapshot[1], s.snapshot[2] = value.counter, value.a, value.b
	s.log(ctx, fmt.Sprintf("mutated:%d:%d:%d", value.counter, value.a, value.b))
	if s.request.mask&1 != 0 {
		ctx.Panic("body")
	}
}
func (s *baseNOwnedState) dispose(value *rustvec.Owned[baseNOwnedInput], ctx rustfmt.Context) {
	switch s.request.disposition {
	case 0:
	case 1:
		value.DropWithContext(ctx)
		s.log(ctx, "manual-done")
	case 2:
		_ = value.Take()
		s.log(ctx, "forgot")
	case 3:
		s.stored = value.MoveWithContext(ctx)
		s.log(ctx, "stored")
	default:
		panic("invalid disposition")
	}
}
func (s *baseNOwnedState) trait(config BaseNConfig) BaseNTrait[baseNOwnedInput] {
	implementation := BaseNTrait[baseNOwnedInput]{
		Into128: func(value *rustvec.Owned[baseNOwnedInput], ctx rustfmt.Context) (n BaseNU128) {
			baseNOwnedScope(ctx, func() {
				s.enter(value.Borrow(), "into", ctx)
				n = value.Borrow().n
				s.dispose(value, ctx)
				s.log(ctx, "return:into")
			}, func(ctx rustfmt.Context) { s.local("into", ctx) })
			return
		},
		EncodedLen: func(base uint, ctx rustfmt.Context) uint {
			s.log(ctx, fmt.Sprintf("len:%d:%d:%d:%d:%d", base, s.request.width, s.snapshot[3], s.snapshot[4], s.snapshot[5]))
			if s.request.mask&32 != 0 {
				ctx.Panic("len")
			}
			return uint(s.request.width)
		},
	}
	override := func(kind string, n uint64) func(*rustvec.Owned[baseNOwnedInput], uint, rustfmt.Context) BaseNString {
		return func(value *rustvec.Owned[baseNOwnedInput], base uint, ctx rustfmt.Context) (result BaseNString) {
			baseNOwnedScope(ctx, func() {
				s.enter(value.Borrow(), kind, ctx)
				result = BaseNToBaseOwnedWithContext(rustvec.Own(BaseNU128{Lo: n}), base, BaseNU128OwnedTrait(config), config, ctx)
				s.dispose(value, ctx)
				s.log(ctx, "return:"+kind)
			}, func(ctx rustfmt.Context) { s.local(kind, ctx) })
			return
		}
	}
	if s.request.kind == 1 || s.request.kind == 3 {
		implementation.ToBase = override("base", 42)
	}
	if s.request.kind == 2 || s.request.kind == 3 {
		implementation.ToBaseFixedLen = override("fixed", 99)
	}
	return implementation
}
func baseNOwnedPrimitive[T any](value T, base uint, fixed bool, trait BaseNTrait[T], config BaseNConfig, ctx rustfmt.Context) BaseNString {
	if fixed {
		return BaseNToBaseFixedLenOwnedWithContext(rustvec.Own(value), base, trait, config, ctx)
	}
	return BaseNToBaseOwnedWithContext(rustvec.Own(value), base, trait, config, ctx)
}
func (s *baseNOwnedState) produce(config BaseNConfig, ctx rustfmt.Context) BaseNString {
	r := s.request
	n := BaseNU128{Hi: r.hi, Lo: r.lo}
	base, fixed := uint(r.base), r.fixed != 0
	switch r.kind {
	case 32:
		return baseNOwnedPrimitive(uint32(n.Lo), base, fixed, BaseNU32OwnedTrait(config), config, ctx)
	case 64:
		return baseNOwnedPrimitive(n.Lo, base, fixed, BaseNU64OwnedTrait(config), config, ctx)
	case 128:
		return baseNOwnedPrimitive(n, base, fixed, BaseNU128OwnedTrait(config), config, ctx)
	}
	owned := rustvec.NewOwned(baseNOwnedInput{counter: 7, a: 11, b: 22, n: n}, rustvec.ValueOps[baseNOwnedInput]{Drop: s.drop})
	s.cell = owned.Borrow()
	trait := s.trait(config)
	if fixed {
		return BaseNToBaseFixedLenOwnedWithContext(owned, base, trait, config, ctx)
	}
	return BaseNToBaseOwnedWithContext(owned, base, trait, config, ctx)
}
func (s *baseNOwnedState) execute(config BaseNConfig, ctx rustfmt.Context) string {
	s.phase = "construct"
	value := s.produce(config, ctx)
	s.log(ctx, "constructed")
	s.phase = "deref"
	text := value.DerefWithContext(ctx)
	asRef := value.AsRefBorrowedWithContext(ctx)
	alias := unsafe.StringData(text) == (*byte)(unsafe.Add(unsafe.Pointer(&value.buf[0]), value.start)) && unsafe.StringData(text) == unsafe.StringData(asRef) && unsafe.StringData(text) == unsafe.StringData(value.DerefWithContext(ctx))
	if s.request.kind == 0 && s.request.width == 0 && s.request.mode == 0 {
		runtime.GC() // retain the actual object across a collection, including its empty suffix
	}
	if text != value.DerefWithContext(ctx) {
		panic("borrow invalidated")
	}
	sink := &baseNFormatSink{expected: unsafe.StringData(text), fail: -1}
	f := rustfmt.Formatter{Writer: sink, Context: ctx, Options: rustfmt.DefaultOptions()}
	s.phase = "display"
	if err := value.WriteDisplayFormatter(&f); err != nil {
		panic(err)
	}
	if !alias || len(sink.calls) != 1 || !strings.HasSuffix(sink.calls[0], ":true") {
		panic("lost buffer alias")
	}
	s.log(ctx, "observed")
	return fmt.Sprintf("ok\t%x\t%t\t%x", text, alias, sink.accepted.String())
}
func baseNOwnedReplay(record baseNOwnedRecord, emit bool) string {
	s := baseNOwnedState{request: baseNOwnedParse(record.Request), emit: emit}
	config := BaseNConfig{OverflowChecks: record.Profile[3] == '1'}
	ctx := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "op"}}
	if s.request.mode == 1 || s.request.mode == 2 {
		ctx.Unwinding = true
		s.log(ctx, "outer-drop")
	}
	if s.request.mode == 3 {
		ctx.ProcessingPanic = true
		s.log(ctx, "processing")
	}
	var result string
	p := baseNOwnedCatch(func() { result = s.execute(config, ctx) })
	if p != nil {
		if s.request.mode == 2 || s.request.mode == 3 {
			os.Exit(134)
		}
		result = s.phase + ":panic\t\tfalse\t"
	}
	after := baseNOwnedCatch(func() {
		if s.stored != nil {
			s.stored.DropWithContext(rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "after"}})
		}
	}) == nil
	return fmt.Sprintf("%s\t%t\t%d:%d:%d:%d:%d:%d\t%s", result, after, s.snapshot[0], s.snapshot[1], s.snapshot[2], s.snapshot[3], s.snapshot[4], s.snapshot[5], strings.Join(s.trace, "|"))
}
func TestBaseNOwnedAbortHelper(t *testing.T) {
	encoded, ok := os.LookupEnv("FERRUGO_BASE_N_OWNED_ABORT")
	if !ok {
		t.Skip("isolated source abort replay")
	}
	var record baseNOwnedRecord
	if err := json.Unmarshal([]byte(encoded), &record); err != nil {
		t.Fatal(err)
	}
	baseNOwnedReplay(record, true)
	t.Fatal("expected source abort returned")
}
func TestBaseNOwnedFullSourceContracts(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("fixture is explicit native 64-bit profile; no 32-bit runtime claim")
	}
	manifestBytes, err := os.ReadFile("../../../../../tools/port-rust-base-n/contracts/owned-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Records    int
		FixtureSHA string `json:"fixture_sha256"`
		RawSHA     string `json:"raw_sha256"`
	}
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	compressed, err := os.ReadFile("testdata/base_n_owned.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(compressed)); got != manifest.FixtureSHA {
		t.Fatal("owned fixture checksum differs")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(io.LimitReader(z, 64<<20))
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprintf("%x", sha256.Sum256(raw)); got != manifest.RawSHA {
		t.Fatal("owned raw checksum differs")
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	records := 0
	seenAbort := map[string]bool{}
	for scanner.Scan() {
		var record baseNOwnedRecord
		if err = json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		records++
		if record.Status != "abort" {
			if got := baseNOwnedReplay(record, false); got != record.Observation {
				t.Fatalf("%s %s:\ngot  %q\nwant %q", record.Profile, record.Request, got, record.Observation)
			}
			continue
		}
		// Debug assertions have no source operations in this file. Both checked
		// arithmetic settings are replayed, with duplicate debug rows retained
		// in the native fixture but sharing their isolated Go execution.
		key := record.Profile[2:] + record.Request
		if seenAbort[key] {
			continue
		}
		seenAbort[key] = true
		encoded, _ := json.Marshal(record)
		deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(deadline, binary, "-test.run=^TestBaseNOwnedAbortHelper$")
		command.Env = append(os.Environ(), "FERRUGO_BASE_N_OWNED_ABORT="+string(encoded))
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		err = command.Run()
		cancel()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 134 {
			t.Fatalf("%s %s expected abort134: %v\n%s\n%s", record.Profile, record.Request, err, stdout.String(), stderr.String())
		}
		if got := strings.ReplaceAll(strings.TrimSuffix(stdout.String(), "\n"), "\n", "|"); got != record.Observation {
			t.Fatalf("%s %s abort trace:\ngot  %q\nwant %q", record.Profile, record.Request, got, record.Observation)
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if records != manifest.Records {
		t.Fatalf("owned records %d != %d", records, manifest.Records)
	}
}

func TestBaseNContractsLiveRegeneration(t *testing.T) {
	value, requested := os.LookupEnv("FERRUGO_BASE_N_CONTRACTS")
	if !requested {
		t.Skip("set FERRUGO_BASE_N_CONTRACTS=1 for original-source formatter/owned checks")
	}
	if value != "1" {
		t.Fatal("FERRUGO_BASE_N_CONTRACTS must be 1 when explicitly requested")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../../../")
	if err != nil {
		t.Fatal(err)
	}
	for _, script := range []string{"generate.py", "owned_generate.py"} {
		deadline, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		command := exec.CommandContext(deadline, python, filepath.Join(root, "tools/port-rust-base-n/contracts", script), "--check")
		command.Dir = root
		output, err := command.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("%s: %v\n%s", script, err, output)
		}
	}
}
