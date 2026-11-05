// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
)

const libTracingProfileKey = "rustc_span::lib::tracing_metadata"

// LibTracingProfile owns stable immutable callsite cells for one selected
// build's verbatim metadata. Its five semantic IDs are the pinned source lines,
// which are distinct in lib.rs. Reuse the same profile for the same build;
// reconstructing profiles would reconstruct the source callsite identities.
type LibTracingProfile struct {
	name  string
	sites map[uint32]*rusttracing.Callsite
}

var logicalLibTracingProfile = &LibTracingProfile{name: "logical-compiler-path", sites: map[uint32]*rusttracing.Callsite{
	2519: libTraceMultiByte, 2546: libTraceBytePos, 2547: libTraceCharPos, 2548: libTraceLine, 2570: libTraceUnavailable,
}}

func (p *LibTracingProfile) Name() string { return p.name }
func (p *LibTracingProfile) Metadata() map[uint32]rusttracing.Metadata {
	result := make(map[uint32]rusttracing.Metadata, len(p.sites))
	for id, site := range p.sites {
		metadata := site.Metadata
		metadata.Fields = append([]string(nil), metadata.Fields...)
		result[id] = metadata
	}
	return result
}

// LogicalLibTracingProfile explicitly identifies the legacy compiler/... Go
// convenience spelling. It does not claim an actual selected build filename.
func LogicalLibTracingProfile() *LibTracingProfile { return logicalLibTracingProfile }

func NewLibTracingProfile(name string, metadata map[uint32]rusttracing.Metadata, runtime ...*rusttracing.Runtime) *LibTracingProfile {
	rt := rusttracing.DefaultRuntime()
	if len(runtime) > 0 {
		rt = runtime[0]
	}
	profile := &LibTracingProfile{name: name, sites: make(map[uint32]*rusttracing.Callsite, len(logicalLibTracingProfile.sites))}
	for id, logical := range logicalLibTracingProfile.sites {
		selected, ok := metadata[id]
		if !ok {
			panic("rustcspan: incomplete selected lib tracing metadata")
		}
		if selected.Kind != logical.Metadata.Kind || selected.Level != logical.Metadata.Level || len(selected.Fields) != len(logical.Metadata.Fields) {
			panic("rustcspan: incompatible selected lib tracing site")
		}
		for i, field := range selected.Fields {
			if field != logical.Metadata.Fields[i] {
				panic("rustcspan: incompatible selected lib tracing fields")
			}
		}
		selected.Fields = append([]string(nil), selected.Fields...)
		profile.sites[id] = rusttracing.NewCallsiteIn(rt, selected)
	}
	return profile
}

// WithLibTracingProfile borrows this selected-build dictionary and preserves
// the caller's tracing payload, panic operations and unrelated profile keys.
func WithLibTracingProfile(context rustfmt.Context, profile *LibTracingProfile) rustfmt.Context {
	if profile == nil || len(profile.sites) != len(logicalLibTracingProfile.sites) {
		panic("rustcspan: invalid selected lib tracing profile")
	}
	metadata := rustfmt.RuntimeMetadata{}
	if context.Runtime != nil {
		metadata = *context.Runtime
	}
	profiles := make(map[string]any, len(metadata.Profiles)+1)
	for key, value := range metadata.Profiles {
		profiles[key] = value
	}
	profiles[libTracingProfileKey] = profile
	metadata.Profiles = profiles
	context.Runtime = &metadata
	return context
}

func libSelectedTraceSite(context rustfmt.Context, logical *rusttracing.Callsite) *rusttracing.Callsite {
	if context.Runtime != nil {
		if profile, ok := context.Runtime.Profiles[libTracingProfileKey].(*LibTracingProfile); ok && profile != nil {
			if selected := profile.sites[logical.Metadata.Line]; selected != nil {
				return selected
			}
		}
	}
	return logical
}
