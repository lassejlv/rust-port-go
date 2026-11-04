// Package contract is development-only semver oracle replay code. It never
// participates in compiler or Cargo production execution.
package contract

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
	"io"
	"strconv"
	"strings"
)

type Request struct {
	Op, Kind, A, B            string
	Operator                  uint64
	Major                     uint64
	Minor, Patch              *uint64
	Pre                       string
	Mode, Fail                uint64
	Input                     string
	Signed                    int64
	Unsigned                  uint64
	Wide                      [2]uint64
	Float                     string
	Override, Collect, Reject bool
}

func (q Request) MarshalJSON() ([]byte, error) {
	type alias Request
	raw, _ := json.Marshal(alias(q))
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	lower := map[string]json.RawMessage{}
	for k, v := range m {
		lower[strings.ToLower(k)] = v
	}
	return json.Marshal(lower)
}
func DecodeRequest(raw []byte) (Request, error) {
	var q Request
	err := json.Unmarshal(raw, &q)
	return q, err
}

type hashable interface{ Hash(semver.Hasher) }
type Raw []string

func (h *Raw) Write(p []byte) { *h = append(*h, hex.EncodeToString(p)) }

type Primitive []string

func (h *Primitive) Write(p []byte)    { *h = append(*h, "bytes:"+hex.EncodeToString(p)) }
func (h *Primitive) WriteU8(v uint8)   { *h = append(*h, fmt.Sprintf("u8:%d", v)) }
func (h *Primitive) WriteU64(v uint64) { *h = append(*h, fmt.Sprintf("u64:%d", v)) }
func (h *Primitive) WriteUsize(v uint) { *h = append(*h, fmt.Sprintf("usize:%d", v)) }
func (h *Primitive) WriteIsize(v int)  { *h = append(*h, fmt.Sprintf("isize:%d", v)) }

type Prefix struct{ Primitive }

func (h *Prefix) WriteString(v string) { h.Primitive = append(h.Primitive, "str:"+v) }
func (h *Prefix) WriteLengthPrefix(v uint) {
	h.Primitive = append(h.Primitive, fmt.Sprintf("len:%d", v))
}
func hashes(v hashable) any {
	r := Raw{}
	p := Primitive{}
	h := Prefix{Primitive{}}
	v.Hash(&r)
	v.Hash(&p)
	v.Hash(&h)
	return map[string]any{"raw": r, "primitive": p, "prefix": h.Primitive}
}

type basicValue interface {
	fmt.Stringer
	hashable
	RustDebug() string
	WriteDebug(io.Writer, semver.DebugOptions) error
}

func basic(v basicValue) map[string]any {
	render := func(o semver.DebugOptions) string { var w strings.Builder; _ = v.WriteDebug(&w, o); return w.String() }
	cloned := false
	switch v := v.(type) {
	case semver.Version:
		cloned = v.Equal(v.Clone())
	case semver.VersionReq:
		cloned = v.Equal(v.Clone())
	case semver.Comparator:
		cloned = v.Equal(v.Clone())
	case semver.Prerelease:
		cloned = v.Equal(v.Clone())
	case semver.BuildMetadata:
		cloned = v.Equal(v.Clone())
	}
	return map[string]any{"ok": true, "display": v.String(), "debug": v.RustDebug(), "pretty": render(semver.DebugOptions{Pretty: true}), "hex": render(semver.DebugOptions{Numbers: semver.DebugLowerHex}), "pretty_hex": render(semver.DebugOptions{Pretty: true, Numbers: semver.DebugUpperHex}), "clone_eq": cloned, "hash": hashes(v)}
}
func parsed(kind, text string) (basicValue, error) {
	switch kind {
	case "version":
		return semver.ParseVersion(text)
	case "req":
		return semver.ParseVersionReq(text)
	case "comparator":
		return semver.ParseComparator(text)
	case "pre":
		return semver.NewPrerelease(text)
	case "build":
		return semver.NewBuildMetadata(text)
	}
	panic("bad kind")
}
func must(kind, text string) basicValue {
	v, e := parsed(kind, text)
	if e != nil {
		panic(e)
	}
	return v
}
func raw(q Request) semver.Comparator {
	pre, e := semver.NewPrerelease(q.Pre)
	if e != nil {
		panic(e)
	}
	c := semver.Comparator{Op: semver.Op(q.Operator), Major: q.Major, Pre: pre}
	if q.Minor != nil {
		c.Minor = semver.Some(*q.Minor)
	}
	if q.Patch != nil {
		c.Patch = semver.Some(*q.Patch)
	}
	return c
}

type sink struct {
	chunks   []string
	accepted strings.Builder
	fail     uint64
}

func (w *sink) Write(p []byte) (int, error) { return w.WriteString(string(p)) }
func (w *sink) WriteString(s string) (int, error) {
	w.chunks = append(w.chunks, s)
	if w.fail != 0 && uint64(len(w.chunks)) == w.fail {
		return 0, fmt.Errorf("injected")
	}
	return w.accepted.WriteString(s)
}
func Observe(q Request) any {
	switch q.Op {
	case "parse":
		v, e := parsed(q.Kind, q.A)
		if e != nil {
			return map[string]any{"ok": false, "display": e.Error(), "debug": e.(*semver.Error).RustDebug()}
		}
		return basic(v)
	case "order":
		a, b := must(q.Kind, q.A), must(q.Kind, q.B)
		switch a := a.(type) {
		case semver.Version:
			c := b.(semver.Version)
			return map[string]any{"cmp": a.Compare(c), "partial": a.PartialCompare(c).Value, "precedence": a.ComparePrecedence(c), "eq": a.Equal(c)}
		case semver.Prerelease:
			c := b.(semver.Prerelease)
			return map[string]any{"cmp": a.Compare(c), "partial": a.PartialCompare(c).Value, "eq": a.Equal(c)}
		case semver.BuildMetadata:
			c := b.(semver.BuildMetadata)
			return map[string]any{"cmp": a.Compare(c), "partial": a.PartialCompare(c).Value, "eq": a.Equal(c)}
		}
	case "matches":
		return must("req", q.A).(semver.VersionReq).Matches(must("version", q.B).(semver.Version))
	case "raw":
		c := raw(q)
		out := basic(c)
		out["matches"] = c.Matches(must("version", q.B).(semver.Version))
		return out
	case "display":
		var v semver.Display
		if q.Kind == "error" {
			_, e := semver.ParseVersion(q.A)
			v = e.(*semver.Error)
		} else {
			v = must(q.Kind, q.A).(semver.Display)
		}
		o := semver.FormatOptions{}
		switch q.Mode {
		case 0:
		case 1:
			o.Width = semver.Some(uint(32))
			o.Align = semver.AlignRight
		case 2:
			o.Width = semver.Some(uint(33))
			o.Fill = semver.Some('*')
			o.Align = semver.AlignCenter
		case 3:
			o.Width = semver.Some(uint(31))
			o.Fill = semver.Some('🦀')
			o.Align = semver.AlignLeft
		case 4:
			o.Width = semver.Some(uint(32))
		case 5:
			o.Width = semver.Some(uint(2))
			o.Align = semver.AlignRight
		default:
			panic("bad displaymode")
		}
		w := &sink{chunks: []string{}, fail: q.Fail}
		e := v.WriteDisplay(w, o)
		return map[string]any{"error": e != nil, "accepted": w.accepted.String(), "chunks": w.chunks}
	case "serde":
		return observeSerde(q)
	}
	panic("bad op " + strconv.Quote(q.Op))
}
