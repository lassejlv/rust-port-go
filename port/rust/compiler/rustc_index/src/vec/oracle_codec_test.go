// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import "fmt"

func storageCodecRequests() []storageRequest {
	var out []storageRequest
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for count := 0; count < 7; count++ {
				for _, op := range []string{"encode", "encode_override", "decode", "decode_override", "extend_one", "extend_reserve"} {
					for _, fail := range []string{"", "emit_len", "encode", "emit_value", "encode_slice", "read_len", "decode", "read_value", "decode_vec", "drop"} {
						for nth := uint(1); nth <= 2; nth++ {
							values := make([]int64, count)
							for i := range values {
								values[i] = int64(i + 10)
							}
							out = append(out, storageRequest{Kind: "codec", Name: fmt.Sprintf("codec-%s-%d-%s-%d", op, count, fail, nth), Nightly: true, Checked: checked, Debug: debug, Values: []int64{1, 3}, Other: []int64{}, Capacity: 2, Limit: 0, Actions: []storageAction{{Op: op, N: uint(count), Value: 17, Values: values, Panic: fail, PanicAt: nth}}})
						}
					}
				}
			}
		}
	}
	return out
}
