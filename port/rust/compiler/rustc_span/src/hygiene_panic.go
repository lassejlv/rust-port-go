// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"strconv"
	"strings"
)

// HygienePanicSite identifies one source operation independently of the actual
// selected build's filename. Callers provide the corresponding location map.
func HygienePanicSite(line, column uint32) string {
	return "source:rustc_span/src/hygiene.rs:" + strconv.FormatUint(uint64(line), 10) + ":" + strconv.FormatUint(uint64(column), 10)
}

type hygieneCodePoint struct{ line, column uint32 }

func hygienePoint(line, column uint32) hygieneCodePoint { return hygieneCodePoint{line, column} }
func hygieneStaticPanic(ctx rustfmt.Context, line, column uint32, text string) {
	ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: text, Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation(HygienePanicSite(line, column))})
}
func hygieneOwnedPanic(ctx rustfmt.Context, line, column uint32, text string) {
	ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: text, Kind: rustfmt.PanicOwnedString, CanUnwind: true, Location: ctx.SourcePanicLocation(HygienePanicSite(line, column))})
}
func hygieneBoundsPanic(ctx rustfmt.Context, point hygieneCodePoint, length, index uint) {
	hygieneFormattedPanicAt(ctx, HygienePanicSite(point.line, point.column), func(f *rustfmt.Formatter) error {
		if e := f.WriteString("index out of bounds: the len is "); e != nil {
			return e
		}
		if e := rustfmt.FormatDebugUint(uint64(length), 64, f); e != nil {
			return e
		}
		if e := f.WriteString(" but the index is "); e != nil {
			return e
		}
		return rustfmt.FormatDebugUint(uint64(index), 64, f)
	})
}
func hygieneNotEqualAssertion(ctx rustfmt.Context, site string, left, right func(*rustfmt.Formatter) error) {
	hygieneFormattedPanicAt(ctx, site, func(f *rustfmt.Formatter) error {
		if e := f.WriteString("assertion `left != right` failed\n  left: "); e != nil {
			return e
		}
		if e := left(f); e != nil {
			return e
		}
		if e := f.WriteString("\n right: "); e != nil {
			return e
		}
		return right(f)
	})
}
func hygieneEqualAssertion(ctx rustfmt.Context, site string, left, right func(*rustfmt.Formatter) error) {
	hygieneFormattedPanicAt(ctx, site, func(f *rustfmt.Formatter) error {
		if e := f.WriteString("assertion `left == right` failed\n  left: "); e != nil {
			return e
		}
		if e := left(f); e != nil {
			return e
		}
		if e := f.WriteString("\n right: "); e != nil {
			return e
		}
		return right(f)
	})
}

// Assertion arguments are formatted while Rust is processing its new panic,
// before ordinary stack cleanup. A nested failure must abort at its own site;
// delayed recovery after argument/closure destruction would change the source.
func hygieneFormattedPanic(ctx rustfmt.Context, render func(*rustfmt.Formatter) error) {
	hygieneFormattedPanicAt(ctx, "rustc_span::hygiene::update_disambiguator.already_set", render)
}
func hygieneFormattedPanicAt(ctx rustfmt.Context, site string, render func(*rustfmt.Formatter) error) {
	ctx.PanicFormattedAt(rustfmt.SourcePanicInfo{
		Kind: rustfmt.PanicOwnedString, CanUnwind: true,
		Location: ctx.SourcePanicLocation(site),
	}, func(formatContext rustfmt.Context) any {
		var text strings.Builder
		f := &rustfmt.Formatter{Writer: &text, Options: rustfmt.DefaultOptions(), Context: formatContext}
		// The source formatted panic payload retains the text produced before
		// a formatting trait returns fmt::Error. An actual nested callback
		// panic still escapes through the processing-panic Context and aborts.
		_ = render(f)
		return text.String()
	})
}

func hygieneParentLocalAssertion(parent CrateNum, ctx rustfmt.Context, site string) {
	hygieneFormattedPanicAt(ctx, site, func(f *rustfmt.Formatter) error {
		if err := f.WriteString("assertion `left == right` failed\n  left: "); err != nil {
			return err
		}
		if err := parent.WriteDebugFormatter(f); err != nil {
			return err
		}
		if err := f.WriteString("\n right: "); err != nil {
			return err
		}
		return LocalCrate().WriteDebugFormatter(f)
	})
}
