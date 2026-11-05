// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcindex_test

import (
	"github.com/lassejlv/rust-port-go/tools/port-rust-panic-runtime/component_guards/validation"
	"testing"
)

func TestIntervalPanicRuntimeGuards(t *testing.T) { validation.Replay(t, "interval") }
