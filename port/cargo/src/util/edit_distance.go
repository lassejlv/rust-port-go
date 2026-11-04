// Port of Cargo src/util/edit_distance.rs.
// Upstream: rust-lang/cargo@24de4721844c7a4d839072bab6fb66d41c9bc1f4
// Source SHA-256: 073eb1a086cd2d2e74d1d8ce98b87013a8756f0187313ea1b7340b63184c6407
// Licensed under MIT OR Apache-2.0; see ../../LICENSE-MIT and ../../LICENSE-APACHE.
package util

import (
	"fmt"
	"iter"
	"unicode/utf8"
)

// EditDistance finds Cargo's case-insensitive restricted Damerau (optimal string
// alignment) edit distance. The boolean is false when the distance exceeds limit.
// Like Rust &str, inputs must contain valid UTF-8; invalid input panics explicitly.
//
// Lowercasing uses the pinned Rust Unicode tables, including expansions and
// contextual Greek final sigma. The dynamic program uses three rows and keeps
// the shorter string as its column dimension. Dynamic programming takes O(m*n)
// time; lowering and trimming are linear. Auxiliary storage is O(m+n), including
// the lowercased Unicode scalar buffers.
func EditDistance(a, b string, limit uint) (uint, bool) {
	ar := editDistanceLower(a)
	br := editDistanceLower(b)
	if len(ar) < len(br) {
		ar, br = br, ar
	}
	minDistance := uint(len(ar) - len(br))
	if minDistance > limit {
		return 0, false
	}
	for len(br) > 0 && ar[0] == br[0] {
		ar = ar[1:]
		br = br[1:]
	}
	for len(br) > 0 && ar[len(ar)-1] == br[len(br)-1] {
		ar = ar[:len(ar)-1]
		br = br[:len(br)-1]
	}
	if len(br) == 0 {
		return minDistance, true
	}
	previousPrevious := make([]uint, len(br)+1)
	previous := make([]uint, len(br)+1)
	current := make([]uint, len(br)+1)
	for j := range previousPrevious {
		previousPrevious[j] = ^uint(0)
		previous[j] = uint(j)
	}
	for i := 1; i <= len(ar); i++ {
		current[0] = uint(i)
		for j := 1; j <= len(br); j++ {
			substitutionCost := uint(1)
			if ar[i-1] == br[j-1] {
				substitutionCost = 0
			}
			current[j] = min(previous[j]+1, min(current[j-1]+1, previous[j-1]+substitutionCost))
			if i > 1 && j > 1 && ar[i-1] == br[j-2] && ar[i-2] == br[j-1] {
				current[j] = min(current[j], previousPrevious[j-2]+1)
			}
		}
		previousPrevious, previous, current = previous, current, previousPrevious
	}
	distance := previous[len(br)]
	if distance > limit {
		return 0, false
	}
	return distance, true
}

// Closest finds the first candidate with minimum edit distance from choice,
// considering only distances at most three. Candidates are consumed once in
// iterator order; key is called once for each candidate, as in Cargo's iterator.
// Strings returned by key, and choice, must be valid UTF-8 or the function panics.
func Closest[T any](choice string, candidates iter.Seq[T], key func(*T) string) (T, bool) {
	editDistanceRequireUTF8(choice)
	var best T
	bestDistance := uint(0)
	found := false
	for candidate := range candidates {
		distance, ok := EditDistance(choice, key(&candidate), 3)
		if ok && (!found || distance < bestDistance) {
			best, bestDistance, found = candidate, distance, true
		}
	}
	return best, found
}

// ClosestMsg returns Cargo's suggestion suffix for the closest candidate, or an
// empty string when none lies within distance three. All strings must be UTF-8.
// The selected candidate's key is called again when formatting, matching Cargo.
func ClosestMsg[T any](choice string, candidates iter.Seq[T], key func(*T) string, kind string) string {
	editDistanceRequireUTF8(kind)
	candidate, found := Closest(choice, candidates, key)
	if !found {
		return ""
	}
	name := key(&candidate)
	editDistanceRequireUTF8(name)
	return fmt.Sprintf("\n\nhelp: a %s with a similar name exists: `%s`", kind, name)
}

func editDistanceRequireUTF8(value string) {
	if !utf8.ValidString(value) {
		panic("edit_distance: input is not valid UTF-8")
	}
}

// editDistanceLower follows Rust str::to_lowercase. Its only context-sensitive
// mapping is capital sigma; all other mappings come from char::to_lowercase.
func editDistanceLower(value string) []rune {
	editDistanceRequireUTF8(value)
	source := []rune(value)
	result := make([]rune, 0, len(source))
	for i, c := range source {
		if c == 'Σ' && editDistanceFinalSigma(source, i) {
			result = append(result, 'ς')
			continue
		}
		lo, hi := 0, len(editDistanceLowercaseMappings)
		for lo < hi {
			middle := int(uint(lo+hi) >> 1)
			if editDistanceLowercaseMappings[middle].source < c {
				lo = middle + 1
			} else {
				hi = middle
			}
		}
		if lo < len(editDistanceLowercaseMappings) && editDistanceLowercaseMappings[lo].source == c {
			mapping := editDistanceLowercaseMappings[lo]
			result = append(result, mapping.lower[:mapping.count]...)
		} else {
			result = append(result, c)
		}
	}
	return result
}

func editDistanceFinalSigma(source []rune, index int) bool {
	before := false
	for i := index - 1; i >= 0; i-- {
		if editDistanceInRanges(source[i], editDistanceCaseIgnorable) {
			continue
		}
		before = editDistanceInRanges(source[i], editDistanceSigmaCased)
		break
	}
	if !before {
		return false
	}
	for i := index + 1; i < len(source); i++ {
		if editDistanceInRanges(source[i], editDistanceCaseIgnorable) {
			continue
		}
		return !editDistanceInRanges(source[i], editDistanceSigmaCased)
	}
	return true
}

func editDistanceInRanges(c rune, ranges []editDistanceRuneRange) bool {
	lo, hi := 0, len(ranges)
	for lo < hi {
		middle := int(uint(lo+hi) >> 1)
		if ranges[middle].last < c {
			lo = middle + 1
		} else {
			hi = middle
		}
	}
	return lo < len(ranges) && ranges[lo].first <= c
}
