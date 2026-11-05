// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"encoding/json"
	"errors"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"reflect"
	"testing"
)

func TestDefIdOriginalPanicMetadata(t *testing.T) {
	f, err := os.Open("testdata/def_id_panic_metadata.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	n := 0
	for scanner.Scan() {
		var row struct {
			Request struct{ Op string }
			Failed  bool
			Hooks   []rustfmt.SourcePanicInfo
		}
		if err = json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		n++
		if len(row.Hooks) != 1 || row.Hooks[0].Location == nil {
			t.Fatalf("incomplete native metadata %s", scanner.Bytes())
		}

		site := ""
		switch row.Request.Op {
		case "crate_u32", "crate_usize", "crate_add_overflow", "crate_add_limit", "crate_idx_limit":
			site = CrateNumPanicSite
		case "def_u32", "def_usize", "def_add_overflow", "def_add_limit", "local_idx_limit", "def_idx_limit":
			site = DefIndexPanicSite
		case "local_idx_overflow", "crate_idx_overflow", "def_idx_overflow":
			site = LocalDefIdIdxPlusPanicSite
		case "crate_forward", "def_forward":
			site = DefIdStepForwardPanicSite
		case "crate_backward", "def_backward":
			site = DefIdStepBackwardPanicSite
		case "expect_local", "expect_partial":
			site = DefIdExpectLocalPanicSite
		case "mod_expect_local":
			site = ModIdExpectLocalPanicSite
		case "format_error":
			site = DefIdFormatErrorPanicSite
		}
		sites := map[string]rustfmt.SourceLocation{site: *row.Hooks[0].Location}

		var hooks []rustfmt.SourcePanicInfo
		context := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: sites, Hook: func(info rustfmt.SourcePanicInfo, processing rustfmt.Context) {
			if !processing.ProcessingPanic || !processing.Unwinding {
				t.Fatal("hook scope")
			}
			hooks = append(hooks, info)
		}})
		original := LoadDefIdDebug()
		if row.Request.Op == "expect_partial" || row.Request.Op == "format_error" {
			SwapDefIdDebug(NewDefIdDebugCallback(func(_ DefId, f *rustfmt.Formatter) error {
				if err := f.WriteString("piece"); err != nil {
					return err
				}
				return errors.New("fmt::Error")
			}))
		}
		_, failed := defIDCatch(func() any {
			switch row.Request.Op {
			case "crate_u32":
				CrateNumFromU32WithContext(0xffffff01, context)
			case "def_u32":
				DefIndexFromU32WithContext(0xffffff01, context)
			case "crate_usize":
				CrateNumFromUsizeWithContext(^uint(0), context)
			case "def_usize":
				DefIndexFromUsizeWithContext(^uint(0), context)
			case "local_idx_limit":
				LocalDefIdIdxWithContext(context).New(0xffffff01)
			case "local_idx_overflow":
				LocalDefIdIdxWithContext(context).Plus(LocalDefId{DefIndexFromU32(1)}, ^uint(0))
			case "crate_idx_overflow":
				CrateNumIdxWithContext(context).Plus(CrateNumFromU32(1), ^uint(0))
			case "def_idx_overflow":
				DefIndexIdxWithContext(context).Plus(DefIndexFromU32(1), ^uint(0))
			case "crate_idx_limit":
				CrateNumIdxWithContext(context).Plus(CrateNumMax(), 1)
			case "def_idx_limit":
				DefIndexIdxWithContext(context).Plus(DefIndexMax(), 1)
			case "crate_add_overflow":
				CrateNumFromU32(1).AddWithContext(^uint(0), context)
			case "def_add_overflow":
				DefIndexFromU32(1).AddWithContext(^uint(0), context)
			case "crate_add_limit":
				CrateNumMax().AddWithContext(1, context)
			case "def_add_limit":
				DefIndexMax().AddWithContext(1, context)
			case "crate_forward":
				CrateNumFromU32(1).ForwardWithContext(^uint(0), context)
			case "def_forward":
				DefIndexFromU32(1).ForwardWithContext(^uint(0), context)
			case "crate_backward":
				CrateNumZero().BackwardWithContext(1, context)
			case "def_backward":
				DefIndexZero().BackwardWithContext(1, context)
			case "expect_local", "expect_partial":
				DefId{Krate: CrateNumFromU32(1), Index: DefIndexFromU32(2)}.ExpectLocalWithContext(context)
			case "mod_expect_local":
				NewModIdUnchecked(DefId{Krate: CrateNumFromU32(1), Index: DefIndexFromU32(2)}).ExpectLocalWithContext(context)
			case "format_error":
				id := DefId{Krate: CrateNumFromU32(1), Index: DefIndexFromU32(2)}
				defIdDebugString(context, id.WriteDebugFormatter)
			default:
				t.Fatal("unknown operation", row.Request.Op)
			}
			return nil
		})
		SwapDefIdDebug(original)
		if failed != row.Failed || !reflect.DeepEqual(hooks, row.Hooks) {
			t.Fatalf("operation%s failed%t hooks%+v want%+v", row.Request.Op, failed, hooks, row.Hooks)
		}
	}
	if scanner.Err() != nil {
		t.Fatal(scanner.Err())
	}
	if n != 22 {
		t.Fatalf("records%d want22", n)
	}
}
