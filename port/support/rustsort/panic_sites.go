// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

const PanicOrderViolationSite = "rustsort.order_violation"

func orderViolation(context Context) {
	context.PanicAt(rustfmt.SourcePanicInfo{Payload: "user-provided comparison function does not correctly implement a total order", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(PanicOrderViolationSite), CanUnwind: true})
}
