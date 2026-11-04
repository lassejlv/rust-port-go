// Code generated from unchanged semver1.0.28 tests by tools/port-semver/original-tests/mirror.py.

package semver_test

import (
	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
	"testing"
)

func TestVersion_test_parse(t *testing.T) {
	err := version_err(t, "")
	assert_to_string(t, err, "empty string, expected a semver version")

	err = version_err(t, "  ")
	assert_to_string(t,
		err,
		"unexpected character ' ' while parsing major version number",
	)

	err = version_err(t, "1")
	assert_to_string(t,
		err,
		"unexpected end of input while parsing major version number",
	)

	err = version_err(t, "1.2")
	assert_to_string(t,
		err,
		"unexpected end of input while parsing minor version number",
	)

	err = version_err(t, "1.2.3-")
	assert_to_string(t, err, "empty identifier segment in pre-release identifier")

	err = version_err(t, "a.b.c")
	assert_to_string(t,
		err,
		"unexpected character 'a' while parsing major version number",
	)

	err = version_err(t, "1.2.3 abc")
	assert_to_string(t, err, "unexpected character ' ' after patch version number")

	err = version_err(t, "1.2.3-01")
	assert_to_string(t, err, "invalid leading zero in pre-release identifier")

	err = version_err(t, "1.2.3++")
	assert_to_string(t, err, "empty identifier segment in build metadata")

	err = version_err(t, "07")
	assert_to_string(t, err, "invalid leading zero in major version number")

	err = version_err(t, "111111111111111111111.0.0")
	assert_to_string(t, err, "value of major version number exceeds u64::MAX")

	err = version_err(t, "8\u0000")
	assert_to_string(t, err, "unexpected character '\\0' after major version number")

	parsed := version(t, "1.2.3")
	expected := semver.NewVersion(1, 2, 3)
	assert_eq(t, parsed, expected)
	expected = semver.Version{
		Major: 1,
		Minor: 2,
		Patch: 3,
		Pre:   semver.EmptyPrerelease(),
		Build: semver.EmptyBuildMetadata(),
	}
	assert_eq(t, parsed, expected)

	parsed = version(t, "1.2.3-alpha1")
	expected = semver.Version{
		Major: 1,
		Minor: 2,
		Patch: 3,
		Pre:   prerelease(t, "alpha1"),
		Build: semver.EmptyBuildMetadata(),
	}
	assert_eq(t, parsed, expected)

	parsed = version(t, "1.2.3+build5")
	expected = semver.Version{
		Major: 1,
		Minor: 2,
		Patch: 3,
		Pre:   semver.EmptyPrerelease(),
		Build: build_metadata(t, "build5"),
	}
	assert_eq(t, parsed, expected)

	parsed = version(t, "1.2.3+5build")
	expected = semver.Version{
		Major: 1,
		Minor: 2,
		Patch: 3,
		Pre:   semver.EmptyPrerelease(),
		Build: build_metadata(t, "5build"),
	}
	assert_eq(t, parsed, expected)

	parsed = version(t, "1.2.3-alpha1+build5")
	expected = semver.Version{
		Major: 1,
		Minor: 2,
		Patch: 3,
		Pre:   prerelease(t, "alpha1"),
		Build: build_metadata(t, "build5"),
	}
	assert_eq(t, parsed, expected)

	parsed = version(t, "1.2.3-1.alpha1.9+build5.7.3aedf")
	expected = semver.Version{
		Major: 1,
		Minor: 2,
		Patch: 3,
		Pre:   prerelease(t, "1.alpha1.9"),
		Build: build_metadata(t, "build5.7.3aedf"),
	}
	assert_eq(t, parsed, expected)

	parsed = version(t, "1.2.3-0a.alpha1.9+05build.7.3aedf")
	expected = semver.Version{
		Major: 1,
		Minor: 2,
		Patch: 3,
		Pre:   prerelease(t, "0a.alpha1.9"),
		Build: build_metadata(t, "05build.7.3aedf"),
	}
	assert_eq(t, parsed, expected)

	parsed = version(t, "0.4.0-beta.1+0851523")
	expected = semver.Version{
		Major: 0,
		Minor: 4,
		Patch: 0,
		Pre:   prerelease(t, "beta.1"),
		Build: build_metadata(t, "0851523"),
	}
	assert_eq(t, parsed, expected)

	parsed = version(t, "1.1.0-beta-10")
	expected = semver.Version{
		Major: 1,
		Minor: 1,
		Patch: 0,
		Pre:   prerelease(t, "beta-10"),
		Build: semver.EmptyBuildMetadata(),
	}
	assert_eq(t, parsed, expected)
}

