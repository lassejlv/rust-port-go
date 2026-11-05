package rustcspan

import (
	"bufio"
	"encoding/json"
	"fmt"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

type defIDContextCase struct {
	Kind, Context                    string
	Once, Default, Get, Drop, Forget bool
	FieldDrop                        bool    `json:"field_drop"`
	PanicCall                        *uint64 `json:"panic_call"`
}

type defIDEnv struct {
	q       defIDContextCase
	event   func(string)
	calls   *uint64
	bump    uint64
	nonCopy []byte
}

func (s *defIDEnv) key(id DefId, extra uint64) DefPathHash {
	return DefPathHash{datastructures.NewFingerprint(rustchashes.NewHash64(uint64(id.Krate.AsU32())), rustchashes.NewHash64(uint64(id.Index.AsU32())+s.bump+extra))}
}
func (s *defIDEnv) callEvent(kind string, id DefId, ctx rustfmt.Context) {
	n := *s.calls
	*s.calls++
	s.event(fmt.Sprintf("call:%s:%d:%d:panic=%t", kind, id.Krate.AsU32(), id.Index.AsU32(), ctx.Panicking()))
	if s.q.PanicCall != nil && *s.q.PanicCall == n {
		ctx.Panic("call")
	}
}
func defIDDropEnv(s *defIDEnv, ctx rustfmt.Context) {
	field := func(c rustfmt.Context) {
		s.event(fmt.Sprintf("field-drop:panic=%t", c.Panicking()))
		if s.q.FieldDrop {
			c.Panic("field drop")
		}
	}
	defer func() {
		p := recover()
		if p != nil {
			unwind := ctx
			unwind.Unwinding = true
			func() {
				defer func() {
					if recover() != nil {
						os.Exit(134)
					}
				}()
				field(unwind)
			}()
			panic(p)
		}
		field(ctx)
	}()
	s.event(fmt.Sprintf("env-drop:panic=%t", ctx.Panicking()))
	if s.q.Drop {
		ctx.Panic("env drop")
	}
}

type defIDProvider struct {
	q     defIDContextCase
	event func(string)
	calls uint64
}

func (p *defIDProvider) DefPathHasherWithContext(ctx rustfmt.Context) DefPathHasher {
	p.event(fmt.Sprintf("new:panic=%t", ctx.Panicking()))
	if p.q.Get {
		ctx.Panic("get")
	}
	ops := DefPathHasherOps[defIDEnv]{Call: func(ref rustvec.Ref[defIDEnv], id DefId) DefPathHash {
		ref.Value.callEvent("fn", id, ref.Context)
		return ref.Value.key(id, 0)
	}, CallMut: func(ref rustvec.Ref[defIDEnv], id DefId) DefPathHash {
		ref.Value.bump += 10
		ref.Value.callEvent("mut", id, ref.Context)
		return ref.Value.key(id, 0)
	}, CallOnce: func(arg *rustvec.Owned[defIDEnv], id DefId) DefPathHash {
		s := arg.Borrow()
		s.callEvent("once", id, arg.Context())
		result := s.key(id, 100)
		if s.q.Forget {
			arg.Take()
		}
		return result
	}, Drop: defIDDropEnv}
	owned := rustvec.NewOwned(defIDEnv{p.q, p.event, &p.calls, 0, []byte{1, 2, 3}}, rustvec.ValueOps[defIDEnv]{Drop: ops.Drop})
	return NewOwnedDefPathHasherWithOpsWithContext(owned, ops, ctx)
}
func defIDObserveContext(q defIDContextCase, live bool) []string {
	events := []string{}
	event := func(s string) {
		events = append(events, s)
		if live {
			fmt.Println("EVENT\t" + s)
		}
	}
	ctx := rustfmt.Context{Unwinding: q.Context == "inherited", ProcessingPanic: q.Context == "processing"}
	if q.Context == "inherited" {
		event("inherited:panic=true")
	}
	if q.Context == "processing" {
		event("processing:panic=true")
	}
	_, panicked := defIDCatch(func() any {
		provider := &defIDProvider{q: q, event: event}
		var source any = provider
		if q.Default {
			source = struct{}{}
		}
		id := func(c, i uint32) DefId { return DefId{Krate: CrateNumFromU32(c), Index: DefIndexFromU32(i)} }
		order := func(n int) string {
			if n < 0 {
				return "Less"
			}
			if n > 0 {
				return "Greater"
			}
			return "Equal"
		}
		switch q.Kind {
		case "raw":
			h := captureDefPathHasher(source, ctx)
			arg := rustvec.NewOwned(h, rustvec.ValueOps[DefPathHasher]{Drop: func(h *DefPathHasher, c rustfmt.Context) { h.CloseWithContext(c) }})
			rustvec.ConsumeValue(arg, ctx, func(arg *rustvec.Owned[DefPathHasher]) struct{} {
				h := arg.Borrow()
				a := h.CallWithContext(id(1, 2), ctx)
				event(fmt.Sprintf("value:%d", a.LocalHash().AsU64()))
				b := h.CallMutWithContext(id(1, 3), ctx)
				event(fmt.Sprintf("value:%d", b.LocalHash().AsU64()))
				c := h.CallOnceWithContext(id(1, 4), ctx)
				event(fmt.Sprintf("value:%d", c.LocalHash().AsU64()))
				return struct{}{}
			})
		case "crate":
			cmp := CrateNumStableCmpWithContext(source, ctx)
			if q.Once {
				event("value:" + order(cmp.CallOnceWithContext(CrateNumFromU32(2), CrateNumFromU32(1), ctx)))
			} else {
				_, p := defIDCatch(func() any { return cmp.CompareWithContext(CrateNumFromU32(2), CrateNumFromU32(1), ctx) })
				event(fmt.Sprintf("inner-caught:%t", p))
				cmp.CloseWithContext(ctx)
			}
		case "local":
			cmp := LocalDefIdStableCmpWithContext(source, ctx)
			a, b := LocalDefId{DefIndexFromU32(2)}, LocalDefId{DefIndexFromU32(1)}
			if q.Once {
				event("value:" + order(cmp.CallOnceWithContext(a, b, ctx)))
			} else {
				_, p := defIDCatch(func() any { return cmp.CompareWithContext(a, b, ctx) })
				event(fmt.Sprintf("inner-caught:%t", p))
				cmp.CloseWithContext(ctx)
			}
		default:
			cmp := DefIdStableCmpWithContext(source, ctx)
			if q.Once {
				event("value:" + order(cmp.CallOnceWithContext(id(2, 2), id(1, 1), ctx)))
			} else {
				_, p := defIDCatch(func() any { return cmp.CompareWithContext(id(2, 2), id(1, 1), ctx) })
				event(fmt.Sprintf("inner-caught:%t", p))
				cmp.CloseWithContext(ctx)
			}
		}
		return nil
	})
	event(fmt.Sprintf("caught:%t", panicked))
	if q.Context == "inherited" || q.Context == "processing" {
		event("outer-caught:true")
	}
	return events
}
func TestDefIdContextChild(t *testing.T) {
	s := os.Getenv("FERRUGO_DEF_ID_CONTEXT_CHILD")
	if s == "" {
		return
	}
	var q defIDContextCase
	if e := json.Unmarshal([]byte(s), &q); e != nil {
		panic(e)
	}
	defIDObserveContext(q, true)
	os.Exit(0)
}
func TestDefIdOriginalOwnedContext(t *testing.T) {
	path := os.Getenv("FERRUGO_DEF_ID_CONTEXT_FIXTURE")
	if path == "" {
		path = "testdata/def_id_context.jsonl"
	}
	f, e := os.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	scan := bufio.NewScanner(f)
	n := 0
	for scan.Scan() {
		n++
		var row struct {
			Request defIDContextCase
			Code    int
			Events  []string
		}
		if e := json.Unmarshal(scan.Bytes(), &row); e != nil {
			t.Fatal(e)
		}
		if row.Code == 0 {
			actual := defIDObserveContext(row.Request, false)
			if !reflect.DeepEqual(actual, row.Events) {
				t.Fatalf("row%d %+v\nGo%v\nRust%v", n, row.Request, actual, row.Events)
			}
			continue
		}
		q, _ := json.Marshal(row.Request)
		cmd := exec.Command(os.Args[0], "-test.run=^TestDefIdContextChild$")
		cmd.Env = append(os.Environ(), "FERRUGO_DEF_ID_CONTEXT_CHILD="+string(q))
		out, e := cmd.Output()
		exit, ok := e.(*exec.ExitError)
		if !ok || exit.ExitCode() != 134 {
			t.Fatalf("row%d exit%v/%s", n, e, out)
		}
		events := []string{}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if !strings.HasPrefix(line, "EVENT\t") {
				t.Fatal(line)
			}
			events = append(events, strings.TrimPrefix(line, "EVENT\t"))
		}
		if !reflect.DeepEqual(events, row.Events) {
			t.Fatalf("row%d %+v\nGo%v\nRust%v", n, row.Request, events, row.Events)
		}
	}
	if e := scan.Err(); e != nil {
		t.Fatal(e)
	}
	if n < 100 {
		t.Fatal("incomplete context corpus", n)
	}
}
