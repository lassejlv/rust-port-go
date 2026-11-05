// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual public import-overflow hook formatter branch; no private lock access.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

func sourceMapLockedImport(sm *SourceMap, name string, length uint32, ctx rustfmt.Context) {
	template, failure := NewSourceFile(CustomFileName(name), "", SourceFileHashMd5, None[SourceFileHashAlgorithm]())
	if failure != nil {
		panic(failure)
	}
	if source, ok := template.Src.Get(); ok {
		defer source.Close()
	}
	sm.NewImportedSourceFileWithContext(template.Name, template.SrcHash, template.ChecksumHash, template.StableId, length, template.UnnormalizedSourceLen, CrateNumFromU32(1), template.Lines, template.MultibyteChars, template.NormalizedPos, 0, ctx).CloseWithContext(ctx)
}
func TestSourceMapNativeLockedFormatterOracle(t *testing.T) {
	file, err := os.Open("testdata/source_map_formats_locked.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scan := bufio.NewScanner(z)
	scan.Buffer(make([]byte, 65536), 4<<20)
	count := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 6 {
			t.Fatal("invalid locked fixture")
		}
		style, _ := strconv.Atoi(p[1])
		fail, _ := strconv.Atoi(p[2])
		config := SpanEncodingConfig{DebugAssertions: p[0][1] == '1', OverflowChecks: p[0][3] == '1'}
		loader := NewOwnedFileLoaderWithOps(NewSpanOwned(struct{}{}, nil), FileLoaderOps[struct{}]{FileExists: func(*struct{}, RustPath, rustfmt.Context) bool { return true }, ReadFile: func(*struct{}, RustPath, rustfmt.Context) (string, error) { return "", nil }, ReadBinaryFile: func(*struct{}, RustPath, rustfmt.Context) (Shared[[]byte], error) {
			return NewShared([]byte{}, nil), nil
		}, CurrentDirectory: func(*struct{}, rustfmt.Context) (RustPath, error) { return NewRustPath("/foo"), nil }})
		sm := NewSourceMapWithInputsAndConfig(SourceMapInputs{FileLoader: loader, PathMapping: EmptyFilePathMapping(), HashKind: SourceFileHashMd5}, config)
		sourceMapLockedImport(sm, "locked-a", 0, rustfmt.Context{})
		sourceMapLockedImport(sm, "locked-b", ^uint32(0), rustfmt.Context{})
		record := ""
		locations := LibPanicLocationsAtPinnedRevision("original/compiler/rustc_span/src/lib.rs", "unused-scoped-tls")
		ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: locations, Hook: func(info rustfmt.SourcePanicInfo, context rustfmt.Context) {
			sink := &sourceMapFormatSink{fail: fail, address: fmt.Sprintf("%x", uintptr(unsafe.Pointer(&sm.fileLoader)))}
			f := &rustfmt.Formatter{Writer: sink, Options: sourceMapFormatOptions(style), Context: context}
			first := sm.WriteDebugFormatter(f)
			after := sourceMapFormatFlags(f.Options)
			second := rustfmt.FormatDebugUint(7, 16, f)
			record = fmt.Sprintf("%t:%s:%s:%d:%d:%x:%t:%x:%x:%x", context.Panicking(), info.Kind, info.Location.File, info.Location.Line, info.Location.Column, fmt.Sprint(info.Payload), first == nil && second == nil, sink.out.String(), strings.Join(sink.calls, "\n"), after)
		}})
		caught := false
		func() {
			defer func() {
				if recover() != nil {
					caught = true
				}
			}()
			sourceMapLockedImport(sm, "locked-c", 0, ctx)
		}()
		guard := sm.Files()
		length := guard.Len()
		guard.Close()
		sm.Close()
		if strconv.FormatBool(caught) != p[3] || record != sourceMapOracleString(p[4]) || fmt.Sprint(length) != p[5] {
			t.Fatalf("%s style%d fail%d caught%t len%d record%q want%s/%s/%q", p[0], style, fail, caught, length, record, p[3], p[5], sourceMapOracleString(p[4]))
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d actual public-trigger locked Debug scripts", count)
}
