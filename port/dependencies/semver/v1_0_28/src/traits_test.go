package semver

import (
	"encoding/binary"
	"errors"
	"iter"
	"reflect"
	"strconv"
	"testing"
)

func TestNativeWordHashAndOptionalHooks(t *testing.T) {
	raw := [][]byte{}
	h := HasherFunc(func(b []byte) { raw = append(raw, append([]byte(nil), b...)) })
	HashUsize(h, 0x1234)
	HashIsize(h, -1)
	size := strconv.IntSize / 8
	want := make([]byte, size)
	if size == 8 {
		binary.NativeEndian.PutUint64(want, 0x1234)
	} else {
		binary.NativeEndian.PutUint32(want, 0x1234)
	}
	if !reflect.DeepEqual(raw[0], want) {
		t.Fatal(raw[0], want)
	}
	for _, b := range raw[1] {
		if b != 255 {
			t.Fatal(raw[1])
		}
	}
}
func TestCloneOwnsComparatorSliceAndIteratorOrder(t *testing.T) {
	a, _ := ParseComparator(">=1.2.3")
	b, _ := ParseComparator("<2")
	q := FromIterator(iter.Seq[Comparator](func(yield func(Comparator) bool) {
		if !yield(a) {
			return
		}
		yield(b)
	}))
	copy := q.Clone()
	copy.Comparators[0].Major = 9
	if q.Comparators[0].Major != 1 || len(q.Comparators) != 2 || !q.Comparators[1].Equal(b) {
		t.Fatal(q)
	}
	from := FromComparators(q.Comparators)
	from.Comparators[0].Major = 7
	if q.Comparators[0].Major != 1 {
		t.Fatal("FromComparators aliases input")
	}
}
func TestFeatureAvailability(t *testing.T) {
	_, err := ParseVersion("")
	_, has := err.(interface{ Source() error })
	if has != FeatureStd {
		t.Fatalf("std Source presence%v feature%v", has, FeatureStd)
	}
	if has && !errors.Is(err.(interface{ Source() error }).Source(), nil) {
		t.Fatal("nonempty source")
	}
}
