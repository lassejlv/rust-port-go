// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type completeBoundCase struct {
	Group      string
	Family     int
	Cache      bool
	Stdout     string
	Allocation []string
}
type completeBoundClone[T any] struct{}

func (completeBoundClone[T]) Clone(*T, rustfmt.Context) *rustvec.Owned[T] { panic("UNEXPECTED Clone") }

type completeBoundCopy[T any] struct{ completeBoundClone[T] }

func (completeBoundCopy[T]) UnordCopyMarker() { panic("UNEXPECTED Copy marker") }

type completeBoundDefault[T any] struct{}

func (completeBoundDefault[T]) Default(rustfmt.Context) *rustvec.Owned[T] {
	panic("UNEXPECTED Default")
}

type completeBoundOrd[T any] struct{}

func (completeBoundOrd[T]) Equal(*T, *T, rustfmt.Context) bool { panic("UNEXPECTED Eq") }
func (completeBoundOrd[T]) PartialCompare(*T, *T, rustfmt.Context) (rustsort.Ordering, bool) {
	panic("UNEXPECTED PartialOrd")
}
func (completeBoundOrd[T]) Compare(*T, *T, rustfmt.Context) rustsort.Ordering {
	panic("UNEXPECTED Ord")
}

type completeBoundInput struct{ tag uint64 }
type completeBoundOtherInput completeBoundInput
type completeBoundRefInput struct{ value *uint64 }
type completeBoundTotal struct{}
type completeBoundTotalTraits struct{}

func (completeBoundTotalTraits) Sum(*rustvec.Owned[completeBoundInput], rustfmt.Context) *rustvec.Owned[completeBoundTotal] {
	panic("UNEXPECTED Sum")
}
func (completeBoundTotalTraits) Product(*rustvec.Owned[completeBoundInput], rustfmt.Context) *rustvec.Owned[completeBoundTotal] {
	panic("UNEXPECTED Product")
}

type completeBoundDefaultValue struct{ number uint64 }
type completeBoundRunner struct{ output *strings.Builder }

