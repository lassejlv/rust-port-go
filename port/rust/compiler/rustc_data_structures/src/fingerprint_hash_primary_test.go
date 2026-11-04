// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures_test

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	hashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type fullHasherRequest struct {
	Kind, Hasher, Context, Adapter string
	First, Second, Seed            uint64
	Fail                           int
	Debug                          bool
}
type fullHasherObservation struct {
	Result  json.RawMessage `json:"result,omitempty"`
	Events  []string        `json:"events"`
	Aborted bool            `json:"aborted,omitempty"`
}
type fullHasherRecord struct {
	Request fullHasherRequest     `json:"request"`
	Want    fullHasherObservation `json:"want"`
}

// This concrete state is deliberately noncomparable and is borrowed through
// Ref.Value. It represents the source caller's actual mutable H cell.
type fullHasherState struct {
	bytes    []byte
	value    uint64
	calls    int
	expected *fullHasherState
	fail     int
	emit     func(string)
}

func fullHasherPanic(call func()) (value any) {
	defer func() { value = recover() }()
	call()
	return nil
}
func (s *fullHasherState) write(ref stablehash.Ref[fullHasherState], bytes []byte) {
	s.emit(fmt.Sprintf("bytes:%x:%t:%t", bytes, ref.Context.Panicking(), s == s.expected))
	s.apply(ref, func() {
		for _, b := range bytes {
			s.value = s.value<<5 | s.value>>59
			s.value ^= uint64(b)
		}
		s.bytes = append(s.bytes, bytes...)
	})
}
func (s *fullHasherState) typed(ref stablehash.Ref[fullHasherState], value uint64) {
	s.emit(fmt.Sprintf("u64:%d:%t:%t", value, ref.Context.Panicking(), s == s.expected))
	s.apply(ref, func() { s.value = (s.value<<9 | s.value>>55) ^ value ^ 0xa55aa55aa55aa55a })
}
func (s *fullHasherState) apply(ref stablehash.Ref[fullHasherState], call func()) {
	n := s.calls
	s.calls++
	defer func() {
		p := recover()
		ctx := ref.Context
		ctx.Unwinding = ctx.Unwinding || p != nil
		s.emit(fmt.Sprintf("local-drop:%d:%t", n, ctx.Panicking()))
		if p != nil {
			panic(p)
		}
	}()
	if n == s.fail {
		ref.Context.Panic("write")
	}
	call()
}

type fullHasherRaw struct {
	state *fullHasherState
	ctx   rustfmt.Context
}

func (h fullHasherRaw) Write(bytes []byte) {
	h.state.write(stablehash.Ref[fullHasherState]{Value: h.state, Context: h.ctx}, bytes)
}
func (h fullHasherRaw) Finish64() uint64 {
	h.state.emit(fmt.Sprintf("finish:%t", h.ctx.Panicking()))
	return h.state.value
}

type fullHasherTyped struct{ fullHasherRaw }

func (h fullHasherTyped) WriteU64(value uint64) {
	h.state.typed(stablehash.Ref[fullHasherState]{Value: h.state, Context: h.ctx}, value)
}

func fullHasherValue(q fullHasherRequest, h stablehash.Hasher) {
	switch q.Kind {
	case "hash64":
		hashes.NewHash64(q.First).HashWithHasher(h)
	case "hash128":
		hashes.NewHash128(hashes.Uint128{Lo: q.First, Hi: q.Second}).HashWithHasher(h)
	case "fingerprint":
		ds.FingerprintFromU64(q.First, q.Second).HashWithHasher(h)
	case "packed":
		ds.PackFingerprint(ds.FingerprintFromU64(q.First, q.Second)).HashWithHasher(h)
	case "hash64_slice":
		hashes.Hash64SliceWithHasher([]hashes.Hash64{hashes.NewHash64(q.First), hashes.NewHash64(q.Second)}, h)
	case "hash128_slice":
		hashes.Hash128SliceWithHasher([]hashes.Hash128{hashes.NewHash128(hashes.Uint128{Lo: q.First, Hi: q.Second}), hashes.NewHash128(hashes.Uint128{Lo: q.Second, Hi: q.First})}, h)
	case "fingerprint_slice":
		ds.FingerprintSliceHashWithHasher([]ds.Fingerprint{ds.FingerprintFromU64(q.First, q.Second), ds.FingerprintFromU64(q.Second, q.First)}, h)
	case "packed_slice":
		ds.PackedFingerprintSliceHashWithHasher([]ds.PackedFingerprint{ds.PackFingerprint(ds.FingerprintFromU64(q.First, q.Second)), ds.PackFingerprint(ds.FingerprintFromU64(q.Second, q.First))}, h)
	case "hash64_empty":
		hashes.Hash64SliceWithHasher(nil, h)
	case "hash128_empty":
		hashes.Hash128SliceWithHasher(nil, h)
	case "fingerprint_empty":
		ds.FingerprintSliceHashWithHasher(nil, h)
	case "packed_empty":
		ds.PackedFingerprintSliceHashWithHasher(nil, h)
	default:
		panic("unknown value")
	}
}

