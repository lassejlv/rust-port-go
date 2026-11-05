// SPDX-License-Identifier: MIT OR Apache-2.0
package util

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
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustanyhow"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
)

type timeSpanRequest struct {
	Op, Span, Inner, Mode string
	Pattern, Fail         int
	Chars, Captured       bool
}
type timeSpanRow struct {
	Profile string
	Request timeSpanRequest
	Want    string
	Aborted bool
	Events  []string
}
type timeSpanManifest struct {
	Schema       int
	ArtifactKind string `json:"artifact_kind"`
	Records      int
	Aborts       int    `json:"actual_aborts"`
	RawSHA       string `json:"raw_sha256"`
	FixtureSHA   string `json:"fixture_sha256"`
	Files        map[string]string
	Profiles     []struct {
		Name       string
		Debug      bool `json:"debug_assertions"`
		Overflow   bool `json:"overflow_checks"`
		Tests      int  `json:"original_tests"`
		Assertions int  `json:"original_assertions"`
	}
	Reference struct {
		Pin    string `json:"cargo_pin"`
		Source string `json:"source_sha256"`
		Anyhow string `json:"anyhow_archive_sha256"`
	}
}

func timeSpanHex(s string) string { return hex.EncodeToString([]byte(s)) }
func timeSpanUnhex(s string) string {
	data, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return string(data)
}
func timeSpanProfile(row timeSpanRow) TimeSpanConfig {
	switch row.Profile {
	case "d0o0", "d0o1", "d1o0", "d1o1":
	default:
		panic("invalid time-span profile")
	}
	return TimeSpanConfig{DebugAssertions: row.Profile[1] == '1', OverflowChecks: row.Profile[3] == '1'}
}
func timeSpanRun(row timeSpanRow) (result string) {
	defer func() {
		if p := recover(); p != nil {
			result = row.Request.Op + "\tpanic\t" + timeSpanHex(fmt.Sprint(p))
		}
	}()
	q := row.Request
	config := timeSpanProfile(row)
	if q.Op == "M" {
		d, ok := MaybeParseTimeSpanWithContext(q.Span, config, rustfmt.Context{})
		if !ok {
			return "M\tnone"
		}
		return fmt.Sprintf("M\tsome\t%d\t%d", d.Seconds(), d.Nanoseconds())
	}
	if q.Op == "F" {
		return timeSpanFormat(row)
	}
	if q.Op != "P" {
		panic("invalid parser request")
	}
	d, err := ParseTimeSpanWithContext(q.Span, config, rustfmt.Context{})
	if err == nil {
		return fmt.Sprintf("P\tsome\t%d\t%d", d.Seconds(), d.Nanoseconds())
	}
	e := err.(*rustanyhow.StringError)
	display, debug := e.Error(), e.RustDebug()
	var alternate strings.Builder
	f := rustfmt.Formatter{Writer: &alternate, Options: rustfmt.DefaultOptions()}
	f.Options.Alternate = true
	if err := e.WriteDebugFormatter(&f); err != nil {
		panic(err)
	}
	stringType, staticType, sourceNone := e.IsString(), e.IsStaticString(), e.Source() == nil
	chainCount := 1
	for cause := e.Source(); cause != nil; {
		chainCount++
		if next, ok := cause.(interface{ Source() error }); ok {
			cause = next.Source()
		} else {
			cause = nil
		}
	}
	cell, trace := e.DowncastOwnedStringRef(), e.Backtrace()
	moved := e.Move()
	stable, traceStable := cell == moved.DowncastOwnedStringRef(), trace == moved.Backtrace()
	snapshots := []string{fmt.Sprintf("initial:%d:%d", cell.Len(), cell.Capacity())}
	before := unsafe.StringData(cell.AsStr())
	cell.Reserve(0)
	snapshots = append(snapshots, fmt.Sprintf("reserve0:%d:%d:%t", cell.Len(), cell.Capacity(), before == unsafe.StringData(cell.AsStr())))
	observe := func(name string, needed uint, body func()) {
		before := unsafe.StringData(cell.AsStr())
		stable := cell.Capacity()-cell.Len() >= needed
		body()
		identity := "-"
		if stable {
			identity = strconv.FormatBool(before == unsafe.StringData(cell.AsStr()))
		}
		snapshots = append(snapshots, fmt.Sprintf("%s:%d:%d:%s", name, cell.Len(), cell.Capacity(), identity))
	}
	observe("push1", 1, func() { cell.PushCharWithContext('!', rustfmt.Context{}) })
	observe("reserve13", 13, func() { cell.Reserve(13) })
	observe("push4", 4, func() { cell.PushCharWithContext('🦀', rustfmt.Context{}) })
	observe("append", 4, func() { cell.PushString("tail") })
	buffer := unsafe.StringData(cell.AsStr())
	owned := moved.DowncastOwnedString()
	payload := owned.Borrow()
	text := payload.AsStr()
	snapshots = append(snapshots, fmt.Sprintf("downcast:%d:%d", payload.Len(), payload.Capacity()))
	output := fmt.Sprintf("P\terror\t%s\t%s\t%s\t%t\t%t\t%t\t%d\t%t\t%t\t%t\t%s\t%s", timeSpanHex(display), timeSpanHex(debug), timeSpanHex(alternate.String()), stringType, staticType, sourceNone, chainCount, stable, traceStable, buffer == unsafe.StringData(text), timeSpanHex(text), strings.Join(snapshots, "|"))
	owned.Close()
	return output
}

