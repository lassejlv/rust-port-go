package rustcserialize

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type mapOracleEncoder struct {
	*MemEncoder
	events *[]string
	mode   int
}

func (e *mapOracleEncoder) DeterministicHashMapEncode() bool { return true }
func (e *mapOracleEncoder) EmitUsize(v uint) {
	*e.events = append(*e.events, fmt.Sprintf("len:%d", v))
	if e.mode == 2 {
		e.RuntimeContext().Panic("length")
	}
	e.MemEncoder.EmitUsize(v)
}
func (e *mapOracleEncoder) EmitU32(v uint32) {
	*e.events = append(*e.events, fmt.Sprintf("num:%d", v))
	e.MemEncoder.EmitU32(v)
}

func TestOriginalDeterministicMapDefaults(t *testing.T) {
	f, err := os.Open("testdata/map-defaults.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	raw, err := io.ReadAll(io.LimitReader(g, 4<<20+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 4<<20 {
		t.Fatal("map fixture exceeds bound")
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 1<<20)
	count := 0
	for scan.Scan() {
		count++
		fields := strings.Split(scan.Text(), "\t")
		if len(fields) != 5 {
			t.Fatalf("row %d fields", count)
		}
		mode, err := strconv.Atoi(fields[1])
		if err != nil {
			t.Fatal(err)
		}
		fail, err := strconv.Atoi(fields[2])
		if err != nil {
			t.Fatal(err)
		}
		m := mapTestMap()
		if fields[0] != "" {
			for _, s := range strings.Split(fields[0], ",") {
				n, err := strconv.ParseUint(s, 10, 32)
				if err != nil {
					t.Fatal(err)
				}
				m.Insert(uint32(n), uint32(n)*7, rustfmt.Context{})
			}
		}
		events := []string{}
		e := &mapOracleEncoder{MemEncoder: NewMemEncoder(), events: &events, mode: mode}
		type env struct{ calls int }
		ops := HashMapEncodeOps[uint32, uint32]{Key: func(k *uint32, e Encoder, _ rustfmt.Context) {
			events = append(events, fmt.Sprintf("key:%d", *k))
			e.EmitU32(*k)
		}, Value: func(v *uint32, e Encoder, _ rustfmt.Context) { e.EmitU32(*v) }, Comparator: func(_ Encoder, ctx rustfmt.Context) EncodeKeyComparator[uint32] {
			events = append(events, "factory")
			if mode == 1 {
				ctx.Panic("factory")
			}
			cell := rustvec.NewOwned(env{}, rustvec.ValueOps[env]{Drop: func(v *env, ctx rustfmt.Context) {
				events = append(events, fmt.Sprintf("drop:%d:%t", v.calls, ctx.Panicking()))
			}})
			return NewOwnedEncodeKeyComparator(cell, func(ref rustvec.Ref[env], a, b *uint32) int {
				n := ref.Value.calls
				ref.Value.calls++
				events = append(events, fmt.Sprintf("cmp:%d:%d:%d", n, *a, *b))
				if n == fail {
					ref.Context.Panic("comparison")
				}
				if *a < *b {
					return -1
				}
				if *a > *b {
					return 1
				}
				return 0
			}, ctx)
		}}
		panicked := false
		func() { defer func() { panicked = recover() != nil }(); EncodeHashMap(e, m, ops) }()
		m.Close(rustfmt.Context{})
		if strconv.FormatBool(panicked) != fields[3] || strings.Join(events, ",") != fields[4] {
			t.Fatalf("row %d input=%s mode=%d fail=%d\npanic=%t events=%s\nwant panic=%s events=%s", count, fields[0], mode, fail, panicked, strings.Join(events, ","), fields[3], fields[4])
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 768 {
		t.Fatalf("records=%d", count)
	}
}
