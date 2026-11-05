// SPDX-License-Identifier: MIT OR Apache-2.0
// Exact used unit declaration from rustc_data_structures/src/lib.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. No whole lib.rs port is claimed.
package datastructures

// FatalErrorMarker identifies the compiler's resume-unwind fatal payload.
// It deliberately has no Error or Debug implementation in the source.
type FatalErrorMarker struct{}
