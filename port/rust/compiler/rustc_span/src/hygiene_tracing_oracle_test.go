// SPDX-License-Identifier: MIT OR Apache-2.0
// Fresh-process observation client for the complete original hygiene tracing sites.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	ds "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	hashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	serialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type hygieneTraceOracleLog struct {
	mu     sync.Mutex
	events []string
}

func (log *hygieneTraceOracleLog) add(format string, args ...any) {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.events = append(log.events, fmt.Sprintf(format, args...))
}
func hygieneTraceOracleMeta(m *rusttracing.Metadata) string {
	kind := "event"
	if m.Kind == rusttracing.SpanKind {
		kind = "span"
	}
	level := []string{"OFF", "ERROR", "WARN", "INFO", "DEBUG", "TRACE"}[m.Level]
	return fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s|%s", m.Name, level, m.Target, m.ModulePath, m.File, m.Line, kind, strings.Join(m.Fields, ","))
}

type hygieneTraceOracleSink struct {
	out     strings.Builder
	calls   []string
	n, fail int
}

func (s *hygieneTraceOracleSink) Write(p []byte) (int, error) { return s.WriteString(string(p)) }
func (s *hygieneTraceOracleSink) WriteString(value string) (int, error) {
	s.n++
	s.calls = append(s.calls, "S:"+hex.EncodeToString([]byte(value)))
	if s.n == s.fail {
		return 0, fmt.Errorf("sink failure")
	}
	return s.out.WriteString(value)
}
func (s *hygieneTraceOracleSink) WriteChar(value rune) error {
	s.n++
	s.calls = append(s.calls, fmt.Sprintf("C:%x", value))
	if s.n == s.fail {
		return fmt.Errorf("sink failure")
	}
	_, err := s.out.WriteRune(value)
	return err
}
func hygieneTraceOracleOptions(style int) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	switch style {
	case 0:
	case 1:
		o.Alternate = true
	case 2:
		o.Fill = '*'
		o.Align = rustfmt.AlignCenter
		o.HasWidth = true
		o.Width = 12
		o.HasPrecision = true
		o.Precision = 3
	case 3:
		o.Alternate = true
		o.DebugLowerHex = true
	default:
		panic("trace style")
	}
	return o
}

type hygieneTraceOracleVisitor struct {
	log         *hygieneTraceOracleLog
	style, fail int
}

func (v *hygieneTraceOracleVisitor) RecordDebug(field *rusttracing.Field, value rusttracing.DebugValue, c rusttracing.Context) {
	sink := &hygieneTraceOracleSink{fail: v.fail}
	f := &rustfmt.Formatter{Writer: sink, Options: hygieneTraceOracleOptions(v.style), Context: c.Panic}
	err := value.WriteDebugFormatter(f)
	v.log.add("debug|%s|%t|%t|%s|%s", field.Name, c.Panic.Panicking(), err == nil, hex.EncodeToString([]byte(sink.out.String())), strings.Join(sink.calls, ","))
}
func (v *hygieneTraceOracleVisitor) RecordBool(f *rusttracing.Field, value bool, c rusttracing.Context) {
	v.log.add("bool|%s|%t|%t", f.Name, value, c.Panic.Panicking())
}
func (v *hygieneTraceOracleVisitor) RecordI64(f *rusttracing.Field, value int64, c rusttracing.Context) {
	v.log.add("i64|%s|%d|%t", f.Name, value, c.Panic.Panicking())
}
func (v *hygieneTraceOracleVisitor) RecordU64(f *rusttracing.Field, value uint64, c rusttracing.Context) {
	v.log.add("u64|%s|%d|%t", f.Name, value, c.Panic.Panicking())
}
func (v *hygieneTraceOracleVisitor) RecordStr(f *rusttracing.Field, value string, c rusttracing.Context) {
	v.log.add("str|%s|%x|%t", f.Name, value, c.Panic.Panicking())
}

type hygieneTraceOracleSubscriber struct {
	cell        *HygieneEncodeCell
	log         *hygieneTraceOracleLog
	interest    string
	enabled     bool
	max         rusttracing.Level
	mu          sync.Mutex
	next        uint64
	style, fail int
}

