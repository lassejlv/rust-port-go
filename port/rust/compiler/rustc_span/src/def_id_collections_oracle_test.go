// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type defIDCollectionRequest struct {
	Kind, Constructor, Action string
	Local                     bool
	Capacity                  uint
	Clone, Drop               int
}
type defIDCollectionLog struct {
	events              []string
	failClone, failDrop int
	dropped             bool
}
type defIDCollectionItem struct {
	tag uint64
	log *defIDCollectionLog
}

func (l *defIDCollectionLog) item(tag uint64) defIDCollectionItem { return defIDCollectionItem{tag, l} }
func (l *defIDCollectionLog) drop(v *defIDCollectionItem, c rustfmt.Context) {
	l.events = append(l.events, fmt.Sprintf("drop:%d:%t", v.tag, c.Panicking()))
	if l.failDrop == int(v.tag) && !l.dropped {
		l.dropped = true
		c.Panic("drop")
	}
}
func (l *defIDCollectionLog) config() IdMapConfig[defIDCollectionItem] {
	return IdMapConfig[defIDCollectionItem]{Layout: rusthashmap.Layout{Size: 24, Align: 8}, Profile: rusthashmap.DefaultProfile(), Values: rusthashmap.ValueOps[defIDCollectionItem]{
		Clone: func(v *defIDCollectionItem, c rustfmt.Context) defIDCollectionItem {
			l.events = append(l.events, fmt.Sprintf("clone:%d:%t", v.tag, c.Panicking()))
			if l.failClone == int(v.tag) {
				c.Panic("clone")
			}
			return *v
		},
		Drop: l.drop,
		Equal: func(a, b *defIDCollectionItem, c rustfmt.Context) bool {
			l.events = append(l.events, fmt.Sprintf("equal:%d:%d:%t", a.tag, b.tag, c.Panicking()))
			return a.tag == b.tag
		},
		Debug: func(v *defIDCollectionItem, f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(v.tag, 64, f) },
	}}
}

type defIDCollectionCommon[K any] interface {
	Insert(K, defIDCollectionItem, ...rusthashmap.Context) (defIDCollectionItem, bool)
	Get(*K, ...rusthashmap.Context) (*defIDCollectionItem, bool)
	Remove(*K, ...rusthashmap.Context) (defIDCollectionItem, bool)
	Len() uint
	Close(...rusthashmap.Context)
	RustDebug() string
}
type defIDCollectionMap[K any] struct {
	defIDCollectionCommon[K]
	owner     any
	clone     func() defIDCollectionMap[K]
	cloneFrom func(defIDCollectionMap[K])
	equal     func(defIDCollectionMap[K]) bool
	entry     func(K, defIDCollectionItem)
}

