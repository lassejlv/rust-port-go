// SPDX-License-Identifier: MIT OR Apache-2.0
// FilePathMapping block of pinned source_map.rs, using actual Rust path rules.
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
)

type CowPath struct {
	Path  RustPath
	Owned bool
}

func BorrowedCowPath(path RustPath) CowPath { return CowPath{Path: path} }
func OwnedCowPath(path RustPath) CowPath    { return CowPath{Path: path, Owned: true} }
func (path CowPath) IsOwned() bool          { return path.Owned }
func (path CowPath) IsBorrowed() bool       { return !path.Owned }
func (path CowPath) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return path.Path.WriteDebugFormatter(f)
}

type RustPathMappingEntry struct{ From, To RustPath }
type FilePathMapping struct {
	mapping                 []RustPathMappingEntry
	filenameRemappingScopes RemapPathScopeComponents
}

// NewFilePathMapping consumes the entries and their owned PathBuf backings,
// matching the source Vec. Use NewRustPath/Clone/owned OS constructors for those
// buffers; a borrowed Path view must be cloned before ownership transfer. The
// caller must not mutate or independently reuse the consumed vector/buffers.
func NewFilePathMapping(entries []RustPathMappingEntry, scopes RemapPathScopeComponents) FilePathMapping {
	return FilePathMapping{mapping: entries, filenameRemappingScopes: scopes}
}
func EmptyFilePathMapping() FilePathMapping {
	return NewFilePathMapping([]RustPathMappingEntry{}, EmptyRemapPathScopeComponents())
}
func (mapping FilePathMapping) Clone() FilePathMapping {
	entries := make([]RustPathMappingEntry, len(mapping.mapping))
	for i := range mapping.mapping {
		entries[i] = RustPathMappingEntry{From: mapping.mapping[i].From.Clone(), To: mapping.mapping[i].To.Clone()}
	}
	return FilePathMapping{mapping: entries, filenameRemappingScopes: mapping.filenameRemappingScopes}
}

