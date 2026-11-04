// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

type sortValue struct {
	key         int64
	id, touches uint64
	shared      *[2]uint64
}

func sortInitial(kind, length int, seed uint64, distribution int) []sortValue {
	values := make([]sortValue, length)
	for id := range values {
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		var key int64
		switch distribution {
		case 0:
			key = int64(seed % 13)
		case 1:
			key = int64(seed % 100003)
		case 2:
			key = int64(id)
		case 3:
			key = int64(length - id)
		case 4:
			key = 17
		case 5:
			key = int64(min(id, length-id))
		case 6:
			key = int64((id/31)*31 + int(seed%5))
		default:
			panic("invalid sort distribution")
		}
		values[id] = sortValue{key: key, id: uint64(id)}
		if kind == 9 {
			values[id].key, values[id].id = 0, 0
		}
		if kind == 5 || kind == 6 {
			values[id].shared = new([2]uint64)
		}
	}
	return values
}
func sortTouches(v *sortValue, kind int) uint64 {
	if kind == 4 {
		return v.touches
	}
	if kind == 5 {
		return v.shared[0]
	}
	if kind == 6 {
		return v.shared[0] + v.shared[1]
	}
	return 0
}
func sortTouch(v *sortValue, kind int) {
	if kind == 4 {
		v.touches++
	}
	if kind == 5 || kind == 6 {
		v.shared[0]++
	}
	if kind == 6 {
		v.shared[1]++
	}
}
func sortWord(h hash.Hash, word uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], word)
	h.Write(b[:])
}
func sortCommit(h hash.Hash, size uint64) string { return fmt.Sprintf("%d:%x", size, h.Sum(nil)) }

func TestOriginalStableSort(t *testing.T) {
	replayOriginalSort(t, "manifest.json", "stable.tsv.gz", "oracle.rs", "generate.py", "SOURCES.json", Stable[sortValue], false)
}
func replayOriginalSort(t *testing.T, manifestName, fixtureName, oracleName, generatorName, auditName string, sort func([]sortValue, Type, Less[sortValue], ...Context), unstable bool) {
	t.Helper()
	const root = "../../../tools/port-rust-unord/sort/"
	data, err := os.ReadFile(root + manifestName)
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases, Callbacks, Panics int
		Fixture                  string `json:"fixture_sha256"`
		Raw                      string `json:"fixture_raw_sha256"`
		Oracle                   string `json:"oracle_sha256"`
		Generator                string `json:"generator_sha256"`
		Audit                    string `json:"source_audit_sha256"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
	for path, want := range map[string]string{root + oracleName: manifest.Oracle, root + generatorName: manifest.Generator, root + auditName: manifest.Audit} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("original sort proof binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/" + fixtureName)
	coverage := manifest.Cases == 98720 && manifest.Callbacks == 7978963 && manifest.Panics == 89414
	if unstable {
		coverage = manifest.Cases == 103895 && manifest.Callbacks == 11487102 && manifest.Panics == 94502
	}
	if err != nil || digest(compressed) != manifest.Fixture || !coverage {
		t.Fatalf("original sort fixture/coverage differs:%v", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 128<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("original sort fixture body differs:%v", err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	count, totalCalls, totalPanics := 0, 0, 0
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 16 {
			t.Fatal("invalid original sort row")
		}
		number := func(index int) int {
			n, err := strconv.Atoi(fields[index])
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
		kind, length, seed, distribution, mode, fail := number(0), number(1), uint64(number(2)), number(3), number(4), number(5)
		typ := Type{Size: uint(number(6)), Align: uint(number(7)), Freeze: fields[8] == "true", Copy: fields[9] == "true"}
		expected := [10]Type{{16, 8, true, true}, {24, 8, true, true}, {64, 8, true, true}, {256, 8, true, true}, {24, 8, false, false}, {8, 8, true, true}, {16, 8, true, true}, {16, 8, true, false}, {4096, 8, true, true}, {0, 1, true, true}}
		if kind < 0 || kind >= len(expected) || typ != expected[kind] {
			t.Fatal("original sort type layout/specialization differs")
		}
		values := sortInitial(kind, length, seed, distribution)
		trace, state, drops := sha256.New(), sha256.New(), sha256.New()
		calls := 0
		panicked, message := false, ""
		func() {
			defer func() {
				if p := recover(); p != nil {
					panicked = true
					message = fmt.Sprint(p)
				}
			}()
			sort(values, typ, func(a, b *sortValue, c Context) bool {
				sortTouch(a, kind)
				sortTouch(b, kind)
				sortWord(trace, a.id)
				sortWord(trace, b.id)
				sortWord(trace, sortTouches(a, kind))
				sortWord(trace, sortTouches(b, kind))
				var same uint64
				if a == b {
					same = 1
				}
				sortWord(trace, same)
				index := calls
				calls++
				if index == fail {
					c.Panic("comparison_failure")
				}
				switch mode {
				case 0:
					return a.key < b.key
				case 1:
					return true
				case 2:
					aKey, bKey := a.key%3, b.key%3
					return aKey != bKey && (aKey+1)%3 == bKey
				case 3:
					if index%2 == 0 {
						return a.key < b.key
					}
					return b.key < a.key
				case 4:
					return index != 1
				default:
					panic("invalid sort comparison mode")
				}
			})
		}()
		for i := range values {
			sortWord(state, values[i].id)
			sortWord(state, uint64(values[i].key))
			sortWord(state, sortTouches(&values[i], kind))
		}
		var dropBytes uint64
		if kind == 7 {
			for i := range values {
				sortWord(drops, values[i].id)
				dropBytes += 8
			}
		}
		got := fmt.Sprintf("%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%t\t%t\t%t\t%d\t%s\t%s\t%s\t%s", kind, length, seed, distribution, mode, fail, typ.Size, typ.Align, typ.Freeze, typ.Copy, panicked, calls, sortCommit(trace, uint64(calls)*40), sortCommit(state, uint64(length)*24), sortCommit(drops, dropBytes), hex.EncodeToString([]byte(message)))
		if got != scanner.Text() {
			t.Fatalf("original stable sort request %d differs\n got:%s\nwant:%s", count, got, scanner.Text())
		}
		count++
		totalCalls += calls
		if panicked {
			totalPanics++
		}
	}
	if err := scanner.Err(); err != nil || count != manifest.Cases || totalCalls != manifest.Callbacks || totalPanics != manifest.Panics {
		t.Fatalf("original sort coverage differs:cases%d/calls%d/panics%d:%v", count, totalCalls, totalPanics, err)
	}
}