func (r completeBoundRunner) input(tag uint64) *rustvec.Owned[completeBoundInput] {
	return rustvec.NewOwned(completeBoundInput{tag}, rustvec.ValueOps[completeBoundInput]{Drop: func(input *completeBoundInput, _ rustfmt.Context) { fmt.Fprintf(r.output, "input:%d\n", input.tag) }})
}
func (r completeBoundRunner) inputOps() UnordIteratorOps[completeBoundInput, uint64] {
	return UnordIteratorOps[completeBoundInput, uint64]{Next: func(ref rustvec.Ref[completeBoundInput]) *rustvec.Owned[uint64] {
		ref.Context.Panic("UNEXPECTED next")
		return nil
	}, SizeHint: func(rustvec.Ref[completeBoundInput]) rustvec.Hint { return rustvec.Hint{} }, Count: func(_ *rustvec.Owned[completeBoundInput], c rustfmt.Context) uint {
		c.Panic("UNEXPECTED next")
		return 0
	}, Clone: func(ref rustvec.Ref[completeBoundInput]) *rustvec.Owned[completeBoundInput] {
		fmt.Fprintf(r.output, "clone:%d\n", ref.Value.tag)
		return r.input(ref.Value.tag + 100)
	}, CloneItem: true}
}
func (r completeBoundRunner) primitive(family int, borrowed bool) {
	c := rustfmt.Context{}
	switch family {
	case 0:
		config := UnordBagConfig[completeBoundDefaultValue]{Layout: rusthashmap.Layout{Size: 8, Align: 8}, Profile: rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}, Values: UnordBagValueOps[completeBoundDefaultValue]{Drop: func(value *completeBoundDefaultValue, _ rustfmt.Context) {
			fmt.Fprintf(r.output, "item:%d\n", value.number)
		}}}
		bag := DefaultUnordBagWithBounds(config, completeBoundDefault[completeBoundDefaultValue]{})
		if bag.inner.Raw.Cap() != 0 {
			panic("nonempty Default capacity")
		}
		fmt.Fprintf(r.output, "empty:%d\n", bag.Len())
		bag.Push(completeBoundDefaultValue{7})
		fmt.Fprintf(r.output, "push:%d:capacity%d\n", bag.Len(), bag.inner.Raw.Cap())
		bag.Close()
	case 1:
		type alias = completeBoundInput
		source := NewUnordItems(r.input(1), r.inputOps())
		destination := NewUnordItems[alias](r.input(2), r.inputOps())
		CloneUnordItemsWithBounds(source, completeBoundClone[uint64]{}).Close()
		CloneFromUnordItemsWithBounds(destination, source, completeBoundClone[uint64]{})
		destination.Close()
		source.Close()
	case 2, 3:
		items := NewUnordItems(r.input(0), r.inputOps())
		selected := UnordIteratorReduction[completeBoundInput, uint64, uint64](func(_ *rustvec.Owned[completeBoundInput], _ rustfmt.Context) *rustvec.Owned[uint64] {
			label := "max_override"
			if family == 3 {
				label = "min_override"
			}
			fmt.Fprintln(r.output, label)
			return rustvec.Own(uint64(7))
		})
		var value *rustvec.Owned[uint64]
		if family == 2 {
			value = MaxUnordItemsWithBounds(items, selected, completeBoundOrd[uint64]{})
		} else {
			value = MinUnordItemsWithBounds(items, selected, completeBoundOrd[uint64]{})
		}
		fmt.Fprintf(r.output, "returned:%d\n", value.Take())
	case 4, 5:
		items := NewUnordItems(r.input(0), r.inputOps())
		selected := UnordIteratorReduction[completeBoundInput, uint64, completeBoundTotal](func(_ *rustvec.Owned[completeBoundInput], c rustfmt.Context) *rustvec.Owned[completeBoundTotal] {
			label := "sum_override"
			if family == 5 {
				label = "product_override"
			}
			fmt.Fprintln(r.output, label)
			c.Panic(label)
			return nil
		})
		if family == 4 {
			SumUnordItemsWithBounds(items, selected, completeBoundTotalTraits{})
		} else {
			ProductUnordItemsWithBounds(items, selected, completeBoundTotalTraits{})
		}
	case 6, 7:
		value := uint64(0)
		state := rustvec.NewOwned(completeBoundRefInput{&value}, rustvec.ValueOps[completeBoundRefInput]{Drop: func(*completeBoundRefInput, rustfmt.Context) { fmt.Fprintln(r.output, "ref_input") }})
		label := "cloned_override"
		if family == 7 {
			label = "copied_override"
		}
		if borrowed {
			items := NewUnordItems(state, UnordIteratorOps[completeBoundRefInput, *uint64]{Next: func(ref rustvec.Ref[completeBoundRefInput]) *rustvec.Owned[*uint64] {
				ref.Context.Panic("UNEXPECTED ref next")
				return nil
			}, SizeHint: func(rustvec.Ref[completeBoundRefInput]) rustvec.Hint { return rustvec.Hint{} }, Count: func(_ *rustvec.Owned[completeBoundRefInput], c rustfmt.Context) uint {
				c.Panic("UNEXPECTED ref next")
				return 0
			}})
			selected := UnordUnaryIteratorConstructor[completeBoundRefInput, *uint64, struct{}, uint64]{Construct: func(_ *rustvec.Owned[completeBoundRefInput], c rustfmt.Context) *rustvec.Owned[struct{}] {
				fmt.Fprintln(r.output, label)
				c.Panic(label)
				return nil
			}}
			if family == 6 {
				ClonedBorrowedUnordItemsWithBounds(items, selected, completeBoundClone[uint64]{}, c)
			} else {
				CopiedBorrowedUnordItemsWithBounds(items, selected, completeBoundCopy[uint64]{}, c)
			}
		} else {
			items := NewUnordItems(state, UnordIteratorOps[completeBoundRefInput, rustvec.Ref[uint64]]{Next: func(ref rustvec.Ref[completeBoundRefInput]) *rustvec.Owned[rustvec.Ref[uint64]] {
				ref.Context.Panic("UNEXPECTED ref next")
				return nil
			}, SizeHint: func(rustvec.Ref[completeBoundRefInput]) rustvec.Hint { return rustvec.Hint{} }, Count: func(_ *rustvec.Owned[completeBoundRefInput], c rustfmt.Context) uint {
				c.Panic("UNEXPECTED ref next")
				return 0
			}})
			selected := UnordUnaryIteratorConstructor[completeBoundRefInput, rustvec.Ref[uint64], struct{}, uint64]{Construct: func(_ *rustvec.Owned[completeBoundRefInput], c rustfmt.Context) *rustvec.Owned[struct{}] {
				fmt.Fprintln(r.output, label)
				c.Panic(label)
				return nil
			}}
			if family == 6 {
				ClonedUnordItemsWithBounds(items, selected, completeBoundClone[uint64]{}, c)
			} else {
				CopiedUnordItemsWithBounds(items, selected, completeBoundCopy[uint64]{}, c)
			}
		}
	}
}

