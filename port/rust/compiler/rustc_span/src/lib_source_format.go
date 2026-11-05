// SPDX-License-Identifier: MIT OR Apache-2.0
// Source-derived Debug and manual Display bodies from pinned rustc_span/lib.rs.
// New whole-span formatting uses the pinned compiler's Unicode18 std profile.
package rustcspan

import (
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func spanDebugOption[T any](value Option[T], f *rustfmt.Formatter, debug func(T, *rustfmt.Formatter) error) error {
	v, ok := value.Get()
	if !ok {
		return f.WriteString("None")
	}
	pretty := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "Some", pretty)
	b.Field(f.Value(pretty, func(child *rustfmt.Formatter) error { return debug(v, child) }))
	return b.Finish()
}
func (value innerRealFileName) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "InnerRealFileName", p)
	b.Field("name", f.Value(p, value.name.WriteDebugFormatter))
	b.Field("working_directory", f.Value(p, value.workingDirectory.WriteDebugFormatter))
	b.Field("embeddable_name", f.Value(p, value.embeddableName.WriteDebugFormatter))
	return b.Finish()
}
func (value RealFileName) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "RealFileName", p)
	b.Field("local", f.Value(p, func(f *rustfmt.Formatter) error {
		return spanDebugOption(value.local, f, func(v innerRealFileName, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) })
	}))
	b.Field("maybe_remapped", f.Value(p, value.maybeRemapped.WriteDebugFormatter))
	b.Field("scopes", f.Value(p, value.scopes.WriteDebugFormatter))
	return b.Finish()
}
func (value FileName) WriteDebugFormatter(f *rustfmt.Formatter) error {
	names := [...]string{"Real", "CfgSpec", "Anon", "MacroExpansion", "ProcMacroSourceCode", "CliCrateAttr", "Custom", "DocTest", "InlineAsm"}
	p := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, names[value.kind], p)
	switch value.kind {
	case fileNameReal:
		b.Field(f.Value(p, value.real.WriteDebugFormatter))
	case fileNameCustom:
		b.Field(f.Value(p, func(f *rustfmt.Formatter) error { return FormatDebugStringPinnedUnicode18(value.custom, f) }))
	case fileNameDocTest:
		b.Field(f.Value(p, value.real.WriteDebugFormatter))
		b.Field(f.Value(p, func(f *rustfmt.Formatter) error {
			return rustfmt.FormatDebugInt(int64(value.line), uint(spanUintBits()), f)
		}))
	default:
		b.Field(f.Value(p, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(value.hash.AsU64(), 64, f) }))
	}
	return b.Finish()
}
func (value SourceFileHashAlgorithm) WriteDebugFormatter(f *rustfmt.Formatter) error {
	names := [...]string{"Md5", "Sha1", "Sha256", "Blake3"}
	return f.WriteString(names[value])
}
func (value SourceFileHash) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "SourceFileHash", p)
	b.Field("kind", f.Value(p, value.Kind.WriteDebugFormatter))
	b.Field("value", f.Value(p, func(f *rustfmt.Formatter) error {
		list := rustfmt.BeginList(f.Writer, f.Options.Alternate)
		for _, byteValue := range value.value {
			list.Entry(f.Value(f.Options.Alternate, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(uint64(byteValue), 8, f) }))
		}
		return list.Finish()
	}))
	return b.Finish()
}
func (value SourceFileHash) WriteDisplayFormatter(f *rustfmt.Formatter) error {
	if err := value.Kind.WriteDisplayFormatter(&rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}); err != nil {
		return err
	}
	if err := f.WriteString("="); err != nil {
		return err
	}
	for _, b := range value.value[:value.Kind.hashLen()] {
		options := rustfmt.DefaultOptions()
		options.DebugLowerHex = true
		options.ZeroPad = true
		options.HasWidth = true
		options.Width = 2
		child := rustfmt.Formatter{Writer: f.Writer, Options: options, Context: f.Context}
		if err := rustfmt.FormatDebugUint(uint64(b), 8, &child); err != nil {
			return err
		}
	}
	return nil
}
func (value SourceFileHash) String() string { return rustfmt.String(value.WriteDisplayFormatter) }
func (value ExternalSourceKind) WriteDebugFormatter(f *rustfmt.Formatter) error {
	switch value.tag {
	case externalAbsentOk:
		return f.WriteString("AbsentOk")
	case externalAbsentErr:
		return f.WriteString("AbsentErr")
	case externalPresent:
		p := f.Options.Alternate
		b := rustfmt.BeginTuple(f.Writer, "Present", p)
		b.Field(f.Value(p, func(f *rustfmt.Formatter) error { return FormatDebugStringPinnedUnicode18(value.src.String(), f) }))
		return b.Finish()
	}
	f.Context.Panic("rustcspan: invalid ExternalSourceKind")
	return nil
}
func (value ExternalSource) WriteDebugFormatter(f *rustfmt.Formatter) error {
	if !value.foreign {
		return f.WriteString("Unneeded")
	}
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "Foreign", p)
	b.Field("kind", f.Value(p, value.kind.WriteDebugFormatter))
	b.Field("metadata_index", f.Value(p, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(uint64(value.metadataIndex), 32, f) }))
	return b.Finish()
}
func (OffsetOverflowError) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return f.WriteString("OffsetOverflowError")
}
func (value *SourceFile) WriteDebugFormatter(f *rustfmt.Formatter) error {
	if err := f.WriteString("SourceFile("); err != nil {
		return err
	}
	child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}
	if err := value.Name.WriteDebugFormatter(child); err != nil {
		return err
	}
	return f.WriteString(")")
}
func (value MultiByteChar) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "MultiByteChar", p)
	b.Field("pos", f.Value(p, value.Pos.WriteDebugFormatter))
	b.Field("bytes", f.Value(p, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(uint64(value.Bytes), 8, f) }))
	return b.Finish()
}
func (value NormalizedPos) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "NormalizedPos", p)
	b.Field("pos", f.Value(p, value.Pos.WriteDebugFormatter))
	b.Field("diff", f.Value(p, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(uint64(value.Diff), 32, f) }))
	return b.Finish()
}

