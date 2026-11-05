// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

func cleanupOnPanic(context Context, body func()) {
	defer func() {
		if recover() != nil {
			context.AbortCleanup()
		}
	}()
	body()
}
