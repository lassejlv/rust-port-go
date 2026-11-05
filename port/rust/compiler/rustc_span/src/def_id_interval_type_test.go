// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"encoding/json"
	index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"os"
	"strings"
	"testing"
)

type defIdIntervalRow struct {
	Kind, TypeName, Compact, Pretty string
	Domain                          uint
}

func checkDefIdIntervalType[I any](t *testing.T, row defIdIntervalRow, idx index.Idx[I], step index.IntervalStep[I]) {
	t.Helper()
	if step.TypeName != row.TypeName {
		t.Fatalf("Step type-name %q want%q", step.TypeName, row.TypeName)
	}
	set := index.NewIntervalSet(idx, row.Domain, step)
	for _, n := range []uint{0, 2, 3, 7, 20, 21, 35} {
		if n < row.Domain {
			set.Insert(idx.New(n))
		}
	}
	if got := set.Debug(); got != row.Compact {
		t.Fatalf("compact %q want%q", got, row.Compact)
	}
	var out strings.Builder
	opts := index.DefaultIndexFormatOptions()
	opts.Alternate = true
	f := index.IndexDebugFormatter{Options: opts, Write: func(s string) error { out.WriteString(s); return nil }}
	if err := set.FormatDebug(&f); err != nil {
		t.Fatal(err)
	}
	if out.String() != row.Pretty {
		t.Fatalf("pretty %q want%q", out.String(), row.Pretty)
	}
}
func TestDefIdCanonicalIntervalTypes(t *testing.T) {
	data, err := os.ReadFile("testdata/def_id_interval_types.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []defIdIntervalRow
	if err = json.Unmarshal(data, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		t.Run(row.Kind, func(t *testing.T) {
			switch row.Kind {
			case "DefIndex":
				checkDefIdIntervalType(t, row, DefIndexIdx(), DefIndexStep())
			case "AttrId":
				checkDefIdIntervalType(t, row, AttrIdIdx(), AttrIdStep())
			case "CrateNum":
				checkDefIdIntervalType(t, row, CrateNumIdx(), CrateNumStep())
			default:
				t.Fatal("unknown source ID")
			}
		})
	}
	if len(rows) != 9 {
		t.Fatalf("cases%d want9", len(rows))
	}
}
