// SPDX-License-Identifier: MIT OR Apache-2.0
// Whole-file port of Cargo src/util/time_span.rs at
// 24de4721844c7a4d839072bab6fb66d41c9bc1f4, source SHA256
// 185b6ee6c972b98a19ca7577482777c9e706b71869e6a6d3008d5a9a1c36f130.
package util

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustanyhow"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
)

// TimeSpanConfig selects independent source build flags and the explicit
// runtime backtrace capture operation. A zero/nil backtrace operation chooses
// the named disabled profile; it does not infer a Rust environment or stack.
// DebugAssertions does not affect these two source functions, but is retained
// independently from OverflowChecks for exact source-profile comparison.
type TimeSpanConfig struct {
	DebugAssertions, OverflowChecks bool
	Backtrace                       rustanyhow.BacktraceOperations
}

func DefaultTimeSpanConfig() TimeSpanConfig {
	return TimeSpanConfig{DebugAssertions: true, OverflowChecks: true}
}

const TimeSpanMultiplyPanicSite = "cargo::util::time_span::factor_multiply"

// TimeSpanPanicLocationsAtPinnedRevision is an explicit profile. The supplied
// filename is retained verbatim; absent profile metadata remains unlocated.
func TimeSpanPanicLocationsAtPinnedRevision(file string) map[string]rustfmt.SourceLocation {
	return map[string]rustfmt.SourceLocation{TimeSpanMultiplyPanicSite: {File: file, Line: 37, Column: 30}}
}

func ParseTimeSpan(span string) (Duration, error) {
	return ParseTimeSpanWithContext(span, DefaultTimeSpanConfig(), rustfmt.Context{})
}
func ParseTimeSpanWithContext(span string, config TimeSpanConfig, context rustfmt.Context) (Duration, error) {
	if duration, ok := MaybeParseTimeSpanWithContext(span, config, context); ok {
		return duration, nil
	}
	const prefix = "expected a value of the form \"N seconds/minutes/hours/days/weeks/months\", got: "
	capacity := ruststr.EstimatedFormatCapacitySDK199(uint(len(prefix)), false, false)
	message := ruststr.NewOwnedStringWithCapacity(capacity, context)
	owned := true
	defer func() {
		if p := recover(); p != nil {
			if context.ProcessingPanic {
				context.ResumeUnwind(p)
			}
			if owned {
				unwind := context
				unwind.Unwinding = true
				message.CloseWithContext(unwind)
			}
			context.ResumeUnwind(p)
		}
	}()
	writer := ruststr.StringWriter{Cell: message.BorrowWithContext(context), Context: context}
	_, _ = writer.WriteString(prefix)
	f := rustfmt.Formatter{Writer: &writer, Options: rustfmt.DefaultOptions(), Context: context}
	if err := rustfmt.FormatDebugString(span, &f); err != nil {
		context.Panic(err)
	}
	owned = false
	return Duration{}, rustanyhow.NewOwnedStringErrorWithContext(message, config.Backtrace, context)
}

func MaybeParseTimeSpan(span string) (Duration, bool) {
	return MaybeParseTimeSpanWithContext(span, DefaultTimeSpanConfig(), rustfmt.Context{})
}
func MaybeParseTimeSpanWithContext(span string, config TimeSpanConfig, context rustfmt.Context) (Duration, bool) {
	if !utf8.ValidString(span) {
		context.Panic("time_span: input must be valid UTF-8")
	}
	rightIndex := 0
	for rightIndex < len(span) && span[rightIndex] >= '0' && span[rightIndex] <= '9' {
		rightIndex++
	}
	if rightIndex == len(span) {
		return Duration{}, false
	}
	left, right := span[:rightIndex], span[rightIndex:]
	if strings.HasPrefix(right, " ") {
		right = right[1:]
	}
	count, err := strconv.ParseUint(left, 10, 64)
	if err != nil {
		return Duration{}, false
	}
	var factor uint64
	switch right {
	case "second", "seconds":
		factor = 1
	case "minute", "minutes":
		factor = 60
	case "hour", "hours":
		factor = 60 * 60
	case "day", "days":
		factor = 24 * 60 * 60
	case "week", "weeks":
		factor = 7 * 24 * 60 * 60
	case "month", "months":
		factor = 2_629_746
	default:
		return Duration{}, false
	}
	if config.OverflowChecks && count > ^uint64(0)/factor {
		context.PanicAt(rustfmt.SourcePanicInfo{Payload: "attempt to multiply with overflow", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation(TimeSpanMultiplyPanicSite), CanUnwind: true})
	}
	return Duration{seconds: factor * count}, true
}
