// SPDX-License-Identifier: MIT OR Apache-2.0
// Borrowed caller-location support for the used macros.rs bug_impl boundary.
// This supporting bridge grants no macros.rs/std source-file credit.
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// BorrowedSourceLocation preserves an actual immutable source Location cell.
// Callers keep the cell and its file bytes alive for the source borrows they
// expose (including the diagnostic callback's source &'static Location). Nil
// explicitly means unlocated and does not manufacture a filename or cell.
func BorrowedSourceLocation(cell *rustfmt.SourceLocation) SourceLocation {
	if cell == nil {
		return SourceLocation{Host: true}
	}
	return SourceLocation{File: cell.File, Line: cell.Line, Column: cell.Column, cell: cell}
}

// BorrowedCell exposes the original immutable cell through the scalar façade.
// Scalar/host conveniences have no such borrow and return nil.
func (l SourceLocation) BorrowedCell() *rustfmt.SourceLocation { return l.cell }
func BugImplAtLocationCellWithContext(session *SessionGlobals, span Option[Span], args SourceFormatArguments, cell *rustfmt.SourceLocation, context rustfmt.Context) {
	BugImplWithContext(session, span, args, BorrowedSourceLocation(cell), context)
}
func SpanBugAtLocationCellWithContext(session *SessionGlobals, span Span, message string, cell *rustfmt.SourceLocation, context rustfmt.Context) {
	BugImplAtLocationCellWithContext(session, Some(span), LiteralSourceFormatArguments(message), cell, context)
}

// EmitBugDiagnosticLocationCellFunc is the borrowed function-pointer witness.
// The function and all referenced callback state are borrowed static metadata;
// an owning environment must not be hidden in the Go function closure. A nil
// cell remains nil for scalar/host convenience calls without a primary borrow.
type EmitBugDiagnosticLocationCellFunc func(*SessionGlobals, Option[Span], SourceFormatArguments, *rustfmt.SourceLocation, rustfmt.Context) bool

// EmitBugDiagnosticBinding retains the exact previous function-pointer adapter
// for explicit restoration. It owns no callback environment or destructor.
type EmitBugDiagnosticBinding struct{ previous emitBugFunction }

func SwapEmitBugDiagnosticLocationCell(callback EmitBugDiagnosticLocationCellFunc) EmitBugDiagnosticBinding {
	if callback == nil {
		panic("nil emit-bug function is not a Rust fn pointer")
	}
	previous := SwapEmitBugDiagnostic(func(session *SessionGlobals, span Option[Span], args SourceFormatArguments, location SourceLocation, context rustfmt.Context) bool {
		return callback(session, span, args, location.BorrowedCell(), context)
	})
	return EmitBugDiagnosticBinding{previous: previous}
}
func (binding EmitBugDiagnosticBinding) Restore() { SwapEmitBugDiagnostic(binding.previous) }