func (s *hygieneTraceOracleSubscriber) visitor() *hygieneTraceOracleVisitor {
	return &hygieneTraceOracleVisitor{s.log, s.style, s.fail}
}
func hygieneTraceOracleSubscriberOps() rusttracing.SubscriberOps[hygieneTraceOracleSubscriber] {
	return rusttracing.SubscriberOps[hygieneTraceOracleSubscriber]{
		RegisterCallsite: func(s *hygieneTraceOracleSubscriber, m *rusttracing.Metadata, c rusttracing.Context) rusttracing.Interest {
			s.log.add("register|%s|%t", hygieneTraceOracleMeta(m), c.Panic.Panicking())
			switch s.interest {
			case "always":
				return rusttracing.Always
			case "never":
				return rusttracing.Never
			default:
				return rusttracing.Sometimes
			}
		},
		Enabled: func(s *hygieneTraceOracleSubscriber, m *rusttracing.Metadata, c rusttracing.Context) bool {
			s.log.add("enabled|%s|%t", hygieneTraceOracleMeta(m), c.Panic.Panicking())
			return s.enabled
		},
		MaxLevelHint: func(s *hygieneTraceOracleSubscriber, c rusttracing.Context) (rusttracing.Level, bool) {
			s.log.add("max|%s|%t", []string{"off", "error", "warn", "info", "debug", "trace"}[s.max], c.Panic.Panicking())
			return s.max, true
		},
		NewSpan: func(s *hygieneTraceOracleSubscriber, a *rusttracing.Attributes, c rusttracing.Context) rusttracing.ID {
			s.mu.Lock()
			id := s.next
			s.next++
			s.mu.Unlock()
			s.log.add("new|%d|%s|%t", id, hygieneTraceOracleMeta(a.Metadata), c.Panic.Panicking())
			a.Record(s.visitor(), c)
			return rusttracing.NewIDWithContext(id, c.Panic)
		},
		Record: func(s *hygieneTraceOracleSubscriber, id rusttracing.ID, values []rusttracing.FieldValue, c rusttracing.Context) {
			s.log.add("record|%d|%t", id.AsU64(), c.Panic.Panicking())
			for _, value := range values {
				if value.Value != nil && value.Field != nil {
					value.Value.Record(value.Field, s.visitor(), c)
				}
			}
		},
		RecordFollowsFrom: func(s *hygieneTraceOracleSubscriber, id, f rusttracing.ID, c rusttracing.Context) {
			s.log.add("follows|%d|%d|%t", id.AsU64(), f.AsU64(), c.Panic.Panicking())
		},
		EventEnabled: func(s *hygieneTraceOracleSubscriber, e *rusttracing.EventRecord, c rusttracing.Context) bool {
			s.log.add("event_enabled|%s|%t", hygieneTraceOracleMeta(e.Metadata), c.Panic.Panicking())
			return true
		},
		Event: func(s *hygieneTraceOracleSubscriber, e *rusttracing.EventRecord, c rusttracing.Context) {
			s.log.add("event|%s|%t", hygieneTraceOracleMeta(e.Metadata), c.Panic.Panicking())
			if s.cell != nil {
				failed := false
				func() {
					defer func() {
						if recover() != nil {
							failed = true
						}
					}()
					g := s.cell.BorrowMutWithContext(c.Panic)
					g.Close()
				}()
				s.log.add("borrow|%t", failed)
			}
			e.Record(s.visitor(), c)
		},
		Enter: func(s *hygieneTraceOracleSubscriber, id rusttracing.ID, c rusttracing.Context) {
			s.log.add("enter|%d|%t", id.AsU64(), c.Panic.Panicking())
		},
		Exit: func(s *hygieneTraceOracleSubscriber, id rusttracing.ID, c rusttracing.Context) {
			s.log.add("exit|%d|%t", id.AsU64(), c.Panic.Panicking())
		},
		CloneSpan: func(s *hygieneTraceOracleSubscriber, id rusttracing.ID, c rusttracing.Context) rusttracing.ID {
			s.log.add("clone|%d|%t", id.AsU64(), c.Panic.Panicking())
			return id
		},
		TryClose: func(s *hygieneTraceOracleSubscriber, id rusttracing.ID, c rusttracing.Context) bool {
			s.log.add("close|%d|%t", id.AsU64(), c.Panic.Panicking())
			return true
		},
	}
}

type hygieneTraceOracleHcx struct{}

