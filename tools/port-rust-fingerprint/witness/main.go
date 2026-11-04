// SPDX-License-Identifier: MIT OR Apache-2.0
// Emit operations from the actual Go port for replay by real Rust hashers.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
)

type trace []string

func (value *trace) WriteU64(n uint64) { *value = append(*value, strconv.FormatUint(n, 10)) }
func main() {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 4 || fields[0] != "H" {
			panic("invalid fingerprint witness request")
		}
		a, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			panic(err)
		}
		b, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			panic(err)
		}
		value := datastructures.FingerprintFromU64(a, b)
		var hash, stable trace
		value.Hash(&hash)
		value.StableHash(struct{}{}, &stable)
		fmt.Printf("%s\t%s\t%s\n", scanner.Text(), strings.Join(hash, ","), strings.Join(stable, ","))
	}
	if err := scanner.Err(); err != nil {
		panic(err)
	}
}
