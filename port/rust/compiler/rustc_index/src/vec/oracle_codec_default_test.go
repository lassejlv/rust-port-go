//go:build !ferrugo_rustc_nightly

// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

const storageCodecEnabled = false

func storageReplayCodec(storageRequest) map[string]any {
	panic("nightly storage codec unavailable in this Go build")
}