// The pinned hash wrapper delegates Debug to u128. This adapter preserves its
// independent numeric formatter and copies back only the mutable std state.
func spanDebugHash128(value rustchashes.Hash128, f *rustfmt.Formatter) (err error) {
	radix := rustchashes.DebugDecimal
	if f.Options.DebugLowerHex {
		radix = rustchashes.DebugLowerHex
	} else if f.Options.DebugUpperHex {
		radix = rustchashes.DebugUpperHex
	}
	width := uint(0)
	if f.Options.HasWidth {
		width = f.Options.Width
	}
	numeric := rustchashes.NumericFormatter{Writer: f.Writer, Options: rustchashes.FormatOptions{Width: width, Alignment: rustchashes.Alignment(f.Options.Align), Fill: f.Options.Fill, FillSet: true, Plus: f.Options.SignPlus, Alternate: f.Options.Alternate, ZeroPad: f.Options.ZeroPad, DebugRadix: radix}}
	defer func() {
		f.Options.Fill = numeric.Options.Fill
		f.Options.Align = rustfmt.Alignment(numeric.Options.Alignment)
	}()
	return value.WriteDebug(&numeric)
}
func (value StableSourceFileId) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "StableSourceFileId", p)
	b.Field(f.Value(p, func(f *rustfmt.Formatter) error { return spanDebugHash128(value.hash, f) }))
	return b.Finish()
}
func (value Loc) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "Loc", p)
	b.Field("file", f.Value(p, value.File.BorrowWithContext(f.Context).WriteDebugFormatter))
	b.Field("line", f.Value(p, func(f *rustfmt.Formatter) error {
		return rustfmt.FormatDebugUint(uint64(value.Line), uint(spanUintBits()), f)
	}))
	b.Field("col", f.Value(p, value.Col.WriteDebugFormatter))
	b.Field("col_display", f.Value(p, func(f *rustfmt.Formatter) error {
		return rustfmt.FormatDebugUint(uint64(value.ColDisplay), uint(spanUintBits()), f)
	}))
	return b.Finish()
}
func (value SourceFileAndLine) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "SourceFileAndLine", p)
	b.Field("sf", f.Value(p, value.SF.BorrowWithContext(f.Context).WriteDebugFormatter))
	b.Field("line", f.Value(p, func(f *rustfmt.Formatter) error {
		return rustfmt.FormatDebugUint(uint64(value.Line), uint(spanUintBits()), f)
	}))
	return b.Finish()
}
func (value SourceFileAndBytePos) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "SourceFileAndBytePos", p)
	b.Field("sf", f.Value(p, value.SF.BorrowWithContext(f.Context).WriteDebugFormatter))
	b.Field("pos", f.Value(p, value.Pos.WriteDebugFormatter))
	return b.Finish()
}
func (value LineInfo) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "LineInfo", p)
	b.Field("line_index", f.Value(p, func(f *rustfmt.Formatter) error {
		return rustfmt.FormatDebugUint(uint64(value.LineIndex), uint(spanUintBits()), f)
	}))
	b.Field("start_col", f.Value(p, value.StartCol.WriteDebugFormatter))
	b.Field("end_col", f.Value(p, value.EndCol.WriteDebugFormatter))
	return b.Finish()
}
