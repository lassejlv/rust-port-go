package util

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
)

type cargoSemverReader struct {
	*bytes.Reader
	t *testing.T
}

func (r cargoSemverReader) byte() byte {
	v, e := r.ReadByte()
	if e != nil {
		r.t.Fatal(e)
	}
	return v
}
func (r cargoSemverReader) u32() uint32 {
	var n uint32
	if e := binary.Read(r, binary.LittleEndian, &n); e != nil {
		r.t.Fatal(e)
	}
	return n
}
func (r cargoSemverReader) u64() uint64 {
	var n uint64
	if e := binary.Read(r, binary.LittleEndian, &n); e != nil {
		r.t.Fatal(e)
	}
	return n
}
func (r cargoSemverReader) data(n uint32) []byte {
	if uint64(n) > uint64(r.Len()) {
		r.t.Fatal("short semver fixture")
	}
	b := make([]byte, int(n))
	if _, e := io.ReadFull(r, b); e != nil {
		r.t.Fatal(e)
	}
	return b
}
func (r cargoSemverReader) text() string { return string(r.data(r.u32())) }
func (r cargoSemverReader) texts() []string {
	values := make([]string, int(r.u32()))
	for i := range values {
		values[i] = r.text()
	}
	return values
}
func (r cargoSemverReader) magic(want string) {
	if got := string(r.data(uint32(len(want)))); got != want {
		r.t.Fatalf("semver fixture magic %q, want %q", got, want)
	}
}

type cargoSemverTrace struct {
	events  []string
	panicAt int
}

func (h *cargoSemverTrace) event(value string) {
	h.events = append(h.events, value)
	if h.panicAt != 0 && len(h.events) == h.panicAt {
		panic("hasher panic")
	}
}
func (h *cargoSemverTrace) Write(v []byte)    { h.event(fmt.Sprintf("write:%x", v)) }
func (h *cargoSemverTrace) WriteU8(v uint8)   { h.event(fmt.Sprintf("write_u8:%d", v)) }
func (h *cargoSemverTrace) WriteU16(v uint16) { h.event(fmt.Sprintf("write_u16:%d", v)) }
func (h *cargoSemverTrace) WriteU32(v uint32) { h.event(fmt.Sprintf("write_u32:%d", v)) }
func (h *cargoSemverTrace) WriteU64(v uint64) { h.event(fmt.Sprintf("write_u64:%d", v)) }
func (h *cargoSemverTrace) WriteUsize(v uint) { h.event(fmt.Sprintf("write_usize:%d", v)) }
func (h *cargoSemverTrace) WriteIsize(v int)  { h.event(fmt.Sprintf("write_isize:%d", v)) }

type cargoSemverState struct {
	Display, Debug                string
	Exact, Locked, Precise        bool
	LockedVersion, PreciseVersion *string
	CloneEqual                    bool
	Hash                          []string
}

func cargoSemverReadState(r cargoSemverReader) cargoSemverState {
	s := cargoSemverState{Display: r.text(), Debug: r.text(), Exact: r.byte() != 0, Locked: r.byte() != 0, Precise: r.byte() != 0}
	if r.byte() != 0 {
		value := r.text()
		s.LockedVersion = &value
	}
	if r.byte() != 0 {
		value := r.text()
		s.PreciseVersion = &value
	}
	s.CloneEqual = r.byte() != 0
	s.Hash = r.texts()
	return s
}
func cargoSemverObserve(value *OptVersionReq) cargoSemverState {
	s := cargoSemverState{Display: value.String(), Debug: value.RustDebug(), Exact: value.IsExact(), Locked: value.IsLocked(), Precise: value.IsPrecise(), CloneEqual: value.Equal(value.Clone())}
	if v, ok := value.LockedVersion(); ok {
		text := v.String()
		s.LockedVersion = &text
	}
	if v, ok := value.PreciseVersion(); ok {
		text := v.String()
		s.PreciseVersion = &text
	}
	h := &cargoSemverTrace{}
	value.Hash(h)
	s.Hash = h.events
	return s
}
func cargoSemverCatch(call func() byte) (result byte, panicText string) {
	defer func() {
		if value := recover(); value != nil {
			panicText = fmt.Sprint(value)
			if strings.Contains(panicText, "attempt to add with overflow") {
				panicText = "overflow"
			}
		}
	}()
	result = call()
	return
}
func cargoSemverBool(value bool) byte {
	if value {
		return 2
	}
	return 1
}

