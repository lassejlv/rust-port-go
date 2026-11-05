// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures_test

import (
	"github.com/lassejlv/rust-port-go/tools/port-rust-panic-runtime/component_guards/validation"
	"testing"
)

func TestFingerprintPanicRuntimeGuards(t *testing.T) { validation.Replay(t, "fingerprint") }
func TestStableHashContextPanicRuntimeGuards(t *testing.T) {
	validation.Replay(t, "stable_hash_context")
}
