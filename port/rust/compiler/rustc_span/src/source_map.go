// SPDX-License-Identifier: MIT OR Apache-2.0
// Port of compiler/rustc_span/src/source_map.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. Supporting standard-library
// ownership/path/locking adapters receive no independent source-file credit.
package rustcspan

import (
	"sync"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rusttracing"
)

// FileLoader is the source's shared-receiver filesystem abstraction. Returned
// strings must be valid UTF-8. Returned byte handles transfer an independent
// owned Arc equivalent. A loader object must honor Rust's Send/Sync and shared
// borrow rules; methods must not mutate ordinary shared fields without locking.
type FileLoader interface {
	FileExists(RustPath) bool
	ReadFile(RustPath) (string, error)
	ReadBinaryFile(RustPath) (Shared[[]byte], error)
	CurrentDirectory() (RustPath, error)
}
type ContextFileLoader interface {
	FileExistsWithContext(rustfmt.Context, RustPath) bool
	ReadFileWithContext(rustfmt.Context, RustPath) (string, error)
	ReadBinaryFileWithContext(rustfmt.Context, RustPath) (Shared[[]byte], error)
	CurrentDirectoryWithContext(rustfmt.Context) (RustPath, error)
}

// FileLoaderOps is the complete trait witness for an arbitrary (including
// non-comparable) boxed loader state. Every operation and the state's complete
// destructor borrow the same actual E cell. Callbacks obey Rust's shared
// receiver, Send and Sync laws. The plain FileLoader interface is also useful
// for pointer-receiver objects and trivial-drop/value-receiver conveniences.
type FileLoaderOps[E any] struct {
	FileExists       func(*E, RustPath, rustfmt.Context) bool
	ReadFile         func(*E, RustPath, rustfmt.Context) (string, error)
	ReadBinaryFile   func(*E, RustPath, rustfmt.Context) (Shared[[]byte], error)
	CurrentDirectory func(*E, rustfmt.Context) (RustPath, error)
}
type sourceMapStateLoader[E any] struct {
	state *SpanOwned[E]
	ops   FileLoaderOps[E]
}

func NewOwnedFileLoaderWithOps[E any](state *SpanOwned[E], ops FileLoaderOps[E]) *OwnedFileLoader {
	return NewOwnedFileLoaderWithOpsAndContext(state, ops, rustfmt.Context{})
}
func NewOwnedFileLoaderWithOpsAndContext[E any](state *SpanOwned[E], ops FileLoaderOps[E], context rustfmt.Context) *OwnedFileLoader {
	if ops.FileExists == nil || ops.ReadFile == nil || ops.ReadBinaryFile == nil || ops.CurrentDirectory == nil {
		context.Panic("rustc_span: FileLoader requires all four operations")
	}
	loader := &sourceMapStateLoader[E]{state: state.MoveWithContext(context), ops: ops}
	return NewOwnedFileLoader(loader, func(_ FileLoader, context rustfmt.Context) { loader.state.DropWithContext(context) })
}
func (loader *sourceMapStateLoader[E]) FileExists(path RustPath) bool {
	return loader.FileExistsWithContext(rustfmt.Context{}, path)
}
func (loader *sourceMapStateLoader[E]) FileExistsWithContext(context rustfmt.Context, path RustPath) bool {
	return loader.ops.FileExists(loader.state.Borrow(), path, context)
}
func (loader *sourceMapStateLoader[E]) ReadFile(path RustPath) (string, error) {
	return loader.ReadFileWithContext(rustfmt.Context{}, path)
}
func (loader *sourceMapStateLoader[E]) ReadFileWithContext(context rustfmt.Context, path RustPath) (string, error) {
	return loader.ops.ReadFile(loader.state.Borrow(), path, context)
}
func (loader *sourceMapStateLoader[E]) ReadBinaryFile(path RustPath) (Shared[[]byte], error) {
	return loader.ReadBinaryFileWithContext(rustfmt.Context{}, path)
}
func (loader *sourceMapStateLoader[E]) ReadBinaryFileWithContext(context rustfmt.Context, path RustPath) (Shared[[]byte], error) {
	return loader.ops.ReadBinaryFile(loader.state.Borrow(), path, context)
}
func (loader *sourceMapStateLoader[E]) CurrentDirectory() (RustPath, error) {
	return loader.CurrentDirectoryWithContext(rustfmt.Context{})
}
func (loader *sourceMapStateLoader[E]) CurrentDirectoryWithContext(context rustfmt.Context) (RustPath, error) {
	return loader.ops.CurrentDirectory(loader.state.Borrow(), context)
}

