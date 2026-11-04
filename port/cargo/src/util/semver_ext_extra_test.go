package util

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
)

func TestOptVersionReqBuildMetadataAndBorrowedPayloads(t *testing.T) {
	foo := semverUpstreamVersion(t, "1.2.3+foo")
	bar := semverUpstreamVersion(t, "1.2.3+bar")
	plain := semverUpstreamVersion(t, "1.2.3")
	req := semverUpstreamReq(t, "^1.2.3")
	for _, item := range []struct {
		value OptVersionReq
		want  bool
	}{{ExactVersionReq(foo), true}, {LockToExactVersionReq(foo), false}, {PreciseVersionReq(plain, req), true}, {PreciseVersionReq(foo, req), false}} {
		for _, mode := range []VersionReqMatchMode{VersionReqDefault, VersionReqPrerelease} {
			if got := item.value.Matches(bar, mode); got != item.want {
				t.Fatalf("%s matched other build=%v, want %v", item.value.RustDebug(), got, item.want)
			}
		}
	}
	locked := LockToExactVersionReq(foo)
	if version, ok := locked.LockedVersion(); !ok || version != &locked.Version || !version.Equal(foo) {
		t.Fatal("locked_version must borrow the active inner version")
	}
	if version, ok := locked.PreciseVersion(); ok || version != nil {
		t.Fatal("locked requirement has no precise_version")
	}
	precise := PreciseVersionReq(foo, req)
	if version, ok := precise.PreciseVersion(); !ok || version != &precise.Version || !version.Equal(foo) {
		t.Fatal("precise_version must borrow the active inner version")
	}
	if locked.Equal(precise) {
		t.Fatal("different active enum variants must differ")
	}
}

func TestOptVersionReqOwnershipCloneAndMutation(t *testing.T) {
	req := semverUpstreamReq(t, "^1.2.3")
	value := FromVersionReq(req)
	req.Comparators[0].Major = 99
	if value.String() != "^1.2.3" {
		t.Fatal("constructor retained mutable caller-owned requirement storage")
	}
	clone := value.Clone()
	clone.Req.Comparators[0].Major = 7
	if value.String() != "^1.2.3" || clone.Equal(value) {
		t.Fatal("clone did not independently own its requirement")
	}
	before := value.Clone()
	_, panicText := cargoSemverCatch(func() byte {
		value.LockTo(semverUpstreamVersion(t, "2.0.0"), VersionReqDefault)
		return 0
	})
	if !strings.Contains(panicText, "cannot lock ^1.2.3 to 2.0.0") || !value.Equal(before) {
		t.Fatal("failed lock must panic before changing the requirement")
	}
	value.PreciseTo(semverUpstreamVersion(t, "99.1.2"))
	if !value.IsPrecise() || value.IsLocked() || value.IsExact() || value.String() != "^1.2.3" || !value.Matches(semverUpstreamVersion(t, "99.1.2+metadata"), VersionReqDefault) {
		t.Fatal("precise_to must preserve the original requirement without requiring a match")
	}
	any := OptVersionReq{}
	any.LockTo(semverUpstreamVersion(t, "0.0.0-alpha+build"), VersionReqDefault)
	if !any.IsLocked() || !any.IsExact() || any.String() != "*" {
		t.Fatal("locking Any must preserve STAR as the original requirement")
	}
}

func TestCargoSemverExtensionOverflowAndUnknownOp(t *testing.T) {
	r := semverUpstreamReq(t, "=18446744073709551615")
	v := semverUpstreamVersion(t, "18446744073709551615.0.1-alpha")
	_, panicText := cargoSemverCatch(func() byte { return cargoSemverBool(MatchesPrerelease(r, v)) })
	if panicText != "overflow" || MatchesPrereleaseWithOverflow(r, v, SemverWrappingOverflow) {
		t.Fatalf("checked/wrapping upper bound differs: %q", panicText)
	}
	unknown := semver.Comparator{Op: semver.Op(255)}
	_, panicText = cargoSemverCatch(func() byte {
		return cargoSemverBool(MatchesPrerelease(semver.VersionReq{Comparators: []semver.Comparator{unknown}}, v))
	})
	if !strings.Contains(panicText, "unreachable") {
		t.Fatalf("unknown non-exhaustive operator must reach source panic: %q", panicText)
	}
	// A failed earlier comparator short-circuits before an unknown operator.
	falseFirst := semver.Comparator{Op: semver.Exact, Major: 1}
	if MatchesPrerelease(semver.VersionReq{Comparators: []semver.Comparator{falseFirst, unknown}}, semver.NewVersion(0, 0, 0)) {
		t.Fatal("later operator was visited despite an earlier mismatch")
	}
	var extension VersionReqExt = VersionReqAdapter{semverUpstreamReq(t, "^1.2.3")}
	if !extension.MatchesPrerelease(semverUpstreamVersion(t, "1.2.4-alpha")) {
		t.Fatal("concrete extension trait adapter did not use Cargo matching")
	}
	for mode, text := range map[VersionReqMatchMode]string{VersionReqDefault: "Default", VersionReqPrerelease: "Prerelease"} {
		if mode.Clone() != mode || mode.RustDebug() != text {
			t.Fatal("matching mode Copy/Clone/Debug differs")
		}
	}
}

func FuzzCargoSemverExtensions(f *testing.F) {
	for _, pair := range [][2]string{{"*", "0.0.0-alpha"}, {"^1.2.3", "1.2.4-alpha+build"}, {"=18446744073709551615", "18446744073709551615.0.1-alpha"}, {">=1.2.3-0, <2", "2.0.0-0"}, {"~0.2", "0.2.3"}} {
		f.Add(pair[0], pair[1])
	}
	f.Fuzz(func(t *testing.T, requirement, version string) {
		if len(requirement) > 512 || len(version) > 512 || !utf8.ValidString(requirement) || !utf8.ValidString(version) {
			t.Skip()
		}
		req, err := semver.ParseVersionReq(requirement)
		if err != nil {
			return
		}
		v, err := semver.ParseVersion(version)
		if err != nil {
			return
		}
		value := FromVersionReq(req)
		for _, mode := range []VersionReqMatchMode{VersionReqDefault, VersionReqPrerelease} {
			for _, overflow := range []SemverOverflowMode{SemverCheckedOverflow, SemverWrappingOverflow} {
				cargoSemverCatch(func() byte { return cargoSemverBool(value.MatchesWithOverflow(v, mode, overflow)) })
				candidate, before := value.Clone(), value.Clone()
				_, panicText := cargoSemverCatch(func() byte { candidate.LockToWithOverflow(v, mode, overflow); return 0 })
				if panicText != "" && !candidate.Equal(before) {
					t.Fatal("panicking lock changed requirement")
				}
			}
		}
		clone := value.Clone()
		a, b := &cargoSemverTrace{}, &cargoSemverTrace{}
		value.Hash(a)
		clone.Hash(b)
		if !value.Equal(clone) || !slices.Equal(a.events, b.events) {
			t.Fatal("equal clone has different hash events")
		}
		if len(clone.Req.Comparators) != 0 {
			clone.Req.Comparators[0].Major++
			if value.Equal(clone) {
				t.Fatal("mutating clone changed original storage")
			}
		}
	})
}
