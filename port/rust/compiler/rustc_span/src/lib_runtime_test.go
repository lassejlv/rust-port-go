package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"reflect"
	"testing"
)

func TestLibSpanTrackUsesCurrentOperationContext(t *testing.T) {
	a := rustfmt.Context{Unwinding: true, Runtime: &rustfmt.RuntimeMetadata{Value: "scope-a"}}
	b := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "scope-b"}}
	var seen []rustfmt.Context
	hook := NewSpanTrackHook(func(_ LocalDefId, ctx rustfmt.Context) { seen = append(seen, ctx) })
	previous := SwapSpanTrackHook(hook)
	defer SwapSpanTrackHook(previous)
	span := NewSpan(nil, 1, 3, RootSyntaxContext(), Some(LocalDefId{DefIndexFromU32(7)}))
	encoder := NewSpanMemEncoderWithContext(nil, a)
	if encoder.RuntimeContext().Runtime != nil {
		t.Fatal("encoder retained constructor scope")
	}
	encoder.EncodeSpanWithContext(span, b)
	if len(seen) != 1 || seen[0] != b {
		t.Fatal("wrong current operation context", seen)
	}
	if encoder.RuntimeContext().Runtime != nil || !encoder.RuntimeContext().Unwinding {
		t.Fatal("previous guard context not restored")
	}
	if LoadSpanTrackHook() != hook {
		t.Fatal("atomic hook descriptor identity changed")
	}
}
func TestLibTrackReplacementIsVisibleToReentry(t *testing.T) {
	var seen []string
	span := NewSpan(nil, 1, 3, RootSyntaxContext(), Some(LocalDefId{DefIndexFromU32(7)}))
	second := NewSpanTrackHook(func(LocalDefId, rustfmt.Context) { seen = append(seen, "second") })
	first := NewSpanTrackHook(func(_ LocalDefId, ctx rustfmt.Context) {
		seen = append(seen, "first")
		SwapSpanTrackHook(second)
		span.DataWithContext(nil, ctx)
	})
	previous := SwapSpanTrackHook(first)
	defer SwapSpanTrackHook(previous)
	span.Data(nil)
	if !reflect.DeepEqual(seen, []string{"first", "second"}) {
		t.Fatal(seen)
	}
}
func TestLibOwnedMapperCurrentContextAndConvenience(t *testing.T) {
	a := rustfmt.Context{Unwinding: true, Runtime: &rustfmt.RuntimeMetadata{Value: "a"}}
	b := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "b"}}
	var seen []rustfmt.Context
	makeMapper := func() CtxtMapper[int] {
		return NewCtxtMapper(0, func(value *int, c SyntaxContext, ctx rustfmt.Context) SyntaxContext {
			*value = 1
			seen = append(seen, ctx)
			return c
		}, func(value *int, ctx rustfmt.Context) {
			if *value != 1 {
				t.Fatal("lost actual mapper cell")
			}
			seen = append(seen, ctx)
		}, a)
	}
	MapCtxtOwnedWithContext(DummySpan(), nil, makeMapper(), b)
	MapCtxtOwned(DummySpan(), nil, makeMapper())
	if len(seen) != 4 || seen[0] != b || seen[1] != b || seen[2].Runtime != nil || seen[3].Runtime != nil || !seen[2].Unwinding || !seen[3].Unwinding {
		t.Fatal(seen)
	}
}
func TestLibSessionScopeRestoresAfterOwnedCallPanic(t *testing.T) {
	scope := NewSessionScope()
	session := NewSessionGlobals(DefaultEdition, nil, None[SourceMapInputs]())
	defer session.Close()
	var address *int
	dropped := false
	owned := NewSpanOwned(0, func(value *int, ctx rustfmt.Context) {
		if value != address || *value != 1 || !ctx.Unwinding || !scope.IsSet() {
			t.Fatal("wrong FnOnce destruction point")
		}
		dropped = true
	})
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("missing callback panic")
			}
		}()
		SetSessionGlobalsThenOwned(scope, session, owned, func(value *int, _ *SessionScope, _ rustfmt.Context) int {
			address = value
			*value = 1
			panic("source callback")
		}, rustfmt.Context{})
	}()
	if scope.IsSet() || !dropped || owned.IsLive() {
		t.Fatal("scope or closure ownership escaped")
	}
}
func TestLibLazyDiffDecodePreservesFailureState(t *testing.T) {
	lines := NewFreezeLock(DiffsSourceFileLines(NewSourceFileDiffs(0, 0, nil)))
	file := SourceFile{Lines: lines}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("division by zero accepted")
			}
		}()
		file.LinesWithContext(rustfmt.Context{})
	}()
	if lines.IsFrozen() {
		t.Fatal("failed conversion froze data")
	}
	guard := lines.Write()
	if guard.Get().IsLines() {
		t.Fatal("failed conversion overwrote diffs")
	}
	*guard.Get() = DiffsSourceFileLines(NewSourceFileDiffs(2, 2, []byte{1, 0, 0, 1, 255}))
	guard.Close()
	if got := file.LinesWithContext(rustfmt.Context{}); !reflect.DeepEqual(got, []RelativeBytePos{0, 1, 257}) || !lines.IsFrozen() {
		t.Fatal(got)
	}
	lines.Close()
}
func TestLibMetavarFreezeOnlyExposesReadEntries(t *testing.T) {
	m := NewMetavarSpansMap()
	defer m.entries.Close()
	a, b := WithRootCtxt(nil, 1, 2), WithRootCtxt(nil, 3, 4)
	if !m.Insert(a, b) || !m.Insert(a, b) || m.Insert(a, a) || !m.Insert(b, a) {
		t.Fatal("entry collision behavior")
	}
	if got := m.Get(a); got != Some(b) {
		t.Fatal(got)
	}
	read := m.FreezeAndGetReadSpans()
	defer read.Close()
	if read.Len() != 1 {
		t.Fatal(read.Len())
	}
	if got, ok := read.Get(&a); !ok || *got != b {
		t.Fatal("wrong frozen map")
	}
	if m.Get(b).IsSome() || m.Get(a) != Some(b) {
		t.Fatal("frozen unread entry became visible")
	}
}
