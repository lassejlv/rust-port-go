// SPDX-License-Identifier: MIT OR Apache-2.0
// Additive access to the existing source HashTable insert_unique operation.
package rusthashmap

// InsertUnique transfers the value to the table without equality checks. It
// preserves insert_unique's actual vacant/deleted-slot growth decision and
// destroys the uninserted value if a source rehash callback panics. The returned
// mutable cell is a temporary source borrow, invalid across incompatible table
// mutation, relocation, move or Drop; its bucket index has the same lifetime.
func (t *RawTable[T]) InsertUnique(hash uint64, value T, context Context) (uint, *T) {
	return t.insertUnique(hash, value, context)
}
