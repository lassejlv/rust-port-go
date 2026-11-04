// SPDX-License-Identifier: MIT OR Apache-2.0
// PARTIAL port of unhash.rs blob0617ef83aad9c791aa11f7b22540c388024afdd6:
// only Unhasher/Default/Hasher are implemented. The map/set/index-map aliases
// remain unported and receive no source-completion credit from this helper.
package datastructures

import "fmt"

// Unhasher expects one WriteU64; its Go zero value models Rust Default in the
// default debug-assertion profile. The release flag is a profile adapter, not a
// promise about Rust's private struct layout.
type Unhasher struct {
	value   uint64
	release bool
	context FingerprintDropContext
}

func NewUnhasher() *Unhasher                                { return &Unhasher{} }
func NewUnhasherWithDebugAssertions(enabled bool) *Unhasher { return &Unhasher{release: !enabled} }
func (hasher *Unhasher) Finish() uint64                     { return hasher.value }
func (hasher *Unhasher) Write(bytes []byte)                 { hasher.context.Panic("not implemented: use write_u64") }
func (hasher *Unhasher) WriteU64(value uint64) {
	if !hasher.release && hasher.value != 0 {
		hasher.context.Panic(fmt.Sprintf("assertion `left == right` failed: Unhasher doesn't mix values!\n  left: 0\n right: %d", hasher.value))
	}
	hasher.value = value
}

func NewUnhasherWithContext(context FingerprintDropContext, debugAssertions bool) *Unhasher {
	return &Unhasher{release: !debugAssertions, context: context}
}
