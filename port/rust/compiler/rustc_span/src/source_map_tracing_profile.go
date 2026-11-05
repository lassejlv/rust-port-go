// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
)

const sourceMapTracingProfileKey = "rustc_span::source_map::tracing_metadata"

// SourceMapTracingProfile owns immutable callsite cells for one selected
// build's verbatim metadata. IDs are semantic sites, not line numbers: the
// instrument span and return event share a line but remain distinct cells.
type SourceMapTracingProfile struct {
	name  string
	sites map[string]*rusttracing.Callsite
}

var logicalSourceMapTracingProfile = &SourceMapTracingProfile{name: "logical-compiler-path", sites: map[string]*rusttracing.Callsite{
	"working_dir": sourceMapTraceWorkingDir, "valid_span": sourceMapTraceValid, "span_lo": sourceMapTraceLo, "span_hi": sourceMapTraceHi, "span_lines": sourceMapTraceLines,
	"width": sourceMapTraceWidth, "width_empty": sourceMapTraceWidthEmpty, "width_locations": sourceMapTraceWidthLoc, "width_files": sourceMapTraceWidthFiles, "width_indices": sourceMapTraceWidthIndex, "width_limit": sourceMapTraceWidthLimit, "width_length": sourceMapTraceWidthLen, "width_malformed": sourceMapTraceWidthMalformed,
	"remap": sourceMapTraceRemap, "remap_try": sourceMapTraceRemapTry, "remap_match": sourceMapTraceRemapMatch, "remap_miss": sourceMapTraceRemapMiss, "remap_none": sourceMapTraceRemapNone, "remap_return": sourceMapTraceRemapReturn, "real_filename": sourceMapTraceRealName, "reverse": sourceMapTraceReverse, "reverse_return": sourceMapTraceReverseReturn,
}}

func (p *SourceMapTracingProfile) Name() string { return p.name }
func (p *SourceMapTracingProfile) Metadata() map[string]rusttracing.Metadata {
	result := make(map[string]rusttracing.Metadata, len(p.sites))
	for id, site := range p.sites {
		value := site.Metadata
		value.Fields = append([]string(nil), value.Fields...)
		result[id] = value
	}
	return result
}

// LogicalSourceMapTracingProfile is an explicitly named Go convenience view.
// Its compiler/... spelling does not claim to be the original build's path.
func LogicalSourceMapTracingProfile() *SourceMapTracingProfile { return logicalSourceMapTracingProfile }
func NewSourceMapTracingProfile(name string, metadata map[string]rusttracing.Metadata, runtime ...*rusttracing.Runtime) *SourceMapTracingProfile {
	rt := rusttracing.DefaultRuntime()
	if len(runtime) > 0 {
		rt = runtime[0]
	}
	p := &SourceMapTracingProfile{name: name, sites: make(map[string]*rusttracing.Callsite, len(logicalSourceMapTracingProfile.sites))}
	for id, logical := range logicalSourceMapTracingProfile.sites {
		value, ok := metadata[id]
		if !ok {
			panic("source-map: incomplete selected tracing metadata profile")
		}
		if value.Kind != logical.Metadata.Kind || value.Level != logical.Metadata.Level || len(value.Fields) != len(logical.Metadata.Fields) {
			panic("source-map: incompatible selected tracing site")
		}
		for i, field := range value.Fields {
			if field != logical.Metadata.Fields[i] {
				panic("source-map: incompatible selected tracing fields")
			}
		}
		value.Fields = append([]string(nil), value.Fields...)
		p.sites[id] = rusttracing.NewCallsiteIn(rt, value)
	}
	return p
}
func WithSourceMapTracingProfile(context rustfmt.Context, profile *SourceMapTracingProfile) rustfmt.Context {
	if profile == nil || len(profile.sites) != len(logicalSourceMapTracingProfile.sites) {
		panic("source-map: invalid selected tracing metadata profile")
	}
	metadata := rustfmt.RuntimeMetadata{}
	if context.Runtime != nil {
		metadata = *context.Runtime
	}
	profiles := make(map[string]any, len(metadata.Profiles)+1)
	for key, value := range metadata.Profiles {
		profiles[key] = value
	}
	profiles[sourceMapTracingProfileKey] = profile
	metadata.Profiles = profiles
	context.Runtime = &metadata
	return context
}
func sourceMapSelectedTraceSite(context rustfmt.Context, logical *rusttracing.Callsite) *rusttracing.Callsite {
	if context.Runtime != nil {
		if profile, ok := context.Runtime.Profiles[sourceMapTracingProfileKey].(*SourceMapTracingProfile); ok && profile != nil {
			for id, site := range logicalSourceMapTracingProfile.sites {
				if site == logical {
					return profile.sites[id]
				}
			}
		}
	}
	return logical
}
