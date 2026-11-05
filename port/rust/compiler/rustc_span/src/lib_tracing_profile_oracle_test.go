// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type libTraceProfileRecord struct {
	Profile, Interest, Maximum, Mode, Stdout string
	Enabled                                  bool
}
type libTraceProfileState struct {
	out      *strings.Builder
	interest string
	enabled  bool
	maximum  rusttracing.Level
	next     uint64
	metadata map[uint32]*rusttracing.Metadata
}

func (s *libTraceProfileState) log(format string, args ...any) {
	fmt.Fprintf(s.out, format+"\n", args...)
}

type libTraceProfileVisitor struct{ state *libTraceProfileState }

func (v libTraceProfileVisitor) RecordDebug(field *rusttracing.Field, value rusttracing.DebugValue, context rusttracing.Context) {
	var output strings.Builder
	formatter := rustfmt.Formatter{Writer: &output, Options: rustfmt.DefaultOptions(), Context: context.Panic}
	if err := value.WriteDebugFormatter(&formatter); err != nil {
		panic(err)
	}
	v.state.log("debug|%s|%t|%s", field.Name, context.Panic.Panicking(), hex.EncodeToString([]byte(output.String())))
}
func libTraceProfileOps() rusttracing.SubscriberOps[libTraceProfileState] {
	meta := sourceMapTraceMeta
	return rusttracing.SubscriberOps[libTraceProfileState]{
		RegisterCallsite: func(s *libTraceProfileState, m *rusttracing.Metadata, c rusttracing.Context) rusttracing.Interest {
			s.metadata[m.Line] = m
			s.log("register|%s|%t", meta(m), c.Panic.Panicking())
			switch s.interest {
			case "always":
				return rusttracing.Always
			case "never":
				return rusttracing.Never
			}
			return rusttracing.Sometimes
		},
		Enabled: func(s *libTraceProfileState, m *rusttracing.Metadata, c rusttracing.Context) bool {
			s.log("enabled|%s|%t", meta(m), c.Panic.Panicking())
			return s.enabled
		},
		MaxLevelHint: func(s *libTraceProfileState, c rusttracing.Context) (rusttracing.Level, bool) {
			s.log("max|%s|%t", []string{"off", "error", "warn", "info", "debug", "trace"}[s.maximum], c.Panic.Panicking())
			return s.maximum, true
		},
		NewSpan: func(s *libTraceProfileState, a *rusttracing.Attributes, c rusttracing.Context) rusttracing.ID {
			id := s.next
			s.next++
			s.log("new|%d|%s", id, meta(a.Metadata))
			a.Record(libTraceProfileVisitor{s}, c)
			return rusttracing.NewIDWithContext(id, c.Panic)
		},
		Record: func(s *libTraceProfileState, id rusttracing.ID, values []rusttracing.FieldValue, c rusttracing.Context) {
			s.log("record|%d", id.AsU64())
			for _, value := range values {
				if value.Value != nil && value.Field != nil {
					value.Value.Record(value.Field, libTraceProfileVisitor{s}, c)
				}
			}
		},
		RecordFollowsFrom: func(s *libTraceProfileState, id, from rusttracing.ID, _ rusttracing.Context) {
			s.log("follows|%d|%d", id.AsU64(), from.AsU64())
		},
		EventEnabled: func(s *libTraceProfileState, e *rusttracing.EventRecord, c rusttracing.Context) bool {
			s.log("event_enabled|%s|%t", meta(e.Metadata), c.Panic.Panicking())
			return true
		},
		Event: func(s *libTraceProfileState, e *rusttracing.EventRecord, c rusttracing.Context) {
			if s.metadata[e.Metadata.Line] != e.Metadata {
				panic("selected lib metadata cell identity changed")
			}
			s.log("event|%s|%t", meta(e.Metadata), c.Panic.Panicking())
			e.Record(libTraceProfileVisitor{s}, c)
		},
		Enter: func(s *libTraceProfileState, id rusttracing.ID, _ rusttracing.Context) { s.log("enter|%d", id.AsU64()) },
		Exit:  func(s *libTraceProfileState, id rusttracing.ID, _ rusttracing.Context) { s.log("exit|%d", id.AsU64()) },
		CloneSpan: func(s *libTraceProfileState, id rusttracing.ID, _ rusttracing.Context) rusttracing.ID {
			s.log("clone|%d", id.AsU64())
			return id
		},
		TryClose: func(s *libTraceProfileState, id rusttracing.ID, _ rusttracing.Context) bool {
			s.log("close|%d", id.AsU64())
			return true
		},
	}
}

