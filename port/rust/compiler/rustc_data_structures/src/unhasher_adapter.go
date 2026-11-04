// SPDX-License-Identifier: MIT OR Apache-2.0
// Partial std Hasher naming bridge used by the scoped collection runtime.
// This does not add whole-file credit for unhash.rs or any map alias.
package datastructures

// Finish64 keeps the actual concrete Unhasher identity, including the source
// Fingerprint Hash specialization, while satisfying the shared Hasher adapter.
func (hasher *Unhasher) Finish64() uint64 { return hasher.Finish() }
