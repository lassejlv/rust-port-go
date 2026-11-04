// Development-only original-source formatting inputs. MIT OR Apache-2.0.
package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
)

type nameFormatRequest struct {
	op                  byte
	name, what          string
	debug, chars, reuse bool
	pattern             byte
	fail                uint32
}

func nameFormatBaseRequests() []nameFormatRequest {
	values := []nameFormatRequest{
		{op: 2, name: "", what: ""},
		{op: 2, name: "", what: "custom `name`\n雪\x00"},
		{op: 2, name: "0", what: "name"},
		{op: 2, name: "\x00\"'\\\n", what: "\x00\"\n雪"},
		{op: 2, name: "a'", what: "name"},
		{op: 2, name: "a\"\\\n", what: "name"},
		{op: 2, name: "a\u0301?", what: "\u0301x"},
		{op: 2, name: "a😀", what: "name"},
		{op: 2, name: "\U0010ffff", what: "name"},
		{op: 3, name: "BUILD-OVERRIDE"},
		{op: 3, name: "CARGO"},
		{op: 3, name: "name\n雪"},
		{op: 4, name: "dep:\x00\"'\\\n😀"},
		{op: 4, name: "a/\x00\"\n"},
		{op: 0, name: "a::"},
		{op: 5, name: "a\t"},
	}
	var result []nameFormatRequest
	for _, value := range values {
		for pattern := byte(0); pattern < 19; pattern++ {
			for _, debug := range []bool{false, true} {
				for _, chars := range []bool{false, true} {
					value.pattern, value.debug, value.chars, value.reuse, value.fail = pattern, debug, chars, true, ^uint32(0)
					result = append(result, value)
				}
			}
		}
	}
	return result
}
func nameFormatEncode(requests []nameFormatRequest) []byte {
	var out bytes.Buffer
	out.WriteString("RNF1")
	write32(&out, uint32(len(requests)))
	bit := func(value bool) byte {
		if value {
			return 1
		}
		return 0
	}
	for _, r := range requests {
		out.WriteByte(r.op)
		text(&out, r.name)
		text(&out, r.what)
		out.Write([]byte{bit(r.debug), r.pattern, bit(r.chars), bit(r.reuse)})
		write32(&out, r.fail)
	}
	return out.Bytes()
}

// Successful original-Rust traces determine the finite writer failure cuts.
// No expected output or control-flow decision is taken from the Go port.
func nameFormatWriteCounts(data []byte, count int) ([]uint32, error) {
	r := bytes.NewReader(data)
	var header [8]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}
	if string(header[:4]) != "RNG1" || binary.LittleEndian.Uint32(header[4:]) != uint32(count) {
		return nil, fmt.Errorf("invalid original formatter response")
	}
	read32 := func() (uint32, error) {
		var b [4]byte
		_, e := io.ReadFull(r, b[:])
		return binary.LittleEndian.Uint32(b[:]), e
	}
	skipText := func() error {
		n, e := read32()
		if e != nil {
			return e
		}
		if n > 16<<20 {
			return fmt.Errorf("oversized formatter text")
		}
		_, e = io.CopyN(io.Discard, r, int64(n))
		return e
	}
	counts := make([]uint32, count)
	for i := range counts {
		// Three result flags and three complete captured formatter states.
		var fixed [75]byte
		if _, err := io.ReadFull(r, fixed[:]); err != nil {
			return nil, err
		}
		if fixed[0] != 1 || fixed[1] != 1 || fixed[2] != 1 {
			return nil, fmt.Errorf("successful formatter discovery failed at%d", i)
		}
		n, err := read32()
		if err != nil {
			return nil, err
		}
		if n > 4096 {
			return nil, fmt.Errorf("too many formatter writes")
		}
		counts[i] = n
		for j := uint32(0); j < n; j++ {
			if _, err := r.ReadByte(); err != nil {
				return nil, err
			}
			if err := skipText(); err != nil {
				return nil, err
			}
		}
		if err := skipText(); err != nil {
			return nil, err
		}
	}
	if r.Len() != 0 {
		return nil, fmt.Errorf("trailing original formatter response")
	}
	return counts, nil
}
func nameFormatCorpus(ctx context.Context, root string, env []string, binaryPath string) ([]byte, int, error) {
	base := nameFormatBaseRequests()
	answers, err := command(ctx, root, env, binaryPath, nameFormatEncode(base), "format")
	if err != nil {
		return nil, 0, err
	}
	counts, err := nameFormatWriteCounts(answers, len(base))
	if err != nil {
		return nil, 0, err
	}
	var all []nameFormatRequest
	for i, r := range base {
		all = append(all, r)
		for fail := uint32(0); fail <= counts[i]+1; fail++ {
			r.fail = fail
			all = append(all, r)
		}
	}
	requests := nameFormatEncode(all)
	answers, err = command(ctx, root, env, binaryPath, requests, "format")
	if err != nil {
		return nil, 0, err
	}
	var out bytes.Buffer
	out.WriteString("RNCF")
	write32(&out, uint32(len(requests)))
	out.Write(requests)
	out.Write(answers)
	return out.Bytes(), len(all), nil
}
