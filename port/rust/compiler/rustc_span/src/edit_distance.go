// SPDX-License-Identifier: MIT OR Apache-2.0
// Port of compiler/rustc_span/src/edit_distance.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179.
package rustcspan

import (
	"sort"
	"strings"
	"unicode/utf8"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// EditDistance is the source restricted Damerau-Levenshtein algorithm. Counts
// are Unicode scalar counts, including NUL; limit is the complete usize domain.
func EditDistance(a, b string, limit uint) Option[uint] {
	if !utf8.ValidString(a) || !utf8.ValidString(b) {
		panic("rustcspan: str must be valid UTF-8")
	}
	left, right := []rune(a), []rune(b)
	if len(left) < len(right) {
		left, right = right, left
	}
	minDist := uint(len(left) - len(right))
	if minDist > limit {
		return None[uint]()
	}
	for len(right) != 0 && left[0] == right[0] {
		left, right = left[1:], right[1:]
	}
	for len(right) != 0 && left[len(left)-1] == right[len(right)-1] {
		left, right = left[:len(left)-1], right[:len(right)-1]
	}
	if len(right) == 0 {
		return Some(minDist)
	}
	previousPrevious, previous, current := make([]uint, len(right)+1), make([]uint, len(right)+1), make([]uint, len(right)+1)
	for i := range previous {
		previousPrevious[i] = ^uint(0)
		previous[i] = uint(i)
	}
	for i := 1; i <= len(left); i++ {
		current[0] = uint(i)
		for j := 1; j <= len(right); j++ {
			substitutionCost := uint(1)
			if left[i-1] == right[j-1] {
				substitutionCost = 0
			}
			current[j] = min(previous[j]+1, min(current[j-1]+1, previous[j-1]+substitutionCost))
			if i > 1 && j > 1 && left[i-1] == right[j-2] && left[i-2] == right[j-1] {
				current[j] = min(current[j], previousPrevious[j-2]+1)
			}
		}
		previousPrevious, previous, current = previous, current, previousPrevious
	}
	distance := previous[len(right)]
	if distance <= limit {
		return Some(distance)
	}
	return None[uint]()
}

func editDistanceAdd(a, b uint, profile SpanEncodingConfig, context rustfmt.Context, site string) uint {
	if profile.OverflowChecks && b > ^uint(0)-a {
		symbolStaticPanic(context, site, "attempt to add with overflow")
	}
	return a + b
}
func editDistanceMul2(a uint, profile SpanEncodingConfig, context rustfmt.Context) uint {
	if profile.OverflowChecks && a > ^uint(0)/2 {
		symbolStaticPanic(context, PanicEditDistanceLengthMulSite, "attempt to multiply with overflow")
	}
	return a * 2
}

// EditDistanceWithSubstringsWithConfig preserves independent Rust overflow
// checks. In particular usize::MAX + a nonzero length difference panics in a
// checked build and wraps in a build with overflow checks disabled.
func EditDistanceWithSubstringsWithConfig(a, b string, limit uint, profile SpanEncodingConfig, context rustfmt.Context) Option[uint] {
	if !utf8.ValidString(a) || !utf8.ValidString(b) {
		context.Panic("rustcspan: str must be valid UTF-8")
	}
	n, m := uint(utf8.RuneCountInString(a)), uint(utf8.RuneCountInString(b))
	bigLenDiff := editDistanceMul2(n, profile, context) < m || editDistanceMul2(m, profile, context) < n
	lenDiff := n - m
	if m > n {
		lenDiff = m - n
	}
	distance, present := EditDistance(a, b, editDistanceAdd(limit, lenDiff, profile, context, PanicEditDistanceLimitAddSite)).Get()
	if !present {
		return None[uint]()
	}
	score := distance - lenDiff
	if score == 0 && lenDiff > 0 && !bigLenDiff {
		score = 1
	} else if !bigLenDiff {
		score = editDistanceAdd(score, lenDiff/2+lenDiff%2, profile, context, PanicEditDistanceScoreAddSite)
	} else {
		score = editDistanceAdd(score, lenDiff, profile, context, PanicEditDistanceScoreAddSite)
	}
	if score <= limit {
		return Some(score)
	}
	return None[uint]()
}
func EditDistanceWithSubstrings(a, b string, limit uint) Option[uint] {
	return EditDistanceWithSubstringsWithConfig(a, b, limit, DefaultSpanEncodingConfig(), rustfmt.Context{})
}

func FindBestMatchForName(session *SessionGlobals, candidates []Symbol, lookup Symbol, distance Option[uint]) Option[Symbol] {
	return FindBestMatchForNameWithContext(session, candidates, lookup, distance, rustfmt.Context{})
}
func FindBestMatchForNameWithContext(session *SessionGlobals, candidates []Symbol, lookup Symbol, distance Option[uint], context rustfmt.Context) Option[Symbol] {
	return findBestMatchForNameImpl(session, false, candidates, lookup, distance, spanEncodingConfig(session), context)
}
func FindBestMatchForNameWithSubstrings(session *SessionGlobals, candidates []Symbol, lookup Symbol, distance Option[uint]) Option[Symbol] {
	return FindBestMatchForNameWithSubstringsWithContext(session, candidates, lookup, distance, rustfmt.Context{})
}
func FindBestMatchForNameWithSubstringsWithContext(session *SessionGlobals, candidates []Symbol, lookup Symbol, distance Option[uint], context rustfmt.Context) Option[Symbol] {
	return findBestMatchForNameImpl(session, true, candidates, lookup, distance, spanEncodingConfig(session), context)
}
func FindBestMatchForNames(session *SessionGlobals, candidates, lookups []Symbol, distance Option[uint]) Option[Symbol] {
	return FindBestMatchForNamesWithContext(session, candidates, lookups, distance, rustfmt.Context{})
}
func FindBestMatchForNamesWithContext(session *SessionGlobals, candidates, lookups []Symbol, distance Option[uint], context rustfmt.Context) Option[Symbol] {
	var best Option[Symbol]
	var minDistance uint
	for _, lookup := range lookups {
		candidate, present := findBestMatchForNameImpl(session, false, candidates, lookup, distance, spanEncodingConfig(session), context).Get()
		if !present {
			continue
		}
		d, _ := EditDistance(lookup.AsStrWithContext(session, context), candidate.AsStrWithContext(session, context), ^uint(0)).Get()
		if !best.IsSome() || d < minDistance {
			minDistance = d
			best = Some(candidate)
		}
	}
	return best
}
func findBestMatchForNameImpl(session *SessionGlobals, substrings bool, candidates []Symbol, lookupSymbol Symbol, distance Option[uint], profile SpanEncodingConfig, context rustfmt.Context) Option[Symbol] {
	lookup := lookupSymbol.AsStrWithContext(session, context)
	lookupUppercase := symbolUppercase(lookup)
	for _, candidate := range candidates {
		if symbolUppercase(candidate.AsStrWithContext(session, context)) == lookupUppercase {
			return Some(candidate)
		}
	}
	dist, present := distance.Get()
	if !present {
		dist = max(uint(utf8.RuneCountInString(lookup)), 3) / 3
	}
	best := None[Symbol]()
	nextCandidates := make([]Symbol, 0)
	for _, candidate := range candidates {
		var d Option[uint]
		if substrings {
			d = EditDistanceWithSubstringsWithConfig(lookup, candidate.AsStrWithContext(session, context), dist, profile, context)
		} else {
			d = EditDistance(lookup, candidate.AsStrWithContext(session, context), dist)
		}
		if value, ok := d.Get(); ok {
			if value == 0 {
				return Some(candidate)
			}
			if substrings {
				if value < dist {
					dist = value
					nextCandidates = nextCandidates[:0]
				}
				nextCandidates = append(nextCandidates, candidate)
			} else {
				dist = value - 1
			}
			best = Some(candidate)
		}
	}
	if len(nextCandidates) > 1 {
		best = findBestMatchForNameImpl(session, false, nextCandidates, lookupSymbol, Some(uint(len(lookup))), profile, context)
	}
	if best.IsSome() {
		return best
	}
	lookupWords := sortSymbolWords(lookup)
	for _, candidate := range candidates {
		words := sortSymbolWords(candidate.AsStrWithContext(session, context))
		if equalSymbolWords(words, lookupWords) {
			best = Some(candidate)
		}
	}
	return best
}
func sortSymbolWords(name string) []string {
	words := strings.Split(name, "_")
	sort.Strings(words)
	return words
}
func equalSymbolWords(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

type SimilarSymbol struct {
	Symbol        Symbol
	IncorrectCase bool
}

func (value Symbol) FindSimilar(session *SessionGlobals, candidates []Symbol) Option[SimilarSymbol] {
	return value.FindSimilarWithContext(session, candidates, rustfmt.Context{})
}
func (value Symbol) FindSimilarWithContext(session *SessionGlobals, candidates []Symbol, context rustfmt.Context) Option[SimilarSymbol] {
	lowercase := symbolLowercase(value.AsStrWithContext(session, context))
	lowercaseSymbol := InternSymbolWithContext(session, lowercase, context)
	for _, candidate := range candidates {
		if candidate == lowercaseSymbol {
			return Some(SimilarSymbol{candidate, true})
		}
	}
	if similar, present := FindBestMatchForNameWithContext(session, candidates, value, None[uint](), context).Get(); present {
		return Some(SimilarSymbol{similar, false})
	}
	return None[SimilarSymbol]()
}