type timeSpanSink struct {
	calls []string
	out   strings.Builder
	fail  int
	chars bool
}

func (s *timeSpanSink) attempt(kind, text string) error {
	s.calls = append(s.calls, kind+":"+timeSpanHex(text))
	if len(s.calls) == s.fail {
		return fmt.Errorf("fmt::Error")
	}
	s.out.WriteString(text)
	return nil
}
func (s *timeSpanSink) Write(p []byte) (int, error) {
	if err := s.attempt("S", string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}
func (s *timeSpanSink) WriteString(p string) (int, error) {
	if err := s.attempt("S", p); err != nil {
		return 0, err
	}
	return len(p), nil
}
func (s *timeSpanSink) WriteChar(c rune) error {
	kind := "S"
	if s.chars {
		kind = "C"
	}
	return s.attempt(kind, string(c))
}
func timeSpanFormat(row timeSpanRow) string {
	q := row.Request
	config := timeSpanProfile(row)
	if q.Captured {
		parts := strings.Split(row.Want, "\t")
		trace := rustanyhow.NewBacktraceRecord(rustanyhow.BacktraceCaptured, timeSpanUnhex(parts[4]))
		config.Backtrace.Capture = func(rustfmt.Context) rustanyhow.Backtrace { return trace }
	}
	_, err := ParseTimeSpanWithContext(q.Span, config, rustfmt.Context{})
	if err == nil {
		panic("formatter input unexpectedly valid")
	}
	e := err.(*rustanyhow.StringError)
	defer e.Close()
	sink := &timeSpanSink{fail: q.Fail, chars: q.Chars}
	f := rustfmt.Formatter{Writer: sink, Options: rustfmt.DefaultOptions()}
	switch q.Pattern {
	case 0, 4:
	case 1, 5:
		f.Options.Alternate = true
	case 2:
		f.Options.HasWidth = true
		f.Options.Width = 30
		f.Options.Align = rustfmt.AlignRight
	case 3:
		f.Options.HasPrecision = true
		f.Options.Precision = 2
	case 6:
		f.Options.ZeroPad = true
		f.Options.HasWidth = true
		f.Options.Width = 8
	case 7:
		f.Options.Alternate = true
		f.Options.DebugLowerHex = true
	case 8, 9:
		f.Options.HasWidth = true
		f.Options.Width = 30
		f.Options.HasPrecision = true
		f.Options.Precision = 2
		f.Options.Align = rustfmt.AlignRight
		f.Options.Fill = 0
		if q.Pattern == 9 {
			f.Options.Fill = '🦀'
			f.Options.Align = rustfmt.AlignCenter
		}
	case 10:
		f.Options.SignPlus = true
		f.Options.Alternate = true
		f.Options.ZeroPad = true
		f.Options.HasWidth = true
		f.Options.Width = 8
		f.Options.DebugUpperHex = true
	case 11:
		f.Options.Fill = '\n'
		f.Options.Align = rustfmt.AlignLeft
		f.Options.HasWidth = true
		f.Options.Width = 60
	default:
		panic("invalid formatter pattern")
	}
	if q.Pattern <= 3 {
		err = e.WriteDisplayFormatter(&f)
	} else {
		err = e.WriteDebugFormatter(&f)
	}
	return fmt.Sprintf("F\t%t\t%s\t%s\t%s\t%t", err == nil, strings.Join(sink.calls, ","), timeSpanHex(sink.out.String()), timeSpanHex(e.Backtrace().DisplayText()), e.Backtrace().Status() == rustanyhow.BacktraceCaptured)
}
func timeSpanAmbient(row timeSpanRow) {
	q := row.Request
	config := timeSpanProfile(row)
	event := func(s string) { fmt.Println("EVENT:" + s) }
	locations := TimeSpanPanicLocationsAtPinnedRevision("cargo-original/src/util/time_span.rs")
	outer := rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, CanUnwind: true}
	for _, line := range row.Events {
		fields := strings.Split(line, ":")
		if len(fields) == 8 && fields[0] == "hook" && timeSpanUnhex(fields[2]) == "capacity overflow" {
			line, _ := strconv.ParseUint(fields[4], 10, 32)
			col, _ := strconv.ParseUint(fields[5], 10, 32)
			locations[ruststr.OwnedStringCapacityPanicSite] = rustfmt.SourceLocation{File: timeSpanUnhex(fields[3]), Line: uint32(line), Column: uint32(col)}
		}
		if len(fields) == 8 && fields[0] == "hook" && timeSpanUnhex(fields[2]) == "outer" {
			line, _ := strconv.ParseUint(fields[4], 10, 32)
			col, _ := strconv.ParseUint(fields[5], 10, 32)
			outer.Location = &rustfmt.SourceLocation{File: timeSpanUnhex(fields[3]), Line: uint32(line), Column: uint32(col)}
		}
	}
	var invoke func(rustfmt.Context)
	ops := &rustfmt.PanicOperations{Locations: locations}
	ops.Hook = func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
		file := ""
		var line, col uint32
		if info.Location != nil {
			file = info.Location.File
			line = info.Location.Line
			col = info.Location.Column
		}
		event(fmt.Sprintf("hook:%s:%s:%s:%d:%d:%t:%t", info.Kind, timeSpanHex(fmt.Sprint(info.Payload)), timeSpanHex(file), line, col, info.CanUnwind, info.ForceNoBacktrace))
		if q.Mode == "processing" && info.Payload == "outer" {
			invoke(c)
		}
	}
	context := rustfmt.WithPanicOperations(rustfmt.Context{}, ops)
	invoke = func(c rustfmt.Context) {
		defer func() {
			if p := recover(); p != nil {
				event("scope_drop:true")
				c.ResumeUnwind(p)
			}
			event("scope_drop:" + strconv.FormatBool(c.Panicking()))
		}()
		event("body:" + strconv.FormatBool(c.Panicking()))
		if q.Inner == "R" {
			_, err := ParseTimeSpanWithContext(q.Span, config, c)
			e := err.(*rustanyhow.StringError)
			defer func() {
				if p := recover(); p != nil {
					if c.ProcessingPanic {
						c.ResumeUnwind(p)
					}
					unwind := c
					unwind.Unwinding = true
					e.CloseWithContext(unwind)
					c.ResumeUnwind(p)
				}
				e.CloseWithContext(c)
			}()
			e.DowncastOwnedStringRefWithContext(c).ReserveWithContext(^uint(0), c)
			return
		}
		if q.Inner == "M" {
			d, ok := MaybeParseTimeSpanWithContext(q.Span, config, c)
			if ok {
				event(fmt.Sprintf("some:%d:%d", d.Seconds(), d.Nanoseconds()))
			} else {
				event("none")
			}
			return
		}
		d, err := ParseTimeSpanWithContext(q.Span, config, c)
		if err != nil {
			event("error:" + timeSpanHex(err.Error()))
			err.(*rustanyhow.StringError).CloseWithContext(c)
		} else {
			event(fmt.Sprintf("some:%d:%d", d.Seconds(), d.Nanoseconds()))
		}
	}
	catch := func(body func()) (caught bool) { defer func() { caught = recover() != nil }(); body(); return }
	if q.Mode == "normal" {
		event("caught:" + strconv.FormatBool(catch(func() { invoke(context) })))
		return
	}
	caught := catch(func() {
		if q.Mode == "unwinding" {
			defer func() {
				p := recover()
				c := context
				c.Unwinding = true
				event("inner_caught:" + strconv.FormatBool(catch(func() { invoke(c) })))
				context.ResumeUnwind(p)
			}()
		}
		context.PanicAt(outer)
	})
	event("outer_caught:" + strconv.FormatBool(caught))
}
func TestTimeSpanNativeAmbientChild(t *testing.T) {
	body, ok := os.LookupEnv("FERRUGO_TIME_SPAN_CHILD_ROW")
	if !ok {
		t.Skip("subprocess helper")
	}
	var row timeSpanRow
	if err := json.Unmarshal([]byte(body), &row); err != nil {
		t.Fatal(err)
	}
	timeSpanAmbient(row)
	os.Exit(0)
}
func TestTimeSpanOriginalOracle(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..")
	manifest := filepath.Join(root, "tools/port-cargo-time-span/manifest.json")
	fixture := "testdata/time-span.jsonl.gz"
	if value, ok := os.LookupEnv("FERRUGO_TIME_SPAN_PROOF"); ok {
		if value == "" {
			t.Fatal("explicitly empty time-span proof")
		}
		info, err := os.Lstat(value)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			t.Fatal("invalid explicit time-span producer")
		}
		manifest = filepath.Join(value, "manifest.json")
		fixture = filepath.Join(value, "time-span.jsonl.gz")
	}
	read := func(path string, limit int64) []byte {
		t.Helper()
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > limit {
			t.Fatal("invalid bounded time-span evidence", path)
		}
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	var profile timeSpanManifest
	encoded := read(manifest, 2<<20)
	if err := json.Unmarshal(encoded, &profile); err != nil {
		t.Fatal(err)
	}
	if profile.Schema != 1 || profile.ArtifactKind != "cargo-time-span-producer-v1" || profile.Reference.Pin != "24de4721844c7a4d839072bab6fb66d41c9bc1f4" || profile.Reference.Source != "185b6ee6c972b98a19ca7577482777c9e706b71869e6a6d3008d5a9a1c36f130" || profile.Reference.Anyhow != "2a4385e2e34eb35d6b3efe798b9eb88096925d87726c0798709bf56d9ed84af3" || profile.Records < 10000 || len(profile.Files) == 0 {
		t.Fatal("invalid time-span producer metadata")
	}
	if len(profile.Profiles) != 4 {
		t.Fatal("time-span profiles missing")
	}
	for i, name := range []string{"d0o0", "d0o1", "d1o0", "d1o1"} {
		p := profile.Profiles[i]
		if p.Name != name || p.Debug != (name[1] == '1') || p.Overflow != (name[3] == '1') || p.Tests != 2 || p.Assertions != 21 {
			t.Fatal("time-span original profile differs")
		}
	}
	for path, digest := range profile.Files {
		clean := filepath.Clean(filepath.FromSlash(path))
		if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
			t.Fatal("invalid proof binding path")
		}
		data, err := os.ReadFile(filepath.Join(root, clean))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != digest {
			t.Fatal("time-span input binding differs", path, err)
		}
	}
	compressed := read(fixture, 32<<20)
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != profile.FixtureSHA {
		t.Fatal("time-span gzip hash differs")
	}
	g, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(g, 32<<20+1))
	if err != nil || len(raw) > 32<<20 {
		t.Fatal("time-span raw bound", err)
	}
	if err = g.Close(); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != profile.RawSHA {
		t.Fatal("time-span raw hash differs")
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 64<<10), 1<<20)
	count, aborts := 0, 0
	for scan.Scan() {
		var row timeSpanRow
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		count++
		if row.Request.Op != "A" {
			if got := timeSpanRun(row); got != row.Want {
				t.Fatalf("case%d %s %q\nGo=%s\nRust=%s", count, row.Profile, row.Request.Span, got, row.Want)
			}
			continue
		}
		body, err := json.Marshal(row)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestTimeSpanNativeAmbientChild$")
		cmd.Env = append(os.Environ(), "FERRUGO_TIME_SPAN_CHILD_ROW="+string(body))
		var out, stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		err = cmd.Run()
		if row.Aborted {
			aborts++
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatal("expected abort", err, stderr.String())
			}
		} else if err != nil {
			t.Fatal(err, stderr.String())
		}
		events := []string{}
		for _, line := range strings.Split(out.String(), "\n") {
			if strings.HasPrefix(line, "EVENT:") {
				events = append(events, strings.TrimPrefix(line, "EVENT:"))
			}
		}
		if strings.Join(events, "\n") != strings.Join(row.Events, "\n") {
			t.Fatalf("ambient%d %+v\nGo=%q\nRust=%q", count, row.Request, events, row.Events)
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != profile.Records || aborts != profile.Aborts {
		t.Fatal("native record/abort count differs", count, aborts)
	}
}
