package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	span "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
)

type hashOperations struct{ values []int }

func (operations *hashOperations) WriteIsize(value int) {
	operations.values = append(operations.values, value)
}

type stableOperations struct{ values []uint8 }

func (operations *stableOperations) WriteU8(value uint8) {
	operations.values = append(operations.values, value)
}

func editionRequests() ([]byte, error) {
	var requests bytes.Buffer
	for _, text := range []string{"2015", "2018", "2021", "2024", "future", "", "2014", "2016", "2027", "Future", "FUTURE", " 2015", "2015 ", "02015", "+2015", "-2015", "2015\x00", "２０１５", "٢٠١٥", "future\n", "Edition2015", "2015.0"} {
		fmt.Fprintf(&requests, "parse\t%s\n", hex.EncodeToString([]byte(text)))
	}
	seed := uint64(0x79c51d3e88a02f64)
	next := func() uint64 { seed ^= seed << 13; seed ^= seed >> 7; seed ^= seed << 17; return seed }
	alphabet := []rune("201584fuetr FUTURE.+-\x00\n\t🦀é٢٠١٥２０１５")
	for sample := 0; sample < 2048; sample++ {
		var text strings.Builder
		for length := int(next() % 13); length > 0; length-- {
			text.WriteRune(alphabet[next()%uint64(len(alphabet))])
		}
		fmt.Fprintf(&requests, "parse\t%s\n", hex.EncodeToString([]byte(text.String())))
	}
	for index, edition := range span.AllEditions() {
		standard := &hashOperations{}
		stable := &stableOperations{}
		edition.Hash(standard)
		edition.StableHash(struct{}{}, stable)
		if len(standard.values) != 1 || len(stable.values) != 1 {
			return nil, fmt.Errorf("Go edition %s emitted wrong hash-operation count", edition.AsStr())
		}
		for length := 0; length <= 128; length++ {
			prefix := make([]byte, length)
			for i := range prefix {
				prefix[i] = byte(next())
			}
			fmt.Fprintf(&requests, "hash\t%d\t%s\t%d\t%d\n", index, hex.EncodeToString(prefix), standard.values[0], stable.values[0])
		}
	}
	decode := func(data []byte, position uint) {
		fmt.Fprintf(&requests, "decode\t%s\t%d\n", hex.EncodeToString(data), position)
	}
	for tag := 0; tag < 256; tag++ {
		decode(append([]byte{byte(tag)}, []byte(span.EditionBlobEndBytes)...), 0)
	}
	for _, bytes := range [][]byte{nil, {}, []byte("rust-end-fil"), []byte("rust-end-file"), {0}, {0, 1, 2, 3, 4}, {99, 0, 1, 2, 3, 4}} {
		for position := uint(0); position <= uint(len(bytes))+1; position++ {
			decode(bytes, position)
			framed := append(append([]byte{}, bytes...), []byte(span.EditionBlobEndBytes)...)
			decode(framed, position)
		}
	}
	for _, edition := range span.AllEditions() {
		// Exercise every decoder starting position inside a mixed valid stream.
		encoder := span.NewEditionMemoryEncoder()
		encoder.EmitU8(byte(edition))
		for _, entry := range span.AllEditions() {
			entry.Encode(encoder)
		}
		data := append(encoder.Finish(), []byte(span.EditionBlobEndBytes)...)
		for position := uint(0); position <= 6; position++ {
			decode(data, position)
		}
	}
	return requests.Bytes(), nil
}

func countEditionRecords(data []byte) map[string]int {
	counts := make(map[string]int)
	for _, line := range bytes.Split(bytes.TrimSuffix(data, []byte{'\n'}), []byte{'\n'}) {
		kind, _, _ := strings.Cut(string(line), "\t")
		counts[kind]++
	}
	return counts
}

// Keep strconv in the generator's dependency surface to bind host-sized inputs
// to the reference profile instead of silently accepting a different width.
const editionHostBits = strconv.IntSize
