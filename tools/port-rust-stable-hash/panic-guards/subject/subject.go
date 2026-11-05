// SPDX-License-Identifier: MIT OR Apache-2.0
// Caller-owned non-Copy state exercising the unchanged source trait bridge.
package subject

import (
	"encoding/json"
	"fmt"
	hash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type Request struct {
	Operation string
	Context   string
	DropFail  bool
	Infos     map[string]rustfmt.SourcePanicInfo
}
type State struct {
	bytes    []byte
	dropFail bool
}

func event(text string) { fmt.Println("E:" + text) }
func caught(body func()) (result bool) {
	defer func() { result = recover() != nil }()
	body()
	return false
}
func local(context rustfmt.Context, name string, body func()) {
	defer func() {
		p := recover()
		event(fmt.Sprintf("local:%s:%t", name, context.Unwinding || p != nil))
		if p != nil {
			context.ResumeUnwind(p)
		}
	}()
	body()
}

func Run(q Request) {
	ops := &rustfmt.PanicOperations{}
	if info, ok := q.Infos["panic in a destructor during cleanup"]; ok {
		ops.CleanupAbort = &info
	}
	context := rustfmt.WithPanicOperations(rustfmt.Context{}, ops)
	var invoke func(rustfmt.Context)
	ops.Hook = func(info rustfmt.SourcePanicInfo, processing rustfmt.Context) {
		data, err := json.Marshal(info)
		if err != nil {
			panic(err)
		}
		fmt.Println("H:" + string(data))
		if q.Context == "processing" {
			invoke(processing)
		}
	}
	raise := func(ctx rustfmt.Context, name string) { ctx.PanicAt(q.Infos[name]) }
	stateOps := hash.ExtendedHasherOps[State, uint64]{
		Write: func(ref hash.Ref[State], bytes []byte) {
			ref.Value.bytes = append(ref.Value.bytes, bytes...)
			event(fmt.Sprintf("write:%t", ref.Context.Unwinding))
			local(ref.Context, "write", func() { raise(ref.Context, "write") })
		},
		Finish64: func(ref hash.Ref[State]) uint64 { return uint64(len(ref.Value.bytes)) },
		Clone: func(ref hash.Ref[State]) State {
			event(fmt.Sprintf("clone:%t", ref.Context.Unwinding))
			local(ref.Context, "clone", func() { raise(ref.Context, "clone") })
			return State{}
		},
		Finish: func(value *hash.Owned[State]) *hash.Owned[uint64] {
			ctx := value.Context()
			event(fmt.Sprintf("finish:%t", ctx.Unwinding))
			local(ctx, "finish", func() { raise(ctx, "finish") })
			return hash.Own(uint64(0))
		},
		Drop: func(value *State, ctx rustfmt.Context) {
			event(fmt.Sprintf("drop-h:%t", ctx.Unwinding))
			if value.dropFail {
				raise(ctx, "drop")
			}
		},
	}
	invoke = func(ctx rustfmt.Context) {
		event(fmt.Sprintf("enter:%t", ctx.Unwinding))
		h := hash.WithHasher(State{bytes: []byte{1, 2, 3}, dropFail: q.DropFail}, stateOps)
		h.SetContext(ctx)
		failed := caught(func() {
			switch q.Operation {
			case "write":
				h.Write([]byte("x"))
			case "clone":
				h.Clone().CloseWithContext(ctx)
			case "finish":
				h.FinishHash()
			default:
				panic("bad operation")
			}
		})
		event(fmt.Sprintf("caught:%t", failed))
		if q.Operation != "finish" {
			h.CloseWithContext(ctx)
		}
	}
	switch q.Context {
	case "inherited":
		caught(func() {
			defer func() {
				p := recover()
				unwind := context
				unwind.Unwinding = true
				invoke(unwind)
				if p != nil {
					context.ResumeUnwind(p)
				}
			}()
			raise(context, "outer")
		})
		event("outer-caught:true")
	case "processing":
		caught(func() { raise(context, "outer") })
		event("outer-caught:true")
	default:
		invoke(context)
	}
}
