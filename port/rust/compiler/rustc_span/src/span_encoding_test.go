package rustcspan

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"

	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
)

func spanTestParent(value uint32) Option[LocalDefId] { return Some(LocalDefId{DefIndexFromU32(value)}) }
func spanTestPanic(call func()) (message string) {
	defer func() {
		if value := recover(); value != nil {
			message = fmt.Sprint(value)
		}
	}()
	call()
	return
}
func spanTestSession() *SessionGlobals { return NewSpanEncodingSession(DefaultSpanEncodingConfig()) }

func TestSpanEncodingExactFormatsAndInternerIdentity(t *testing.T) {
	if unsafe.Sizeof(Span{}) != 8 || unsafe.Alignof(Span{}) != 4 {
		t.Fatalf("Span size/alignment=%d/%d", unsafe.Sizeof(Span{}), unsafe.Alignof(Span{}))
	}
	session := spanTestSession()
	cases := []struct {
		lo, length, context uint32
		parent              Option[LocalDefId]
		bits                uint64
	}{
		{7, 5, 0xffff, None[LocalDefId](), uint64(5)<<48 | uint64(0xffff)<<32 | 7},
		{0x12345678, 0x3fff, 0, spanTestParent(0x7fff), uint64(6)<<61 | uint64(0x3fff)<<47 | uint64(0x7fff)<<32 | 0x12345678},
		{0xffffff, 127, 0x7fff, spanTestParent(0xffff), uint64(2)<<62 | uint64(127)<<55 | uint64(0x7fff)<<40 | uint64(0xffff)<<24 | 0xffffff},
		{0, 32768, SyntaxContextMaxAsU32, None[LocalDefId](), uint64(7)<<61 | uint64(SyntaxContextMaxAsU32)<<32},
	}
	for _, c := range cases {
		span := NewSpan(session, BytePos(c.lo), BytePos(c.lo+c.length), SyntaxContextFromU32(c.context), c.parent)
		if span.bits() != c.bits {
			t.Fatalf("%+v encoded%016x want%016x", c, span.bits(), c.bits)
		}
		data := span.DataUntracked(session)
		want := SpanData{BytePos(c.lo), BytePos(c.lo + c.length), SyntaxContextFromU32(c.context), c.parent}
		if !data.Equal(want) {
			t.Fatalf("decoded%+v want%+v", data, want)
		}
		if !SpanFromRawSpan(span.ToRawSpan()).Equal(span) {
			t.Fatal("raw roundtrip differs")
		}
	}
	otherContext := NewSpan(session, 0, 32768, SyntaxContextFromU32(65536), None[LocalDefId]())
	if internedFromSpan(otherContext).index != 0 || len(session.spanInterner.spans) != 1 {
		t.Fatal("interner key incorrectly includes context")
	}
	otherParent := NewSpan(session, 0, 32768, SyntaxContextFromU32(65536), spanTestParent(65536))
	if internedFromSpan(otherParent).index != 1 || len(session.spanInterner.spans) != 2 {
		t.Fatal("interner key failed to distinguish parent")
	}
	if NewSpan(session, 10, 5, RootSyntaxContext(), None[LocalDefId]()).DataUntracked(session) != (SpanData{5, 10, RootSyntaxContext(), None[LocalDefId]()}) {
		t.Fatal("new did not swap reversed positions")
	}
}

