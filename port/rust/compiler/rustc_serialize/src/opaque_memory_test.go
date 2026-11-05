package rustcserialize

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

func memoryParseU128(text string) Uint128 {
	if len(text) != 32 {
		panic("invalid native u128")
	}
	hi, err := strconv.ParseUint(text[:16], 16, 64)
	if err != nil {
		panic(err)
	}
	lo, err := strconv.ParseUint(text[16:], 16, 64)
	if err != nil {
		panic(err)
	}
	return Uint128{Lo: lo, Hi: hi}
}
func memoryHexU128(v Uint128) string { return fmt.Sprintf("%016x%016x", v.Hi, v.Lo) }
func TestOriginalMemoryPrimitives(t *testing.T) {
	file, err := os.Open("testdata/memory.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	raw, err := io.ReadAll(io.LimitReader(gz, 4*1024*1024+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 4*1024*1024 {
		t.Fatal("memory fixture exceeds bound")
	}
	scan := bufio.NewScanner(strings.NewReader(string(raw)))
	count := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 6 {
			t.Fatal("malformed memory record")
		}
		e := NewMemEncoder()
		v := Uint128{}
		if p[0] != "str" {
			v = memoryParseU128(p[1])
		}
		switch p[0] {
		case "usize":
			e.EmitUsize(uint(v.Lo))
		case "u128":
			e.EmitU128(v)
		case "u64":
			e.EmitU64(v.Lo)
		case "u32":
			e.EmitU32(uint32(v.Lo))
		case "u16":
			e.EmitU16(uint16(v.Lo))
		case "u8":
			e.EmitU8(uint8(v.Lo))
		case "isize":
			e.EmitIsize(int(v.Lo))
		case "i128":
			e.EmitI128(Int128{Lo: v.Lo, Hi: v.Hi})
		case "i64":
			e.EmitI64(int64(v.Lo))
		case "i32":
			e.EmitI32(int32(v.Lo))
		case "i16":
			e.EmitI16(int16(v.Lo))
		case "i8":
			e.EmitI8(int8(v.Lo))
		case "str":
			input, err := hex.DecodeString(p[1])
			if err != nil {
				t.Fatal(err)
			}
			e.EmitStr(string(input))
		default:
			t.Fatal(p[0])
		}
		capacity := cap(e.Data)
		encoded := e.Finish()
		if got := hex.EncodeToString(encoded); got != p[2] {
			t.Fatalf("%s %s bytes %s != %s", p[0], p[1], got, p[2])
		}
		if p[0] != "str" && strconv.Itoa(capacity) != p[3] {
			t.Fatalf("%s capacity %d != %s", p[0], capacity, p[3])
		}
		d, err := NewMemDecoder(append(encoded, []byte(MagicEndBytes)...), 0)
		if err != nil {
			t.Fatal(err)
		}
		got := Uint128{}
		switch p[0] {
		case "usize":
			got.Lo = uint64(d.ReadUsize())
		case "u128":
			got = d.ReadU128()
		case "u64":
			got.Lo = d.ReadU64()
		case "u32":
			got.Lo = uint64(d.ReadU32())
		case "u16":
			got.Lo = uint64(d.ReadU16())
		case "u8":
			got.Lo = uint64(d.ReadU8())
		case "isize":
			n := int64(d.ReadIsize())
			got = Uint128{Lo: uint64(n), Hi: signHi(n)}
		case "i128":
			n := d.ReadI128()
			got = Uint128{Lo: n.Lo, Hi: n.Hi}
		case "i64":
			n := d.ReadI64()
			got = Uint128{Lo: uint64(n), Hi: signHi(n)}
		case "i32":
			n := int64(d.ReadI32())
			got = Uint128{Lo: uint64(n), Hi: signHi(n)}
		case "i16":
			n := int64(d.ReadI16())
			got = Uint128{Lo: uint64(n), Hi: signHi(n)}
		case "i8":
			n := int64(d.ReadI8())
			got = Uint128{Lo: uint64(n), Hi: signHi(n)}
		case "str":
			if got := hex.EncodeToString([]byte(d.ReadStr())); got != p[4] {
				t.Fatalf("str %s != %s", got, p[4])
			}
		}
		if p[0] != "str" && memoryHexU128(got) != p[4] {
			t.Fatalf("%s %s decode %s != %s", p[0], p[1], memoryHexU128(got), p[4])
		}
		if strconv.FormatUint(uint64(d.Position()), 10) != p[5] {
			t.Fatal("decoder position mismatch")
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 3256 {
		t.Fatal(count)
	}
}
func TestMemEncoderFinishIsUnframedAndConsumed(t *testing.T) {
	e := NewMemEncoder()
	e.EmitU8(42)
	raw := e.Finish()
	if _, err := NewMemDecoder(raw, 0); err == nil {
		t.Fatal("unframed memory output accepted")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("consumed encoder reused")
		}
	}()
	e.EmitU8(0)
}
func TestMemDecoderPositionRestoredOnPanic(t *testing.T) {
	d, err := NewMemDecoder(append([]byte{1, 2, 3}, []byte(MagicEndBytes)...), 1)
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("callback did not panic")
			}
		}()
		d.WithPosition(0, func(inner *MemDecoder) {
			if inner.ReadU8() != 1 {
				t.Fatal("wrong borrowed decoder")
			}
			panic("callback")
		})
	}()
	if d.Position() != 1 || d.ReadU8() != 2 {
		t.Fatal("source cursor not restored")
	}
}
