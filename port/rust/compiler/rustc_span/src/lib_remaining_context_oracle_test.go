// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type libRemainingRequest struct {
	Op, Mode, Strategy          string
	Call, Drop, Debug, Overflow bool
}
type libRemainingRow struct {
	Request libRemainingRequest
	Want    struct {
		Aborted bool
		Events  []string
	}
}
type libRemainingManifest struct {
	ArtifactKind string `json:"artifact_kind"`
	Revision     string `json:"source_revision"`
	ManifestSHA  string `json:"-"`
	Records      int
	Aborts       int                          `json:"actual_aborts"`
	RawSHA       string                       `json:"raw_sha256"`
	FixtureSHA   string                       `json:"fixture_sha256"`
	Locations    map[string][]json.RawMessage `json:"source_locations"`
	Files        map[string]string
}
type libRemainingCallback struct{ state *libRemainingCallbackState }
type libRemainingCallbackState struct {
	calls   uint32
	initial *libRemainingCallbackState
	result  Shared[struct{}]
}
type libRemainingResult struct{ value Shared[struct{}] }
type libRemainingState struct {
	q         libRemainingRequest
	cfg       SpanEncodingConfig
	scope     *SessionScope
	session   *SessionGlobals
	span      Span
	id        ExpnId
	source    SourceFile
	callback  *rustvec.Owned[libRemainingCallback]
	witness   Shared[struct{}]
	events    []string
	trace     bool
	locations map[string]rustfmt.SourceLocation
}

