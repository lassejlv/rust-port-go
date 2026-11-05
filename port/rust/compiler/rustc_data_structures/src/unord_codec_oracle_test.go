// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

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

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type codecConfig struct {
	kind, length                         int
	seed                                 uint64
	readFail, hashFail, eqFail, dropFail int
	defaultFail                          bool
}
type codecWorld struct {
	config             codecConfig
	events             []string
	hashes, eqs, drops int
}

func (w *codecWorld) event(format string, args ...any) {
	w.events = append(w.events, fmt.Sprintf(format, args...))
}
func (w *codecWorld) drop(kind string, id uint64, c rustfmt.Context) {
	index := w.drops
	w.drops++
	w.event("D:%s:%d:%t", kind, id, c.Panicking())
	if index == w.config.dropFail {
		c.Panic("drop")
	}
}

type codecKey struct{ key, id uint64 }
type codecValue struct{ value, id uint64 }
type codecReader struct {
	rustcserialize.Decoder
	world *codecWorld
	next  int
	seed  uint64
}

func (r *codecReader) ReadUsize() uint {
	r.world.event("LEN:%d:false", r.world.config.length)
	return uint(r.world.config.length)
}
func (r *codecReader) ReadU64() uint64 {
	index := r.next
	r.next++
	v := unordTestRandom(&r.seed)
	if r.world.config.kind%2 == 1 || index%2 == 0 {
		v %= 5
	}
	r.world.event("R:%d:%d:false", index, v)
	if index == r.world.config.readFail {
		panic("read")
	}
	return v
}
func (r *codecReader) Position() uint { return uint(8 + r.next*8) }

type codecHasher struct {
	inner *rusthashmap.FxHasher
	world *codecWorld
}

func (h *codecHasher) Write(b []byte)    { h.inner.Write(b) }
func (h *codecHasher) WriteU64(v uint64) { h.world.event("W:%d:false", v); h.inner.WriteU64(v) }
func (h *codecHasher) Finish64() uint64 {
	v := h.inner.Finish64()
	h.world.event("FIN:%d:false", v)
	return v
}

type codecBuilder struct{ world *codecWorld }

