// SPDX-License-Identifier: MIT OR Apache-2.0
// Mirrors compiler/rustc_span/src/edit_distance/tests.rs, in declaration and
// assertion order, at 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
package editdistance_test

import (
	"testing"
	"unicode/utf8"

	span "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
)

func assertDistance(t *testing.T, actual, expected span.Option[uint]) {
	t.Helper()
	if actual != expected {
		t.Fatalf("distance: got %v, want %v", actual, expected)
	}
}

// Rust: test_edit_distance.
func TestEditDistance(t *testing.T) {
	// The source range is exclusive of char::MAX, and excludes surrogates.
	for c := rune(0); c < utf8.MaxRune; c++ {
		if !utf8.ValidRune(c) {
			continue
		}
		text := string(c)
		assertDistance(t, span.EditDistance(text, text, ^uint(0)), span.Some(uint(0)))
	}
	a := "\nMäry häd ä little lämb\n\nLittle lämb\n"
	b := "\nMary häd ä little lämb\n\nLittle lämb\n"
	c := "Mary häd ä little lämb\n\nLittle lämb\n"
	assertDistance(t, span.EditDistance(a, b, ^uint(0)), span.Some(uint(1)))
	assertDistance(t, span.EditDistance(b, a, ^uint(0)), span.Some(uint(1)))
	assertDistance(t, span.EditDistance(a, c, ^uint(0)), span.Some(uint(2)))
	assertDistance(t, span.EditDistance(c, a, ^uint(0)), span.Some(uint(2)))
	assertDistance(t, span.EditDistance(b, c, ^uint(0)), span.Some(uint(1)))
	assertDistance(t, span.EditDistance(c, b, ^uint(0)), span.Some(uint(1)))
}

// Rust: test_edit_distance_limit.
func TestEditDistanceLimit(t *testing.T) {
	assertDistance(t, span.EditDistance("abc", "abcd", 1), span.Some(uint(1)))
	assertDistance(t, span.EditDistance("abc", "abcd", 0), span.None[uint]())
	assertDistance(t, span.EditDistance("abc", "xyz", 3), span.Some(uint(3)))
	assertDistance(t, span.EditDistance("abc", "xyz", 2), span.None[uint]())
}

// Rust: test_method_name_similarity_score.
func TestMethodNameSimilarityScore(t *testing.T) {
	assertDistance(t, span.EditDistanceWithSubstrings("empty", "is_empty", 1), span.Some(uint(1)))
	assertDistance(t, span.EditDistanceWithSubstrings("shrunk", "rchunks", 2), span.None[uint]())
	assertDistance(t, span.EditDistanceWithSubstrings("abc", "abcd", 1), span.Some(uint(1)))
	assertDistance(t, span.EditDistanceWithSubstrings("a", "abcd", 1), span.None[uint]())
	assertDistance(t, span.EditDistanceWithSubstrings("edf", "eq", 1), span.None[uint]())
	assertDistance(t, span.EditDistanceWithSubstrings("abc", "xyz", 3), span.Some(uint(3)))
	assertDistance(t, span.EditDistanceWithSubstrings("abcdef", "abcdef", 2), span.Some(uint(0)))
}

// Rust: test_find_best_match_for_name.
func TestFindBestMatchForName(t *testing.T) {
	session := span.NewSpanEncodingSession(span.DefaultSpanEncodingConfig())
	intern := func(text string) span.Symbol { return span.InternSymbol(session, text) }
	assertMatch := func(actual, expected span.Option[span.Symbol]) {
		t.Helper()
		if actual != expected {
			t.Fatalf("match: got %v, want %v", actual, expected)
		}
	}
	input := []span.Symbol{intern("aaab"), intern("aaabc")}
	assertMatch(span.FindBestMatchForName(session, input, intern("aaaa"), span.None[uint]()), span.Some(intern("aaab")))
	assertMatch(span.FindBestMatchForName(session, input, intern("1111111111"), span.None[uint]()), span.None[span.Symbol]())
	input = []span.Symbol{intern("AAAA")}
	assertMatch(span.FindBestMatchForName(session, input, intern("aaaa"), span.None[uint]()), span.Some(intern("AAAA")))
	input = []span.Symbol{intern("AAAA")}
	assertMatch(span.FindBestMatchForName(session, input, intern("aaaa"), span.Some(uint(4))), span.Some(intern("AAAA")))
	input = []span.Symbol{intern("a_longer_variable_name")}
	assertMatch(span.FindBestMatchForName(session, input, intern("a_variable_longer_name"), span.None[uint]()), span.Some(intern("a_longer_variable_name")))
}

// Rust: test_precise_algorithm.
func TestPreciseAlgorithm(t *testing.T) {
	if span.EditDistance("ab", "ba", ^uint(0)) == span.Some(uint(2)) {
		t.Fatal("unexpected Levenshtein distance")
	}
	if span.EditDistance("abde", "bcaed", ^uint(0)) == span.Some(uint(3)) {
		t.Fatal("unexpected unrestricted Damerau-Levenshtein distance")
	}
	assertDistance(t, span.EditDistance("abde", "bcaed", ^uint(0)), span.Some(uint(4)))
}
