// Mirrors semver 1.0.28 tests/test_identifier.rs in source declaration order.
package semver_test

import (
	"strings"
	"testing"

	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
)

func TestIdentifier_test_new(t *testing.T) {
	test := func(identifier semver.Prerelease, expected string) {
		assert_eq(t, identifier.IsEmpty(), expected == "")
		assert_eq(t, identifier.Len(), len(expected))
		assert_eq(t, identifier.AsStr(), expected)
		assert_eq(t, identifier, identifier)
		assert_eq(t, identifier, identifier.Clone())
	}
	var string strings.Builder
	limit := 280
	if originalMiriProfile {
		limit = 40
	}
	for i := 0; i < limit; i++ {
		test(prerelease(t, string.String()), string.String())
		string.WriteByte('1')
	}
	if !originalMiriProfile {
		string := strings.Repeat(string.String(), 20000)
		test(prerelease(t, string), string)
	}
}

func TestIdentifier_test_eq(t *testing.T) {
	assert_eq(t, prerelease(t, "-"), prerelease(t, "-"))
	assert_ne(t, prerelease(t, "a"), prerelease(t, "aa"))
	assert_ne(t, prerelease(t, "aa"), prerelease(t, "a"))
	assert_ne(t, prerelease(t, "aaaaaaaaa"), prerelease(t, "a"))
	assert_ne(t, prerelease(t, "a"), prerelease(t, "aaaaaaaaa"))
	assert_ne(t, prerelease(t, "aaaaaaaaa"), prerelease(t, "bbbbbbbbb"))
	assert_ne(t, build_metadata(t, "1"), build_metadata(t, "001"))
}

func TestIdentifier_test_prerelease(t *testing.T) {
	err := prerelease_err(t, "1.b\x00")
	assert_to_string(t, err, "unexpected character in pre-release identifier")
}
