//go:build semver_serde

package semver_test

import "testing"

func TestOriginalSerdeOracle(t *testing.T) { replay(t, "serde", 3136) }
