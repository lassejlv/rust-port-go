package rustcspan

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

var errSpanDebugWrite = errors.New("span debug writer failure")

type spanDebugSink struct {
	events       *[]string
	writes, fail int
	text         string
}

func (w *spanDebugSink) WriteString(text string) (int, error) {
	*w.events = append(*w.events, "write:"+text)
	w.writes++
	if w.fail != 0 && w.writes == w.fail {
		return 0, errSpanDebugWrite
	}
	w.text += text
	return len(text), nil
}
func (w *spanDebugSink) Write(text []byte) (int, error) { return w.WriteString(string(text)) }

func TestSpanDebugGetterEvaluationAfterWriterError(t *testing.T) {
	session := spanTestSession()
	span := NewSpan(session, 1, 6, RootSyntaxContext(), spanTestParent(7))
	var events []string
	previous := SwapSpanTrack(func(parent LocalDefId) {
		events = append(events, fmt.Sprintf("track:%d", parent.LocalDefIndex.AsU32()))
	})
	defer SwapSpanTrack(previous)
	sink := &spanDebugSink{events: &events, fail: 1}
	err := span.WriteDebug(session, sink, rustfmt.DefaultOptions())
	if !errors.Is(err, errSpanDebugWrite) || !reflect.DeepEqual(events, []string{"write:Span", "track:7", "track:7"}) {
		t.Fatalf("Span getter/error sequencing: %q, %v", events, err)
	}
	events = nil
	sink = &spanDebugSink{events: &events, fail: 1}
	err = (OrdSpan{Span: span}).WriteDebug(session, sink, rustfmt.DefaultOptions())
	if !errors.Is(err, errSpanDebugWrite) || !reflect.DeepEqual(events, []string{"write:OrdSpan"}) {
		t.Fatalf("errored outer tuple still formatted child: %q, %v", events, err)
	}
	events = nil
	sink = &spanDebugSink{events: &events, fail: 3}
	err = (OrdSpan{Span: span}).WriteDebug(session, sink, rustfmt.DefaultOptions())
	if !errors.Is(err, errSpanDebugWrite) || !reflect.DeepEqual(events, []string{"write:OrdSpan", "write:(", "write:Span", "track:7", "track:7"}) {
		t.Fatalf("nested Span getter/error sequencing: %q, %v", events, err)
	}
}

func TestSpanDebugActualFallbackBlocks(t *testing.T) {
	session := spanTestSession()
	span := NewSpan(session, 1, 6, SyntaxContextFromU32(10), spanTestParent(7))
	if got := span.RustDebug(session); got != "Span { lo: BytePos(1), hi: BytePos(6), ctxt: #10 }" {
		t.Fatalf("fallback Debug %q", got)
	}
	if got := (OrdSpan{Span: span}).RustDebug(session); got != "OrdSpan(Span { lo: BytePos(1), hi: BytePos(6), ctxt: #10 })" {
		t.Fatalf("OrdSpan Debug %q", got)
	}
	var events []string
	sink := &spanDebugSink{events: &events}
	options := rustfmt.DefaultOptions()
	options.DebugUpperHex = true
	if err := span.WriteDebug(session, sink, options); err != nil {
		t.Fatal(err)
	}
	if sink.text != "Span { lo: BytePos(1), hi: BytePos(6), ctxt: #10 }" {
		t.Fatalf("manual context inherited hex flags: %q", sink.text)
	}
}

type spanDebugProfileWriter struct {
	strings.Builder
	profile []SpanEncodingConfig
}

func (w *spanDebugProfileWriter) Write(text []byte) (int, error) {
	w.profile[0] = DefaultSpanEncodingConfig()
	return w.Builder.Write(text)
}
func (w *spanDebugProfileWriter) WriteString(text string) (int, error) {
	w.profile[0] = DefaultSpanEncodingConfig()
	return w.Builder.WriteString(text)
}

func TestSpanDebugSnapshotsBuildProfileBeforeWriterCallbacks(t *testing.T) {
	// Rust build flags are constant throughout an operation. The explicit Go
	// profile adapter must retain that property even if user callbacks mutate
	// the caller's variadic argument backing array after the outer tuple header.
	profile := []SpanEncodingConfig{{DebugAssertions: false, OverflowChecks: true}}
	writer := &spanDebugProfileWriter{profile: profile}
	span := spanFromBits(uint64(1)<<48 | uint64(^uint32(0)))
	if err := (OrdSpan{Span: span}).WriteDebug(nil, writer, rustfmt.DefaultOptions(), profile...); err != nil {
		t.Fatal(err)
	}
	if got := writer.String(); got != "OrdSpan(Span { lo: BytePos(4294967295), hi: BytePos(0), ctxt: #0 })" {
		t.Fatalf("mutable writer changed the selected build profile: %q", got)
	}
}

type spanPanicSiteWitness struct {
	session      *SessionGlobals
	held, called bool
}

func (w *spanPanicSiteWitness) Panic(value any) {
	w.called = true
	w.held = !w.session.spanMu.TryLock()
	if !w.held {
		w.session.spanMu.Unlock()
	}
	panic(value)
}

func TestSpanContextRaisesBeforeInternerCleanup(t *testing.T) {
	session := spanTestSession()
	witness := &spanPanicSiteWitness{session: session}
	if message := spanTestPanic(func() { spanInternerDataWithContext(session, 0, witness) }); !strings.Contains(message, "index out of range") || !witness.called || !witness.held {
		t.Fatalf("bounds failure did not call the runtime while holding the lock: %q, %+v", message, witness)
	}
	if !session.spanMu.TryLock() {
		t.Fatal("ordinary unwind failed to release interner lock")
	}
	session.spanMu.Unlock()
	// Production Context.Panic aborts immediately in processing mode. This
	// witness observes the identical raise site before unwinding; it makes no
	// claim to inspect Go state after process termination.
	witness = &spanPanicSiteWitness{session: session}
	if message := spanTestPanic(func() { spanInternerDataWithContext(nil, 0, witness) }); !strings.Contains(message, "session globals are not set") || !witness.called || witness.held {
		t.Fatalf("missing-session raise site: %q, %+v", message, witness)
	}
	witness = &spanPanicSiteWitness{session: session}
	if message := spanTestPanic(func() {
		inlineCtxtFromSpan(spanFromBits(uint64(1)<<48|uint64(^uint32(0)))).dataWithContext(DefaultSpanEncodingConfig(), witness)
	}); !strings.Contains(message, "add with overflow") || !witness.called || witness.held {
		t.Fatalf("inline-overflow raise site: %q, %+v", message, witness)
	}
}
