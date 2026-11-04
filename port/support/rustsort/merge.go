// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

func (e *engine[T]) merge(v, scratch []T, mid int) {
	n := len(v)
	if mid == 0 || mid >= n || len(scratch) < min(mid, n-mid) {
		return
	}
	leftShorter := mid <= n-mid
	saveBase, saveLength := 0, mid
	if !leftShorter {
		saveBase, saveLength = mid, n-mid
	}
	copy(scratch[:saveLength], v[saveBase:saveBase+saveLength])
	start, end, destination := 0, saveLength, saveBase
	defer func() { copy(v[destination:destination+end-start], scratch[start:end]) }()
	if leftShorter {
		right := mid
		for start != end && right != n {
			consumeLeft := !e.less(&v[right], &scratch[start])
			if consumeLeft {
				v[destination] = scratch[start]
				start++
			} else {
				v[destination] = v[right]
				right++
			}
			destination++
		}
	} else {
		out := n
		for {
			left, right := destination-1, end-1
			out--
			consumeLeft := e.less(&scratch[right], &v[left])
			if consumeLeft {
				v[out] = v[left]
				destination = left
			} else {
				v[out] = scratch[right]
				end = right
			}
			if destination == 0 || end == 0 {
				break
			}
		}
	}
}
