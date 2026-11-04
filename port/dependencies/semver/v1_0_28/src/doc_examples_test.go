// SPDX-License-Identifier: MIT OR Apache-2.0
// All four code examples in official semver 1.0.28 src/{lib,parse}.rs, preserving
// every example literal and all four runtime assertion sites. Its const-fn
// constructor witness maps to a Go constructor (Go has no const functions).
package semver

import (
	"reflect"
	"slices"
	"testing"
)

func TestOriginalCrateExample(t *testing.T) {
	req, e := ParseVersionReq(">=1.2.3, <1.8.0")
	if e != nil {
		t.Fatal(e)
	}
	pre, e := NewPrerelease("alpha.1")
	if e != nil {
		t.Fatal(e)
	}
	version := Version{Major: 1, Minor: 2, Patch: 3, Pre: pre, Build: EmptyBuildMetadata()}
	if req.Matches(version) {
		t.Fatal("example prerelease unexpectedly matched")
	}
	version, e = ParseVersion("1.3.0")
	if e != nil {
		t.Fatal(e)
	}
	if !req.Matches(version) {
		t.Fatal("example release did not match")
	}
}
func TestOriginalConstConstructorExample(t *testing.T) {
	newEquivalent := func(major, minor, patch uint64) Version {
		return Version{Major: major, Minor: minor, Patch: patch, Pre: EmptyPrerelease(), Build: EmptyBuildMetadata()}
	}
	if !NewVersion(1, 2, 3).Equal(newEquivalent(1, 2, 3)) {
		t.Fatal("constructor differs from documented literal")
	}
}
func TestOriginalComparePrecedenceExample(t *testing.T) {
	parse := func(text string) Version {
		v, e := ParseVersion(text)
		if e != nil {
			t.Fatal(e)
		}
		return v
	}
	versions := []Version{parse("1.20.0+c144a98"), parse("1.20.0"), parse("1.0.0"), parse("1.0.0-alpha"), parse("1.20.0+bc17664")}
	slices.SortStableFunc(versions, Version.ComparePrecedence)
	if !reflect.DeepEqual(versions, []Version{parse("1.0.0-alpha"), parse("1.0.0"), parse("1.20.0+c144a98"), parse("1.20.0"), parse("1.20.0+bc17664")}) {
		t.Fatal(versions)
	}
	slices.SortFunc(versions, Version.Compare)
	if !reflect.DeepEqual(versions, []Version{parse("1.0.0-alpha"), parse("1.0.0"), parse("1.20.0"), parse("1.20.0+bc17664"), parse("1.20.0+c144a98")}) {
		t.Fatal(versions)
	}
}

// The fourth original doctest is parse.rs Error's unwrap_err/eprintln example.
func TestOriginalParseErrorExample(t *testing.T) {
	_, err := ParseVersion("1.q.r")
	if err == nil {
		t.Fatal("unwrap_err example accepted invalid input")
	}
	if err.Error() != "unexpected character 'q' while parsing minor version number" {
		t.Fatal(err)
	}
}
