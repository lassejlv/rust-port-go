// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: semver 1.0.28 src/lib.rs, official crate SHA-256
// 8a7852d02fc848982e0c167ef163aaff9cd91dc640ba85e263cb1ce46fae51cd.
// See the enclosing source-manifest.json for file-level provenance.

// Package semver implements Cargo's Semantic Versioning parser and evaluator.
// Inputs must be valid UTF-8, matching Rust's &str contract. Identifiers retain
// immutable string values instead of Rust's private inline/heap representation.
package semver

// Option represents Rust Option without conflating None with Some(0).
type Option[T any] struct {
	Value   T
	Present bool
}

func Some[T any](value T) Option[T] { return Option[T]{Value: value, Present: true} }

type Version struct {
	Major, Minor, Patch uint64
	Pre                 Prerelease
	Build               BuildMetadata
}
type VersionReq struct{ Comparators []Comparator }
type Comparator struct {
	Op           Op
	Major        uint64
	Minor, Patch Option[uint64]
	Pre          Prerelease
}
type Op uint8

const (
	Exact Op = iota
	Greater
	GreaterEq
	Less
	LessEq
	Tilde
	Caret
	Wildcard
)

type Prerelease struct{ identifier identifier }
type BuildMetadata struct{ identifier identifier }

func NewVersion(major, minor, patch uint64) Version {
	return Version{Major: major, Minor: minor, Patch: patch}
}

// Star is the immutable VersionReq::STAR constant represented by a fresh value.
// The zero VersionReq also has its source Default semantics.
func Star() VersionReq                    { return VersionReq{} }
func EmptyPrerelease() Prerelease         { return Prerelease{} }
func EmptyBuildMetadata() BuildMetadata   { return BuildMetadata{} }
func (value Prerelease) AsStr() string    { return value.identifier.text }
func (value Prerelease) Len() int         { return len(value.identifier.text) }
func (value Prerelease) IsEmpty() bool    { return value.identifier.text == "" }
func (value BuildMetadata) AsStr() string { return value.identifier.text }
func (value BuildMetadata) Len() int      { return len(value.identifier.text) }
func (value BuildMetadata) IsEmpty() bool { return value.identifier.text == "" }
