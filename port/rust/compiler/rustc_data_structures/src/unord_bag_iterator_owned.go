// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// UnordBagExtendIntoIterator is the ordinary Extend primary with an explicitly
// owned actual IntoIterator Self. Selected is Vec::extend::<S>, including its
// real IntoIterator conversion and concrete specialization. It may take, move
// or deliberately forget S; otherwise the source parameter is destroyed at
// this boundary. It must preserve prepared iterator/result leaks and current
// call metadata. The existing IndexIntoIterator-based Extend is a convenience.
func UnordBagExtendIntoIterator[S, T any](bag *UnordBag[T], input *rustvec.Owned[S], selected UnordBagExtendIteratorOps[S, T], contexts ...rustfmt.Context) {
	c := unordContext(contexts)
	bag.check(c)
	rustvec.ConsumeValue(input, c, func(input *rustvec.Owned[S]) struct{} {
		if selected.Extend == nil {
			c.Panic("datastructures: missing selected Vec::extend implementation")
		}
		selected.Extend(bag.inner, input, c)
		return struct{}{}
	})
}
