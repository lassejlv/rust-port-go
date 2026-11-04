package contract

import (
	"encoding/hex"
	"fmt"
	hash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"math/rand"
	"strings"
)

type Action struct {
	Op    string `json:"op"`
	Value uint64 `json:"value"`
	Hi    uint64 `json:"hi"`
	Bytes string `json:"bytes"`
	Text  string `json:"text"`
	Len   uint   `json:"len"`
}
type Request struct {
	Kind    string   `json:"kind"`
	Key0    uint64   `json:"key0"`
	Key1    uint64   `json:"key1"`
	Actions []Action `json:"actions"`
}
type debugHasher interface {
	hash.Hasher
	WriteDebugFormatter(*rustfmt.Formatter) error
}

func snapshot(h debugHasher, sum hash.SipHasher128Hash) map[string]any {
	render := func(options rustfmt.Options) string {
		var text strings.Builder
		f := rustfmt.Formatter{Writer: &text, Options: options}
		if err := h.WriteDebugFormatter(&f); err != nil {
			panic(err)
		}
		return text.String()
	}
	pretty := rustfmt.DefaultOptions()
	pretty.Alternate = true
	hex := pretty
	hex.HasWidth = true
	hex.Width = 8
	hex.SignPlus = true
	hex.ZeroPad = true
	hex.DebugUpperHex = true
	return map[string]any{"hash": sum, "finish64": h.Finish64(), "debug": render(rustfmt.DefaultOptions()), "pretty": render(pretty), "hex": render(hex)}
}
func apply(h hash.Hasher, a Action) {
	switch a.Op {
	case "bytes":
		b, e := hex.DecodeString(a.Bytes)
		if e != nil {
			panic(e)
		}
		h.Write(b)
	case "str":
		hash.HashString(h, a.Text)
	case "len":
		hash.HashLengthPrefix(h, uint(a.Value))
	case "u8":
		hash.HashU8(h, uint8(a.Value))
	case "u16":
		hash.HashU16(h, uint16(a.Value))
	case "u32":
		hash.HashU32(h, uint32(a.Value))
	case "u64":
		hash.HashU64(h, a.Value)
	case "u128":
		hash.HashU128(h, hash.UInt128{a.Value, a.Hi})
	case "usize":
		hash.HashUsize(h, uint(a.Value))
	case "i8":
		hash.HashI8(h, int8(a.Value))
	case "i16":
		hash.HashI16(h, int16(a.Value))
	case "i32":
		hash.HashI32(h, int32(a.Value))
	case "i64":
		hash.HashI64(h, int64(a.Value))
	case "i128":
		hash.HashI128(h, hash.Int128{a.Value, a.Hi})
	case "isize":
		hash.HashIsize(h, int(a.Value))
	default:
		panic("unknown action")
	}
}
func Observe(q Request, profile hash.ArithmeticProfile) []map[string]any {
	steps := []map[string]any{}
	capture := func(body func()) (value any) { defer func() { value = recover() }(); body(); return }
	if q.Kind == "raw" {
		h := hash.NewSipHasher128WithProfile(q.Key0, q.Key1, profile)
		clones := []*hash.SipHasher128{}
		take := func() map[string]any { return snapshot(h, h.Clone().Finish128()) }
		steps = append(steps, map[string]any{"state": take()})
		for _, a := range q.Actions {
			p := capture(func() {
				switch a.Op {
				case "short":
					b, e := hex.DecodeString(a.Bytes)
					if e != nil {
						panic(e)
					}
					h.ShortWrite(a.Len, b)
				case "clone":
					clones = append(clones, h.Clone())
				case "restore":
					h.CloneFrom(clones[a.Value])
				default:
					apply(h, a)
				}
			})
			row := map[string]any{"state": take()}
			if p != nil {
				row["panic"] = fmt.Sprint(p)
			}
			steps = append(steps, row)
		}
	} else {
		h := hash.WithHasher(hash.NewSipHasher128WithProfile(q.Key0, q.Key1, profile), sipReferenceOps(profile))
		clones := []*hash.StableSipHasher128{}
		take := func() map[string]any { return snapshot(h, h.Clone().FinishHash()) }
		steps = append(steps, map[string]any{"state": take()})
		for _, a := range q.Actions {
			p := capture(func() {
				switch a.Op {
				case "clone":
					clones = append(clones, h.Clone())
				case "restore":
					h.CloneFrom(clones[a.Value])
				default:
					apply(h, a)
				}
			})
			row := map[string]any{"state": take()}
			if p != nil {
				row["panic"] = fmt.Sprint(p)
			}
			steps = append(steps, row)
		}
	}
	return steps
}
func sipReferenceOps(profile hash.ArithmeticProfile) hash.ExtendedHasherOps[*hash.SipHasher128, hash.SipHasher128Hash] {
	return hash.ExtendedHasherOps[*hash.SipHasher128, hash.SipHasher128Hash]{Write: func(r hash.Ref[*hash.SipHasher128], p []byte) { (*r.Value).Write(p) }, Finish64: func(r hash.Ref[*hash.SipHasher128]) uint64 { return (*r.Value).Finish64() }, Finish: func(v *hash.Owned[*hash.SipHasher128]) *hash.Owned[hash.SipHasher128Hash] {
		return hash.Own((*v.Borrow()).Finish128())
	}, ShortWrite: func(r hash.Ref[*hash.SipHasher128], n uint, p []byte) { (*r.Value).ShortWrite(n, p) }, Clone: func(r hash.Ref[*hash.SipHasher128]) *hash.SipHasher128 { return (*r.Value).Clone() }, Debug: func(r hash.Ref[*hash.SipHasher128], f *rustfmt.Formatter) error {
		return (*r.Value).WriteDebugFormatter(f)
	}, WriteU8: func(r hash.Ref[*hash.SipHasher128], v uint8) { (*r.Value).WriteU8(v) }, WriteI8: func(r hash.Ref[*hash.SipHasher128], v int8) { (*r.Value).WriteI8(v) }, WriteString: func(r hash.Ref[*hash.SipHasher128], s string) { hash.HashString(*r.Value, s) }}
}
func Corpus() []Request {
	result := []Request{}
	bytesAction := func(n int) Action {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(i*17 + 3)
		}
		return Action{Op: "bytes", Bytes: hex.EncodeToString(b)}
	}
	for _, kind := range []string{"raw", "stable"} {
		for _, keys := range [][2]uint64{{0, 0}, {0x0706050403020100, 0x0f0e0d0c0b0a0908}, {18446744073709551615, 0x8000000000000000}} {
			for n := 0; n < 146; n++ {
				result = append(result, Request{Kind: kind, Key0: keys[0], Key1: keys[1], Actions: []Action{bytesAction(n)}})
			}
		}
	}
	types := []string{"u8", "u16", "u32", "u64", "u128", "usize", "i8", "i16", "i32", "i64", "i128", "isize"}
	for _, kind := range []string{"raw", "stable"} {
		for offset := 0; offset < 65; offset++ {
			for _, typ := range types {
				result = append(result, Request{Kind: kind, Key0: 7, Key1: 13, Actions: []Action{bytesAction(offset), {Op: typ, Value: 0xfedcba9876543210, Hi: 0x0123456789abcdef}}})
			}
		}
	}
	for offset := 0; offset < 64; offset++ {
		for _, length := range []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 16, 32} {
			if length > 9 && offset+length >= 64 {
				continue
			}
			b := bytesAction(length)
			b.Op = "short"
			b.Len = uint(length)
			result = append(result, Request{Kind: "raw", Key0: 1, Key1: 2, Actions: []Action{bytesAction(offset), b}})
		}
	}
	for _, kind := range []string{"raw", "stable"} {
		for _, s := range []string{"", "a", "aa", "\x00", "é", "🦀", strings.Repeat("λ", 33), strings.Repeat("z", 64)} {
			result = append(result, Request{Kind: kind, Actions: []Action{{Op: "str", Text: s}, {Op: "str", Text: "tail"}, {Op: "len", Value: 17}}})
		}
		for _, value := range []uint64{0, 1, 254, 255, 256, 65535, 18446744073709551615, 0x8000000000000000} {
			result = append(result, Request{Kind: kind, Actions: []Action{{Op: "isize", Value: value}, {Op: "usize", Value: value}, {Op: "u128", Value: value, Hi: 18446744073709551615}, {Op: "i128", Value: value, Hi: 18446744073709551615}}})
		}
	}
	rng := rand.New(rand.NewSource(0x736970313238))
	for i := 0; i < 500; i++ {
		kind := "raw"
		if i%2 == 0 {
			kind = "stable"
		}
		q := Request{Kind: kind, Key0: rng.Uint64(), Key1: rng.Uint64()}
		for j := 0; j < 16; j++ {
			if j == 4 {
				q.Actions = append(q.Actions, Action{Op: "clone"})
			}
			if j == 11 {
				q.Actions = append(q.Actions, Action{Op: "restore", Value: 0})
			}
			if rng.Intn(3) == 0 {
				q.Actions = append(q.Actions, bytesAction(rng.Intn(140)))
			} else {
				q.Actions = append(q.Actions, Action{Op: types[rng.Intn(len(types))], Value: rng.Uint64(), Hi: rng.Uint64()})
			}
		}
		result = append(result, q)
	}
	return result
}
