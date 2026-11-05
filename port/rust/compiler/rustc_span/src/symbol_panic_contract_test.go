// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type symbolPanicRequest struct {
	op        string
	mode      int
	value     uint64
	fail, aux int
}

func parseSymbolPanicRequest(line string) symbolPanicRequest {
	p := strings.Split(line, "\t")
	if len(p) != 5 {
		panic("invalid Symbol panic request")
	}
	mode, err := strconv.Atoi(p[1])
	if err != nil {
		panic(err)
	}
	value, err := strconv.ParseUint(p[2], 10, 64)
	if err != nil {
		panic(err)
	}
	fail, err := strconv.Atoi(p[3])
	if err != nil {
		panic(err)
	}
	aux, err := strconv.Atoi(p[4])
	if err != nil {
		panic(err)
	}
	return symbolPanicRequest{p[0], mode, value, fail, aux}
}

type symbolPanicLocation struct {
	File      string   `json:"file"`
	Line      uint32   `json:"line"`
	Column    uint32   `json:"column"`
	CanUnwind bool     `json:"can_unwind"`
	Force     bool     `json:"force_no_backtrace"`
	Kinds     []string `json:"kinds"`
}
type symbolPanicProfile struct {
	Name       string                         `json:"name"`
	Debug      bool                           `json:"debug_assertions"`
	Checked    bool                           `json:"overflow_checks"`
	Locations  map[string]symbolPanicLocation `json:"locations"`
	ThreadSafe bool                           `json:"dynamic_thread_safe"`
}
type symbolPanicWant struct {
	Abort  bool     `json:"abort"`
	Result string   `json:"result"`
	Trace  []string `json:"trace"`
}
type symbolPanicRecord struct {
	Request  string          `json:"request"`
	Want     symbolPanicWant `json:"want"`
	Profiles []string        `json:"profiles"`
}
type symbolPanicManifest struct {
	Revision   string               `json:"rust_revision"`
	Bindings   map[string]string    `json:"bindings"`
	Profiles   []symbolPanicProfile `json:"profiles"`
	Records    int                  `json:"native_observations"`
	Rows       int                  `json:"retained_rows"`
	RawBytes   int                  `json:"raw_bytes"`
	RawSHA     string               `json:"raw_sha256"`
	FixtureSHA string               `json:"fixture_sha256"`
}
type symbolPanicProbe struct {
	request        symbolPanicRequest
	profile        symbolPanicProfile
	phase          string
	trace          []string
	emit           bool
	ops            rustfmt.PanicOperations
	session        *SessionGlobals
	result         string
	diagnosticCell *rustfmt.SourceLocation
}

func (p *symbolPanicProbe) event(c rustfmt.Context, s string) {
	s = fmt.Sprintf("%s:%t:%s", s, c.Panicking(), p.phase)
	p.trace = append(p.trace, s)
	if p.emit {
		fmt.Println(s)
	}
}
func (p *symbolPanicProbe) ctx(c rustfmt.Context, phase string) rustfmt.Context {
	p.phase = phase
	c.Runtime = &rustfmt.RuntimeMetadata{Value: phase, Panic: &p.ops}
	return c
}
func (p *symbolPanicProbe) guard(c rustfmt.Context, name string, body func()) {
	defer func() {
		v := recover()
		if v != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(v)
			}
			c.Unwinding = true
		}
		p.event(c, "guard:"+name)
		if v != nil {
			c.ResumeUnwind(v)
		}
	}()
	body()
}
func (p *symbolPanicProbe) info(site, payload string) rustfmt.SourcePanicInfo {
	return rustfmt.SourcePanicInfo{Payload: payload, Kind: rustfmt.PanicStaticStr, Location: p.ctxLocation(site), CanUnwind: true}
}
func (p *symbolPanicProbe) ctxLocation(site string) *rustfmt.SourceLocation {
	if cell, ok := p.ops.LocationCells[site]; ok {
		return cell
	}
	v, ok := p.ops.Locations[site]
	if !ok {
		return nil
	}
	return &v
}
func (p *symbolPanicProbe) hook(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
	payload := fmt.Sprint(info.Payload)
	if _, ok := info.Payload.(ExplicitBug); ok {
		payload = "ExplicitBug"
	}
	file := ""
	var line, column uint32
	if info.Location != nil {
		file, line, column = info.Location.File, info.Location.Line, info.Location.Column
	}
	p.event(c, fmt.Sprintf("hook:%s:%x:%t:%t:%x:%d:%d", info.Kind, payload, info.CanUnwind, info.ForceNoBacktrace, file, line, column))
	if payload == "called `Ident::new` with empty symbol" || payload == "ExplicitBug" {
		p.event(c, fmt.Sprintf("location-same:%t", info.Location == p.diagnosticCell))
	}
	if p.request.op == "reenter-index-hook" {
		p.event(c, "reenter")
		NewSymbol(1).AsStrWithContext(p.session, c)
	}
	if p.request.mode == 3 {
		p.event(c, "processing")
		p.result = p.operation(c)
	}
}

