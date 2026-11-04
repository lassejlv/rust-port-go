// SPDX-License-Identifier: MIT OR Apache-2.0
package vec_test

import "fmt"

func storageIteratorRequests() []storageRequest {
	var out []storageRequest
	scripts := [][]string{
		{"next", "hint", "nth:0", "nth_back:1", "next", "back", "hint", "close"},
		{"nth:2", "hint", "back", "count"},
		{"nth_back:2", "hint", "next", "last"},
		{"count"}, {"last"},
		{"catch_nth:2", "hint", "next", "back", "count"},
		{"catch_nth_back:2", "hint", "next", "back", "last"},
	}
	for _, checked := range []bool{false, true} {
		for _, debug := range []bool{false, true} {
			for length := 0; length <= 6; length++ {
				for _, op := range []string{"iter", "iter_mut", "iter_enum", "iter_enum_mut", "indices", "owned", "owned_enum", "drain", "drain_enum"} {
					for script, commands := range scripts {
						for _, failure := range []string{"", "new", "drop"} {
							for nth := uint(1); nth <= 4; nth++ {
								if failure == "" && nth != 1 {
									continue
								}
								values := make([]int64, length)
								for i := range values {
									values[i] = int64(i*2 + 1)
								}
								out = append(out, storageRequest{Name: fmt.Sprintf("iterator-%s-%d-%d-%s-%d", op, length, script, failure, nth), Checked: checked, Debug: debug, Values: values, Other: []int64{}, Capacity: uint(length + 4), Limit: 64, Actions: []storageAction{{Op: op, A: 0, B: uint(length), StartKind: 1, EndKind: 2, Panic: failure, PanicAt: nth, Commands: commands}}})
							}
						}
					}
				}
			}
		}
	}
	return out
}
