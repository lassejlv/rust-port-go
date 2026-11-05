// SPDX-License-Identifier: MIT OR Apache-2.0
// File registration/import/lookup portions of pinned source_map.rs.
package rustcspan

import (
	"strconv"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func sourceMapDropFileOnExit(file *SourceFile, transferred *bool, context rustfmt.Context) {
	payload := recover()
	if payload != nil && context.ProcessingPanic {
		context.ResumeUnwind(payload)
	}
	if !*transferred {
		if payload != nil {
			unwind := context
			unwind.Unwinding = true
			spanCleanupDuringUnwindWithContext(unwind, func() { file.CloseWithContext(unwind) })
		} else {
			file.CloseWithContext(context)
		}
	}
	if payload != nil {
		context.ResumeUnwind(payload)
	}
}
func sourceMapPushFile(out *[]Shared[SourceFile], file Shared[SourceFile], context rustfmt.Context) {
	transferred := false
	defer func() {
		payload := recover()
		if payload != nil && context.ProcessingPanic {
			context.ResumeUnwind(payload)
		}
		if !transferred {
			if payload != nil {
				unwind := context
				unwind.Unwinding = true
				spanCleanupDuringUnwindWithContext(unwind, func() { file.CloseWithContext(unwind) })
			} else {
				file.CloseWithContext(context)
			}
		}
		if payload != nil {
			context.ResumeUnwind(payload)
		}
	}()
	analysisPush(out, file, uint(strconv.IntSize/8), context)
	transferred = true
}
func (sm *SourceMap) registerSourceFile(id StableSourceFileId, file SourceFile, context rustfmt.Context) (result Shared[SourceFile], failure error) {
	transferred := false
	defer sourceMapDropFileOnExit(&file, &transferred, context)
	sm.filesMu.Lock()
	defer sm.filesMu.Unlock()
	start := BytePos(0)
	if n := len(sm.files.sourceFiles); n > 0 {
		last := sm.files.sourceFiles[n-1].Borrow().EndPositionWithContext(context, sm.config)
		if uint32(last) == ^uint32(0) {
			return Shared[SourceFile]{}, OffsetOverflowError{}
		}
		start = BytePos(uint32(last) + 1)
	}
	file.StartPos = start
	result = NewShared(file, func(file *SourceFile, context rustfmt.Context) { file.CloseWithContext(context) })
	transferred = true
	returned := false
	defer func() {
		payload := recover()
		if payload != nil && context.ProcessingPanic {
			context.ResumeUnwind(payload)
		}
		if !returned {
			if payload != nil {
				unwind := context
				unwind.Unwinding = true
				spanCleanupDuringUnwindWithContext(unwind, func() { result.CloseWithContext(unwind) })
			} else {
				result.CloseWithContext(context)
			}
		}
		if payload != nil {
			context.ResumeUnwind(payload)
		}
	}()
	sourceMapPushFile(&sm.files.sourceFiles, result.CloneWithContext(context), context)
	old, present := sm.files.stableIdToSourceFile[id]
	sm.files.stableIdToSourceFile[id] = result.CloneWithContext(context)
	if present {
		old.CloseWithContext(context)
	}
	returned = true
	return result, nil
}
func (sm *SourceMap) tryNewSourceFile(filename FileName, src string, context rustfmt.Context) (Shared[SourceFile], error) {
	id := StableSourceFileIdFromFilenameInCurrentCrate(&filename)
	if file, ok := sm.SourceFileByStableIdWithContext(id, context).Get(); ok {
		return file, nil
	}
	file, failure := NewSourceFileWithContext(filename, src, sm.hashKind, sm.checksumHashKind, context, sm.config)
	if failure != nil {
		return Shared[SourceFile]{}, failure
	}
	transferred := false
	defer sourceMapDropFileOnExit(&file, &transferred, context)
	sourceMapAssertStableId(&file.StableId, &id, sm.config, context)
	transferred = true
	return sm.registerSourceFile(id, file, context)
}
func (sm *SourceMap) NewSourceFile(filename FileName, src string) Shared[SourceFile] {
	return sm.NewSourceFileWithContext(filename, src, rustfmt.Context{})
}
func (sm *SourceMap) NewSourceFileWithContext(filename FileName, src string, context rustfmt.Context) Shared[SourceFile] {
	file, failure := sm.tryNewSourceFile(filename, src, context)
	if failure == nil {
		return file
	}
	// This is the original fatal diagnostic/marker path, not an ordinary panic.
	sourceMapPrintFatal(context, sm.stderr)
	(FatalError{}).RaiseWithContext(context)
	return Shared[SourceFile]{}
}
func (sm *SourceMap) NewImportedSourceFile(filename FileName, srcHash SourceFileHash, checksumHash Option[SourceFileHash], stableId StableSourceFileId, normalizedSourceLen, unnormalizedSourceLen uint32, cnum CrateNum, fileLocalLines *FreezeLock[SourceFileLines], multibyteChars []MultiByteChar, normalizedPos []NormalizedPos, metadataIndex uint32) Shared[SourceFile] {
	return sm.NewImportedSourceFileWithContext(filename, srcHash, checksumHash, stableId, normalizedSourceLen, unnormalizedSourceLen, cnum, fileLocalLines, multibyteChars, normalizedPos, metadataIndex, rustfmt.Context{})
}
func (sm *SourceMap) NewImportedSourceFileWithContext(filename FileName, srcHash SourceFileHash, checksumHash Option[SourceFileHash], stableId StableSourceFileId, normalizedSourceLen, unnormalizedSourceLen uint32, cnum CrateNum, fileLocalLines *FreezeLock[SourceFileLines], multibyteChars []MultiByteChar, normalizedPos []NormalizedPos, metadataIndex uint32, context rustfmt.Context) Shared[SourceFile] {
	file := SourceFile{Name: filename, Src: None[SharedString](), SrcHash: srcHash, ChecksumHash: checksumHash, ExternalSrc: NewFreezeLock(ForeignExternalSource(AbsentOkExternalSourceKind(), metadataIndex)), StartPos: BytePos(0), NormalizedSourceLen: RelativeBytePosFromU32(normalizedSourceLen), UnnormalizedSourceLen: unnormalizedSourceLen, Lines: fileLocalLines, MultibyteChars: multibyteChars, NormalizedPos: normalizedPos, StableId: stableId, Cnum: cnum}
	result, failure := sm.registerSourceFile(stableId, file, context)
	if failure != nil {
		sourceMapPanic(context, "rustc_span::source_map::new_imported_source_file.expect_offset", rustfmt.PanicOwnedString, "not enough address space for imported source file: OffsetOverflowError")
	}
	return result
}
func (sm *SourceMap) LookupSourceFile(pos BytePos) Shared[SourceFile] {
	return sm.LookupSourceFileWithContext(pos, rustfmt.Context{})
}
func (sm *SourceMap) LookupSourceFileWithContext(pos BytePos, context rustfmt.Context) Shared[SourceFile] {
	index := sm.LookupSourceFileIdxWithContext(pos, context)
	guard := sm.FilesWithContext(context)
	defer guard.Close()
	return guard.CloneAtWithContext(index, context)
}
func (sm *SourceMap) GetSourceFile(filename *FileName) Option[Shared[SourceFile]] {
	return sm.GetSourceFileWithContext(filename, rustfmt.Context{})
}
func (sm *SourceMap) GetSourceFileWithContext(filename *FileName, context rustfmt.Context) Option[Shared[SourceFile]] {
	guard := sm.FilesWithContext(context)
	defer guard.Close()
	for index := uint(0); index < guard.Len(); index++ {
		if filename.Equal(guard.BorrowWithContext(index, context).BorrowWithContext(context).Name) {
			return Some(guard.CloneAtWithContext(index, context))
		}
	}
	return None[Shared[SourceFile]]()
}
func (sm *SourceMap) LookupByteOffset(pos BytePos) SourceFileAndBytePos {
	return sm.LookupByteOffsetWithContext(pos, rustfmt.Context{})
}
func (sm *SourceMap) LookupByteOffsetWithContext(pos BytePos, context rustfmt.Context) SourceFileAndBytePos {
	file := sm.LookupSourceFileWithContext(pos, context)
	moved := false
	defer sourceMapCloseSharedUnlessMoved(file, &moved, context)
	offset := pos.SubWithContext(file.BorrowWithContext(context).StartPos, sm.config, context)
	moved = true
	return SourceFileAndBytePos{SF: file, Pos: offset}
}
func (sm *SourceMap) CountLines() uint { return sm.CountLinesWithContext(rustfmt.Context{}) }
func (sm *SourceMap) CountLinesWithContext(context rustfmt.Context) uint {
	guard := sm.FilesWithContext(context)
	defer guard.Close()
	count := uint(0)
	for index := uint(0); index < guard.Len(); index++ {
		n := uint(len(guard.BorrowWithContext(index, context).BorrowWithContext(context).LinesWithContext(context, sm.config)))
		if sm.config.OverflowChecks && n > ^uint(0)-count {
			sourceMapPanic(context, "rustc_span::source_map::count_lines.add", rustfmt.PanicStaticStr, "attempt to add with overflow")
		}
		count += n
	}
	return count
}
func (sm *SourceMap) LoadFile(path RustPath) (Shared[SourceFile], error) {
	return sm.LoadFileWithContext(path, rustfmt.Context{})
}
func (sm *SourceMap) LoadFileWithContext(path RustPath, context rustfmt.Context) (Shared[SourceFile], error) {
	src, failure := sm.fileLoader.readFile(context, path)
	if failure != nil {
		return Shared[SourceFile]{}, failure
	}
	name := RealFileNameFileName(sm.pathMapping.ToRealFilenameWithContext(&sm.workingDir, path, context))
	return sm.NewSourceFileWithContext(name, src, context), nil
}
func (sm *SourceMap) LoadBinaryFile(session *SessionGlobals, path RustPath) (Shared[[]byte], Span, error) {
	return sm.LoadBinaryFileWithContext(session, path, rustfmt.Context{})
}
func (sm *SourceMap) LoadBinaryFileWithContext(session *SessionGlobals, path RustPath, context rustfmt.Context) (bytes Shared[[]byte], span Span, failure error) {
	bytes, failure = sm.fileLoader.readBinaryFile(context, path)
	if failure != nil {
		return Shared[[]byte]{}, Span{}, failure
	}
	returned := false
	defer func() {
		payload := recover()
		if payload != nil && context.ProcessingPanic {
			context.ResumeUnwind(payload)
		}
		if !returned {
			if payload != nil {
				unwind := context
				unwind.Unwinding = true
				spanCleanupDuringUnwindWithContext(unwind, func() { bytes.CloseWithContext(unwind) })
			} else {
				bytes.CloseWithContext(context)
			}
		}
		if payload != nil {
			context.ResumeUnwind(payload)
		}
	}()
	text := ""
	if utf8.Valid(*bytes.Borrow()) {
		text = string(*bytes.Borrow())
	}
	name := RealFileNameFileName(sm.pathMapping.ToRealFilenameWithContext(&sm.workingDir, path, context))
	file := sm.NewSourceFileWithContext(name, text, context)
	defer sourceMapCloseSharedOnExit(file, context)
	lo := file.Borrow().StartPos
	hi := BytePos(sourceMapU32Add(uint32(lo), uint32(file.Borrow().NormalizedSourceLen), sm.config, context, "rustc_span::source_map::load_binary_file.add"))
	span = newSpanWithContext(session, lo, hi, RootSyntaxContext(), None[LocalDefId](), sm.config, context)
	returned = true
	return bytes, span, nil
}