// OwnedFileLoader carries the consumed Box<dyn FileLoader> and complete
// destruction witness. Copies of a descriptor do not create another owner;
// constructors move it. Destructible environment fields belong in Loader.
type OwnedFileLoader struct{ value *SpanOwned[FileLoader] }

func NewOwnedFileLoader(loader FileLoader, drop func(FileLoader, rustfmt.Context)) *OwnedFileLoader {
	return &OwnedFileLoader{value: NewSpanOwned(loader, func(loader *FileLoader, context rustfmt.Context) {
		if drop != nil {
			drop(*loader, context)
		}
	})}
}
func (loader *OwnedFileLoader) Borrow() FileLoader { return *loader.value.Borrow() }
func (loader *OwnedFileLoader) move(context rustfmt.Context) *OwnedFileLoader {
	return &OwnedFileLoader{value: loader.value.MoveWithContext(context)}
}
func (loader *OwnedFileLoader) CloseWithContext(context rustfmt.Context) {
	loader.value.DropWithContext(context)
}
func (loader *OwnedFileLoader) Close() { loader.CloseWithContext(rustfmt.Context{}) }
func (loader *OwnedFileLoader) fileExists(context rustfmt.Context, path RustPath) (value bool) {
	spanRunContext(context, func() {
		object := loader.Borrow()
		if primary, ok := object.(ContextFileLoader); ok {
			value = primary.FileExistsWithContext(context, path)
		} else {
			value = object.FileExists(path)
		}
	})
	return
}
func (loader *OwnedFileLoader) readFile(context rustfmt.Context, path RustPath) (value string, failure error) {
	spanRunContext(context, func() {
		object := loader.Borrow()
		if primary, ok := object.(ContextFileLoader); ok {
			value, failure = primary.ReadFileWithContext(context, path)
		} else {
			value, failure = object.ReadFile(path)
		}
	})
	return
}
func (loader *OwnedFileLoader) readBinaryFile(context rustfmt.Context, path RustPath) (value Shared[[]byte], failure error) {
	spanRunContext(context, func() {
		object := loader.Borrow()
		if primary, ok := object.(ContextFileLoader); ok {
			value, failure = primary.ReadBinaryFileWithContext(context, path)
		} else {
			value, failure = object.ReadBinaryFile(path)
		}
	})
	return
}
func (loader *OwnedFileLoader) currentDirectory(context rustfmt.Context) (value RustPath, failure error) {
	spanRunContext(context, func() {
		object := loader.Borrow()
		if primary, ok := object.(ContextFileLoader); ok {
			value, failure = primary.CurrentDirectoryWithContext(context)
		} else {
			value, failure = object.CurrentDirectory()
		}
	})
	return
}

type SourceMapInputs struct {
	FileLoader       *OwnedFileLoader
	PathMapping      FilePathMapping
	HashKind         SourceFileHashAlgorithm
	ChecksumHashKind Option[SourceFileHashAlgorithm]
	// Borrowed ambient-runtime descriptor, not a field of the original Rust
	// inputs. Its operations receive the current caller's Context each time.
	Stderr *SourceMapStderr
}
type sourceMapFiles struct {
	sourceFiles          []Shared[SourceFile]
	stableIdToSourceFile map[StableSourceFileId]Shared[SourceFile]
}

// SourceMap owns a monotonically growing sequence of real source files.
// It must not be copied after construction: borrow its pointer or clone its
// Shared handle. All returned file handles obey Shared's explicit ownership.
type SourceMap struct {
	filesMu          sync.RWMutex
	files            sourceMapFiles
	fileLoader       *OwnedFileLoader
	pathMapping      FilePathMapping
	workingDir       RealFileName
	hashKind         SourceFileHashAlgorithm
	checksumHashKind Option[SourceFileHashAlgorithm]
	config           SpanEncodingConfig
	closed           bool
	stderr           *SourceMapStderr
}