type symbolPanicField struct {
	seen uint32
	fail bool
}
type symbolPanicEnvironment struct {
	value       uint32
	fail        bool
	field       symbolPanicField
	constructed string
}

func (p *symbolPanicProbe) ownedContext(c rustfmt.Context) string {
	p.phase = "construction"
	old := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "construction", Panic: &rustfmt.PanicOperations{Hook: func(_ rustfmt.SourcePanicInfo, c rustfmt.Context) { p.event(c, "stale-hook") }}}}
	env := symbolPanicEnvironment{value: 7, fail: p.request.fail&1 != 0, field: symbolPanicField{fail: p.request.fail&2 != 0}, constructed: "construction"}
	owned := rustvec.NewOwned(env, rustvec.ValueOps[symbolPanicEnvironment]{Drop: func(v *symbolPanicEnvironment, c rustfmt.Context) {
		spanDropFields(c, func(c rustfmt.Context) {
			p.guard(c, "context", func() {
				p.event(c, fmt.Sprintf("drop:%d", v.value))
				p.event(c, "saved-scope:"+v.constructed)
				p.event(c, "runtime-scope:"+c.Runtime.Value.(string))
				v.value++
				v.field.seen = v.value
				p.event(c, fmt.Sprintf("value:%d", v.value))
				s := InternSymbolWithContext(p.session, "symbol_drop_current_context", c)
				p.event(c, fmt.Sprintf("drop-symbol:%d:%x", s.AsU32(), s.AsStrWithContext(p.session, c)))
				if v.fail {
					c.PanicAt(p.info("symbol.client.context_drop", "context-drop"))
				}
			})
		}, func(c rustfmt.Context) {
			p.guard(c, "field", func() {
				p.event(c, fmt.Sprintf("field:%d", v.field.seen))
				if v.field.fail {
					c.PanicAt(p.info("symbol.client.field_drop", "field-drop"))
				}
			})
		})
	}}).MoveWithContext(old)
	factory := p.ctx(c, "factory")
	p.session = NewSessionGlobalsWithContext(DefaultEdition, nil, None[SourceMapInputs](), factory, SpanEncodingConfig{DebugAssertions: p.profile.Debug, OverflowChecks: p.profile.Checked})
	var compare SymbolStableComparator
	func() {
		defer p.session.CloseWithContext(factory)
		compare = SymbolStableComparatorOwnedWithContext(owned, factory)
	}()
	p.session = nil
	p.event(factory, "returned")
	for i, words := range [][2]string{{"zzzz_context", "aaaa_context"}, {"aaaa_context", "zzzz_context"}} {
		active := p.ctx(c, fmt.Sprintf("invoke%d", i+1))
		s := NewSessionGlobalsWithContext(DefaultEdition, nil, None[SourceMapInputs](), active, SpanEncodingConfig{DebugAssertions: p.profile.Debug, OverflowChecks: p.profile.Checked})
		a, b := InternSymbolWithContext(s, words[0], active), InternSymbolWithContext(s, words[1], active)
		order := compare(s, &a, &b, active)
		s.CloseWithContext(active)
		p.event(active, "compare:"+symbolOracleOrder(order))
	}
	p.ctx(c, "call")
	return "compared"
}

