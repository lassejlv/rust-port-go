//go:build semver_miri

package semver_test

// This selects the original test workload cap; it does not emulate Miri.
const originalMiriProfile = true
