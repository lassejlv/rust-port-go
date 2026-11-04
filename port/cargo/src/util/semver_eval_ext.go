// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete port of Cargo src/util/semver_eval_ext.rs at
// 24de4721844c7a4d839072bab6fb66d41c9bc1f4.
// RFC 3493 prerelease matching retains the original current behavior, including
// its documented unresolved upper-bound semantics.
package util

import semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"

// SemverOverflowMode specifies the original Cargo build's arithmetic profile.
// Default entry points use Rust's checked/debug u64 arithmetic. The explicit
// wrapping profile reproduces a build with overflow checks disabled.
type SemverOverflowMode uint8

const (
	SemverCheckedOverflow SemverOverflowMode = iota
	SemverWrappingOverflow
)

func semverIncrement(value uint64, overflow SemverOverflowMode) uint64 {
	if overflow == SemverCheckedOverflow && value == ^uint64(0) {
		panic("semver_eval_ext: attempt to add with overflow")
	}
	return value + 1
}

func MatchesPrerelease(req semver.VersionReq, ver semver.Version) bool {
	return MatchesPrereleaseWithOverflow(req, ver, SemverCheckedOverflow)
}

func MatchesPrereleaseWithOverflow(req semver.VersionReq, ver semver.Version, overflow SemverOverflowMode) bool {
	lowerBoundPrerelease := false
	for _, cmp := range req.Comparators {
		if (cmp.Op == semver.Greater || cmp.Op == semver.GreaterEq) && !cmp.Pre.IsEmpty() {
			lowerBoundPrerelease = true
			break
		}
	}
	for _, cmp := range req.Comparators {
		if !semverMatchesPrereleaseImpl(cmp, ver, lowerBoundPrerelease, overflow) {
			return false
		}
	}
	return true
}

func semverMatchesPrereleaseImpl(cmp semver.Comparator, ver semver.Version, lowerBoundPrerelease bool, overflow SemverOverflowMode) bool {
	switch cmp.Op {
	case semver.Exact, semver.Wildcard:
		return semverMatchesExactPrerelease(cmp, ver, overflow)
	case semver.Greater:
		return semverMatchesGreater(cmp, ver)
	case semver.GreaterEq:
		if semverMatchesExactPrerelease(cmp, ver, overflow) {
			return true
		}
		return semverMatchesGreater(cmp, ver)
	case semver.Less:
		if lowerBoundPrerelease {
			return semverMatchesLess(semverFillPartialReq(cmp), ver)
		}
		return semverMatchesLess(semverFillPartialReqIncludePre(cmp), ver)
	case semver.LessEq:
		if semverMatchesExactPrerelease(cmp, ver, overflow) {
			return true
		}
		return semverMatchesLess(semverFillPartialReq(cmp), ver)
	case semver.Tilde:
		return semverMatchesTildePrerelease(cmp, ver, overflow)
	case semver.Caret:
		return semverMatchesCaretPrerelease(cmp, ver, overflow)
	default:
		panic("semver_eval_ext: internal error: entered unreachable code")
	}
}

func semverMatchesExact(cmp semver.Comparator, ver semver.Version) bool {
	if ver.Major != cmp.Major {
		return false
	}
	if cmp.Minor.Present && ver.Minor != cmp.Minor.Value {
		return false
	}
	if cmp.Patch.Present && ver.Patch != cmp.Patch.Value {
		return false
	}
	return ver.Pre.Equal(cmp.Pre)
}

func semverMatchesGreater(cmp semver.Comparator, ver semver.Version) bool {
	if ver.Major != cmp.Major {
		return ver.Major > cmp.Major
	}
	if !cmp.Minor.Present {
		return false
	}
	if ver.Minor != cmp.Minor.Value {
		return ver.Minor > cmp.Minor.Value
	}
	if !cmp.Patch.Present {
		return false
	}
	if ver.Patch != cmp.Patch.Value {
		return ver.Patch > cmp.Patch.Value
	}
	return ver.Pre.Compare(cmp.Pre) > 0
}

func semverMatchesLess(cmp semver.Comparator, ver semver.Version) bool {
	if ver.Major != cmp.Major {
		return ver.Major < cmp.Major
	}
	if !cmp.Minor.Present {
		return false
	}
	if ver.Minor != cmp.Minor.Value {
		return ver.Minor < cmp.Minor.Value
	}
	if !cmp.Patch.Present {
		return false
	}
	if ver.Patch != cmp.Patch.Value {
		return ver.Patch < cmp.Patch.Value
	}
	return ver.Pre.Compare(cmp.Pre) < 0
}

