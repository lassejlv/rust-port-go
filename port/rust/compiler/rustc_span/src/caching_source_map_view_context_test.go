// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
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
)

type cachingContextRecord struct {
	Profile, Mode, Header, Operation string
	Aborted                          bool
	Lines                            []string
}

func cachingContextReplay(q cachingContextRecord, emit func(string)) {
	cfg := SpanEncodingConfig{DebugAssertions: q.Profile[1] == '1', OverflowChecks: q.Profile[3] == '1'}
	mapValue := NewSourceMapWithConfig(EmptyFilePathMapping(), cfg)
	defer mapValue.Close()
	if strings.HasPrefix(q.Header, "I,") {
		cachingOracleImport(mapValue, q.Header)
	} else {
		n, _ := strconv.Atoi(q.Header)
		for i := 0; i < n; i++ {
			cachingOracleAdd(mapValue, i)
		}
	}
	var view *CachingSourceMapView
	if q.Operation != "N" {
		view = NewCachingSourceMapView(mapValue)
		defer view.Close()
	}
	ctx := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "active operation"}}
	ctx.Unwinding = q.Mode == "caught" || q.Mode == "escaping"
	ctx.ProcessingPanic = q.Mode == "processing"
	event := func(label string, c rustfmt.Context) {
		emit(fmt.Sprintf("E:%s:%t:%s", label, c.Panicking(), cachingOracleCounts(mapValue)))
	}
	operation := func() {
		defer func() {
			payload := recover()
			dropContext := ctx
			if payload != nil {
				dropContext.Unwinding = true
			}
			event("local", dropContext)
			if payload != nil {
				panic(payload)
			}
		}()
		event("before", ctx)
		p := strings.Split(q.Operation, ",")
		switch p[0] {
		case "N":
			v := NewCachingSourceMapViewWithContext(mapValue, ctx)
			event("new", ctx)
			v.CloseWithContext(ctx)
		case "b":
			n, _ := strconv.ParseUint(p[1], 10, 32)
			v, ok := view.BytePosToLineAndColWithContext(BytePos(n), ctx)
			if ok {
				event(fmt.Sprintf("b:%d:%d:%d", v.File.Borrow().StartPos, v.Line, v.Column), ctx)
				v.File.CloseWithContext(ctx)
			} else {
				event("none", ctx)
			}
		case "s":
			lo, _ := strconv.ParseUint(p[1], 10, 32)
			hi, _ := strconv.ParseUint(p[2], 10, 32)
			v, ok := view.SpanDataToLinesAndColsWithContext(&SpanData{Lo: BytePos(lo), Hi: BytePos(hi), Ctxt: RootSyntaxContext()}, ctx)
			if ok {
				event(fmt.Sprintf("s:%d:%d:%d:%d:%d", v.File.StartPos, v.LoLine, v.LoColumn, v.HiLine, v.HiColumn), ctx)
			} else {
				event("none", ctx)
			}
		}
	}
	returned := false
	if q.Mode == "processing" {
		operation()
		returned = true
	} else {
		panicked := false
		func() { defer func() { panicked = recover() != nil }(); operation() }()
		if panicked && q.Mode == "escaping" {
			os.Exit(134)
		}
		returned = !panicked
	}
	result := fmt.Sprintf("%t:%s:%s:%t", returned, cachingOracleCounts(mapValue), cachingOracleMetadata(mapValue), q.Mode == "processing")
	if view != nil {
		view.Close()
	}
	emit(result + ":drop:" + cachingOracleCounts(mapValue))
}

func TestCachingSourceMapContextChild(t *testing.T) {
	encoded, ok := os.LookupEnv("FERRUGO_CACHING_CONTEXT_CHILD")
	if !ok {
		return
	}
	var q cachingContextRecord
	if err := json.Unmarshal([]byte(encoded), &q); err != nil {
		t.Fatal(err)
	}
	cachingContextReplay(q, func(line string) {
		if !strings.HasPrefix(line, "E:") {
			line = "R:" + line
		}
		fmt.Println(line)
	})
	os.Exit(0)
}

func TestOriginalCachingSourceMapContexts(t *testing.T) {
	manifest, err := os.ReadFile(filepath.Join("../../../../../tools/port-rust-caching-source-map-view", "context-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var proof struct {
		Records int             `json:"records"`
		SHA     string          `json:"fixture_sha256"`
		Files   []spanProofFile `json:"files"`
	}
	if err := json.Unmarshal(manifest, &proof); err != nil {
		t.Fatal(err)
	}
	verifySpanProofFiles(t, proof.Files)
	compressed, err := os.ReadFile("testdata/caching_source_map_view_context.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != proof.SHA {
		t.Fatal("native context fixture SHA-256 differs")
	}
	file, err := os.Open("testdata/caching_source_map_view_context.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	r, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	scanner := bufio.NewScanner(r)
	count := 0
	for scanner.Scan() {
		var q cachingContextRecord
		if err := json.Unmarshal(scanner.Bytes(), &q); err != nil {
			t.Fatal(err)
		}
		var lines []string
		if q.Aborted {
			data, _ := json.Marshal(q)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCachingSourceMapContextChild$")
			cmd.Env = append(os.Environ(), "FERRUGO_CACHING_CONTEXT_CHILD="+string(data))
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			err := cmd.Run()
			cancel()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("child%d exited %v: %s", count, err, &stderr)
			}
			lines = strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
		} else {
			cachingContextReplay(q, func(line string) {
				if !strings.HasPrefix(line, "E:") {
					line = "R:" + line
				}
				lines = append(lines, line)
			})
		}
		if !reflect.DeepEqual(lines, q.Lines) {
			t.Fatalf("context%d %s %s %s %s\ngot %q\nwant %q", count, q.Profile, q.Mode, q.Header, q.Operation, lines, q.Lines)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != proof.Records {
		t.Fatalf("native context count%d differs from%d", count, proof.Records)
	}
	t.Logf("%d actual-source runtime/cleanup records match", count)
}
