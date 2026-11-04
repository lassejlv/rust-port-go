// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: semver 1.0.28 src/eval.rs; prerelease gating is Cargo's exact policy.
package semver

func (req VersionReq) Matches(version Version) bool {
	for _, comparator := range req.Comparators {
		if !matchesImpl(comparator, version) {
			return false
		}
	}
	if version.Pre.IsEmpty() {
		return true
	}
	for _, comparator := range req.Comparators {
		if preIsCompatible(comparator, version) {
			return true
		}
	}
	return false
}
func (comparator Comparator) Matches(version Version) bool {
	return matchesImpl(comparator, version) && (version.Pre.IsEmpty() || preIsCompatible(comparator, version))
}
func matchesImpl(comparator Comparator, version Version) bool {
	switch comparator.Op {
	case Exact, Wildcard:
		return matchesExact(comparator, version)
	case Greater:
		return matchesGreater(comparator, version)
	case GreaterEq:
		return matchesExact(comparator, version) || matchesGreater(comparator, version)
	case Less:
		return matchesLess(comparator, version)
	case LessEq:
		return matchesExact(comparator, version) || matchesLess(comparator, version)
	case Tilde:
		return matchesTilde(comparator, version)
	case Caret:
		return matchesCaret(comparator, version)
	default:
		panic("semver: invalid Op")
	}
}
func matchesExact(comparator Comparator, version Version) bool {
	if version.Major != comparator.Major {
		return false
	}
	if comparator.Minor.Present && version.Minor != comparator.Minor.Value {
		return false
	}
	if comparator.Patch.Present && version.Patch != comparator.Patch.Value {
		return false
	}
	return version.Pre.Equal(comparator.Pre)
}
func matchesGreater(comparator Comparator, version Version) bool {
	if version.Major != comparator.Major {
		return version.Major > comparator.Major
	}
	if !comparator.Minor.Present {
		return false
	}
	if version.Minor != comparator.Minor.Value {
		return version.Minor > comparator.Minor.Value
	}
	if !comparator.Patch.Present {
		return false
	}
	if version.Patch != comparator.Patch.Value {
		return version.Patch > comparator.Patch.Value
	}
	return version.Pre.Compare(comparator.Pre) > 0
}
func matchesLess(comparator Comparator, version Version) bool {
	if version.Major != comparator.Major {
		return version.Major < comparator.Major
	}
	if !comparator.Minor.Present {
		return false
	}
	if version.Minor != comparator.Minor.Value {
		return version.Minor < comparator.Minor.Value
	}
	if !comparator.Patch.Present {
		return false
	}
	if version.Patch != comparator.Patch.Value {
		return version.Patch < comparator.Patch.Value
	}
	return version.Pre.Compare(comparator.Pre) < 0
}
func matchesTilde(comparator Comparator, version Version) bool {
	if version.Major != comparator.Major {
		return false
	}
	if comparator.Minor.Present && version.Minor != comparator.Minor.Value {
		return false
	}
	if comparator.Patch.Present && version.Patch != comparator.Patch.Value {
		return version.Patch > comparator.Patch.Value
	}
	return version.Pre.Compare(comparator.Pre) >= 0
}
func matchesCaret(comparator Comparator, version Version) bool {
	if version.Major != comparator.Major {
		return false
	}
	if !comparator.Minor.Present {
		return true
	}
	minor := comparator.Minor.Value
	if !comparator.Patch.Present {
		if comparator.Major > 0 {
			return version.Minor >= minor
		}
		return version.Minor == minor
	}
	patch := comparator.Patch.Value
	if comparator.Major > 0 {
		if version.Minor != minor {
			return version.Minor > minor
		}
		if version.Patch != patch {
			return version.Patch > patch
		}
	} else if minor > 0 {
		if version.Minor != minor {
			return false
		}
		if version.Patch != patch {
			return version.Patch > patch
		}
	} else if version.Minor != minor || version.Patch != patch {
		return false
	}
	return version.Pre.Compare(comparator.Pre) >= 0
}
func preIsCompatible(comparator Comparator, version Version) bool {
	return comparator.Major == version.Major && comparator.Minor.Present && comparator.Minor.Value == version.Minor && comparator.Patch.Present && comparator.Patch.Value == version.Patch && !comparator.Pre.IsEmpty()
}
