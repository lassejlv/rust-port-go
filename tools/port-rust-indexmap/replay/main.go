// Development-only command wrapper; no production Rust fallback.
package main

import (
	"bufio"
	"encoding/json"
	"os"

	subject "github.com/lassejlv/rust-port-go/tools/port-rust-indexmap/subject"
)

func main() {
	scan := bufio.NewScanner(os.Stdin)
	scan.Buffer(make([]byte, 4096), 4<<20)
	enc := json.NewEncoder(os.Stdout)
	for scan.Scan() {
		var q subject.Request
		if err := json.Unmarshal(scan.Bytes(), &q); err != nil {
			panic(err)
		}
		raw := json.RawMessage(append([]byte(nil), scan.Bytes()...))
		if err := enc.Encode(map[string]any{"request": raw, "result": subject.Run(q)}); err != nil {
			panic(err)
		}
	}
	if err := scan.Err(); err != nil {
		panic(err)
	}
}
