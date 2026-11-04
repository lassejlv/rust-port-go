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

type auditControl struct {
	events         []string
	counter, fail  int
	fired, enabled bool
	second         int
	live           bool
}
type auditPanic int

func (a *auditControl) event(kind string, id uint64, c Context) {
	if !a.enabled {
		return
	}
	a.events = append(a.events, fmt.Sprintf("%s:%d:%t", kind, id, c.Panicking()))
	if a.live {
		fmt.Printf("HASHMAP_EVENT\t%s\n", hex.EncodeToString([]byte(a.events[len(a.events)-1])))
	}
	index := a.counter
	a.counter++
	if index == a.fail && !a.fired {
		a.fired = true
		c.Panic(auditPanic(index))
	}
	if index == a.second && a.fired {
		c.Panic(auditPanic(index))
	}
}
func (a *auditControl) mark(text string) {
	a.events = append(a.events, text)
	if a.live {
		fmt.Printf("HASHMAP_EVENT\t%s\n", hex.EncodeToString([]byte(text)))
	}
}

type auditKey struct{ slot, token uint64 }
type auditValue struct{ token uint64 }
type auditHasher struct {
	inner   *FxHasher
	control *auditControl
	context Context
}

func (h *auditHasher) Write(b []byte) {
	h.control.event("bytes", uint64(len(b)), h.context)
	h.inner.Write(b)
}
func (h *auditHasher) WriteU64(value uint64) {
	h.control.event("write", value, h.context)
	h.inner.WriteU64(value)
}
func (h *auditHasher) Finish64() uint64 {
	h.control.event("finish", 0, h.context)
	return h.inner.Finish64()
}

type auditBuilder struct {
	control *auditControl
	live    bool
}

func (b *auditBuilder) Build(c Context) *OwnedHasher {
	b.control.event("build", 0, c)
	h := &auditHasher{NewFxHasherProfile(64, 0), b.control, c}
	return NewOwnedHasher(h, func(_ Hasher, ctx Context) { b.control.event("hdrop", 0, ctx) })
}
func (b *auditBuilder) Clone(c Context) Builder {
	b.control.event("bclone", 0, c)
	return &auditBuilder{b.control, true}
}
func (b *auditBuilder) CloneFrom(source Builder, c Context) { b.control.event("bclonefrom", 0, c) }
func (b *auditBuilder) Close(c Context) {
	if b.live {
		b.live = false
		b.control.event("bdrop", 0, c)
	}
}
func auditMap(control *auditControl, capacity uint) *Map[auditKey, auditValue] {
	return auditMapContext(control, capacity, Context{})
}
func auditMapContext(control *auditControl, capacity uint, c Context) *Map[auditKey, auditValue] {
	keys := KeyOps[auditKey]{Hash: func(k *auditKey, h Hasher, c Context) { control.event("hash", k.token, c); HashU64(h, k.slot) }, Equal: func(a, b *auditKey, c Context) bool {
		control.event("eq", a.token*1000+b.token, c)
		return a.slot == b.slot
	}, Clone: func(k *auditKey, c Context) auditKey {
		control.event("kclone", k.token, c)
		return auditKey{k.slot, k.token + 100}
	}, Drop: func(k *auditKey, c Context) { control.event("kdrop", k.token, c) }}
	values := ValueOps[auditValue]{Clone: func(v *auditValue, c Context) auditValue {
		control.event("vclone", v.token, c)
		return auditValue{v.token + 100}
	}, Drop: func(v *auditValue, c Context) { control.event("vdrop", v.token, c) }}
	return NewMapWithCapacity(capacity, Layout{24, 8}, DefaultProfile(), keys, values, &auditBuilder{control, true}, c)
}
func auditInitial(control *auditControl, offset uint64) *Map[auditKey, auditValue] {
	m := auditMap(control, 0)
	for slot := uint64(1); slot <= 3; slot++ {
		m.Insert(auditKey{slot, slot + 10 + offset}, auditValue{slot + 20 + offset}, Context{})
	}
	return m
}
func auditCatch(body func()) (result string) {
	result = "ok"
	defer func() {
		if p := recover(); p != nil {
			if index, ok := p.(auditPanic); ok {
				result = fmt.Sprintf("event:%d", index)
			} else if text, ok := p.(string); ok && strings.Contains(strings.ToLower(text), "capacity") {
				result = "capacity"
			} else {
				panic(p)
			}
		}
	}()
	body()
	return
}
func auditSnapshot(m *Map[auditKey, auditValue]) string {
	if m == nil {
		return "none"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "%d:%d:", m.Len(), m.Capacity())
	i := m.Iter(Context{})
	for {
		pair, ok := i.Next(Context{})
		if !ok {
			break
		}
		fmt.Fprintf(&out, "%d/%d/%d,", pair.Key.slot, pair.Key.token, pair.Value.token)
	}
	return out.String()
}

