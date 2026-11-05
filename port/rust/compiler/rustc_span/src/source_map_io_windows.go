//go:build windows

// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped pinned std Windows File::open, Path::exists, current_dir, UTF-16
// path normalization and io::Error contracts used by RealFileLoader. These
// helpers grant no standard-library source-file completion credit.
package rustcspan

import (
	"errors"
	"os"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

var sourceMapKernel32 = syscall.NewLazyDLL("kernel32.dll")
var sourceMapGetFullPathName = sourceMapKernel32.NewProc("GetFullPathNameW")
var sourceMapGetCurrentDirectory = sourceMapKernel32.NewProc("GetCurrentDirectoryW")
var sourceMapSetLastError = sourceMapKernel32.NewProc("SetLastError")
var sourceMapGetModuleHandle = sourceMapKernel32.NewProc("GetModuleHandleW")
var sourceMapFormatMessage = sourceMapKernel32.NewProc("FormatMessageW")

func sourceMapNativeInterrupted(error) bool { return false }
func sourceMapWrapIOErrorWithBinding(failure error, _ *rusterrno.Binding) error {
	return sourceMapWrapIOError(failure)
}
func sourceMapWrapIOError(failure error) error {
	if _, ok := failure.(ownedSourceIOError); ok {
		return failure
	}
	var number syscall.Errno
	if errors.As(failure, &number) {
		return newSourceIOErrorOSWithFormat(int32(uint32(number)), sourceMapWindowsErrorKind(int32(uint32(number))), sourceMapWindowsFormatError)
	}
	return NewSourceIOErrorOtherString(failure.Error())
}
func sourceMapWindowsFormatError(code int32, f *rustfmt.Formatter) error {
	number := code
	module := uintptr(0)
	flags := uintptr(0)
	if uint32(number)&0x10000000 != 0 {
		name := [...]uint16{'N', 'T', 'D', 'L', 'L', '.', 'D', 'L', 'L', 0}
		module, _, _ = sourceMapGetModuleHandle.Call(uintptr(unsafe.Pointer(&name[0])))
		if module != 0 {
			number ^= 0x10000000
			flags = 0x800
		}
	}
	var buffer [2048]uint16
	n, _, failure := sourceMapFormatMessage.Call(flags|0x1000|0x200, module, uintptr(uint32(number)), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0)
	if n == 0 {
		if err := f.WriteString("OS Error "); err != nil {
			return err
		}
		if err := rustfmt.FormatDebugInt(int64(number), 32, f); err != nil {
			return err
		}
		if err := f.WriteString(" (FormatMessageW() returned error "); err != nil {
			return err
		}
		if err := rustfmt.FormatDebugInt(int64(int32(failure.(syscall.Errno))), 32, f); err != nil {
			return err
		}
		return f.WriteString(")")
	}
	valid := true
	for i := uint(0); i < uint(n); i++ {
		v := buffer[i]
		if v >= 0xd800 && v <= 0xdbff {
			if i+1 >= uint(n) || buffer[i+1] < 0xdc00 || buffer[i+1] > 0xdfff {
				valid = false
				break
			}
			i++
		} else if v >= 0xdc00 && v <= 0xdfff {
			valid = false
			break
		}
	}
	if !valid {
		if err := f.WriteString("OS Error "); err != nil {
			return err
		}
		if err := rustfmt.FormatDebugInt(int64(number), 32, f); err != nil {
			return err
		}
		return f.WriteString(" (FormatMessageW() returned invalid UTF-16)")
	}
	text := string(utf16.Decode(buffer[:n]))
	for len(text) > 0 {
		b := text[len(text)-1]
		if b == ' ' || b >= '\t' && b <= '\r' {
			text = text[:len(text)-1]
		} else {
			break
		}
	}
	return f.WriteString(text)
}

// This is OsStr::encode_wide over the stored canonical WTF-8 representation.
func sourceMapWindowsWide(path RustPath) ([]uint16, error) {
	wide := make([]uint16, 0, len(path.raw)+1)
	for i := 0; i < len(path.raw); {
		if surrogate, n := pathSurrogateAt(path.raw[i:]); n != 0 {
			wide = append(wide, surrogate)
			i += n
			continue
		}
		char, n := utf8.DecodeRuneInString(path.raw[i:])
		if char == utf8.RuneError && n == 1 {
			return nil, NewSourceIOErrorSimpleMessage("InvalidInput", "invalid Windows WTF-8 path")
		}
		if char == 0 {
			return nil, NewSourceIOErrorSimpleMessage("InvalidInput", "strings passed to WinAPI cannot contain NULs")
		}
		if char > 0xffff {
			a, b := utf16.EncodeRune(char)
			wide = append(wide, uint16(a), uint16(b))
		} else {
			wide = append(wide, uint16(char))
		}
		i += n
	}
	return append(wide, 0), nil
}
func sourceMapWindowsFillUTF16(call func(*uint16, uint32) (uint32, syscall.Errno)) ([]uint16, error) {
	// fill_utf16_buf's reset/call/last-error observation is one source-thread
	// operation. Pin only this scoped Windows protocol, without global state.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var stack [512]uint16
	buffer := stack[:]
	size := uint(512)
	heapCapacity := uint(0)
	for {
		if size > uint(len(buffer)) {
			capacity := heapCapacity * 2
			if capacity < size {
				capacity = size
			}
			if capacity > uint(^uint32(0)) {
				capacity = uint(^uint32(0))
			}
			buffer = make([]uint16, capacity)
			heapCapacity = capacity
			size = capacity
		}
		sourceMapSetLastError.Call(0)
		n, failure := call(&buffer[0], uint32(size))
		k := uint(n)
		if k == 0 && failure != 0 {
			return nil, sourceMapWrapIOError(failure)
		}
		if k == size && failure == syscall.ERROR_INSUFFICIENT_BUFFER {
			if size > uint(^uint32(0))/2 {
				size = uint(^uint32(0))
			} else {
				size *= 2
			}
		} else if k > size {
			size = k
		} else if k == size {
			panic("internal error: entered unreachable code")
		} else {
			return append([]uint16(nil), buffer[:k]...), nil
		}
	}
}
func sourceMapWindowsLongPath(path RustPath) ([]uint16, error) {
	wide, failure := sourceMapWindowsWide(path)
	if failure != nil {
		return nil, failure
	}
	has := func(prefix []uint16) bool {
		if len(wide) < len(prefix) {
			return false
		}
		for i, v := range prefix {
			if wide[i] != v {
				return false
			}
		}
		return true
	}
	sep := func(v uint16) bool { return v == '\\' || v == '/' }
	if has([]uint16{'\\', '\\', '?', '\\'}) || has([]uint16{'\\', '?', '?', '\\'}) || len(wide) == 1 {
		return wide, nil
	}
	if len(wide) < 248 {
		if len(wide) >= 3 && wide[1] == ':' && !sep(wide[0]) && (len(wide) == 3 || sep(wide[2])) {
			return wide, nil
		}
		if len(wide) >= 2 && sep(wide[0]) && sep(wide[1]) {
			return wide, nil
		}
	}
	absolute, err := sourceMapWindowsFillUTF16(func(buffer *uint16, n uint32) (uint32, syscall.Errno) {
		result, _, failure := sourceMapGetFullPathName.Call(uintptr(unsafe.Pointer(&wide[0])), uintptr(n), uintptr(unsafe.Pointer(buffer)), 0)
		return uint32(result), failure.(syscall.Errno)
	})
	if err != nil {
		return nil, err
	}
	var prefix []uint16
	if len(absolute) >= 3 && absolute[1] == ':' && absolute[2] == '\\' {
		prefix = []uint16{'\\', '\\', '?', '\\'}
	} else if len(absolute) >= 4 && absolute[0] == '\\' && absolute[1] == '\\' && absolute[2] == '.' && absolute[3] == '\\' {
		absolute = absolute[4:]
		prefix = []uint16{'\\', '\\', '?', '\\'}
	} else if len(absolute) >= 4 && absolute[0] == '\\' && (absolute[1] == '\\' && absolute[2] == '?' || absolute[1] == '?' && absolute[2] == '?') && absolute[3] == '\\' {
	} else if len(absolute) >= 2 && absolute[0] == '\\' && absolute[1] == '\\' {
		absolute = absolute[2:]
		prefix = []uint16{'\\', '\\', '?', '\\', 'U', 'N', 'C', '\\'}
	}
	out := make([]uint16, 0, len(prefix)+len(absolute)+1)
	out = append(out, prefix...)
	out = append(out, absolute...)
	return append(out, 0), nil
}
func sourceMapNativeOpen(path RustPath) (*os.File, error) {
	wide, failure := sourceMapWindowsLongPath(path)
	if failure != nil {
		return nil, failure
	}
	handle, err := syscall.CreateFile(&wide[0], syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, sourceMapWrapIOError(err)
	}
	return os.NewFile(uintptr(handle), path.ToStringLossy().String()), nil
}

var sourceMapGetFileInformationEx = sourceMapKernel32.NewProc("GetFileInformationByHandleEx")
var sourceMapFindFirstFileEx = sourceMapKernel32.NewProc("FindFirstFileExW")

// The source File::file_attr probes reparse metadata only when that bit was
// returned by GetFileInformationByHandle; failure propagates before len/type.
func sourceMapWindowsFileAttr(handle syscall.Handle) (syscall.ByHandleFileInformation, bool, error) {
	var info syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(handle, &info); err != nil {
		return info, false, err
	}
	tag := uint32(0)
	if info.FileAttributes&0x400 != 0 {
		var attributeTag struct{ Attributes, Tag uint32 }
		good, _, err := sourceMapGetFileInformationEx.Call(uintptr(handle), 9, uintptr(unsafe.Pointer(&attributeTag)), unsafe.Sizeof(attributeTag))
		if good == 0 {
			return info, false, err
		}
		if attributeTag.Attributes&0x400 != 0 {
			tag = attributeTag.Tag
		}
	}
	return info, info.FileAttributes&0x400 != 0 && tag&0x20000000 != 0, nil
}
func sourceMapNativeMetadataLen(file *os.File) (uint64, error) {
	info, _, failure := sourceMapWindowsFileAttr(syscall.Handle(file.Fd()))
	if failure != nil {
		return 0, sourceMapWrapIOError(failure)
	}
	return uint64(info.FileSizeLow) | uint64(info.FileSizeHigh)<<32, nil
}
func sourceMapWindowsMetadata(wide []uint16, openReparse bool) (bool, error) {
	flags := uint32(syscall.FILE_FLAG_BACKUP_SEMANTICS)
	if openReparse {
		flags |= 0x200000
	}
	handle, err := syscall.CreateFile(&wide[0], 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE, nil, syscall.OPEN_EXISTING, flags, 0)
	if err == nil {
		defer syscall.CloseHandle(handle)
		_, symlink, err := sourceMapWindowsFileAttr(handle)
		return symlink, err
	}
	if err == syscall.Errno(32) || err == syscall.Errno(5) {
		var data syscall.Win32finddata
		found, _, _ := sourceMapFindFirstFileEx.Call(uintptr(unsafe.Pointer(&wide[0])), 1, uintptr(unsafe.Pointer(&data)), 0, 0, 0)
		if found == ^uintptr(0) {
			return false, err
		}
		syscall.FindClose(syscall.Handle(found))
		symlink := data.FileAttributes&0x400 != 0 && data.Reserved0&0x20000000 != 0
		if !openReparse && symlink {
			return false, err
		}
		return symlink, nil
	}
	return false, err
}
func sourceMapNativeFileExists(path RustPath) bool {
	wide, failure := sourceMapWindowsLongPath(path)
	if failure != nil {
		CloseSourceIOError(failure, rustfmt.Context{})
		return false
	}
	_, err := sourceMapWindowsMetadata(wide, false)
	if err == syscall.Errno(1920) {
		symlink, alternate := sourceMapWindowsMetadata(wide, true)
		return alternate == nil && !symlink
	}
	return err == nil
}
func sourceMapNativeCurrentDirectory() (RustPath, error) {
	wide, failure := sourceMapWindowsFillUTF16(func(buffer *uint16, n uint32) (uint32, syscall.Errno) {
		result, _, err := sourceMapGetCurrentDirectory.Call(uintptr(n), uintptr(unsafe.Pointer(buffer)))
		return uint32(result), err.(syscall.Errno)
	})
	if failure != nil {
		return RustPath{}, failure
	}
	return RustPathFromUTF16(wide), nil
}
