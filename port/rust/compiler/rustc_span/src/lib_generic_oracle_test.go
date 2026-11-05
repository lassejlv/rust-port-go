package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type libGenericNode struct{ id, calls uint32 }
type libGenericHasher struct{ events *[]string }
type libGenericStableContext struct{ events *[]string }
type libGenericEncoder struct {
	*SpanMemEncoder
	events *[]string
	mode   int
}

func (e *libGenericEncoder) OpaqueContiguousBytes() bool { return false }
func (e *libGenericEncoder) EncodeSpan(span Span) {
	*e.events = append(*e.events, "enc_span")
	if e.mode == 9 {
		e.RuntimeContext().Panic("span encode")
	}
	e.SpanMemEncoder.EncodeSpan(span)
}
func (e *libGenericEncoder) EncodeDefIndex(id DefIndex) {
	*e.events = append(*e.events, fmt.Sprintf("def_index:%d", id.AsU32()))
	if e.mode == 12 {
		e.RuntimeContext().Panic("parent encode")
	}
	e.EmitU32(id.AsU32())
}

type libGenericDecoder struct {
	*SpanMemDecoder
	events *[]string
	mode   int
}
type libGenericOverrideEncoder struct{ *libGenericEncoder }

func (e *libGenericOverrideEncoder) EncodeDefKey(_ Option[DefIndex], value SpanEncodableValue) {
	ctx := e.RuntimeContext()
	defer spanCloseOwnedCallback(ctx, value.CloseWithContext)
	*e.events = append(*e.events, "override:"+value.RustTypeName)
	if e.mode == 13 {
		ctx.Panic("override")
	}
}

func (d *libGenericDecoder) DecodeSpan() Span {
	*d.events = append(*d.events, "dec_span")
	if d.mode == 9 {
		d.RuntimeContext().Panic("span decode")
	}
	return WithRootCtxt(d.session, 7, 9)
}

func (h libGenericStableContext) StableHashSpan(span datastructures.RawSpan, hasher *datastructures.StableHasher) {
	*h.events = append(*h.events, fmt.Sprintf("span:%016x", span.Value))
	hasher.WriteU64(span.Value)
}
func (h libGenericStableContext) DefPathHash(datastructures.RawDefId) datastructures.Fingerprint {
	panic("unexpected def_path_hash")
}
func (h libGenericStableContext) StableHashControls() datastructures.StableHashControls {
	panic("unexpected controls")
}
func (h libGenericStableContext) AssertDefaultStableHashControls(string) {
	panic("unexpected assert controls")
}

