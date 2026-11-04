// SPDX-License-Identifier: MIT OR Apache-2.0
// Both original semver_ext.rs inline tests, preserving all cases and outcomes.
package util

import "testing"

func TestUpstreamSemverPrerelease(t *testing.T) {
	for _, item := range []struct {
		req, version string
		want         bool
	}{
		{"1.2.3", "1.2.3-0", false},
		{"1.2.3", "1.2.3-1", false},
		{"1.2.3", "1.2.4-0", true},
		{">=1.2.3", "1.2.3-0", false},
		{">=1.2.3", "1.2.3-1", false},
		{">=1.2.3", "1.2.4-0", true},
		{">1.2.3", "1.2.3-0", false},
		{">1.2.3", "1.2.3-1", false},
		{">1.2.3", "1.2.4-0", true},
		{">1.2.3, <1.2.4", "1.2.3-0", false},
		{">1.2.3, <1.2.4", "1.2.3-1", false},
		{">1.2.3, <1.2.4", "1.2.4-0", false},
		{">=1.2.3, <1.2.4", "1.2.3-0", false},
		{">=1.2.3, <1.2.4", "1.2.3-1", false},
		{">=1.2.3, <1.2.4", "1.2.4-0", false},
		{">1.2.3, <=1.2.4", "1.2.3-0", false},
		{">1.2.3, <=1.2.4", "1.2.3-1", false},
		{">1.2.3, <=1.2.4", "1.2.4-0", true},
		{">=1.2.3-0, <1.2.3", "1.2.3-0", true},
		{">=1.2.3-0, <1.2.3", "1.2.3-1", true},
		{">=1.2.3-0, <1.2.3", "1.2.4-0", false},
		{"1.2.3", "2.0.0-0", false},
		{"=1.2.3-0", "1.2.3", false},
		{"=1.2.3-0", "1.2.3-0", true},
		{"=1.2.3-0", "1.2.4", false},
		{">=1.2.3-2, <1.2.3-4", "1.2.3-0", false},
		{">=1.2.3-2, <1.2.3-4", "1.2.3-3", true},
		{">=1.2.3-2, <1.2.3-4", "1.2.3-5", false},
	} {
		req := FromVersionReq(semverUpstreamReq(t, item.req))
		v := semverUpstreamVersion(t, item.version)
		if got := req.Matches(v, VersionReqPrerelease); got != item.want {
			t.Fatalf("req: %s; ver: %s; got %v, want %v", item.req, item.version, got, item.want)
		}
	}
}
func TestUpstreamOptVersionReqMatchesPrerelease(t *testing.T) {
	r := semverUpstreamReq(t, "^1.2.3-rc.0")
	v := semverUpstreamVersion(t, "1.2.3-rc.0")
	for _, req := range []OptVersionReq{FromVersionReq(r), LockedVersionReq(v, r), PreciseVersionReq(v, r)} {
		if !req.Matches(v, VersionReqPrerelease) {
			t.Fatalf("%s did not match %s", req.RustDebug(), v.String())
		}
	}
}
