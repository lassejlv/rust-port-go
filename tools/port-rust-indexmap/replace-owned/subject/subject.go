// SPDX-License-Identifier: MIT OR Apache-2.0
package subject

import (
	"encoding/json"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type Request struct {
	Operation, Context, Observer, Failure string
	Infos                                 map[string]rustfmt.SourcePanicInfo
}
type field struct {
	id   uint32
	data []byte
}
type value struct {
	id    uint32
	field field
}

func event(text string) { fmt.Println("E:" + text) }
func observer(c rustfmt.Context) string {
	if c.Runtime == nil || c.Runtime.Value == nil {
		return "none"
	}
	return c.Runtime.Value.(string)
}
func catch(body func()) (failed bool) {
	defer func() { failed = recover() != nil }()
	body()
	return false
}
func Run(q Request) {
	var context rustfmt.Context
	if q.Observer != "none" {
		context.Runtime = &rustfmt.RuntimeMetadata{Value: q.Observer}
	}
	var anchor *value
	fieldDrop := func(f *field, c rustfmt.Context) {
		event(fmt.Sprintf("field:%d:%s:%t", f.id, observer(c), c.Panicking()))
		if f.id == 1 && (q.Failure == "field" || q.Failure == "both") {
			c.PanicAt(q.Infos["field"])
		}
	}
	drop := func(v *value, c rustfmt.Context) {
		pending := true
		defer func() {
			if p := recover(); p != nil {
				unwind := c
				unwind.Unwinding = true
				if pending {
					func() {
						defer func() {
							if recover() != nil {
								unwind.AbortCleanup()
							}
						}()
						fieldDrop(&v.field, unwind)
					}()
				}
				c.ResumeUnwind(p)
			}
		}()
		original := v.id
		event(fmt.Sprintf("drop:%d:%s:%t:%t", original, observer(c), c.Panicking(), anchor == v))
		func() {
			defer func() {
				p := recover()
				event(fmt.Sprintf("local:%d:%s:%t", original, observer(c), c.Panicking() || p != nil))
				if p != nil {
					c.ResumeUnwind(p)
				}
			}()
			v.id += 1000
			event(fmt.Sprintf("mut:%d", v.id))
			if original == 1 && (q.Failure == "body" || q.Failure == "both") {
				c.PanicAt(q.Infos["body"])
			}
		}()
		pending = false
		fieldDrop(&v.field, c)
	}
	var action func(rustfmt.Context)
	action = func(c rustfmt.Context) {
		event(fmt.Sprintf("enter:%s:%t", observer(c), c.Panicking()))
		stale := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "stale-constructor"}}
		target := rustvec.NewOwned(value{1, field{1, []byte{1, 2, 3}}}, rustvec.ValueOps[value]{Drop: drop}).MoveWithContext(stale)
		replacement := rustvec.NewOwned(value{2, field{2, []byte{4, 5}}}, rustvec.ValueOps[value]{Drop: drop}).MoveWithContext(stale)
		defer target.DropWithContext(c)
		defer replacement.DropWithContext(c)
		anchor = target.Borrow()
		failed := catch(func() {
			if q.Operation == "assign" {
				rustvec.ReplaceOwnedWithContext(target, replacement, c)
			} else {
				source := replacement.Borrow()
				event(fmt.Sprintf("clone:%d:%s:%t", source.id, observer(c), c.Panicking()))
				cloned := value{source.id + 10, field{source.id + 10, append([]byte(nil), source.field.data...)}}
				rustvec.ReplaceOwnedWithContext(target, rustvec.NewOwned(cloned, rustvec.ValueOps[value]{Drop: drop}), c)
			}
		})
		event(fmt.Sprintf("caught:%t", failed))
		v := target.Borrow()
		event(fmt.Sprintf("state:%d:%d:%t", v.id, v.field.id, v == anchor))
		anchor = nil
	}
	if q.Observer != "none" {
		ops := &rustfmt.PanicOperations{}
		if info, ok := q.Infos["panic in a destructor during cleanup"]; ok {
			ops.CleanupAbort = &info
		}
		ops.Hook = func(info rustfmt.SourcePanicInfo, processing rustfmt.Context) {
			data, err := json.Marshal(info)
			if err != nil {
				panic(err)
			}
			fmt.Println("H:" + string(data))
			if q.Context == "processing" {
				action(processing)
			}
		}
		context = rustfmt.WithPanicOperations(context, ops)
	}
	switch q.Context {
	case "inherited":
		catch(func() {
			defer func() {
				p := recover()
				unwind := context
				unwind.Unwinding = true
				action(unwind)
				if p != nil {
					context.ResumeUnwind(p)
				}
			}()
			context.PanicAt(q.Infos["outer"])
		})
		event("outer-caught:true")
	case "processing":
		if q.Observer == "none" {
			processing := context
			processing.Unwinding = true
			processing.ProcessingPanic = true
			action(processing)
		} else {
			catch(func() { context.PanicAt(q.Infos["outer"]) })
		}
		event("outer-caught:true")
	default:
		action(context)
	}
}
