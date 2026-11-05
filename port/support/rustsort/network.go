// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

func (e *engine[T]) unstableThreshold() int {
	if !e.typ.Freeze {
		return 16
	}
	if e.typ.Copy && e.typ.Size <= 8 {
		return 32
	}
	if e.typ.Size <= 4096/48 {
		return 32
	}
	return 16
}
func (e *engine[T]) unstableSmall(v []T) {
	if len(v) < 2 {
		return
	}
	if e.typ.Freeze && e.typ.Copy && e.typ.Size <= 8 {
		e.smallNetwork(v)
		return
	}
	if e.typ.Freeze && e.typ.Size <= 4096/48 {
		e.smallGeneral(v, make([]T, 48))
		return
	}
	e.insertion(v, 1)
}
func (e *engine[T]) smallNetwork(v []T) {
	n, mid := len(v), len(v)/2
	noMerge := n < 18
	region := v
	if !noMerge {
		region = v[:mid]
	}
	for pass := 0; ; pass++ {
		presorted := 1
		if len(region) >= 13 {
			e.network(region, network13)
			presorted = 13
		} else if len(region) >= 9 {
			e.network(region, network9)
			presorted = 9
		}
		e.insertion(region, presorted)
		if noMerge {
			return
		}
		if pass > 0 {
			break
		}
		region = v[mid:]
	}
	scratch := make([]T, 32)
	e.bidirectional(v, scratch)
	copy(v, scratch[:n])
}
func (e *engine[T]) network(v []T, pairs [][2]int) {
	for _, pair := range pairs {
		if e.less(&v[pair[1]], &v[pair[0]]) {
			v[pair[0]], v[pair[1]] = v[pair[1]], v[pair[0]]
		}
	}
}

// Comparator sequences are copied directly from sort9_optimal/sort13_optimal
// in the audited SDK's shared/smallsort.rs, preserving their source order.
var network9 = [][2]int{{0, 3}, {1, 7}, {2, 5}, {4, 8}, {0, 7}, {2, 4}, {3, 8}, {5, 6}, {0, 2}, {1, 3}, {4, 5}, {7, 8}, {1, 4}, {3, 6}, {5, 7}, {0, 1}, {2, 4}, {3, 5}, {6, 8}, {2, 3}, {4, 5}, {6, 7}, {1, 2}, {3, 4}, {5, 6}}
var network13 = [][2]int{{0, 12}, {1, 10}, {2, 9}, {3, 7}, {5, 11}, {6, 8}, {1, 6}, {2, 3}, {4, 11}, {7, 9}, {8, 10}, {0, 4}, {1, 2}, {3, 6}, {7, 8}, {9, 10}, {11, 12}, {4, 6}, {5, 9}, {8, 11}, {10, 12}, {0, 5}, {3, 8}, {4, 7}, {6, 11}, {9, 10}, {0, 1}, {2, 5}, {6, 9}, {7, 8}, {10, 11}, {1, 3}, {2, 4}, {5, 6}, {9, 10}, {1, 2}, {3, 4}, {5, 7}, {6, 8}, {2, 3}, {4, 5}, {6, 7}, {8, 9}, {3, 4}, {5, 6}}
