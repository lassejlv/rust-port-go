// SPDX-License-Identifier: MIT OR Apache-2.0
package rustclexer_test

import (
	"github.com/lassejlv/rust-port-go/tools/port-rust-panic-runtime/component_guards/validation"
	"testing"
)

func TestPredicatePanicRuntimeGuards(t *testing.T)    { validation.Replay(t, "predicate") }
func TestTokenForEachPanicRuntimeGuards(t *testing.T) { validation.Replay(t, "foreach") }
func TestTokenFoldPanicRuntimeGuards(t *testing.T)    { validation.Replay(t, "fold") }
