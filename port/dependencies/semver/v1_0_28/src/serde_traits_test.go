//go:build semver_serde

package semver

import (
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"
)

func TestSerdePreservesAssociatedErrorsAndConsumedInputChoice(t *testing.T) {
	failure := errors.New("caller error")
	v := NewVersion(1, 2, 3)
	result, e := v.Serialize(SerializerAdapter{CollectStringOverride: func(value Display) (any, error) {
		if value.String() != "1.2.3" {
			t.Fatal(value)
		}
		return nil, failure
	}})
	if result != nil || e != failure {
		t.Fatal(result, e)
	}
	d := DeserializerAdapter{DeserializeString: func(v SerdeVisitor) (any, error) { return v.Visit(SerdeInput{Kind: SerdeString, Text: "bad"}) }, Custom: func(Display) error { return failure }}
	if _, e := DeserializeVersion(d); e != failure {
		t.Fatal(e)
	}
	child := DeserializerAdapter{InvalidTypeOverride: func(u Unexpected, expected SerdeExpected) error {
		if u.Kind() != UnexpectedOption || expected.Expecting() != "semver version" {
			t.Fatal(u, expected)
		}
		return failure
	}}
	d = DeserializerAdapter{DeserializeString: func(v SerdeVisitor) (any, error) {
		return v.Visit(SerdeInput{Kind: SerdeSome, Data: func() { t.Fatal("traversed child") }, ErrorFactory: child})
	}, Custom: func(Display) error { t.Fatal("used outer Error instead of child Error"); return nil }}
	if _, e := DeserializeVersionReq(d); e != failure {
		t.Fatal(e)
	}
	d.DeserializeString = func(v SerdeVisitor) (any, error) {
		if e := v.WriteExpecting(failingSerdeSink{}); !errors.Is(e, io.ErrClosedPipe) {
			t.Fatal(e)
		}
		return nil, failure
	}
	if _, e := DeserializeComparator(d); e != failure {
		t.Fatal(e)
	}
}

type failingSerdeSink struct{}

func (failingSerdeSink) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }
func TestSerdeFrontendFailureKeepsReceiver(t *testing.T) {
	initial := NewVersion(8, 9, 10)
	v := initial
	for _, input := range []string{`"bad"`, `"1.2.3\ud800"`, "null", "[", "{}", `"1.2.3" null`} {
		if e := v.UnmarshalJSON([]byte(input)); e == nil || !v.Equal(initial) {
			t.Fatal(input, e, v)
		}
	}
	if e := v.UnmarshalJSON([]byte(`"1.2.3"`)); e != nil || v.String() != "1.2.3" {
		t.Fatal(e, v)
	}
	q, _ := ParseVersionReq("<2.0.0")
	raw, e := q.MarshalJSON()
	if e != nil || string(raw) != `"<2.0.0"` {
		t.Fatal(string(raw), e)
	}
	if e := v.UnmarshalText([]byte{255}); e == nil {
		t.Fatal("invalidUTF8 accepted")
	}
	raw, e = v.MarshalText()
	if e != nil || !reflect.DeepEqual(raw, []byte("1.2.3")) {
		t.Fatal(string(raw), e)
	}
	// Source inputs are Rust &str; the Go adapter rejects invalid UTF-8 rather
	// than silently substituting U+FFFD at the parser/protocol boundary.
	d := DeserializerAdapter{DeserializeString: func(visitor SerdeVisitor) (any, error) {
		return visitor.Visit(SerdeInput{Kind: SerdeString, Text: string([]byte{255})})
	}}
	defer func() {
		if p := recover(); p == nil || !strings.Contains(p.(string), "UTF-8") {
			t.Fatal("missing UTF8 panic", p)
		}
	}()
	_, _ = DeserializeVersion(d)
}

// Expected is delegated lazily inside the Display passed to Error::custom.
// A failure in an earlier literal/Unexpected must prevent its invocation.
type observedExpected struct {
	calls   *int
	failure error
}

func (value observedExpected) Expecting() string {
	panic("default invalid_type eagerly requested a String")
}
func (value observedExpected) WriteExpecting(w io.Writer) error {
	*value.calls++
	if _, e := io.WriteString(w, "dynamic expectation"); e != nil {
		return e
	}
	return value.failure
}
func TestSerdeInvalidTypeDefersExpectedAndPreservesFailure(t *testing.T) {
	for _, earlyFailure := range []bool{true, false} {
		calls := 0
		expectedFailure := errors.New("expecting failure")
		sink := &observedDebugSink{}
		if earlyFailure {
			sink.fail = 1
		}
		var rendered error
		factory := DeserializerAdapter{Custom: func(message Display) error {
			rendered = message.WriteDisplay(sink, FormatOptions{Width: Some(uint(30)), Fill: Some('*')})
			return rendered
		}}
		err := factory.InvalidType(Unexpected{input: SerdeInput{Kind: SerdeI64, Signed: -42}}, observedExpected{calls: &calls, failure: expectedFailure})
		if earlyFailure {
			if calls != 0 || !errors.Is(err, errDebugSink) {
				t.Fatal(calls, err)
			}
		} else {
			if calls != 1 || err != expectedFailure || rendered != expectedFailure {
				t.Fatal(calls, err)
			}
		}
	}
}

func TestSerdeJSONValidatesOnlyConsumedScalarBeforeVisitor(t *testing.T) {
	for _, raw := range []string{`"bad" "\ud800"`, `true "\ud800"`, `[ "\ud800" ]`, `{ "x":"\ud800" }`, `"bad" ` + string([]byte{255}), "[ " + string([]byte{255})} {
		v := NewVersion(8, 9, 10)
		err := v.UnmarshalJSON([]byte(raw))
		if err == nil || err.(*SerdeError).Syntax || v.String() != "8.9.10" {
			t.Fatalf("%q: %v /%v", raw, err, v)
		}
	}
	for _, raw := range []string{`"1.2.3" "\ud800"`, `"1.2.3" ` + string([]byte{255}), `"1.2.3\ud800"`} {
		v := NewVersion(8, 9, 10)
		err := v.UnmarshalJSON([]byte(raw))
		if err == nil || !err.(*SerdeError).Syntax || v.String() != "8.9.10" {
			t.Fatalf("%q: %v /%v", raw, err, v)
		}
	}
}