func TestSpanEncodingBoundaryRoundtrips(t *testing.T) {
	session := spanTestSession()
	formats := map[string]int{}
	parents := []Option[LocalDefId]{None[LocalDefId](), spanTestParent(0), spanTestParent(32767), spanTestParent(32768), spanTestParent(65535), spanTestParent(65536), spanTestParent(DefIndexMaxAsU32)}
	for _, lo := range []uint32{0, 1, 0xffffff, 0x1000000, 0xffff0000} {
		for _, length := range []uint32{0, 1, 127, 128, 16383, 16384, 32767, 32768} {
			if uint64(lo)+uint64(length) > uint64(^uint32(0)) {
				continue
			}
			for _, context := range []uint32{0, 1, 32767, 32768, 65535, 65536, SyntaxContextMaxAsU32} {
				for _, parent := range parents {
					ctxt := SyntaxContextFromU32(context)
					span := NewOrderedSpan(session, BytePos(lo), BytePos(lo+length), ctxt, parent)
					want := SpanData{BytePos(lo), BytePos(lo + length), ctxt, parent}
					if !span.DataUntracked(session).Equal(want) || span.Ctxt() != ctxt || span.Parent(session) != parent || span.FromExpansion() != (context != 0) {
						t.Fatalf("roundtrip failed: %+v bits%016x", want, span.bits())
					}
					bits := span.bits()
					format := "interned"
					if inlineCtxtPrefix.get(bits) == 0 {
						format = "context"
					} else if inlineParentPrefix.get(bits) == 6 {
						format = "parent"
					} else if inlinePairPrefix.get(bits) == 2 {
						format = "pair"
					}
					formats[format]++
				}
			}
		}
	}
	if len(formats) != 4 {
		t.Fatalf("missing encoding family: %v", formats)
	}
}

func TestSpanTrackingAndContextMappingOrder(t *testing.T) {
	session := spanTestSession()
	var events []string
	restore := SwapSpanTrack(func(parent LocalDefId) {
		events = append(events, fmt.Sprintf("track:%d", parent.LocalDefIndex.AsU32()))
	})
	defer SwapSpanTrack(restore)
	span := NewSpan(session, 10, 20, RootSyntaxContext(), spanTestParent(7))
	span.DataUntracked(session)
	span.Ctxt()
	span.Parent(session)
	span.IsDummy(session)
	if len(events) != 0 {
		t.Fatal("untracked accessor introduced tracking")
	}
	span.Data(session)
	if !reflect.DeepEqual(events, []string{"track:7"}) {
		t.Fatalf("Data events%v", events)
	}
	events = nil
	span = span.MapCtxt(session, func(context SyntaxContext) SyntaxContext {
		events = append(events, fmt.Sprintf("map:%d", context.AsU32()))
		return SyntaxContextFromU32(9)
	})
	if !reflect.DeepEqual(events, []string{"map:0"}) {
		t.Fatalf("map_ctxt introduced tracking or duplicate callback: %v", events)
	}
	events = nil
	span = span.WithParent(session, spanTestParent(7))
	if !reflect.DeepEqual(events, []string{"track:7"}) {
		t.Fatalf("same-parent replacement must still track old parent: %v", events)
	}
	events = nil
	span = span.WithParent(session, None[LocalDefId]())
	if !reflect.DeepEqual(events, []string{"track:7"}) || span.Parent(session).IsSome() {
		t.Fatalf("removing parent events%v", events)
	}
	events = nil
	span.WithParent(session, None[LocalDefId]())
	if len(events) != 0 {
		t.Fatal("inline no-parent identity path tracked")
	}
	old := NewSpan(session, 0, 40000, SyntaxContextFromU32(65536), spanTestParent(8))
	other := NewSpan(session, 0, 40000, SyntaxContextFromU32(65537), spanTestParent(9))
	if !(OrdSpan{Span: old}).Equal(session, OrdSpan{Span: other}) || !reflect.DeepEqual(events, []string{"track:8", "track:9"}) {
		t.Fatalf("OrdSpan equality location/tracking order%v", events)
	}
}

func TestSpanTrackingIsOutsideInternerLockAndPanicSafe(t *testing.T) {
	session := spanTestSession()
	span := NewSpan(session, 0, 40000, SyntaxContextFromU32(65536), spanTestParent(5))
	done := make(chan bool, 1)
	previous := SwapSpanTrack(func(LocalDefId) { NewSpan(session, 1, 50000, SyntaxContextFromU32(65536), None[LocalDefId]()) })
	defer SwapSpanTrack(previous)
	go func() { span.Data(session); done <- true }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("tracking callback ran while holding the interner lock")
	}
	SwapSpanTrack(func(LocalDefId) { panic("query tracking panic") })
	if got := spanTestPanic(func() { span.Data(session) }); got != "query tracking panic" {
		t.Fatalf("tracking panic not propagated: %q", got)
	}
	if span.DataUntracked(session).Lo != 0 {
		t.Fatal("tracking panic changed stored span")
	}
	before := len(session.spanInterner.spans)
	if got := spanTestPanic(func() { span.WithParent(session, spanTestParent(6)) }); got != "query tracking panic" || len(session.spanInterner.spans) != before {
		t.Fatal("parent replacement allocated before old-parent tracking succeeded")
	}
}

