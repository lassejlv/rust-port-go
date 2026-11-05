// SPDX-License-Identifier: MIT OR Apache-2.0
// RealFileLoader block of pinned source_map.rs. Native path/I/O details are
// supplied by the scoped host adapter, not a synthetic filesystem.
package rustcspan

import (
	"errors"
	"io"
	"os"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// RealFileLoader uses the actual host strerror_r binding on Unix. Without
// cgo, UnixErrorBinding supplies the complete explicit FFI operation; no Go
// errno string or inferred capitalization is substituted. Windows ignores it.
type RealFileLoader struct{ UnixErrorBinding *rusterrno.Binding }

func (loader RealFileLoader) wrapError(err error) error {
	return sourceMapWrapIOErrorWithBinding(err, loader.UnixErrorBinding)
}

func (loader RealFileLoader) FileExists(path RustPath) bool {
	return loader.FileExistsWithContext(rustfmt.Context{}, path)
}
func (RealFileLoader) FileExistsWithContext(context rustfmt.Context, path RustPath) bool {
	return sourceMapNativeFileExists(path)
}
func (loader RealFileLoader) ReadFile(path RustPath) (string, error) {
	return loader.ReadFileWithContext(rustfmt.Context{}, path)
}
func (loader RealFileLoader) ReadFileWithContext(context rustfmt.Context, path RustPath) (string, error) {
	file, failure := sourceMapNativeOpen(path)
	if failure != nil {
		return "", loader.wrapError(failure)
	}
	defer file.Close()
	size := uint64(0)
	if length, err := sourceMapNativeMetadataLen(file); err == nil {
		size = length
	} else {
		CloseSourceIOError(err, context)
	}
	if size > uint64(^uint32(0)-1) {
		return "", NewSourceIOErrorOtherString("text files larger than 4294967294 bytes are unsupported")
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return "", loader.wrapError(err)
	}
	if !utf8.Valid(contents) {
		return "", NewSourceIOErrorSimpleMessage("InvalidData", "stream did not contain valid UTF-8")
	}
	return string(contents), nil
}
func (loader RealFileLoader) ReadBinaryFile(path RustPath) (Shared[[]byte], error) {
	return loader.ReadBinaryFileWithContext(rustfmt.Context{}, path)
}
func (loader RealFileLoader) ReadBinaryFileWithContext(context rustfmt.Context, path RustPath) (Shared[[]byte], error) {
	file, failure := sourceMapNativeOpen(path)
	if failure != nil {
		return Shared[[]byte]{}, loader.wrapError(failure)
	}
	defer file.Close()
	metadataLen, err := sourceMapNativeMetadataLen(file)
	if err != nil {
		return Shared[[]byte]{}, loader.wrapError(err)
	}
	length := sourceMapBinaryArcLength(metadataLen, context)
	bytes := make([]byte, int(length))
	_, err = io.ReadFull(file, bytes)
	if err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			bytes = nil // The original discards the first Arc before reopening.
			return loader.readBinaryFallback(path)
		}
		return Shared[[]byte]{}, loader.wrapError(err)
	}
	var probe [32]byte
	n := 0
	for {
		n, err = file.Read(probe[:])
		if err != nil {
			if errors.Is(err, io.EOF) && n == 0 {
				return NewShared(bytes, nil), nil
			}
			if sourceMapNativeInterrupted(err) {
				continue
			}
			return Shared[[]byte]{}, loader.wrapError(err)
		}
		if n == 0 {
			return NewShared(bytes, nil), nil
		}
		break
	}
	// The known prefix and first nonempty probe are retained before reading the
	// remainder, matching the source growing-file fallback's byte order.
	combined := make([]byte, 0, len(bytes)+n)
	combined = append(combined, bytes...)
	combined = append(combined, probe[:n]...)
	tail, err := io.ReadAll(file)
	if err != nil {
		return Shared[[]byte]{}, loader.wrapError(err)
	}
	combined = append(combined, tail...)
	return NewShared(combined, nil), nil
}
func (loader RealFileLoader) readBinaryFallback(path RustPath) (Shared[[]byte], error) {
	file, failure := sourceMapNativeOpen(path)
	if failure != nil {
		return Shared[[]byte]{}, loader.wrapError(failure)
	}
	defer file.Close()
	bytes, err := io.ReadAll(file)
	if err != nil {
		return Shared[[]byte]{}, loader.wrapError(err)
	}
	return NewShared(bytes, nil), nil
}
func (loader RealFileLoader) CurrentDirectory() (RustPath, error) {
	return loader.CurrentDirectoryWithContext(rustfmt.Context{})
}
func (loader RealFileLoader) CurrentDirectoryWithContext(context rustfmt.Context) (RustPath, error) {
	path, err := sourceMapNativeCurrentDirectory()
	if err != nil {
		return RustPath{}, loader.wrapError(err)
	}
	return path, nil
}

// Native files are closed explicitly on every branch. Go's garbage-collected
// backing allocation is not a claim of Rust Arc allocation/free timing or peak
// allocator memory. The source metadata/probe/reopen protocol is retained.
var _ FileLoader = RealFileLoader{}
var _ ContextFileLoader = RealFileLoader{}
var _ io.Reader = (*os.File)(nil)
