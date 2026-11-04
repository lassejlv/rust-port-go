// Duration adaptation for the port of Cargo src/util/queue.rs.
// Licensed under MIT OR Apache-2.0; see ../../LICENSE-MIT and LICENSE-APACHE.
package util

import "time"

const durationNanosPerSecond = uint64(1_000_000_000)

// Duration represents a nonnegative Rust std::time::Duration, including its
// entire uint64-seconds range. Its zero value is a zero-length duration.
// Unlike time.Duration, it does not overflow after approximately 292 years.
type Duration struct {
	seconds uint64
	nanos   uint32
}

// NewDuration constructs a duration, carrying whole seconds from nanoseconds.
// It panics if this normalization overflows uint64, as Rust Duration::new does.
func NewDuration(seconds uint64, nanoseconds uint32) Duration {
	carry := uint64(nanoseconds) / durationNanosPerSecond
	if seconds > ^uint64(0)-carry {
		panic("duration seconds overflow")
	}
	return Duration{seconds: seconds + carry, nanos: uint32(uint64(nanoseconds) % durationNanosPerSecond)}
}

// DurationFromGo adapts a Go duration without truncation. Negative durations
// panic because Rust durations cannot be negative.
func DurationFromGo(duration time.Duration) Duration {
	if duration < 0 {
		panic("duration must not be negative")
	}
	return Duration{seconds: uint64(duration / time.Second), nanos: uint32(duration % time.Second)}
}

// Seconds returns whole seconds, corresponding to Rust Duration::as_secs.
func (duration Duration) Seconds() uint64 { return duration.seconds }

// Nanoseconds returns the fractional nanoseconds (always below one billion),
// corresponding to Rust Duration::subsec_nanos rather than as_nanos.
func (duration Duration) Nanoseconds() uint32 { return duration.nanos }

func (duration Duration) isZero() bool { return duration.seconds == 0 && duration.nanos == 0 }

// subtractElapsed saturates at zero. Queue waits account for each interval
// separately so even durations too large for Go's timers remain representable.
func (duration Duration) subtractElapsed(elapsed time.Duration) Duration {
	seconds, nanos := uint64(elapsed/time.Second), uint32(elapsed%time.Second)
	if duration.seconds < seconds || duration.seconds == seconds && duration.nanos <= nanos {
		return Duration{}
	}
	duration.seconds -= seconds
	if duration.nanos < nanos {
		duration.seconds--
		duration.nanos += uint32(durationNanosPerSecond)
	}
	duration.nanos -= nanos
	return duration
}

// Limit individual waits to a day. This keeps the Go timer and monotonic elapsed
// measurements within their ranges while preserving the full Rust duration.
func (duration Duration) waitInterval() time.Duration {
	const interval = 24 * time.Hour
	if duration.seconds >= uint64(interval/time.Second) {
		return interval
	}
	return time.Duration(duration.seconds)*time.Second + time.Duration(duration.nanos)
}
