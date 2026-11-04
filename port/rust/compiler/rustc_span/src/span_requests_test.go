package rustcspan

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

type spanOracleOp map[string]any
type spanOracleSourceMap struct {
	Text  string `json:"text"`
	Panic bool   `json:"panic"`
}
type spanOracleReentry struct {
	Lo, Hi, Ctxt uint32
	Parent       *uint32
}

func (value spanOracleReentry) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]any{"lo": value.Lo, "hi": value.Hi, "ctxt": value.Ctxt, "parent": value.Parent})
}

type spanOracleRequest struct {
	ID           string               `json:"id"`
	Debug        bool                 `json:"debug_assertions"`
	Overflow     bool                 `json:"overflow_checks"`
	Session      bool                 `json:"session"`
	SourceMap    *spanOracleSourceMap `json:"source_map,omitempty"`
	TrackPanicAt *int                 `json:"track_panic_at,omitempty"`
	TrackReenter *spanOracleReentry   `json:"track_reenter,omitempty"`
	Abort        bool                 `json:"abort,omitempty"`
	Ops          []spanOracleOp       `json:"ops"`
}

func spanOracleUint(value uint32) *uint32 { return &value }
func spanOracleInt(value int) *int        { return &value }
func spanOracleNew(op string, lo, hi, context uint32, parent *uint32) spanOracleOp {
	return spanOracleOp{"op": op, "lo": lo, "hi": hi, "ctxt": context, "parent": parent}
}
func spanOracleAccessors(slot int) []spanOracleOp {
	return []spanOracleOp{{"op": "inline_from", "span": slot}, {"op": "data_untracked", "span": slot}, {"op": "data", "span": slot}, {"op": "ctxt", "span": slot}, {"op": "parent", "span": slot}, {"op": "is_dummy", "span": slot}, {"op": "from_expansion", "span": slot}, {"op": "hash", "span": slot}}
}