func semverFillPartialReq(cmp semver.Comparator) semver.Comparator {
	cmp = cmp.Clone()
	if !cmp.Minor.Present {
		cmp.Minor = semver.Some(uint64(0))
		cmp.Patch = semver.Some(uint64(0))
	} else if !cmp.Patch.Present {
		cmp.Patch = semver.Some(uint64(0))
	}
	return cmp
}

func semverFillPartialReqIncludePre(cmp semver.Comparator) semver.Comparator {
	cmp = cmp.Clone()
	if !cmp.Minor.Present {
		cmp.Minor = semver.Some(uint64(0))
		cmp.Patch = semver.Some(uint64(0))
		cmp.Pre = semverZeroPrerelease()
	} else if !cmp.Patch.Present {
		cmp.Patch = semver.Some(uint64(0))
	}
	if cmp.Pre.IsEmpty() {
		cmp.Pre = semverZeroPrerelease()
	}
	return cmp
}

func semverZeroPrerelease() semver.Prerelease {
	value, err := semver.NewPrerelease("0")
	if err != nil {
		panic(err)
	}
	return value
}

func semverMatchesExactPrerelease(cmp semver.Comparator, ver semver.Version, overflow SemverOverflowMode) bool {
	if semverMatchesExact(cmp, ver) {
		return true
	}
	if !cmp.Pre.IsEmpty() {
		return false
	}
	if !semverMatchesGreater(semverFillPartialReq(cmp), ver) {
		return false
	}
	upper := cmp.Clone()
	upper.Op, upper.Pre = semver.Less, semverZeroPrerelease()
	switch {
	case upper.Minor.Present && upper.Patch.Present:
		upper.Patch = semver.Some(semverIncrement(upper.Patch.Value, overflow))
	case upper.Minor.Present && !upper.Patch.Present:
		upper.Minor = semver.Some(semverIncrement(upper.Minor.Value, overflow))
		upper.Patch = semver.Some(uint64(0))
	case !upper.Minor.Present && !upper.Patch.Present:
		upper.Major = semverIncrement(upper.Major, overflow)
		upper.Minor, upper.Patch = semver.Some(uint64(0)), semver.Some(uint64(0))
	}
	return semverMatchesLess(upper, ver)
}

func semverMatchesTildePrerelease(cmp semver.Comparator, ver semver.Version, overflow SemverOverflowMode) bool {
	if semverMatchesExact(cmp, ver) {
		return true
	}
	if !semverMatchesGreater(semverFillPartialReq(cmp), ver) {
		return false
	}
	upper := cmp.Clone()
	upper.Op, upper.Pre = semver.Less, semverZeroPrerelease()
	switch {
	case upper.Minor.Present:
		upper.Minor = semver.Some(semverIncrement(upper.Minor.Value, overflow))
		upper.Patch = semver.Some(uint64(0))
	case !upper.Minor.Present && !upper.Patch.Present:
		upper.Major = semverIncrement(upper.Major, overflow)
		upper.Minor, upper.Patch = semver.Some(uint64(0)), semver.Some(uint64(0))
	}
	return semverMatchesLess(upper, ver)
}

func semverMatchesCaretPrerelease(cmp semver.Comparator, ver semver.Version, overflow SemverOverflowMode) bool {
	if semverMatchesExact(cmp, ver) {
		return true
	}
	if !semverMatchesGreater(semverFillPartialReq(cmp), ver) {
		return false
	}
	upper := cmp.Clone()
	upper.Op, upper.Pre = semver.Less, semverZeroPrerelease()
	switch {
	case upper.Major > 0 || (!upper.Minor.Present && !upper.Patch.Present):
		upper.Major = semverIncrement(upper.Major, overflow)
		upper.Minor, upper.Patch = semver.Some(uint64(0)), semver.Some(uint64(0))
	case upper.Minor.Present && !upper.Patch.Present:
		upper.Minor = semver.Some(semverIncrement(upper.Minor.Value, overflow))
		upper.Patch = semver.Some(uint64(0))
	case upper.Minor.Present && upper.Minor.Value > 0:
		upper.Minor = semver.Some(semverIncrement(upper.Minor.Value, overflow))
		upper.Patch = semver.Some(uint64(0))
	case upper.Minor.Present && upper.Minor.Value == 0:
		if !upper.Patch.Present {
			upper.Patch = semver.Some(uint64(1))
		} else {
			upper.Patch = semver.Some(semverIncrement(upper.Patch.Value, overflow))
		}
	}
	return semverMatchesLess(upper, ver)
}
