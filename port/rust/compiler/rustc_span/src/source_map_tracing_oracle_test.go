// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete original SourceMap calls under a genuine tracing subscriber.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/hex"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type sourceMapTraceLog struct {
	mu     sync.Mutex
	events []string
}

func (log *sourceMapTraceLog) add(format string, args ...any) {
	log.mu.Lock()
	defer log.mu.Unlock()
	log.events = append(log.events, fmt.Sprintf(format, args...))
}
func sourceMapTraceMeta(m *rusttracing.Metadata) string {
	kind := "event"
	if m.Kind == rusttracing.SpanKind {
		kind = "span"
	}
	level := []string{"OFF", "ERROR", "WARN", "INFO", "DEBUG", "TRACE"}[m.Level]
	return fmt.Sprintf("%s|%s|%s|%s|%s|%d|%s|%s", m.Name, level, m.Target, m.ModulePath, m.File, m.Line, kind, strings.Join(m.Fields, ","))
}

type sourceMapTraceSink struct {
	out     strings.Builder
	calls   []string
	n, fail int
}

func (s *sourceMapTraceSink) Write(p []byte) (int, error) { return s.WriteString(string(p)) }
func (s *sourceMapTraceSink) WriteString(value string) (int, error) {
	s.n++
	s.calls = append(s.calls, "S:"+hex.EncodeToString([]byte(value)))
	if s.n == s.fail {
		return 0, fmt.Errorf("sink failure")
	}
	return s.out.WriteString(value)
}
func (s *sourceMapTraceSink) WriteChar(value rune) error {
	s.n++
	s.calls = append(s.calls, fmt.Sprintf("C:%x", value))
	if s.n == s.fail {
		return fmt.Errorf("sink failure")
	}
	_, err := s.out.WriteRune(value)
	return err
}
func sourceMapTraceOptions(style int) rustfmt.Options {
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

type sourceMapTraceVisitor struct {
	log         *sourceMapTraceLog
	style, fail int
}

func (v *sourceMapTraceVisitor) RecordDebug(field *rusttracing.Field, value rusttracing.DebugValue, c rusttracing.Context) {
	sink := &sourceMapTraceSink{fail: v.fail}
	f := &rustfmt.Formatter{Writer: sink, Options: sourceMapTraceOptions(v.style), Context: c.Panic}
	err := value.WriteDebugFormatter(f)
	v.log.add("debug|%s|%t|%t|%s|%s", field.Name, c.Panic.Panicking(), err == nil, hex.EncodeToString([]byte(sink.out.String())), strings.Join(sink.calls, ","))
}
func (v *sourceMapTraceVisitor) RecordBool(f *rusttracing.Field, value bool, c rusttracing.Context) {
	v.log.add("bool|%s|%t|%t", f.Name, value, c.Panic.Panicking())
}
func (v *sourceMapTraceVisitor) RecordI64(f *rusttracing.Field, value int64, c rusttracing.Context) {
	v.log.add("i64|%s|%d|%t", f.Name, value, c.Panic.Panicking())
}
func (v *sourceMapTraceVisitor) RecordU64(f *rusttracing.Field, value uint64, c rusttracing.Context) {
	v.log.add("u64|%s|%d|%t", f.Name, value, c.Panic.Panicking())
}
func (v *sourceMapTraceVisitor) RecordStr(f *rusttracing.Field, value string, c rusttracing.Context) {
	v.log.add("str|%s|%x|%t", f.Name, value, c.Panic.Panicking())
}

type sourceMapTraceSubscriber struct {
	log         *sourceMapTraceLog
	interest    string
	enabled     bool
	max         rusttracing.Level
	mu          sync.Mutex
	next        uint64
	style, fail int
}

func (s *sourceMapTraceSubscriber) visitor() *sourceMapTraceVisitor {
	return &sourceMapTraceVisitor{s.log, s.style, s.fail}
}
func sourceMapTraceSubscriberOps() rusttracing.SubscriberOps[sourceMapTraceSubscriber] {
	return rusttracing.SubscriberOps[sourceMapTraceSubscriber]{
		RegisterCallsite: func(s *sourceMapTraceSubscriber, m *rusttracing.Metadata, c rusttracing.Context) rusttracing.Interest {
			s.log.add("register|%s|%t", sourceMapTraceMeta(m), c.Panic.Panicking())
			switch s.interest {
			case "always":
				return rusttracing.Always
			case "never":
				return rusttracing.Never
			default:
				return rusttracing.Sometimes
			}
		},
		Enabled: func(s *sourceMapTraceSubscriber, m *rusttracing.Metadata, c rusttracing.Context) bool {
			s.log.add("enabled|%s|%t", sourceMapTraceMeta(m), c.Panic.Panicking())
			return s.enabled
		},
		MaxLevelHint: func(s *sourceMapTraceSubscriber, c rusttracing.Context) (rusttracing.Level, bool) {
			s.log.add("max|%s|%t", []string{"off", "error", "warn", "info", "debug", "trace"}[s.max], c.Panic.Panicking())
			return s.max, true
		},
		NewSpan: func(s *sourceMapTraceSubscriber, a *rusttracing.Attributes, c rusttracing.Context) rusttracing.ID {
			s.mu.Lock()
			id := s.next
			s.next++
			s.mu.Unlock()
			s.log.add("new|%d|%s|%t", id, sourceMapTraceMeta(a.Metadata), c.Panic.Panicking())
			a.Record(s.visitor(), c)
			return rusttracing.NewIDWithContext(id, c.Panic)
		},
		Record: func(s *sourceMapTraceSubscriber, id rusttracing.ID, values []rusttracing.FieldValue, c rusttracing.Context) {
			s.log.add("record|%d|%t", id.AsU64(), c.Panic.Panicking())
			for _, value := range values {
				if value.Value != nil && value.Field != nil {
					value.Value.Record(value.Field, s.visitor(), c)
				}
			}
		},
		RecordFollowsFrom: func(s *sourceMapTraceSubscriber, id, f rusttracing.ID, c rusttracing.Context) {
			s.log.add("follows|%d|%d|%t", id.AsU64(), f.AsU64(), c.Panic.Panicking())
		},
		EventEnabled: func(s *sourceMapTraceSubscriber, e *rusttracing.EventRecord, c rusttracing.Context) bool {
			s.log.add("event_enabled|%s|%t", sourceMapTraceMeta(e.Metadata), c.Panic.Panicking())
			return true
		},
		Event: func(s *sourceMapTraceSubscriber, e *rusttracing.EventRecord, c rusttracing.Context) {
			s.log.add("event|%s|%t", sourceMapTraceMeta(e.Metadata), c.Panic.Panicking())
			e.Record(s.visitor(), c)
		},
		Enter: func(s *sourceMapTraceSubscriber, id rusttracing.ID, c rusttracing.Context) {
			s.log.add("enter|%d|%t", id.AsU64(), c.Panic.Panicking())
		},
		Exit: func(s *sourceMapTraceSubscriber, id rusttracing.ID, c rusttracing.Context) {
			s.log.add("exit|%d|%t", id.AsU64(), c.Panic.Panicking())
		},
		CloneSpan: func(s *sourceMapTraceSubscriber, id rusttracing.ID, c rusttracing.Context) rusttracing.ID {
			s.log.add("clone|%d|%t", id.AsU64(), c.Panic.Panicking())
			return id
		},
		TryClose: func(s *sourceMapTraceSubscriber, id rusttracing.ID, c rusttracing.Context) bool {
			s.log.add("close|%d|%t", id.AsU64(), c.Panic.Panicking())
			return true
		},
	}
}

type sourceMapTraceLoader struct{ log *sourceMapTraceLog }

func sourceMapTraceLoaderOps() FileLoaderOps[sourceMapTraceLoader] {
	return FileLoaderOps[sourceMapTraceLoader]{
		FileExists: func(s *sourceMapTraceLoader, _ RustPath, c rustfmt.Context) bool {
			s.log.add("exists|%t", c.Panicking())
			return true
		},
		ReadFile: func(s *sourceMapTraceLoader, _ RustPath, c rustfmt.Context) (string, error) {
			s.log.add("read|%t", c.Panicking())
			return "a…b", nil
		},
		ReadBinaryFile: func(s *sourceMapTraceLoader, _ RustPath, c rustfmt.Context) (Shared[[]byte], error) {
			s.log.add("binary|%t", c.Panicking())
			return NewShared([]byte{'a'}, nil), nil
		},
		CurrentDirectory: func(s *sourceMapTraceLoader, c rustfmt.Context) (RustPath, error) {
			s.log.add("cwd|%t", c.Panicking())
			return NewRustPath("/foo"), nil
		},
	}
}
func sourceMapTraceRun(s *SessionGlobals, op string, log *sourceMapTraceLog, context rustfmt.Context, cfg SpanEncodingConfig) {
	mapping := NewFilePathMapping(sourceMapTestEntries("/foo", "FOO", "/foo/bar", "BAR"), AllRemapPathScopeComponents())
	loader := NewOwnedFileLoaderWithOps(NewSpanOwned(sourceMapTraceLoader{log}, func(s *sourceMapTraceLoader, c rustfmt.Context) { s.log.add("loader_drop|%t", c.Panicking()) }), sourceMapTraceLoaderOps())
	sm := NewSourceMapWithContext(context, SourceMapInputs{FileLoader: loader, PathMapping: mapping, HashKind: SourceFileHashMd5}, cfg)
	defer sm.CloseWithContext(context)
	if op == "construct" {
		return
	}
	text := "a…b"
	if op == "malformed" {
		text = ""
	}
	file := sm.NewSourceFileWithContext(CustomFileName("input.rs"), text, context)
	defer file.CloseWithContext(context)
	sp := WithRootCtxt(s, 1, 4, cfg)
	ok := false
	func() {
		defer func() { recover() }()
		switch op {
		case "valid":
			a, b, e := sm.IsValidSpanWithContext(s, sp, context)
			if e == nil {
				b.File.CloseWithContext(context)
				a.File.CloseWithContext(context)
			}
		case "lines":
			v, e := sm.SpanToLinesWithContext(s, sp, context)
			if e == nil {
				v.File.CloseWithContext(context)
			}
		case "point":
			sm.NextPointWithContext(s, WithRootCtxt(s, 0, 1, cfg), context)
		case "empty":
			sm.EndPointWithContext(s, WithRootCtxt(s, 1, 1, cfg), context)
		case "malformed":
			sm.NextPointWithContext(s, WithRootCtxt(s, 0, 1, cfg), context)
		case "different":
			other := sm.NewSourceFileWithContext(CustomFileName("other.rs"), "z", context)
			defer other.CloseWithContext(context)
			sm.NextPointWithContext(s, WithRootCtxt(s, 1, other.Borrow().StartPos+1, cfg), context)
		case "remap":
			sm.PathMapping().ToRealFilenameWithContext(sm.WorkingDir(), NewRustPath("/foo/bar/input.rs"), context)
		case "remap_miss":
			sm.PathMapping().ToRealFilenameWithContext(sm.WorkingDir(), NewRustPath("/unknown/input.rs"), context)
		case "reverse":
			template, e := NewSourceFileWithContext(CustomFileName("external-template"), "a…b", SourceFileHashMd5, None[SourceFileHashAlgorithm](), context, cfg)
			if e != nil {
				panic(e)
			}
			if src, ok := template.Src.Get(); ok {
				defer src.CloseWithContext(context)
			}
			foreign := sm.NewImportedSourceFileWithContext(RealFileNameFileName(RealFileNameFromVirtualPath(NewRustPath("FOO/input.rs"))), template.SrcHash, template.ChecksumHash, template.StableId, uint32(template.NormalizedSourceLen), template.UnnormalizedSourceLen, CrateNumFromU32(1), template.Lines, template.MultibyteChars, template.NormalizedPos, 0, context)
			defer foreign.CloseWithContext(context)
			sm.EnsureSourceFileSourcePresentWithContext(foreign.Borrow(), context)
		default:
			panic("invalid source trace op")
		}
		ok = true
	}()
	log.add("outcome|%t|%d|%t", ok, file.StrongCount(), context.Panicking())
}
func TestSourceMapTracingNativeChild(t *testing.T) {
	if os.Getenv("FERRUGO_SOURCE_MAP_TRACE_CHILD") != "1" {
		return
	}
	p := strings.Split(os.Getenv("FERRUGO_SOURCE_MAP_TRACE_ARGS"), ",")
	cfg := SpanEncodingConfig{DebugAssertions: p[0][1] == '1', OverflowChecks: p[0][3] == '1'}
	SetDynThreadSafeMode(true)
	session := NewSpanEncodingSession(cfg)
	log := &sourceMapTraceLog{}
	max := rusttracing.Trace
	switch p[2] {
	case "off":
		max = rusttracing.Off
	case "info":
		max = rusttracing.Info
	case "debug":
		max = rusttracing.Debug
	}
	style, _ := strconv.Atoi(p[5])
	fail, _ := strconv.Atoi(p[6])
	state := rustvec.NewOwned(sourceMapTraceSubscriber{log: log, interest: p[1], enabled: p[3] == "true", max: max, next: 1, style: style, fail: fail}, rustvec.ValueOps[sourceMapTraceSubscriber]{Drop: func(s *sourceMapTraceSubscriber, c rustfmt.Context) { s.log.add("subscriber_drop|%t", c.Panicking()) }})
	dispatch := rusttracing.NewDispatchOwned(rusttracing.DefaultRuntime(), state, sourceMapTraceSubscriberOps(), rusttracing.Context{})
	scope := rusttracing.NewScope(dispatch)
	context := rusttracing.AttachScope(rustfmt.Context{}, scope)
	context = WithSourceMapTracingProfile(context, LogicalSourceMapTracingProfile())
	context = WithLibTracingProfile(context, LogicalLibTracingProfile())
	if os.Getenv("FERRUGO_SOURCE_MAP_TRACE_VERBATIM") == "1" {
		context = sourceMapTraceVerbatimProfile(context)
	}
	sourceMapTraceRun(session, p[4], log, context, cfg)
	scope.Close(rustfmt.Context{})
	dispatch.CloseWithScope(scope, rustfmt.Context{})
	session.Close()
	for _, event := range log.events {
		fmt.Println(event)
	}
	os.Exit(0)
}
func TestSourceMapTracingNativeOracle(t *testing.T) {
	sourceMapRunTracingNativeOracle(t, "testdata/source_map_tracing.tsv.gz", false)
}
func TestSourceMapTracingVerbatimNativeOracle(t *testing.T) {
	sourceMapRunTracingNativeOracle(t, "testdata/source_map_tracing_verbatim.tsv.gz", true)
}
func sourceMapTraceVerbatimProfile(context rustfmt.Context) rustfmt.Context {
	metadata := LogicalSourceMapTracingProfile().Metadata()
	for id, value := range metadata {
		value.File = "original/compiler/rustc_span/src/source_map.rs"
		if value.Kind == rusttracing.EventKind {
			value.Name = "event " + value.File + ":" + strconv.FormatUint(uint64(value.Line), 10)
		}
		metadata[id] = value
	}
	context = WithSourceMapTracingProfile(context, NewSourceMapTracingProfile("whole-original-copied-source", metadata))
	lib := LogicalLibTracingProfile().Metadata()
	for line, value := range lib {
		value.File = "original/compiler/rustc_span/src/lib.rs"
		value.Name = "event " + value.File + ":" + strconv.FormatUint(uint64(value.Line), 10)
		lib[line] = value
	}
	return WithLibTracingProfile(context, NewLibTracingProfile("whole-original-copied-source", lib))
}
func sourceMapRunTracingNativeOracle(t *testing.T, path string, verbatim bool) {
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	scan := bufio.NewScanner(reader)
	scan.Buffer(make([]byte, 65536), 8<<20)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 8 {
			t.Fatal("invalid source tracing fixture")
		}
		childContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(childContext, binary, "-test.run=^TestSourceMapTracingNativeChild$")
		command.Env = append(os.Environ(), "FERRUGO_SOURCE_MAP_TRACE_CHILD=1", "FERRUGO_SOURCE_MAP_TRACE_ARGS="+strings.Join(p[:7], ","))
		flag := "0"
		if verbatim {
			flag = "1"
		}
		command.Env = append(command.Env, "FERRUGO_SOURCE_MAP_TRACE_VERBATIM="+flag)
		out, err := command.Output()
		cancel()
		if err != nil {
			t.Fatalf("%v child: %v %s", p[:7], err, out)
		}
		want := sourceMapOracleString(p[7])
		if string(out) != want {
			a, b := strings.Split(string(out), "\n"), strings.Split(want, "\n")
			for i := 0; i < len(a) && i < len(b); i++ {
				if a[i] != b[i] {
					t.Fatalf("%v event%d got%q want%q", p[:7], i, a[i], b[i])
				}
			}
			t.Fatalf("%v trace length got%d want%d", p[:7], len(a), len(b))
		}
		n++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	wantCount := 3072
	if verbatim {
		wantCount = 3840
	}
	if n != wantCount {
		t.Fatalf("tracing scripts%d want%d", n, wantCount)
	}
	t.Logf("%d original SourceMap tracing scripts", n)
}
