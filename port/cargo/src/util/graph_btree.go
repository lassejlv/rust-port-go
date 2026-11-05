// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped std BTreeMap/BTreeSet operations used by Cargo graph traversals.
// Source/reference binding is the Rust 1.99.1 allocation-collection profile;
// no complete Rust standard-library source-file credit is claimed here.
package util

import (
	imrc "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type graphBTreeOps[K, V any] struct {
	compare   func(K, K, rustfmt.Context) int
	moveKey   func(K, rustfmt.Context) K
	dropKey   func(K, rustfmt.Context)
	dropValue func(V, rustfmt.Context)
}
type graphBTreeEntry[K, V any] struct {
	key   K
	value V
}
type graphBTreeNode[K, V any] struct {
	entries     []*graphBTreeEntry[K, V]
	children    []*graphBTreeNode[K, V]
	parent      *graphBTreeNode[K, V]
	parentIndex int
}
type graphBTree[K, V any] struct {
	root *graphBTreeNode[K, V]
	ops  graphBTreeOps[K, V]
	size uint
}

func (tree *graphBTree[K, V]) search(key K, ctx rustfmt.Context) (*graphBTreeNode[K, V], int, bool) {
	n := tree.root
	if n == nil {
		return nil, 0, false
	}
	for {
		index := 0
		for index < len(n.entries) {
			c := tree.ops.compare(key, n.entries[index].key, ctx)
			if c == 0 {
				return n, index, true
			}
			if c < 0 {
				break
			}
			index++
		}
		if len(n.children) == 0 {
			return n, index, false
		}
		n = n.children[index]
	}
}
func graphBTreeFixChildren[K, V any](n *graphBTreeNode[K, V]) {
	for i, child := range n.children {
		child.parent = n
		child.parentIndex = i
	}
}
func graphBTreeInsertAt[T any](items []T, index int, value T) []T {
	var zero T
	items = append(items, zero)
	copy(items[index+1:], items[index:])
	items[index] = value
	return items
}
func graphBTreeRemoveAt[T any](items []T, index int) []T {
	copy(items[index:], items[index+1:])
	var zero T
	items[len(items)-1] = zero
	return items[:len(items)-1]
}
func (tree *graphBTree[K, V]) insertAt(n *graphBTreeNode[K, V], index int, entry *graphBTreeEntry[K, V]) {
	if n == nil {
		tree.root = &graphBTreeNode[K, V]{entries: []*graphBTreeEntry[K, V]{entry}}
		tree.size++
		return
	}
	var right *graphBTreeNode[K, V]
	for {
		// node::splitpoint chooses the promoted old KV according to insertion
		// edge, before insertion. In the combined 12-entry list that is 5 or6.
		full := len(n.entries) == 11
		median := 0
		if full {
			if index < 5 || index == 6 {
				median = 5
			} else {
				median = 6
			}
		}
		n.entries = graphBTreeInsertAt(n.entries, index, entry)
		if right != nil {
			n.children = graphBTreeInsertAt(n.children, index+1, right)
			graphBTreeFixChildren(n)
		}
		if !full {
			tree.size++
			return
		}
		promoted := n.entries[median]
		right = &graphBTreeNode[K, V]{entries: append([]*graphBTreeEntry[K, V](nil), n.entries[median+1:]...)}
		n.entries = n.entries[:median]
		if len(n.children) != 0 {
			right.children = append([]*graphBTreeNode[K, V](nil), n.children[median+1:]...)
			n.children = n.children[:median+1]
			graphBTreeFixChildren(n)
			graphBTreeFixChildren(right)
		}
		parent := n.parent
		if parent == nil {
			tree.root = &graphBTreeNode[K, V]{entries: []*graphBTreeEntry[K, V]{promoted}, children: []*graphBTreeNode[K, V]{n, right}}
			graphBTreeFixChildren(tree.root)
			tree.size++
			return
		}
		entry, index, n = promoted, n.parentIndex, parent
	}
}
func (tree *graphBTree[K, V]) entryOrInsert(key K, makeValue func() V, ctx rustfmt.Context) (result *V, added bool) {
	if tree.ops.moveKey != nil {
		key = tree.ops.moveKey(key, ctx)
	}
	remaining := true
	imrc.Scope(ctx, func() {
		n, index, found := tree.search(key, ctx)
		if found {
			result = &n.entries[index].value
			return
		}
		value := makeValue()
		entry := &graphBTreeEntry[K, V]{key: key, value: value}
		tree.insertAt(n, index, entry)
		remaining = false
		added = true
		result = &entry.value
	}, func(ctx rustfmt.Context) {
		if remaining && tree.ops.dropKey != nil {
			tree.ops.dropKey(key, ctx)
		}
	})
	return
}
func (tree *graphBTree[K, V]) insertSet(key K, ctx rustfmt.Context) bool {
	_, added := tree.entryOrInsert(key, func() (value V) { return }, ctx)
	return added
}
func (tree *graphBTree[K, V]) lookup(key K, ctx rustfmt.Context) (V, bool) {
	n, i, found := tree.search(key, ctx)
	if found {
		return n.entries[i].value, true
	}
	var zero V
	return zero, false
}
func (tree *graphBTree[K, V]) contains(key K, ctx rustfmt.Context) bool {
	_, _, found := tree.search(key, ctx)
	return found
}

