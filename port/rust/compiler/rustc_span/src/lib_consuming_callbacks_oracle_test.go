// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	rv "github.com/lassejlv/rust-port-go/port/support/rustvec"
)

const consumingProfileEnv = "FERRUGO_SPAN_CONSUMING_PROOF"
const consumingDigestEnv = "FERRUGO_SPAN_CONSUMING_PROOF_SHA256"

type consumingRequest struct {
	Route, Action, Mode string
	Debug, Overflow     bool
}
type consumingManifest struct {
	Kind, Revision  string
	Records, Aborts int
	Raw, Gzip       string
	Locations       map[string]string
	Files           map[string]string
}
type consumingF struct {
	core  *consumingCore
	token sp.Shared[[]byte]
	data  []byte
	state uint32
}
type consumingCore struct {
	q                consumingRequest
	token            sp.Shared[[]byte]
	file             *sp.SourceFile
	session          *sp.SessionGlobals
	span             sp.Span
	argument, stored *rv.Owned[consumingF]
}

func consumingRuntime(c rf.Context) int {
	if c.Runtime == nil {
		return 0
	}
	v, _ := c.Runtime.Value.(int)
	return v
}
func (s *consumingCore) event(c rf.Context, text string) {
	phase := "_"
	if s.file != nil {
		phase = strconv.FormatBool(s.file.ExternalSrc.IsFrozen())
	}
	fmt.Printf("%s:%t:%d:%d:%s\n", text, c.Panicking(), consumingRuntime(c), s.token.StrongCount(), phase)
}
func consumingClose(c rf.Context, close func(rf.Context)) {
	p := recover()
	if p == nil {
		close(c)
		return
	}
	if c.ProcessingPanic {
		c.ResumeUnwind(p)
	}
	u := c
	u.Unwinding = true
	func() {
		defer func() {
			if recover() != nil {
				c.AbortCleanup()
			}
		}()
		close(u)
	}()
	c.ResumeUnwind(p)
}
func consumingRaise(c rf.Context, text, site string) {
	c.PanicAt(rf.SourcePanicInfo{Payload: text, Kind: rf.PanicStaticStr, CanUnwind: true, Location: c.SourcePanicLocation(site)})
}
func (s *consumingCore) drop(f *consumingF, c rf.Context) {
	defer consumingClose(c, func(c rf.Context) {
		f.token.CloseWithContext(c)
		f.data = nil
		s.event(c, "field")
		if s.q.Action == "field" || s.q.Action == "drop_field" {
			consumingRaise(c, "field", "field")
		}
	})
	defer consumingClose(c, func(c rf.Context) { s.event(c, "drop_local") })
	if len(f.data) != 3 || len(*f.token.BorrowWithContext(c)) != 1 {
		panic("invalid actual consuming callback fields")
	}
	s.event(c, fmt.Sprintf("self_drop:%d", f.state))
	if s.q.Action == "drop" || s.q.Action == "drop_field" {
		consumingRaise(c, "self_drop", "self_drop")
	}
}
func (s *consumingCore) call(argument *rv.Owned[consumingF], c rf.Context) {
	defer consumingClose(c, func(c rf.Context) { s.event(c, "call_local") })
	argument.Borrow().state++
	s.event(c, fmt.Sprintf("call:%d", argument.Borrow().state))
	site := "call_map"
	if strings.HasPrefix(s.q.Route, "external") {
		site = "call_external"
	}
	site += "_" + s.q.Action
	switch s.q.Action {
	case "take_drop":
		value := argument.Take()
		s.drop(&value, c)
		s.event(c, "after_explicit_drop")
	case "forget", "forget_panic":
		argument.Take()
		s.event(c, "forgot")
		if s.q.Action == "forget_panic" {
			consumingRaise(c, "call", site)
		}
	case "move", "move_panic":
		s.stored = argument.MoveWithContext(c)
		s.event(c, "moved")
		if s.q.Action == "move_panic" {
			consumingRaise(c, "call", site)
		}
	case "panic":
		consumingRaise(c, "call", site)
	}
}
func (s *consumingCore) report(c rf.Context, label string, call func()) {
	defer func() {
		if p := recover(); p != nil {
			kind := "static_str"
			if strings.HasPrefix(fmt.Sprint(p), "unexpected state ") {
				kind = "owned_string"
			}
			s.event(c, fmt.Sprintf("caught:%s:true:%s:%x", label, kind, fmt.Sprint(p)))
		}
	}()
	call()
	s.event(c, "caught:"+label+":false::")
}
func (s *consumingCore) invoke(c rf.Context) {
	f := s.argument
	s.argument = nil
	if strings.HasPrefix(s.q.Route, "external") {
		result := sp.AddExternalSrcConsuming(s.file, f, func(f *rv.Owned[consumingF], c rf.Context) sp.Option[string] { s.call(f, c); return sp.Some("abc") }, c, sp.SpanEncodingConfig{DebugAssertions: s.q.Debug, OverflowChecks: s.q.Overflow})
		s.event(c, "returned:external:"+strconv.FormatBool(result))
	} else {
		result := sp.MapCtxtConsuming(s.span, s.session, f, func(f *rv.Owned[consumingF], old sp.SyntaxContext, c rf.Context) sp.SyntaxContext {
			s.call(f, c)
			return old
		}, c, sp.SpanEncodingConfig{DebugAssertions: s.q.Debug, OverflowChecks: s.q.Overflow})
		data := result.DataUntrackedWithContext(s.session, c)
		s.event(c, fmt.Sprintf("returned:map:%d,%d,%d", data.Lo, data.Hi, data.Ctxt.AsU32()))
	}
}
func (s *consumingCore) setup(c rf.Context) {
	cfg := sp.SpanEncodingConfig{DebugAssertions: s.q.Debug, OverflowChecks: s.q.Overflow}
	s.session = sp.NewSessionGlobalsWithContext(sp.Edition2024, nil, sp.None[sp.SourceMapInputs](), c, cfg)
	if strings.HasPrefix(s.q.Route, "external") {
		file, err := sp.NewSourceFileWithContext(sp.CustomFileName("consuming"), "abc", sp.SourceFileHashMd5, sp.None[sp.SourceFileHashAlgorithm](), c, cfg)
		if err != nil {
			panic(err)
		}
		if src, ok := file.Src.Get(); ok {
			src.CloseWithContext(c)
		}
		file.Src = sp.None[sp.SharedString]()
		file.ExternalSrc.CloseWithContext(c)
		value := sp.ForeignExternalSource(sp.AbsentOkExternalSourceKind(), 0)
		if s.q.Route == "external_frozen" {
			value = sp.ForeignExternalSource(sp.AbsentErrExternalSourceKind(), 0)
		} else if s.q.Route == "external_bad_state" {
			value = sp.UnneededExternalSource()
		}
		file.ExternalSrc = sp.NewFreezeLockWithDrop(value, s.q.Route == "external_frozen", func(v *sp.ExternalSource, c rf.Context) { v.CloseWithContext(c) })
		s.file = &file
	} else {
		lo, hi, ctxt, parent := sp.BytePos(1), sp.BytePos(70000), uint32(0), sp.Some(sp.LocalDefId{LocalDefIndex: sp.DefIndexFromU32(22)})
		switch s.q.Route {
		case "map_inline":
			hi = 2
			parent = sp.None[sp.LocalDefId]()
		case "map_parent":
			hi = 2
		case "map_pair":
			hi = 2
			ctxt = 2
		}
		s.span = sp.NewSpan(s.session, lo, hi, sp.SyntaxContextFromU32(ctxt), parent, cfg)
		if s.q.Route == "map_absent" {
			s.session.CloseWithContext(c)
			s.session = nil
		}
	}
}
func consumingLocation(text string) rf.SourceLocation {
	p := strings.LastIndexByte(text, ':')
	col, e := strconv.ParseUint(text[p+1:], 10, 32)
	if e != nil {
		panic(e)
	}
	text = text[:p]
	p = strings.LastIndexByte(text, ':')
	line, e := strconv.ParseUint(text[p+1:], 10, 32)
	if e != nil {
		panic(e)
	}
	return rf.SourceLocation{File: text[:p], Line: uint32(line), Column: uint32(col)}
}
func consumingExecute(q consumingRequest, m consumingManifest) {
	s := &consumingCore{q: q, token: sp.NewShared([]byte{7}, nil)}
	defer s.token.Close()
	old := sp.SwapSpanTrackHook(sp.NewSpanTrackHook(func(id sp.LocalDefId, c rf.Context) {
		defer consumingClose(c, func(c rf.Context) { s.event(c, "track_local") })
		s.event(c, fmt.Sprintf("track:%d", id.LocalDefIndex.AsU32()))
	}))
	defer sp.SwapSpanTrackHook(old)
	a := rf.WithPanicOperations(rf.Context{Runtime: &rf.RuntimeMetadata{Value: 1}}, &rf.PanicOperations{Hook: func(rf.SourcePanicInfo, rf.Context) { fmt.Println("stale_hook") }})
	s.setup(a)
	s.event(a, "construct")
	s.argument = rv.NewOwned(consumingF{s, s.token.CloneWithContext(a), []byte{1, 2, 3}, 0}, rv.ValueOps[consumingF]{Drop: func(f *consumingF, c rf.Context) { s.drop(f, c) }}).MoveWithContext(a)
	s.event(a, "prepared")
	cleanup := rf.CleanupPanicInfoNightly20261104()
	ops := &rf.PanicOperations{CleanupAbort: &cleanup, Locations: map[string]rf.SourceLocation{}}
	for site, text := range m.Locations {
		ops.Locations[site] = consumingLocation(text)
	}
	processing := q.Mode == "processing"
	ops.Hook = func(info rf.SourcePanicInfo, c rf.Context) {
		loc := ""
		if info.Location != nil {
			loc = fmt.Sprintf("%s:%d:%d", info.Location.File, info.Location.Line, info.Location.Column)
		}
		s.event(c, fmt.Sprintf("hook:%s:%x:%t:%t:%s", info.Kind, fmt.Sprint(info.Payload), info.CanUnwind, info.ForceNoBacktrace, loc))
		if processing {
			s.report(c, "target", func() { s.invoke(c) })
			s.event(c, "hook_return")
		}
	}
	id := 2
	if q.Mode == "normal" {
		id = 1
	}
	c := rf.WithPanicOperations(rf.Context{Runtime: &rf.RuntimeMetadata{Value: id}}, ops)
	if q.Mode == "nil" {
		c = rf.Context{}
	}
	s.event(c, "invoke")
	switch q.Mode {
	case "normal", "currentB", "nil":
		s.report(c, "target", func() { s.invoke(c) })
	case "processing":
		s.report(c, "ambient", func() { consumingRaise(c, "outer", "outer") })
	default:
		s.report(c, "ambient", func() {
			defer consumingClose(c, func(u rf.Context) {
				s.event(u, "ambient_drop")
				if q.Mode == "inherited_caught" {
					s.report(u, "target", func() { s.invoke(u) })
				} else {
					s.invoke(u)
				}
			})
			consumingRaise(c, "outer", "outer")
		})
	}
	processing = false
	s.event(c, "after_target")
	late := rf.WithPanicOperations(rf.Context{Runtime: &rf.RuntimeMetadata{Value: 3}}, ops)
	if s.stored != nil {
		stored := s.stored
		s.stored = nil
		s.event(late, "stored_before_drop")
		s.report(late, "stored", func() { stored.DropWithContext(late) })
		s.event(late, "stored_after_drop")
	}
	s.event(late, "complete")
	if s.file != nil {
		s.file.CloseWithContext(late)
		s.file = nil
	}
	s.event(late, "file_dropped")
	if s.session != nil {
		s.session.CloseWithContext(late)
	}
	s.event(late, "finish")
}

