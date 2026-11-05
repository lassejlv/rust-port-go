package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type defIDRequest struct {
	Op, Kind, Method, Name, Cfg           string
	RawHex                                string `json:"raw_hex"`
	Metadata                              []string
	VersionHex                            *string `json:"version_hex"`
	Crate, Index                          uint32
	First, Second, A, B, N                uint64
	Mode                                  int
	Fail                                  int
	Chars, Exe, Debug, Overflow, Override bool
}

func defIDRaw(v DefId) [2]uint32 { return [2]uint32{v.Krate.AsU32(), v.Index.AsU32()} }
func defIDMaybe[T any](v Option[T]) any {
	if x, ok := v.Get(); ok {
		return x
	}
	return nil
}

type defIDTrace []string

func (t *defIDTrace) WriteU32(n uint32) { *t = append(*t, "u32:"+strconv.FormatUint(uint64(n), 10)) }
func (t *defIDTrace) WriteU64(n uint64) { *t = append(*t, "u64:"+strconv.FormatUint(n, 10)) }
func defIDHash(v any) []string {
	out := defIDTrace{}
	switch v := v.(type) {
	case DefId:
		v.Hash(&out)
	case LocalDefId:
		v.Hash(&out)
	case ModId:
		v.Hash(&out)
	case LocalModId:
		v.Hash(&out)
	case CrateNum:
		v.Hash(&out)
	case DefIndex:
		v.Hash(&out)
	case DefPathHash:
		v.Hash(&out)
	case StableCrateId:
		v.Hash(&out)
	}
	return out
}

type defIDContext struct{ events []string }