func TestVersion_test_eq(t *testing.T) {
	assert_eq(t, version(t, "1.2.3"), version(t, "1.2.3"))
	assert_eq(t, version(t, "1.2.3-alpha1"), version(t, "1.2.3-alpha1"))
	assert_eq(t, version(t, "1.2.3+build.42"), version(t, "1.2.3+build.42"))
	assert_eq(t, version(t, "1.2.3-alpha1+42"), version(t, "1.2.3-alpha1+42"))
}

func TestVersion_test_ne(t *testing.T) {
	assert_ne(t, version(t, "0.0.0"), version(t, "0.0.1"))
	assert_ne(t, version(t, "0.0.0"), version(t, "0.1.0"))
	assert_ne(t, version(t, "0.0.0"), version(t, "1.0.0"))
	assert_ne(t, version(t, "1.2.3-alpha"), version(t, "1.2.3-beta"))
	assert_ne(t, version(t, "1.2.3+23"), version(t, "1.2.3+42"))
}

func TestVersion_test_display(t *testing.T) {
	assert_to_string(t, version(t, "1.2.3"), "1.2.3")
	assert_to_string(t, version(t, "1.2.3-alpha1"), "1.2.3-alpha1")
	assert_to_string(t, version(t, "1.2.3+build.42"), "1.2.3+build.42")
	assert_to_string(t, version(t, "1.2.3-alpha1+42"), "1.2.3-alpha1+42")
}

func TestVersion_test_lt(t *testing.T) {
	assert_true(t, rust_cmp(version(t, "0.0.0"), version(t, "1.2.3-alpha2")) < 0)
	assert_true(t, rust_cmp(version(t, "1.0.0"), version(t, "1.2.3-alpha2")) < 0)
	assert_true(t, rust_cmp(version(t, "1.2.0"), version(t, "1.2.3-alpha2")) < 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha1"), version(t, "1.2.3")) < 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha1"), version(t, "1.2.3-alpha2")) < 0)
	assert_true(t, !(rust_cmp(version(t, "1.2.3-alpha2"), version(t, "1.2.3-alpha2")) < 0))
	assert_true(t, rust_cmp(version(t, "1.2.3+23"), version(t, "1.2.3+42")) < 0)
}

func TestVersion_test_le(t *testing.T) {
	assert_true(t, rust_cmp(version(t, "0.0.0"), version(t, "1.2.3-alpha2")) <= 0)
	assert_true(t, rust_cmp(version(t, "1.0.0"), version(t, "1.2.3-alpha2")) <= 0)
	assert_true(t, rust_cmp(version(t, "1.2.0"), version(t, "1.2.3-alpha2")) <= 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha1"), version(t, "1.2.3-alpha2")) <= 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha2"), version(t, "1.2.3-alpha2")) <= 0)
	assert_true(t, rust_cmp(version(t, "1.2.3+23"), version(t, "1.2.3+42")) <= 0)
}

func TestVersion_test_gt(t *testing.T) {
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha2"), version(t, "0.0.0")) > 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha2"), version(t, "1.0.0")) > 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha2"), version(t, "1.2.0")) > 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha2"), version(t, "1.2.3-alpha1")) > 0)
	assert_true(t, rust_cmp(version(t, "1.2.3"), version(t, "1.2.3-alpha2")) > 0)
	assert_true(t, !(rust_cmp(version(t, "1.2.3-alpha2"), version(t, "1.2.3-alpha2")) > 0))
	assert_true(t, !(rust_cmp(version(t, "1.2.3+23"), version(t, "1.2.3+42")) > 0))
}

func TestVersion_test_ge(t *testing.T) {
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha2"), version(t, "0.0.0")) >= 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha2"), version(t, "1.0.0")) >= 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha2"), version(t, "1.2.0")) >= 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha2"), version(t, "1.2.3-alpha1")) >= 0)
	assert_true(t, rust_cmp(version(t, "1.2.3-alpha2"), version(t, "1.2.3-alpha2")) >= 0)
	assert_true(t, !(rust_cmp(version(t, "1.2.3+23"), version(t, "1.2.3+42")) >= 0))
}

func TestVersion_test_spec_order(t *testing.T) {
	vs := []string{
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
	}
	i := 1
	for i < len(vs) {
		a := version(t, vs[i-1])
		b := version(t, vs[i])
		assert_true(t, rust_cmp(a, b) < 0, "nope {:?} < {:?}", a, b)
		i += 1
	}
}

func TestVersion_test_align(t *testing.T) {
	version := version(t, "1.2.3-rc1")
	assert_eq(t, "1.2.3-rc1           ", rust_format("{:20}", version))
	assert_eq(t, "*****1.2.3-rc1******", rust_format("{:*^20}", version))
	assert_eq(t, "           1.2.3-rc1", rust_format("{:>20}", version))
}
