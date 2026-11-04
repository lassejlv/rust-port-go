// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete port of Cargo src/util/semver_ext.rs at
// 24de4721844c7a4d839072bab6fb66d41c9bc1f4.
package util

import (
	"fmt"
	"io"
	"strings"

	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
)

// VersionExt is the required part of Cargo's public extension trait. The free
// VersionToExactReq and VersionToCaretReq functions apply the trait defaults,
// honoring optional methods with those names on custom implementers. Defaults
// dispatch the implementing object's ToReq; they do not require a concrete
// semver.Version or consult IsPrerelease.
type VersionExt interface {
	IsPrerelease() bool
	ToReq(semver.Op) semver.VersionReq
}

type VersionAdapter struct{ Version semver.Version }

func (v VersionAdapter) IsPrerelease() bool { return !v.Version.Pre.IsEmpty() }
func (v VersionAdapter) ToReq(op semver.Op) semver.VersionReq {
	return semver.VersionReq{Comparators: []semver.Comparator{{Op: op, Major: v.Version.Major, Minor: semver.Some(v.Version.Minor), Patch: semver.Some(v.Version.Patch), Pre: v.Version.Pre.Clone()}}}
}

func VersionToExactReq(v VersionExt) semver.VersionReq {
	if custom, ok := v.(interface{ ToExactReq() semver.VersionReq }); ok {
		return custom.ToExactReq()
	}
	return v.ToReq(semver.Exact)
}

func VersionToCaretReq(v VersionExt) semver.VersionReq {
	if custom, ok := v.(interface{ ToCaretReq() semver.VersionReq }); ok {
		return custom.ToCaretReq()
	}
	return v.ToReq(semver.Caret)
}

// VersionReqExt corresponds to the public trait, including implementations for
// arbitrary custom Go types. VersionReqAdapter supplies the concrete semver
// implementation; its default overflow profile is checked.
type VersionReqExt interface {
	MatchesPrerelease(semver.Version) bool
}

type VersionReqAdapter struct{ Req semver.VersionReq }

func (r VersionReqAdapter) MatchesPrerelease(v semver.Version) bool {
	return MatchesPrerelease(r.Req, v)
}

type VersionReqMatchMode uint8

const (
	VersionReqDefault VersionReqMatchMode = iota
	VersionReqPrerelease
)

func (m VersionReqMatchMode) Clone() VersionReqMatchMode { return m }
func (m VersionReqMatchMode) RustDebug() string {
	switch m {
	case VersionReqDefault:
		return "Default"
	case VersionReqPrerelease:
		return "Prerelease"
	default:
		panic("semver_ext: invalid match mode")
	}
}

func (m VersionReqMatchMode) RustDebugWith(_ semver.DebugOptions) string { return m.RustDebug() }
func (m VersionReqMatchMode) WriteDebug(w io.Writer, options semver.DebugOptions) error {
	return m.WriteDebugFormatter(&semver.DebugFormatter{Writer: w, Options: options})
}
func (m VersionReqMatchMode) WriteDebugFormatter(f *semver.DebugFormatter) error {
	return semverOptWriteText(f.Writer, m.RustDebug())
}

type OptVersionReqKind uint8

const (
	OptVersionAny OptVersionReqKind = iota
	OptVersionRequirement
	OptVersionLocked
	OptVersionPrecise
)

// OptVersionReq mirrors Cargo's public enum. Req is active in every variant
// except Any; Version is active only in Locked and Precise. Inactive payloads
// are ignored by Clone, equality, Hash and formatting. The zero value is Any.
// Constructors take independent copies of slice-bearing requirements. Direct
// struct literals and borrowed getters require Rust-equivalent alias discipline
// from Go callers; concurrent mutation is unsupported.
type OptVersionReq struct {
	Kind    OptVersionReqKind
	Version semver.Version
	Req     semver.VersionReq
}

func FromVersionReq(req semver.VersionReq) OptVersionReq {
	return OptVersionReq{Kind: OptVersionRequirement, Req: req.Clone()}
}
func LockedVersionReq(version semver.Version, req semver.VersionReq) OptVersionReq {
	return OptVersionReq{Kind: OptVersionLocked, Version: version.Clone(), Req: req.Clone()}
}
func PreciseVersionReq(version semver.Version, req semver.VersionReq) OptVersionReq {
	return OptVersionReq{Kind: OptVersionPrecise, Version: version.Clone(), Req: req.Clone()}
}
func ExactVersionReq(version semver.Version) OptVersionReq {
	return FromVersionReq(VersionToExactReq(VersionAdapter{version}))
}
func LockToExactVersionReq(version semver.Version) OptVersionReq {
	return LockedVersionReq(version, VersionToExactReq(VersionAdapter{version}))
}

