// SPDX-License-Identifier: MIT OR Apache-2.0
package rusttracing_test

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	tr "github.com/lassejlv/rust-port-go/port/support/rusttracing"
	rv "github.com/lassejlv/rust-port-go/port/support/rustvec"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type visitor struct {
	log   *[]string
	label string
}

func (v visitor) RecordDebug(f *tr.Field, value tr.DebugValue, c tr.Context) {
	var s strings.Builder
	formatter := &rf.Formatter{Writer: &s, Options: rf.DefaultOptions(), Context: c.Panic}
	if err := value.WriteDebugFormatter(formatter); err != nil {
		c.Panic.Panic(err)
	}
	*v.log = append(*v.log, fmt.Sprintf("debug:%s:%s:%s", v.label, f.Name, s.String()))
}
func (v visitor) RecordU64(f *tr.Field, value uint64, c tr.Context) {
	*v.log = append(*v.log, fmt.Sprintf("u64:%s:%s:%d", v.label, f.Name, value))
}
func (v visitor) RecordBool(f *tr.Field, value bool, c tr.Context) {
	*v.log = append(*v.log, fmt.Sprintf("bool:%s:%s:%t", v.label, f.Name, value))
}
func (v visitor) RecordStr(f *tr.Field, value string, c tr.Context) {
	text := rf.String(func(f *rf.Formatter) error { return rf.FormatDebugStringUnicode18(value, f) })
	*v.log = append(*v.log, fmt.Sprintf("str:%s:%s:%s", v.label, f.Name, text))
}

type sub struct {
	log            *[]string
	label          string
	interest       tr.Interest
	enabled, event bool
	max            int
	cell           *uint64
	next           uint64
	nested         bool
}