func (b *codecBuilder) Build(c rustfmt.Context) *rusthashmap.OwnedHasher {
	b.world.event("BUILD:%t", c.Panicking())
	h := &codecHasher{rusthashmap.NewFxHasher(), b.world}
	return rusthashmap.NewOwnedHasher(h, func(_ rusthashmap.Hasher, c rustfmt.Context) {
		b.world.event("HD:%d:%t", h.inner.Finish64(), c.Panicking())
	})
}
func (b *codecBuilder) Clone(rustfmt.Context) rusthashmap.Builder { return &codecBuilder{b.world} }
func (b *codecBuilder) CloneFrom(other rusthashmap.Builder, _ rustfmt.Context) {
	b.world = other.(*codecBuilder).world
}
func (b *codecBuilder) Close(c rustfmt.Context) { b.world.event("BD:%t", c.Panicking()) }
func codecMapSnapshot(m *rusthashmap.Map[codecKey, codecValue]) string {
	var s strings.Builder
	s.WriteString("UnordBag { inner: [")
	i := m.Iter(rustfmt.Context{})
	count := 0
	for {
		p, ok := i.Next(rustfmt.Context{})
		if !ok {
			break
		}
		if count > 0 {
			s.WriteString(", ")
		}
		fmt.Fprintf(&s, "(%d, %d, %d, %d)", p.Key.key, p.Key.id, p.Value.value, p.Value.id)
		count++
	}
	s.WriteString("] }")
	return s.String()
}
func codecSetSnapshot(m *rusthashmap.Set[codecKey]) string {
	var s strings.Builder
	s.WriteString("UnordBag { inner: [")
	i := m.Iter(rustfmt.Context{})
	count := 0
	for {
		p, ok := i.Next(rustfmt.Context{})
		if !ok {
			break
		}
		if count > 0 {
			s.WriteString(", ")
		}
		fmt.Fprintf(&s, "(%d, %d)", p.key, p.id)
		count++
	}
	s.WriteString("] }")
	return s.String()
}
func codecVectorSnapshot(raw rustcindex.IndexRaw[codecKey]) string {
	var s strings.Builder
	s.WriteString("UnordBag { inner: [")
	for i := uint(0); i < raw.Len(); i++ {
		if i > 0 {
			s.WriteString(", ")
		}
		value := raw.At(i)
		fmt.Fprintf(&s, "(%d, %d)", value.key, value.id)
	}
	s.WriteString("] }")
	return s.String()
}
func runCodecCase(config codecConfig) (failure, snapshot, events string, reads int) {
	world := &codecWorld{config: config}
	reader := &codecReader{world: world, seed: config.seed}
	keys := rusthashmap.KeyOps[codecKey]{Hash: func(k *codecKey, h rusthashmap.Hasher, c rustfmt.Context) {
		index := world.hashes
		world.hashes++
		world.event("HASH:%d:%d:%t", k.id, k.key, c.Panicking())
		rusthashmap.HashU64(h, k.key)
		if index == config.hashFail {
			c.Panic("hash")
		}
	}, Equal: func(a, b *codecKey, c rustfmt.Context) bool {
		index := world.eqs
		world.eqs++
		world.event("EQ:%d:%d:%t", a.id, b.id, c.Panicking())
		if index == config.eqFail {
			c.Panic("eq")
		}
		return a.key == b.key
	}, Drop: func(k *codecKey, c rustfmt.Context) { world.drop("K", k.id, c) }}
	values := rusthashmap.ValueOps[codecValue]{Drop: func(v *codecValue, c rustfmt.Context) { world.drop("V", v.id, c) }}
	key := func(d rustcserialize.Decoder, c rustfmt.Context) *rustvec.Owned[codecKey] {
		r := d.(*codecReader)
		id := uint64(r.next)
		value := r.ReadU64()
		world.event("K:%d:%d:%t", id, value, c.Panicking())
		return rustvec.NewOwned(codecKey{value, id}, rustvec.ValueOps[codecKey]{Drop: keys.Drop})
	}
	value := func(d rustcserialize.Decoder, c rustfmt.Context) *rustvec.Owned[codecValue] {
		r := d.(*codecReader)
		id := uint64(r.next)
		value := r.ReadU64()
		world.event("V:%d:%d:%t", id, value, c.Panicking())
		return rustvec.NewOwned(codecValue{value, id}, rustvec.ValueOps[codecValue]{Drop: values.Drop})
	}
	builder := func(c rustfmt.Context) rusthashmap.Builder {
		world.event("DEFAULT:%t", c.Panicking())
		if config.defaultFail {
			c.Panic("default")
		}
		return &codecBuilder{world}
	}
	bagConfig := UnordBagConfig[codecKey]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rustcindex.IndexStorageConfig{OverflowChecks: true, DebugAssertions: true}, Values: UnordBagValueOps[codecKey]{Drop: keys.Drop}}
	snapshot = "-"
	func() {
		defer func() {
			if p := recover(); p != nil {
				failure = fmt.Sprint(p)
			}
		}()
		switch config.kind {
		case 0:
			m := rustcserialize.DecodeHashMap(reader, rustcserialize.HashMapDecodeOps[codecKey, codecValue]{Layout: rusthashmap.Layout{Size: 32, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: keys, Values: values, DecodeKey: key, DecodeValue: value, DefaultHasher: builder})
			snapshot = codecMapSnapshot(m)
			m.Close(rustfmt.Context{})
		case 1:
			m := rustcserialize.DecodeHashSet(reader, rustcserialize.HashSetDecodeOps[codecKey]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: keys, DecodeKey: key, DefaultHasher: builder})
			snapshot = codecSetSnapshot(m)
			m.Close(rustfmt.Context{})
		case 2:
			m := DecodeUnordMap(reader, UnordMapConfig[codecKey, codecValue]{Layout: rusthashmap.Layout{Size: 32, Align: 8}, Profile: rusthashmap.DefaultProfile(), Keys: keys, Values: values}, key, value)
			snapshot = codecMapSnapshot(m.inner)
			m.Close()
		case 3:
			m := DecodeUnordSet(reader, UnordSetConfig[codecKey]{Layout: rusthashmap.Layout{Size: 16, Align: 8}, Profile: rusthashmap.DefaultProfile(), Values: keys}, key)
			snapshot = codecSetSnapshot(m.inner)
			m.Close()
		case 4:
			m := rustcserialize.DecodeVector(reader, rustcserialize.VectorDecodeOps[codecKey, *rustcindex.IndexVec[uint, codecKey]]{
				Element: key,
				Collect: func(iterator rustvec.Iterator[codecKey], c rustfmt.Context) *rustcindex.IndexVec[uint, codecKey] {
					return collectDecodedVector(iterator, bagConfig, c)
				},
			})
			snapshot = fmt.Sprintf("Vec { len: %d, capacity: %d, values: %s }", m.Len(), m.Raw.Cap(), codecVectorSnapshot(m.Raw))
			m.CloseWithRuntimeContext(rustfmt.Context{})
		case 5:
			m := DecodeUnordBag(reader, bagConfig, key, nil)
			snapshot = codecVectorSnapshot(m.inner.Raw)
			m.Close()
		default:
			panic("invalid codec kind")
		}
	}()
	return failure, snapshot, strings.Join(world.events, "\n"), reader.next
}
func TestOriginalUnordContainerDecode(t *testing.T) {
	metadata, e := os.ReadFile("../../../../../tools/port-rust-unord/codec/manifest.json")
	if e != nil {
		t.Fatal(e)
	}
	var m struct {
		Cases   int
		Raw     string            `json:"raw_sha256"`
		Fixture string            `json:"fixture_sha256"`
		Files   map[string]string `json:"files"`
	}
	if e = json.Unmarshal(metadata, &m); e != nil {
		t.Fatal(e)
	}
	digest := func(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
	for p, h := range m.Files {
		b, e := os.ReadFile("../../../../../" + p)
		if e != nil || digest(b) != h {
			t.Fatalf("codec binding %s: %v", p, e)
		}
	}
	fixture, e := os.ReadFile("testdata/unord_decode.tsv.gz")
	if e != nil || digest(fixture) != m.Fixture {
		t.Fatal("codec fixture digest", e)
	}
	z, e := gzip.NewReader(bytes.NewReader(fixture))
	if e != nil {
		t.Fatal(e)
	}
	raw, e := io.ReadAll(io.LimitReader(z, 16<<20))
	if e != nil {
		t.Fatal(e)
	}
	if e = z.Close(); e != nil {
		t.Fatal(e)
	}
	if digest(raw) != m.Raw {
		t.Fatal("codec raw digest")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count := 0
	for scanner.Scan() {
		f := strings.Split(scanner.Text(), "\t")
		if len(f) != 12 {
			t.Fatal("codec protocol", len(f))
		}
		integer := func(i int) int {
			v, e := strconv.Atoi(f[i])
			if e != nil {
				t.Fatal(e)
			}
			return v
		}
		seed, e := strconv.ParseUint(f[2], 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		config := codecConfig{integer(0), integer(1), seed, integer(3), integer(4), integer(5), integer(6), f[7] == "true"}
		failure, snapshot, events, reads := runCodecCase(config)
		decode := func(index int) string {
			b, e := hex.DecodeString(f[index])
			if e != nil {
				t.Fatal(e)
			}
			return string(b)
		}
		if failure != decode(8) || reads != integer(9) || snapshot != decode(10) || events != decode(11) {
			t.Fatalf("codec case %d %#v\nfailure/reads %q/%d want %q/%s\nsnapshot %s want %s\nGOT:\n%s\nWANT:\n%s", count, config, failure, reads, decode(8), f[9], snapshot, decode(10), events, decode(11))
		}
		count++
	}
	if scanner.Err() != nil || count != 1674 || m.Cases != 1674 {
		t.Fatal("codec coverage", count, scanner.Err())
	}
}
