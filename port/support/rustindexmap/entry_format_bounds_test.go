// SPDX-License-Identifier: MIT OR Apache-2.0
package rustindexmap

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"strings"
	"testing"
)

type entryBoundsHasher uint64

func (h *entryBoundsHasher) Write([]byte)      { panic("unexpected raw Hash write") }
func (h *entryBoundsHasher) WriteU64(v uint64) { *h = entryBoundsHasher(v) }
func (h *entryBoundsHasher) Finish64() uint64  { return uint64(*h) }
func TestVacantEntryDebugDoesNotRequireValueDebug(t *testing.T) {
	type undebug struct{ data []byte }
	config := Config[uint64, undebug]{BucketLayout: Layout{Size: 40, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true, Keys: KeyOps[uint64]{KeyOps: rusthashmap.KeyOps[uint64]{Hash: func(k *uint64, h Hasher, _ Context) { rusthashmap.HashU64(h, *k) }, Equal: func(a, b *uint64, _ Context) bool { return *a == *b }, Debug: func(k *uint64, f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(*k, 64, f) }}}}
	builder := NewOwnedBuilder(rustvec.Own(struct{}{}), BuilderOps[struct{}]{Build: func(rustvec.Ref[struct{}]) *rusthashmap.OwnedHasher {
		return rusthashmap.NewOwnedHasher(new(entryBoundsHasher), nil)
	}})
	m := WithHasher(config, builder)
	defer m.Close()
	vacant, ok := m.Entry(7).TakeVacant(Context{})
	if !ok {
		t.Fatal("unexpected occupied entry")
	}
	if got := rustfmt.String(vacant.WriteDebugFormatter); got != "VacantEntry(7)" {
		t.Fatal(got)
	}
	vacant.Close(Context{})
	entry := m.Entry(8)
	defer entry.Close(Context{})
	writes := 0
	var payload any
	func() {
		defer func() { payload = recover() }()
		entry.WriteDebugFormatter(&rustfmt.Formatter{Options: rustfmt.DefaultOptions(), Writer: entryBoundWriter{count: &writes}})
	}()
	if payload == nil || !strings.Contains(payload.(string), "unavailable source entry Debug traits") || writes != 0 {
		t.Fatal("Entry enum fabricated unavailable V Debug or wrote before rejecting it")
	}
}

type entryBoundWriter struct{ count *int }

func (w entryBoundWriter) Write(value []byte) (int, error) { *w.count++; return len(value), nil }