type cargoSemverAction struct {
	op    byte
	value string
}
type cargoSemverRequest struct {
	kind, constructor    byte
	overflow             SemverOverflowMode
	requirement, version string
	raw                  bool
	comparators          []semver.Comparator
	actions              []cargoSemverAction
}

func cargoSemverReadRequest(q cargoSemverReader) cargoSemverRequest {
	c := cargoSemverRequest{kind: q.byte()}
	if q.byte() == 0 {
		c.overflow = SemverWrappingOverflow
	}
	c.constructor, c.requirement, c.version = q.byte(), q.text(), q.text()
	c.raw = q.byte() != 0
	c.comparators = make([]semver.Comparator, int(q.u32()))
	for i := range c.comparators {
		v := semver.Comparator{Op: semver.Op(q.byte()), Major: q.u64()}
		if q.byte() != 0 {
			v.Minor = semver.Some(q.u64())
		}
		if q.byte() != 0 {
			v.Patch = semver.Some(q.u64())
		}
		var err error
		v.Pre, err = semver.NewPrerelease(q.text())
		if err != nil {
			q.t.Fatal(err)
		}
		c.comparators[i] = v
	}
	c.actions = make([]cargoSemverAction, int(q.u32()))
	for i := range c.actions {
		c.actions[i] = cargoSemverAction{q.byte(), q.text()}
	}
	return c
}
func cargoSemverConstruct(c cargoSemverRequest) (OptVersionReq, semver.VersionReq, semver.Version, string) {
	req := semver.VersionReq{Comparators: c.comparators}
	var err error
	if !c.raw {
		req, err = semver.ParseVersionReq(c.requirement)
		if err != nil {
			return OptVersionReq{}, req, semver.Version{}, "requirement: " + err.Error()
		}
	}
	version, err := semver.ParseVersion(c.version)
	if err != nil {
		return OptVersionReq{}, req, version, "version: " + err.Error()
	}
	var value OptVersionReq
	switch c.constructor {
	case 0:
	case 1, 6:
		value = FromVersionReq(req)
	case 2:
		value = LockedVersionReq(version, req)
	case 3:
		value = PreciseVersionReq(version, req)
	case 4:
		value = ExactVersionReq(version)
	case 5:
		value = LockToExactVersionReq(version)
	default:
		panic("bad semver constructor fixture")
	}
	return value, req, version, ""
}
func cargoSemverAct(value *OptVersionReq, req semver.VersionReq, action cargoSemverAction, overflow SemverOverflowMode, trace *cargoSemverTrace) byte {
	switch action.op {
	case 0:
		return 0
	case 7:
		*value = OptVersionReq{}
		return 0
	case 8:
		trace.panicAt, _ = strconv.Atoi(action.value)
		value.Hash(trace)
		return 0
	case 11:
		*value = FromVersionReq(req)
		return 0
	case 12:
		kind, text, _ := strings.Cut(action.value, "|")
		version, _ := semver.ParseVersion(text)
		var other OptVersionReq
		switch kind {
		case "0":
		case "1":
			other = FromVersionReq(req)
		case "2":
			other = LockedVersionReq(version, req)
		case "3":
			other = PreciseVersionReq(version, req)
		default:
			panic("bad equality fixture")
		}
		return cargoSemverBool(value.Equal(other))
	}
	version, err := semver.ParseVersion(action.value)
	if err != nil {
		panic(err)
	}
	switch action.op {
	case 1:
		return cargoSemverBool(value.MatchesWithOverflow(version, VersionReqDefault, overflow))
	case 2:
		return cargoSemverBool(value.MatchesWithOverflow(version, VersionReqPrerelease, overflow))
	case 3:
		return cargoSemverBool(MatchesPrereleaseWithOverflow(req, version, overflow))
	case 4:
		value.LockToWithOverflow(version, VersionReqDefault, overflow)
	case 5:
		value.LockToWithOverflow(version, VersionReqPrerelease, overflow)
	case 6:
		value.PreciseTo(version)
	case 9:
		*value = ExactVersionReq(version)
	case 10:
		*value = LockToExactVersionReq(version)
	default:
		panic("bad semver fixture action")
	}
	return 0
}

type cargoSemverTraitBase struct {
	version semver.Version
	log     *[]string
	mask    uint8
}

