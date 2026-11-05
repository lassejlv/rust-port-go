// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped SDK1.101 eprintln operation used by source_map.rs, no std-file credit.
package rustcspan

import (
	"io"
	"strings"

	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststdio"
)

// SourceMapStderr is a borrowed complete ambient stderr/capture/locking
// operation witness. PrintFatal receives the three actual fixed primitive
// format arguments in write order. It includes source output-capture and
// reentrant-lock behavior, returns the completed std write_fmt result and
// borrows its caller-owned state for this operation only. Its callback has no
// additional owned closure destructor. It must outlive every SourceMap using
// it and receive each later caller's Context, never a captured earlier Scope.
// Nil PrintFatal uses the actual host backend. UnixErrorBinding supplies the
// host's strerror_r operation when cgo is unavailable.
type SourceMapStderr struct {
	// Binding is the primary complete shared stdio operation. A component
	// resolver may select the actual caller's source-thread scope using the
	// current Context. A fixed-thread binding is valid only on that thread.
	Binding          *ruststdio.Binding
	PrintFatal       func(parts [3]string, context rustfmt.Context) error
	UnixErrorBinding *rusterrno.Binding
}

var sourceMapFatalParts = [3]string{"fatal error: rustc does not support text files larger than ", "4294967294", " bytes\n"}

func sourceMapPrintFatal(context rustfmt.Context, stderr *SourceMapStderr) {
	var failure error
	if stderr != nil && stderr.PrintFatal != nil {
		spanRunContext(context, func() { failure = stderr.PrintFatal(sourceMapFatalParts, context) })
	} else {
		statement := func(writer io.Writer, current rustfmt.Context) error {
			for _, part := range sourceMapFatalParts {
				if err := rustfmt.WriteString(writer, part); err != nil {
					return err
				}
			}
			return nil
		}
		if stderr != nil && stderr.Binding != nil {
			spanRunContext(context, func() { failure = stderr.Binding.PrintStderr(statement, context) })
		} else {
			failure = ruststdio.PrintStderr(statement, context)
		}
	}
	if failure == nil {
		return
	}
	var binding *rusterrno.Binding
	if stderr != nil {
		binding = stderr.UnixErrorBinding
	}
	failure = sourceMapWrapIOErrorWithBinding(failure, binding)
	defer spanCloseOwnedCallback(context, func(unwind rustfmt.Context) { CloseSourceIOError(failure, unwind) })
	context.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: context.SourcePanicLocation(SourceMapPanicStderr), CanUnwind: true}, func(processing rustfmt.Context) any {
		var out strings.Builder
		f := &rustfmt.Formatter{Writer: &out, Options: rustfmt.DefaultOptions(), Context: processing}
		if err := f.WriteString("failed printing to stderr: "); err != nil {
			processing.Panic(err)
		}
		_ = FormatSourceIOErrorDisplay(failure, f)
		return out.String()
	})
}