func (r OptVersionReq) IsExact() bool {
	switch r.Kind {
	case OptVersionAny:
		return false
	case OptVersionRequirement, OptVersionPrecise:
		if len(r.Req.Comparators) != 1 {
			return false
		}
		cmp := r.Req.Comparators[0]
		return cmp.Op == semver.Exact && cmp.Minor.Present && cmp.Patch.Present
	case OptVersionLocked:
		return true
	default:
		panic("semver_ext: invalid optional requirement variant")
	}
}

func (r *OptVersionReq) LockTo(version semver.Version, mode VersionReqMatchMode) {
	r.LockToWithOverflow(version, mode, SemverCheckedOverflow)
}
func (r *OptVersionReq) LockToWithOverflow(version semver.Version, mode VersionReqMatchMode, overflow SemverOverflowMode) {
	if !r.MatchesWithOverflow(version, mode, overflow) {
		panic(fmt.Sprintf("cannot lock %s to %s", r.String(), version.String()))
	}
	version = version.Clone()
	switch r.Kind {
	case OptVersionAny:
		*r = OptVersionReq{Kind: OptVersionLocked, Version: version}
	case OptVersionRequirement, OptVersionLocked, OptVersionPrecise:
		*r = OptVersionReq{Kind: OptVersionLocked, Version: version, Req: r.Req.Clone()}
	default:
		panic("semver_ext: invalid optional requirement variant")
	}
}

func (r *OptVersionReq) PreciseTo(version semver.Version) {
	version = version.Clone()
	switch r.Kind {
	case OptVersionAny:
		*r = OptVersionReq{Kind: OptVersionPrecise, Version: version}
	case OptVersionRequirement, OptVersionLocked, OptVersionPrecise:
		*r = OptVersionReq{Kind: OptVersionPrecise, Version: version, Req: r.Req.Clone()}
	default:
		panic("semver_ext: invalid optional requirement variant")
	}
}

func (r OptVersionReq) IsPrecise() bool { return r.Kind == OptVersionPrecise }
func (r OptVersionReq) IsLocked() bool  { return r.Kind == OptVersionLocked }

// PreciseVersion and LockedVersion return borrowed payloads. Callers must treat
// the pointers as read-only and must not mutate the enum while using them.
func (r *OptVersionReq) PreciseVersion() (*semver.Version, bool) {
	if r.Kind == OptVersionPrecise {
		return &r.Version, true
	}
	return nil, false
}
func (r *OptVersionReq) LockedVersion() (*semver.Version, bool) {
	if r.Kind == OptVersionLocked {
		return &r.Version, true
	}
	return nil, false
}

func (r OptVersionReq) Matches(version semver.Version, mode VersionReqMatchMode) bool {
	return r.MatchesWithOverflow(version, mode, SemverCheckedOverflow)
}
func (r OptVersionReq) MatchesWithOverflow(version semver.Version, mode VersionReqMatchMode, overflow SemverOverflowMode) bool {
	switch r.Kind {
	case OptVersionAny:
		return true
	case OptVersionRequirement:
		switch mode {
		case VersionReqDefault:
			return r.Req.Matches(version)
		case VersionReqPrerelease:
			return r.Req.Matches(version) || (!version.Pre.IsEmpty() && MatchesPrereleaseWithOverflow(r.Req, version, overflow))
		default:
			panic("semver_ext: invalid match mode")
		}
	case OptVersionLocked:
		// Locked versions retain build metadata to preserve reproducibility.
		return r.Version.Equal(version)
	case OptVersionPrecise:
		// A precise version without build metadata permits any metadata.
		v := r.Version
		return v.Major == version.Major && v.Minor == version.Minor && v.Patch == version.Patch && v.Pre.Equal(version.Pre) && (v.Build.Equal(version.Build) || v.Build.IsEmpty())
	default:
		panic("semver_ext: invalid optional requirement variant")
	}
}

func (r OptVersionReq) String() string {
	switch r.Kind {
	case OptVersionAny:
		return "*"
	case OptVersionRequirement, OptVersionLocked, OptVersionPrecise:
		return r.Req.String()
	default:
		panic("semver_ext: invalid optional requirement variant")
	}
}

