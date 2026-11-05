// SPDX-License-Identifier: MIT OR Apache-2.0
package rustvec

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

const PanicCapacityOverflowSite = "rustvec.capacity_overflow"

func capacityOverflow(context Context) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: "capacity overflow", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(PanicCapacityOverflowSite), CanUnwind: true})
}