func (hygieneTraceOracleHcx) StableHashSpan(s ds.RawSpan, h *ds.StableHasher) { h.WriteU64(s.Value) }
func (hygieneTraceOracleHcx) DefPathHash(id ds.RawDefId) ds.Fingerprint {
	return ds.NewFingerprint(hashes.NewHash64(uint64(id.Crate)+100), hashes.NewHash64(uint64(id.Index)+1000))
}
func (hygieneTraceOracleHcx) StableHashControls() ds.StableHashControls {
	return ds.StableHashControls{HashSpans: true}
}
func (hygieneTraceOracleHcx) AssertDefaultStableHashControls(string) {}
func hygieneTraceOracleRun(session *SessionGlobals, op int, log *hygieneTraceOracleLog, state *hygieneTraceOracleSubscriber, ctx rustfmt.Context) {
	base := DefaultExpnData(ExpnKind{Tag: ExpnMacro, MacroKind: BangMacro, Name: InternSymbol(session, "traced🦀")}, DummySpan(), Edition2024, None[DefId](), None[ModId]())
	a := FreshLocalExpnIdWithContext(session, base.CloneWithContext(ctx), hygieneTraceOracleHcx{}, ctx).ToExpnId()
	FreshLocalExpnIdWithContext(session, base, hygieneTraceOracleHcx{}, ctx)
	ca := RootSyntaxContext().ApplyMarkWithContext(session, a, Opaque, ctx)
	call := NewSpan(session, 11, 19, ca, None[LocalDefId]())
	data := NewExpnData(ExpnKind{Tag: ExpnMacro, MacroKind: AttrMacro, Name: InternSymbol(session, "nested")}, a, call, DummySpan(), None[Shared[[]Symbol]](), Edition2021, None[DefId](), None[ModId](), false, false, true, false)
	b := FreshLocalExpnIdWithContext(session, data, hygieneTraceOracleHcx{}, ctx).ToExpnId()
	cb := ca.ApplyMarkWithContext(session, b, Transparent, ctx)
	if op == 0 || op == 3 {
		cb.MarksWithContext(session, ctx)
		WalkChainWithContext(session, NewSpan(session, 40, 50, cb, None[LocalDefId]()), RootSyntaxContext(), ctx)
		WalkChainCollapsedWithContext(session, NewSpan(session, 40, 50, cb, None[LocalDefId]()), DummySpan(), ctx)
	}
	if op == 1 || op == 3 {
		cell := NewHygieneEncodeCell()
		state.cell = cell
		g := cell.BorrowMut()
		g.Get().EnqueueSyntaxContextForEncodingWithContext(cb, ctx)
		g.Get().ScheduleExpnDataForEncodingWithContext(b, ctx)
		g.Close()
		EncodeHygieneOwned(session, cell, new(struct{}), HygieneContextEncoder[struct{}, struct{}]{Call: func(_ *struct{}, _ *struct{}, _ SyntaxContext, key *SyntaxContextKey, c rustfmt.Context) {
			g := cell.BorrowMutWithContext(c)
			defer g.Close()
			g.Get().EnqueueSyntaxContextForEncodingWithContext(key.Parent, c)
			g.Get().ScheduleExpnDataForEncodingWithContext(key.Expansion, c)
		}}, HygieneExpansionEncoder[struct{}, struct{}]{Call: func(_ *struct{}, _ *struct{}, _ ExpnId, data Option[*ExpnData], _ ExpnHash, c rustfmt.Context) {
			if d, ok := data.Get(); ok {
				g := cell.BorrowMutWithContext(c)
				defer g.Close()
				g.Get().ScheduleExpnDataForEncodingWithContext(d.Parent, c)
				g.Get().EnqueueSyntaxContextForEncodingWithContext(d.CallSite.Ctxt(), c)
			}
		}}, ctx)
		state.cell = nil
		g = cell.BorrowMut()
		g.Get().EnqueueSyntaxContextForEncodingWithContext(cb, ctx)
		g.Close()
		EncodeHygieneOwned(session, cell, new(struct{}), HygieneContextEncoder[struct{}, struct{}]{Call: func(*struct{}, *struct{}, SyntaxContext, *SyntaxContextKey, rustfmt.Context) {}}, HygieneExpansionEncoder[struct{}, struct{}]{Call: func(*struct{}, *struct{}, ExpnId, Option[*ExpnData], ExpnHash, rustfmt.Context) {}}, ctx)
		cell.CloseWithContext(ctx)
	}
	if op == 2 || op == 3 {
		DecodeHygieneExpnIdOwned(session, CrateNumFromU32(1), 0, hashmap.PlainOnce(func(ExpnId, rustfmt.Context) DecodedExpnData { panic("root callback") }), ctx)
		e := serialize.NewMemEncoder()
		e.EmitU32(0)
		d, err := serialize.NewMemDecoder(append(e.Finish(), []byte(serialize.MagicEndBytes)...), 0)
		if err != nil {
			panic(err)
		}
		cache := NewHygieneDecodeContext()
		defer cache.CloseWithContext(ctx)
		DecodeHygieneSyntaxContextOwned(session, d, cache, hashmap.PlainOnce(func(HygieneSyntaxDecodeArgs[*serialize.MemDecoder], rustfmt.Context) SyntaxContextKey {
			panic("root callback")
		}), ctx)
		log.add("read|%d", d.Position())
	}
	log.add("done|%d", op)
}
func TestHygieneTracingNativeChild(t *testing.T) {
	if os.Getenv("FERRUGO_HYGIENE_TRACE_CHILD") != "1" {
		return
	}
	args := strings.Split(os.Getenv("FERRUGO_HYGIENE_TRACE_ARGS"), ",")
	op, _ := strconv.Atoi(args[0])
	mode, _ := strconv.Atoi(args[1])
	config := mode % 8
	interest, max, enabled := "always", rusttracing.Trace, true
	switch config {
	case 1:
		interest = "sometimes"
	case 2:
		interest = "sometimes"
		enabled = false
	case 3:
		interest = "never"
	case 4:
		max = rusttracing.Info
	case 5:
		max = rusttracing.Debug
	case 6:
		max = rusttracing.Off
	case 7:
		enabled = false
	}
	SetDynThreadSafeMode(false)
	session := NewSessionGlobals(Edition2015, nil, None[SourceMapInputs]())
	log := &hygieneTraceOracleLog{}
	owned := rustvec.NewOwned(hygieneTraceOracleSubscriber{log: log, interest: interest, max: max, enabled: enabled, next: 1, style: mode / 8 % 4, fail: []int{0, 1, 3}[mode/32]}, rustvec.ValueOps[hygieneTraceOracleSubscriber]{Drop: func(s *hygieneTraceOracleSubscriber, c rustfmt.Context) {
		s.log.add("subscriber_drop|%t", c.Panicking())
	}})
	// Borrow the actual subscriber cell before moving its owned handle to Dispatch.
	state := owned.Borrow()
	dispatch := rusttracing.NewDispatchOwned(rusttracing.DefaultRuntime(), owned, hygieneTraceOracleSubscriberOps(), rusttracing.Context{})
	scope := rusttracing.NewScope(dispatch)
	ctx := rusttracing.AttachScope(rustfmt.Context{}, scope)
	ctx = WithHygieneTracingProfile(ctx, hygieneTraceOracleSelectedProfile(t))
	hygieneTraceOracleRun(session, op, log, state, ctx)
	scope.Close(rustfmt.Context{})
	dispatch.CloseWithScope(scope, rustfmt.Context{})
	session.Close()
	for _, event := range log.events {
		fmt.Println(event)
	}
	os.Exit(0)
}
func hygieneTraceOracleSelectedProfile(t *testing.T) *HygieneTracingProfile {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	data, err := os.ReadFile(filepath.Join(root, "tools/port-rust-hygiene/tracing-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		MetadataProfile struct {
			Name  string
			Sites map[uint32]rusttracing.Metadata
		} `json:"metadata_profile"`
	}
	if err = json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	return NewHygieneTracingProfile(m.MetadataProfile.Name, m.MetadataProfile.Sites)
}
func TestOriginalHygieneTracing(t *testing.T) {
	file, err := os.Open("testdata/hygiene_tracing.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scan := bufio.NewScanner(z)
	scan.Buffer(make([]byte, 65536), 8<<20)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for scan.Scan() {
		var row struct {
			N, Mode int
			Output  string
		}
		if err = json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestHygieneTracingNativeChild$")
		command.Env = append(os.Environ(), "FERRUGO_HYGIENE_TRACE_CHILD=1", fmt.Sprintf("FERRUGO_HYGIENE_TRACE_ARGS=%d,%d", row.N, row.Mode))
		out, err := command.Output()
		cancel()
		if err != nil {
			t.Fatalf("op%d mode%d: %v %s", row.N, row.Mode, err, out)
		}
		if string(out) != row.Output {
			a, b := strings.Split(string(out), "\n"), strings.Split(row.Output, "\n")
			for i := 0; i < len(a) && i < len(b); i++ {
				if a[i] != b[i] {
					t.Fatalf("op%d mode%d event%d got%q want%q", row.N, row.Mode, i, a[i], b[i])
				}
			}
			t.Fatalf("op%d mode%d event counts%d/%d", row.N, row.Mode, len(a), len(b))
		}
		n++
	}
	if err = scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d whole-source hygiene tracing scripts", n)
}