// WriteDisplay preserves the source formatter's writes and early error return.
// Any writes one literal; other variants delegate to the original requirement
// formatter. Neither applies the caller's outer width or precision padding.
func (r OptVersionReq) WriteDisplay(w io.Writer, options semver.FormatOptions) error {
	switch r.Kind {
	case OptVersionAny:
		n, err := io.WriteString(w, "*")
		if err == nil && n != 1 {
			return io.ErrShortWrite
		}
		return err
	case OptVersionRequirement, OptVersionLocked, OptVersionPrecise:
		return r.Req.WriteDisplay(w, options)
	default:
		panic("semver_ext: invalid optional requirement variant")
	}
}

// Format follows the dependency's Go bridge: ordinary verbs use Display,
// ignoring outer precision as the source does; %#v selects compact Rust Debug.
// Full alternate/hex Debug is available through RustDebugWith.
func (r OptVersionReq) Format(state fmt.State, verb rune) {
	if verb == 'v' && state.Flag('#') {
		_, _ = io.WriteString(state, r.RustDebug())
		return
	}
	options := semver.FormatOptions{}
	if width, present := state.Width(); present {
		options.Width = semver.Some(uint(width))
	}
	_ = r.WriteDisplay(state, options)
}
func (r OptVersionReq) Clone() OptVersionReq {
	switch r.Kind {
	case OptVersionAny:
		return OptVersionReq{}
	case OptVersionRequirement:
		return FromVersionReq(r.Req)
	case OptVersionLocked:
		return LockedVersionReq(r.Version, r.Req)
	case OptVersionPrecise:
		return PreciseVersionReq(r.Version, r.Req)
	default:
		panic("semver_ext: invalid optional requirement variant")
	}
}
func (r OptVersionReq) Equal(other OptVersionReq) bool {
	if r.Kind != other.Kind {
		return false
	}
	switch r.Kind {
	case OptVersionAny:
		return true
	case OptVersionRequirement:
		return r.Req.Equal(other.Req)
	case OptVersionLocked, OptVersionPrecise:
		return r.Version.Equal(other.Version) && r.Req.Equal(other.Req)
	default:
		panic("semver_ext: invalid optional requirement variant")
	}
}
func (r OptVersionReq) RustDebug() string {
	return r.RustDebugWith(semver.DebugOptions{})
}

func semverOptWriteText(w io.Writer, text string) error {
	n, err := io.WriteString(w, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}

func (r OptVersionReq) RustDebugWith(options semver.DebugOptions) string {
	var result strings.Builder
	_ = r.WriteDebug(&result, options)
	return result.String()
}

// WriteDebug delegates to the validated standard Debug tuple builder and the
// concrete fields' streaming Debug implementations. Caller flags, real empty
// writes, nested indentation, and the first writer failure remain observable.
func (r OptVersionReq) WriteDebug(w io.Writer, options semver.DebugOptions) error {
	return r.WriteDebugFormatter(&semver.DebugFormatter{Writer: w, Options: options})
}

// WriteDebugFormatter preserves the actual mutable outer Formatter state.
// Compact fields share it; alternate fields use the source builder's copies.
func (r OptVersionReq) WriteDebugFormatter(f *semver.DebugFormatter) error {
	switch r.Kind {
	case OptVersionAny:
		return semverOptWriteText(f.Writer, "Any")
	case OptVersionRequirement:
		return semver.WriteDebugTupleFormatter(f, "Req", []semver.DebugFormatterFunc{r.Req.WriteDebugFormatter})
	case OptVersionLocked:
		return semver.WriteDebugTupleFormatter(f, "Locked", []semver.DebugFormatterFunc{r.Version.WriteDebugFormatter, r.Req.WriteDebugFormatter})
	case OptVersionPrecise:
		return semver.WriteDebugTupleFormatter(f, "Precise", []semver.DebugFormatterFunc{r.Version.WriteDebugFormatter, r.Req.WriteDebugFormatter})
	default:
		panic("semver_ext: invalid optional requirement variant")
	}
}

// Hash preserves the derived enum discriminant and active-field hash order,
// delegating primitive writes to semver's complete Rust Hasher adapter.
func (r OptVersionReq) Hash(h semver.Hasher) {
	semver.HashIsize(h, int(r.Kind))
	switch r.Kind {
	case OptVersionAny:
	case OptVersionRequirement:
		r.Req.Hash(h)
	case OptVersionLocked, OptVersionPrecise:
		r.Version.Hash(h)
		r.Req.Hash(h)
	default:
		panic("semver_ext: invalid optional requirement variant")
	}
}
