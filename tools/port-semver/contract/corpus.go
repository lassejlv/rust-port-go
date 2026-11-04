package contract

import (
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
)

func CoreCorpus() []Request {
	out := []Request{}
	add := func(q Request) { out = append(out, q) }
	examples := []string{"", "0", "1", "01", "1.2", "1.2.3", "1.2.3.4", "1.2.3-pre+build", "1.2.3-0", "1.2.3+001", "v1.2.3", " 1.2.3", "1.2.3 ", "1.2.3\n", "1.2.3\t", "1.2.3-", "1.2.3+", "1.2.3-a..b", "1.2.3+a..b", "1.2.3+🦀", "1.2.3-\u2060", "1.2.3\x00", "1.2.3||<2", "1.2.3,", "1.2.3, 2", "*", "*.*", "x", "X", "x.1", "=1", "==1", "~1", "^1", ">=1", "<=1", "1 - 2", "1 || 2", "1, 2", "1 , 2", strings.Repeat("1,", 31) + "1", strings.Repeat("1,", 32) + "1"}
	for _, n := range []string{"0", "00", "01", "9", "10", "18446744073709551614", "18446744073709551615", "18446744073709551616", "999999999999999999999999999999999999"} {
		for _, prefix := range []string{"", "1.", "1.2.", "1.2.3-", "1.2.3+"} {
			examples = append(examples, prefix+n, prefix+n+"x")
		}
	}
	for _, op := range []string{"", "=", "==", ">", ">=", "<", "<=", "~", "^", "!", "!="} {
		for _, middle := range []string{"", " ", "  ", "\t", "\n"} {
			for _, suffix := range []string{"1", "1.2", "1.2.3", "1.2.3-pre+build", "1.*", "1.x", "1.X", "1.*.2", "*.1", "x.2", "1.2.*-pre", "1.2.3+001", "0.0.0", "01.2.3"} {
				examples = append(examples, op+middle+suffix)
			}
		}
	}
	identifiers := []string{"", "0", "1", "2", "9", "10", "99", "100", "01", "001", "00", "000", "a", "b", "-", "--", "A", "Z", "a-0", "a.0", "a.1", "a.a", "0.a", "a.00", "0.0", "a..b", ".a", "a.", "a+b", "a_b", "🦀", "\n", "\x00", strings.Repeat("a", 8), strings.Repeat("a", 9), strings.Repeat("z", 4096), strings.Repeat("9", 100)}
	for _, kind := range []string{"version", "req", "comparator", "pre", "build"} {
		for _, text := range append(append([]string{}, examples...), identifiers...) {
			add(Request{Op: "parse", Kind: kind, A: text})
		}
	}
	// Character errors include ASCII controls and Unicode scalar boundaries from
	// the exact Rust debug escaping table's important planes.
	for _, ch := range []rune{'\x00', '\x01', '\t', '\n', '\r', '\x1f', '\x7f', '\u0085', '\u00a0', 'é', '\u034f', '\u061c', '\u200b', '\u2060', '\u2028', '\ufeff', '\U0001e6c0', '\U0001f980', '\U0010ffff'} {
		for _, kind := range []string{"version", "req", "comparator", "pre", "build"} {
			for _, prefix := range []string{"", "1.", "1.2.", "1.2.3", "1.2.3-", "1.2.3+"} {
				add(Request{Op: "parse", Kind: kind, A: prefix + string(ch)})
			}
		}
	}
	rng := rand.New(rand.NewSource(0x73656d766572))
	alphabet := []rune("0123456789aAzZ-+.^*xX~<>=!, \t\n🦀\u2060\x00")
	for i := 0; i < 2000; i++ {
		n := rng.Intn(50)
		var text strings.Builder
		for j := 0; j < n; j++ {
			text.WriteRune(alphabet[rng.Intn(len(alphabet))])
		}
		for _, kind := range []string{"version", "req", "comparator", "pre", "build"} {
			add(Request{Op: "parse", Kind: kind, A: text.String()})
		}
	}
	pre := []string{"", "0", "1", "9", "10", "99", "100", "a", "b", "-", "A", "a.0", "a.1", "a.a", "0.a", "0.0", strings.Repeat("9", 100), strings.Repeat("a", 9)}
	build := append(append([]string{}, pre...), "00", "000", "01", "001", "002", "02", "a.00")
	for _, set := range []struct {
		kind   string
		values []string
	}{{"pre", pre}, {"build", build}} {
		for _, a := range set.values {
			for _, b := range set.values {
				add(Request{Op: "order", Kind: set.kind, A: a, B: b})
			}
		}
	}
	versions := []string{}
	for _, n := range []string{"0.0.0", "0.0.1", "0.1.0", "0.2.3", "1.0.0", "1.2.0", "1.2.3", "1.2.4", "1.3.0", "2.0.0", "18446744073709551615.18446744073709551615.18446744073709551615"} {
		for _, p := range []string{"", "-0", "-alpha", "-alpha.1", "-beta.2"} {
			for _, b := range []string{"", "+0", "+001", "+build"} {
				versions = append(versions, n+p+b)
			}
		}
	}
	for i := 0; i < 3000; i++ {
		add(Request{Op: "order", Kind: "version", A: versions[rng.Intn(len(versions))], B: versions[rng.Intn(len(versions))]})
	}
	requirements := []string{"*", "^0", "^0.0", "^0.0.0", "^0.0.1", "^0.1.0", "^1.2.3", "^1.2.3-alpha", "~1", "~1.2", "~1.2.3", "~0.0.1", "=1", "=1.2", "=1.2.3", "=1.2.3+001", ">0", ">=0", ">=1.2.3-alpha", ">1.2.3-alpha", "<1", "<1.2.3", "<=1.2.3", "1.*", "1.2.*", ">=1.0.0, <2.0.0", "^1.2.3, >=1.2.3-alpha", "^18446744073709551615", "~18446744073709551615.18446744073709551615.18446744073709551615"}
	for _, a := range requirements {
		for _, b := range versions {
			add(Request{Op: "matches", A: a, B: b})
		}
	}
	some := func(n uint64) *uint64 { return &n }
	for op := uint64(0); op < 8; op++ {
		for _, major := range []uint64{0, 1, 18446744073709551615} {
			for _, minor := range []*uint64{nil, some(0), some(2), some(18446744073709551615)} {
				for _, patch := range []*uint64{nil, some(0), some(3), some(18446744073709551615)} {
					for _, p := range []string{"", "0", "alpha"} {
						for i := 0; i < 4; i++ {
							add(Request{Op: "raw", Operator: op, Major: major, Minor: minor, Patch: patch, Pre: p, B: versions[rng.Intn(len(versions))]})
						}
					}
				}
			}
		}
	}
	for _, v := range []struct{ kind, a string }{{"version", "1.2.3"}, {"version", "18446744073709551615.2.3-pre+001"}, {"version", "1.2.3-pre+build"}, {"req", "*"}, {"req", "^1.2.3, <2"}, {"comparator", "1.2.*"}, {"comparator", ">=1.2.3-alpha"}, {"pre", ""}, {"pre", "alpha.1"}, {"build", ""}, {"build", "001.build"}, {"error", "\x00.2.3"}, {"error", "\u2060.2.3"}, {"error", "1.2.3+"}} {
		for mode := uint64(0); mode < 6; mode++ {
			for fail := uint64(0); fail < 45; fail++ {
				add(Request{Op: "display", Kind: v.kind, A: v.a, Mode: mode, Fail: fail})
			}
		}
	}
	return out
}
func Summary(corpus []Request) string {
	counts := map[string]int{}
	for _, q := range corpus {
		counts[q.Op]++
	}
	return fmt.Sprintf("%d requests %v", len(corpus), counts)
}
func SerdeCorpus() []Request {
	out := []Request{}
	for _, kind := range []string{"version", "req", "comparator"} {
		for _, a := range []string{`"1.2.3"`, `"1.2.3-pre+001"`, `"^1.2.3"`, `"<2.0.0"`, `""`, `"v1.2.3"`, `"1.2.3\u0000"`, `"1.2.3\ud83e\udd80"`, `"1.2.3\uD800"`, `"1.2.3\uDC00"`, "null", "true", "false", "42", "-42", "1.5", "-0", "1e30", "1e400", "18446744073709551615", "18446744073709551616", "-9223372036854775809", "[]", "[", "{}", "{", `"1.2.3" null`, `"bad" null`} {
			out = append(out, Request{Op: "serde", Kind: kind, Input: "json", A: a})
		}

		for _, override := range []bool{false, true} {
			for _, input := range []string{"str", "borrowed_str", "string"} {
				for _, text := range []string{"", "1", "1.2.3", "1.2.3-pre+build", "^1.2.3", "*", ">=1.2.3, <2", "1.2.3\x00", "1.2.3\u2060", "é.2.3"} {
					out = append(out, Request{Op: "serde", Kind: kind, Input: input, A: text, Override: override})
				}
			}
			for _, c := range []string{"0", "1", "*", "🦀", "\x00", "\u2060", "'"} {
				out = append(out, Request{Op: "serde", Kind: kind, Input: "char", A: c, Override: override})
			}
			for _, input := range []string{"bool", "i8", "i16", "i32", "i64", "i128", "u8", "u16", "u32", "u64", "u128", "f32", "f64", "bytes", "borrowed_bytes", "byte_buf", "none", "some", "unit", "newtype", "seq", "map", "enum"} {
				q := Request{Op: "serde", Kind: kind, Input: input, A: "1.2.3", Signed: -42, Unsigned: 42, Wide: [2]uint64{18446744073709551615, 18446744073709551615}, Float: "3.141592653589793", Override: override}
				out = append(out, q)
				if input == "f32" || input == "f64" {
					for _, f := range []string{"0", "-0", "1", "-1", "0.00000000001", "1e30", "-1e308", "5e-324", "NaN", "inf", "-inf", "1.2345678901234567"} {
						q.Float = f
						out = append(out, q)
					}
				}
				if input == "i128" || input == "u128" {
					for _, wide := range [][2]uint64{{0, 0}, {0, 1}, {0, 18446744073709551615}, {1, 0}, {9223372036854775807, 18446744073709551615}, {9223372036854775808, 0}} {
						q.Wide = wide
						out = append(out, q)
					}
				}
			}
		}
		for _, collect := range []bool{false, true} {
			for _, reject := range []bool{false, true} {
				for _, a := range []string{"1.2.3", "1.2.3-pre+build"} {
					out = append(out, Request{Op: "serde", Kind: kind, Input: "serialize", A: a, Collect: collect, Reject: reject})
				}
			}
		}
	}
	// The default invalid_type passes a composite fmt::Arguments to a caller's
	// Error::custom. Observe fallible streaming rather than just its final text.
	for _, kind := range []string{"version", "req", "comparator"} {
		for _, input := range []string{"str", "char", "bool", "i8", "i16", "i32", "i64", "i128", "u8", "u16", "u32", "u64", "u128", "f32", "f64", "bytes", "borrowed_bytes", "byte_buf", "none", "some", "unit", "newtype", "seq", "map", "enum"} {
			for fail := uint64(0); fail < 15; fail++ {
				out = append(out, Request{Op: "serde", Kind: kind, Input: input, A: "!", Signed: -42, Unsigned: 42, Wide: [2]uint64{18446744073709551615, 18446744073709551615}, Float: "3.141592653589793", Reject: true, Fail: fail})
			}
		}
	}
	for _, value := range []string{"0", "-0", "1", "-1", "0.00000000001", "1e30", "-1e308", "5e-324", "NaN", "inf", "-inf", "1.2345678901234567"} {
		for fail := uint64(0); fail < 25; fail++ {
			out = append(out, Request{Op: "serde", Kind: "version", Input: "f64", Float: value, Reject: true, Fail: fail})
		}
	}
	random := rand.New(rand.NewSource(0x7365726465666d74))
	for i := 0; i < 1000; i++ {
		number := math.Float64frombits(random.Uint64())
		text := strconv.FormatFloat(number, 'g', -1, 64)
		fail := uint64(0)
		if i%3 != 0 {
			fail = uint64(i % 16)
		}
		input := "f64"
		if i%2 == 0 {
			input = "f32"
		}
		out = append(out, Request{Op: "serde", Kind: "version", Input: input, Float: text, Reject: true, Fail: fail})
	}
	for _, kind := range []string{"version", "req", "comparator"} {
		for _, data := range []string{`"bad" "\ud800"`, `true "\ud800"`, `[ "\ud800" ]`, `{ "x":"\ud800" }`, `"1.2.3" "\ud800"`, `"bad" ` + string([]byte{255}), "[ " + string([]byte{255}), `"1.2.3" ` + string([]byte{255}), `"1.2.3` + string([]byte{255}) + `"`} {
			out = append(out, Request{Op: "serde", Kind: kind, Input: "json_bytes", A: hex.EncodeToString([]byte(data))})
		}
	}
	return out
}