// Request construction is independent of all Go span results. The native tool
// consumes this deterministic stream and supplies every expected observation.
func spanOracleRequests() []spanOracleRequest {
	var all []spanOracleRequest
	add := func(request spanOracleRequest) {
		if len(request.Ops) > 0 && request.Ops[0]["op"] != "inline_try" {
			request.Ops = append(request.Ops, spanOracleOp{"op": "clone", "span": 0}, spanOracleOp{"op": "ord_clone", "span": 0})
		}
		request.ID = fmt.Sprintf("span-%06d", len(all))
		all = append(all, request)
	}
	parents := []*uint32{nil, spanOracleUint(0), spanOracleUint(32767), spanOracleUint(32768), spanOracleUint(65535), spanOracleUint(65536), spanOracleUint(DefIndexMaxAsU32)}
	for _, debug := range []bool{false, true} {
		for _, overflow := range []bool{false, true} {
			base := func(session bool, ops []spanOracleOp) spanOracleRequest {
				return spanOracleRequest{Debug: debug, Overflow: overflow, Session: session, Ops: ops}
			}
			for _, lo := range []uint32{0, 0xffffff, 0x1000000} {
				for _, length := range []uint32{0, 1, 127, 128, 16383, 16384, 32767, 32768} {
					for _, context := range []uint32{0, 1, 32767, 32768, 65535, 65536, SyntaxContextMaxAsU32} {
						for _, parent := range parents {
							ops := []spanOracleOp{spanOracleNew("new", lo, lo+length, context, parent)}
							ops = append(ops, spanOracleAccessors(0)...)
							ops = append(ops, spanOracleOp{"op": "map_ctxt", "span": 0, "to": context ^ 1}, spanOracleOp{"op": "with_parent", "span": 0, "parent": nil}, spanOracleOp{"op": "ord_eq", "span": 0, "other": 1}, spanOracleOp{"op": "ord_ne", "span": 0, "other": 2}, spanOracleOp{"op": "ord_cmp", "span": 1, "other": 2}, spanOracleOp{"op": "ord_partial_cmp", "span": 0, "other": 1})
							add(base(true, ops))
						}
					}
				}
			}
			for _, session := range []bool{false, true} {
				for _, bits := range []uint64{0, 0x7fffffffffffffff, 0x8000000000000000, 0xbfffffffffffffff, 0xc000000000000000, 0xdfffffffffffffff, 0xe000000000000000, 0xffffffffffffffff, 0x00010000ffffffff, 0x8000000000000001, 0x8000000100000001} {
					ops := []spanOracleOp{{"op": "raw", "bits": fmt.Sprintf("%016x", bits)}}
					ops = append(ops, spanOracleAccessors(0)...)
					ops = append(ops, spanOracleOp{"op": "map_ctxt", "span": 0, "to": 0}, spanOracleOp{"op": "map_ctxt", "span": 0, "to": 65536}, spanOracleOp{"op": "with_parent", "span": 0, "parent": nil}, spanOracleOp{"op": "with_parent", "span": 0, "parent": uint32(0)}, spanOracleOp{"op": "ord_eq", "span": 0, "other": 0}, spanOracleOp{"op": "hash", "span": 0, "panic_at": 0})
					add(base(session, ops))
				}
				for _, lo := range []uint32{0, ^uint32(0)} {
					for _, length := range []uint32{0, 32767, 32768, ^uint32(0)} {
						for _, context := range []uint32{0, 65535, 65536, ^uint32(0)} {
							add(base(session, []spanOracleOp{{"op": "inline_try", "lo": lo, "len": length, "ctxt": context}}))
						}
					}
				}
				for _, ctor := range []spanOracleOp{spanOracleNew("new_ordered", 100, 0, 0, nil), spanOracleNew("new", 0, 40000, 65536, nil), spanOracleNew("new", 0, 0, SyntaxContextMaxAsU32+1, nil), spanOracleNew("new", 0, 0, 0, spanOracleUint(DefIndexMaxAsU32+1))} {
					add(base(session, []spanOracleOp{{"op": "dummy"}, ctor, {"op": "data", "span": 0}}))
				}
			}
			for _, tracker := range []*int{nil, spanOracleInt(0), spanOracleInt(1), spanOracleInt(2)} {
				for _, reentry := range []bool{false, true} {
					ops := []spanOracleOp{spanOracleNew("new", 1, 40001, 65536, spanOracleUint(7)), spanOracleNew("new", 1, 40001, 65537, spanOracleUint(9)), {"op": "data", "span": 0}, {"op": "with_parent", "span": 0, "parent": uint32(9)}, {"op": "ord_eq", "span": 0, "other": 1}, {"op": "debug", "span": 0, "kind": "span", "pattern": 0, "chars": false, "fail": 0, "reuse": true}}
					r := base(true, ops)
					r.TrackPanicAt = tracker
					if reentry {
						r.TrackReenter = &spanOracleReentry{Lo: 10, Hi: 50000, Ctxt: 65536}
					}
					add(r)
				}
			}
			shapes := []spanOracleOp{spanOracleNew("new", 10, 31, 26, nil), spanOracleNew("new", 10, 31, 0, spanOracleUint(7)), spanOracleNew("new", 10, 31, 26, spanOracleUint(65535)), spanOracleNew("new", 0xffff0000, 0xfffffff0, 65536, spanOracleUint(65536))}
			for _, shape := range shapes {
				for pattern := 0; pattern < 19; pattern++ {
					for _, chars := range []bool{false, true} {
						for _, fail := range []int{-1, 0, 1, 2, 3, 6, 12, 30} {
							for _, reuse := range []bool{false, true} {
								add(base(true, []spanOracleOp{shape, {"op": "debug", "span": 0, "kind": "span", "pattern": pattern, "chars": chars, "fail": fail, "reuse": reuse}, {"op": "debug", "span": 0, "kind": "ord", "pattern": pattern, "chars": chars, "fail": fail, "reuse": reuse}, {"op": "debug", "span": 0, "kind": "data", "pattern": pattern, "chars": chars, "fail": fail, "reuse": reuse}}))
							}
						}
					}
				}
			}
			for _, text := range []string{"", "demo.rs:1:2: 1:7", "雪.rs:2:1", "first\nsecond"} {
				for _, mapPanic := range []bool{false, true} {
					for _, fail := range []int{-1, 0, 1, 2, 3} {
						ops := []spanOracleOp{spanOracleNew("new", 10, 31, 26, spanOracleUint(7)), {"op": "debug", "span": 0, "kind": "span", "pattern": 13, "chars": true, "fail": fail, "reuse": true}, {"op": "debug", "span": 0, "kind": "ord", "pattern": 1, "chars": false, "fail": fail, "reuse": true}}
						r := base(true, ops)
						r.SourceMap = &spanOracleSourceMap{text, mapPanic}
						add(r)
					}
				}
			}
			for _, ambient := range []string{"normal", "unwinding"} {
				for _, move := range []bool{false, true} {
					for _, callPanic := range []bool{false, true} {
						for _, shape := range shapes {
							ops := []spanOracleOp{shape, {"op": "map_owned", "span": 0, "to": 0, "call_panic": callPanic, "drop_panic": false, "move_token": move, "ambient": ambient}}
							add(base(true, ops))
						}
					}
				}
			}
		}
	}
	// Owned failure/ambient profiles are kept separate from the larger numeric
	// matrix. Abort-capable requests are explicitly isolated by the native tool.
	for _, debug := range []bool{false, true} {
		for _, overflow := range []bool{false, true} {
			addOwned := func(session bool, bits string, to uint32, ambient string, callPanic, dropPanic, move, abort bool) {
				add(spanOracleRequest{Debug: debug, Overflow: overflow, Session: session, Abort: abort, Ops: []spanOracleOp{{"op": "raw", "bits": bits}, {"op": "map_owned", "span": 0, "to": to, "call_panic": callPanic, "drop_panic": dropPanic, "move_token": move, "ambient": ambient}}})
			}
			for _, ambient := range []string{"normal", "unwinding"} {
				for _, move := range []bool{false, true} {
					addOwned(true, "0001000000000001", 0, ambient, false, true, move, false)
					addOwned(true, "e000000000000064", 0, ambient, false, false, move, false)
					addOwned(false, "0001000000000001", 65536, ambient, false, false, move, false)
					addOwned(true, "0001000000000001", SyntaxContextMaxAsU32+1, ambient, false, false, move, false)
					addOwned(true, "0001000000000001", 0, ambient, true, true, move, true)
					addOwned(true, "e000000000000064", 0, ambient, false, true, move, true)
				}
			}
			for _, move := range []bool{false, true} {
				addOwned(true, "0001000000000001", 0, "processing_panic", false, false, move, false)
				addOwned(true, "0001000000000001", 0, "processing_panic", true, false, move, true)
				addOwned(true, "0001000000000001", 0, "processing_panic", false, true, move, true)
				addOwned(false, "0001000000000001", 65536, "processing_panic", false, false, move, true)
				addOwned(true, "e000000000000064", 0, "processing_panic", false, false, move, true)
			}
		}
	}
	return all
}

func TestDumpSpanRequests(t *testing.T) {
	path, present := os.LookupEnv("FERRUGO_SPAN_DUMP_REQUESTS")
	if !present {
		t.Skip("native request dump not requested")
	}
	if path == "" {
		t.Fatal("FERRUGO_SPAN_DUMP_REQUESTS is explicitly empty")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		t.Fatal(err)
	}
	writer := bufio.NewWriter(file)
	encoder := json.NewEncoder(writer)
	count := 0
	for _, request := range spanOracleRequests() {
		if err := encoder.Encode(request); err != nil {
			file.Close()
			t.Fatal(err)
		}
		count++
	}
	if err := writer.Flush(); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d deterministic original-span requests", count)
}
