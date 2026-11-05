// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

import "math/bits"

// Unstable implements the normal Rust 1.101 ipnsort strategy. Source Type and
// borrowed callback contracts are the same as Stable, including panic context.
func Unstable[T any](v []T, typ Type, less Less[T], contexts ...Context) {
	var context Context
	if len(contexts) > 1 {
		panic("rustsort: expected at most one context")
	}
	if len(contexts) == 1 {
		context = contexts[0]
	}
	if typ.Align == 0 || typ.Align&(typ.Align-1) != 0 || typ.Size%typ.Align != 0 {
		context.Panic("rustsort: invalid source type layout")
	}
	if typ.Size == 0 || len(v) < 2 {
		return
	}
	if less == nil {
		context.Panic("rustsort: missing comparison implementation")
	}
	e := engine[T]{typ: typ, context: context, compare: less}
	if len(v) <= 20 {
		e.insertion(v, 1)
		return
	}
	run, reversed := e.run(v)
	if run == len(v) {
		if reversed {
			reverse(v)
		}
		return
	}
	e.unstableQuick(v, nil, 2*(bits.Len(uint(len(v))|1)-1))
}

func (e *engine[T]) unstableQuick(v []T, ancestor *T, limit int) {
	for {
		if len(v) <= e.unstableThreshold() {
			e.unstableSmall(v)
			return
		}
		if limit == 0 {
			e.heap(v)
			return
		}
		limit--
		pivotPosition := e.pivot(v)
		if ancestor != nil && !e.less(ancestor, &v[pivotPosition]) {
			less := e.unstablePartition(v, pivotPosition, true)
			v = v[less+1:]
			ancestor = nil
			continue
		}
		less := e.unstablePartition(v, pivotPosition, false)
		pivot := &v[less]
		e.unstableQuick(v[:less], ancestor, limit)
		v = v[less+1:]
		ancestor = pivot
	}
}
func (e *engine[T]) unstablePartition(v []T, pivotPosition int, invert bool) int {
	if len(v) == 0 {
		return 0
	}
	v[0], v[pivotPosition] = v[pivotPosition], v[0]
	pivot := &v[0]
	var less int
	if e.typ.Size <= 96 {
		less = e.lomuto(v[1:], pivot, invert)
	} else {
		less = e.hoare(v[1:], pivot, invert)
	}
	v[0], v[less] = v[less], v[0]
	return less
}
func (e *engine[T]) partitionLess(a, b *T, invert bool) bool {
	if invert {
		return !e.less(b, a)
	}
	return e.less(a, b)
}
func (e *engine[T]) lomuto(v []T, pivot *T, invert bool) int {
	if len(v) == 0 {
		return 0
	}
	gapValue := v[0]
	gapPosition, less := 0, 0
	complete := false
	defer func() {
		if !complete {
			v[gapPosition] = gapValue
		}
	}()
	// The source unrolls this linear scan for small types. Its callback and
	// move order is identical; the final comparison uses the saved gap cell.
	for right := 1; right <= len(v); right++ {
		source := &gapValue
		if right < len(v) {
			source = &v[right]
		}
		isLess := e.partitionLess(source, pivot, invert)
		v[gapPosition] = v[less]
		v[less] = *source
		if right < len(v) {
			gapPosition = right
		}
		if isLess {
			less++
		}
	}
	complete = true
	return less
}
func (e *engine[T]) hoare(v []T, pivot *T, invert bool) int {
	if len(v) == 0 {
		return 0
	}
	left, right := 0, len(v)
	var gapValue T
	gapPosition, hasGap := 0, false
	defer func() {
		if hasGap {
			v[gapPosition] = gapValue
		}
	}()
	for {
		for left < right && e.partitionLess(&v[left], pivot, invert) {
			left++
		}
		for {
			right--
			if left >= right || e.partitionLess(&v[right], pivot, invert) {
				break
			}
		}
		if left >= right {
			break
		}
		first := !hasGap
		if first {
			gapValue = v[left]
			gapPosition = right
			hasGap = true
		}
		if !first {
			v[gapPosition] = v[left]
		}
		gapPosition = right
		v[left] = v[right]
		left++
	}
	return left
}
func (e *engine[T]) heap(v []T) {
	n := len(v)
	for i := n + n/2 - 1; i >= 0; i-- {
		sift := 0
		if i >= n {
			sift = i - n
		} else {
			v[0], v[i] = v[i], v[0]
		}
		e.siftDown(v[:min(i, n)], sift)
	}
}
func (e *engine[T]) siftDown(v []T, node int) {
	for {
		child := 2*node + 1
		if child >= len(v) {
			return
		}
		if child+1 < len(v) && e.less(&v[child], &v[child+1]) {
			child++
		}
		if !e.less(&v[node], &v[child]) {
			return
		}
		v[node], v[child] = v[child], v[node]
		node = child
	}
}
