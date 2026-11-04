// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

func (e *engine[T]) run(v []T) (int, bool) {
	if len(v) < 2 {
		return len(v), false
	}
	n := 2
	descending := e.less(&v[1], &v[0])
	if descending {
		for n < len(v) && e.less(&v[n], &v[n-1]) {
			n++
		}
	} else {
		for n < len(v) && !e.less(&v[n], &v[n-1]) {
			n++
		}
	}
	return n, descending
}

func (e *engine[T]) pivot(v []T) int {
	n := len(v) / 8
	if len(v) < 64 {
		return e.median(v, 0, n*4, n*7)
	}
	return e.medianRecursive(v, 0, n*4, n*7, n)
}
func (e *engine[T]) medianRecursive(v []T, a, b, c, n int) int {
	if n*8 >= 64 {
		n8 := n / 8
		a = e.medianRecursive(v, a, a+n8*4, a+n8*7, n8)
		b = e.medianRecursive(v, b, b+n8*4, b+n8*7, n8)
		c = e.medianRecursive(v, c, c+n8*4, c+n8*7, n8)
	}
	return e.median(v, a, b, c)
}
func (e *engine[T]) median(v []T, a, b, c int) int {
	x := e.less(&v[a], &v[b])
	y := e.less(&v[a], &v[c])
	if x == y {
		z := e.less(&v[b], &v[c])
		if z != x {
			return c
		}
		return b
	}
	return a
}

func (e *engine[T]) insertTail(v []T, tail int) {
	sift := tail - 1
	if !e.less(&v[tail], &v[sift]) {
		return
	}
	temporary := v[tail]
	gap := tail
	defer func() { v[gap] = temporary }()
	for {
		v[gap] = v[sift]
		gap = sift
		if sift == 0 {
			break
		}
		sift--
		if !e.less(&temporary, &v[sift]) {
			break
		}
	}
}
func (e *engine[T]) insertion(v []T, offset int) {
	for tail := offset; tail < len(v); tail++ {
		e.insertTail(v, tail)
	}
}

func (e *engine[T]) sort4(v, dst []T) {
	c1 := e.less(&v[1], &v[0])
	c2 := e.less(&v[3], &v[2])
	a, b, c, d := 0, 1, 2, 3
	if c1 {
		a, b = 1, 0
	}
	if c2 {
		c, d = 3, 2
	}
	c3 := e.less(&v[c], &v[a])
	c4 := e.less(&v[d], &v[b])
	minimum, maximum := a, d
	if c3 {
		minimum = c
	}
	if c4 {
		maximum = b
	}
	unknownLeft, unknownRight := b, c
	if c4 {
		unknownLeft, unknownRight = c, d
	}
	if c3 {
		unknownLeft = a
		if !c4 {
			unknownRight = b
		}
	}
	c5 := e.less(&v[unknownRight], &v[unknownLeft])
	lo, hi := unknownLeft, unknownRight
	if c5 {
		lo, hi = unknownRight, unknownLeft
	}
	dst[0], dst[1], dst[2], dst[3] = v[minimum], v[lo], v[hi], v[maximum]
}
func (e *engine[T]) sort8(v, dst, scratch []T) {
	e.sort4(v, scratch)
	e.sort4(v[4:], scratch[4:])
	e.bidirectional(scratch[:8], dst)
}

func (e *engine[T]) smallGeneral(v, scratch []T) {
	n := len(v)
	if n < 2 {
		return
	}
	if len(scratch) < n+16 {
		e.context.Panic("rustsort: insufficient small-sort scratch")
	}
	mid := n / 2
	presorted := 1
	if e.typ.Size <= 16 && n >= 16 {
		e.sort8(v, scratch, scratch[n:n+8])
		e.sort8(v[mid:], scratch[mid:], scratch[n+8:n+16])
		presorted = 8
	} else if n >= 8 {
		e.sort4(v, scratch)
		e.sort4(v[mid:], scratch[mid:])
		presorted = 4
	} else {
		scratch[0], scratch[mid] = v[0], v[mid]
	}
	for _, offset := range [2]int{0, mid} {
		desired := mid
		if offset != 0 {
			desired = n - mid
		}
		for i := presorted; i < desired; i++ {
			scratch[offset+i] = v[offset+i]
			e.insertTail(scratch[offset:offset+desired], i)
		}
	}
	complete := false
	defer func() {
		if !complete {
			copy(v, scratch[:n])
		}
	}()
	e.bidirectional(scratch[:n], v)
	complete = true
}

func (e *engine[T]) bidirectional(v, dst []T) {
	n, mid := len(v), len(v)/2
	left, right, out := 0, mid, 0
	leftReverse, rightReverse, outReverse := mid-1, n-1, n-1
	for i := 0; i < mid; i++ {
		isLeft := !e.less(&v[right], &v[left])
		if isLeft {
			dst[out] = v[left]
			left++
		} else {
			dst[out] = v[right]
			right++
		}
		out++
		isLeft = !e.less(&v[rightReverse], &v[leftReverse])
		if isLeft {
			dst[outReverse] = v[rightReverse]
			rightReverse--
		} else {
			dst[outReverse] = v[leftReverse]
			leftReverse--
		}
		outReverse--
	}
	leftEnd, rightEnd := leftReverse+1, rightReverse+1
	if n%2 != 0 {
		if left < leftEnd {
			dst[out] = v[left]
			left++
		} else {
			dst[out] = v[right]
			right++
		}
	}
	if left != leftEnd || right != rightEnd {
		e.context.Panic("user-provided comparison function does not correctly implement a total order")
	}
}
