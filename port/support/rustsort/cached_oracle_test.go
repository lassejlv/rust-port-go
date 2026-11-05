// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type cachedCase struct {
	kind, length               int
	chain                      bool
	seed                       uint64
	keyFail, cmpFail, dropFail int
	fnFail, tokenFail, moved   bool
}
type cachedItem struct {
	key int64
	id  uint64
}
type cachedKey struct {
	key         int64
	id, touches uint64
}
type cachedEnvironment struct {
	calls int
	first *cachedEnvironment
	token bool
}

func cachedSourceType(kind int) Type {
	return [...]Type{{1, 1, true, true}, {16, 8, true, false}, {24, 8, false, false}, {0, 1, true, false}, {16, 8, true, true}, {256, 8, true, false}}[kind]
}
func replayCachedCase(spec cachedCase) (message, state, events string, compares, drops int) {
	return replayCachedCaseContext(spec, Context{}, nil)
}
func replayCachedCaseContext(spec cachedCase, context Context, stream func(string)) (message, state, events string, compares, drops int) {
	var trace []string
	event := func(value string) {
		trace = append(trace, value)
		if stream != nil {
			stream(value)
		}
	}
	raise := func(c Context, label string) {
		if c.Runtime != nil {
			if control, ok := c.Runtime.Value.(interface{ ResumeCachedPanic() bool }); ok && control.ResumeCachedPanic() {
				c.ResumeUnwind(label)
			}
		}
		c.PanicAt(rustfmt.SourcePanicInfo{Payload: label, Kind: rustfmt.PanicStaticStr, Location: c.SourcePanicLocation("cached.client." + label), CanUnwind: true})
	}
	seed := spec.seed
	values := make([]cachedItem, spec.length)
	for i := range values {
		seed ^= seed << 13
		seed ^= seed >> 7
		seed ^= seed << 17
		values[i] = cachedItem{key: int64(seed % 13), id: uint64(i)}
	}
	typ := cachedSourceType(spec.kind)
	narrow, wide := tupleIndexType(typ, 4, context), tupleIndexType(typ, 8, context)
	event(fmt.Sprintf("TYPE:%d:%d:%d:%d", typ.Size, typ.Align, narrow.Size, wide.Size))
	movedToken := false
	tokenDrop := func(c Context) {
		event(fmt.Sprintf("TD:%t", c.Panicking()))
		if spec.tokenFail {
			raise(c, "token_drop")
		}
	}
	keyDrop := func(key *cachedKey, c Context) {
		index := drops
		drops++
		event(fmt.Sprintf("KD:%d:%d:%t", key.id, key.touches, c.Panicking()))
		if index == spec.dropFail {
			raise(c, "key_drop")
		}
	}
	keyOps := CachedKeyOps[cachedKey]{}
	if spec.kind != 0 && spec.kind != 4 {
		keyOps.Drop = keyDrop
	}
	comparison := func(label string, a, b *cachedKey, c Context) Ordering {
		if spec.kind == 2 {
			a.touches++
			b.touches++
		}
		index := compares
		compares++
		event(fmt.Sprintf("%s:%d:%d:%d:%d:%t", label, a.id, b.id, a.touches, b.touches, c.Panicking()))
		if index == spec.cmpFail {
			raise(c, "key_compare")
		}
		if a.key < b.key {
			return LessThan
		}
		if a.key > b.key {
			return GreaterThan
		}
		return Equal
	}
	keyOps.PartialCompare = func(a, b *cachedKey, c Context) (Ordering, bool) { return comparison("P", a, b, c), true }
	if spec.chain {
		keyOps.ChainingLess = func(a, b *cachedKey, c Context) ChainingLess {
			order := comparison("C", a, b, c)
			return ChainingLess{Continue: order == Equal, Less: order == LessThan}
		}
	}
	environment := rustvec.NewOwned(cachedEnvironment{token: true}, rustvec.ValueOps[cachedEnvironment]{Drop: func(env *cachedEnvironment, c Context) {
		// This witness includes the complete Rust field-drop glue. Token is
		// destroyed even when the Function destructor body starts unwinding.
		defer func() {
			p := recover()
			if env.token {
				env.token = false
				if p != nil {
					unwind := c
					unwind.Unwinding = true
					func() {
						defer func() {
							if recover() != nil {
								unwind.AbortCleanup()
							}
						}()
						tokenDrop(unwind)
					}()
				} else {
					tokenDrop(c)
				}
			}
			if p != nil {
				c.ResumeUnwind(p)
			}
		}()
		event(fmt.Sprintf("FD:%d:%t:%t:%t", env.calls, env.token, env.calls == 0 || env.first == env, c.Panicking()))
		if spec.fnFail {
			raise(c, "function_drop")
		}
	}})
	function := NewKeyFunction(environment, func(env *cachedEnvironment, value *cachedItem, c Context) *rustvec.Owned[cachedKey] {
		if env.calls == 0 {
			env.first = env
		}
		index := env.calls
		env.calls++
		event(fmt.Sprintf("F:%d:%d:%t:%t", value.id, env.calls, env.first == env, c.Panicking()))
		if spec.moved && index == 0 {
			env.token = false
			movedToken = true
			event(fmt.Sprintf("move:%t", c.Panicking()))
		}
		if index == spec.keyFail {
			raise(c, "key_function")
		}
		key := cachedKey{key: value.key, id: value.id}
		if spec.kind == 0 {
			key.key = int64(uint8(value.key))
			key.id = uint64(key.key)
		}
		if spec.kind == 3 {
			key.key, key.id = 0, 0
		}
		return rustvec.NewOwned(key, rustvec.ValueOps[cachedKey]{Drop: keyOps.Drop})
	})
	func() {
		defer func() {
			if p := recover(); p != nil {
				message = fmt.Sprint(p)
			}
		}()
		Cached(values, function, CachedConfig{KeyType: typ, PointerBits: 64}, keyOps, context)
	}()
	event(fmt.Sprintf("end:%t:%t", message != "", context.Panicking()))
	if movedToken {
		tokenDrop(context)
	}
	var out strings.Builder
	for _, value := range values {
		fmt.Fprintf(&out, "%d:%d,", value.id, value.key)
	}
	return message, out.String(), strings.Join(trace, "\n"), compares, drops
}