func (b *cargoSemverTraitBase) call(label string, mask uint8) {
	*b.log = append(*b.log, label)
	if b.mask&mask != 0 {
		panic("custom trait panic")
	}
}
func (b *cargoSemverTraitBase) IsPrerelease() bool {
	b.call("is_prerelease", 1)
	return VersionAdapter{b.version}.IsPrerelease()
}
func (b *cargoSemverTraitBase) ToReq(op semver.Op) semver.VersionReq {
	b.call("to_req:"+op.RustDebug(), 2)
	return VersionAdapter{b.version}.ToReq(op)
}
func (b *cargoSemverTraitBase) exact() semver.VersionReq {
	b.call("to_exact_req", 4)
	return VersionAdapter{b.version}.ToReq(semver.Tilde)
}
func (b *cargoSemverTraitBase) caret() semver.VersionReq {
	b.call("to_caret_req", 8)
	return VersionAdapter{b.version}.ToReq(semver.Greater)
}

type cargoSemverTraitExact struct{ *cargoSemverTraitBase }

func (b cargoSemverTraitExact) ToExactReq() semver.VersionReq { return b.exact() }

type cargoSemverTraitCaret struct{ *cargoSemverTraitBase }

func (b cargoSemverTraitCaret) ToCaretReq() semver.VersionReq { return b.caret() }

type cargoSemverTraitBoth struct{ *cargoSemverTraitBase }

func (b cargoSemverTraitBoth) ToExactReq() semver.VersionReq { return b.exact() }
func (b cargoSemverTraitBoth) ToCaretReq() semver.VersionReq { return b.caret() }

type cargoSemverCustomReq struct{ base *cargoSemverTraitBase }

func (r cargoSemverCustomReq) MatchesPrerelease(v semver.Version) bool {
	r.base.call("req_ext:"+v.String(), 16)
	return v.Patch%2 == 0
}
func cargoSemverTraits(t *testing.T, c cargoSemverRequest, r cargoSemverReader, index uint32) int {
	panics := 0
	if err := r.text(); err != "" {
		t.Fatalf("trait constructor: %s", err)
	}
	version, err := semver.ParseVersion(c.version)
	if err != nil {
		t.Fatal(err)
	}
	mask, _ := strconv.Atoi(c.requirement)
	var log []string
	base := &cargoSemverTraitBase{version, &log, uint8(mask)}
	var value VersionExt = base
	switch c.constructor {
	case 0:
	case 1:
		value = cargoSemverTraitExact{base}
	case 2:
		value = cargoSemverTraitCaret{base}
	case 3:
		value = cargoSemverTraitBoth{base}
	default:
		t.Fatal("bad trait fixture")
	}
	if r.u32() != uint32(len(c.actions)) {
		t.Fatal("trait action count differs")
	}
	for step, action := range c.actions {
		wantPanic, want := r.text(), r.text()
		wantLog := r.texts()
		actual := ""
		_, panicText := cargoSemverCatch(func() byte {
			switch action.op {
			case 0:
				actual = strconv.FormatBool(value.IsPrerelease())
			case 1:
				operation, _ := strconv.Atoi(action.value)
				req := value.ToReq(semver.Op(operation))
				actual = req.String() + "\n" + req.RustDebug()
			case 2:
				req := VersionToExactReq(value)
				actual = req.String() + "\n" + req.RustDebug()
			case 3:
				req := VersionToCaretReq(value)
				actual = req.String() + "\n" + req.RustDebug()
			case 4:
				v, _ := semver.ParseVersion(action.value)
				var req VersionReqExt = cargoSemverCustomReq{base}
				actual = strconv.FormatBool(req.MatchesPrerelease(v))
			default:
				t.Fatal("bad trait action")
			}
			return 0
		})
		if panicText != "" {
			panics++
		}
		if wantPanic != panicText || want != actual || !slices.Equal(wantLog, log) {
			t.Fatalf("request %d trait variant %d mask %d action %d/%d: panic %q want %q, value %q want %q, log %v want %v", index, c.constructor, mask, step, action.op, panicText, wantPanic, actual, want, log, wantLog)
		}
	}
	return panics
}

