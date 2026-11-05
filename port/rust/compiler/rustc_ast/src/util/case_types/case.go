// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: rustc_ast/src/util/case.rs, blob 1afd7dea7408ecb9c247e101ac7f4f951c636cf3.
package casetypes

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// Case has precisely the two source variants. Its zero value is Sensitive;
// ordinary Go value assignment supplies the source Copy operation.
type Case struct{ tag uint8 }

func Sensitive() Case                       { return Case{} }
func Insensitive() Case                     { return Case{tag: 1} }
func (value Case) Clone() Case              { return value }
func (value *Case) CloneFrom(source Case)   { *value = source }
func (value Case) Equal(other Case) bool    { return value == other }
func (value Case) NotEqual(other Case) bool { return !value.Equal(other) }
func (value Case) WriteDebugFormatter(formatter *rustfmt.Formatter) error {
	if value.tag == 0 {
		return formatter.WriteString("Sensitive")
	}
	if value.tag == 1 {
		return formatter.WriteString("Insensitive")
	}
	formatter.Context.Panic("rustcast: invalid Go Case variant")
	return nil
}
func (value Case) RustDebug() string { return rustfmt.String(value.WriteDebugFormatter) }