func libTraceActualMetadata(t *testing.T) map[uint32]rusttracing.Metadata {
	t.Helper()
	raw, err := os.ReadFile("../../../../../tools/port-rust-span-lib/tracing-profile-manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Metadata map[string]struct {
			Name, Level, Target, ModulePath, File, Kind string
			Line                                        uint32
			Fields                                      []string
		} `json:"actual_metadata"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	result := make(map[uint32]rusttracing.Metadata)
	for key, value := range manifest.Metadata {
		line, err := strconv.ParseUint(key, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		level := rusttracing.Debug
		if value.Level == "INFO" {
			level = rusttracing.Info
		}
		result[uint32(line)] = rusttracing.Metadata{Name: value.Name, Level: level, Target: value.Target, ModulePath: value.ModulePath, File: value.File, Line: value.Line, Kind: rusttracing.EventKind, Fields: value.Fields}
	}
	return result
}

func TestLibOriginalVerbatimTracingProfile(t *testing.T) {
	metadata := libTraceActualMetadata(t)
	file, err := os.Open("testdata/lib-tracing-profile.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	data, err := io.ReadAll(io.LimitReader(gz, 8*1024*1024+1))
	if err != nil || len(data) > 8*1024*1024 {
		t.Fatal("invalid bounded lib trace fixture", err)
	}
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	scan.Buffer(nil, 2*1024*1024)
	count := 0
	for scan.Scan() {
		var record libTraceProfileRecord
		if err := json.Unmarshal(scan.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		var output strings.Builder
		runtime := rusttracing.NewRuntime()
		profile := NewLibTracingProfile("actual-verbatim-whole-span", metadata, runtime)
		maximum := rusttracing.Trace
		switch record.Maximum {
		case "off":
			maximum = rusttracing.Off
		case "info":
			maximum = rusttracing.Info
		case "debug":
			maximum = rusttracing.Debug
		}
		state := rustvec.NewOwned(libTraceProfileState{out: &output, interest: record.Interest, enabled: record.Enabled, maximum: maximum, next: 1, metadata: make(map[uint32]*rusttracing.Metadata)}, rustvec.ValueOps[libTraceProfileState]{Drop: func(s *libTraceProfileState, c rustfmt.Context) { s.log("drop|%t", c.Panicking()) }})
		dispatch := rusttracing.NewDispatchOwned(runtime, state, libTraceProfileOps(), rusttracing.Context{})
		scope := rusttracing.NewScope(dispatch)
		context := WithLibTracingProfile(rusttracing.AttachScope(rustfmt.Context{}, scope), profile)
		config := SpanEncodingConfig{DebugAssertions: record.Profile[1] == '1', OverflowChecks: record.Profile[3] == '1'}
		source, err := NewSourceFileWithContext(CustomFileName("trace.rs"), "éβ\nx", SourceFileHashMd5, None[SourceFileHashAlgorithm](), context, config)
		if err != nil {
			t.Fatal(err)
		}
		if record.Mode == "missing" {
			if src, ok := source.Src.Get(); ok {
				src.CloseWithContext(context)
			}
			source.Src = None[SharedString]()
		}
		for i := 0; i < 2; i++ {
			source.LookupFilePosWithColDisplayWithContext(2, context, config)
		}
		source.CloseWithContext(context)
		scope.Close(rustfmt.Context{})
		dispatch.Close(rustfmt.Context{})
		if output.String() != record.Stdout {
			t.Fatalf("%s/%s/%s/%t/%s trace differs\ngot%s\nwant%s", record.Profile, record.Interest, record.Maximum, record.Enabled, record.Mode, output.String(), record.Stdout)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 192 {
		t.Fatalf("lib tracing corpus count%d", count)
	}
}

func TestLibTracingProfileCompositionAndImmutability(t *testing.T) {
	metadata := LogicalLibTracingProfile().Metadata()
	profile := NewLibTracingProfile("explicit logical view", metadata)
	first := metadata[2519]
	first.Fields[0] = "changed outside"
	metadata[2519] = first
	if profile.Metadata()[2519].Fields[0] != "message" {
		t.Fatal("profile retained mutable caller fields")
	}
	ops := &rustfmt.PanicOperations{}
	other := new(int)
	context := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: other, Panic: ops, Profiles: map[string]any{"other": other}}}
	selected := WithLibTracingProfile(context, profile)
	if selected.Runtime.Value != other || selected.Runtime.Panic != ops || selected.Runtime.Profiles["other"] != other || context.Runtime.Profiles[libTracingProfileKey] != nil {
		t.Fatal("lib profile damaged or mutated caller runtime metadata")
	}
	if libSelectedTraceSite(selected, libTraceMultiByte) != libSelectedTraceSite(selected, libTraceMultiByte) {
		t.Fatal("profile recreated callsite cell")
	}
}
