// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete port of binary_search_util/tests.rs at Rust commit
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
package binarysearchutil

import (
	"slices"
	"testing"
)

type element struct {
	key   uint
	value string
}

func testMap() []element {
	data := []element{{3, "three-a"}, {0, "zero"}, {3, "three-b"}, {22, "twenty-two"}}
	slices.SortStableFunc(data, func(a, b element) int {
		if a.key < b.key {
			return -1
		}
		if a.key > b.key {
			return 1
		}
		return 0
	})
	return data
}
func getKey(data *element) uint { return data.key }

func TestBinarySearchSliceUpstream(t *testing.T) {
	data := testMap()
	for _, test := range []struct {
		key      uint
		expected []element
	}{
		{0, []element{{0, "zero"}}},
		{1, nil},
		{3, []element{{3, "three-a"}, {3, "three-b"}}},
		{22, []element{{22, "twenty-two"}}},
		{23, nil},
	} {
		actual := BinarySearchSlice(data, getKey, test.key)
		if !slices.Equal(actual, test.expected) {
			t.Fatalf("key %d: %+v, want %+v", test.key, actual, test.expected)
		}
	}
}