func defIDCollectionWrap(m *DefIdMap[defIDCollectionItem]) defIDCollectionMap[DefId] {
	return defIDCollectionMap[DefId]{defIDCollectionCommon: m, owner: m,
		clone:     func() defIDCollectionMap[DefId] { return defIDCollectionWrap(m.Clone()) },
		cloneFrom: func(v defIDCollectionMap[DefId]) { m.CloneFrom(v.owner.(*DefIdMap[defIDCollectionItem])) },
		equal:     func(v defIDCollectionMap[DefId]) bool { return m.Equal(v.owner.(*DefIdMap[defIDCollectionItem])) },
		entry: func(k DefId, v defIDCollectionItem) {
			e := m.Entry(k)
			e = e.AndModify(defIDCollectionModify, rustfmt.Context{})
			e.OrInsert(v, rustfmt.Context{})
		},
	}
}
func localDefIDCollectionWrap(m *LocalDefIdMap[defIDCollectionItem]) defIDCollectionMap[LocalDefId] {
	return defIDCollectionMap[LocalDefId]{defIDCollectionCommon: m, owner: m,
		clone:     func() defIDCollectionMap[LocalDefId] { return localDefIDCollectionWrap(m.Clone()) },
		cloneFrom: func(v defIDCollectionMap[LocalDefId]) { m.CloneFrom(v.owner.(*LocalDefIdMap[defIDCollectionItem])) },
		equal: func(v defIDCollectionMap[LocalDefId]) bool {
			return m.Equal(v.owner.(*LocalDefIdMap[defIDCollectionItem]))
		},
		entry: func(k LocalDefId, v defIDCollectionItem) {
			e := m.Entry(k)
			e = e.AndModify(defIDCollectionModify, rustfmt.Context{})
			e.OrInsert(v, rustfmt.Context{})
		},
	}
}
func defIDCollectionModify(v *defIDCollectionItem) {
	v.log.events = append(v.log.events, fmt.Sprintf("modify:%d", v.tag))
	v.tag += 1000
}
func defIDCollectionKey(n uint32) DefId {
	return DefId{Krate: CrateNumFromU32(n % 3), Index: DefIndexFromU32(n * 7)}
}
func localDefIDCollectionKey(n uint32) LocalDefId {
	return LocalDefId{LocalDefIndex: DefIndexFromU32(n * 7)}
}
func defIDCollectionPairs[K any](key func(uint32) K, l *defIDCollectionLog) rustvec.IntoIterator[rusthashmap.Pair[K, defIDCollectionItem]] {
	values := make([]rusthashmap.Pair[K, defIDCollectionItem], 6)
	for n := range values {
		values[n] = rusthashmap.Pair[K, defIDCollectionItem]{Key: key(uint32(n)), Value: l.item(10 + uint64(n))}
	}
	return rustvec.NewVecIntoIter(values, rustvec.ElementOps[rusthashmap.Pair[K, defIDCollectionItem]]{Drop: func(v *rusthashmap.Pair[K, defIDCollectionItem], c rustfmt.Context) { l.drop(&v.Value, c) }}).IntoIterator()
}
func defIDCollectionSnapshot[K any](m defIDCollectionMap[K], key func(uint32) K) any {
	values := make([]any, 10)
	for n := uint32(0); n < 10; n++ {
		k := key(n)
		if v, ok := m.Get(&k); ok {
			values[n] = v.tag
		}
	}
	return map[string]any{"len": m.Len(), "values": values}
}
func defIDCollectionMaps[K any](q defIDCollectionRequest, source, dest defIDCollectionMap[K], key func(uint32) K, l *defIDCollectionLog) any {
	if q.Constructor != "iterator" {
		for n := uint32(0); n < 6; n++ {
			source.Insert(key(n), l.item(10+uint64(n)))
		}
	}
	for n := uint32(0); n < 3; n++ {
		dest.Insert(key(n), l.item(100+uint64(n)))
	}
	if v, ok := source.Insert(key(2), l.item(42)); ok {
		l.drop(&v, rustfmt.Context{})
	}
	source.entry(key(1), l.item(999))
	source.entry(key(9), l.item(99))
	k := key(3)
	if v, ok := source.Remove(&k); ok {
		l.drop(&v, rustfmt.Context{})
	}
	l.failClone, l.failDrop = q.Clone, q.Drop
	var cloned *defIDCollectionMap[K]
	p, failed := defIDCatch(func() any {
		if q.Action == "clone_from" {
			dest.cloneFrom(source)
		} else {
			v := source.clone()
			cloned = &v
		}
		return nil
	})
	if !failed {
		p = nil
	}
	l.failClone, l.failDrop = -1, -1
	var equal, snapshot any
	if cloned != nil {
		equal = source.equal(*cloned)
		snapshot = defIDCollectionSnapshot(*cloned, key)
	}
	result := map[string]any{"source": defIDCollectionSnapshot(source, key), "dest": defIDCollectionSnapshot(dest, key), "cloned": snapshot, "equal": equal, "panic": p, "debug": source.RustDebug(), "pair_layout": [2]uint{24, 8}}
	if cloned != nil {
		cloned.Close()
	}
	dest.Close()
	source.Close()
	return map[string]any{"state": result, "events": l.events}
}

type defIDCollectionSet[K any] interface {
	Insert(K, ...rusthashmap.Context) bool
	Remove(*K, ...rusthashmap.Context) bool
	Contains(*K, ...rusthashmap.Context) bool
	Len() uint
	GetOnly(...rusthashmap.Context) (*K, bool)
	Clear(...rusthashmap.Context)
	Close(...rusthashmap.Context)
	RustDebug() string
}

