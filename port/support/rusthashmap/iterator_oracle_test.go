// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type extendState[T any] struct {
	values      []T
	front, back int
}

func extendDropValues[T any](state *extendState[T], drop func(*T, Context), c Context) {
	for state.front < state.back {
		index := state.front
		state.front++
		func() {
			defer func() {
				if p := recover(); p != nil {
					unwind := c
					unwind.Unwinding = true
					cleanupOnPanic(unwind, func() { extendDropValues(state, drop, unwind) })
					panic(p)
				}
			}()
			drop(&state.values[index], c)
		}()
	}
}
func extendIterator[T any](values []T, drop func(*T, Context), mode int, skip bool, control *auditControl) rustvec.IntoIterator[T] {
	c := Context{}
	var iterator rustvec.Iterator[T]
	if mode == 5 {
		builtin := rustvec.NewVecIntoIter(values, rustvec.ElementOps[T]{Drop: drop})
		if skip {
			value, ok := builtin.Next()
			if ok {
				drop(&value, c)
			}
		}
		iterator = builtin.IntoIterator().IntoIter(c)
	} else {
		state := extendState[T]{values: values, back: len(values)}
		next := func(ref rustvec.Ref[extendState[T]]) (T, bool) {
			s := ref.Value
			control.event("next", uint64(s.back-s.front), ref.Context)
			var zero T
			if s.front == s.back {
				return zero, false
			}
			value := s.values[s.front]
			s.values[s.front] = zero
			s.front++
			return value, true
		}
		if skip {
			value, ok := next(rustvec.Ref[extendState[T]]{Value: &state})
			if ok {
				drop(&value, c)
			}
		}
		ops := rustvec.IteratorOps[extendState[T], T]{Next: next, SizeHint: func(ref rustvec.Ref[extendState[T]]) rustvec.Hint {
			n := uint(ref.Value.back - ref.Value.front)
			control.event("hint", uint64(n), ref.Context)
			return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
		}, Drop: func(s *extendState[T], ctx Context) {
			fieldsStarted := false
			defer func() {
				if p := recover(); p != nil {
					if !fieldsStarted {
						unwind := ctx
						unwind.Unwinding = true
						cleanupOnPanic(unwind, func() { extendDropValues(s, drop, unwind) })
					}
					panic(p)
				}
			}()
			control.event("iterdrop", uint64(s.back-s.front), ctx)
			fieldsStarted = true
			extendDropValues(s, drop, ctx)
		}}
		if mode == 1 || mode == 3 {
			ops.ForEach = func(arg *rustvec.Owned[extendState[T]], consume func(T)) {
				s := arg.Borrow()
				control.event("foreach", 0, arg.Context())
				for s.front < s.back {
					s.back--
					value := s.values[s.back]
					var zero T
					s.values[s.back] = zero
					consume(value)
				}
			}
		}
		if mode == 2 || mode == 3 {
			ops.FoldUnit = func(arg *rustvec.Owned[extendState[T]], consume func(T)) {
				s := arg.Borrow()
				control.event("fold", 0, arg.Context())
				for s.front < s.back {
					index := s.front
					s.front++
					value := s.values[index]
					var zero T
					s.values[index] = zero
					consume(value)
				}
			}
		}
		if mode == 4 {
			ops.MapToUnit = func(arg *rustvec.Owned[extendState[T]], mapping rustvec.UnitMapping[T]) *rustvec.Owned[extendState[T]] {
				ctx := mapping.Context()
				control.event("map_ctor", 0, ctx)
				value, ok := next(rustvec.Ref[extendState[T]]{Value: arg.Borrow(), Context: ctx})
				if ok {
					mapping.Apply(mapping.Own(value)).DropWithContext(ctx)
				}
				ctx.Panic("map_ctor")
				return nil
			}
		}
		iterator = rustvec.NewIterator(state, ops)
	}
	type inputState struct{ iterator rustvec.Iterator[T] }
	return rustvec.NewIntoIterator(inputState{iterator}, rustvec.IntoIteratorOps[inputState, T]{IntoIter: func(arg *rustvec.Owned[inputState]) rustvec.Iterator[T] {
		control.event("into", 0, arg.Context())
		s := arg.Borrow()
		result := s.iterator
		s.iterator = rustvec.Iterator[T]{}
		return result
	}, Drop: func(s *inputState, ctx Context) {
		fieldsStarted := false
		defer func() {
			if p := recover(); p != nil {
				if !fieldsStarted {
					unwind := ctx
					unwind.Unwinding = true
					cleanupOnPanic(unwind, func() { s.iterator.CloseWithContext(unwind) })
				}
				panic(p)
			}
		}()
		control.event("inputdrop", 0, ctx)
		fieldsStarted = true
		s.iterator.CloseWithContext(ctx)
	}})
}
func extendCatch(body func()) (result string) {
	result = "ok"
	defer func() {
		if p := recover(); p != nil {
			if n, ok := p.(auditPanic); ok {
				result = fmt.Sprintf("event:%d", n)
			} else if p == "map_ctor" {
				result = "ctor"
			} else {
				panic(p)
			}
		}
	}()
	body()
	return
}
func extendDictionaries(control *auditControl) (KeyOps[auditKey], ValueOps[auditValue]) {
	keys := KeyOps[auditKey]{Hash: func(k *auditKey, h Hasher, c Context) { control.event("hash", k.token, c); HashU64(h, k.slot) }, Equal: func(a, b *auditKey, c Context) bool {
		control.event("eq", a.token*1000+b.token, c)
		return a.slot == b.slot
	}, Drop: func(k *auditKey, c Context) { control.event("kdrop", k.token, c) }}
	values := ValueOps[auditValue]{Drop: func(v *auditValue, c Context) { control.event("vdrop", v.token, c) }}
	return keys, values
}
func extendMapDropPair(pair *Pair[auditKey, auditValue], keys KeyOps[auditKey], values ValueOps[auditValue], c Context) {
	started := false
	defer func() {
		if p := recover(); p != nil {
			if !started {
				unwind := c
				unwind.Unwinding = true
				cleanupOnPanic(unwind, func() { values.Drop(&pair.Value, unwind) })
			}
			panic(p)
		}
	}()
	keys.Drop(&pair.Key, c)
	started = true
	values.Drop(&pair.Value, c)
}
func extendObserve(kind string, mode int, from bool, n int, skip bool, control *auditControl) (action, final, state string) {
	c := Context{}
	keys, values := extendDictionaries(control)
	if kind == "map" {
		items := make([]Pair[auditKey, auditValue], n)
		ids := [4]uint64{1, 2, 1, 3}
		for i := range items {
			items[i] = Pair[auditKey, auditValue]{auditKey{ids[i%4], uint64(i) + 11}, auditValue{uint64(i) + 101}}
		}
		input := extendIterator(items, func(pair *Pair[auditKey, auditValue], ctx Context) { extendMapDropPair(pair, keys, values, ctx) }, mode, skip, control)
		var m *Map[auditKey, auditValue]
		if !from {
			m = NewMap[auditKey, auditValue](Layout{16, 8}, DefaultProfile(), keys, values, FxBuilder{64})
			m.Insert(auditKey{99, 99}, auditValue{199}, c)
		}
		control.enabled = true
		control.counter = 0
		control.events = nil
		action = extendCatch(func() {
			if from {
				m = MapFromIterator(input, Layout{16, 8}, DefaultProfile(), keys, values, func(Context) Builder { return FxBuilder{64} }, c)
			} else {
				m.Extend(input, c)
			}
		})
		state = "none"
		if m != nil {
			var out strings.Builder
			fmt.Fprintf(&out, "%d:", m.Len())
			i := m.Iter(c)
			for {
				pair, ok := i.Next(c)
				if !ok {
					break
				}
				fmt.Fprintf(&out, "%d/%d/%d,", pair.Key.slot, pair.Key.token, pair.Value.token)
			}
			state = out.String()
		}
		control.mark("final")
		final = extendCatch(func() {
			if m != nil {
				old := m
				m = nil
				old.Close(c)
			}
		})
	} else {
		items := make([]auditKey, n)
		ids := [4]uint64{1, 2, 1, 3}
		for i := range items {
			items[i] = auditKey{ids[i%4], uint64(i) + 11}
		}
		input := extendIterator(items, keys.Drop, mode, skip, control)
		var set *Set[auditKey]
		if !from {
			set = NewSet(Layout{16, 8}, DefaultProfile(), keys, FxBuilder{64})
			set.Insert(auditKey{99, 99}, c)
		}
		control.enabled = true
		control.counter = 0
		control.events = nil
		action = extendCatch(func() {
			if from {
				set = SetFromIterator(input, Layout{16, 8}, DefaultProfile(), keys, func(Context) Builder { return FxBuilder{64} }, c)
			} else {
				set.Extend(input, c)
			}
		})
		state = "none"
		if set != nil {
			var out strings.Builder
			fmt.Fprintf(&out, "%d:", set.Len())
			i := set.Iter(c)
			for {
				key, ok := i.Next(c)
				if !ok {
					break
				}
				fmt.Fprintf(&out, "%d/%d,", key.slot, key.token)
			}
			state = out.String()
		}
		control.mark("final")
		final = extendCatch(func() {
			if set != nil {
				old := set
				set = nil
				old.Close(c)
			}
		})
	}
	return
}
func TestOriginalMapSetIteratorDispatch(t *testing.T) {
	metadata, err := os.ReadFile("../../../tools/port-rust-unord/table/iterator_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases     int    `json:"cases"`
		Raw       string `json:"raw_sha256"`
		Fixture   string `json:"fixture_sha256"`
		Oracle    string `json:"oracle_sha256"`
		Generator string `json:"generator_sha256"`
		Support   string `json:"support_generator_sha256"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{"../../../tools/port-rust-unord/table/iterator_oracle.rs": manifest.Oracle, "../../../tools/port-rust-unord/table/iterator_generate.py": manifest.Generator, "../../../tools/port-rust-unord/table/generate.py": manifest.Support} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("iterator reference binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/iterator.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if digest(compressed) != manifest.Fixture || manifest.Cases != 1494 {
		t.Fatal("iterator fixture/coverage differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(reader, 16<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("iterator reference body differs:%v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	cases := 0
	for scanner.Scan() {
		cases++
		f := strings.Split(scanner.Text(), "\t")
		if len(f) != 11 {
			t.Fatalf("malformed iterator row%d", cases)
		}
		mode, _ := strconv.Atoi(f[1])
		from, _ := strconv.ParseBool(f[2])
		n, _ := strconv.Atoi(f[3])
		skip, _ := strconv.ParseBool(f[4])
		fail, _ := strconv.Atoi(f[5])
		counter, _ := strconv.Atoi(f[8])
		wantState, _ := hex.DecodeString(f[9])
		wantEvents, _ := hex.DecodeString(f[10])
		control := &auditControl{fail: fail, second: -1}
		action, final, state := extendObserve(f[0], mode, from, n, skip, control)
		if action != f[6] || final != f[7] || control.counter != counter || state != string(wantState) || strings.Join(control.events, "|") != string(wantEvents) {
			t.Fatalf("iterator row%d (%s):\ngot%s/%s/%d/%s\n%s\nwant%s/%s/%d/%s\n%s", cases, strings.Join(f[:6], "\t"), action, final, control.counter, state, strings.Join(control.events, "|"), f[6], f[7], counter, wantState, wantEvents)
		}
	}
	if err := scanner.Err(); err != nil || cases != manifest.Cases {
		t.Fatalf("iterator proof incomplete:%d:%v", cases, err)
	}
}
