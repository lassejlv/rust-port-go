// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

import "math/bits"

type driftRun struct {
	length int
	sorted bool
}

func (e *engine[T]) drift(v, scratch []T, eager bool) {
	n := len(v)
	if n < 2 {
		return
	}
	scale := (uint64(1) << 62) / uint64(n)
	if (uint64(1)<<62)%uint64(n) != 0 {
		scale++
	}
	minimum := min(n-n/2, 64)
	if n > 64*64 {
		log := bits.Len(uint(n)|1) - 1
		shift := (log + 1) / 2
		minimum = ((1 << shift) + (n >> shift)) / 2
	}
	var runs [66]driftRun
	var depths [66]int
	stackLength, scan := 0, 0
	previous := driftRun{sorted: true}
	for {
		next := driftRun{sorted: true}
		depth := 0
		if scan < n {
			next = e.createRun(v[scan:], scratch, minimum, eager)
			x := uint64(scan-previous.length) + uint64(scan)
			y := uint64(scan) + uint64(scan+next.length)
			depth = bits.LeadingZeros64(scale*x ^ scale*y)
		}
		for stackLength > 1 && depths[stackLength-1] >= depth {
			left := runs[stackLength-1]
			mergedLength := left.length + previous.length
			previous = e.logicalMerge(v[scan-mergedLength:scan], scratch, left, previous)
			stackLength--
		}
		runs[stackLength], depths[stackLength] = previous, depth
		stackLength++
		if scan >= n {
			break
		}
		scan += next.length
		previous = next
	}
	if !previous.sorted {
		e.stableQuick(v, scratch)
	}
}
func (e *engine[T]) logicalMerge(v, scratch []T, left, right driftRun) driftRun {
	if len(v) > len(scratch) || left.sorted || right.sorted {
		if !left.sorted {
			e.stableQuick(v[:left.length], scratch)
		}
		if !right.sorted {
			e.stableQuick(v[left.length:], scratch)
		}
		e.merge(v, scratch, left.length)
		return driftRun{len(v), true}
	}
	return driftRun{len(v), false}
}
func (e *engine[T]) createRun(v, scratch []T, minimum int, eager bool) driftRun {
	if len(v) >= minimum {
		n, reversed := e.run(v)
		if n >= minimum {
			if reversed {
				reverse(v[:n])
			}
			return driftRun{n, true}
		}
	}
	if eager {
		n := min(e.stableThreshold(), len(v))
		e.quick(v[:n], scratch, 0, nil)
		return driftRun{n, true}
	}
	return driftRun{min(minimum, len(v)), false}
}
func (e *engine[T]) stableQuick(v, scratch []T) {
	e.quick(v, scratch, 2*(bits.Len(uint(len(v))|1)-1), nil)
}
func (e *engine[T]) quick(v, scratch []T, limit int, ancestor *T) {
	for {
		if len(v) <= e.stableThreshold() {
			e.stableSmall(v, scratch)
			return
		}
		if limit == 0 {
			e.drift(v, scratch, true)
			return
		}
		limit--
		pivotPosition := e.pivot(v)
		pivotCopy := v[pivotPosition]
		var pivotReference *T
		if e.typ.Freeze {
			pivotReference = &pivotCopy
		}
		equalPartition := false
		if ancestor != nil {
			equalPartition = !e.less(ancestor, &v[pivotPosition])
		}
		leftLength := 0
		if !equalPartition {
			leftLength = e.partition(v, scratch, pivotPosition, false, false)
			equalPartition = leftLength == 0
		}
		if equalPartition {
			mid := e.partition(v, scratch, pivotPosition, true, true)
			v = v[mid:]
			ancestor = nil
			continue
		}
		e.quick(v[leftLength:], scratch, limit, pivotReference)
		v = v[:leftLength]
	}
}
func (e *engine[T]) partition(v, scratch []T, pivot int, pivotLeft, invert bool) int {
	n := len(v)
	if len(scratch) < n || pivot >= n {
		e.context.Panic("rustsort: invalid stable partition scratch or pivot")
	}
	leftCount, pivotScratch := 0, 0
	for scan := 0; scan < n; scan++ {
		towardsLeft := pivotLeft
		if scan != pivot {
			if invert {
				towardsLeft = !e.less(&v[pivot], &v[scan])
			} else {
				towardsLeft = e.less(&v[scan], &v[pivot])
			}
		}
		destination := leftCount
		if !towardsLeft {
			destination = n - 1 - scan + leftCount
		}
		scratch[destination] = v[scan]
		if scan == pivot {
			pivotScratch = destination
		}
		if towardsLeft {
			leftCount++
		}
	}
	if !e.typ.Freeze {
		scratch[pivotScratch] = v[pivot]
	}
	copy(v[:leftCount], scratch[:leftCount])
	for i := 0; i < n-leftCount; i++ {
		v[leftCount+i] = scratch[n-1-i]
	}
	return leftCount
}