func (c *defIDContext) StableHashSpan(span datastructures.RawSpan, h *datastructures.StableHasher) {
	c.events = append(c.events, fmt.Sprintf("span:%d", span.Value))
	h.WriteU64(span.Value)
}
func (c *defIDContext) DefPathHash(id datastructures.RawDefId) datastructures.Fingerprint {
	c.events = append(c.events, fmt.Sprintf("key:%d:%d", id.Crate, id.Index))
	return datastructures.NewFingerprint(rustchashes.NewHash64(0xa55aa55aa55aa55a^uint64(id.Crate)), rustchashes.NewHash64(0x123456789abc0000+uint64(id.Index)))
}
func (c *defIDContext) StableHashControls() datastructures.StableHashControls {
	c.events = append(c.events, "controls")
	return datastructures.StableHashControls{HashSpans: true}
}
func (c *defIDContext) AssertDefaultStableHashControls(s string) {
	c.events = append(c.events, "assert:"+s)
}
func defIDStable(v interface {
	StableHash(datastructures.StableHashCtxt, *datastructures.StableHasher)
}) map[string]any {
	c := &defIDContext{events: []string{}}
	h := datastructures.NewStableHasher()
	h.Write([]byte("prefix"))
	v.StableHash(c, h)
	return map[string]any{"hash": h.FinishHash(), "events": c.events}
}
func defIDCatch(call func() any) (value any, panicked bool) {
	defer func() {
		if p := recover(); p != nil {
			value = fmt.Sprint(p)
			panicked = true
		}
	}()
	return call(), false
}
func defIDCore(q defIDRequest) any {
	a := DefId{Krate: CrateNumFromU32(q.Crate), Index: DefIndexFromU32(q.Index)}
	local := LocalDefId{a.Index}
	modu := NewModIdUnchecked(a)
	localmod := NewLocalModIdUnchecked(local)
	fp := DefPathHash{datastructures.NewFingerprint(rustchashes.NewHash64(q.First), rustchashes.NewHash64(q.Second))}
	stable := fp.StableCrateId()
	zero := ZeroDefPathHash()
	c := &defIDContext{events: []string{}}
	key := a.ToStableHashKey(c)
	var asLocal any
	if value, ok := a.AsLocal().Get(); ok {
		asLocal = value.LocalDefIndex.AsU32()
	}
	expected, _ := defIDCatch(func() any { return a.ExpectLocal().LocalDefIndex.AsU32() })
	var asRoot any
	if value, ok := a.AsCrateRoot().Get(); ok {
		asRoot = value.AsU32()
	}
	var modLocal any
	if value, ok := modu.AsLocal().Get(); ok {
		modLocal = value.ToLocalDefId().LocalDefIndex.AsU32()
	}
	unhash := datastructures.NewUnhasher()
	fp.Hash(unhash)
	return map[string]any{"raw": defIDRaw(a), "roundtrip": defIDRaw(DefIdFromRawDefId(a.ToRawDefId())), "is_local": a.IsLocal(), "as_local": asLocal, "expect_local": expected, "is_root": a.IsCrateRoot(), "as_root": asRoot, "is_top": a.IsTopLevelModule(), "crate_def": defIDRaw(a.Krate.AsDefId()), "crate_mod": defIDRaw(a.Krate.AsModId().ToDefId()), "local_def": defIDRaw(local.ToDefId()), "local_top": local.IsTopLevelModule(), "mod_def": defIDRaw(modu.ToDefId()), "mod_local": modLocal, "mod_top": modu.IsTopLevelModule(), "localmod_def": defIDRaw(localmod.ToDefId()), "localmod_mod": defIDRaw(localmod.ToModId().ToDefId()), "hash_id": defIDHash(a), "hash_local": defIDHash(local), "hash_mod": defIDHash(modu), "hash_localmod": defIDHash(localmod), "hash_crate": defIDHash(a.Krate), "hash_index": defIDHash(a.Index), "hash_path": defIDHash(fp), "hash_stable_id": defIDHash(stable), "unhash_path": unhash.Finish(), "path_parts": [2]uint64{stable.AsU64(), fp.LocalHash().AsU64()}, "zero_path": [2]uint64{zero.StableCrateId().AsU64(), zero.LocalHash().AsU64()}, "stable_id": defIDStable(a), "stable_local": defIDStable(local), "stable_crate": defIDStable(a.Krate), "stable_mod": defIDStable(modu), "stable_localmod": defIDStable(localmod), "stable_path": defIDStable(fp), "stable_stable_id": defIDStable(stable), "key": [2]uint64{key.StableCrateId().AsU64(), key.LocalHash().AsU64()}, "key_events": c.events, "clones": []bool{a.Equal(a.Clone()), local.Equal(local.Clone()), modu.Equal(modu.Clone()), localmod.Equal(localmod.Clone()), fp.Equal(fp.Clone()), stable.Equal(stable.Clone())}, "marker": DefPathHashCanUseUnstableSort}
}
func defIDStep(q defIDRequest) any {
	n := uint(q.N)
	config := DefIdConfig{DebugAssertions: q.Debug, OverflowChecks: q.Overflow}
	if q.Kind == "crate" {
		value := CrateNumFromUsize(uint(q.A))
		switch q.Method {
		case "new":
			return value.Index()
		case "between":
			lo, hi := CrateNumStepsBetween(value, CrateNumFromUsize(uint(q.B)))
			return []any{lo, defIDMaybe(hi)}
		case "plus":
			return CrateNumIdxWithContext(rustfmt.Context{}, config).Plus(value, n).Index()
		case "increment":
			p, failed := defIDCatch(func() any { CrateNumIdxWithContext(rustfmt.Context{}, config).IncrementBy(&value, n); return nil })
			if !failed {
				p = nil
			}
			return map[string]any{"after": value.Index(), "panic": p}
		case "add":
			return value.Add(n, config).Index()
		case "assign":
			p, failed := defIDCatch(func() any { value.AddAssign(n, config); return nil })
			if !failed {
				p = nil
			}
			return map[string]any{"after": value.Index(), "panic": p}
		case "forward_checked":
			v, ok := value.ForwardChecked(n).Get()
			if !ok {
				return nil
			}
			return v.Index()
		case "backward_checked":
			v, ok := value.BackwardChecked(n).Get()
			if !ok {
				return nil
			}
			return v.Index()
		case "forward_overflowing":
			v, o := value.ForwardOverflowing(n)
			return []any{v.Index(), o}
		case "backward_overflowing":
			v, o := value.BackwardOverflowing(n)
			return []any{v.Index(), o}
		case "forward":
			return value.Forward(n).Index()
		case "backward":
			return value.Backward(n).Index()
		}
	}
	value := DefIndexFromUsize(uint(q.A))
	switch q.Method {
	case "new":
		return value.Index()
	case "between":
		lo, hi := DefIndexStepsBetween(value, DefIndexFromUsize(uint(q.B)))
		return []any{lo, defIDMaybe(hi)}
	case "plus":
		return DefIndexIdxWithContext(rustfmt.Context{}, config).Plus(value, n).Index()
	case "increment":
		p, failed := defIDCatch(func() any { DefIndexIdxWithContext(rustfmt.Context{}, config).IncrementBy(&value, n); return nil })
		if !failed {
			p = nil
		}
		return map[string]any{"after": value.Index(), "panic": p}
	case "add":
		return value.Add(n, config).Index()
	case "assign":
		p, failed := defIDCatch(func() any { value.AddAssign(n, config); return nil })
		if !failed {
			p = nil
		}
		return map[string]any{"after": value.Index(), "panic": p}
	case "forward_checked":
		v, ok := value.ForwardChecked(n).Get()
		if !ok {
			return nil
		}
		return v.Index()
	case "backward_checked":
		v, ok := value.BackwardChecked(n).Get()
		if !ok {
			return nil
		}
		return v.Index()
	case "forward_overflowing":
		v, o := value.ForwardOverflowing(n)
		return []any{v.Index(), o}
	case "backward_overflowing":
		v, o := value.BackwardOverflowing(n)
		return []any{v.Index(), o}
	case "forward":
		return value.Forward(n).Index()
	case "backward":
		return value.Backward(n).Index()
	}
	panic("unknown step operation")
}

