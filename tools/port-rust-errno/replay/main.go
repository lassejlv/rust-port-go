// SPDX-License-Identifier: MIT OR Apache-2.0
package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type sink struct {
	out   strings.Builder
	calls []string
	fail  int
}

func (s *sink) Write(p []byte) (int, error) { return s.WriteString(string(p)) }
func (s *sink) WriteString(value string) (int, error) {
	s.calls = append(s.calls, "S:"+hex.EncodeToString([]byte(value)))
	if len(s.calls) == s.fail {
		return 0, errors.New("sink failure")
	}
	return s.out.WriteString(value)
}
func (s *sink) WriteChar(value rune) error {
	s.calls = append(s.calls, fmt.Sprintf("C:%x", value))
	if len(s.calls) == s.fail {
		return errors.New("sink failure")
	}
	_, err := s.out.WriteRune(value)
	return err
}
func main() {
	if !rusterrno.NativeAvailable() {
		panic("native errno proof requires the actual host cgo binding")
	}
	scan := bufio.NewScanner(os.Stdin)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		fail, _ := strconv.Atoi(p[3])
		s := &sink{fail: fail}
		f := &rustfmt.Formatter{Writer: s, Options: rustfmt.DefaultOptions()}
		var err error
		if p[1] == "errno" {
			n, _ := strconv.ParseInt(p[2], 10, 32)
			err = rusterrno.FormatError(int32(n), f, nil)
			if err == nil {
				err = f.WriteString(" (os error ")
			}
			if err == nil {
				err = rustfmt.FormatDebugInt(n, 32, f)
			}
			if err == nil {
				err = f.WriteString(")")
			}
		} else {
			bytes, e := hex.DecodeString(p[2])
			if e != nil {
				panic(e)
			}
			err = rusterrno.FormatCStrBytes(bytes, f)
		}
		fmt.Fprintf(out, "%s\t%t\t%s\t%s\n", p[0], err == nil, hex.EncodeToString([]byte(s.out.String())), strings.Join(s.calls, ","))
	}
	if err := scan.Err(); err != nil {
		panic(err)
	}
}