func NewSourceMap(mapping FilePathMapping) *SourceMap {
	return NewSourceMapWithConfig(mapping, DefaultSpanEncodingConfig())
}
func NewSourceMapWithConfig(mapping FilePathMapping, config SpanEncodingConfig) *SourceMap {
	return NewSourceMapWithInputsAndConfig(SourceMapInputs{FileLoader: NewOwnedFileLoader(&RealFileLoader{}, nil), PathMapping: mapping, HashKind: SourceFileHashMd5}, config)
}
func NewSourceMapWithInputs(inputs SourceMapInputs) *SourceMap {
	return NewSourceMapWithInputsAndConfig(inputs, DefaultSpanEncodingConfig())
}
func NewSourceMapWithInputsAndConfig(inputs SourceMapInputs, config SpanEncodingConfig) *SourceMap {
	return NewSourceMapWithContext(rustfmt.Context{}, inputs, config)
}
func NewSourceMapWithContext(context rustfmt.Context, inputs SourceMapInputs, config SpanEncodingConfig) *SourceMap {
	value := newSourceMapValue(context, inputs, config)
	return &value
}
func NewSharedSourceMapWithInputs(inputs SourceMapInputs) Shared[SourceMap] {
	return NewSharedSourceMapWithInputsAndConfig(inputs, DefaultSpanEncodingConfig())
}
func NewSharedSourceMapWithInputsAndConfig(inputs SourceMapInputs, config SpanEncodingConfig) Shared[SourceMap] {
	return NewSharedSourceMapWithContext(rustfmt.Context{}, inputs, config)
}
func NewSharedSourceMapWithContext(context rustfmt.Context, inputs SourceMapInputs, config SpanEncodingConfig) Shared[SourceMap] {
	// The mutex has not been used and no reference has escaped at this point.
	return NewShared(newSourceMapValue(context, inputs, config), func(value *SourceMap, context rustfmt.Context) { value.CloseWithContext(context) })
}
func newSourceMapValue(context rustfmt.Context, inputs SourceMapInputs, config SpanEncodingConfig) (value SourceMap) {
	loader := inputs.FileLoader.move(context)
	transferred := false
	defer func() {
		payload := recover()
		if !transferred {
			spanCloseOwnedCallbackPanic(context, loader.CloseWithContext, payload)
		} else if payload != nil {
			context.ResumeUnwind(payload)
		}
	}()
	cwd, failure := loader.currentDirectory(context)
	if failure != nil {
		sourceMapExpectIOFailure(context, "expecting a current working directory to exist", failure)
	}
	empty := EmptyRealFileName()
	workingDir := inputs.PathMapping.ToRealFilenameWithContext(&empty, cwd, context)
	sourceMapTraceEvent(context, sourceMapTraceWorkingDir, func() []rusttracing.FieldValue {
		return []rusttracing.FieldValue{{Name: "working_dir", Value: rusttracing.DebugField(workingDir.WriteDebugFormatter)}}
	})
	value = SourceMap{files: sourceMapFiles{sourceFiles: []Shared[SourceFile]{}, stableIdToSourceFile: make(map[StableSourceFileId]Shared[SourceFile])}, fileLoader: loader, pathMapping: inputs.PathMapping, workingDir: workingDir, hashKind: inputs.HashKind, checksumHashKind: inputs.ChecksumHashKind, config: config, stderr: inputs.Stderr}
	transferred = true
	return
}
func (sm *SourceMap) BuildConfig() SpanEncodingConfig { return sm.config }
func (sm *SourceMap) PathMapping() *FilePathMapping   { return &sm.pathMapping }
func (sm *SourceMap) WorkingDir() *RealFileName       { return &sm.workingDir }
func (sm *SourceMap) FileExists(path RustPath) bool {
	return sm.FileExistsWithContext(path, rustfmt.Context{})
}
func (sm *SourceMap) FileExistsWithContext(path RustPath, context rustfmt.Context) bool {
	return sm.fileLoader.fileExists(context, path)
}

type sourceMapFilesReadState struct {
	sourceMap *SourceMap
	closed    bool
}

// SourceMapFilesReadGuard is the actual shared lock/monotonic vector borrow.
// Borrow/AsSlice descriptors cannot be mutated or closed; Clone creates an
// independent Arc owner. Every borrowed view expires when the guard closes.
type SourceMapFilesReadGuard struct{ state *sourceMapFilesReadState }