func defIDCollectionSets[K any](source, dest defIDCollectionSet[K], key func(uint32) K, clone func() defIDCollectionSet[K], cloneFrom func(), equal func(defIDCollectionSet[K]) bool) any {
	defer source.Close()
	defer dest.Close()
	inserted := make([]bool, 10)
	for n := uint32(0); n < 10; n++ {
		inserted[n] = source.Insert(key(n))
	}
	duplicate := source.Insert(key(2))
	k := key(4)
	removed := source.Remove(&k)
	k = key(11)
	missing := source.Remove(&k)
	cloned := clone()
	defer cloned.Close()
	dest.Insert(key(20))
	cloneFrom()
	same := equal(cloned)
	text := source.RustDebug()
	source.Clear()
	source.Insert(key(9))
	v, ok := source.GetOnly()
	var only any
	if ok {
		only = reflect.DeepEqual(*v, key(9))
	}
	contains := make([]bool, 12)
	for n := uint32(0); n < 12; n++ {
		k = key(n)
		contains[n] = dest.Contains(&k)
	}
	keySize := uint(8)
	if _, local := any(key(0)).(LocalDefId); local {
		keySize = 4
	}
	return map[string]any{"inserted": inserted, "duplicate": duplicate, "removed": removed, "missing": missing, "len": dest.Len(), "equal": same, "only": only, "debug": text, "contains": contains, "key_layout": [2]uint{keySize, 4}}
}
func defIDCollectionSetInput[K any](key func(uint32) K) rustvec.IntoIterator[K] {
	values := make([]K, 10)
	for n := range values {
		values[n] = key(uint32(n))
	}
	return rustvec.NewVecIntoIter(values, rustvec.ElementOps[K]{}).IntoIterator()
}
func defIDCollectionsObserve(q defIDCollectionRequest) any {
	if q.Kind == "set" {
		if q.Local {
			source := NewLocalDefIdSetWithCapacity(q.Capacity)
			if q.Constructor == "iterator" {
				source.Close()
				source = LocalDefIdSetFromIterator(defIDCollectionSetInput(localDefIDCollectionKey), rusthashmap.DefaultProfile())
			}
			dest := NewLocalDefIdSet()
			return defIDCollectionSets[LocalDefId](source, dest, localDefIDCollectionKey, func() defIDCollectionSet[LocalDefId] { return source.Clone() }, func() { dest.CloneFrom(source) }, func(v defIDCollectionSet[LocalDefId]) bool { return dest.Equal(v.(*LocalDefIdSet)) })
		}
		source := NewDefIdSetWithCapacity(q.Capacity)
		if q.Constructor == "iterator" {
			source.Close()
			source = DefIdSetFromIterator(defIDCollectionSetInput(defIDCollectionKey), rusthashmap.DefaultProfile())
		}
		dest := NewDefIdSet()
		return defIDCollectionSets[DefId](source, dest, defIDCollectionKey, func() defIDCollectionSet[DefId] { return source.Clone() }, func() { dest.CloneFrom(source) }, func(v defIDCollectionSet[DefId]) bool { return dest.Equal(v.(*DefIdSet)) })
	}
	l := &defIDCollectionLog{events: []string{}, failClone: -1, failDrop: -1}
	config := l.config()
	if q.Local {
		source := NewLocalDefIdMapWithCapacity(q.Capacity, config)
		if q.Constructor == "iterator" {
			source.Close()
			source = LocalDefIdMapFromIterator(defIDCollectionPairs(localDefIDCollectionKey, l), config)
		}
		return defIDCollectionMaps(q, localDefIDCollectionWrap(source), localDefIDCollectionWrap(NewLocalDefIdMapWithCapacity(q.Capacity, config)), localDefIDCollectionKey, l)
	}
	source := NewDefIdMapWithCapacity(q.Capacity, config)
	if q.Constructor == "iterator" {
		source.Close()
		source = DefIdMapFromIterator(defIDCollectionPairs(defIDCollectionKey, l), config)
	}
	return defIDCollectionMaps(q, defIDCollectionWrap(source), defIDCollectionWrap(NewDefIdMapWithCapacity(q.Capacity, config)), defIDCollectionKey, l)
}
func TestDefIdOriginalCollections(t *testing.T) {
	file, err := os.Open("testdata/def_id_collections.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	scan := bufio.NewScanner(reader)
	count := 0
	for scan.Scan() {
		var row struct {
			Request defIDCollectionRequest
			Result  json.RawMessage
		}
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		got, err := json.Marshal(defIDCollectionsObserve(row.Request))
		if err != nil {
			t.Fatal(err)
		}
		var a, b any
		dec := json.NewDecoder(strings.NewReader(string(got)))
		dec.UseNumber()
		if err := dec.Decode(&a); err != nil {
			t.Fatal(err)
		}
		dec = json.NewDecoder(strings.NewReader(string(row.Result)))
		dec.UseNumber()
		if err := dec.Decode(&b); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(a, b) {
			t.Fatalf("row%d %+v\ngot%s\nwant%s", count, row.Request, got, row.Result)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count < 50 {
		t.Fatalf("incomplete collection corpus:%d", count)
	}
}
