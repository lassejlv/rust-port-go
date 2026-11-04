// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: rustc-stable-hash 0.1.2 src/int_overflow.rs (official archive SHA
// 781442f29170c5c93b7185ad559492601acdc71d5bb0706f5868094f45cfcd08).
package stablehash

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// ArithmeticProfile describes the two independent Rust codegen switches used
// by the original private DebugStrictAdd/Sub implementations. The debug branch
// uses ordinary arithmetic, and consequently still obeys overflow-checks.
type ArithmeticProfile struct{ DebugAssertions, OverflowChecks bool }

func DefaultProfile() ArithmeticProfile { return ArithmeticProfile{true, true} }
func ReleaseProfile() ArithmeticProfile { return ArithmeticProfile{} }
func debugStrictAdd(a, b uint, p ArithmeticProfile, c rustfmt.Context) uint {
	if p.DebugAssertions && p.OverflowChecks && b > ^uint(0)-a {
		c.Panic("attempt to add with overflow")
	}
	return a + b
}
func debugStrictSub(a, b uint, p ArithmeticProfile, c rustfmt.Context) uint {
	if p.DebugAssertions && p.OverflowChecks && a < b {
		c.Panic("attempt to subtract with overflow")
	}
	return a - b
}