func TestSpanEncodingIndependentBuildProfilesAndFastPaths(t *testing.T) {
	for _, debug := range []bool{false, true} {
		for _, overflow := range []bool{false, true} {
			config := SpanEncodingConfig{debug, overflow}
			session := NewSpanEncodingSession(config)
			panicText := spanTestPanic(func() { NewOrderedSpan(session, 100, 0, RootSyntaxContext(), None[LocalDefId]()) })
			if debug && !strings.Contains(panicText, "lo <= hi") || !debug && overflow && !strings.Contains(panicText, "subtract") || !debug && !overflow && panicText != "" {
				t.Fatalf("profile%+v ordered-position result%q", config, panicText)
			}
			raw := SpanFromRawSpan(datastructures.RawSpan{Value: uint64(1)<<48 | uint64(^uint32(0))})
			panicText = spanTestPanic(func() { raw.DataUntracked(session) })
			if (panicText != "") != (debug && overflow) {
				t.Fatalf("profile%+v DebugStrictAdd result%q", config, panicText)
			}
			if raw.Ctxt() != RootSyntaxContext() || !raw.WithParent(session, None[LocalDefId]()).Equal(raw) {
				t.Fatal("context/identity fast path decoded overflowing lo+len")
			}
			calls := 0
			mapped := raw.MapCtxt(session, func(c SyntaxContext) SyntaxContext { calls++; return c })
			if calls != 1 || !mapped.Equal(raw) {
				t.Fatal("inline map fast path decoded overflowing lo+len")
			}
			if !debug || !overflow {
				if data := raw.DataUntracked(session); data.Hi != 0 {
					t.Fatal("DebugStrictAdd wrapping profile failed")
				}
			}
			withoutSession := spanTestPanic(func() { raw.DataUntracked(nil, config) })
			if (withoutSession != "") != (debug && overflow) {
				t.Fatalf("profile%+v absent-session DebugStrictAdd result%q", config, withoutSession)
			}
		}
	}
}

func TestSpanSessionRequirementAndDummyContext(t *testing.T) {
	inline := NewSpan(nil, 0, 0, SyntaxContextFromU32(1), spanTestParent(0))
	if !inline.IsDummy(nil) || inline.Equal(DummySpan()) || !inline.FromExpansion() {
		t.Fatal("dummy context/parent semantics differ")
	}
	session := spanTestSession()
	interned := NewSpan(session, 0, 0, SyntaxContextFromU32(65536), spanTestParent(65536))
	if !interned.IsDummy(session) || interned.Ctxt().AsU32() != 65536 {
		t.Fatal("interned dummy semantics differ")
	}
	for _, operation := range []func(){func() { NewSpan(nil, 0, 40000, RootSyntaxContext(), None[LocalDefId]()) }, func() { interned.DataUntracked(nil) }, func() { interned.Parent(nil) }, func() { interned.IsDummy(nil) }} {
		if got := spanTestPanic(operation); !strings.Contains(got, "session globals are not set") {
			t.Fatalf("missing session accepted: %q", got)
		}
	}
	if got := spanTestPanic(func() { DefIndexFromU32(DefIndexMaxAsU32 + 1) }); got == "" {
		t.Fatal("DefIndex MAX unchecked")
	}
	if got := spanTestPanic(func() { SyntaxContextFromU32(SyntaxContextMaxAsU32 + 1) }); got == "" {
		t.Fatal("SyntaxContext MAX unchecked")
	}
}

type spanHashSpy struct {
	events  []string
	panicAt int
}

