// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"encoding/hex"
	"fmt"
	"strings"
	"testing"

	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
)

func hygieneInvalidCodecInput(session *sp.SessionGlobals, n int, override bool) (int, []hygieneCodecAtom) {
	if n < 10 {
		return []int{0, 3, 6, 9, 23}[n/2], []hygieneCodecAtom{{"u8", func() uint8 {
			if n%2 != 0 {
				return 255
			}
			return []uint8{3, 3, 3, 13, 4}[n/2]
		}()}}
	}
	if n < 12 {
		return 47, []hygieneCodecAtom{{"u32", []uint32{0x20000000, ^uint32(0)}[n-10]}}
	}
	if n < 14 {
		crate := uint32(1)
		if n == 13 {
			crate = 0xffffff00
		}
		return 50, []hygieneCodecAtom{{"expn", sp.ExpnId{Krate: sp.CrateNumFromU32(crate), LocalID: sp.ExpnIndexFromU32(17)}}}
	}
	if n < 28 {
		c := &hygieneCodec{session: session, fail: -1}
		var e sp.SpanEncoder = c
		if override {
			e = hygieneBoolCodec{c}
		}
		encodeHygieneCodec(session, 55, e)
		a := c.atoms
		switch {
		case n <= 15:
			v := uint8(2)
			if n == 15 {
				v = 255
			}
			a[7] = hygieneCodecAtom{"u8", v}
		case n <= 17:
			v := uint8(5)
			if n == 17 {
				v = 255
			}
			a[11] = hygieneCodecAtom{"u8", v}
		case n <= 19:
			v := uint8(2)
			if n == 19 {
				v = 255
			}
			a[12] = hygieneCodecAtom{"u8", v}
		case n <= 21:
			v := uint8(2)
			if n == 21 {
				v = 255
			}
			a[13] = hygieneCodecAtom{"u8", v}
		case n <= 23:
			v := ^uint(0)
			if n == 23 {
				v = uint(^uint(0)>>1)/4 + 1
			}
			a[8] = hygieneCodecAtom{"usize", v}
		case n <= 25:
			v := uint32(1)
			if n == 25 {
				v = ^uint32(0)
			}
			a[5] = hygieneCodecAtom{"u32", v}
		case n <= 27:
			if override {
				a[15] = hygieneCodecAtom{"bool", true}
			} else {
				v := uint8(2)
				if n == 27 {
					v = 255
				}
				a[15] = hygieneCodecAtom{"u8", v}
			}
		}
		return 55, a
	}
	if n < 30 {
		value := hygieneCodecAtom{"bool", true}
		if !override {
			v := uint8(2)
			if n == 29 {
				v = 255
			}
			value = hygieneCodecAtom{"u8", v}
		}
		return 20, []hygieneCodecAtom{{"u8", uint8(11)}, value}
	}
	if n == 30 {
		return 47, []hygieneCodecAtom{{"u32", uint32(0x1fffffff)}}
	}
	return 9, []hygieneCodecAtom{{"u8", uint8(12)}}
}
func TestOriginalHygieneInvalidCodec(t *testing.T) {
	session := sp.NewSessionGlobals(sp.DefaultEdition, nil, sp.None[sp.SourceMapInputs]())
	defer session.Close()
	readHygieneOracle(t, "invalid_codec", func(row hygieneOracleRow) string {
		ty, atoms := hygieneInvalidCodecInput(session, row.N, row.Mode != 0)
		c := &hygieneCodec{session: session, atoms: atoms, fail: -1}
		var d sp.SpanDecoder = c
		if row.Mode != 0 {
			d = hygieneBoolCodec{c}
		}
		value, message := "", ""
		func() {
			defer func() {
				if p := recover(); p != nil {
					message = fmt.Sprint(p)
				}
			}()
			value = decodeHygieneCodec(session, ty, d)
		}()
		return fmt.Sprintf("decode %t %d %s\nvalue %s\npanic %s\n", message != "", c.position, strings.Join(c.events, ","), value, hex.EncodeToString([]byte(message)))
	})
}