func (sm *SourceMap) Files() *SourceMapFilesReadGuard { return sm.FilesWithContext(rustfmt.Context{}) }
func (sm *SourceMap) FilesWithContext(context rustfmt.Context) *SourceMapFilesReadGuard {
	sm.filesMu.RLock()
	return &SourceMapFilesReadGuard{state: &sourceMapFilesReadState{sourceMap: sm}}
}
func (guard *SourceMapFilesReadGuard) check(context rustfmt.Context) {
	if guard == nil || guard.state == nil || guard.state.closed {
		context.Panic("source-map: use of closed files guard")
	}
}
func (guard *SourceMapFilesReadGuard) Len() uint {
	guard.check(rustfmt.Context{})
	return uint(len(guard.state.sourceMap.files.sourceFiles))
}
func (guard *SourceMapFilesReadGuard) Borrow(index uint) *Shared[SourceFile] {
	return guard.BorrowWithContext(index, rustfmt.Context{})
}
func (guard *SourceMapFilesReadGuard) BorrowWithContext(index uint, context rustfmt.Context) *Shared[SourceFile] {
	guard.check(context)
	if index >= uint(len(guard.state.sourceMap.files.sourceFiles)) {
		context.Panic("source-map: file index out of bounds")
	}
	return &guard.state.sourceMap.files.sourceFiles[index]
}
func (guard *SourceMapFilesReadGuard) CloneAt(index uint) Shared[SourceFile] {
	return guard.CloneAtWithContext(index, rustfmt.Context{})
}
func (guard *SourceMapFilesReadGuard) CloneAtWithContext(index uint, context rustfmt.Context) Shared[SourceFile] {
	return guard.BorrowWithContext(index, context).CloneWithContext(context)
}
func (guard *SourceMapFilesReadGuard) AsSlice() []Shared[SourceFile] {
	return guard.AsSliceWithContext(rustfmt.Context{})
}
func (guard *SourceMapFilesReadGuard) AsSliceWithContext(context rustfmt.Context) []Shared[SourceFile] {
	guard.check(context)
	return guard.state.sourceMap.files.sourceFiles
}
func (guard *SourceMapFilesReadGuard) Close() {
	if guard != nil && guard.state != nil && !guard.state.closed {
		guard.state.closed = true
		guard.state.sourceMap.filesMu.RUnlock()
	}
}

func (sm *SourceMap) SourceFileByStableId(id StableSourceFileId) Option[Shared[SourceFile]] {
	return sm.SourceFileByStableIdWithContext(id, rustfmt.Context{})
}
func (sm *SourceMap) SourceFileByStableIdWithContext(id StableSourceFileId, context rustfmt.Context) Option[Shared[SourceFile]] {
	guard := sm.FilesWithContext(context)
	defer guard.Close()
	if file, ok := sm.files.stableIdToSourceFile[id]; ok {
		return Some(file.CloneWithContext(context))
	}
	return None[Shared[SourceFile]]()
}
func (sm *SourceMap) LookupSourceFileIdx(pos BytePos) uint {
	return sm.LookupSourceFileIdxWithContext(pos, rustfmt.Context{})
}
func (sm *SourceMap) LookupSourceFileIdxWithContext(pos BytePos, context rustfmt.Context) uint {
	guard := sm.FilesWithContext(context)
	defer guard.Close()
	size := guard.Len()
	base := uint(0)
	point := uint(0)
	if size > 0 {
		for size > 1 {
			half := size / 2
			middle := base + half
			if guard.BorrowWithContext(middle, context).BorrowWithContext(context).StartPos <= pos {
				base = middle
			}
			size -= half
		}
		point = base
		if guard.BorrowWithContext(base, context).BorrowWithContext(context).StartPos <= pos {
			point++
		}
	}
	if point == 0 && sm.config.OverflowChecks {
		sourceMapPanic(context, SourceMapPanicLookupIndex, rustfmt.PanicStaticStr, "attempt to subtract with overflow")
	}
	return point - 1
}

func (sm *SourceMap) Close() { sm.CloseWithContext(rustfmt.Context{}) }
func (sm *SourceMap) CloseWithContext(context rustfmt.Context) {
	if sm.closed {
		return
	}
	sm.closed = true
	// SourceMapFiles drops its monotonic vector before the stable-ID map, and
	// the outer SourceMap drops both before the consumed loader environment.
	for i := range sm.files.sourceFiles {
		sm.files.sourceFiles[i].CloseWithContext(context)
	}
	sm.files.sourceFiles = nil
	for id, file := range sm.files.stableIdToSourceFile {
		file.CloseWithContext(context)
		delete(sm.files.stableIdToSourceFile, id)
	}
	sm.fileLoader.CloseWithContext(context)
}
