package main

import (
	"bufio"
	"encoding/json"
	subject "github.com/lassejlv/rust-port-go/tools/port-rust-indexmap/subject"
	"os"
)

func main() {
	s := bufio.NewScanner(os.Stdin)
	e := json.NewEncoder(os.Stdout)
	for s.Scan() {
		var raw json.RawMessage
		raw = append(raw, s.Bytes()...)
		var q subject.OwnedRequest
		if err := json.Unmarshal(raw, &q); err != nil {
			panic(err)
		}
		if err := e.Encode(map[string]any{"request": raw, "result": subject.RunOwned(q)}); err != nil {
			panic(err)
		}
	}
	if err := s.Err(); err != nil {
		panic(err)
	}
}
