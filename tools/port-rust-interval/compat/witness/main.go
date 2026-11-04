// SPDX-License-Identifier: MIT OR Apache-2.0
// Isolated development child for component-local source abort comparisons.
package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	probe "github.com/lassejlv/rust-port-go/tools/port-rust-interval/compat/validation"
)

func main() {
	var request probe.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		panic(err)
	}
	result := probe.Replay(request, func(event string) { fmt.Println("event:" + hex.EncodeToString([]byte(event))) })
	data, err := json.Marshal(result)
	if err != nil {
		panic(err)
	}
	fmt.Println("return:" + string(data))
}
