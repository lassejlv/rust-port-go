// SPDX-License-Identifier: MIT OR Apache-2.0
// All tracing callsites in pinned rustc_span/src/lib.rs. Source fmt::Arguments
// remain lazy borrowed values; subscriber fields are never pre-rendered.
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
	"strconv"
)

func libTraceSite(line uint32, level rusttracing.Level) *rusttracing.Callsite {
	const file = "compiler/rustc_span/src/lib.rs"
	return rusttracing.NewCallsite(rusttracing.Metadata{Name: "event " + file + ":" + strconv.FormatUint(uint64(line), 10), Target: "rustc_span", ModulePath: "rustc_span", File: file, Line: line, Level: level, Kind: rusttracing.EventKind, Fields: []string{"message"}})
}

var (
	libTraceMultiByte   = libTraceSite(2519, rusttracing.Debug)
	libTraceBytePos     = libTraceSite(2546, rusttracing.Debug)
	libTraceCharPos     = libTraceSite(2547, rusttracing.Debug)
	libTraceLine        = libTraceSite(2548, rusttracing.Debug)
	libTraceUnavailable = libTraceSite(2570, rusttracing.Info)
)

func libTraceMessage(ctx rustfmt.Context, site *rusttracing.Callsite, format func(*rustfmt.Formatter) error) {
	site = libSelectedTraceSite(ctx, site)
	rusttracing.Event(rusttracing.ScopeFromContext(ctx), site, ctx, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "message", Value: rusttracing.MessageField(format)}}
	})
}
func libTraceMultiByteMessage(ctx rustfmt.Context, mbc *MultiByteChar) {
	libTraceMessage(ctx, libTraceMultiByte, func(f *rustfmt.Formatter) error {
		if err := rustfmt.FormatDebugUint(uint64(mbc.Bytes), 8, f); err != nil {
			return err
		}
		if err := f.WriteString("-byte char at "); err != nil {
			return err
		}
		return mbc.Pos.WriteDebugFormatter(f)
	})
}
func libTraceBytePosMessage(ctx rustfmt.Context, pos, linePos RelativeBytePos) {
	libTraceMessage(ctx, libTraceBytePos, func(f *rustfmt.Formatter) error {
		if err := f.WriteString("byte pos "); err != nil {
			return err
		}
		if err := pos.WriteDebugFormatter(f); err != nil {
			return err
		}
		if err := f.WriteString(" is on the line at byte pos "); err != nil {
			return err
		}
		return linePos.WriteDebugFormatter(f)
	})
}
func libTraceCharPosMessage(ctx rustfmt.Context, pos, linePos CharPos) {
	libTraceMessage(ctx, libTraceCharPos, func(f *rustfmt.Formatter) error {
		if err := f.WriteString("char pos "); err != nil {
			return err
		}
		if err := pos.WriteDebugFormatter(f); err != nil {
			return err
		}
		if err := f.WriteString(" is on the line at char pos "); err != nil {
			return err
		}
		return linePos.WriteDebugFormatter(f)
	})
}
func libTraceLineMessage(ctx rustfmt.Context, line uint) {
	libTraceMessage(ctx, libTraceLine, func(f *rustfmt.Formatter) error {
		if err := f.WriteString("byte is on line: "); err != nil {
			return err
		}
		return rustfmt.FormatDebugUint(uint64(line), uint(spanUintBits()), f)
	})
}
func libTraceUnavailableMessage(ctx rustfmt.Context, line uint, name *FileName) {
	libTraceMessage(ctx, libTraceUnavailable, func(f *rustfmt.Formatter) error {
		if err := f.WriteString("couldn't find line "); err != nil {
			return err
		}
		if err := rustfmt.FormatDebugUint(uint64(line), uint(spanUintBits()), f); err != nil {
			return err
		}
		if err := f.WriteString(" "); err != nil {
			return err
		}
		return name.WriteDebugFormatter(f)
	})
}
