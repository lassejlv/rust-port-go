//go:build !semver_serde

package contract

func observeSerde(Request) any { panic("serde feature disabled") }