type completeBoundKey struct{ output *strings.Builder }

func (completeBoundKey) Equal(*uint64, *uint64, rustfmt.Context) bool { panic("UNEXPECTED key Eq") }
func (completeBoundKey) Compare(*uint64, *uint64, rustfmt.Context) rustsort.Ordering {
	panic("UNEXPECTED key Ord")
}
func (completeBoundKey) StableHash(*uint64, StableHashCtxt, *StableHasher, rustfmt.Context) {
	panic("UNEXPECTED key StableHash")
}
func (key completeBoundKey) PartialCompare(a, b *uint64, _ rustfmt.Context) (rustsort.Ordering, bool) {
	fmt.Fprintf(key.output, "partial:%d:%d\n", *a, *b)
	if *a < *b {
		return rustsort.LessThan, true
	}
	if *a > *b {
		return rustsort.GreaterThan, true
	}
	return rustsort.Equal, true
}
func (key completeBoundKey) less(a, b *uint64, _ rustfmt.Context) bool {
	fmt.Fprintf(key.output, "lt:%d:%d\n", *a, *b)
	return *a < *b
}
func (r completeBoundRunner) key() UnordStableKeyBounds[uint64, uint64] {
	value := completeBoundKey{r.output}
	return UnordStableKeyBounds[uint64, uint64]{Value: value, Used: UnordStableKeyOps[uint64, uint64]{Project: func(item *uint64, _ StableHashCtxt, _ rustfmt.Context) *rustvec.Owned[uint64] {
		fmt.Fprintf(r.output, "project:%d\n", *item)
		return rustvec.Own(*item)
	}, Less: value.less, KeyType: rustsort.Type{Size: 8, Align: 8, Freeze: true}, PointerBits: 64, Cached: rustsort.CachedKeyOps[uint64]{PartialCompare: value.PartialCompare}}}
}

type completeBoundArray struct {
	values [2]uint64
	front  uint
}

