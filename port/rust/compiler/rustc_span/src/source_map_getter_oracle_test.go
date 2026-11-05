// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestSourceMapScopedGetterNativeOracle(t *testing.T) {
	file, err := os.Open("testdata/source_map_getter.tsv.gz")
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
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 2 {
			t.Fatal("invalid getter fixture")
		}
		cfg := SpanEncodingConfig{DebugAssertions: p[0][1] == '1', OverflowChecks: p[0][3] == '1'}
		var out strings.Builder
		for _, present := range []bool{false, true} {
			scope := NewSessionScope()
			inputs := None[SourceMapInputs]()
			if present {
				loader := NewOwnedFileLoaderWithOps(NewSpanOwned(struct{}{}, func(_ *struct{}, c rustfmt.Context) { fmt.Fprintf(&out, "loader_drop:%t\n", c.Panicking()) }), FileLoaderOps[struct{}]{FileExists: func(*struct{}, RustPath, rustfmt.Context) bool { return true }, ReadFile: func(*struct{}, RustPath, rustfmt.Context) (string, error) { return "", nil }, ReadBinaryFile: func(*struct{}, RustPath, rustfmt.Context) (Shared[[]byte], error) {
					return NewShared([]byte{}, nil), nil
				}, CurrentDirectory: func(*struct{}, rustfmt.Context) (RustPath, error) { return NewRustPath("/foo"), nil }})
				inputs = Some(SourceMapInputs{FileLoader: loader, PathMapping: EmptyFilePathMapping(), HashKind: SourceFileHashMd5})
			}
			session := NewSessionGlobals(Edition2024, nil, inputs, cfg)
			SetSessionGlobalsThen(scope, session, func() struct{} {
				if a, ok := GetSourceMapInScope(scope).Get(); ok {
					b, _ := GetSourceMapInScope(scope).Get()
					fmt.Fprintf(&out, "some:%d:%d:%t\n", a.StrongCount(), b.StrongCount(), a.Borrow() == b.Borrow())
					source := a.Borrow().NewSourceFile(CustomFileName("getter.rs"), "abc")
					source.Close()
					guard := b.Borrow().Files()
					fmt.Fprintf(&out, "shared_files:%d\n", guard.Len())
					guard.Close()
					a.Close()
					fmt.Fprintf(&out, "after_drop:%d\n", b.StrongCount())
					b.Close()
				} else {
					fmt.Fprintf(&out, "none:%t\n", !GetSourceMapInScope(scope).IsSome())
				}
				return struct{}{}
			})
			session.Close()
			fmt.Fprintf(&out, "after:%t\n", present)
		}
		if want := sourceMapOracleString(p[1]); out.String() != want {
			t.Fatalf("%s getter got%q want%q", p[0], out.String(), want)
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
}
