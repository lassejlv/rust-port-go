// SPDX-License-Identifier: MIT OR Apache-2.0
// Additive scoped access to the actual bucket identity of the existing iterator.
package rusthashmap

// NextBucket preserves Next's current-context validation and cursor/terminal
// behavior while returning the actual yielded bucket and cell. Its borrow has
// exactly Next's lifetime: exhaust/end the iterator and all yielded borrows
// before an incompatible table mutation, move, relocation or destruction.
// Numeric bucket identities alone grant no retained mutable cell borrow.
func (i *RawIter[T]) NextBucket(context Context) (uint, *T, bool) {
	value, ok := i.Next(context)
	if !ok {
		return 0, nil, false
	}
	return i.next - 1, value, true
}
