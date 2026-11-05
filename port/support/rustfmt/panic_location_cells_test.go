// SPDX-License-Identifier: MIT OR Apache-2.0
package rustfmt

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestPanicLocationCellsNative(t *testing.T) {
	file, err := os.Open("testdata/panic-location-cells.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	const name = "tools/port-rust-panic-runtime/location-cells/oracle.rs"
	a := SourceLocation{File: name, Line: 4, Column: 29}
	b := SourceLocation{File: name, Line: 5, Column: 29}
	var cells []*SourceLocation
	ctx := WithPanicOperations(Context{}, &PanicOperations{LocationCells: map[string]*SourceLocation{"a": &a, "b": &b}, Hook: func(info SourcePanicInfo, current Context) {
		if !current.ProcessingPanic || !current.Unwinding {
			t.Fatal("wrong hook context")
		}
		cells = append(cells, info.Location)
	}})
	scan := bufio.NewScanner(z)
	count, hooks, pairs := 0, 0, 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 3 {
			t.Fatal("invalid location-cell fixture")
		}
		cells = nil
		for _, site := range p[0] {
			func() {
				defer func() {
					if recover() != "same" {
						t.Fatal("panic payload was wrapped")
					}
				}()
				ctx.PanicAt(SourcePanicInfo{Payload: "same", Kind: PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation(string(site))})
			}()
		}
		var matrix strings.Builder
		var names []string
		for _, a := range cells {
			names = append(names, fmt.Sprintf("%s:%d:%d", a.File, a.Line, a.Column))
			for _, b := range cells {
				if a == b {
					matrix.WriteByte('1')
				} else {
					matrix.WriteByte('0')
				}
			}
		}
		if strings.Join(names, "|") != p[1] || matrix.String() != p[2] {
			t.Fatalf("%q got%q/%q want%q/%q", p[0], names, matrix.String(), p[1], p[2])
		}
		hooks += len(cells)
		pairs += len(cells) * len(cells)
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 107 || hooks != 1283 || pairs != 20635 {
		t.Fatalf("incomplete native location corpus: %d scripts/%d hooks/%d pairs", count, hooks, pairs)
	}
}
func TestPanicLocationCellsPrecedenceAndConveniences(t *testing.T) {
	cell := &SourceLocation{File: "verbatim/path", Line: 7, Column: 9}
	fallback := SourceLocation{File: "fallback", Line: 1, Column: 2}
	ctx := WithPanicOperations(Context{}, &PanicOperations{LocationCells: map[string]*SourceLocation{"cell": cell, "nil": nil}, Locations: map[string]SourceLocation{"cell": fallback, "nil": fallback, "value": fallback}})
	if ctx.SourcePanicLocation("cell") != cell || ctx.SourcePanicLocation("cell") != ctx.SourcePanicLocation("cell") {
		t.Fatal("primary cell was copied")
	}
	if ctx.SourcePanicLocation("nil") != nil || ctx.SourcePanicLocation("missing") != nil || (Context{}).SourcePanicLocation("cell") != nil {
		t.Fatal("explicit nil/default must remain nil")
	}
	a, b := ctx.SourcePanicLocation("value"), ctx.SourcePanicLocation("value")
	if a == b || *a != fallback || *b != fallback {
		t.Fatal("value-profile convenience must retain independent copies")
	}
	a.File = "local copy"
	if ctx.SourcePanicLocation("value").File != "fallback" {
		t.Fatal("value profile mutated")
	}
}