func (s *sub) push(format string, args ...any) { *s.log = append(*s.log, fmt.Sprintf(format, args...)) }
func borrowed(cell *uint64) func(*rf.Formatter) error {
	return func(f *rf.Formatter) error {
		if err := f.WriteString("borrow("); err != nil {
			return err
		}
		child := &rf.Formatter{Writer: f.Writer, Options: rf.DefaultOptions(), Context: f.Context}
		if err := rf.FormatDebugUint(*cell, 64, child); err != nil {
			return err
		}
		return f.WriteString(")")
	}
}
func runTracingOracle(a []int) string {
	log := []string{}
	cell := uint64(7)
	runtime := tr.NewRuntime()
	ctx := rf.Context{}
	event := tr.NewCallsiteIn(runtime, tr.Metadata{Name: "event", Target: "ferrugo", Level: tr.Debug, Kind: tr.EventKind, Fields: []string{"n", "ok", "text", "value", "message"}})
	nested := tr.NewCallsiteIn(runtime, tr.Metadata{Name: "nested", Target: "ferrugo", Level: tr.Debug, Kind: tr.EventKind, Fields: []string{"n"}})
	spanSite := tr.NewCallsiteIn(runtime, tr.Metadata{Name: "span", Target: "ferrugo", Level: tr.Debug, Kind: tr.SpanKind, Fields: []string{"value", "ret", "n"}})
	emitNested := func(c tr.Context) {
		tr.Event(nil, nested, c.Panic, func() []tr.FieldValue {
			log = append(log, "eval:nested")
			return []tr.FieldValue{{Name: "n", Value: tr.U64Field(99)}}
		})
	}
	ops := tr.SubscriberOps[sub]{OnRegisterDispatch: func(s *sub, d *tr.Dispatch, c tr.Context) { s.push("dispatch:%s", s.label) }, RegisterCallsite: func(s *sub, m *tr.Metadata, c tr.Context) tr.Interest {
		s.push("register:%s:%s", s.label, m.Name)
		return s.interest
	}, Enabled: func(s *sub, m *tr.Metadata, c tr.Context) bool {
		s.push("enabled:%s:%s", s.label, m.Name)
		return s.enabled
	}, MaxLevelHint: func(s *sub, c tr.Context) (tr.Level, bool) {
		s.push("hint:%s", s.label)
		switch s.max {
		case 0:
			return tr.Off, true
		case 1:
			return tr.Info, true
		case 2:
			return tr.Debug, true
		}
		return tr.Off, false
	}, NewSpan: func(s *sub, attrs *tr.Attributes, c tr.Context) tr.ID {
		id := s.next
		s.next++
		s.push("new:%s:%s:%d", s.label, attrs.Metadata.Name, id)
		attrs.Record(visitor{s.log, s.label}, c)
		return tr.NewID(id)
	}, Record: func(s *sub, id tr.ID, values []tr.FieldValue, c tr.Context) {
		s.push("record:%s:%d", s.label, id.AsU64())
		for _, value := range values {
			if value.Value != nil {
				value.Value.Record(value.Field, visitor{s.log, s.label}, c)
			}
		}
	}, RecordFollowsFrom: func(s *sub, id, other tr.ID, c tr.Context) {
		s.push("follows:%s:%d:%d", s.label, id.AsU64(), other.AsU64())
	}, EventEnabled: func(s *sub, e *tr.EventRecord, c tr.Context) bool {
		s.push("event-enabled:%s:%s", s.label, e.Metadata.Name)
		*s.cell++
		return s.event
	}, Event: func(s *sub, e *tr.EventRecord, c tr.Context) {
		s.push("event:%s:%s", s.label, e.Metadata.Name)
		e.Record(visitor{s.log, s.label}, c)
	}, Enter: func(s *sub, id tr.ID, c tr.Context) {
		s.push("enter:%s:%d", s.label, id.AsU64())
		if s.nested {
			emitNested(c)
		}
	}, Exit: func(s *sub, id tr.ID, c tr.Context) { s.push("exit:%s:%d", s.label, id.AsU64()) }, CloneSpan: func(s *sub, id tr.ID, c tr.Context) tr.ID {
		s.push("clone:%s:%d", s.label, id.AsU64())
		return tr.NewID(id.AsU64() + 100)
	}, TryClose: func(s *sub, id tr.ID, c tr.Context) bool { s.push("close:%s:%d", s.label, id.AsU64()); return true }}
	makeDispatch := func(label string, interest int) *tr.Dispatch {
		state := rv.NewOwned(sub{&log, label, tr.Interest(interest), a[2] != 0, a[3] != 0, a[4], &cell, 1, a[5] != 0}, rv.ValueOps[sub]{Drop: func(s *sub, c rf.Context) { s.push("drop:%s:%t", s.label, c.Panicking()) }})
		return tr.NewDispatchOwned(runtime, state, ops, tr.Context{Panic: ctx})
	}
	da := makeDispatch("a", a[0])
	var db *tr.Dispatch
	if a[1] >= 0 {
		db = makeDispatch("b", a[1])
	}
	emit := func(scope *tr.Scope) {
		tr.Event(scope, event, ctx, func() []tr.FieldValue {
			log = append(log, "eval:n")
			n := cell
			message := cell
			return []tr.FieldValue{{Name: "n", Value: tr.U64Field(n)}, {Name: "ok", Value: tr.BoolField(true)}, {Name: "text", Value: tr.StrField("a\x00🦀")}, {Name: "value", Value: tr.DebugField(borrowed(&cell))}, {Name: "message", Value: tr.MessageField(func(f *rf.Formatter) error {
				if err := f.WriteString("message "); err != nil {
					return err
				}
				return rf.FormatDebugUint(message, 64, f)
			})}}
		})
	}
	makeSpan := func(scope *tr.Scope) *tr.Span {
		return tr.NewSpan(scope, spanSite, ctx, func() []tr.FieldValue {
			log = append(log, "eval:span")
			return []tr.FieldValue{{Name: "value", Value: tr.DebugField(borrowed(&cell))}, {Name: "ret"}, {Name: "n", Value: tr.U64Field(3)}}
		})
	}
	scope := tr.NewScope(da)
	emit(scope)
	emit(scope)
	span := makeSpan(scope)
	log = append(log, fmt.Sprintf("disabled:%t", span.IsDisabled()))
	entered := span.EnterWithScope(scope, ctx)
	emit(scope)
	span.RecordFieldWithScope(scope, ctx, "ret", tr.DebugField(borrowed(&cell)))
	clone := span.CloneWithScope(scope, ctx)
	if id, ok := span.ID(); ok {
		clone.FollowsFromWithScope(scope, ctx, id)
	}
	clone.CloseWithScope(scope, ctx)
	entered.CloseWithScope(scope, ctx)
	span.CloseWithScope(scope, ctx)
	runtime.RebuildInterest(scope, ctx)
	emit(scope)
	if db != nil {
		guard := scope.SetDefault(db, ctx)
		emit(scope)
		guard.Close(ctx)
	}
	scope.Close(ctx)
	emit(nil)
	outside := makeSpan(nil)
	log = append(log, fmt.Sprintf("outside-disabled:%t", outside.IsDisabled()))
	outside.Close(ctx)
	db.Close(ctx)
	da.Close(ctx)
	return strings.Join(log, "\n") + "\n"
}

func TestOriginalTracingDispatch(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
	metadata, err := os.ReadFile(filepath.Join(root, "tools/port-rust-tracing/dispatch-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases              int
		UncompressedSHA256 string            `json:"uncompressed_sha256"`
		GoBindings         map[string]string `json:"go_bindings"`
	}
	if err = json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	for name, expected := range manifest.GoBindings {
		data, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != expected {
			t.Fatalf("tracing source binding changed:%s; regenerate actual source evidence", name)
		}
	}
	path := filepath.Join(root, "port/support/rusttracing/testdata/dispatch.jsonl.gz")
	input, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	reader, err := gzip.NewReader(input)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	digest := sha256.New()
	scanner := bufio.NewScanner(io.TeeReader(reader, digest))
	scanner.Buffer(make([]byte, 65536), 1<<20)
	count := 0
	for scanner.Scan() {
		var row struct {
			Args   []int
			Output string
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		actual := runTracingOracle(row.Args)
		if actual != row.Output {
			t.Fatalf("native tracing mismatch args=%v\nexpected:\n%s\nactual:\n%s", row.Args, row.Output, actual)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != manifest.Cases {
		t.Fatalf("expected%d nativecases, got%d", manifest.Cases, count)
	}
	if hex.EncodeToString(digest.Sum(nil)) != manifest.UncompressedSHA256 {
		t.Fatal("tracing corpus checksum changed")
	}
}