type symbolPanicSink struct {
	p           *symbolPanicProbe
	c           rustfmt.Context
	calls, fail int
}

func (s *symbolPanicSink) Write(b []byte) (int, error) { return s.WriteString(string(b)) }
func (s *symbolPanicSink) WriteString(v string) (int, error) {
	s.calls++
	s.p.event(s.c, fmt.Sprintf("write:%x", v))
	if s.calls == s.fail {
		return 0, errors.New("fmt::Error")
	}
	return len(v), nil
}
func (p *symbolPanicProbe) operation(c rustfmt.Context) (result string) {
	if p.request.op == "context" {
		return p.ownedContext(c)
	}
	p.guard(c, "operation", func() {
		r := p.request
		s := p.session
		switch r.op {
		case "new-symbol":
			result = strconv.FormatUint(uint64(NewSymbolWithContext(uint32(r.value), c).AsU32()), 10)
		case "new-byte":
			result = strconv.FormatUint(uint64(NewByteSymbolWithContext(uint32(r.value), c).AsU32()), 10)
		case "get-symbol", "reenter-index-hook":
			result = fmt.Sprintf("%x", NewSymbolWithContext(uint32(r.value), c).AsStrWithContext(s, c))
		case "no-session":
			result = fmt.Sprintf("%x", NewSymbol(1).AsStrWithContext(nil, c))
		case "get-byte":
			result = fmt.Sprintf("%x", NewByteSymbolWithContext(uint32(r.value), c).AsByteStrWithContext(s, c))
		case "clamp-symbol":
			result = fmt.Sprint(NewSymbol(0).ClampWithContext(NewSymbol(2), NewSymbol(1), c).AsU32())
		case "clamp-byte":
			result = fmt.Sprint(NewByteSymbol(0).ClampWithContext(NewByteSymbol(2), NewByteSymbol(1), c).AsU32())
		case "prefill":
			extra := []string{"symbol_duplicate_a", "symbol_duplicate_a"}
			if r.value != 0 {
				extra = []string{"async"}
			}
			session := NewSessionGlobalsWithContext(Edition2024, extra, None[SourceMapInputs](), c, SpanEncodingConfig{DebugAssertions: p.profile.Debug, OverflowChecks: p.profile.Checked})
			session.CloseWithContext(c)
			result = "constructed"
		case "lifetime", "debug-get":
			sink := &symbolPanicSink{p: p, c: c, fail: r.aux}
			f := &rustfmt.Formatter{Writer: sink, Options: rustfmt.DefaultOptions(), Context: c}
			var err error
			if r.op == "lifetime" {
				word := "bare"
				if r.value != 0 {
					word = "'static"
				}
				printer := NewIdentPrinter(InternSymbolWithContext(s, word, c), IdentRawLifetime, None[Span]())
				err = printer.WriteDisplayFormatter(s, f)
			} else {
				err = NewSymbolWithContext(uint32(r.value), c).WriteDebugFormatter(s, f)
			}
			result = "ok"
			if err != nil {
				result = "error"
			}
		case "format-error":
			p.event(c, "format-error")
			symbolToStringError(c)
		case "substring":
			v, ok := EditDistanceWithSubstringsWithConfig("a", "aa", uint(r.value), SpanEncodingConfig{DebugAssertions: p.profile.Debug, OverflowChecks: p.profile.Checked}, c).Get()
			result = "none"
			if ok {
				result = fmt.Sprint(v)
			}
		case "ident":
			binding := SwapEmitBugDiagnosticLocationCell(func(_ *SessionGlobals, _ Option[Span], args SourceFormatArguments, cell *rustfmt.SourceLocation, c rustfmt.Context) bool {
				p.diagnosticCell = cell
				message, literal := args.AsStr().Get()
				file := ""
				var line, column uint32
				if cell != nil {
					file, line, column = cell.File, cell.Line, cell.Column
				}
				p.event(c, fmt.Sprintf("diagnostic:%x:%t:%x:%d:%d", message, literal, file, line, column))
				return r.fail != 0
			})
			defer binding.Restore()
			result = fmt.Sprint(NewIdentAtLocationCellWithContext(s, Sym_empty(), DummySpan(), p.ctxLocation("symbol.client.ident"), c).Name.AsU32())
		default:
			panic("unknown Symbol panic operation")
		}
	})
	return
}
func symbolPanicReplay(line string, profile symbolPanicProfile, emit bool) (result string, trace []string) {
	p := &symbolPanicProbe{request: parseSymbolPanicRequest(line), profile: profile, phase: "call", emit: emit}
	p.ops.Locations = map[string]rustfmt.SourceLocation{}
	p.ops.LocationCells = map[string]*rustfmt.SourceLocation{}
	for key, v := range profile.Locations {
		cell := &rustfmt.SourceLocation{File: v.File, Line: v.Line, Column: v.Column}
		p.ops.Locations[key] = *cell
		p.ops.LocationCells[key] = cell
	}
	if m, ok := profile.Locations["symbol.runtime.cleanup"]; ok {
		loc := p.ops.Locations["symbol.runtime.cleanup"]
		info := rustfmt.SourcePanicInfo{Payload: "panic in a destructor during cleanup", Kind: rustfmt.PanicStaticStr, Location: &loc, CanUnwind: m.CanUnwind, ForceNoBacktrace: m.Force}
		p.ops.CleanupAbort = &info
	}
	p.ops.Hook = p.hook
	c := p.ctx(rustfmt.Context{}, "call")
	if p.request.op != "prefill" && p.request.op != "context" && p.request.op != "no-session" {
		p.session = NewSessionGlobalsWithContext(DefaultEdition, nil, None[SourceMapInputs](), c, SpanEncodingConfig{DebugAssertions: profile.Debug, OverflowChecks: profile.Checked})
		defer p.session.Close()
	}
	switch p.request.mode {
	case 0:
		result = "panic"
		symbolPanicCapture(func() { result = p.operation(c) })
	case 1, 2:
		symbolPanicCapture(func() {
			outer := rustvec.NewOwned(p, rustvec.ValueOps[*symbolPanicProbe]{Drop: func(_ **symbolPanicProbe, unwind rustfmt.Context) {
				p.event(unwind, "outer-drop")
				if p.request.mode == 1 {
					p.result = "panic"
					symbolPanicCapture(func() { p.result = p.operation(unwind) })
					p.event(unwind, "inner-caught")
				} else {
					p.result = p.operation(unwind)
				}
			}})
			rustvec.ConsumeValue(outer, c, func(*rustvec.Owned[*symbolPanicProbe]) struct{} {
				c.PanicAt(p.info("symbol.client.outer12", "outer"))
				return struct{}{}
			})
		})
		result = p.result
	case 3:
		symbolPanicCapture(func() { c.PanicAt(p.info("symbol.client.outer3", "outer")) })
		result = p.result
	}
	after := p.ctx(rustfmt.Context{}, "after")
	if p.request.op == "get-symbol" || p.request.op == "get-byte" || p.request.op == "debug-get" {
		good := InternSymbolWithContext(p.session, "symbol_lock_released", after)
		p.event(after, fmt.Sprintf("after-symbol:%d:%x", good.AsU32(), good.AsStrWithContext(p.session, after)))
	}
	return result, p.trace
}