type auditEnv struct{ token, calls uint64 }

func auditEnvOwned(control *auditControl) *rustvec.Owned[auditEnv] {
	return rustvec.NewOwned(auditEnv{90, 0}, rustvec.ValueOps[auditEnv]{Drop: func(env *auditEnv, c Context) { control.event("envdrop", env.token*1000+env.calls, c) }})
}

func auditWork(scenario int, control *auditControl, c Context) (action, final, state string) {
	var m, other *Map[auditKey, auditValue]
	if scenario != 24 {
		m = auditInitial(control, 0)
	}
	if scenario == 14 {
		other = auditInitial(control, 200)
	}
	defer func() {
		if other != nil {
			other.Close(c)
		}
		if m != nil {
			m.Close(c)
		}
	}()
	control.enabled = true
	control.counter = 0
	control.events = nil
	lookup := BorrowedLookup[auditKey, uint64]{Hash: func(q *uint64, h Hasher, c Context) { HashU64(h, *q) }, Equal: func(q *uint64, k *auditKey, c Context) bool { return *q == k.slot }}
	action = auditCatch(func() {
		switch scenario {
		case 0, 1:
			slot := uint64(4)
			if scenario == 1 {
				slot = 1
			}
			old, ok := m.InsertOwned(rustvec.NewOwned(auditKey{slot, slot + 40}, rustvec.ValueOps[auditKey]{Drop: m.keys.Drop}), rustvec.NewOwned(auditValue{slot + 50}, rustvec.ValueOps[auditValue]{Drop: m.values.Drop}), c)
			if ok {
				old.DropWithContext(c)
			}
		case 2:
			q := uint64(1)
			old, ok := RemoveBorrowedOwned(m, &q, lookup, c)
			if ok {
				old.DropWithContext(c)
			}
		case 3, 4:
			slot := uint64(1)
			if scenario == 4 {
				slot = 4
			}
			m.Entry(auditKey{slot, slot + 40}, c).Close(c)
		case 5, 6:
			slot := uint64(1)
			if scenario == 6 {
				slot = 4
			}
			entry := m.Entry(auditKey{slot, slot + 40}, c)
			entry.OrInsert(auditValue{slot + 50}, c)
		case 7, 8:
			slot := uint64(1)
			if scenario == 8 {
				slot = 4
			}
			_, err := m.TryInsert(auditKey{slot, slot + 40}, auditValue{slot + 50}, c)
			if err != nil {
				err.Close(c)
			}
		case 9:
			e, _ := m.Entry(auditKey{1, 41}, c).TakeOccupied(c)
			defer e.Close(c)
			old := e.InsertOwned(rustvec.NewOwned(auditValue{51}, rustvec.ValueOps[auditValue]{Drop: m.values.Drop}), c)
			old.DropWithContext(c)
		case 10:
			e, _ := m.Entry(auditKey{1, 41}, c).TakeOccupied(c)
			e.RemoveOwned(c).DropWithContext(c)
		case 11:
			e, _ := m.Entry(auditKey{1, 41}, c).TakeOccupied(c)
			e.RemoveEntryOwned(c).DropWithContext(c)
		case 12:
			e, _ := m.Entry(auditKey{4, 44}, c).TakeVacant(c)
			e.IntoKeyOwned(c).DropWithContext(c)
		case 13:
			m.Clone(c).Close(c)
		case 14:
			m.CloneFrom(other, c)
		case 15:
			m.Clear(c)
		case 16:
			old := m
			m = nil
			old.IntoIter(c).Close(c)
		case 17:
			old := m
			m = nil
			iterator := old.IntoIter(c)
			pair, ok := iterator.Next(c)
			var first *rustvec.Owned[Pair[auditKey, auditValue]]
			if ok {
				first = rustvec.NewOwned(pair, rustvec.ValueOps[Pair[auditKey, auditValue]]{Drop: old.dropPair})
				defer cleanupState(first, c)
			}
			iterator.Close(c)
			if first != nil {
				first.DropWithContext(c)
			}
		case 18, 19:
			slot := uint64(1)
			if scenario == 19 {
				slot = 4
			}
			env := auditEnvOwned(control)
			defer cleanupState(env, c)
			entry := m.Entry(auditKey{slot, slot + 40}, c)
			makeValue := NewOnce(env, func(env *auditEnv, _ *auditKey, ctx Context) *rustvec.Owned[auditValue] {
				env.calls++
				control.event("call", env.token, ctx)
				return rustvec.NewOwned(auditValue{90 + env.calls}, rustvec.ValueOps[auditValue]{Drop: m.values.Drop})
			})
			entry.OrInsertWithKeyOwned(makeValue, c)
		case 20, 21:
			slot := uint64(1)
			if scenario == 21 {
				slot = 4
			}
			env := auditEnvOwned(control)
			defer cleanupState(env, c)
			entry := m.Entry(auditKey{slot, slot + 40}, c)
			modify := NewOnce(env, func(env *auditEnv, value *auditValue, ctx Context) struct{} {
				env.calls++
				value.token++
				control.event("call", env.token, ctx)
				return struct{}{}
			})
			entry.AndModifyOwned(modify, c).Close(c)
		case 22, 23:
			slot := uint64(1)
			if scenario == 23 {
				slot = 4
			}
			m.Entry(auditKey{slot, slot + 40}, c).InsertEntry(auditValue{slot + 50}, c).Close(c)
		case 24:
			auditMapContext(control, ^uint(0), c).Close(c)
		case 25:
			replacement := m.Clone(c)
			old := m
			func() { defer func() { m = replacement }(); old.Close(c) }()
		case 26:
			q := uint64(1)
			GetBorrowed(m, &q, lookup, c)
		case 27:
			query := rustvec.NewOwned(auditKey{1, 41}, rustvec.ValueOps[auditKey]{Drop: m.keys.Drop})
			defer cleanupState(query, c)
			old, ok := m.RemoveOwned(query.Borrow(), c)
			if ok {
				old.DropWithContext(c)
			}
		default:
			panic("unknown owned reference scenario")
		}
	})
	state = auditSnapshot(m)
	control.mark("final")
	final = auditCatch(func() {
		if other != nil {
			old := other
			other = nil
			old.Close(c)
		}
		if m != nil {
			old := m
			m = nil
			old.Close(c)
		}
	})
	return
}

