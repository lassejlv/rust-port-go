// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete port of compiler/rustc_data_structures/src/thousands/tests.rs at
// rust-lang/rust@2d20b42b883dcfc014f3c2c56081eeb6091c2179, plus edge cases.
// Source SHA-256: 9596a03a2af2075b20952c1483b31d43a6b9d76918b9184148a96bb60b6de56f
package thousands

import (
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestFormatWithUnderscores(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"", ""}, {"0", "0"}, {"12345.67e14", "12_345.67e14"},
		{"-1234.5678e10", "-1_234.5678e10"}, {"------", "------"},
		{"abcdefgh", "abcdefgh"}, {"-1b", "-1b"}, {"-3456xyz", "-3_456xyz"},
	} {
		if got := formatWithUnderscores(test.input); got != test.want {
			t.Errorf("formatWithUnderscores(%q) = %q; want %q", test.input, got, test.want)
		}
	}
}

func TestUsizeWithUnderscores(t *testing.T) {
	for _, test := range []struct {
		input uint
		want  string
	}{
		{0, "0"}, {1, "1"}, {99, "99"}, {345, "345"}, {1000, "1_000"},
		{12001, "12_001"}, {999999, "999_999"}, {1000000, "1_000_000"},
		{12345678, "12_345_678"},
	} {
		if got := UsizeWithUnderscores(test.input); got != test.want {
			t.Errorf("UsizeWithUnderscores(%d) = %q; want %q", test.input, got, test.want)
		}
	}
}

func TestIsizeWithUnderscores(t *testing.T) {
	for _, test := range []struct {
		input int
		want  string
	}{
		{0, "0"}, {-1, "-1"}, {99, "99"}, {345, "345"}, {-1000, "-1_000"},
		{12001, "12_001"}, {-999999, "-999_999"}, {1000000, "1_000_000"},
		{-12345678, "-12_345_678"},
	} {
		if got := IsizeWithUnderscores(test.input); got != test.want {
			t.Errorf("IsizeWithUnderscores(%d) = %q; want %q", test.input, got, test.want)
		}
	}
}

func TestF64p1WithUnderscores(t *testing.T) {
	for _, test := range []struct {
		input float64
		want  string
	}{
		{0, "0.0"}, {0.00000001, "0.0"}, {-0.00000001, "-0.0"},
		{0.9999999, "1.0"}, {-0.9999999, "-1.0"}, {345.4999999, "345.5"},
		{-100000, "-100_000.0"}, {123456789.123456789, "123_456_789.1"},
		{-123456789.123456789, "-123_456_789.1"},
	} {
		if got := F64p1WithUnderscores(test.input); got != test.want {
			t.Errorf("F64p1WithUnderscores(%g) = %q; want %q", test.input, got, test.want)
		}
	}
}

func TestDigitRunBoundaries(t *testing.T) {
	for _, test := range []struct{ input, want string }{
		{"-", "-"}, {"+12345", "+12345"}, {"12345_6789", "12_345_6789"},
		{"1234567e12345", "1_234_567e12345"}, {"1234.567890", "1_234.567890"},
		{"1234é5678", "1_234é5678"}, {"-1234\x005678", "-1_234\x005678"},
		{"１２３４５", "１２３４５"}, {"١٢٣٤٥", "١٢٣٤٥"},
		{"123٤5678", "123٤5678"}, {"-1234🦀5678", "-1_234🦀5678"},
	} {
		if got := formatWithUnderscores(test.input); got != test.want {
			t.Errorf("formatWithUnderscores(%q) = %q; want %q", test.input, got, test.want)
		}
	}
	// Rust String cannot contain invalid UTF-8; reject that extra Go input domain.
	defer func() {
		if recover() == nil {
			t.Error("invalid UTF-8 input did not panic")
		}
	}()
	formatWithUnderscores("1234\xff")
}

func TestIntegerHostLimits(t *testing.T) {
	wantUnsigned, wantPositive, wantNegative := "4_294_967_295", "2_147_483_647", "-2_147_483_648"
	if strconv.IntSize == 64 {
		wantUnsigned, wantPositive, wantNegative = "18_446_744_073_709_551_615", "9_223_372_036_854_775_807", "-9_223_372_036_854_775_808"
	}
	maxInt := int(^uint(0) >> 1)
	if got := UsizeWithUnderscores(^uint(0)); got != wantUnsigned {
		t.Errorf("usize::MAX = %q; want %q", got, wantUnsigned)
	}
	if got := IsizeWithUnderscores(maxInt); got != wantPositive {
		t.Errorf("isize::MAX = %q; want %q", got, wantPositive)
	}
	if got := IsizeWithUnderscores(-maxInt - 1); got != wantNegative {
		t.Errorf("isize::MIN = %q; want %q", got, wantNegative)
	}
}

func TestFloatSpecialsAndHalfwayRounding(t *testing.T) {
	for _, test := range []struct {
		input float64
		want  string
	}{
		{math.Copysign(0, -1), "-0.0"}, {math.Inf(1), "inf"}, {math.Inf(-1), "-inf"},
		{math.Float64frombits(0x7ff0000000000001), "NaN"}, {math.Float64frombits(0xfff8000000000001), "NaN"},
		{math.SmallestNonzeroFloat64, "0.0"}, {-math.SmallestNonzeroFloat64, "-0.0"},
		{1.25, "1.2"}, {1.75, "1.8"}, {-1.25, "-1.2"}, {-1.75, "-1.8"},
		{math.Nextafter(1.25, math.Inf(1)), "1.3"},
		{math.Nextafter(1.25, math.Inf(-1)), "1.2"},
	} {
		if got := F64p1WithUnderscores(test.input); got != test.want {
			t.Errorf("float bits %016x: got %q; want %q", math.Float64bits(test.input), got, test.want)
		}
	}
	if got := F64p1WithUnderscores(math.MaxFloat64); strings.ContainsAny(got, "eE") || !strings.HasSuffix(got, ".0") {
		t.Fatalf("largest finite float lost fixed precision: %q", got)
	}
}
