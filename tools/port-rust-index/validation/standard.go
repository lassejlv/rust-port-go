//go:build !ferrugo_rustc_nightly

// SPDX-License-Identifier: MIT OR Apache-2.0
package indexoracle

import index "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"

const NightlyEnabled = false

func nightlyConvert[K any](adapter index.Idx[K], a, b K, operation string) index.TypedSliceIndexResult {
	panic("nightly conversion unavailable without ferrugo_rustc_nightly tag")
}