// FilePathMappingToRealFilenameOwned preserves a custom consumed
// Into<Cow<Path>> implementation, including its receiver destruction before
// any remapping or tracing in the method body. A borrowed result must borrow
// external storage for the declared call lifetime, never the consumed receiver.
func FilePathMappingToRealFilenameOwned[E any](mapping *FilePathMapping, workingDirectory *RealFileName, input *SpanOwned[E], into func(*E, rustfmt.Context) CowPath, context rustfmt.Context) RealFileName {
	path := sourceMapIntoCowPath(input, into, context)
	return mapping.ToRealFilenameWithContext(workingDirectory, path.Path, context)
}
func FilePathMappingMapPrefixOwned[E any](mapping *FilePathMapping, input *SpanOwned[E], into func(*E, rustfmt.Context) CowPath, context rustfmt.Context) (CowPath, bool) {
	path := sourceMapIntoCowPath(input, into, context)
	return mapping.MapPrefixCowWithContext(path, context)
}
func sourceMapIntoCowPath[E any](input *SpanOwned[E], into func(*E, rustfmt.Context) CowPath, context rustfmt.Context) (path CowPath) {
	value := input.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, value.DropWithContext)
	spanRunContext(context, func() { path = into(value.Borrow(), context) })
	return
}
func (mapping *FilePathMapping) CloneFrom(source *FilePathMapping) { *mapping = source.Clone() }
func (mapping *FilePathMapping) MapPrefix(path RustPath) (CowPath, bool) {
	return mapping.MapPrefixCow(BorrowedCowPath(path))
}
func (mapping *FilePathMapping) MapPrefixCow(path CowPath) (CowPath, bool) {
	return mapping.MapPrefixCowWithContext(path, rustfmt.Context{})
}
func (mapping *FilePathMapping) MapPrefixCowWithContext(path CowPath, context rustfmt.Context) (CowPath, bool) {
	if path.Path.IsEmpty() {
		return path, false
	}
	traceSpan, traceScope := sourceMapTraceSpan(context, sourceMapTraceRemap, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "path", Value: rusttracing.DebugField(path.WriteDebugFormatter)}}
	})
	defer traceSpan.DeferCloseWithScope(traceScope, context)
	traceEntered := traceSpan.EnterWithScope(traceScope, context)
	defer traceEntered.DeferCloseWithScope(traceScope, context)
	finish := func(path CowPath, remapped bool) (CowPath, bool) {
		sourceMapTraceEvent(context, sourceMapTraceRemapReturn, func() []rusttracing.FieldValue {
			return []rusttracing.FieldValue{{Name: "return", Value: rusttracing.DebugField(func(f *rustfmt.Formatter) error {
				pretty := f.Options.Alternate
				builder := rustfmt.BeginTuple(f.Writer, "", pretty)
				builder.Field(f.Value(pretty, path.WriteDebugFormatter))
				builder.Field(f.Value(pretty, func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugBool(remapped, f) }))
				return builder.Finish()
			})}}
		})
		return path, remapped
	}
	for i := len(mapping.mapping) - 1; i >= 0; i-- {
		entry := &mapping.mapping[i]
		sourceMapTraceMessage(context, sourceMapTraceRemapTry, func(f *rustfmt.Formatter) error {
			if err := f.WriteString("Trying to apply "); err != nil {
				return err
			}
			if err := entry.From.WriteDebugFormatter(f); err != nil {
				return err
			}
			if err := f.WriteString(" => "); err != nil {
				return err
			}
			return entry.To.WriteDebugFormatter(f)
		})
		if rest, ok := path.Path.StripPrefix(entry.From); ok {
			var remapped CowPath
			if rest.IsEmpty() {
				remapped = BorrowedCowPath(entry.To)
			} else {
				remapped = OwnedCowPath(entry.To.Join(rest))
			}
			sourceMapTraceLiteral(context, sourceMapTraceRemapMatch, "Match - remapped")
			return finish(remapped, true)
		}
		sourceMapTraceMessage(context, sourceMapTraceRemapMiss, func(f *rustfmt.Formatter) error {
			if err := f.WriteString("No match - prefix "); err != nil {
				return err
			}
			if err := entry.From.WriteDebugFormatter(f); err != nil {
				return err
			}
			return f.WriteString(" does not match")
		})
	}
	sourceMapTraceLiteral(context, sourceMapTraceRemapNone, "not remapped")
	return finish(path, false)
}
func (mapping *FilePathMapping) ToRealFilename(workingDirectory *RealFileName, localPath RustPath) RealFileName {
	return mapping.ToRealFilenameWithContext(workingDirectory, localPath, rustfmt.Context{})
}
func (mapping *FilePathMapping) ToRealFilenameWithContext(workingDirectory *RealFileName, localPath RustPath, context rustfmt.Context) RealFileName {
	remapped, wasRemapped := mapping.MapPrefixCowWithContext(BorrowedCowPath(localPath), context)
	sourceMapTraceEvent(context, sourceMapTraceRealName, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{
			{Name: "local_path", Value: rusttracing.DebugField(localPath.WriteDebugFormatter)},
			{Name: "remapped_path", Value: rusttracing.DebugField(remapped.WriteDebugFormatter)},
			{Name: "was_remapped", Value: rusttracing.DebugField(func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugBool(wasRemapped, f) })},
			{Name: "self.filename_remapping_scopes", Value: rusttracing.DebugField(mapping.filenameRemappingScopes.WriteDebugFormatter)},
		}
	})
	workingLocal, ok := workingDirectory.LocalPath().Get()
	if !ok {
		sourceMapPanic(context, SourceMapPanicWorkingDirectory, rustfmt.PanicOwnedString, "working directory should be local")
	}
	local := innerRealFileName{name: localPath.Clone(), workingDirectory: workingLocal.Clone()}
	if localPath.IsAbsolute() {
		local.embeddableName = localPath.Clone()
	} else {
		local.embeddableName = workingLocal.Join(localPath)
	}
	var embeddable RustPath
	if remapped.Path.IsAbsolute() || wasRemapped {
		wasRemapped = wasRemapped || workingDirectory.WasRemapped()
		embeddable = remapped.Path.Clone()
	} else {
		absolute, absoluteWasRemapped := mapping.MapPrefixCowWithContext(OwnedCowPath(workingDirectory.maybeRemapped.name.Join(remapped.Path)), context)
		wasRemapped = absoluteWasRemapped || workingDirectory.WasRemapped()
		embeddable = absolute.Path.Clone()
	}
	scopes := EmptyRemapPathScopeComponents()
	if wasRemapped {
		scopes = mapping.filenameRemappingScopes
	}
	return RealFileName{local: Some(local), maybeRemapped: innerRealFileName{name: remapped.Path.Clone(), workingDirectory: workingDirectory.maybeRemapped.name.Clone(), embeddableName: embeddable}, scopes: scopes}
}
func (mapping *FilePathMapping) ReverseMapPrefixHeuristically(path RustPath) Option[RustPath] {
	return mapping.ReverseMapPrefixHeuristicallyWithContext(path, rustfmt.Context{})
}
func (mapping *FilePathMapping) ReverseMapPrefixHeuristicallyWithContext(path RustPath, context rustfmt.Context) Option[RustPath] {
	traceSpan, traceScope := sourceMapTraceSpan(context, sourceMapTraceReverse, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "path", Value: rusttracing.DebugField(path.WriteDebugFormatter)}}
	})
	defer traceSpan.DeferCloseWithScope(traceScope, context)
	traceEntered := traceSpan.EnterWithScope(traceScope, context)
	defer traceEntered.DeferCloseWithScope(traceScope, context)
	finish := func(result Option[RustPath]) Option[RustPath] {
		sourceMapTraceEvent(context, sourceMapTraceReverseReturn, func() []rusttracing.FieldValue {
			return []rusttracing.FieldValue{{Name: "return", Value: rusttracing.DebugField(func(f *rustfmt.Formatter) error {
				return spanDebugOption(result, f, func(path RustPath, f *rustfmt.Formatter) error { return path.WriteDebugFormatter(f) })
			})}}
		})
		return result
	}
	found := None[RustPath]()
	for i := range mapping.mapping {
		entry := &mapping.mapping[i]
		hasNormal := false
		components := entry.To.Components()
		for {
			component, ok := components.Next()
			if !ok {
				break
			}
			if component.Kind == RustPathNormal && !component.Path.IsEmpty() {
				hasNormal = true
				break
			}
		}
		if !hasNormal {
			continue
		}
		rest, ok := path.StripPrefix(entry.To)
		if !ok {
			continue
		}
		if found.IsSome() {
			return finish(None[RustPath]())
		}
		found = Some(entry.From.Join(rest))
	}
	return finish(found)
}
func (mapping *FilePathMapping) WriteDebugFormatter(f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	outer := rustfmt.BeginStruct(f.Writer, "FilePathMapping", pretty)
	outer.Field("mapping", f.Value(pretty, func(f *rustfmt.Formatter) error {
		list := rustfmt.BeginList(f.Writer, f.Options.Alternate)
		for i := range mapping.mapping {
			entry := &mapping.mapping[i]
			list.Entry(f.Value(f.Options.Alternate, func(f *rustfmt.Formatter) error {
				tuple := rustfmt.BeginTuple(f.Writer, "", f.Options.Alternate)
				tuple.Field(f.Value(f.Options.Alternate, entry.From.WriteDebugFormatter))
				tuple.Field(f.Value(f.Options.Alternate, entry.To.WriteDebugFormatter))
				return tuple.Finish()
			}))
		}
		return list.Finish()
	}))
	outer.Field("filename_remapping_scopes", f.Value(pretty, mapping.filenameRemappingScopes.WriteDebugFormatter))
	return outer.Finish()
}
func (mapping *FilePathMapping) RustDebug() string {
	return rustfmt.String(mapping.WriteDebugFormatter)
}
