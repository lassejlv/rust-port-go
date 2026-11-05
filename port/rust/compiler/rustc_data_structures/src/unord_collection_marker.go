// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"reflect"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// A distinct Go embedding wrapper promotes private methods too. Returning the
// actual collection receiver lets the source's private marker require exact
// nominal Self, while genuine Go aliases retain the same type. Comparing type
// descriptors avoids comparing arbitrary, possibly noncomparable wrapper data.
func (m *UnordMap[K, V]) unordCollectionOwner() any { return m }
func (s *UnordSet[T]) unordCollectionOwner() any    { return s }
func (b *UnordBag[T]) unordCollectionOwner() any    { return b }

func unordRequireExactCollection[C UnordCollection](collection C, c rustfmt.Context) {
	// A nil outer embedding pointer would otherwise dereference its promoted
	// receiver before we can reject its distinct Self. A nil actual collection
	// is not a valid source borrow either; neither case may consume the input.
	value := reflect.ValueOf(collection)
	if value.IsValid() && value.Kind() == reflect.Pointer && value.IsNil() {
		c.Panic("datastructures: source UnordCollection requires exact collection Self")
	}
	owner, ok := any(collection).(interface{ unordCollectionOwner() any })
	if !ok || reflect.TypeOf(collection) != reflect.TypeOf(owner.unordCollectionOwner()) {
		c.Panic("datastructures: source UnordCollection requires exact collection Self")
	}
}
