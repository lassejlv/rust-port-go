// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL genuine StableCrateIdMap binding. The currently supported operations
// are actual indexmap2.14.0 operations, not an unrelated map implementation.
// Remaining IndexMap APIs and higher Unord layers still block whole def_id.rs
// completion; see tools/port-rust-indexmap.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusthashmap"
	"github.com/lassejlv/rust-port-go/port/support/rustindexmap"
)

type StableCrateIdMap = rustindexmap.Map[StableCrateId, CrateNum]

// Configuration is source target/codegen metadata, not extra fields of the
// Rust BuildHasherDefault value. Its profile must describe actual Bucket and
// usize layouts. The default constructor uses the recorded Linux64 profile.
type StableCrateIdMapConfig struct {
	BucketLayout    rusthashmap.Layout
	Profile         rusthashmap.Profile
	DebugAssertions bool
}

func DefaultStableCrateIdMapConfig() StableCrateIdMapConfig {
	return StableCrateIdMapConfig{BucketLayout: rusthashmap.Layout{Size: 24, Align: 8}, Profile: rusthashmap.DefaultProfile(), DebugAssertions: true}
}

type stableCrateIdBuilder struct{ debug bool }

func (b stableCrateIdBuilder) Build(context rustfmt.Context) *rusthashmap.OwnedHasher {
	return rusthashmap.NewOwnedHasher(datastructures.NewUnhasherWithContext(context, b.debug), nil)
}
func (b stableCrateIdBuilder) Clone(rustfmt.Context) rusthashmap.Builder      { return b }
func (b stableCrateIdBuilder) CloneFrom(rusthashmap.Builder, rustfmt.Context) {}
func (b stableCrateIdBuilder) Close(rustfmt.Context)                          {}
func stableCrateIdMapConfig(config StableCrateIdMapConfig) rustindexmap.Config[StableCrateId, CrateNum] {
	return rustindexmap.Config[StableCrateId, CrateNum]{BucketLayout: config.BucketLayout, Profile: config.Profile, DebugAssertions: config.DebugAssertions,
		Keys: rustindexmap.KeyOps[StableCrateId]{KeyOps: rusthashmap.KeyOps[StableCrateId]{
			Hash:  func(value *StableCrateId, h rusthashmap.Hasher, _ rustfmt.Context) { value.HashWithHasher(h) },
			Equal: func(a, b *StableCrateId, _ rustfmt.Context) bool { return a.Equal(*b) }, TrivialClone: true,
			Debug: func(value *StableCrateId, f *rustfmt.Formatter) error { return value.WriteDebugFormatter(f) },
		}}, Values: rustindexmap.ValueOps[CrateNum]{ValueOps: rusthashmap.ValueOps[CrateNum]{
			Equal: func(a, b *CrateNum, _ rustfmt.Context) bool { return a.Equal(*b) }, TrivialClone: true,
			Debug: func(value *CrateNum, f *rustfmt.Formatter) error { return value.WriteDebugFormatter(f) },
		}},
	}
}
func NewStableCrateIdMap(contexts ...rustfmt.Context) *StableCrateIdMap {
	return NewStableCrateIdMapWithCapacity(0, contexts...)
}
func NewStableCrateIdMapWithCapacity(capacity uint, contexts ...rustfmt.Context) *StableCrateIdMap {
	return NewStableCrateIdMapWithConfig(capacity, DefaultStableCrateIdMapConfig(), contexts...)
}
func NewStableCrateIdMapWithConfig(capacity uint, config StableCrateIdMapConfig, contexts ...rustfmt.Context) *StableCrateIdMap {
	return rustindexmap.WithCapacityAndHasher(capacity, stableCrateIdMapConfig(config), stableCrateIdBuilder{config.DebugAssertions}, contexts...)
}