func fullHasherRefValue[H, Output any](q fullHasherRequest, h stablehash.HasherRef[H, Output]) {
	switch q.Kind {
	case "fingerprint":
		ds.FingerprintHashWithHasherRef(ds.FingerprintFromU64(q.First, q.Second), h)
	case "packed":
		ds.PackedFingerprintHashWithHasherRef(ds.PackFingerprint(ds.FingerprintFromU64(q.First, q.Second)), h)
	case "fingerprint_slice":
		ds.FingerprintSliceHashWithHasherRef([]ds.Fingerprint{ds.FingerprintFromU64(q.First, q.Second), ds.FingerprintFromU64(q.Second, q.First)}, h)
	case "packed_slice":
		ds.PackedFingerprintSliceHashWithHasherRef([]ds.PackedFingerprint{ds.PackFingerprint(ds.FingerprintFromU64(q.First, q.Second)), ds.PackFingerprint(ds.FingerprintFromU64(q.Second, q.First))}, h)
	case "fingerprint_empty":
		ds.FingerprintSliceHashWithHasherRef(nil, h)
	case "packed_empty":
		ds.PackedFingerprintSliceHashWithHasherRef(nil, h)
	default:
		fullHasherValue(q, h)
	}
}

func fullHasherCustom(q fullHasherRequest, ctx rustfmt.Context, emit func(string)) any {
	state := fullHasherState{bytes: []byte{0xfe, 0xed}, value: q.Seed, fail: q.Fail, emit: emit}
	state.expected = &state
	ops := stablehash.ExtendedHasherOps[fullHasherState, struct{}]{
		Write: func(ref stablehash.Ref[fullHasherState], bytes []byte) { ref.Value.write(ref, bytes) },
		Finish64: func(ref stablehash.Ref[fullHasherState]) uint64 {
			emit(fmt.Sprintf("finish:%t", ref.Context.Panicking()))
			return ref.Value.value
		},
	}
	if q.Hasher == "typed" {
		ops.WriteU64 = func(ref stablehash.Ref[fullHasherState], n uint64) { ref.Value.typed(ref, n) }
	}
	h := ops.Bind(stablehash.Ref[fullHasherState]{Value: &state, Context: ctx})
	var complete stablehash.Hasher = h
	if q.Adapter == "direct" {
		raw := fullHasherRaw{state: &state, ctx: ctx}
		complete = raw
		if q.Hasher == "typed" {
			complete = fullHasherTyped{raw}
		}
	}
	p := fullHasherPanic(func() {
		if q.Adapter == "direct" {
			fullHasherValue(q, complete)
			return
		}
		fullHasherRefValue(q, h)
	})
	emit(fmt.Sprintf("after-hash:%t", ctx.Panicking()))
	var failure any
	if p != nil {
		failure = fmt.Sprint(p)
	}
	result := map[string]any{"panic": failure, "value": complete.Finish64(), "calls": state.calls, "bytes": hex.EncodeToString(state.bytes)}
	emit(fmt.Sprintf("state-drop:%d:%d:%t", state.calls, state.value, ctx.Panicking()))
	return result
}

// A distinct source Hasher type must not inherit Fingerprint's specialization.
type fullHasherUnhashWrapper struct{ *ds.Unhasher }

