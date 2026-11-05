// SPDX-License-Identifier: MIT OR Apache-2.0
// All original inline tests and21 assert_eq! bodies, in source order.
package util

import "testing"

func timeSpanAssert(t *testing.T, input string, expected uint64, present bool) {
	t.Helper()
	actual, ok := MaybeParseTimeSpan(input)
	if ok != present || ok && (actual.Seconds() != expected || actual.Nanoseconds() != 0) {
		t.Fatalf("%q: %v %v, wanted %d %v", input, actual, ok, expected, present)
	}
}
func TestTimeSpanUpstreamTimeSpans(t *testing.T) {
	timeSpanAssert(t, "0 seconds", 0, true)
	timeSpanAssert(t, "1second", 1, true)
	timeSpanAssert(t, "23 seconds", 23, true)
	timeSpanAssert(t, "5 minutes", 60*5, true)
	timeSpanAssert(t, "2 hours", 60*60*2, true)
	timeSpanAssert(t, "1 day", 60*60*24, true)
	timeSpanAssert(t, "2 weeks", 60*60*24*14, true)
	timeSpanAssert(t, "6 months", 2_629_746*6, true)
}
func TestTimeSpanUpstreamTimeSpanErrors(t *testing.T) {
	timeSpanAssert(t, "", 0, false)
	timeSpanAssert(t, "1", 0, false)
	timeSpanAssert(t, "second", 0, false)
	timeSpanAssert(t, "+2 seconds", 0, false)
	timeSpanAssert(t, "day", 0, false)
	timeSpanAssert(t, "-1 days", 0, false)
	timeSpanAssert(t, "1.5 days", 0, false)
	timeSpanAssert(t, "1 dayz", 0, false)
	timeSpanAssert(t, "always", 0, false)
	timeSpanAssert(t, "never", 0, false)
	timeSpanAssert(t, "1 day ", 0, false)
	timeSpanAssert(t, " 1 day", 0, false)
	timeSpanAssert(t, "1  second", 0, false)
}
