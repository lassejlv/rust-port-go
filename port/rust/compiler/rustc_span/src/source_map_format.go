// SPDX-License-Identifier: MIT OR Apache-2.0
// Manual SourceMap/SourceMapFiles Debug implementations from source_map.rs.
package rustcspan

import (
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func (files *sourceMapFiles) writeDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	builder := rustfmt.BeginList(f.Writer, pretty)
	for index := range files.sourceFiles {
		// Iterator::map constructs every temporary String even when the debug
		// builder remembers an earlier writer error. The strings' Debug sees
		// the caller's original options, not FileNameDisplay's Display flags.
		name := rustfmt.String(func(child *rustfmt.Formatter) error {
			child.Context = f.Context
			return files.sourceFiles[index].BorrowWithContext(f.Context).Name.PreferRemappedUnconditionally().WriteDisplayFormatter(child)
		})
		builder.Entry(f.Value(pretty, func(child *rustfmt.Formatter) error { return FormatDebugStringPinnedUnicode18(name, child) }))
	}
	return builder.Finish()
}
func (sm *SourceMap) writeFilesDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	outer := rustfmt.BeginTuple(f.Writer, "RwLock", pretty)
	outer.Field(f.Value(pretty, func(f *rustfmt.Formatter) error {
		pretty := f.Options.Alternate
		inner := rustfmt.BeginStruct(f.Writer, "RwLock", pretty)
		// parking_lot Debug uses try_read, even after the opening writer failed.
		if sm.filesMu.TryRLock() {
			func() {
				defer sm.filesMu.RUnlock()
				inner.Field("data", f.Value(pretty, sm.files.writeDebugFormatter))
			}()
		} else {
			inner.Field("data", f.Value(pretty, func(child *rustfmt.Formatter) error { return child.WriteString("<locked>") }))
		}
		return inner.Finish()
	}))
	return outer.Finish()
}
func (sm *SourceMap) WriteDebugFormatter(f *rustfmt.Formatter) error {
	defer spanDebugContextGuard(f.Context)
	pretty := f.Options.Alternate
	builder := rustfmt.BeginStruct(f.Writer, "SourceMap", pretty)
	builder.Field("files", f.Value(pretty, sm.writeFilesDebugFormatter))
	builder.Field("file_loader", f.Value(pretty, func(child *rustfmt.Formatter) error {
		if err := child.WriteString("<file_loader@"); err != nil {
			return err
		}
		if err := sourcePointerFormat(uintptr(unsafe.Pointer(&sm.fileLoader)), child); err != nil {
			return err
		}
		return child.WriteString(">")
	}))
	builder.Field("path_mapping", f.Value(pretty, sm.pathMapping.WriteDebugFormatter))
	builder.Field("working_dir", f.Value(pretty, sm.workingDir.WriteDebugFormatter))
	builder.Field("hash_kind", f.Value(pretty, sm.hashKind.WriteDebugFormatter))
	builder.Field("checksum_hash_kind", f.Value(pretty, func(child *rustfmt.Formatter) error {
		return spanDebugOption(sm.checksumHashKind, child, func(value SourceFileHashAlgorithm, child *rustfmt.Formatter) error {
			return value.WriteDebugFormatter(child)
		})
	}))
	return builder.Finish()
}
func (sm *SourceMap) RustDebug() string { return rustfmt.String(sm.WriteDebugFormatter) }
