// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// These concrete witnesses implement the default adapter methods selected by
// the retained UnordItems get_only/count calls. They grant no std iterator credit.
// Count consumes a freshly constructed adapter; get_only consumes and drops a
// separately constructed adapter, including its partially traversed state.
type adapterItemsRequest struct {
	Length    int
	Inner     int
	Keep      uint64
	NextFail  int  `json:"next_fail"`
	CallFail  int  `json:"call_fail"`
	ItemFail  int  `json:"item_fail"`
	EnvFail   bool `json:"env_fail"`
	TokenFail bool `json:"token_fail"`
	InputFail bool `json:"input_fail"`
	Resume    bool
}
type adapterItemsCase struct {
	Mode            string
	Family          int
	Request         adapterItemsRequest
	Outcome, Stdout string
}
type adapterItemsManifest struct {
	Cases, Aborts int
	Locations     map[string]rustfmt.SourceLocation
	Raw           string `json:"raw_sha256"`
	Fixture       string `json:"fixture_sha256"`
	Files         map[string]string
	Reference     map[string]string `json:"reference_inputs"`
}
type adapterItemsValue struct{ id uint64 }
type adapterItemsInput struct {
	next, length int
	kind         uint64
}
type adapterItemsOverride struct{ input adapterItemsInput }
type adapterItemsFunction struct{ calls int }
type adapterItemsMap[A, B any] struct {
	input    *rustvec.Owned[adapterItemsInput]
	function UnordFunction[A, B]
}
type adapterItemsFlat struct {
	mapped *rustvec.Owned[adapterItemsMap[adapterItemsValue, adapterItemsInput]]
	front  *rustvec.Owned[adapterItemsInput]
}
type adapterItemsChain struct {
	left, right *rustvec.Owned[adapterItemsInput]
}
type adapterItemsRunner struct {
	spec      adapterItemsCase
	locations map[string]rustfmt.SourceLocation
	out       io.Writer
}