func TestOriginalCachedSort(t *testing.T) {
	const root = "../../../tools/port-rust-unord/cached/"
	metadata, err := os.ReadFile(root + "manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Counts    map[string]int
		Raw       string `json:"raw_sha256"`
		Fixture   string `json:"fixture_sha256"`
		Oracle    string `json:"oracle_sha256"`
		Generator string `json:"generator_sha256"`
		Shared    string `json:"shared_generator_sha256"`
		Audit     string `json:"source_audit_sha256"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{root + "oracle.rs": manifest.Oracle, root + "generate.py": manifest.Generator, root + "SOURCES.json": manifest.Audit, root + "../sort/generate.py": manifest.Shared} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("cached-key proof binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/cached.tsv.gz")
	if err != nil || digest(compressed) != manifest.Fixture || manifest.Counts["cases"] != 5768 {
		t.Fatalf("original cached-key fixture differs:%v", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 64<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("original cached-key raw fixture differs:%v", err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count := 0
	for scanner.Scan() {
		want := scanner.Text()
		f := strings.Split(want, "\t")
		if len(f) != 15 {
			t.Fatal("invalid cached-key fixture row")
		}
		number := func(index int) int {
			n, err := strconv.Atoi(f[index])
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
		spec := cachedCase{kind: number(0), chain: f[1] == "true", length: number(2), seed: uint64(number(3)), keyFail: number(4), cmpFail: number(5), dropFail: number(6), fnFail: f[7] == "true", tokenFail: f[8] == "true", moved: f[9] == "true"}
		message, state, events, compares, drops := replayCachedCase(spec)
		got := fmt.Sprintf("%d\t%t\t%d\t%d\t%d\t%d\t%d\t%t\t%t\t%t\t%s\t%d\t%d\t%s\t%s", spec.kind, spec.chain, spec.length, spec.seed, spec.keyFail, spec.cmpFail, spec.dropFail, spec.fnFail, spec.tokenFail, spec.moved, hex.EncodeToString([]byte(message)), compares, drops, hex.EncodeToString([]byte(events)), hex.EncodeToString([]byte(state)))
		if got != want {
			expectedEvents, _ := hex.DecodeString(f[13])
			t.Fatalf("original cached-key record%d differs spec%+v\nmessage:%q,want%s; compares:%d,want%s drops:%d,want%s\n got events:%s\nwant events:%s\nstate:%s", count, spec, message, f[10], compares, f[11], drops, f[12], events, expectedEvents, state)
		}
		count++
	}
	if err := scanner.Err(); err != nil || count != manifest.Counts["cases"] {
		t.Fatalf("cached-key coverage differs:%d:%v", count, err)
	}
}