func TestOriginalCargoSemverOracle(t *testing.T) {
	path := "testdata/semver_cases.bin.gz"
	compressed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile("../../../../tools/port-cargo-semver/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Hashes map[string]string `json:"sha256"`
		Cases  int               `json:"cases"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(compressed)
	if hex.EncodeToString(sum[:]) != manifest.Hashes[path] {
		t.Fatal("semver fixture compressed hash differs")
	}
	zip, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(io.LimitReader(zip, 256<<20))
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 256<<20 {
		t.Fatal("oversized semver fixture")
	}
	if err := zip.Close(); err != nil {
		t.Fatal(err)
	}
	sum = sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != manifest.Hashes[path+".raw"] {
		t.Fatal("semver fixture raw hash differs")
	}
	all := cargoSemverReader{bytes.NewReader(data), t}
	all.magic("CGSC")
	profiles := all.u32()
	if profiles != 2 {
		t.Fatal("missing semver overflow profiles")
	}
	total, steps, panics := 0, 0, 0
	for profile := uint32(0); profile < profiles; profile++ {
		q := cargoSemverReader{bytes.NewReader(all.data(all.u32())), t}
		r := cargoSemverReader{bytes.NewReader(all.data(all.u32())), t}
		q.magic("CGSR")
		count := q.u32()
		r.magic("CGSO")
		if r.u32() != count {
			t.Fatal("semver request/response count differs")
		}
		for index := uint32(0); index < count; index++ {
			c := cargoSemverReadRequest(q)
			total++
			if r.byte() != c.kind {
				t.Fatal("semver response kind differs")
			}
			if c.kind == 1 {
				panics += cargoSemverTraits(t, c, r, index)
				steps += len(c.actions)
				continue
			}
			value, req, version, constructorError := cargoSemverConstruct(c)
			if wanted := r.text(); wanted != constructorError {
				t.Fatalf("request %d profile %d req %q version %q: constructor error %q want %q", index, profile, c.requirement, c.version, constructorError, wanted)
			}
			if constructorError != "" {
				continue
			}
			if c.kind == 2 {
				for op := 0; op < 4; op++ {
					wantPanic, want := r.text(), r.byte()
					actual, panicText := cargoSemverCatch(func() byte {
						switch op {
						case 0:
							return cargoSemverBool(req.Matches(version))
						case 1:
							return cargoSemverBool(MatchesPrereleaseWithOverflow(req, version, c.overflow))
						case 2:
							return cargoSemverBool(valueFromReq(req).MatchesWithOverflow(version, VersionReqDefault, c.overflow))
						default:
							return cargoSemverBool(valueFromReq(req).MatchesWithOverflow(version, VersionReqPrerelease, c.overflow))
						}
					})
					steps++
					if panicText != "" {
						panics++
					}
					if actual != want || panicText != wantPanic {
						t.Fatalf("request %d profile %d op %d req %s/%+v version %s: got %d panic %q want %d panic %q", index, profile, op, req.String(), req.Comparators, version.String(), actual, panicText, want, wantPanic)
					}
				}
				continue
			}
			if wanted, actual := cargoSemverReadState(r), cargoSemverObserve(&value); !reflect.DeepEqual(wanted, actual) {
				t.Fatalf("request %d profile %d initial state differs\nactual: %+v\nwant: %+v", index, profile, actual, wanted)
			}
			if r.u32() != uint32(len(c.actions)) {
				t.Fatal("semver action count differs")
			}
			for step, action := range c.actions {
				wantPanic, want, wantEvents := r.text(), r.byte(), r.texts()
				wantState := cargoSemverReadState(r)
				trace := &cargoSemverTrace{}
				actual, panicText := cargoSemverCatch(func() byte { return cargoSemverAct(&value, req, action, c.overflow, trace) })
				actualState := cargoSemverObserve(&value)
				steps++
				if panicText != "" {
					panics++
				}
				if want != actual || wantPanic != panicText || !slices.Equal(wantEvents, trace.events) || !reflect.DeepEqual(wantState, actualState) {
					t.Fatalf("request %d profile %d ctor %d req %q version %q action %d/%d (%q): got %d panic %q events %v\nstate %+v\nwant %d panic %q events %v\nstate %+v", index, profile, c.constructor, c.requirement, c.version, step, action.op, action.value, actual, panicText, trace.events, actualState, want, wantPanic, wantEvents, wantState)
				}
			}
		}
		if q.Len() != 0 || r.Len() != 0 {
			t.Fatalf("trailing semver profile bytes %d/%d", q.Len(), r.Len())
		}
	}
	if all.Len() != 0 || total != manifest.Cases {
		t.Fatalf("semver corpus count=%d want %d trailing=%d", total, manifest.Cases, all.Len())
	}
	t.Logf("%d original requests, %d matching/mutator/hash/custom-trait observations, %d caught panics", total, steps, panics)
}
func valueFromReq(req semver.VersionReq) OptVersionReq {
	return OptVersionReq{Kind: OptVersionRequirement, Req: req}
}