func libRemainingRuntime(c rustfmt.Context) int {
	if c.Runtime == nil {
		return 0
	}
	v, _ := c.Runtime.Value.(int)
	return v
}
func (s *libRemainingState) event(c rustfmt.Context, text string) {
	value := fmt.Sprintf("%s:%t:%d", text, c.Panicking(), libRemainingRuntime(c))
	s.events = append(s.events, value)
	if s.trace {
		fmt.Fprintln(os.Stderr, "EVENT:"+value)
	}
}
func (s *libRemainingState) context(runtime int) rustfmt.Context {
	if runtime == 0 {
		return rustfmt.Context{}
	}
	cleanup := rustfmt.CleanupPanicInfoNightly20261104()
	locations := map[string]rustfmt.SourceLocation{}
	for key, value := range s.locations {
		locations[key] = value
	}
	ops := &rustfmt.PanicOperations{Locations: locations, CleanupAbort: &cleanup}
	ops.Hook = func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
		kind := "opaque"
		if info.Kind == rustfmt.PanicStaticStr {
			kind = "static_str"
		}
		if info.Kind == rustfmt.PanicOwnedString {
			kind = "owned_string"
		}
		file := ""
		var line, column uint32
		if info.Location != nil {
			file = info.Location.File
			line = info.Location.Line
			column = info.Location.Column
		}
		s.event(current, fmt.Sprintf("hook:%s:%s:%s:%d:%d:%t:%t", kind, hex.EncodeToString([]byte(fmt.Sprint(info.Payload))), hex.EncodeToString([]byte(file)), line, column, info.CanUnwind, info.ForceNoBacktrace))
	}
	return rustfmt.WithPanicOperations(rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: runtime}}, ops)
}
func (s *libRemainingState) raise(c rustfmt.Context, text string) {
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: text, Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: c.SourcePanicLocation(text)})
}
func (s *libRemainingState) prepare(c rustfmt.Context) {
	s.session = NewSessionGlobalsWithContext(Edition2021, nil, None[SourceMapInputs](), c, s.cfg)
	d := NewExpnData(ExpnKind{Tag: ExpnMacro, MacroKind: BangMacro, Name: InternSymbol(s.session, "remaining_macro")}, RootExpnId(), DummySpan(), DummySpan(), None[Shared[[]Symbol]](), Edition2021, None[DefId](), None[ModId](), false, false, false, false)
	s.id = FreshLocalExpnId(s.session, d, spanWrapperHcx{}).ToExpnId()
	ctxt := RootSyntaxContext().ApplyMark(s.session, s.id, Opaque)
	s.span = NewSpan(s.session, 7, 40007, ctxt, Some(LocalDefId{DefIndexFromU32(9)}), s.cfg)
	var err error
	if s.q.Op == "lines_bad0" || s.q.Op == "lines_bad3" {
		width := byte(0)
		if s.q.Op == "lines_bad3" {
			width = 3
		}
		s.source = cachingOracleRawLineSource(1, width, nil)
	} else {
		s.source, err = NewSourceFile(CustomFileName("remaining"), "a\nb\n", SourceFileHashMd5, None[SourceFileHashAlgorithm]())
		if err != nil {
			panic(err)
		}
	}
	if strings.HasSuffix(s.q.Op, "present") {
		s.scope.current = s.session
	} else {
		s.session.CloseWithContext(c)
		s.session = nil
	}
	s.event(c, "prepared_input")
	if s.isOwned() {
		s.witness = NewShared(struct{}{}, nil)
		state := &libRemainingCallbackState{result: s.witness.Clone()}
		state.initial = state
		env := rustvec.NewOwned(libRemainingCallback{state: state}, rustvec.ValueOps[libRemainingCallback]{Drop: func(callback *libRemainingCallback, current rustfmt.Context) {
			value := callback.state
			defer spanCloseOwnedCallback(current, func(drop rustfmt.Context) { s.event(drop, "env_field_drop"); value.result.CloseWithContext(drop) })
			s.event(current, fmt.Sprintf("env_drop:%d:%t:%d", value.calls, value.initial == value, value.result.StrongCount()))
			if s.q.Op != "meta_absent" && s.q.Op != "with_absent" && s.q.Op != "create_ctorfail" {
				s.event(current, "env_drop_edition:"+RootSyntaxContext().EditionWithContext(s.scope.CurrentWithContext(current), current).String())
			}
			if s.q.Drop {
				s.raise(current, "drop")
			}
		}})
		s.callback = env.MoveWithContext(c)
		s.event(c, fmt.Sprintf("construct:%d", s.witness.StrongCount()))
	}
}
func (s *libRemainingState) shape(value Span, c rustfmt.Context) string {
	d := value.DataUntrackedWithContext(s.session, c, s.cfg)
	parent := "_"
	if p, ok := d.Parent.Get(); ok {
		parent = fmt.Sprint(p.LocalDefIndex.AsU32())
	}
	return fmt.Sprintf("%d,%d,%d,%s", d.Lo, d.Hi, d.Ctxt.AsU32(), parent)
}
func (s *libRemainingState) surface(c rustfmt.Context) {
	op := strings.Split(s.q.Op, "_")[0]
	value := ""
	switch op {
	case "def":
		value = s.shape(s.span.WithDefSiteCtxtWithContext(s.session, s.id, c, s.cfg), c)
	case "call":
		value = s.shape(s.span.WithCallSiteCtxtWithContext(s.session, s.id, c, s.cfg), c)
	case "mixed":
		value = s.shape(s.span.WithMixedSiteCtxtWithContext(s.session, s.id, c, s.cfg), c)
	case "ed2015":
		value = fmt.Sprint(s.span.IsRust2015WithContext(s.session, c))
	case "ed2018":
		value = fmt.Sprint(s.span.AtLeastRust2018WithContext(s.session, c))
	case "ed2021":
		value = fmt.Sprint(s.span.AtLeastRust2021WithContext(s.session, c))
	case "ed2024":
		value = fmt.Sprint(s.span.AtLeastRust2024WithContext(s.session, c))
	case "subst":
		a := s.span
		if s.q.Op == "subst_dummy" {
			a = DummySpan()
		}
		value = s.shape(a.SubstituteDummyWithContext(s.session, WithRootCtxt(nil, 10, 20), c), c)
	case "lines":
		value = fmt.Sprint(s.source.CountLinesWithContext(c, s.cfg))
	default:
		panic(op)
	}
	s.event(c, "result:"+value)
}
func (s *libRemainingState) caught(c rustfmt.Context, call func()) {
	defer func() {
		if p := recover(); p != nil {
			s.event(c, "caught:"+hex.EncodeToString([]byte(fmt.Sprint(p))))
		} else {
			s.event(c, "caught:none")
		}
	}()
	call()
}
func (s *libRemainingState) invoke(ref rustvec.Ref[libRemainingCallbackState]) *rustvec.Owned[libRemainingResult] {
	value, c := ref.Value, ref.Context
	s.event(c, fmt.Sprintf("call:%d:%t:%d", value.calls, value.initial == value, value.result.StrongCount()))
	value.calls++
	s.event(c, "edition:"+RootSyntaxContext().EditionWithContext(s.scope.CurrentWithContext(c), c).String())
	if s.q.Call {
		s.raise(c, "callback")
	}
	result := rustvec.NewOwned(libRemainingResult{value: value.result.CloneWithContext(c)}, rustvec.ValueOps[libRemainingResult]{Drop: func(value *libRemainingResult, current rustfmt.Context) {
		s.event(current, fmt.Sprintf("result_drop:%d", value.value.StrongCount()))
		value.value.CloseWithContext(current)
	}})
	s.event(c, fmt.Sprintf("prepared:%d", result.Borrow().value.StrongCount()))
	return result
}
func (s *libRemainingState) owned(c rustfmt.Context) {
	s.caught(c, func() {
		var result *rustvec.Owned[libRemainingResult]
		call := func(owned *rustvec.Owned[libRemainingCallback], current rustfmt.Context) *rustvec.Owned[libRemainingResult] {
			var state *libRemainingCallbackState
			if s.q.Strategy == "early" {
				s.event(current, "early_forget_self")
				state = owned.Take().state
			} else {
				state = owned.Borrow().state
			}
			result := s.invoke(rustvec.Ref[libRemainingCallbackState]{Value: state, Context: current})
			if s.q.Strategy == "late" {
				s.event(current, "forget_self")
				owned.Take()
			}
			return result
		}
		noArgs := func(owned *rustvec.Owned[libRemainingCallback], _ *SessionScope, current rustfmt.Context) *rustvec.Owned[libRemainingResult] {
			return call(owned, current)
		}
		sessionArg := func(owned *rustvec.Owned[libRemainingCallback], _ *SessionGlobals, current rustfmt.Context) *rustvec.Owned[libRemainingResult] {
			return call(owned, current)
		}
		switch s.q.Op {
		case "default", "default_present":
			result = CreateDefaultSessionGlobalsThenOwned(s.scope, s.callback, noArgs, c, s.cfg)
		case "meta_present", "meta_absent":
			result = WithMetavarSpansOwned(s.scope, s.callback, func(owned *rustvec.Owned[libRemainingCallback], m *MetavarSpansMap, current rustfmt.Context) *rustvec.Owned[libRemainingResult] {
				span := WithRootCtxt(nil, 1, 3)
				inserted := m.InsertWithContext(DummySpan(), span, current)
				same := m.GetWithContext(DummySpan(), current) == Some(span)
				s.event(current, fmt.Sprintf("metavar:%t:%t", inserted, same))
				return call(owned, current)
			}, c)
		case "set", "set_present":
			session := NewSessionGlobalsWithContext(Edition2018, nil, None[SourceMapInputs](), c, s.cfg)
			defer spanCloseOwnedCallback(c, session.CloseWithContext)
			result = SetSessionGlobalsThenConsuming(s.scope, session, s.callback, noArgs, c)
		case "with_present", "with_absent":
			result = WithSessionGlobalsConsuming(s.scope, s.callback, sessionArg, c)
		case "create", "create_present", "create_ctorfail":
			var extra []string
			if s.q.Op == "create_ctorfail" {
				extra = []string{"remaining_duplicate_symbol", "remaining_duplicate_symbol"}
			}
			result = CreateSessionGlobalsThenConsuming(s.scope, Edition2018, extra, None[SourceMapInputs](), s.callback, noArgs, c, s.cfg)
		case "if_present", "if_absent":
			result = CreateSessionIfNotSetThenConsuming(s.scope, Edition2018, s.callback, sessionArg, c, s.cfg)
		default:
			panic(s.q.Op)
		}
		s.event(c, fmt.Sprintf("returned:%d", result.Borrow().value.StrongCount()))
		result.DropWithContext(c)
	})
	s.event(c, fmt.Sprintf("witness:%d", s.witness.StrongCount()))
	s.witness.CloseWithContext(c)
	if !strings.HasSuffix(s.q.Op, "present") {
		CreateDefaultSessionGlobalsThenOwned(s.scope, rustvec.Own(struct{}{}), func(_ *rustvec.Owned[struct{}], _ *SessionScope, _ rustfmt.Context) struct{} {
			s.event(c, "scope_restored")
			return struct{}{}
		}, c, s.cfg)
	} else {
		WithSessionGlobalsOwned(s.scope, NewSpanOwned(struct{}{}, nil), func(_ *struct{}, _ *SessionGlobals, current rustfmt.Context) struct{} {
			s.event(current, "scope_present")
			return struct{}{}
		}, c)
	}
}
func (s *libRemainingState) target(c rustfmt.Context) {
	if s.isOwned() {
		s.owned(c)
	} else {
		s.caught(c, func() { s.surface(c) })
	}
}
func libRemainingLocations(m libRemainingManifest, op string) map[string]rustfmt.SourceLocation {
	out := map[string]rustfmt.SourceLocation{}
	for composite, raw := range m.Locations {
		prefix := op + "|"
		if !strings.HasPrefix(composite, prefix) {
			continue
		}
		payload := strings.TrimPrefix(composite, prefix)
		var value rustfmt.SourceLocation
		json.Unmarshal(raw[0], &value.File)
		json.Unmarshal(raw[1], &value.Line)
		json.Unmarshal(raw[2], &value.Column)
		key := payload
		switch payload {
		case "cannot access a scoped thread local variable without calling `set` first":
			key = ScopedSessionPanicSite
		case "SESSION_GLOBALS should never be overwritten! Use another thread if you need another SessionGlobals":
			if op == "set_present" {
				key = LibPanicSite(164, 5)
			} else {
				key = LibPanicSite(154, 5)
			}
		case "attempt to divide by zero":
			key = LibPanicSite(2281, 32)
		case "internal error: entered unreachable code":
			key = LibPanicSite(2312, 18)
		}
		if strings.HasPrefix(payload, "duplicate symbols in the rustc symbol list") {
			key = PanicSymbolDuplicatePrefillSite
		}
		out[key] = value
	}
	return out
}
func libRemainingExecute(q libRemainingRequest, m libRemainingManifest, trace bool) []string {
	SetDynThreadSafeMode(true)
	s := &libRemainingState{q: q, cfg: SpanEncodingConfig{q.Debug, q.Overflow}, scope: NewSessionScope(), trace: trace, locations: libRemainingLocations(m, q.Op)}
	s.prepare(s.context(1))
	runtime := 2
	if q.Mode == "normal" {
		runtime = 1
	}
	if q.Mode == "nil" {
		runtime = 0
	}
	c := s.context(runtime)
	s.event(c, "invoke")
	switch q.Mode {
	case "normal", "currentB", "nil":
		s.target(c)
	case "unwinding":
		func() {
			defer func() { p := recover(); s.event(c, fmt.Sprintf("outer_caught:%t", p != nil)) }()
			defer spanCloseOwnedCallback(c, func(u rustfmt.Context) { s.event(u, "ambient_drop"); s.target(u) })
			s.raise(c, "outer")
		}()
	case "processing":
		c.Runtime.Panic.Hook = func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
			file := ""
			var line, column uint32
			if info.Location != nil {
				file = info.Location.File
				line = info.Location.Line
				column = info.Location.Column
			}
			s.event(current, fmt.Sprintf("hook:static_str:%s:%s:%d:%d:%t:%t", hex.EncodeToString([]byte(fmt.Sprint(info.Payload))), hex.EncodeToString([]byte(file)), line, column, info.CanUnwind, info.ForceNoBacktrace))
			if info.Payload == "outer" {
				s.target(current)
			}
		}
		func() {
			defer func() { p := recover(); s.event(c, fmt.Sprintf("outer_caught:%t", p != nil)) }()
			s.raise(c, "outer")
		}()
	default:
		panic(q.Mode)
	}
	s.event(c, "after")
	s.source.CloseWithContext(c)
	if s.session != nil {
		s.scope.current = nil
		s.session.CloseWithContext(c)
	}
	return s.events
}
func libRemainingLoad(t *testing.T, selection libRemainingEvidence) (libRemainingManifest, []byte) {
	t.Helper()
	m, err := libRemainingProfileMetadata(selection, "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(selection.Fixture)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != m.FixtureSHA {
		t.Fatal("remaining fixture hash")
	}
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	raw, err := io.ReadAll(io.LimitReader(reader, (16<<20)+1))
	if err != nil || len(raw) > 16<<20 {
		t.Fatal("remaining raw bound", err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != m.RawSHA {
		t.Fatal("remaining raw hash")
	}
	return m, raw
}
func TestLibRemainingContextAbortChild(t *testing.T) {
	qraw := os.Getenv("FERRUGO_LIB_REMAINING_CHILD")
	if qraw == "" {
		return
	}
	var q libRemainingRequest
	if err := json.Unmarshal([]byte(qraw), &q); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join("..", "..", "..", "..", "..")
	selection, err := libRemainingEvidencePaths(root, os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	expected, present := os.LookupEnv(libRemainingManifestSHAEnv)
	if !present || expected == "" {
		t.Fatal("remaining child lacks captured parent producer")
	}
	m, err := libRemainingProfileMetadata(selection, expected)
	if err != nil {
		t.Fatal(err)
	}
	libRemainingExecute(q, m, true)
}
func TestLibOriginalRemainingContexts(t *testing.T) {
	selection, err := libRemainingEvidencePaths(filepath.Join("..", "..", "..", "..", ".."), os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	m, raw := libRemainingLoad(t, selection)
	selection.ManifestSHA = m.ManifestSHA
	files := []spanProofFile{}
	for path, hash := range m.Files {
		files = append(files, spanProofFile{Path: path, SHA256: hash})
	}
	verifySpanProofFiles(t, files)
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scan.Scan() {
		count++
		var row libRemainingRow
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		var got []string
		if row.Want.Aborted {
			aborts++
			q, _ := json.Marshal(row.Request)
			cmd := exec.Command(os.Args[0], "-test.run=^TestLibRemainingContextAbortChild$")
			cmd.Env = append(libRemainingChildEnvironment(os.Environ(), selection), "FERRUGO_LIB_REMAINING_CHILD="+string(q))
			out, err := cmd.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("remaining child%+v status%v %s", row.Request, err, out)
			}
			for _, line := range strings.Split(string(out), "\n") {
				if strings.HasPrefix(line, "EVENT:") {
					got = append(got, strings.TrimPrefix(line, "EVENT:"))
				}
			}
		} else {
			got = libRemainingExecute(row.Request, m, false)
		}
		if !reflect.DeepEqual(got, row.Want.Events) {
			t.Fatalf("remaining row%d %+v\nGo %q\nRust %q", count, row.Request, got, row.Want.Events)
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != m.Records || aborts != m.Aborts || count != 3520 || aborts != 1324 {
		t.Fatal("remaining counts", count, aborts, m.Records, m.Aborts)
	}
}

func (s *libRemainingState) isOwned() bool {
	switch strings.Split(s.q.Op, "_")[0] {
	case "def", "call", "mixed", "ed2015", "ed2018", "ed2021", "ed2024", "subst", "lines":
		return false
	}
	return true
}
