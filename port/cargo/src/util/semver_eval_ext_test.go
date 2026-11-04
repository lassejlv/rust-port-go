// SPDX-License-Identifier: MIT OR Apache-2.0
// All nine original semver_eval_ext.rs inline tests, with the same requirements,
// version lists, outcomes, and iteration order.
package util

import (
	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
	"testing"
)

func semverUpstreamReq(t *testing.T, s string) semver.VersionReq {
	t.Helper()
	v, e := semver.ParseVersionReq(s)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func semverUpstreamVersion(t *testing.T, s string) semver.Version {
	t.Helper()
	v, e := semver.ParseVersion(s)
	if e != nil {
		t.Fatal(e)
	}
	return v
}
func semverAssertUpstream(t *testing.T, r semver.VersionReq, versions []string, want bool) {
	t.Helper()
	for _, text := range versions {
		v := semverUpstreamVersion(t, text)
		if got := MatchesPrerelease(r, v); got != want {
			t.Fatalf("%s vs %s: %v, want %v", r.String(), text, got, want)
		}
	}
}
func TestUpstreamSemverEvalExact(t *testing.T) {
	{
		r := semverUpstreamReq(t, "=4.2.1-0")
		semverAssertUpstream(t, r, []string{"4.2.1-0"}, true)
		semverAssertUpstream(t, r, []string{"1.2.3", "4.2.0", "4.2.1-1", "4.2.2"}, false)
	}
	for _, text := range []string{"=4.2.1", ">=4.2.1, <4.2.2-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"4.2.1"}, true)
		semverAssertUpstream(t, r, []string{"1.2.3", "4.2.1-0", "4.2.2-0", "4.2.2"}, false)
	}
	for _, text := range []string{"=4.2", ">=4.2.0, <4.3.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"4.2.0", "4.2.1", "4.2.9"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "2.1.2-0", "4.2.0-0"}, false)
		semverAssertUpstream(t, r, []string{"4.3.0-0", "4.3.0", "5.0.0-0", "5.0.0"}, false)
	}
	for _, text := range []string{"=4", ">=4.0.0, <5.0.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"4.0.0", "4.2.1", "4.2.4-0", "4.9.9"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "2.1.2-0", "4.0.0-0"}, false)
		semverAssertUpstream(t, r, []string{"5.0.0-0", "5.0.0", "5.0.1"}, false)
	}
}
func TestUpstreamSemverEvalGreaterEq(t *testing.T) {
	{
		r := semverUpstreamReq(t, ">=4.2.1-0")
		semverAssertUpstream(t, r, []string{"4.2.1-0", "4.2.1", "5.0.0"}, true)
		semverAssertUpstream(t, r, []string{"0.0.0", "1.2.3"}, false)
	}
	{
		r := semverUpstreamReq(t, ">=4.2.1")
		semverAssertUpstream(t, r, []string{"4.2.1", "5.0.0"}, true)
		semverAssertUpstream(t, r, []string{"0.0.0", "4.2.1-0"}, false)
	}
	for _, text := range []string{">=4.2", ">=4.2.0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"4.2.1-0", "4.2.0", "4.3.0"}, true)
		semverAssertUpstream(t, r, []string{"0.0.0", "4.1.1", "4.2.0-0"}, false)
	}
	for _, text := range []string{">=4", ">=4.0.0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"4.0.0", "4.1.0-1", "5.0.0"}, true)
		semverAssertUpstream(t, r, []string{"0.0.0", "1.2.3", "4.0.0-0"}, false)
	}
}
func TestUpstreamSemverEvalLess(t *testing.T) {
	for _, text := range []string{"<4.2.1", "<4.2.1-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.0.0", "4.0.0"}, true)
		semverAssertUpstream(t, r, []string{"4.2.1-0", "4.2.2", "5.0.0-0", "5.0.0"}, false)
	}
	for _, text := range []string{"<4.2", "<4.2.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.0.0", "4.1.0"}, true)
		semverAssertUpstream(t, r, []string{"4.2.0-0", "4.2.0", "4.3.0-0", "4.3.0"}, false)
	}
	for _, text := range []string{"<4", "<4.0.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.0.0", "3.9.0"}, true)
		semverAssertUpstream(t, r, []string{"4.0.0-0", "4.0.0", "5.0.0-1", "5.0.0"}, false)
	}
}
func TestUpstreamSemverEvalLessUpperBound(t *testing.T) {
	for _, text := range []string{">1.2.3, <2", ">1.2.3, <2.0", ">1.2.3, <2.0.0", ">=1.2.3, <2.0.0", ">1.2.3, <2.0.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"1.2.4", "1.9.9"}, true)
		semverAssertUpstream(t, r, []string{"2.0.0-0", "2.0.0", "2.1.2"}, false)
	}
	for _, text := range []string{">1.2.3-0, <2", ">1.2.3-0, <2.0", ">1.2.3-0, <2.0.0", ">=1.2.3-0, <2.0.0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"1.2.4", "1.9.9", "2.0.0-0"}, true)
		semverAssertUpstream(t, r, []string{"2.0.0", "2.1.2"}, false)
	}
	for _, text := range []string{">=2.0.0-0, <2", ">=2.0.0-0, <2.0", ">=2.0.0-0, <2.0.0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"2.0.0-0", "2.0.0-11"}, true)
		semverAssertUpstream(t, r, []string{"0.0.9", "2.0.0"}, false)
	}
	{
		r := semverUpstreamReq(t, ">5.0.0, <2.0.0")
		semverAssertUpstream(t, r, []string{"1.2.3", "3.0.0", "6.0.0"}, false)
	}
	{
		r := semverUpstreamReq(t, ">5.0.0-0, <2.0.0")
		semverAssertUpstream(t, r, []string{"1.2.3", "3.0.0", "6.0.0"}, false)
	}
}
func TestUpstreamSemverEvalCaret(t *testing.T) {
	for _, text := range []string{"^1.2.3-0", ">=1.2.3-0, <2.0.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"1.2.3-0", "1.2.3-1", "1.2.3", "1.9.9"}, true)
		semverAssertUpstream(t, r, []string{"0.0.9", "1.1.1-0", "2.0.0-0", "2.1.1"}, false)
	}
	for _, text := range []string{"^1.2.3", ">=1.2.3, <2.0.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"1.2.3", "1.9.9"}, true)
		semverAssertUpstream(t, r, []string{"0.0.9", "1.1.1-0", "1.2.3-0", "1.2.3-1", "2.0.0-0", "2.1.1"}, false)
	}
	for _, text := range []string{"^0.2.3-0", ">=0.2.3-0, <0.3.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.2.3-0", "0.2.3", "0.2.9-0", "0.2.9"}, true)
		semverAssertUpstream(t, r, []string{"0.0.9", "0.3.0-0", "0.3.11", "1.1.1"}, false)
	}
	for _, text := range []string{"^0.2.3", ">=0.2.3, <0.3.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.2.3", "0.2.9-0", "0.2.9"}, true)
		semverAssertUpstream(t, r, []string{"0.0.9", "0.2.3-0", "0.3.0-0", "0.3.11", "1.1.1"}, false)
	}
	for _, text := range []string{"^0.0.3-0", ">=0.0.3-0, <0.1.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.0.3-0", "0.0.3-1", "0.0.3"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "0.3.0-0", "0.4.0-0", "1.1.1"}, false)
	}
	for _, text := range []string{"^0.0.3", ">=0.0.3, <0.1.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.0.3"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "0.0.3-0", "0.3.0-0", "0.0.3-1", "0.4.0-0", "1.1.1"}, false)
	}
	for _, text := range []string{"^1.2", ">=1.2.0, <2.0.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"1.2.0", "1.9.0-0", "1.9.9"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "0.0.4-0", "1.2.0-0", "2.0.0-0", "4.0.1"}, false)
	}
	for _, text := range []string{"^0.0", ">=0.0.0, <0.1.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.0.0", "0.0.1", "0.0.4-0"}, true)
		semverAssertUpstream(t, r, []string{"0.0.0-0", "0.1.0-0", "0.1.0", "1.1.1"}, false)
	}
	for _, text := range []string{"^1", ">=1.0.0, <2.0.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"1.0.0", "1.0.1"}, true)
		semverAssertUpstream(t, r, []string{"0.1.0-0", "0.1.0", "1.0.0-0", "2.0.0-0", "3.1.2"}, false)
	}
}
func TestUpstreamSemverEvalWildcard(t *testing.T) {
	for _, text := range []string{"4.2.*", "=4.2"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"4.2.0", "4.2.1", "4.2.9"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "2.1.2-0", "4.2.0-0"}, false)
		semverAssertUpstream(t, r, []string{"4.3.0-0", "4.3.0", "5.0.0", "5.0.1"}, false)
	}
	for _, text := range []string{"4.*", "4.*.*", "=4"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"4.0.0", "4.2.1", "4.9.9"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "2.1.2-0", "4.0.0-0"}, false)
		semverAssertUpstream(t, r, []string{"5.0.0-0", "5.0.0", "5.0.1"}, false)
	}
}
func TestUpstreamSemverEvalGreater(t *testing.T) {
	{
		r := semverUpstreamReq(t, ">4.2.1-0")
		semverAssertUpstream(t, r, []string{"4.2.1", "4.2.2", "5.0.0"}, true)
		semverAssertUpstream(t, r, []string{"0.0.0", "4.2.1-0"}, false)
	}
	{
		r := semverUpstreamReq(t, ">4.2.1")
		semverAssertUpstream(t, r, []string{"4.2.2", "5.0.0-0", "5.0.0"}, true)
		semverAssertUpstream(t, r, []string{"0.0.0", "4.2.1-0", "4.2.1"}, false)
	}
	for _, text := range []string{">4.2", ">=4.3.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"4.3.0-0", "4.3.0", "5.0.0"}, true)
		semverAssertUpstream(t, r, []string{"0.0.0", "4.2.1"}, false)
	}
	for _, text := range []string{">4", ">=5.0.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"5.0.0-0", "5.0.0"}, true)
		semverAssertUpstream(t, r, []string{"0.0.0", "4.2.1"}, false)
	}
}
func TestUpstreamSemverEvalLessEq(t *testing.T) {
	{
		r := semverUpstreamReq(t, "<=4.2.1")
		semverAssertUpstream(t, r, []string{"0.0.0", "4.2.1-0", "4.2.1"}, true)
		semverAssertUpstream(t, r, []string{"4.2.2", "5.0.0-0", "5.0.0"}, false)
	}
	{
		r := semverUpstreamReq(t, "<=4.2.1-0")
		semverAssertUpstream(t, r, []string{"0.0.0", "4.2.1-0"}, true)
		semverAssertUpstream(t, r, []string{"4.2.1", "4.2.2", "5.0.0-0", "5.0.0"}, false)
	}
	for _, text := range []string{"<=4.2", "<4.3.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.0.0", "4.2.0-0"}, true)
		semverAssertUpstream(t, r, []string{"4.3.0-0", "4.3.0", "4.4.0"}, false)
	}
	for _, text := range []string{"<=4", "<5.0.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.0.0", "4.0.0-0", "4.0.0"}, true)
		semverAssertUpstream(t, r, []string{"5.0.0-1", "5.0.0"}, false)
	}
}
func TestUpstreamSemverEvalTilde(t *testing.T) {
	for _, text := range []string{"~1.2.3-0", ">= 1.2.3-0, < 1.3.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"1.2.3-0", "1.2.3", "1.2.4-0", "1.2.4"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "1.1.0-0"}, false)
		semverAssertUpstream(t, r, []string{"1.3.0-0", "1.3.0", "1.3.1", "2.0.0"}, false)
	}
	for _, text := range []string{"~1.2.3", ">= 1.2.3, < 1.3.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"1.2.3", "1.2.4-0", "1.2.4"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "1.1.0-0", "1.2.3-0"}, false)
		semverAssertUpstream(t, r, []string{"1.3.0-0", "1.3.0", "1.3.1", "2.0.0"}, false)
	}
	for _, text := range []string{"~0.24", ">=0.24.0, <0.25.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"0.24.0", "0.24.1-0", "0.24.1", "0.24.9"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "0.9.9", "0.24.0-0"}, false)
		semverAssertUpstream(t, r, []string{"0.25.0-0", "1.1.0", "1.2.3", "2.0.0"}, false)
	}
	for _, text := range []string{"~1", ">=1.0.0, <2.0.0-0"} {
		r := semverUpstreamReq(t, text)
		semverAssertUpstream(t, r, []string{"1.0.0", "1.1.0-0", "1.1.0"}, true)
		semverAssertUpstream(t, r, []string{"0.0.1", "0.9.9", "1.0.0-0"}, false)
		semverAssertUpstream(t, r, []string{"2.0.0-0", "2.0.0", "2.0.1"}, false)
	}
}