func fullHasherUnhash(q fullHasherRequest, ctx rustfmt.Context, emit func(string)) any {
	h := ds.NewUnhasherWithContext(ctx, q.Debug)
	h.WriteU64(q.Seed)
	p := fullHasherPanic(func() {
		if q.Hasher == "unhash" && q.Adapter == "dictionary" {
			op := stablehash.ExtendedHasherOps[ds.Unhasher, struct{}]{
				Write: func(r stablehash.Ref[ds.Unhasher], b []byte) {
					r.Value.SetRuntimeContext(r.Context)
					r.Value.Write(b)
				},
				WriteU64: func(r stablehash.Ref[ds.Unhasher], n uint64) {
					r.Value.SetRuntimeContext(r.Context)
					r.Value.WriteU64(n)
				},
				Finish64: func(r stablehash.Ref[ds.Unhasher]) uint64 { return r.Value.Finish64() },
			}
			r := op.Bind(stablehash.Ref[ds.Unhasher]{Value: h, Context: ctx})
			fullHasherRefValue(q, r)
			return
		}
		var hasher stablehash.Hasher = h
		if q.Hasher == "wrapper_unhash" {
			hasher = fullHasherUnhashWrapper{h}
		}
		if q.Hasher == "ref_unhash" {
			// The actual concrete H here is a source &mut Unhasher, rather
			// than Unhasher. Its exclusive cell therefore has type **Unhasher.
			op := stablehash.ExtendedHasherOps[*ds.Unhasher, struct{}]{
				Write: func(r stablehash.Ref[*ds.Unhasher], b []byte) { (*r.Value).Write(b) },
				WriteU64: func(r stablehash.Ref[*ds.Unhasher], n uint64) {
					(*r.Value).SetRuntimeContext(r.Context)
					(*r.Value).WriteU64(n)
				},
				Finish64: func(r stablehash.Ref[*ds.Unhasher]) uint64 { return (*r.Value).Finish64() },
			}
			r := op.Bind(stablehash.Ref[*ds.Unhasher]{Value: &h, Context: ctx})
			fullHasherRefValue(q, r)
			return
		}
		fullHasherValue(q, hasher)
	})
	emit(fmt.Sprintf("after-hash:%t", ctx.Panicking()))
	var failure any
	if p != nil {
		failure = fmt.Sprint(p)
	}
	return map[string]any{"panic": failure, "value": h.Finish64()}
}

func runFullHasher(q fullHasherRequest, live bool) fullHasherObservation {
	ctx := rustfmt.Context{Unwinding: q.Context == "inherited", ProcessingPanic: q.Context == "processing"}
	got := fullHasherObservation{Events: []string{}}
	emit := func(text string) {
		got.Events = append(got.Events, text)
		if live {
			fmt.Println("E:" + text)
		}
	}
	emit(fmt.Sprintf("enter:%t", ctx.Panicking()))
	var result any
	if q.Hasher == "raw" || q.Hasher == "typed" {
		result = fullHasherCustom(q, ctx, emit)
	} else {
		result = fullHasherUnhash(q, ctx, emit)
	}
	var err error
	got.Result, err = json.Marshal(result)
	if err != nil {
		panic(err)
	}
	return got
}

func TestFullHasherContractAbortHelper(t *testing.T) {
	value, found := os.LookupEnv("FERRUGO_HASH_CONTRACT_CHILD")
	if !found {
		t.Skip("isolated source panic-processing witness")
	}
	var q fullHasherRequest
	if err := json.Unmarshal([]byte(value), &q); err != nil {
		t.Fatal(err)
	}
	got := runFullHasher(q, true)
	encoded, _ := json.Marshal(got)
	fmt.Println("R:" + string(encoded))
}

func TestFullHasherContract(t *testing.T) {
	var endian [2]byte
	binary.NativeEndian.PutUint16(endian[:], 1)
	if endian[0] != 1 {
		t.Skip("retained native raw-write fixtures use little-endian Rust")
	}
	file, err := os.Open("testdata/fingerprint_hash_contract.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	count := 0
	for scanner.Scan() {
		var row fullHasherRecord
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		var got fullHasherObservation
		if row.Want.Aborted {
			binary, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(row.Request)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, binary, "-test.run=^TestFullHasherContractAbortHelper$")
			cmd.Env = append(os.Environ(), "FERRUGO_HASH_CONTRACT_CHILD="+string(encoded))
			out, err := cmd.Output()
			cancel()
			exit, failed := err.(*exec.ExitError)
			if !failed || exit.ExitCode() != 134 {
				t.Fatalf("row%d processing panic must abort134, got%v: %s", count, err, out)
			}
			got.Aborted = true
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "E:") {
					got.Events = append(got.Events, strings.TrimPrefix(line, "E:"))
				}
			}
		} else {
			got = runFullHasher(row.Request, false)
		}
		var actual, want any
		a, _ := json.Marshal(got)
		b, _ := json.Marshal(row.Want)
		// RawMessage values can differ in object field order; compare exact
		// integer lexemes using UseNumber rather than lossy float64 decoding.
		d := json.NewDecoder(strings.NewReader(string(a)))
		d.UseNumber()
		if err := d.Decode(&actual); err != nil {
			t.Fatal(err)
		}
		d = json.NewDecoder(strings.NewReader(string(b)))
		d.UseNumber()
		if err := d.Decode(&want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, want) {
			t.Fatalf("row%d request%+v\ngot %s\nwant%s", count, row.Request, a, b)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count < 500 {
		t.Fatalf("incomplete native corpus: %d", count)
	}
}
