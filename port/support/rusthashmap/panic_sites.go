// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// The caller's immutable selected SDK profile supplies the exact source file
// spelling and coordinates. Nil metadata does not invent a Rust location.
const PanicCapacityOverflowSite = "rusthashmap.capacity_overflow"

func capacityOverflow(c Context) {
	c.PanicAt(rustfmt.SourcePanicInfo{Payload: "Hash table capacity overflow", Kind: rustfmt.PanicStaticStr, Location: c.SourcePanicLocation(PanicCapacityOverflowSite), CanUnwind: true})
}