func (h libGenericHasher) Write(data []byte) {
	*h.events = append(*h.events, "write:"+hex.EncodeToString(data))
}
func (h libGenericHasher) Finish64() uint64  { *h.events = append(*h.events, "finish"); return 0 }
func (h libGenericHasher) WriteU32(v uint32) { *h.events = append(*h.events, fmt.Sprintf("u32:%d", v)) }
func (h libGenericHasher) WriteU64(v uint64) {
	*h.events = append(*h.events, fmt.Sprintf("u64:%016x", v))
}
func libGenericObserved(session *SessionGlobals, op string, mode, variant int) string {
	events := []string{}
	ctx := rustfmt.Context{}
	event := func(node *libGenericNode, name string, ctx rustfmt.Context) {
		n := node.calls
		node.calls++
		events = append(events, fmt.Sprintf("%s:%d:%d:%t", name, node.id, n, ctx.Panicking()))
	}
	ops := SpannedOps[libGenericNode]{
		Clone: func(node *libGenericNode, ctx rustfmt.Context) libGenericNode {
			event(node, "clone", ctx)
			if mode == 1 {
				ctx.Panic("clone")
			}
			return libGenericNode{id: node.id}
		},
		Drop: func(node *libGenericNode, ctx rustfmt.Context) {
			event(node, "drop", ctx)
			if mode == 6 && node.id == 1 {
				ctx.Panic("drop")
			}
		},
		Equal: func(a, b *libGenericNode, ctx rustfmt.Context) bool {
			event(a, "eq", ctx)
			if mode == 2 {
				ctx.Panic("eq")
			}
			return a.id == b.id
		},
		Hash: func(node *libGenericNode, h stablehash.Hasher, ctx rustfmt.Context) {
			event(node, "hash", ctx)
			if mode == 3 {
				ctx.Panic("hash")
			}
			stablehash.HashU32(h, node.id)
		},
		StableHash: func(node *libGenericNode, _ datastructures.StableHashCtxt, h *datastructures.StableHasher, ctx rustfmt.Context) {
			event(node, "stable", ctx)
			if mode == 5 {
				ctx.Panic("stable")
			}
			h.WriteU32(node.id)
		},
		Encode: func(node *libGenericNode, e SpanEncoder, ctx rustfmt.Context) {
			event(node, "encode", ctx)
			if mode == 4 {
				ctx.Panic("encode")
			}
			e.EmitU32(node.id)
		},
		Debug: func(node *libGenericNode, f *rustfmt.Formatter) error {
			event(node, "debug", f.Context)
			if mode == 7 {
				return libFormatFailure
			}
			if mode == 8 {
				f.Context.Panic("debug")
			}
			return rustfmt.FormatDebugUint(uint64(node.id), 32, f)
		},
	}
	span := WithRootCtxt(session, BytePos(variant), BytePos(variant+3))
	a := Respan(span, libGenericNode{id: 1})
	b := Respan(span, libGenericNode{id: 1})
	if variant%2 != 0 {
		b = DummySpanned(libGenericNode{id: 2})
	}
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		switch op {
		case "clone":
			c := a.CloneWith(ops, ctx)
			events = append(events, fmt.Sprintf("result:%d:%d", c.Node.id, c.Node.calls))
			c.CloseWith(ops, ctx)
		case "clone_from":
			a.CloneFromWith(&b, ops, ctx)
			events = append(events, fmt.Sprintf("result:%d:%d:%t", a.Node.id, a.Node.calls, a.Span == b.Span))
		case "eq":
			events = append(events, fmt.Sprintf("result:%t", a.EqualWith(&b, ops, ctx)))
		case "ne":
			events = append(events, fmt.Sprintf("result:%t", a.NotEqualWith(&b, ops, ctx)))
		case "hash":
			a.HashWith(libGenericHasher{&events}, ops, ctx)
		case "stable":
			h := datastructures.NewStableHasher()
			a.StableHashWith(libGenericStableContext{&events}, h, ops, ctx)
			value := h.FinishHash()
			var raw [16]byte
			binary.LittleEndian.PutUint64(raw[:8], value[0])
			binary.LittleEndian.PutUint64(raw[8:], value[1])
			events = append(events, "result:"+hex.EncodeToString(raw[:]))
		case "encode":
			e := NewSpanMemEncoder(session)
			a.EncodeWith(e, ops)
			events = append(events, "result:"+hex.EncodeToString(e.Finish()))
		case "encode_custom":
			e := &libGenericEncoder{SpanMemEncoder: NewSpanMemEncoder(session), events: &events, mode: mode}
			a.EncodeWith(e, ops)
			events = append(events, "result:"+hex.EncodeToString(e.Finish()))
		case "def_key":
			e := &libGenericEncoder{SpanMemEncoder: NewSpanMemEncoder(session), events: &events, mode: mode}
			value := NewSpanEncodableValue(libGenericNode{id: 3}, "Node", ops.Encode, ops.Drop)
			SpanEncodeDefKey(e, Some(DefIndexFromU32(7)), value)
			events = append(events, "result:"+hex.EncodeToString(e.Finish()))
		case "def_key_override":
			e := &libGenericOverrideEncoder{&libGenericEncoder{SpanMemEncoder: NewSpanMemEncoder(session), events: &events, mode: mode}}
			value := NewSpanEncodableValue(libGenericNode{id: 3}, "native_generic::Node", ops.Encode, ops.Drop)
			SpanEncodeDefKey(e, Some(DefIndexFromU32(7)), value)
			events = append(events, "result:"+hex.EncodeToString(e.Finish()))
		case "decode":
			base, err := NewSpanMemDecoder(session, append([]byte{3}, []byte(rustcserialize.MagicEndBytes)...), 0)
			if err != nil {
				panic(err)
			}
			d := &libGenericDecoder{SpanMemDecoder: base, events: &events, mode: mode}
			c := DecodeSpanned(d, func(d SpanDecoder, ctx rustfmt.Context) libGenericNode {
				events = append(events, fmt.Sprintf("decode:%t", ctx.Panicking()))
				if mode == 10 {
					ctx.Panic("node decode")
				}
				return libGenericNode{id: d.ReadU32()}
			}, ops.Drop)
			events = append(events, fmt.Sprintf("result:%d:%d:%d", c.Node.id, c.Node.calls, d.Position()))
			c.CloseWith(ops, ctx)
		case "debug":
			var text strings.Builder
			f := &rustfmt.Formatter{Writer: &text, Options: rustfmt.DefaultOptions(), Context: ctx}
			err := a.WriteDebugFormatterWith(session, f, ops)
			events = append(events, fmt.Sprintf("result:%t:%s", err == nil, hex.EncodeToString([]byte(text.String()))))
		default:
			panic(op)
		}
	}()
	events = append(events, fmt.Sprintf("panic:%t", panicked))
	mode = 0
	a.CloseWith(ops, ctx)
	b.CloseWith(ops, ctx)
	return strings.Join(events, ",")
}
func libSessionObserved(op string, panicCall, panicDrop, trace bool) string {
	return libSessionObservedWithContext(op, panicCall, panicDrop, trace, rustfmt.Context{}, true)
}
func libSessionObservedWithContext(op string, panicCall, panicDrop, trace bool, ctx rustfmt.Context, observeFinal bool) string {
	events := []string{}
	scope := NewSessionScope()
	emit := func(text string) {
		events = append(events, text)
		if trace {
			fmt.Fprintln(os.Stderr, "EVENT:"+text)
		}
	}
	type state struct{ calls uint32 }
	owned := NewSpanOwned(state{}, func(v *state, ctx rustfmt.Context) {
		emit(fmt.Sprintf("callback_drop:%d:%t:%t", v.calls, scope.IsSet(), ctx.Panicking()))
		if panicDrop {
			ctx.Panic("drop")
		}
	})
	call := func(v *state, ctx rustfmt.Context) {
		emit(fmt.Sprintf("call:%d:%t", v.calls, ctx.Panicking()))
		v.calls++
		if panicCall {
			ctx.Panic("callback")
		}
	}
	withScope := func(v *state, _ *SessionScope, ctx rustfmt.Context) struct{} { call(v, ctx); return struct{}{} }
	withSession := func(v *state, _ *SessionGlobals, ctx rustfmt.Context) struct{} { call(v, ctx); return struct{}{} }
	panicked := false
	func() {
		defer func() { panicked = recover() != nil }()
		switch op {
		case "create":
			CreateSessionGlobalsThenOwned(scope, Edition2024, nil, None[SourceMapInputs](), owned, withScope, ctx)
		case "default":
			CreateSessionGlobalsThenOwned(scope, DefaultEdition, nil, None[SourceMapInputs](), owned, withScope, ctx)
		case "with_absent":
			WithSessionGlobalsOwned(scope, owned, withSession, ctx)
		case "if_absent":
			CreateSessionIfNotSetThenOwned(scope, Edition2018, owned, withSession, ctx)
		case "set":
			s := NewSessionGlobals(Edition2021, nil, None[SourceMapInputs]())
			defer s.Close()
			SetSessionGlobalsThenOwned(scope, s, owned, withScope, ctx)
		case "with_present":
			CreateDefaultSessionGlobalsThen(scope, func() struct{} { return WithSessionGlobalsOwned(scope, owned, withSession, ctx) })
		case "if_present":
			CreateDefaultSessionGlobalsThen(scope, func() struct{} { return CreateSessionIfNotSetThenOwned(scope, Edition2015, owned, withSession, ctx) })
		case "create_present":
			CreateDefaultSessionGlobalsThen(scope, func() struct{} {
				return CreateSessionGlobalsThenOwned(scope, Edition2018, nil, None[SourceMapInputs](), owned, withScope, ctx)
			})
		case "set_present":
			CreateDefaultSessionGlobalsThen(scope, func() struct{} {
				s := NewSessionGlobals(Edition2018, nil, None[SourceMapInputs]())
				defer s.Close()
				return SetSessionGlobalsThenOwned(scope, s, owned, withScope, ctx)
			})
		default:
			panic(op)
		}
	}()
	emit(fmt.Sprintf("panic:%t", panicked))
	if observeFinal {
		emit(fmt.Sprintf("final_scope:%t", scope.IsSet()))
	}
	return strings.Join(events, ",")
}
func TestLibGenericAbortChild(t *testing.T) {
	op := os.Getenv("FERRUGO_LIB_GENERIC_ABORT")
	if op == "" {
		t.Skip("isolated source abort witness")
	}
	ctx := rustfmt.Context{}
	ambient := os.Getenv("FERRUGO_LIB_GENERIC_AMBIENT")
	ctx.Unwinding = ambient == "unwinding"
	ctx.ProcessingPanic = ambient == "processing"
	drop := os.Getenv("FERRUGO_LIB_GENERIC_DROP") != "false"
	libSessionObservedWithContext(op, os.Getenv("FERRUGO_LIB_GENERIC_CALL") == "true", drop, true, ctx, ambient != "processing")
	t.Fatal("expected abort")
}
func TestLibOriginalGenericAndSession(t *testing.T) {
	file, err := os.Open("testdata/lib-generic.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	raw, err := io.ReadAll(io.LimitReader(gz, 1<<20+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 1<<20 {
		t.Fatal("generic fixture bound")
	}
	session := NewSessionGlobals(DefaultEdition, nil, None[SourceMapInputs]())
	defer session.Close()
	scan := bufio.NewScanner(bytes.NewReader(raw))
	count := 0
	for scan.Scan() {
		count++
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 5 {
			t.Fatal("generic fixture shape")
		}
		got := ""
		switch p[0] {
		case "generic":
			mode, err := strconv.Atoi(p[2])
			if err != nil {
				t.Fatal(err)
			}
			variant, err := strconv.Atoi(p[3])
			if err != nil {
				t.Fatal(err)
			}
			got = libGenericObserved(session, p[1], mode, variant)
		case "session":
			got = libSessionObserved(p[1], p[2] == "true", p[3] == "true", false)
		case "abort":
			cmd := exec.Command(os.Args[0], "-test.run=^TestLibGenericAbortChild$")
			cmd.Env = append(os.Environ(), "FERRUGO_LIB_GENERIC_ABORT="+p[1], "FERRUGO_LIB_GENERIC_CALL="+p[2])
			output, err := cmd.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("abort %s: %v %s", p[1], err, output)
			}
			events := []string{}
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "EVENT:") {
					events = append(events, strings.TrimPrefix(line, "EVENT:"))
				}
			}
			got = strings.Join(events, ",")
		default:
			t.Fatal(p[0])
		}
		if got != p[4] {
			t.Fatalf("row %d %v\nGo: %s\nRust: %s", count, p[:4], got, p[4])
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 188 {
		t.Fatal(count)
	}
}
