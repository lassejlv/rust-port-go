//go:build !semver_node

package semver_test

import semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"

func requirement_matches(req semver.VersionReq, version semver.Version) bool {
	return req.Matches(version)
}