func (tree *graphBTree[K, V]) merge(parent *graphBTreeNode[K, V], pivot int) {
	left, right := parent.children[pivot], parent.children[pivot+1]
	left.entries = append(left.entries, parent.entries[pivot])
	left.entries = append(left.entries, right.entries...)
	if len(right.children) != 0 {
		left.children = append(left.children, right.children...)
		graphBTreeFixChildren(left)
	}
	parent.entries = graphBTreeRemoveAt(parent.entries, pivot)
	parent.children = graphBTreeRemoveAt(parent.children, pivot+1)
	graphBTreeFixChildren(parent)
	right.entries = nil
	right.children = nil
	right.parent = nil
}
func (tree *graphBTree[K, V]) stealLeft(parent *graphBTreeNode[K, V], pivot int) {
	left, right := parent.children[pivot], parent.children[pivot+1]
	right.entries = graphBTreeInsertAt(right.entries, 0, parent.entries[pivot])
	last := len(left.entries) - 1
	parent.entries[pivot] = left.entries[last]
	left.entries = left.entries[:last]
	if len(left.children) != 0 {
		end := len(left.children) - 1
		right.children = graphBTreeInsertAt(right.children, 0, left.children[end])
		left.children = left.children[:end]
		graphBTreeFixChildren(left)
		graphBTreeFixChildren(right)
	}
}
func (tree *graphBTree[K, V]) stealRight(parent *graphBTreeNode[K, V], pivot int) {
	left, right := parent.children[pivot], parent.children[pivot+1]
	left.entries = append(left.entries, parent.entries[pivot])
	parent.entries[pivot] = right.entries[0]
	right.entries = graphBTreeRemoveAt(right.entries, 0)
	if len(right.children) != 0 {
		left.children = append(left.children, right.children[0])
		right.children = graphBTreeRemoveAt(right.children, 0)
		graphBTreeFixChildren(left)
		graphBTreeFixChildren(right)
	}
}
func (tree *graphBTree[K, V]) rebalance(n *graphBTreeNode[K, V]) {
	for len(n.entries) < 5 {
		parent := n.parent
		if parent == nil {
			if len(n.entries) == 0 && len(n.children) != 0 {
				tree.root = n.children[0]
				tree.root.parent = nil
				tree.root.parentIndex = 0
			}
			return
		}
		pivot := n.parentIndex - 1
		if pivot >= 0 {
			if len(parent.children[pivot].entries)+1+len(n.entries) <= 11 {
				tree.merge(parent, pivot)
				n = parent
				continue
			}
			for len(n.entries) < 5 {
				tree.stealLeft(parent, pivot)
			}
			return
		}
		pivot = 0
		if len(n.entries)+1+len(parent.children[1].entries) <= 11 {
			tree.merge(parent, pivot)
			n = parent
			continue
		}
		for len(n.entries) < 5 {
			tree.stealRight(parent, pivot)
		}
		return
	}
}
func (tree *graphBTree[K, V]) remove(key K, ctx rustfmt.Context) (value V, found bool) {
	n, index, found := tree.search(key, ctx)
	if !found {
		return value, false
	}
	target := n.entries[index]
	oldKey, oldValue := target.key, target.value
	if len(n.children) == 0 {
		n.entries = graphBTreeRemoveAt(n.entries, index)
		tree.rebalance(n)
	} else {
		// remove_internal_kv removes the predecessor leaf KV and performs all
		// repairs before replacing the target at its possibly moved position.
		leaf := n.children[index]
		for len(leaf.children) != 0 {
			leaf = leaf.children[len(leaf.children)-1]
		}
		last := len(leaf.entries) - 1
		predecessor := leaf.entries[last]
		leaf.entries = leaf.entries[:last]
		tree.rebalance(leaf)
		target.key, target.value = predecessor.key, predecessor.value
	}
	tree.size--
	if tree.ops.dropKey != nil {
		tree.ops.dropKey(oldKey, ctx)
	}
	return oldValue, true
}
func graphBTreeOrdered[K, V any](n *graphBTreeNode[K, V], out *[]*graphBTreeEntry[K, V]) {
	if n == nil {
		return
	}
	for i, entry := range n.entries {
		if len(n.children) != 0 {
			graphBTreeOrdered(n.children[i], out)
		}
		*out = append(*out, entry)
	}
	if len(n.children) != 0 {
		graphBTreeOrdered(n.children[len(n.entries)], out)
	}
}
func (tree *graphBTree[K, V]) ordered() []*graphBTreeEntry[K, V] {
	var out []*graphBTreeEntry[K, V]
	graphBTreeOrdered(tree.root, &out)
	return out
}
func (tree *graphBTree[K, V]) close(ctx rustfmt.Context) {
	entries := tree.ordered()
	tree.root = nil
	tree.size = 0
	imrc.DropSequence(entries, ctx, func(entry *graphBTreeEntry[K, V], ctx rustfmt.Context) {
		imrc.Scope(ctx, func() {
			if tree.ops.dropKey != nil {
				tree.ops.dropKey(entry.key, ctx)
			}
		}, func(ctx rustfmt.Context) {
			if tree.ops.dropValue != nil {
				tree.ops.dropValue(entry.value, ctx)
			}
		})
	})
}
