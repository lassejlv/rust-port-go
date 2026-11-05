// SPDX-License-Identifier: MIT OR Apache-2.0
// Canonical source module forwarding for rustc_ast/src/util/case.rs.
// The one actual Case identity lives in a leaf so parent token code can use
// it without importing comments.go, which itself needs parent CommentKind.
package astutil

import casetypes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_ast/src/util/case_types"

type Case = casetypes.Case

func Sensitive() Case   { return casetypes.Sensitive() }
func Insensitive() Case { return casetypes.Insensitive() }
