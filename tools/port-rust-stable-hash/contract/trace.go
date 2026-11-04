package contract

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	hash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"math/big"
	"strconv"
	"strings"
)

type TraceRequest struct {
	Full    bool     `json:"full"`
	Short   bool     `json:"short"`
	Direct  bool     `json:"direct"`
	Actions []Action `json:"actions"`
}
type traceH struct{ bytes []byte }
type traceOut struct{ bytes []byte }

func wide(value [2]uint64, signed bool) string {
	v := new(big.Int).SetUint64(value[1])
	v.Lsh(v, 64)
	v.Or(v, new(big.Int).SetUint64(value[0]))
	if signed && value[1]>>63 != 0 {
		v.Sub(v, new(big.Int).Lsh(big.NewInt(1), 128))
	}
	return v.String()
}
func nativeWide(v hash.UInt128) (b [16]byte) {
	var probe [2]byte
	binary.NativeEndian.PutUint16(probe[:], 1)
	if probe[0] == 1 {
		binary.NativeEndian.PutUint64(b[:8], v[0])
		binary.NativeEndian.PutUint64(b[8:], v[1])
	} else {
		binary.NativeEndian.PutUint64(b[:8], v[1])
		binary.NativeEndian.PutUint64(b[8:], v[0])
	}
	return
}
func ObserveTrace(q TraceRequest) []string {
	events := []string{}
	log := func(s string) { events = append(events, s) }
	appendBytes := func(r hash.Ref[traceH], bytes []byte) { r.Value.bytes = append(r.Value.bytes, bytes...) }
	ops := hash.ExtendedHasherOps[traceH, []byte]{Write: func(r hash.Ref[traceH], b []byte) { log("write:" + hex.EncodeToString(b)); appendBytes(r, b) }, Finish64: func(r hash.Ref[traceH]) uint64 {
		log(fmt.Sprintf("finish64:%d", len(r.Value.bytes)))
		return uint64(len(r.Value.bytes))
	}, Finish: func(arg *hash.Owned[traceH]) *hash.Owned[[]byte] {
		log("finish-owned")
		return hash.Own(append([]byte(nil), arg.Borrow().bytes...))
	}, Clone: func(r hash.Ref[traceH]) traceH { log("clone"); return traceH{append([]byte(nil), r.Value.bytes...)} }, Drop: func(v *traceH, _ hash.DropContext) { log("drop-h:" + hex.EncodeToString(v.bytes)) }, Debug: func(r hash.Ref[traceH], f *rustfmt.Formatter) error {
		width := "None"
		if f.Options.HasWidth {
			width = fmt.Sprintf("Some(%d)", f.Options.Width)
		}
		log(fmt.Sprintf("debug:alternate=%t:width=%s:fill=%c", f.Options.Alternate, width, f.Options.Fill))
		if e := f.WriteString("Base("); e != nil {
			return e
		}
		if e := rustfmt.FormatDebugUint(uint64(len(r.Value.bytes)), uint(strconv.IntSize), f); e != nil {
			return e
		}
		return f.WriteString(")")
	}, WriteU8: func(r hash.Ref[traceH], v uint8) { log(fmt.Sprintf("u8:%d", v)); appendBytes(r, []byte{v}) }, WriteString: func(r hash.Ref[traceH], s string) {
		log("str:" + hex.EncodeToString([]byte(s)))
		appendBytes(r, []byte(s))
		appendBytes(r, []byte{255})
	}, WriteLengthPrefix: func(r hash.Ref[traceH], n uint) {
		log(fmt.Sprintf("len:%d", n))
		if strconv.IntSize == 64 {
			var b [8]byte
			binary.NativeEndian.PutUint64(b[:], uint64(n))
			appendBytes(r, b[:])
		} else {
			var b [4]byte
			binary.NativeEndian.PutUint32(b[:], uint32(n))
			appendBytes(r, b[:])
		}
	}}
	if q.Short {
		ops.ShortWrite = func(r hash.Ref[traceH], length uint, b []byte) {
			log(fmt.Sprintf("short:%d:%s", length, hex.EncodeToString(b)))
			appendBytes(r, b)
		}
	}
	if q.Full {
		ops.WriteU16 = func(r hash.Ref[traceH], v uint16) {
			log(fmt.Sprintf("u16:%d", v))
			var b [2]byte
			binary.NativeEndian.PutUint16(b[:], v)
			appendBytes(r, b[:])
		}
		ops.WriteU32 = func(r hash.Ref[traceH], v uint32) {
			log(fmt.Sprintf("u32:%d", v))
			var b [4]byte
			binary.NativeEndian.PutUint32(b[:], v)
			appendBytes(r, b[:])
		}
		ops.WriteU64 = func(r hash.Ref[traceH], v uint64) {
			log(fmt.Sprintf("u64:%d", v))
			var b [8]byte
			binary.NativeEndian.PutUint64(b[:], v)
			appendBytes(r, b[:])
		}
		ops.WriteU128 = func(r hash.Ref[traceH], v hash.UInt128) {
			log("u128:" + wide([2]uint64(v), false))
			b := nativeWide(v)
			appendBytes(r, b[:])
		}
		ops.WriteUsize = func(r hash.Ref[traceH], v uint) {
			log(fmt.Sprintf("usize:%d", v))
			if strconv.IntSize == 64 {
				var b [8]byte
				binary.NativeEndian.PutUint64(b[:], uint64(v))
				appendBytes(r, b[:])
			} else {
				var b [4]byte
				binary.NativeEndian.PutUint32(b[:], uint32(v))
				appendBytes(r, b[:])
			}
		}
		ops.WriteI8 = func(r hash.Ref[traceH], v int8) { log(fmt.Sprintf("i8:%d", v)); appendBytes(r, []byte{uint8(v)}) }
		ops.WriteI16 = func(r hash.Ref[traceH], v int16) {
			log(fmt.Sprintf("i16:%d", v))
			var b [2]byte
			binary.NativeEndian.PutUint16(b[:], uint16(v))
			appendBytes(r, b[:])
		}
		ops.WriteI32 = func(r hash.Ref[traceH], v int32) {
			log(fmt.Sprintf("i32:%d", v))
			var b [4]byte
			binary.NativeEndian.PutUint32(b[:], uint32(v))
			appendBytes(r, b[:])
		}
		ops.WriteI64 = func(r hash.Ref[traceH], v int64) {
			log(fmt.Sprintf("i64:%d", v))
			var b [8]byte
			binary.NativeEndian.PutUint64(b[:], uint64(v))
			appendBytes(r, b[:])
		}
		ops.WriteI128 = func(r hash.Ref[traceH], v hash.Int128) {
			log("i128:" + wide([2]uint64(v), true))
			b := nativeWide(hash.UInt128(v))
			appendBytes(r, b[:])
		}
		ops.WriteIsize = func(r hash.Ref[traceH], v int) {
			log(fmt.Sprintf("isize:%d", v))
			if strconv.IntSize == 64 {
				var b [8]byte
				binary.NativeEndian.PutUint64(b[:], uint64(v))
				appendBytes(r, b[:])
			} else {
				var b [4]byte
				binary.NativeEndian.PutUint32(b[:], uint32(v))
				appendBytes(r, b[:])
			}
		}
	}
	if q.Direct {
		owned := hash.NewOwned(traceH{}, hash.ValueOps[traceH]{Drop: ops.Drop})
		view := ops.Bind(hash.Ref[traceH]{Value: owned.Borrow()})
		for _, a := range q.Actions {
			if a.Op == "short" {
				b, e := hex.DecodeString(a.Bytes)
				if e != nil {
					panic(e)
				}
				ops.ShortWriteArray(hash.Ref[traceH]{Value: owned.Borrow()}, a.Len, b)
			} else {
				apply(view, a)
			}
		}
		log(fmt.Sprintf("sum:%d", view.Finish64()))
		result := hash.FinishExtended(owned, ops)
		log("result:" + hex.EncodeToString(result.Take()))
	} else {
		h := hash.WithHasher(traceH{}, ops)
		for _, a := range q.Actions {
			apply(h, a)
		}
		cloned := h.Clone()
		cloned.CloneFrom(h)
		cloned.Close()
		var output strings.Builder
		f := rustfmt.Formatter{Writer: &output, Options: rustfmt.Options{Fill: ' ', HasWidth: true, Width: 32, Alternate: true, SignPlus: true, ZeroPad: true}}
		if e := h.WriteDebugFormatter(&f); e != nil {
			panic(e)
		}
		log("debug-text:" + output.String())
		log(fmt.Sprintf("sum:%d", h.Finish64()))
		out := hash.FinishInto(h, hash.FromStableHash[[]byte, traceOut]{From: func(arg *hash.Owned[[]byte]) *hash.Owned[traceOut] {
			log("from:" + hex.EncodeToString(*arg.Borrow()))
			return hash.NewOwned(traceOut{arg.Take()}, hash.ValueOps[traceOut]{Drop: func(v *traceOut, _ hash.DropContext) { log("drop-out:" + hex.EncodeToString(v.bytes)) }})
		}})
		log("result:" + hex.EncodeToString(out.Borrow().bytes))
		out.Drop()
	}
	return events
}
func TraceCorpus() []TraceRequest {
	result := []TraceRequest{}
	for _, full := range []bool{false, true} {
		for _, short := range []bool{false, true} {
			for _, direct := range []bool{false, true} {
				q := TraceRequest{Full: full, Short: short, Direct: direct}
				for _, op := range []string{"u8", "u16", "u32", "u64", "u128", "usize", "i8", "i16", "i32", "i64", "i128", "isize"} {
					for _, v := range []uint64{0, 1, 254, 255, 256, 18446744073709551615, 0x8000000000000000} {
						q.Actions = append(q.Actions, Action{Op: op, Value: v, Hi: 18446744073709551615})
					}
				}
				q.Actions = append(q.Actions, Action{Op: "bytes", Bytes: ""}, Action{Op: "bytes", Bytes: "0001ff"}, Action{Op: "str", Text: ""}, Action{Op: "str", Text: "🦀\x00"}, Action{Op: "len", Value: 42})
				if direct {
					for _, length := range []int{0, 1, 2, 4, 8, 9, 10, 16, 32} {
						b := make([]byte, length)
						for i := range b {
							b[i] = byte(i + 1)
						}
						q.Actions = append(q.Actions, Action{Op: "short", Len: uint(length), Bytes: hex.EncodeToString(b)})
					}
				}
				result = append(result, q)
			}
		}
	}
	return result
}
