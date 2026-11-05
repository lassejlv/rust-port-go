package main

import (
	"bufio"
	"encoding/json"
	subject "github.com/lassejlv/rust-port-go/tools/port-rust-indexmap/subject"
	"os"
)

func main() {
	s := bufio.NewScanner(os.Stdin)
	s.Buffer(make([]byte, 65536), 1<<20)
	e := json.NewEncoder(os.Stdout)
	for s.Scan() {
		raw := append(json.RawMessage{}, s.Bytes()...)
		var q subject.ProjectionRequest
		if err := json.Unmarshal(raw, &q); err != nil {
			panic(err)
		}
		if err := e.Encode(map[string]any{"request": raw, "result": subject.RunProjection(q)}); err != nil {
			panic(err)
		}
	}
	if err := s.Err(); err != nil {
		panic(err)
	}
}