func (r *adapterItemsRunner) event(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}
func (r *adapterItemsRunner) raise(label string, c rustfmt.Context) {
	if r.spec.Request.Resume {
		c.ResumeUnwind(label)
	}
	location := r.locations[label]
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
}
func adapterItemsCleanup(c rustfmt.Context, drop func(rustfmt.Context)) {
	if p := recover(); p != nil {
		if c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		unwind := c
		unwind.Unwinding = true
		bagItemsUnwind(unwind, func() { drop(unwind) })
		c.ResumeUnwind(p)
	}
	drop(c)
}
func (r *adapterItemsRunner) value(id uint64) *rustvec.Owned[adapterItemsValue] {
	return rustvec.NewOwned(adapterItemsValue{id}, rustvec.ValueOps[adapterItemsValue]{Drop: func(v *adapterItemsValue, c rustfmt.Context) {
		r.event("item:%d:%t", v.id, c.Panicking())
		if int(v.id) == r.spec.Request.ItemFail {
			r.raise("item_drop", c)
		}
	}})
}
func (r *adapterItemsRunner) dropInput(input *adapterItemsInput, c rustfmt.Context) {
	r.event("input:%d:%d:%t", input.kind, input.next, c.Panicking())
	if r.spec.Request.InputFail {
		r.raise("input_drop", c)
	}
}
func (r *adapterItemsRunner) newInput(length int, kind uint64) *rustvec.Owned[adapterItemsInput] {
	return rustvec.NewOwned(adapterItemsInput{length: length, kind: kind}, rustvec.ValueOps[adapterItemsInput]{Drop: r.dropInput})
}
func (r *adapterItemsRunner) next(input *adapterItemsInput, c rustfmt.Context) *rustvec.Owned[adapterItemsValue] {
	n := input.next
	input.next++
	r.event("next:%d:%d:%t", input.kind, n, c.Panicking())
	if n == r.spec.Request.NextFail {
		r.raise("next", c)
	}
	if n < input.length {
		return r.value(input.kind*1000 + uint64(n))
	}
	return nil
}
func adapterItemsRemaining(input *adapterItemsInput) uint {
	if input.next >= input.length {
		return 0
	}
	return uint(input.length - input.next)
}
func (r *adapterItemsRunner) inputOps() UnordIteratorOps[adapterItemsInput, adapterItemsValue] {
	return UnordIteratorOps[adapterItemsInput, adapterItemsValue]{Next: func(ref rustvec.Ref[adapterItemsInput]) *rustvec.Owned[adapterItemsValue] {
		return r.next(ref.Value, ref.Context)
	}, SizeHint: func(ref rustvec.Ref[adapterItemsInput]) rustvec.Hint {
		n := adapterItemsRemaining(ref.Value)
		return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
	}, Count: func(owner *rustvec.Owned[adapterItemsInput], c rustfmt.Context) uint {
		r.event("count:%d:%t", owner.Borrow().kind, c.Panicking())
		return adapterItemsRemaining(owner.Borrow())
	}}
}
func (r *adapterItemsRunner) function() *rustvec.Owned[adapterItemsFunction] {
	return rustvec.NewOwned(adapterItemsFunction{}, rustvec.ValueOps[adapterItemsFunction]{Drop: func(f *adapterItemsFunction, c rustfmt.Context) {
		defer adapterItemsCleanup(c, func(c rustfmt.Context) {
			r.event("token:%t", c.Panicking())
			if r.spec.Request.TokenFail {
				r.raise("token_drop", c)
			}
		})
		r.event("function:%d:%t", f.calls, c.Panicking())
		if r.spec.Request.EnvFail {
			r.raise("function_drop", c)
		}
	}})
}
func (r *adapterItemsRunner) observe(f *adapterItemsFunction, value *adapterItemsValue, label string, c rustfmt.Context) bool {
	n := f.calls
	f.calls++
	r.event("%s:%d:%d:%t", label, n, value.id, c.Panicking())
	if n == r.spec.Request.CallFail {
		r.raise("call", c)
	}
	return value.id%2 == r.spec.Request.Keep
}
func (r *adapterItemsRunner) predicate() UnordFunction[rustvec.Ref[adapterItemsValue], bool] {
	invoke := func(f *adapterItemsFunction, value *rustvec.Owned[rustvec.Ref[adapterItemsValue]], label string, c rustfmt.Context) *rustvec.Owned[bool] {
		return rustvec.Own(r.observe(f, value.Borrow().Value, label, c))
	}
	return NewUnordFunction(r.function(), UnordFunctionOps[adapterItemsFunction, rustvec.Ref[adapterItemsValue], bool]{Call: func(ref rustvec.Ref[adapterItemsFunction], value *rustvec.Owned[rustvec.Ref[adapterItemsValue]]) *rustvec.Owned[bool] {
		return invoke(ref.Value, value, "shared", ref.Context)
	}, CallMut: func(ref rustvec.Ref[adapterItemsFunction], value *rustvec.Owned[rustvec.Ref[adapterItemsValue]]) *rustvec.Owned[bool] {
		return invoke(ref.Value, value, "mut", ref.Context)
	}, CallOnce: func(owner *rustvec.Owned[adapterItemsFunction], value *rustvec.Owned[rustvec.Ref[adapterItemsValue]], c rustfmt.Context) *rustvec.Owned[bool] {
		return invoke(owner.Borrow(), value, "once", c)
	}})
}
func (r *adapterItemsRunner) optional() UnordFunction[adapterItemsValue, UnordOptional[adapterItemsValue]] {
	invoke := func(f *adapterItemsFunction, value *rustvec.Owned[adapterItemsValue], label string, c rustfmt.Context) *rustvec.Owned[UnordOptional[adapterItemsValue]] {
		var some *rustvec.Owned[adapterItemsValue]
		if r.observe(f, value.Borrow(), label, c) {
			some = r.value(value.Borrow().id + 1000)
		}
		return rustvec.NewOwned(UnordOptional[adapterItemsValue]{Some: some}, rustvec.ValueOps[UnordOptional[adapterItemsValue]]{Drop: func(value *UnordOptional[adapterItemsValue], c rustfmt.Context) {
			if value.Some != nil {
				value.Some.DropWithContext(c)
			}
		}})
	}
	return NewUnordFunction(r.function(), UnordFunctionOps[adapterItemsFunction, adapterItemsValue, UnordOptional[adapterItemsValue]]{Call: func(ref rustvec.Ref[adapterItemsFunction], value *rustvec.Owned[adapterItemsValue]) *rustvec.Owned[UnordOptional[adapterItemsValue]] {
		return invoke(ref.Value, value, "shared", ref.Context)
	}, CallMut: func(ref rustvec.Ref[adapterItemsFunction], value *rustvec.Owned[adapterItemsValue]) *rustvec.Owned[UnordOptional[adapterItemsValue]] {
		return invoke(ref.Value, value, "mut", ref.Context)
	}, CallOnce: func(owner *rustvec.Owned[adapterItemsFunction], value *rustvec.Owned[adapterItemsValue], c rustfmt.Context) *rustvec.Owned[UnordOptional[adapterItemsValue]] {
		return invoke(owner.Borrow(), value, "once", c)
	}})
}
func (r *adapterItemsRunner) flatFunction() UnordFunction[adapterItemsValue, *UnordItems[adapterItemsValue]] {
	invoke := func(f *adapterItemsFunction, value *rustvec.Owned[adapterItemsValue], label string, c rustfmt.Context) *rustvec.Owned[*UnordItems[adapterItemsValue]] {
		r.observe(f, value.Borrow(), label, c)
		items := NewUnordItems(r.newInput(r.spec.Request.Inner, 100+value.Borrow().id), r.inputOps())
		return rustvec.NewOwned(items, rustvec.ValueOps[*UnordItems[adapterItemsValue]]{Drop: func(items **UnordItems[adapterItemsValue], c rustfmt.Context) { (*items).Close(c) }})
	}
	return NewUnordFunction(r.function(), UnordFunctionOps[adapterItemsFunction, adapterItemsValue, *UnordItems[adapterItemsValue]]{Call: func(ref rustvec.Ref[adapterItemsFunction], value *rustvec.Owned[adapterItemsValue]) *rustvec.Owned[*UnordItems[adapterItemsValue]] {
		return invoke(ref.Value, value, "shared", ref.Context)
	}, CallMut: func(ref rustvec.Ref[adapterItemsFunction], value *rustvec.Owned[adapterItemsValue]) *rustvec.Owned[*UnordItems[adapterItemsValue]] {
		return invoke(ref.Value, value, "mut", ref.Context)
	}, CallOnce: func(owner *rustvec.Owned[adapterItemsFunction], value *rustvec.Owned[adapterItemsValue], c rustfmt.Context) *rustvec.Owned[*UnordItems[adapterItemsValue]] {
		return invoke(owner.Borrow(), value, "once", c)
	}})
}
func adapterItemsNewMap[A, B any](input *rustvec.Owned[adapterItemsInput], function *rustvec.Owned[UnordFunction[A, B]], c rustfmt.Context) *rustvec.Owned[adapterItemsMap[A, B]] {
	return rustvec.NewOwned(adapterItemsMap[A, B]{input: input.MoveWithContext(c), function: function.Take()}, rustvec.ValueOps[adapterItemsMap[A, B]]{Drop: func(state *adapterItemsMap[A, B], c rustfmt.Context) {
		defer adapterItemsCleanup(c, state.function.Close)
		state.input.DropWithContext(c)
	}})
}
func adapterItemsFold[A, B any](r *adapterItemsRunner, mapped *rustvec.Owned[adapterItemsMap[A, B]], step func(*UnordFunction[A, B], *rustvec.Owned[adapterItemsValue], rustfmt.Context) uint, c rustfmt.Context) uint {
	state := mapped.Take()
	function := unordFunctionValue(state.function, c)
	return rustvec.ConsumeValue(state.input, c, func(input *rustvec.Owned[adapterItemsInput]) uint {
		return rustvec.ConsumeValue(function, c, func(function *rustvec.Owned[UnordFunction[A, B]]) uint {
			r.event("fold:%d:%t", input.Borrow().kind, c.Panicking())
			count := uint(0)
			for {
				value := r.next(input.Borrow(), c)
				if value == nil {
					return count
				}
				count += step(function.Borrow(), value, c)
			}
		})
	})
}
func (r *adapterItemsRunner) filterConstructor() UnordIteratorConstructor[adapterItemsInput, adapterItemsValue, UnordFunction[rustvec.Ref[adapterItemsValue], bool], adapterItemsMap[rustvec.Ref[adapterItemsValue], bool], adapterItemsValue] {
	return UnordIteratorConstructor[adapterItemsInput, adapterItemsValue, UnordFunction[rustvec.Ref[adapterItemsValue], bool], adapterItemsMap[rustvec.Ref[adapterItemsValue], bool], adapterItemsValue]{Construct: func(input *rustvec.Owned[adapterItemsInput], function *rustvec.Owned[UnordFunction[rustvec.Ref[adapterItemsValue], bool]], c rustfmt.Context) *rustvec.Owned[adapterItemsMap[rustvec.Ref[adapterItemsValue], bool]] {
		return adapterItemsNewMap(input, function, c)
	}, Result: UnordIteratorOps[adapterItemsMap[rustvec.Ref[adapterItemsValue], bool], adapterItemsValue]{
		Next: func(ref rustvec.Ref[adapterItemsMap[rustvec.Ref[adapterItemsValue], bool]]) *rustvec.Owned[adapterItemsValue] {
			for {
				value := r.next(ref.Value.input.Borrow(), ref.Context)
				if value == nil {
					return nil
				}
				kept := rustvec.ConsumeValue(value, ref.Context, func(value *rustvec.Owned[adapterItemsValue]) *rustvec.Owned[adapterItemsValue] {
					keep := ref.Value.function.CallMut(rustvec.Own(rustvec.Ref[adapterItemsValue]{Value: value.Borrow(), Context: ref.Context}), ref.Context).Take()
					if keep {
						return value.MoveWithContext(ref.Context)
					}
					return nil
				})
				if kept != nil {
					return kept
				}
			}
		},
		SizeHint: func(ref rustvec.Ref[adapterItemsMap[rustvec.Ref[adapterItemsValue], bool]]) rustvec.Hint {
			return rustvec.Hint{Upper: adapterItemsRemaining(ref.Value.input.Borrow()), HasUpper: true}
		},
		Count: func(owner *rustvec.Owned[adapterItemsMap[rustvec.Ref[adapterItemsValue], bool]], c rustfmt.Context) uint {
			return adapterItemsFold(r, owner, func(f *UnordFunction[rustvec.Ref[adapterItemsValue], bool], value *rustvec.Owned[adapterItemsValue], c rustfmt.Context) uint {
				return rustvec.ConsumeValue(value, c, func(value *rustvec.Owned[adapterItemsValue]) uint {
					if f.CallMut(rustvec.Own(rustvec.Ref[adapterItemsValue]{Value: value.Borrow(), Context: c}), c).Take() {
						return 1
					}
					return 0
				})
			}, c)
		},
	}}
}
func (r *adapterItemsRunner) optionalConstructor() UnordIteratorConstructor[adapterItemsInput, adapterItemsValue, UnordFunction[adapterItemsValue, UnordOptional[adapterItemsValue]], adapterItemsMap[adapterItemsValue, UnordOptional[adapterItemsValue]], adapterItemsValue] {
	return UnordIteratorConstructor[adapterItemsInput, adapterItemsValue, UnordFunction[adapterItemsValue, UnordOptional[adapterItemsValue]], adapterItemsMap[adapterItemsValue, UnordOptional[adapterItemsValue]], adapterItemsValue]{Construct: func(input *rustvec.Owned[adapterItemsInput], function *rustvec.Owned[UnordFunction[adapterItemsValue, UnordOptional[adapterItemsValue]]], c rustfmt.Context) *rustvec.Owned[adapterItemsMap[adapterItemsValue, UnordOptional[adapterItemsValue]]] {
		return adapterItemsNewMap(input, function, c)
	}, Result: UnordIteratorOps[adapterItemsMap[adapterItemsValue, UnordOptional[adapterItemsValue]], adapterItemsValue]{
		Next: func(ref rustvec.Ref[adapterItemsMap[adapterItemsValue, UnordOptional[adapterItemsValue]]]) *rustvec.Owned[adapterItemsValue] {
			for {
				value := r.next(ref.Value.input.Borrow(), ref.Context)
				if value == nil {
					return nil
				}
				optional := ref.Value.function.CallMut(value, ref.Context).Take()
				if optional.Some != nil {
					return optional.Some
				}
			}
		},
		SizeHint: func(ref rustvec.Ref[adapterItemsMap[adapterItemsValue, UnordOptional[adapterItemsValue]]]) rustvec.Hint {
			return rustvec.Hint{Upper: adapterItemsRemaining(ref.Value.input.Borrow()), HasUpper: true}
		},
		Count: func(owner *rustvec.Owned[adapterItemsMap[adapterItemsValue, UnordOptional[adapterItemsValue]]], c rustfmt.Context) uint {
			return adapterItemsFold(r, owner, func(f *UnordFunction[adapterItemsValue, UnordOptional[adapterItemsValue]], value *rustvec.Owned[adapterItemsValue], c rustfmt.Context) uint {
				return rustvec.ConsumeValue(f.CallMut(value, c), c, func(optional *rustvec.Owned[UnordOptional[adapterItemsValue]]) uint {
					if optional.Borrow().Some != nil {
						return 1
					}
					return 0
				})
			}, c)
		},
	}}
}
func (r *adapterItemsRunner) flatConstructor() UnordIteratorConstructor[adapterItemsInput, adapterItemsValue, UnordFunction[adapterItemsValue, adapterItemsInput], adapterItemsFlat, adapterItemsValue] {
	return UnordIteratorConstructor[adapterItemsInput, adapterItemsValue, UnordFunction[adapterItemsValue, adapterItemsInput], adapterItemsFlat, adapterItemsValue]{Construct: func(input *rustvec.Owned[adapterItemsInput], function *rustvec.Owned[UnordFunction[adapterItemsValue, adapterItemsInput]], c rustfmt.Context) *rustvec.Owned[adapterItemsFlat] {
		return rustvec.NewOwned(adapterItemsFlat{mapped: adapterItemsNewMap(input, function, c)}, rustvec.ValueOps[adapterItemsFlat]{Drop: func(state *adapterItemsFlat, c rustfmt.Context) {
			defer adapterItemsCleanup(c, func(c rustfmt.Context) {
				if state.front != nil {
					state.front.DropWithContext(c)
				}
			})
			if state.mapped != nil {
				state.mapped.DropWithContext(c)
			}
		}})
	}, Result: UnordIteratorOps[adapterItemsFlat, adapterItemsValue]{
		Next: func(ref rustvec.Ref[adapterItemsFlat]) *rustvec.Owned[adapterItemsValue] {
			state, c := ref.Value, ref.Context
			for {
				if state.front != nil {
					if value := r.next(state.front.Borrow(), c); value != nil {
						return value
					}
					front := state.front
					state.front = nil
					front.DropWithContext(c)
				}
				if state.mapped == nil {
					return nil
				}
				mapped := state.mapped.Borrow()
				value := r.next(mapped.input.Borrow(), c)
				if value == nil {
					owner := state.mapped
					state.mapped = nil
					owner.DropWithContext(c)
					return nil
				}
				state.front = mapped.function.CallMut(value, c)
			}
		},
		SizeHint: func(ref rustvec.Ref[adapterItemsFlat]) rustvec.Hint {
			lower := uint(0)
			if ref.Value.front != nil {
				lower = adapterItemsRemaining(ref.Value.front.Borrow())
			}
			upper := ref.Value.mapped == nil || adapterItemsRemaining(ref.Value.mapped.Borrow().input.Borrow()) == 0
			return rustvec.Hint{Lower: lower, Upper: lower, HasUpper: upper}
		},
		Count: func(owner *rustvec.Owned[adapterItemsFlat], c rustfmt.Context) uint {
			state := owner.Take()
			if state.front != nil {
				c.Panic("adapters proof: Count requires its actual fresh UnordItems state")
			}
			if state.mapped == nil {
				return 0
			}
			return adapterItemsFold(r, state.mapped, func(f *UnordFunction[adapterItemsValue, adapterItemsInput], value *rustvec.Owned[adapterItemsValue], c rustfmt.Context) uint {
				return rustvec.ConsumeValue(f.CallMut(value, c), c, func(input *rustvec.Owned[adapterItemsInput]) uint { return r.inputOps().Count(input, c) })
			}, c)
		},
	}}
}
func (r *adapterItemsRunner) chainConstructor() UnordChainConstructor[adapterItemsInput, adapterItemsInput, adapterItemsValue, adapterItemsChain] {
	return UnordChainConstructor[adapterItemsInput, adapterItemsInput, adapterItemsValue, adapterItemsChain]{Construct: func(left, right *rustvec.Owned[adapterItemsInput], c rustfmt.Context) *rustvec.Owned[adapterItemsChain] {
		return rustvec.NewOwned(adapterItemsChain{left: left.MoveWithContext(c), right: right.MoveWithContext(c)}, rustvec.ValueOps[adapterItemsChain]{Drop: func(state *adapterItemsChain, c rustfmt.Context) {
			defer adapterItemsCleanup(c, func(c rustfmt.Context) {
				if state.right != nil {
					state.right.DropWithContext(c)
				}
			})
			if state.left != nil {
				state.left.DropWithContext(c)
			}
		}})
	}, Result: UnordIteratorOps[adapterItemsChain, adapterItemsValue]{
		Next: func(ref rustvec.Ref[adapterItemsChain]) *rustvec.Owned[adapterItemsValue] {
			state, c := ref.Value, ref.Context
			if state.left != nil {
				if value := r.next(state.left.Borrow(), c); value != nil {
					return value
				}
				left := state.left
				state.left = nil
				left.DropWithContext(c)
			}
			if state.right == nil {
				return nil
			}
			return r.next(state.right.Borrow(), c)
		},
		SizeHint: func(ref rustvec.Ref[adapterItemsChain]) rustvec.Hint {
			n := uint(0)
			if ref.Value.left != nil {
				n += adapterItemsRemaining(ref.Value.left.Borrow())
			}
			if ref.Value.right != nil {
				n += adapterItemsRemaining(ref.Value.right.Borrow())
			}
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		},
		Count: func(owner *rustvec.Owned[adapterItemsChain], c rustfmt.Context) uint {
			state := owner.Take()
			return rustvec.ConsumeValue(state.right, c, func(right *rustvec.Owned[adapterItemsInput]) uint {
				leftCount := rustvec.ConsumeValue(state.left, c, func(left *rustvec.Owned[adapterItemsInput]) uint { return r.inputOps().Count(left, c) })
				rightCount := r.inputOps().Count(right, c)
				right.DropWithContext(c)
				return leftCount + rightCount
			})
		},
	}}
}
func (r *adapterItemsRunner) display(items *UnordItems[adapterItemsValue], c rustfmt.Context) {
	if r.spec.Family%2 == 1 {
		r.event("returned:%d", items.Count(c))
		return
	}
	value, _ := items.GetOnly(c)
	label := "none"
	if value != nil {
		label = strconv.FormatUint(value.Borrow().id, 10)
	}
	r.event("returned:%s", label)
	if value != nil {
		value.DropWithContext(c)
	}
}
func (r *adapterItemsRunner) execute(c rustfmt.Context) {
	if r.spec.Family >= 8 {
		r.override(c)
		return
	}
	input := NewUnordItems(r.newInput(r.spec.Request.Length, 0), r.inputOps())
	switch r.spec.Family {
	case 0, 1:
		r.display(FilterUnordItems(input, r.predicate(), r.filterConstructor(), c), c)
	case 2, 3:
		r.display(FilterMapUnordItems(input, r.optional(), r.optionalConstructor(), c), c)
	case 4, 5:
		r.display(FlatMapUnordItems(input, r.flatFunction(), r.flatConstructor(), c), c)
	case 6, 7:
		r.display(ChainUnordItems(input, NewUnordItems(r.newInput(r.spec.Request.Inner, 1), r.inputOps()), r.chainConstructor(), c), c)
	}
}
func (r *adapterItemsRunner) override(c rustfmt.Context) {
	state := rustvec.NewOwned(adapterItemsOverride{input: adapterItemsInput{length: r.spec.Request.Length, kind: 0}}, rustvec.ValueOps[adapterItemsOverride]{Drop: func(state *adapterItemsOverride, c rustfmt.Context) { r.dropInput(&state.input, c) }})
	items := NewUnordItems(state, UnordIteratorOps[adapterItemsOverride, adapterItemsValue]{Next: func(ref rustvec.Ref[adapterItemsOverride]) *rustvec.Owned[adapterItemsValue] {
		return r.next(&ref.Value.input, ref.Context)
	}, SizeHint: func(rustvec.Ref[adapterItemsOverride]) rustvec.Hint { return rustvec.Hint{} }, Count: func(owner *rustvec.Owned[adapterItemsOverride], c rustfmt.Context) uint {
		count := uint(0)
		for {
			value := r.next(&owner.Borrow().input, c)
			if value == nil {
				return count
			}
			value.DropWithContext(c)
			count++
		}
	}})
	switch r.spec.Family {
	case 8:
		selected := UnordIteratorConstructor[adapterItemsOverride, adapterItemsValue, UnordFunction[rustvec.Ref[adapterItemsValue], bool], struct{}, adapterItemsValue]{Construct: func(input *rustvec.Owned[adapterItemsOverride], function *rustvec.Owned[UnordFunction[rustvec.Ref[adapterItemsValue], bool]], c rustfmt.Context) *rustvec.Owned[struct{}] {
			r.event("filter_override")
			rustvec.ConsumeValue(r.value(99), c, func(value *rustvec.Owned[adapterItemsValue]) struct{} {
				result := function.Take().CallOnce(rustvec.Own(rustvec.Ref[adapterItemsValue]{Value: value.Borrow(), Context: c}), c).Take()
				r.event("constructed:%t", result)
				value.DropWithContext(c)
				r.raise("filter_constructor", c)
				return struct{}{}
			})
			return nil
		}}
		FilterUnordItems(items, r.predicate(), selected, c)
	case 9:
		selected := UnordIteratorConstructor[adapterItemsOverride, adapterItemsValue, UnordFunction[adapterItemsValue, UnordOptional[adapterItemsValue]], struct{}, adapterItemsValue]{Construct: func(input *rustvec.Owned[adapterItemsOverride], function *rustvec.Owned[UnordFunction[adapterItemsValue, UnordOptional[adapterItemsValue]]], c rustfmt.Context) *rustvec.Owned[struct{}] {
			r.event("filter_map_override")
			result := function.Take().CallOnce(r.value(99), c)
			r.event("constructed")
			result.DropWithContext(c)
			r.raise("filter_map_constructor", c)
			return nil
		}}
		FilterMapUnordItems(items, r.optional(), selected, c)
	case 10:
		selected := UnordChainConstructor[adapterItemsOverride, adapterItemsInput, adapterItemsValue, struct{}]{Construct: func(left *rustvec.Owned[adapterItemsOverride], right *rustvec.Owned[adapterItemsInput], c rustfmt.Context) *rustvec.Owned[struct{}] {
			r.event("chain_override")
			right.DropWithContext(c)
			r.raise("chain_constructor", c)
			return nil
		}}
		ChainUnordItems(items, NewUnordItems(r.newInput(r.spec.Request.Inner, 1), r.inputOps()), selected, c)
	}
}
func (r *adapterItemsRunner) body(c rustfmt.Context) {
	r.event("body:%t", c.Panicking())
	message := ""
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		r.execute(c)
	}()
	r.event("caught:%s:%t", hex.EncodeToString([]byte(message)), c.Panicking())
}
func (r *adapterItemsRunner) run() {
	cleanupLocation := r.locations["panic in a destructor during cleanup"]
	cleanup := rustfmt.SourcePanicInfo{Payload: "panic in a destructor during cleanup", Kind: rustfmt.PanicStaticStr, Location: &cleanupLocation, CanUnwind: false, ForceNoBacktrace: true}
	c := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{CleanupAbort: &cleanup, Hook: func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
		location := rustfmt.SourceLocation{}
		if info.Location != nil {
			location = *info.Location
		}
		r.event("HOOK:%s:%s:%t:%t:%t:%s:%d:%d", info.Kind, hex.EncodeToString([]byte(fmt.Sprint(info.Payload))), c.Panicking(), info.CanUnwind, info.ForceNoBacktrace, hex.EncodeToString([]byte(location.File)), location.Line, location.Column)
		if r.spec.Mode == "processing" {
			r.body(c)
		}
	}})
	if r.spec.Mode == "normal" {
		r.body(c)
		return
	}
	func() {
		defer func() {
			if recover() != nil && r.spec.Mode == "unwind" {
				unwind := c
				unwind.Unwinding = true
				r.body(unwind)
			}
		}()
		location := r.locations["outer."+r.spec.Mode]
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: &location, CanUnwind: true})
	}()
	r.event("outer_end:false")
}
func adapterItemsDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func loadAdapterItemsManifest(t *testing.T) adapterItemsManifest {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/adapters/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata adapterItemsManifest
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			if err != nil || adapterItemsDigest(data) != want {
				t.Fatalf("adapters binding %s %v", path, err)
			}
		}
	}
	return metadata
}
func TestOriginalUnordItemsAdapter(t *testing.T) {
	metadata := loadAdapterItemsManifest(t)
	data, err := os.ReadFile("testdata/unord_items_adapter.jsonl.gz")
	if err != nil || adapterItemsDigest(data) != metadata.Fixture {
		t.Fatal("adapters fixture", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 16<<20))
	if err != nil {
		t.Fatal(err)
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if adapterItemsDigest(raw) != metadata.Raw {
		t.Fatal("adapters raw")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, aborts := 0, 0
	for scanner.Scan() {
		var spec adapterItemsCase
		if err = json.Unmarshal(scanner.Bytes(), &spec); err != nil {
			t.Fatal(err)
		}
		got := ""
		if spec.Outcome == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestUnordItemsAdapterChild$")
			command.Env = append(os.Environ(), "FERRUGO_UNORD_ITEMS_ADAPTER="+scanner.Text(), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
			output, err := command.Output()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("adapters abort%d %v %s", count, err, output)
			}
			got = string(output)
		} else {
			var output strings.Builder
			runner := adapterItemsRunner{spec: spec, locations: metadata.Locations, out: &output}
			runner.run()
			got = output.String()
		}
		if got != spec.Stdout {
			t.Fatalf("adapters case%d %+v\nGOT\n%sWANT\n%s", count, spec, got, spec.Stdout)
		}
		count++
	}
	if scanner.Err() != nil || count != 1650 || metadata.Cases != 1650 || aborts != metadata.Aborts {
		t.Fatal("adapters coverage", count, aborts, scanner.Err())
	}
}
func TestUnordItemsAdapterChild(t *testing.T) {
	data := os.Getenv("FERRUGO_UNORD_ITEMS_ADAPTER")
	if data == "" {
		return
	}
	metadata := loadAdapterItemsManifest(t)
	var spec adapterItemsCase
	if err := json.Unmarshal([]byte(data), &spec); err != nil {
		t.Fatal(err)
	}
	runner := adapterItemsRunner{spec: spec, locations: metadata.Locations, out: os.Stdout}
	runner.run()
	os.Exit(0)
}