func TestSymbolOrdinaryPanicContracts(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("explicit native64 Symbol panic profile")
	}
	m, raw, err := symbolPanicLoadEvidence()
	if err != nil {
		t.Fatal(err)
	}
	profiles := map[string]symbolPanicProfile{}
	for _, p := range m.Profiles {
		profiles[p.Name] = p
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 65536), 1<<20)
	rows, records := 0, 0
	for scanner.Scan() {
		var r symbolPanicRecord
		if err = json.Unmarshal(scanner.Bytes(), &r); err != nil {
			t.Fatal(err)
		}
		rows++
		for _, name := range r.Profiles {
			p, ok := profiles[name]
			if !ok {
				t.Fatal("unknown panic profile")
			}
			records++
			if r.Want.Abort {
				symbolPanicAssertAbort(t, r, p)
				continue
			}
			result, trace := symbolPanicReplay(r.Request, p, false)
			if result != r.Want.Result || strings.Join(trace, "|") != strings.Join(r.Want.Trace, "|") {
				t.Fatalf("%s %s\ngot %q %s\nwant%q %s", name, r.Request, result, strings.Join(trace, "|"), r.Want.Result, strings.Join(r.Want.Trace, "|"))
			}
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if rows != m.Rows || records != m.Records {
		t.Fatalf("Symbol panic coverage rows%d/%d observations%d/%d", rows, m.Rows, records, m.Records)
	}
}
func symbolPanicAssertAbort(t *testing.T, r symbolPanicRecord, p symbolPanicProfile) {
	t.Helper()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	profile, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(deadline, binary, "-test.run=^TestSymbolPanicAbortHelper$")
	value, selected := os.LookupEnv(symbolPanicProfileDirectory)
	_, _, directory, err := symbolPanicEvidencePaths(value, selected)
	if err != nil {
		t.Fatal(err)
	}
	cmd.Env = append(os.Environ(), "FERRUGO_SYMBOL_PANIC_CHILD="+r.Request, "FERRUGO_SYMBOL_PANIC_PROFILE="+string(profile))
	if selected {
		cmd.Env = append(cmd.Env, symbolPanicProfileDirectory+"="+directory)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 134 {
		t.Fatalf("%s %s expected134: %v\n%s\n%s", p.Name, r.Request, err, &stdout, &stderr)
	}
	if got := strings.TrimSuffix(stdout.String(), "\n"); got != strings.Join(r.Want.Trace, "\n") {
		t.Fatalf("%s %s abort trace\ngot %s\nwant%s", p.Name, r.Request, got, strings.Join(r.Want.Trace, "\n"))
	}
}
func TestSymbolPanicAbortHelper(t *testing.T) {
	line, ok := os.LookupEnv("FERRUGO_SYMBOL_PANIC_CHILD")
	if !ok {
		t.Skip("isolated actual-source Symbol abort")
	}
	var p symbolPanicProfile
	if err := json.Unmarshal([]byte(os.Getenv("FERRUGO_SYMBOL_PANIC_PROFILE")), &p); err != nil {
		t.Fatal(err)
	}
	m, _, err := symbolPanicLoadEvidence()
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, selected := range m.Profiles {
		if selected.Name == p.Name && reflect.DeepEqual(selected, p) {
			matched = true
		}
	}
	if !matched {
		t.Fatal("abort child profile differs from selected producer")
	}
	SetDynThreadSafeMode(p.ThreadSafe)
	symbolPanicReplay(line, p, true)
	t.Fatal("expected abort returned")
}
func TestSymbolPanicLiveRegeneration(t *testing.T) {
	if value, ok := os.LookupEnv("FERRUGO_SYMBOL_PANIC_LIVE"); !ok {
		t.Skip("set FERRUGO_SYMBOL_PANIC_LIVE=1 for exact original whole-crate proof")
	} else if value != "1" {
		t.Fatal("FERRUGO_SYMBOL_PANIC_LIVE must be1")
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	deadline, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	arguments := []string{filepath.Join(root, "tools/port-rust-symbol/panic/generate.py"), "--prepared", "--offline", "--check"}
	value, selected := os.LookupEnv(symbolPanicProfileDirectory)
	_, _, directory, err := symbolPanicEvidencePaths(value, selected)
	if err != nil {
		t.Fatal(err)
	}
	if selected {
		arguments = append(arguments, "--output-dir", directory)
	}
	cmd := exec.CommandContext(deadline, python, arguments...)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Symbol panic native check:%v\n%s", err, out)
	}
}

func symbolPanicCapture(call func()) (payload any) {
	defer func() { payload = recover() }()
	call()
	return nil
}

// Selected-build source spellings are caller-owned metadata. The absence of
// that metadata never fabricates the known reference filename or a Go caller.
func TestSymbolOrdinaryPanicLocationWitnesses(t *testing.T) {
	manifest, raw, err := symbolPanicLoadEvidence()
	if err != nil {
		t.Fatal(err)
	}
	var source symbolPanicProfile
	for _, p := range manifest.Profiles {
		if p.Name == "d1o1" {
			source = p
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 65536), 1<<20)
	wanted := map[string]bool{"new-symbol": false, "get-symbol": false, "clamp-symbol": false, "prefill": false, "lifetime": false, "substring": false, "no-session": false, "format-error": false}
	for scanner.Scan() {
		var record symbolPanicRecord
		if err = json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		req := parseSymbolPanicRequest(record.Request)
		done, needed := wanted[req.op]
		if !needed || done || req.mode != 0 || record.Want.Abort {
			continue
		}
		applies := false
		for _, name := range record.Profiles {
			if name == source.Name {
				applies = true
			}
		}
		hasHook := false
		for _, e := range record.Want.Trace {
			if strings.HasPrefix(e, "hook:") {
				hasHook = true
			}
		}
		if !applies || !hasHook {
			continue
		}
		wanted[req.op] = true
		for _, located := range []bool{false, true} {
			p := source
			p.Locations = map[string]symbolPanicLocation{}
			for key, v := range source.Locations {
				v.File = "selected/symbol-build/./original.rs"
				v.Line = 73
				v.Column = 19
				if located {
					p.Locations[key] = v
				}
			}
			result, trace := symbolPanicReplay(record.Request, p, false)
			expected := append([]string(nil), record.Want.Trace...)
			for i, e := range expected {
				if strings.HasPrefix(e, "hook:") {
					fields := strings.Split(e, ":")
					fields[5], fields[6], fields[7] = "", "0", "0"
					if located {
						fields[5], fields[6], fields[7] = fmt.Sprintf("%x", "selected/symbol-build/./original.rs"), "73", "19"
					}
					expected[i] = strings.Join(fields, ":")
				}
			}
			if result != record.Want.Result || strings.Join(trace, "|") != strings.Join(expected, "|") {
				t.Fatalf("%s explicitlocation%t\ngot %s\nwant%s", record.Request, located, strings.Join(trace, "|"), strings.Join(expected, "|"))
			}
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for op, seen := range wanted {
		if !seen {
			t.Fatalf("missing actual native panic witness: %s", op)
		}
	}
}

// The original whole-crate client observes the same immutable Location cell in
// the diagnostic and panic callbacks. Scalar wrappers remain conveniences;
// primary cells also survive a scalar callback swap/restore roundtrip.
func TestIdentBorrowedLocationCell(t *testing.T) {
	s := NewSessionGlobals(DefaultEdition, nil, None[SourceMapInputs](), SpanEncodingConfig{DebugAssertions: true})
	defer s.Close()
	for _, emitted := range []bool{false, true} {
		for _, fixed := range []bool{false, true} {
			for _, located := range []bool{false, true} {
				var cell *rustfmt.SourceLocation
				if located {
					cell = &rustfmt.SourceLocation{File: "selected/./immutable-caller.rs", Line: 73, Column: 19}
				}
				var diagnostic, panicked *rustfmt.SourceLocation
				binding := SwapEmitBugDiagnosticLocationCell(func(_ *SessionGlobals, _ Option[Span], _ SourceFormatArguments, actual *rustfmt.SourceLocation, _ rustfmt.Context) bool {
					diagnostic = actual
					return emitted
				})
				old := SwapEmitBugDiagnostic(func(_ *SessionGlobals, _ Option[Span], _ SourceFormatArguments, _ SourceLocation, _ rustfmt.Context) bool {
					t.Fatal("temporary callback retained after restoration")
					return false
				})
				SwapEmitBugDiagnostic(old)
				c := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{LocationCells: map[string]*rustfmt.SourceLocation{PanicIdentDummySpanSite: cell}, Hook: func(info rustfmt.SourcePanicInfo, _ rustfmt.Context) { panicked = info.Location }})
				payload := symbolPanicCapture(func() {
					if fixed {
						NewIdentWithDummySpanWithContext(s, Sym_empty(), c)
					} else {
						NewIdentAtLocationCellWithContext(s, Sym_empty(), DummySpan(), cell, c)
					}
				})
				binding.Restore()
				if payload == nil || diagnostic != cell || panicked != cell {
					t.Fatalf("emit%t/fixed%t/located%t lost original cell: diagnostic%p panic%p want%p", emitted, fixed, located, diagnostic, panicked, cell)
				}
			}
		}
	}
}