func completeBoundArrayItems() *UnordItems[uint64] {
	return NewUnordItems(rustvec.Own(completeBoundArray{values: [2]uint64{2, 1}}), UnordIteratorOps[completeBoundArray, uint64]{Next: func(ref rustvec.Ref[completeBoundArray]) *rustvec.Owned[uint64] {
		if ref.Value.front == 2 {
			return nil
		}
		value := ref.Value.values[ref.Value.front]
		ref.Value.front++
		return rustvec.Own(value)
	}, SizeHint: func(ref rustvec.Ref[completeBoundArray]) rustvec.Hint {
		n := 2 - ref.Value.front
		return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
	}, Count: func(owner *rustvec.Owned[completeBoundArray], _ rustfmt.Context) uint {
		return 2 - owner.Borrow().front
	}, Clone: func(ref rustvec.Ref[completeBoundArray]) *rustvec.Owned[completeBoundArray] {
		return rustvec.Own(*ref.Value)
	}, CloneItem: true})
}
func completeBoundArrayCollector() UnordVectorCollector[completeBoundArray, uint64] {
	return UnordVectorCollector[completeBoundArray, uint64]{Type: rustsort.Type{Size: 8, Align: 8, Copy: true, Freeze: true}, Collect: func(owner *rustvec.Owned[completeBoundArray], c rustfmt.Context) *rustcindex.IndexVec[uint, uint64] {
		state := owner.Take()
		// Actual ArrayIntoIter's TrustedLen collection and independently selected
		// array fold move the remaining initialized range without Next callbacks.
		fold := func(consume func(uint64)) {
			for state.front < 2 {
				value := state.values[state.front]
				state.front++
				consume(value)
			}
		}
		input := rustcindex.IndexIntoIterator[uint64]{IntoIterWithContext: func(rustfmt.Context) rustcindex.IndexValueIterator[uint64] {
			return rustcindex.IndexValueIterator[uint64]{TrustedLength: true, SizeHint: func() (uint, uint, bool) { n := 2 - state.front; return n, n, true }, Next: func() (uint64, bool) {
				if state.front == 2 {
					return 0, false
				}
				value := state.values[state.front]
				state.front++
				return value, true
			}, ForEachOverride: fold, FoldUnitOverride: fold}
		}}
		return rustcindex.IndexVecFromIteratorWithConfigAndContext(input, rustcindex.UsizeIdx(), rustcindex.ValueOps[uint64]{RustSize: 8, TrivialClone: true}, rustcindex.IndexStorageConfig{DebugAssertions: true, OverflowChecks: true}, c)
	}}
}
func (r completeBoundRunner) show(values []uint64) {
	parts := make([]string, len(values))
	for i, value := range values {
		parts[i] = fmt.Sprint(value)
	}
	fmt.Fprintf(r.output, "returned:%s\n", strings.Join(parts, ","))
}
func (r completeBoundRunner) keys(family int, cache bool) {
	key := r.key()
	typ := rustsort.Type{Size: 8, Align: 8, Copy: true, Freeze: true}
	switch family {
	case 0, 1:
		var vector *rustcindex.IndexVec[uint, uint64]
		if family == 0 {
			vector = CollectUnordItemsSortedWithBounds(completeBoundArrayItems(), nil, cache, completeBoundArrayCollector().forSorting(), key).Take()
		} else {
			vector = IntoSortedUnordItemsWithBounds(completeBoundArrayItems(), nil, completeBoundArrayCollector(), key)
		}
		r.show(vector.Raw.AsGoSlice())
		vector.Close()
	case 2, 3, 4:
		config := UnordMapConfig[uint64, uint64]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: unordTestKeys()}
		mapValue := NewUnordMap(config)
		mapValue.Insert(2, 102)
		mapValue.Insert(1, 101)
		if family == 2 {
			vector := UnordMapToSortedWithBounds(mapValue, nil, cache, key)
			values := make([]uint64, vector.Len())
			for i, value := range vector.Raw.AsGoSlice() {
				values[i] = *value.Key
			}
			r.show(values)
			vector.Close()
			mapValue.Close()
		} else if family == 3 {
			vector := UnordMapIntoSortedWithBounds(mapValue, nil, cache, rustsort.Type{Size: 16, Align: 8, Copy: true, Freeze: true}, key)
			values := make([]uint64, vector.Len())
			for i, value := range vector.Raw.AsGoSlice() {
				values[i] = value.Key
			}
			r.show(values)
			vector.Close()
		} else {
			iterator := UnordMapValuesSortedWithBounds(mapValue, nil, cache, key)
			var values []uint64
			for {
				value, ok := iterator.Next(rustfmt.Context{})
				if !ok {
					break
				}
				values = append(values, *value)
			}
			r.show(values)
			iterator.Close()
			mapValue.Close()
		}
	case 5, 6:
		config := UnordSetConfig[uint64]{Layout: rusthashmap.Layout{Size: 8, Align: 8}, Profile: rusthashmap.DefaultProfile(), Values: unordTestKeys()}
		set := NewUnordSet(config)
		set.Insert(2)
		set.Insert(1)
		if family == 5 {
			vector := UnordSetToSortedWithBounds(set, nil, cache, key)
			values := make([]uint64, vector.Len())
			for i, value := range vector.Raw.AsGoSlice() {
				values[i] = *value
			}
			r.show(values)
			vector.Close()
			set.Close()
		} else {
			vector := UnordSetIntoSortedWithBounds(set, nil, cache, typ, key)
			r.show(vector.Raw.AsGoSlice())
			vector.Close()
		}
	}
}
func TestOriginalUnordCompleteBounds(t *testing.T) {
	data, err := os.ReadFile("../../../../../tools/port-rust-unord/bounds/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		Bounds []struct {
			Configuration string
			Compiles      bool
		}
		Cases     []completeBoundCase
		Files     map[string]string
		Reference map[string]string `json:"reference_inputs"`
	}
	if err = json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	for _, table := range []map[string]string{metadata.Files, metadata.Reference} {
		for path, want := range table {
			data, err := os.ReadFile("../../../../../" + path)
			sum := sha256.Sum256(data)
			if err != nil || hex.EncodeToString(sum[:]) != want {
				t.Fatalf("complete bounds binding %s: %v", path, err)
			}
		}
	}
	if len(metadata.Bounds) != 18 || len(metadata.Cases) != 22 {
		t.Fatal("native complete bound coverage")
	}
	for _, spec := range metadata.Cases {
		for _, borrowed := range []bool{false, true} {
			if borrowed && (spec.Group != "primitive" || spec.Family < 6) {
				continue
			}
			var output strings.Builder
			r := completeBoundRunner{&output}
			func() {
				defer func() {
					if p := recover(); p != nil {
						fmt.Fprintf(&output, "caught:%v\n", p)
					}
				}()
				if spec.Group == "primitive" {
					r.primitive(spec.Family, borrowed)
				} else {
					r.keys(spec.Family, spec.Cache)
				}
			}()
			if output.String() != spec.Stdout {
				t.Fatalf("complete bounds %+v borrowed=%t\nGOT\n%sWANT\n%s", spec, borrowed, output.String(), spec.Stdout)
			}
			if spec.Group == "primitive" && spec.Family == 0 && (len(spec.Allocation) != 1 || spec.Allocation[0] != "ALLOC:0:1:32") {
				t.Fatal("original empty allocation/capacity evidence", spec.Allocation)
			}
		}
	}
	completeBoundCloneFromAdmission(t)
}
func completeBoundCloneFromAdmission(t *testing.T) {
	for _, opaque := range []bool{false, true} {
		var output strings.Builder
		r := completeBoundRunner{&output}
		source := NewUnordItems(r.input(1), r.inputOps())
		var destination *UnordItems[uint64]
		if opaque {
			destination = unordOpaqueItems(r.input(2), r.inputOps())
		} else {
			state := rustvec.NewOwned(completeBoundOtherInput{2}, rustvec.ValueOps[completeBoundOtherInput]{Drop: func(value *completeBoundOtherInput, _ rustfmt.Context) { fmt.Fprintf(&output, "input:%d\n", value.tag) }})
			destination = NewUnordItems(state, UnordIteratorOps[completeBoundOtherInput, uint64]{Next: func(rustvec.Ref[completeBoundOtherInput]) *rustvec.Owned[uint64] { panic("UNEXPECTED next") }, SizeHint: func(rustvec.Ref[completeBoundOtherInput]) rustvec.Hint { return rustvec.Hint{} }, Count: func(*rustvec.Owned[completeBoundOtherInput], rustfmt.Context) uint { panic("UNEXPECTED next") }, Clone: func(rustvec.Ref[completeBoundOtherInput]) *rustvec.Owned[completeBoundOtherInput] {
				panic("UNEXPECTED clone")
			}, CloneItem: true})
		}
		message := ""
		func() {
			defer func() {
				if p := recover(); p != nil {
					message = fmt.Sprint(p)
				}
			}()
			CloneFromUnordItemsWithBounds(destination, source, completeBoundClone[uint64]{})
		}()
		want := "same concrete Iterator Self"
		if opaque {
			want = "bounds do not expose Clone"
		}
		if !strings.Contains(message, want) || output.Len() != 0 {
			t.Fatalf("invalid CloneFrom performed effects: %q/%q", message, output.String())
		}
		destination.Close()
		source.Close()
		if output.String() != "input:2\ninput:1\n" {
			t.Fatal("rejected CloneFrom owners changed", output.String())
		}
	}
}
