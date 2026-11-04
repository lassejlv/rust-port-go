// Mirrors semver 1.0.28 tests/util/mod.rs and shared assertion helpers.
package semver_test

import (
	"fmt"
	"hash/fnv"
	"reflect"
	"strings"
	"testing"

	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
)

func version(t *testing.T, text string) semver.Version {
	t.Helper()
	value, err := semver.ParseVersion(text)
	if err != nil {
		t.Fatalf("version(%q): %v", text, err)
	}
	return value
}
func version_err(t *testing.T, text string) error {
	t.Helper()
	_, err := semver.ParseVersion(text)
	if err == nil {
		t.Fatalf("version_err(%q) succeeded", text)
	}
	return err
}
func req(t *testing.T, text string) requirement {
	t.Helper()
	value, err := semver.ParseVersionReq(text)
	if err != nil {
		t.Fatalf("req(%q): %v", text, err)
	}
	return requirement{value}
}
func req_err(t *testing.T, text string) error {
	t.Helper()
	_, err := semver.ParseVersionReq(text)
	if err == nil {
		t.Fatalf("req_err(%q) succeeded", text)
	}
	return err
}
func comparator(t *testing.T, text string) semver.Comparator {
	t.Helper()
	value, err := semver.ParseComparator(text)
	if err != nil {
		t.Fatalf("comparator(%q): %v", text, err)
	}
	return value
}
func comparator_err(t *testing.T, text string) error {
	t.Helper()
	_, err := semver.ParseComparator(text)
	if err == nil {
		t.Fatalf("comparator_err(%q) succeeded", text)
	}
	return err
}
func prerelease(t *testing.T, text string) semver.Prerelease {
	t.Helper()
	value, err := semver.NewPrerelease(text)
	if err != nil {
		t.Fatalf("prerelease(%q): %v", text, err)
	}
	return value
}
func prerelease_err(t *testing.T, text string) error {
	t.Helper()
	_, err := semver.NewPrerelease(text)
	if err == nil {
		t.Fatalf("prerelease_err(%q) succeeded", text)
	}
	return err
}
func build_metadata(t *testing.T, text string) semver.BuildMetadata {
	t.Helper()
	value, err := semver.NewBuildMetadata(text)
	if err != nil {
		t.Fatalf("build_metadata(%q): %v", text, err)
	}
	return value
}
func assert_to_string(t *testing.T, value any, expected string) {
	t.Helper()
	assert_eq(t, fmt.Sprint(value), expected)
}

type requirement struct{ value semver.VersionReq }

func (r requirement) String() string                    { return r.value.String() }
func (r requirement) Equal(other requirement) bool      { return r.value.Equal(other.value) }
func (r requirement) Hash(hasher semver.Hasher)         { r.value.Hash(hasher) }
func (r requirement) Matches(value semver.Version) bool { return requirement_matches(r.value, value) }
func starReq() requirement                              { return requirement{semver.Star()} }

func assert_match_all(t *testing.T, req requirement, versions []string) {
	t.Helper()
	for _, string := range versions {
		parsed := version(t, string)
		assert_true(t, req.Matches(parsed), "did not match {}", string)
	}
}
func assert_match_none(t *testing.T, req requirement, versions []string) {
	t.Helper()
	for _, string := range versions {
		parsed := version(t, string)
		assert_true(t, !req.Matches(parsed), "matched {}", string)
	}
}

func rust_eq(a, b any) bool {
	switch value := a.(type) {
	case semver.Version:
		other, ok := b.(semver.Version)
		return ok && value.Equal(other)
	case semver.Prerelease:
		other, ok := b.(semver.Prerelease)
		return ok && value.Equal(other)
	case semver.BuildMetadata:
		other, ok := b.(semver.BuildMetadata)
		return ok && value.Equal(other)
	case semver.Comparator:
		other, ok := b.(semver.Comparator)
		return ok && value.Equal(other)
	case requirement:
		other, ok := b.(requirement)
		return ok && value.Equal(other)
	default:
		return reflect.DeepEqual(a, b)
	}
}
func rust_cmp(a, b any) int {
	switch value := a.(type) {
	case semver.Version:
		return value.Compare(b.(semver.Version))
	case semver.Prerelease:
		return value.Compare(b.(semver.Prerelease))
	case semver.BuildMetadata:
		return value.Compare(b.(semver.BuildMetadata))
	case int:
		other := b.(int)
		if value < other {
			return -1
		}
		if value > other {
			return 1
		}
		return 0
	default:
		panic("unhandled source ordering")
	}
}
func assert_eq(t *testing.T, a, b any) {
	t.Helper()
	if !rust_eq(a, b) {
		t.Fatalf("assert_eq: %#v != %#v", a, b)
	}
}
func assert_ne(t *testing.T, a, b any) {
	t.Helper()
	if rust_eq(a, b) {
		t.Fatalf("assert_ne: %#v == %#v", a, b)
	}
}
func assert_true(t *testing.T, value bool, args ...any) {
	t.Helper()
	if !value {
		if len(args) > 0 {
			t.Fatal(rust_format(args[0].(string), args[1:]...))
		}
		t.Fatal("source assertion failed")
	}
}

func rust_format(template string, args ...any) string {
	var output strings.Builder
	for _, arg := range args {
		start := strings.IndexByte(template, '{')
		if start < 0 {
			panic("missing source formatting argument")
		}
		end := strings.IndexByte(template[start:], '}') + start
		output.WriteString(template[:start])
		spec := template[start : end+1]
		switch spec {
		case "{}":
			output.WriteString(fmt.Sprint(arg))
		case "{:?}":
			output.WriteString(arg.(interface{ RustDebug() string }).RustDebug())
		case "{:20}", "{:*^20}", "{:>20}":
			options := semver.FormatOptions{Width: semver.Some(uint(20))}
			if spec == "{:*^20}" {
				options.Align = semver.AlignCenter
				options.Fill = semver.Some('*')
			}
			if spec == "{:>20}" {
				options.Align = semver.AlignRight
			}
			if err := arg.(semver.Display).WriteDisplay(&output, options); err != nil {
				panic(err)
			}
		default:
			panic("unhandled original test format " + spec)
		}
		template = template[end+1:]
	}
	output.WriteString(template)
	return output.String()
}

// The original Rust helper and DefaultHasher implementation remain exercised
// by the unchanged Rust fixture. Its Go assertion observes repeatability only;
// this caller-owned deterministic hasher does not claim to port DefaultHasher.
func calculate_hash(value interface{ Hash(semver.Hasher) }) uint64 {
	hasher := fnv.New64a()
	value.Hash(semver.HasherFunc(func(bytes []byte) { _, _ = hasher.Write(bytes) }))
	return hasher.Sum64()
}