func (h *spanHashSpy) event(value string) {
	h.events = append(h.events, value)
	if h.panicAt != 0 && len(h.events) == h.panicAt {
		panic("span hasher panic")
	}
}
func (h *spanHashSpy) WriteU64(value uint64) { h.event(fmt.Sprintf("u64:%d", value)) }
func (h *spanHashSpy) WriteU32(value uint32) { h.event(fmt.Sprintf("u32:%d", value)) }
func (h *spanHashSpy) WriteIsize(value int)  { h.event(fmt.Sprintf("isize:%d", value)) }

func TestSpanHashUsesEncodedIdentityAndNoTracking(t *testing.T) {
	previous := SwapSpanTrack(func(LocalDefId) { panic("hash must not track") })
	defer SwapSpanTrack(previous)
	// The raw interner index is absent. Hashing and cloning still operate on
	// encoded identity without attempting to decode the span.
	span := SpanFromRawSpan(datastructures.RawSpan{Value: ^uint64(0)})
	h := &spanHashSpy{}
	span.Hash(h)
	if !reflect.DeepEqual(h.events, []string{"u64:18446744073709551615"}) || !span.Equal(span.Clone()) {
		t.Fatalf("encoded identity hash/clone%v", h.events)
	}
	h = &spanHashSpy{panicAt: 1}
	if got := spanTestPanic(func() { span.Hash(h) }); got != "span hasher panic" || !span.Equal(SpanFromRawSpan(datastructures.RawSpan{Value: ^uint64(0)})) {
		t.Fatal("hasher panic was suppressed or changed span")
	}
	data := SpanData{1, 2, SyntaxContextFromU32(3), spanTestParent(4)}
	h = &spanHashSpy{}
	data.Hash(h)
	if !reflect.DeepEqual(h.events, []string{"u32:1", "u32:2", "u32:3", "isize:1", "u32:4"}) {
		t.Fatalf("SpanData hash field order%v", h.events)
	}
	key := spanDataNoCtxt{data.Lo, data.Hi, data.Parent}
	h = &spanHashSpy{}
	key.hash(h)
	if !reflect.DeepEqual(h.events, []string{"u32:1", "u32:2", "isize:1", "u32:4"}) || !key.equal(key.clone()) {
		t.Fatalf("interner key hash included context or clone differed: %v", h.events)
	}
}

func FuzzSpanEncodingRoundtrip(f *testing.F) {
	for _, seed := range [][4]uint32{{0, 0, 0, 0}, {1, 6, 1, 1}, {0xffffff, 0x100007e, 0x7fff, 0xffff}, {0, 32768, 65536, 65536}, {^uint32(0), 0, SyntaxContextMaxAsU32, DefIndexMaxAsU32}} {
		f.Add(seed[0], seed[1], seed[2], seed[3], true)
	}
	f.Fuzz(func(t *testing.T, lo, hi, context, parent uint32, hasParent bool) {
		context &= SyntaxContextMaxAsU32
		if parent > DefIndexMaxAsU32 {
			parent = DefIndexMaxAsU32
		}
		p := None[LocalDefId]()
		if hasParent {
			p = spanTestParent(parent)
		}
		session := spanTestSession()
		span := NewSpan(session, BytePos(lo), BytePos(hi), SyntaxContextFromU32(context), p)
		if lo > hi {
			lo, hi = hi, lo
		}
		want := SpanData{BytePos(lo), BytePos(hi), SyntaxContextFromU32(context), p}
		if !span.DataUntracked(session).Equal(want) || !SpanFromRawSpan(span.ToRawSpan()).Equal(span) {
			t.Fatal("packed span did not roundtrip")
		}
		calls := 0
		mapped := span.MapCtxt(session, func(SyntaxContext) SyntaxContext { calls++; return RootSyntaxContext() })
		want.Ctxt = RootSyntaxContext()
		if calls != 1 || !mapped.DataUntracked(session).Equal(want) {
			t.Fatal("context mapping changed locations/parent or callback count")
		}
		withoutParent := mapped.WithParent(session, None[LocalDefId]())
		want.Parent = None[LocalDefId]()
		if !withoutParent.DataUntracked(session).Equal(want) {
			t.Fatal("parent removal changed locations/context")
		}
	})
}
