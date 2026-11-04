// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import "os"

func cleanupOnPanic(body func()) {
	defer func() {
		if recover() != nil {
			os.Exit(134)
		}
	}()
	body()
}
