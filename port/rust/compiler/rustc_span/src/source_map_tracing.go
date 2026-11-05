// SPDX-License-Identifier: MIT OR Apache-2.0
// Exact source-map tracing callsite inventory; the scoped tracing adapter owns
// interest/dispatcher/macro runtime semantics. Context.Runtime borrows the
// current caller Scope; SourceMap and SourceFile retain no ambient Scope.
package rustcspan

import (
	"strconv"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
)

const sourceMapTraceFile = "compiler/rustc_span/src/source_map.rs"
const sourceMapTraceModule = "rustc_span::source_map"

func sourceMapTraceSite(name string, line uint32, level rusttracing.Level, kind rusttracing.Kind, fields ...string) *rusttracing.Callsite {
	if name == "" {
		name = "event " + sourceMapTraceFile + ":" + strconv.FormatUint(uint64(line), 10)
	}
	return rusttracing.NewCallsite(rusttracing.Metadata{Name: name, Target: sourceMapTraceModule, ModulePath: sourceMapTraceModule, File: sourceMapTraceFile, Line: line, Level: level, Kind: kind, Fields: fields})
}

var (
	sourceMapTraceWorkingDir     = sourceMapTraceSite("", 257, rusttracing.Debug, rusttracing.EventKind, "working_dir")
	sourceMapTraceValid          = sourceMapTraceSite("is_valid_span", 539, rusttracing.Trace, rusttracing.SpanKind, "sp")
	sourceMapTraceLo             = sourceMapTraceSite("", 542, rusttracing.Trace, rusttracing.EventKind, "lo")
	sourceMapTraceHi             = sourceMapTraceSite("", 544, rusttracing.Trace, rusttracing.EventKind, "hi")
	sourceMapTraceLines          = sourceMapTraceSite("", 562, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceWidth          = sourceMapTraceSite("find_width_of_character_at_span", 1015, rusttracing.Info, rusttracing.SpanKind, "forwards")
	sourceMapTraceWidthEmpty     = sourceMapTraceSite("", 1018, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceWidthLoc       = sourceMapTraceSite("", 1024, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceWidthFiles     = sourceMapTraceSite("", 1027, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceWidthIndex     = sourceMapTraceSite("", 1033, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceWidthLimit     = sourceMapTraceSite("", 1038, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceWidthLen       = sourceMapTraceSite("", 1043, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceWidthMalformed = sourceMapTraceSite("", 1046, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceRemap          = sourceMapTraceSite("remap_path_prefix", 1188, rusttracing.Debug, rusttracing.SpanKind, "path")
	sourceMapTraceRemapTry       = sourceMapTraceSite("", 1197, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceRemapMatch     = sourceMapTraceSite("", 1211, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceRemapMiss      = sourceMapTraceSite("", 1215, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceRemapNone      = sourceMapTraceSite("", 1219, rusttracing.Debug, rusttracing.EventKind, "message")
	sourceMapTraceRemapReturn    = sourceMapTraceSite("", 1188, rusttracing.Debug, rusttracing.EventKind, "return")
	sourceMapTraceRealName       = sourceMapTraceSite("", 1236, rusttracing.Debug, rusttracing.EventKind, "local_path", "remapped_path", "was_remapped", "self.filename_remapping_scopes")
	sourceMapTraceReverse        = sourceMapTraceSite("reverse_map_prefix_heuristically", 1300, rusttracing.Debug, rusttracing.SpanKind, "path")
	sourceMapTraceReverseReturn  = sourceMapTraceSite("", 1300, rusttracing.Debug, rusttracing.EventKind, "return")
)

func sourceMapTraceEvent(context rustfmt.Context, site *rusttracing.Callsite, fields func() []rusttracing.FieldValue) {
	rusttracing.Event(rusttracing.ScopeFromContext(context), sourceMapSelectedTraceSite(context, site), context, fields)
}
func sourceMapTraceMessage(context rustfmt.Context, site *rusttracing.Callsite, format func(*rustfmt.Formatter) error) {
	sourceMapTraceEvent(context, site, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "message", Value: rusttracing.MessageField(format)}}
	})
}
func sourceMapTraceLiteral(context rustfmt.Context, site *rusttracing.Callsite, message string) {
	sourceMapTraceMessage(context, site, func(f *rustfmt.Formatter) error { return f.WriteString(message) })
}
func sourceMapTraceSpan(context rustfmt.Context, site *rusttracing.Callsite, fields func() []rusttracing.FieldValue) (*rusttracing.Span, *rusttracing.Scope) {
	scope := rusttracing.ScopeFromContext(context)
	span := rusttracing.NewSpan(scope, sourceMapSelectedTraceSite(context, site), context, fields)
	// The caller must install span's deferred Close before calling Enter: an
	// entering subscriber may panic. Entry remains explicit at callsites.
	return span, scope
}
