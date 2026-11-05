// SPDX-License-Identifier: MIT OR Apache-2.0
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

func TestSourceMapNativePredicateBorrowOracle(t *testing.T) {
	f, err := os.Open("testdata/source_map_predicate_borrow.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scan := bufio.NewScanner(z)
	count := 0
	sessions := map[string]*SessionGlobals{}
	defer func() {
		for _, s := range sessions {
			s.Close()
		}
	}()
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 6 {
			t.Fatal("invalid predicate record")
		}
		config := SpanEncodingConfig{DebugAssertions: p[0][1] == '1', OverflowChecks: p[0][3] == '1'}
		session := sessions[p[0]]
		if session == nil {
			session = NewSpanEncodingSession(config)
			sessions[p[0]] = session
		}
		lo, _ := strconv.ParseUint(p[2], 10, 32)
		hi, _ := strconv.ParseUint(p[3], 10, 32)
		limit, _ := strconv.ParseUint(p[4], 10, 64)
		sm := NewSourceMapWithConfig(EmptyFilePathMapping(), config)
		sm.NewSourceFile(CustomFileName("borrow.rs"), sourceMapOracleString(p[1])).Close()
		type env struct {
			count, limit uint
			tag          []byte
		}
		var borrowedEnv, borrowedChar uintptr
		events := []string{}
		owned := NewSpanOwned(env{limit: uint(limit), tag: []byte{1, 2}}, func(value *env, context rustfmt.Context) {
			address := uintptr(unsafe.Pointer(value))
			events = append(events, fmt.Sprintf("drop:%d:%t:%t", value.count, borrowedEnv == 0 || borrowedEnv == address, context.Panicking()))
		})
		out := SpanTakeWhileOwned(sm, session, WithRootCtxt(session, BytePos(lo), BytePos(hi), config), owned, func(value *env, char *rune, context rustfmt.Context) bool {
			address := uintptr(unsafe.Pointer(value))
			previous := borrowedEnv
			borrowedEnv = address
			character := uintptr(unsafe.Pointer(char))
			last := borrowedChar
			borrowedChar = character
			n := value.count
			value.count++
			result := n < value.limit
			events = append(events, fmt.Sprintf("call:%x:%t:%t:%t:%t", *char, result, previous == 0 || previous == address, last == 0 || last == character, context.Panicking()))
			return result
		}, rustfmt.Context{})
		events = append(events, fmt.Sprintf("out:%d:%d", out.Lo(session), out.Hi(session)))
		sm.Close()
		want := sourceMapOracleString(p[5])
		if strings.Join(events, "|") != want {
			t.Fatalf("record%d profile%s src%q span%s:%s limit%s got%q want%q", count, p[0], sourceMapOracleString(p[1]), p[2], p[3], p[4], events, want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d whole-source borrowed predicate cases", count)
}
