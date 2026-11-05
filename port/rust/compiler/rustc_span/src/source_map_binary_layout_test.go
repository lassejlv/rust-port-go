// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestSourceMapBinaryArcLayoutNativeOracle(t *testing.T) {
	file, err := os.Open("testdata/source_map_binary_layout.tsv.gz")
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
	count := 0
	const source = "/rustc/708c94ef093ecc42fb8851e17d8383d999727abc/library/alloc/src/rcs/arc.rs"
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 6 {
			t.Fatal("invalid Arc layout fixture")
		}
		bits, _ := strconv.ParseUint(p[0], 10, 8)
		metadata, _ := strconv.ParseUint(p[2], 10, 64)
		caught := false
		state, events := "", ""
		if p[1] == "layout" {
			length, size, align, kind := sourceMapBinaryArcLayout(metadata, uint(bits))
			state = kind
			if kind == "ok" {
				state = fmt.Sprintf("ok:%d:%d:%d", length, size, align)
			}
		} else {
			if int(bits) != strconv.IntSize {
				continue
			}
			ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: map[string]rustfmt.SourceLocation{SourceMapPanicBinaryArrayLayout: {File: source, Line: 2381, Column: 41}, SourceMapPanicBinaryArcCapacity: {File: source, Line: 417, Column: 29}}, Hook: func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
				if !current.Panicking() || !current.ProcessingPanic || info.Location == nil {
					t.Fatal("wrong binary allocation panic context/profile")
				}
				events = fmt.Sprintf("%s|%s|%d|%d|%x", info.Kind, info.Location.File, info.Location.Line, info.Location.Column, fmt.Sprint(info.Payload))
			}})
			func() {
				defer func() {
					if recover() != nil {
						caught = true
					}
				}()
				length := sourceMapBinaryArcLength(metadata, ctx)
				state = fmt.Sprintf("ok:%d", length)
			}()
		}
		if strconv.FormatBool(caught) != p[3] || state != p[4] || events != sourceMapOracleString(p[5]) {
			t.Fatalf("%v got%t/%q/%q want%s/%q/%q", p[:3], caught, state, events, p[3], p[4], sourceMapOracleString(p[5]))
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d actual SDK Arc/public-layout records", count)
}
func TestSourceMapBinaryArcLayout32BitCastAndAlignment(t *testing.T) {
	for _, test := range []struct {
		metadata, length, size uint64
		kind                   string
	}{{0, 0, 8, "ok"}, {1, 1, 12, "ok"}, {2147483636, 2147483636, 2147483644, "ok"}, {2147483637, 2147483637, 0, "header"}, {2147483647, 2147483647, 0, "header"}, {2147483648, 2147483648, 0, "array"}, {4294967295, 4294967295, 0, "array"}, {4294967296, 0, 8, "ok"}, {^uint64(0), 4294967295, 0, "array"}} {
		length, size, align, kind := sourceMapBinaryArcLayout(test.metadata, 32)
		if length != test.length || size != test.size || align != 4 || kind != test.kind {
			t.Fatalf("metadata%d got%d/%d/%d/%s", test.metadata, length, size, align, kind)
		}
	}
}
