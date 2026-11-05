// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
	"strconv"
)

const hygieneTraceFile = "compiler/rustc_span/src/hygiene.rs"
const hygieneTracingProfileKey = "rustc_span::hygiene::tracing_metadata"

// HygieneTracingProfile borrows one selected build's verbatim static metadata.
// Its callsite cells and metadata remain immutable after construction. Distinct
// selected builds have distinct callsite identity and registration caches.
type HygieneTracingProfile struct {
	name  string
	sites map[uint32]*rusttracing.Callsite
}

func (p *HygieneTracingProfile) Name() string { return p.name }
func NewHygieneTracingProfile(name string, metadata map[uint32]rusttracing.Metadata, runtime ...*rusttracing.Runtime) *HygieneTracingProfile {
	rt := rusttracing.DefaultRuntime()
	if len(runtime) > 0 {
		rt = runtime[0]
	}
	p := &HygieneTracingProfile{name: name, sites: map[uint32]*rusttracing.Callsite{}}
	for line := range logicalHygieneTracingProfile.sites {
		data, ok := metadata[line]
		if !ok {
			panic("hygiene: incomplete selected tracing metadata profile")
		}
		p.sites[line] = rusttracing.NewCallsiteIn(rt, data)
	}
	return p
}
func WithHygieneTracingProfile(ctx rustfmt.Context, profile *HygieneTracingProfile) rustfmt.Context {
	if profile == nil || len(profile.sites) != len(logicalHygieneTracingProfile.sites) {
		panic("hygiene: invalid selected tracing metadata profile")
	}
	metadata := rustfmt.RuntimeMetadata{}
	if ctx.Runtime != nil {
		metadata = *ctx.Runtime
	}
	profiles := make(map[string]any, len(metadata.Profiles)+1)
	for name, value := range metadata.Profiles {
		profiles[name] = value
	}
	profiles[hygieneTracingProfileKey] = profile
	metadata.Profiles = profiles
	ctx.Runtime = &metadata
	return ctx
}

// LogicalHygieneTracingProfile is the explicitly named Go convenience view.
// Operations without an attached selected-build profile use this view. It does
// not claim that its compiler/... spelling is the original build's metadata.
func LogicalHygieneTracingProfile() *HygieneTracingProfile { return logicalHygieneTracingProfile }
func hygieneSelectedTraceCallsite(ctx rustfmt.Context, logical *rusttracing.Callsite) *rusttracing.Callsite {
	if ctx.Runtime != nil {
		if profile, ok := ctx.Runtime.Profiles[hygieneTracingProfileKey].(*HygieneTracingProfile); ok && profile != nil {
			return profile.sites[logical.Metadata.Line]
		}
	}
	return logical
}

func hygieneTraceSite(line uint32, level rusttracing.Level) *rusttracing.Callsite {
	return rusttracing.NewCallsite(rusttracing.Metadata{Name: "event " + hygieneTraceFile + ":" + strconv.FormatUint(uint64(line), 10), Target: "rustc_span::hygiene", ModulePath: "rustc_span::hygiene", File: hygieneTraceFile, Line: line, Level: level, Kind: rusttracing.EventKind, Fields: []string{"message"}})
}

var (
	hygieneTraceMarks           = hygieneTraceSite(635, rusttracing.Debug)
	hygieneTraceWalk            = hygieneTraceSite(645, rusttracing.Debug)
	hygieneTraceWalkCtxt        = hygieneTraceSite(646, rusttracing.Debug)
	hygieneTraceWalkOuter       = hygieneTraceSite(649, rusttracing.Debug)
	hygieneTraceWalkData        = hygieneTraceSite(651, rusttracing.Debug)
	hygieneTraceWalkReturn      = hygieneTraceSite(654, rusttracing.Debug)
	hygieneTraceCollapsed       = hygieneTraceSite(661, rusttracing.Debug)
	hygieneTraceCollapsedCtxt   = hygieneTraceSite(662, rusttracing.Debug)
	hygieneTraceCollapsedOuter  = hygieneTraceSite(668, rusttracing.Debug)
	hygieneTraceCollapsedData   = hygieneTraceSite(670, rusttracing.Debug)
	hygieneTraceCollapsedReturn = hygieneTraceSite(676, rusttracing.Debug)
	hygieneTraceEncodeRound     = hygieneTraceSite(1466, rusttracing.Debug)
	hygieneTraceEncodeDone      = hygieneTraceSite(1500, rusttracing.Debug)
	hygieneTraceDecodeRootExpn  = hygieneTraceSite(1561, rusttracing.Trace)
	hygieneTraceDecodeRootCtxt  = hygieneTraceSite(1610, rusttracing.Trace)
	hygieneTraceDisambiguator   = hygieneTraceSite(1670, rusttracing.Debug)
)
var logicalHygieneTracingProfile = &HygieneTracingProfile{name: "logical-compiler-path", sites: map[uint32]*rusttracing.Callsite{
	635: hygieneTraceMarks, 645: hygieneTraceWalk, 646: hygieneTraceWalkCtxt, 649: hygieneTraceWalkOuter, 651: hygieneTraceWalkData, 654: hygieneTraceWalkReturn, 661: hygieneTraceCollapsed, 662: hygieneTraceCollapsedCtxt, 668: hygieneTraceCollapsedOuter, 670: hygieneTraceCollapsedData, 676: hygieneTraceCollapsedReturn, 1466: hygieneTraceEncodeRound, 1500: hygieneTraceEncodeDone, 1561: hygieneTraceDecodeRootExpn, 1610: hygieneTraceDecodeRootCtxt, 1670: hygieneTraceDisambiguator,
}}

func hygieneTraceMessage(ctx rustfmt.Context, site *rusttracing.Callsite, render func(*rustfmt.Formatter) error) {
	rusttracing.Event(rusttracing.ScopeFromContext(ctx), hygieneSelectedTraceCallsite(ctx, site), ctx, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "message", Value: rusttracing.MessageField(render)}}
	})
}
func hygieneTraceLiteral(ctx rustfmt.Context, site *rusttracing.Callsite, text string) {
	hygieneTraceMessage(ctx, site, func(f *rustfmt.Formatter) error { return f.WriteString(text) })
}
func hygieneMessage(f *rustfmt.Formatter, literals []string, arguments ...func(*rustfmt.Formatter) error) error {
	for i, argument := range arguments {
		if err := f.WriteString(literals[i]); err != nil {
			return err
		}
		child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}
		if err := argument(child); err != nil {
			return err
		}
	}
	if tail := literals[len(literals)-1]; tail != "" {
		return f.WriteString(tail)
	}
	return nil
}