func consumingRoot() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
}

type consumingSelection struct {
	directory, manifest, fixture, digest string
	explicit                             bool
}

func consumingSelect(root string, lookup func(string) (string, bool)) (consumingSelection, error) {
	dir := filepath.Join(root, "tools/port-rust-span-lib")
	explicit := false
	if value, present := lookup(consumingProfileEnv); present {
		if value == "" {
			return consumingSelection{}, fmt.Errorf("%s explicitly empty", consumingProfileEnv)
		}
		var err error
		dir, err = filepath.Abs(value)
		if err != nil {
			return consumingSelection{}, err
		}
		explicit = true
	}
	i, e := os.Lstat(dir)
	if e != nil || !i.IsDir() || i.Mode()&os.ModeSymlink != 0 {
		return consumingSelection{}, fmt.Errorf("invalid selected consuming producer directory %s", dir)
	}
	result := consumingSelection{directory: dir, manifest: filepath.Join(dir, "consuming-callbacks-manifest.json"), fixture: filepath.Join(dir, "consuming-callbacks.jsonl.gz"), explicit: explicit}
	for _, file := range []string{result.manifest, result.fixture} {
		i, e := os.Lstat(file)
		limit := int64(64 << 20)
		if file == result.manifest {
			limit = 8 << 20
		}
		if e != nil || !i.Mode().IsRegular() || i.Size() > limit {
			return result, fmt.Errorf("invalid bounded selected consuming producer file %s", file)
		}
	}
	return result, nil
}
func consumingMetadata(selection consumingSelection, expected string) (consumingManifest, string, error) {
	var m consumingManifest
	data, e := os.ReadFile(selection.manifest)
	if e != nil {
		return m, "", e
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	if expected != "" && digest != expected {
		return m, "", fmt.Errorf("consuming child producer manifest digest differs")
	}
	if e = json.Unmarshal(data, &m); e != nil {
		return m, "", e
	}
	if m.Kind != "span-consuming-fnonce-v1" || m.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || m.Records <= 0 || len(m.Files) == 0 || m.Raw == "" || m.Gzip == "" {
		return m, "", fmt.Errorf("invalid consuming producer schema/profile")
	}
	return m, digest, nil
}
func consumingChildEnv(env []string, s consumingSelection) []string {
	out := []string{}
	for _, v := range env {
		if !strings.HasPrefix(v, consumingProfileEnv+"=") && !strings.HasPrefix(v, consumingDigestEnv+"=") {
			out = append(out, v)
		}
	}
	if s.explicit {
		out = append(out, consumingProfileEnv+"="+s.directory)
	}
	out = append(out, consumingDigestEnv+"="+s.digest)
	return out
}
func TestOriginalConsumingCallbacksChild(t *testing.T) {
	if os.Getenv("FERRUGO_CONSUMING_CHILD") != "1" {
		return
	}
	s, e := consumingSelect(consumingRoot(), os.LookupEnv)
	if e != nil {
		t.Fatal(e)
	}
	want, present := os.LookupEnv(consumingDigestEnv)
	if !present || want == "" {
		t.Fatal("consuming child has no captured producer digest")
	}
	m, _, e := consumingMetadata(s, want)
	if e != nil {
		t.Fatal(e)
	}
	var q consumingRequest
	if e = json.Unmarshal([]byte(os.Getenv("FERRUGO_CONSUMING_REQUEST")), &q); e != nil {
		t.Fatal(e)
	}
	consumingExecute(q, m)
	os.Exit(0)
}
func TestOriginalConsumingCallbacks(t *testing.T) {
	s, e := consumingSelect(consumingRoot(), os.LookupEnv)
	if e != nil {
		t.Fatal(e)
	}
	m, digest, e := consumingMetadata(s, "")
	if e != nil {
		t.Fatal(e)
	}
	s.digest = digest
	for name, want := range m.Files {
		clean := filepath.Clean(filepath.FromSlash(name))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			t.Fatal("invalid consuming binding path", name)
		}
		data, e := os.ReadFile(filepath.Join(consumingRoot(), clean))
		if e != nil {
			t.Fatal(e)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != want {
			t.Fatal("consuming production/source binding differs", name)
		}
	}
	blob, e := os.ReadFile(s.fixture)
	if e != nil {
		t.Fatal(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(blob)) != m.Gzip {
		t.Fatal("consuming gzip differs")
	}
	z, e := gzip.NewReader(bytes.NewReader(blob))
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.New()
	n, e := io.Copy(h, io.LimitReader(z, (256<<20)+1))
	z.Close()
	if e != nil || n > 256<<20 || fmt.Sprintf("%x", h.Sum(nil)) != m.Raw {
		t.Fatal("consuming raw checksum/bound differs", e)
	}
	z, e = gzip.NewReader(bytes.NewReader(blob))
	if e != nil {
		t.Fatal(e)
	}
	defer z.Close()
	scan := bufio.NewScanner(z)
	scan.Buffer(make([]byte, 65536), 2<<20)
	binary, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	count, aborts := 0, 0
	for scan.Scan() {
		var row struct {
			Request consumingRequest
			Want    struct {
				Abort  bool
				Status int
				Output string
			}
		}
		if e = json.Unmarshal(scan.Bytes(), &row); e != nil {
			t.Fatal(e)
		}
		if row.Want.Abort && row.Want.Status != -6 || !row.Want.Abort && row.Want.Status != 0 {
			t.Fatal("native consuming status differs")
		}
		q, _ := json.Marshal(row.Request)
		c, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		cmd := exec.CommandContext(c, binary, "-test.run=^TestOriginalConsumingCallbacksChild$")
		cmd.Env = append(consumingChildEnv(os.Environ(), s), "FERRUGO_CONSUMING_CHILD=1", "FERRUGO_CONSUMING_REQUEST="+string(q))
		out, err := cmd.Output()
		cancel()
		abort := false
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("%+v host error %v\n%s", row.Request, err, out)
			}
			abort = true
		}
		if abort != row.Want.Abort || string(out) != row.Want.Output {
			t.Fatalf("%+v abort%t/%t\nGo:\n%s\nRust:\n%s", row.Request, abort, row.Want.Abort, out, row.Want.Output)
		}
		if abort {
			aborts++
		}
		count++
	}
	if e = scan.Err(); e != nil {
		t.Fatal(e)
	}
	if count != m.Records || aborts != m.Aborts {
		t.Fatal("consuming evidence count differs", count, aborts)
	}
}
func TestConsumingProducerSelector(t *testing.T) {
	dir := t.TempDir()
	absent := func(string) (string, bool) { return "", false }
	if _, e := consumingSelect(dir, absent); e == nil {
		t.Fatal("missing historical evidence accepted")
	}
	for _, value := range []string{"", filepath.Join(dir, "missing")} {
		if _, e := consumingSelect(consumingRoot(), func(string) (string, bool) { return value, true }); e == nil {
			t.Fatal("bad explicit producer fell back")
		}
	}
	for _, name := range []string{"consuming-callbacks-manifest.json", "consuming-callbacks.jsonl.gz"} {
		if e := os.WriteFile(filepath.Join(dir, name), []byte("data"), 0644); e != nil {
			t.Fatal(e)
		}
	}
	s, e := consumingSelect(consumingRoot(), func(string) (string, bool) { return dir, true })
	if e != nil || !s.explicit || s.directory != dir {
		t.Fatal("explicit consuming producer not selected", e)
	}
	s.digest = "parent-captured"
	env := consumingChildEnv([]string{consumingProfileEnv + "=/wrong", consumingDigestEnv + "=wrong", "KEEP=1"}, s)
	if strings.Join(env, "\n") != "KEEP=1\n"+consumingProfileEnv+"="+dir+"\n"+consumingDigestEnv+"=parent-captured" {
		t.Fatal("child selection changed", env)
	}
	if e = os.Remove(s.fixture); e != nil {
		t.Fatal(e)
	}
	if _, e = consumingSelect(consumingRoot(), func(string) (string, bool) { return dir, true }); e == nil {
		t.Fatal("partial producer fell back")
	}
}