func TestOriginalHashMapOwnership(t *testing.T) {
	metadata, err := os.ReadFile("../../../tools/port-rust-unord/table/ownership_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases     int    `json:"cases"`
		Fixture   string `json:"fixture_sha256"`
		Raw       string `json:"raw_sha256"`
		Oracle    string `json:"oracle_sha256"`
		Generator string `json:"generator_sha256"`
		Support   string `json:"support_generator_sha256"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{"../../../tools/port-rust-unord/table/ownership_oracle.rs": manifest.Oracle, "../../../tools/port-rust-unord/table/ownership_generate.py": manifest.Generator, "../../../tools/port-rust-unord/table/generate.py": manifest.Support} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("owned reference binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/ownership.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if digest(compressed) != manifest.Fixture || manifest.Cases != 1098 {
		t.Fatal("owned reference fixture/coverage differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(reader, 16<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("owned reference body differs:%v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	cases := 0
	for scanner.Scan() {
		cases++
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 8 {
			t.Fatalf("malformed owned case%d", cases)
		}
		scenario, _ := strconv.Atoi(fields[0])
		fail, _ := strconv.Atoi(fields[2])
		counter, _ := strconv.Atoi(fields[5])
		wantState, _ := hex.DecodeString(fields[6])
		wantEvents, _ := hex.DecodeString(fields[7])
		control := &auditControl{fail: fail, second: -1}
		c := Context{Unwinding: fields[1] == "unwinding"}
		action, final, state := auditWork(scenario, control, c)
		if action != fields[3] || final != fields[4] || state != string(wantState) || control.counter != counter || strings.Join(control.events, "|") != string(wantEvents) {
			t.Fatalf("owned case%d (%s):\ngot %s,%s,%d state=%s\nevents=%s\nwant %s,%s,%d state=%s\nevents=%s", cases, strings.Join(fields[:3], "\t"), action, final, control.counter, state, strings.Join(control.events, "|"), fields[3], fields[4], counter, wantState, wantEvents)
		}
	}
	if err := scanner.Err(); err != nil || cases != manifest.Cases {
		t.Fatalf("owned reference incomplete:%d:%v", cases, err)
	}
}
