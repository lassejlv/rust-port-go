// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped used-contract translation of selected SDK 1.101 str range indexing,
// char boundaries and slice_error_fail_rt. This grants no standard-file credit.
package ruststr

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Every raw string argument has Rust &str's valid-UTF8 precondition. Keeping
// that type invariant explicit preserves O(1) valid indexing. Checked provides
// the separate O(n) validation boundary for previously unchecked Go input.
type Str struct{ text string }

func Checked(text string) (Str, bool) {
	if !utf8.ValidString(text) {
		return Str{}, false
	}
	return Str{text}, true
}
func BorrowValid(text string) Str { return Str{text} }
func (value Str) String() string  { return value.text }
func (value Str) Slice(begin, end uint, ctx rustfmt.Context, site string) Str {
	return Str{Slice(value.text, begin, end, ctx, site)}
}

func IsCharBoundary(text string, index uint) bool {
	if index == 0 {
		return true
	}
	if index >= uint(len(text)) {
		return index == uint(len(text))
	}
	return utf8.RuneStart(text[index])
}
func FloorCharBoundary(text string, index uint) uint {
	if index >= uint(len(text)) {
		return uint(len(text))
	}
	if utf8.RuneStart(text[index]) {
		return index
	}
	if utf8.RuneStart(text[index-1]) {
		return index - 1
	}
	if utf8.RuneStart(text[index-2]) {
		return index - 2
	}
	return index - 3
}
func CeilCharBoundary(text string, index uint) uint {
	if index >= uint(len(text)) {
		return uint(len(text))
	}
	i := index
	for !utf8.RuneStart(text[i]) {
		i++
		if i >= uint(len(text)) {
			break
		}
	}
	return i
}
func validRange(text string, begin, end uint) bool {
	if begin > end || end > uint(len(text)) {
		return false
	}
	if begin == uint(len(text)) {
		return true
	}
	return (begin == 0 || utf8.RuneStart(text[begin])) && (end == uint(len(text)) || utf8.RuneStart(text[end]))
}
func Get(text string, begin, end uint) (string, bool) {
	if validRange(text, begin, end) {
		return text[begin:end], true
	}
	return "", false
}
func Slice(text string, begin, end uint, ctx rustfmt.Context, site string) string {
	if value, ok := Get(text, begin, end); ok {
		return value
	}
	failSlice(text, begin, end, ctx, site)
	return ""
}
func SliceFrom(text string, begin uint, ctx rustfmt.Context, site string) string {
	return Slice(text, begin, uint(len(text)), ctx, site)
}
func SliceTo(text string, end uint, ctx rustfmt.Context, site string) string {
	return Slice(text, 0, end, ctx, site)
}

// InclusiveRange is a consumed source RangeInclusive state witness. It does
// not implement or claim the rest of Rust's integer-range Iterator API.
type InclusiveRange struct {
	start, end uint
	exhausted  bool
}

func Inclusive(start, end uint) InclusiveRange { return InclusiveRange{start: start, end: end} }

// ExhaustedInclusive mirrors a singleton range after its one next() call.
// Source 1.101 advances start even at the last value; its private exhausted
// flag is set only if advancing that endpoint overflows. Index diagnostics
// observe this state even though both cases are semantically exhausted.
func ExhaustedInclusive(endpoint uint) InclusiveRange {
	return InclusiveRange{start: endpoint + 1, end: endpoint, exhausted: endpoint == ^uint(0)}
}
func (value InclusiveRange) Parts() (uint, uint, bool) {
	return value.start, value.end, value.exhausted
}
func GetInclusive(text string, value InclusiveRange) (string, bool) {
	if value.end >= uint(len(text)) {
		return "", false
	}
	end := value.end + 1
	start := value.start
	if value.exhausted {
		start = end
	}
	return Get(text, start, end)
}
func SliceInclusive(text string, value InclusiveRange, ctx rustfmt.Context, site string) string {
	start, end := value.start, value.end
	if end < uint(len(text)) {
		end++
		if value.exhausted {
			start = end
		}
		if start <= end && IsCharBoundary(text, start) && IsCharBoundary(text, end) {
			return text[start:end]
		}
	}
	failSlice(text, start, end, ctx, site)
	return ""
}
func SliceToInclusive(text string, end uint, ctx rustfmt.Context, site string) string {
	return SliceInclusive(text, Inclusive(0, end), ctx, site)
}

func failSlice(text string, begin, end uint, ctx rustfmt.Context, site string) {
	ctx.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: ctx.SourcePanicLocation(site), CanUnwind: true}, func(processing rustfmt.Context) any {
		length := uint(len(text))
		if begin > length {
			return fmt.Sprintf("start byte index %d is out of bounds for string of length %d", begin, length)
		}
		if end > length {
			return fmt.Sprintf("end byte index %d is out of bounds for string of length %d", end, length)
		}
		if begin > end {
			return fmt.Sprintf("byte range starts at %d but ends at %d", begin, end)
		}
		if !IsCharBoundary(text, begin) {
			return boundaryMessage(text, "start", begin, processing)
		}
		if !IsCharBoundary(text, end) {
			return boundaryMessage(text, "end", end, processing)
		}
		// Every checked caller reaches this final branch only for an inclusive
		// index whose original end equals len; the source debug_assert_eq holds.
		return fmt.Sprintf("end byte index %d is out of bounds for string of length %d", end, length)
	})
}
func boundaryMessage(text, edge string, index uint, ctx rustfmt.Context) string {
	floor, ceil := FloorCharBoundary(text, index), CeilCharBoundary(text, index)
	ch, _ := utf8.DecodeRuneInString(text[floor:ceil])
	var debug strings.Builder
	_ = rustfmt.FormatDebugCharUnicode18(ch, &rustfmt.Formatter{Writer: &debug, Options: rustfmt.DefaultOptions(), Context: ctx})
	return fmt.Sprintf("%s byte index %d is not a char boundary; it is inside %s (bytes %d..%d of string)", edge, index, debug.String(), floor, ceil)
}
