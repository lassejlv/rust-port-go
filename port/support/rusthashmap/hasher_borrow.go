// SPDX-License-Identifier: MIT OR Apache-2.0
// Additive access to the actual hasher returned by a source BuildHasher.
package rusthashmap

// Borrow returns a temporary exclusive source borrow of the actual live H.
// The caller must end the borrow before another incompatible borrow, move or
// Close; the Hasher interface must not escape that lifetime. It does not copy,
// move, finish or destroy H. The explicit current context controls rejection
// of a consumed owner, rather than an earlier ambient-context snapshot.
func (h *OwnedHasher) Borrow(context Context) Hasher {
	if h == nil || !h.live || h.value == nil {
		context.Panic("rusthashmap: borrow of consumed owned Hasher")
	}
	return h.value
}
