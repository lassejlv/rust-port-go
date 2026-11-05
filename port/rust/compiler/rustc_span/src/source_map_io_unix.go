//go:build !windows

// SPDX-License-Identifier: MIT OR Apache-2.0
// Used native Unix path/io::Error contracts; no std source-file credit.
package rustcspan

import (
	"errors"
	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"runtime"
	"strings"
	"syscall"
)

func sourceMapNativeOpen(path RustPath) (*os.File, error) {
	if strings.ContainsRune(path.raw, 0) {
		return nil, NewSourceIOErrorSimpleMessage("InvalidInput", "file name contained an unexpected NUL byte")
	}
	file, failure := os.Open(path.raw)
	if failure != nil {
		return nil, failure
	}
	return file, nil
}
func sourceMapNativeFileExists(path RustPath) bool {
	_, failure := os.Stat(path.raw)
	return failure == nil
}
func sourceMapNativeCurrentDirectory() (RustPath, error) {
	// os.Getwd may substitute PWD on Unix. Rust current_dir uses getcwd, so
	// use the actual system directory rather than that environment shortcut.
	path, failure := syscall.Getwd()
	if failure != nil {
		return RustPath{}, failure
	}
	return NewRustPath(path), nil
}
func sourceMapNativeInterrupted(failure error) bool { return errors.Is(failure, syscall.EINTR) }
func sourceMapWrapIOError(failure error) error      { return sourceMapWrapIOErrorWithBinding(failure, nil) }
func sourceMapWrapIOErrorWithBinding(failure error, binding *rusterrno.Binding) error {
	if _, ok := failure.(ownedSourceIOError); ok {
		return failure
	}
	var number syscall.Errno
	if errors.As(failure, &number) {
		code := int32(number)
		number = syscall.Errno(uintptr(uint32(code)))
		return newSourceIOErrorOSWithFormat(code, sourceMapUnixErrorKind(number), func(code int32, f *rustfmt.Formatter) error { return rusterrno.FormatError(code, f, binding) })
	}
	return NewSourceIOErrorOtherString(failure.Error())
}
func sourceMapUnixErrorKind(number syscall.Errno) string {
	// Keep the pinned std Unix match order, including aliases and AIX's
	// ENOTEMPTY/EEXIST distinction. Guarded aliases deliberately use if tests.
	for _, entry := range []struct {
		number syscall.Errno
		kind   string
	}{
		{syscall.E2BIG, "ArgumentListTooLong"}, {syscall.EADDRINUSE, "AddrInUse"}, {syscall.EADDRNOTAVAIL, "AddrNotAvailable"}, {syscall.EBUSY, "ResourceBusy"}, {syscall.ECONNABORTED, "ConnectionAborted"}, {syscall.ECONNREFUSED, "ConnectionRefused"}, {syscall.ECONNRESET, "ConnectionReset"}, {syscall.EDEADLK, "Deadlock"}, {syscall.EDQUOT, "QuotaExceeded"}, {syscall.EEXIST, "AlreadyExists"}, {syscall.EFBIG, "FileTooLarge"}, {syscall.EHOSTUNREACH, "HostUnreachable"}, {syscall.EINTR, "Interrupted"}, {syscall.EINVAL, "InvalidInput"}, {syscall.EISDIR, "IsADirectory"}, {syscall.ELOOP, "FilesystemLoop"}, {syscall.ENOENT, "NotFound"}, {syscall.ENOMEM, "OutOfMemory"}, {syscall.ENOSPC, "StorageFull"}, {syscall.EMLINK, "TooManyLinks"}, {syscall.ENAMETOOLONG, "InvalidFilename"}, {syscall.ENETDOWN, "NetworkDown"}, {syscall.ENETUNREACH, "NetworkUnreachable"}, {syscall.ENOTCONN, "NotConnected"}, {syscall.ENOTDIR, "NotADirectory"}, {syscall.ENOTEMPTY, "DirectoryNotEmpty"}, {syscall.EPIPE, "BrokenPipe"}, {syscall.EROFS, "ReadOnlyFilesystem"}, {syscall.ESPIPE, "NotSeekable"}, {syscall.ESTALE, "StaleNetworkFileHandle"}, {syscall.ETIMEDOUT, "TimedOut"}, {syscall.ETXTBSY, "ExecutableFileBusy"}, {syscall.EXDEV, "CrossesDevices"}, {syscall.EINPROGRESS, "InProgress"}, {syscall.EMFILE, "TooManyOpenFiles"}, {syscall.ENFILE, "TooManyOpenFiles"}, {syscall.EIO, "InputOutputError"}, {syscall.EACCES, "PermissionDenied"}, {syscall.EPERM, "PermissionDenied"}, {syscall.ENOSYS, "Unsupported"},
	} {
		if runtime.GOOS == "aix" && entry.kind == "DirectoryNotEmpty" {
			continue
		}
		if number == entry.number {
			return entry.kind
		}
	}
	if number == syscall.EOPNOTSUPP || number == syscall.ENOTSUP {
		return "Unsupported"
	}
	if number == syscall.EAGAIN || number == syscall.EWOULDBLOCK {
		return "WouldBlock"
	}
	return "Uncategorized"
}

func sourceMapNativeMetadataLen(file *os.File) (uint64, error) {
	metadata, failure := file.Stat()
	if failure != nil {
		return 0, failure
	}
	return uint64(metadata.Size()), nil
}
