package rustcserialize

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type mapTestEncoder struct {
	*MemEncoder
	events *[]string
}

func (e *mapTestEncoder) DeterministicHashMapEncode() bool { return true }
func (e *mapTestEncoder) OpaqueContiguousBytes() bool      { return false }
func (e *mapTestEncoder) EmitUsize(v uint) {
	*e.events = append(*e.events, "len:"+strconv.FormatUint(uint64(v), 10))
	e.MemEncoder.EmitUsize(v)
}
func (e *mapTestEncoder) EmitU32(v uint32) {
	*e.events = append(*e.events, "num:"+strconv.FormatUint(uint64(v), 10))
	e.MemEncoder.EmitU32(v)
}
func mapTestMap() *rusthashmap.Map[uint32, uint32] {
	keys := rusthashmap.KeyOps[uint32]{Hash: func(value *uint32, h rusthashmap.Hasher, _ rusthashmap.Context) { rusthashmap.HashU32(h, *value) }, Equal: func(a, b *uint32, _ rusthashmap.Context) bool { return *a == *b }, TrivialClone: true}
	return rusthashmap.NewMap(rusthashmap.Layout{Size: 8, Align: 4}, rusthashmap.DefaultProfile(), keys, rusthashmap.ValueOps[uint32]{TrivialClone: true}, rusthashmap.FxBuilder{})
}
func mapTestOps(events *[]string, panicCompare bool) HashMapEncodeOps[uint32, uint32] {
	type env struct{ calls int }
	return HashMapEncodeOps[uint32, uint32]{Key: func(v *uint32, e Encoder, _ rustfmt.Context) { *events = append(*events, "key"); e.EmitU32(*v) }, Value: func(v *uint32, e Encoder, _ rustfmt.Context) { *events = append(*events, "value"); e.EmitU32(*v) }, Comparator: func(e Encoder, ctx rustfmt.Context) EncodeKeyComparator[uint32] {
		*events = append(*events, "factory")
		owned := rustvec.NewOwned(env{}, rustvec.ValueOps[env]{Drop: func(v *env, ctx rustfmt.Context) {
			*events = append(*events, "drop:"+strconv.FormatBool(ctx.Panicking()))
		}})
		return NewOwnedEncodeKeyComparator(owned, func(ref rustvec.Ref[env], a, b *uint32) int {
			ref.Value.calls++
			*events = append(*events, "compare")
			if panicCompare {
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
}
func TestDeterministicEmptyMapStillCreatesAndDropsComparator(t *testing.T) {
	events := []string{}
	e := &mapTestEncoder{MemEncoder: NewMemEncoder(), events: &events}
	m := mapTestMap()
	defer m.Close(rusthashmap.Context{})
	EncodeHashMap(e, m, mapTestOps(&events, false))
	if !reflect.DeepEqual(events, []string{"len:0", "factory", "drop:false"}) {
		t.Fatal(events)
	}
}
func TestDeterministicMapDropsComparatorBeforeEncodingSortedKeys(t *testing.T) {
	events := []string{}
	e := &mapTestEncoder{MemEncoder: NewMemEncoder(), events: &events}
	m := mapTestMap()
	defer m.Close(rusthashmap.Context{})
	for _, key := range []uint32{3, 1, 2} {
		m.Insert(key, key*7, rusthashmap.Context{})
	}
	EncodeHashMap(e, m, mapTestOps(&events, false))
	joined := strings.Join(events, ",")
	if !strings.HasPrefix(joined, "len:3,factory,") || !strings.Contains(joined, "drop:false,key,num:1,value,num:7,key,num:2,value,num:14,key,num:3,value,num:21") {
		t.Fatal(events)
	}
}
func TestDeterministicMapComparatorPanicDestroysItsMutatedCell(t *testing.T) {
	events := []string{}
	e := &mapTestEncoder{MemEncoder: NewMemEncoder(), events: &events}
	m := mapTestMap()
	defer m.Close(rusthashmap.Context{})
	m.Insert(1, 2, rusthashmap.Context{})
	m.Insert(2, 3, rusthashmap.Context{})
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("missing comparison panic")
			}
		}()
		EncodeHashMap(e, m, mapTestOps(&events, true))
	}()
	if !reflect.DeepEqual(events, []string{"len:2", "factory", "compare", "drop:true"}) {
		t.Fatal(events)
	}
}