type defIDSink struct {
	calls []string
	text  strings.Builder
	fail  int
	chars bool
}

func (s *defIDSink) emit(kind, text string) (int, error) {
	n := len(s.calls)
	s.calls = append(s.calls, kind+":"+hex.EncodeToString([]byte(text)))
	if n == s.fail {
		return 0, errors.New("fmt sink")
	}
	s.text.WriteString(text)
	return len(text), nil
}
func (s *defIDSink) Write(b []byte) (int, error)          { return s.emit("S", string(b)) }
func (s *defIDSink) WriteString(text string) (int, error) { return s.emit("S", text) }
func (s *defIDSink) WriteChar(c rune) error {
	kind := "S"
	if s.chars {
		kind = "C"
	}
	_, e := s.emit(kind, string(c))
	return e
}
func defIDOptions(mode int) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	width := func(n uint) { o.Width = n; o.HasWidth = true }
	switch mode {
	case 1:
		o.Alternate = true
	case 2:
		o.DebugLowerHex = true
	case 3:
		o.DebugUpperHex = true
	case 4:
		o.Alternate = true
		o.DebugLowerHex = true
	case 5:
		o.SignPlus = true
		o.ZeroPad = true
		width(8)
	case 6:
		o.ZeroPad = true
		o.DebugLowerHex = true
		width(8)
	case 7:
		o.Fill = '*'
		o.Align = rustfmt.AlignRight
		width(12)
	case 8:
		o.Fill = '*'
		o.Align = rustfmt.AlignLeft
		width(12)
	case 9:
		o.Fill = '*'
		o.Align = rustfmt.AlignCenter
		width(13)
	case 10:
		o.HasPrecision = true
		o.Precision = 2
	case 11:
		o.SignPlus = true
		o.Alternate = true
		o.ZeroPad = true
		o.DebugLowerHex = true
		width(8)
	case 12:
		o.SignMinus = true
		o.ZeroPad = true
		width(8)
	case 13:
		o.Fill = '🦀'
		o.Align = rustfmt.AlignCenter
		width(12)
	case 14:
		o.Fill = '\n'
		o.Align = rustfmt.AlignRight
		width(8)
	case 15:
		o.Fill = 0
		o.Align = rustfmt.AlignRight
		width(8)
	}
	return o
}
func defIDFormat(q defIDRequest) any {
	a := DefId{Krate: CrateNumFromU32(q.Crate), Index: DefIndexFromU32(q.Index)}
	fp := DefPathHash{datastructures.NewFingerprint(rustchashes.NewHash64(q.First), rustchashes.NewHash64(q.Second))}
	var call func(*rustfmt.Formatter) error
	switch q.Kind {
	case "crate":
		call = a.Krate.WriteDebugFormatter
	case "crate_display":
		call = a.Krate.WriteDisplayFormatter
	case "stable_hex":
		call = fp.StableCrateId().WriteLowerHexFormatter
	case "index":
		call = a.Index.WriteDebugFormatter
	case "id":
		call = a.WriteDebugFormatter
	case "local":
		call = (LocalDefId{a.Index}).WriteDebugFormatter
	case "mod":
		call = NewModIdUnchecked(a).WriteDebugFormatter
	case "localmod":
		call = NewLocalModIdUnchecked(LocalDefId{a.Index}).WriteDebugFormatter
	case "path":
		call = fp.WriteDebugFormatter
	case "stable":
		call = fp.StableCrateId().WriteDebugFormatter
	case "controls":
		call = (datastructures.StableHashControls{HashSpans: q.Exe}).WriteDebugFormatter
	default:
		panic("unknown format kind")
	}
	sink := &defIDSink{calls: []string{}, fail: q.Fail, chars: q.Chars}
	f := rustfmt.Formatter{Writer: sink, Options: defIDOptions(q.Mode)}
	e := call(&f)
	return map[string]any{"error": e != nil, "text": sink.text.String(), "calls": sink.calls, "after": [2]uint64{uint64(f.Options.Fill), uint64(f.Options.Align)}}
}
func defIDObserve(q defIDRequest) any {
	result, failed := defIDCatch(func() any {
		switch q.Op {
		case "full_hash":
			return defIDFullHash(q)
		case "core":
			return defIDCore(q)
		case "codec":
			return defIDCodec(q)
		case "new_crate":
			version := q.Cfg
			if q.VersionHex != nil {
				raw, e := hex.DecodeString(*q.VersionHex)
				if e != nil {
					panic(e)
				}
				version = defIdLossyUTF8(string(raw))
			}
			value := newStableCrateIdText(q.Name, q.Exe, q.Metadata, version, rustfmt.Context{})
			return map[string]any{"id": value.AsU64(), "debug": value.RustDebug()}

		case "step":
			return defIDStep(q)
		case "format":
			return defIDFormat(q)
		default:
			panic("unknown operation")
		}
	})
	if failed {
		return map[string]any{"panic": result}
	}
	return result
}
func TestDefIdOriginalOracle(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("native reference records amd64; cross builds checked separately")
	}
	path := os.Getenv("FERRUGO_DEF_ID_FIXTURE")
	if path == "" {
		path = "testdata/def_id_oracle.jsonl.gz"
	}
	file, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer file.Close()
	reader, e := gzip.NewReader(file)
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	scan := bufio.NewScanner(reader)
	scan.Buffer(make([]byte, 4096), 4<<20)
	n := 0
	for scan.Scan() {
		n++
		var row struct {
			Request defIDRequest
			Result  json.RawMessage
		}
		if e := json.Unmarshal(scan.Bytes(), &row); e != nil {
			t.Fatal(e)
		}
		actual, e := json.Marshal(defIDObserve(row.Request))
		if e != nil {
			t.Fatal(e)
		}
		var a, b any
		dec := json.NewDecoder(strings.NewReader(string(actual)))
		dec.UseNumber()
		if e := dec.Decode(&a); e != nil {
			t.Fatal(e)
		}
		dec = json.NewDecoder(strings.NewReader(string(row.Result)))
		dec.UseNumber()
		if e := dec.Decode(&b); e != nil {
			t.Fatal(e)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("row%d %+v\nGo%s\nRust%s", n, row.Request, actual, row.Result)
		}
	}
	if e := scan.Err(); e != nil {
		t.Fatal(e)
	}
	if n < 10000 {
		t.Fatalf("incomplete native corpus:%d", n)
	}
}
