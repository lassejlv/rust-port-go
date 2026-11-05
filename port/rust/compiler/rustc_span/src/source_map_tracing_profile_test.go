// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
)

func TestSourceMapTracingProfileCellsAndContext(t *testing.T) {
	metadata := LogicalSourceMapTracingProfile().Metadata()
	if len(metadata) != 22 {
		t.Fatal("source tracing inventory changed")
	}
	for id, value := range metadata {
		value.File = "verbatim/../original/source_map.rs"
		if value.Kind == rusttracing.EventKind {
			value.Name = "selected event filename"
		}
		metadata[id] = value
	}
	profile := NewSourceMapTracingProfile("selected-build", metadata)
	ops := &rustfmt.PanicOperations{}
	value := new(int)
	other := new(int)
	base := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: value, Panic: ops, Profiles: map[string]any{"other": other}}}
	context := WithSourceMapTracingProfile(base, profile)
	if context.Runtime.Value != value || context.Runtime.Panic != ops || context.Runtime.Profiles["other"] != other || len(base.Runtime.Profiles) != 1 {
		t.Fatal("profile installation lost or mutated caller state")
	}
	for _, pair := range [][2]*rusttracing.Callsite{{sourceMapTraceRemap, sourceMapTraceRemapReturn}, {sourceMapTraceReverse, sourceMapTraceReverseReturn}} {
		span, event := sourceMapSelectedTraceSite(context, pair[0]), sourceMapSelectedTraceSite(context, pair[1])
		if span == event || span.Metadata.Line != event.Metadata.Line || span.Metadata.Kind == event.Metadata.Kind {
			t.Fatal("same-line span and return event lost distinct identity")
		}
		if span != sourceMapSelectedTraceSite(context, pair[0]) || event != sourceMapSelectedTraceSite(context, pair[1]) {
			t.Fatal("selected callsite was recreated")
		}
		if span.Metadata.File != "verbatim/../original/source_map.rs" || event.Metadata.Name != "selected event filename" {
			t.Fatal("verbatim metadata was normalized")
		}
	}
	metadata["remap"].Fields[0] = "caller's changed copy"
	if sourceMapSelectedTraceSite(context, sourceMapTraceRemap).Metadata.Fields[0] != "path" {
		t.Fatal("caller mutation reached immutable metadata")
	}
	copy := profile.Metadata()
	copy["remap"].Fields[0] = "second changed copy"
	if profile.Metadata()["remap"].Fields[0] != "path" {
		t.Fatal("profile getter exposed mutable fields")
	}
}
