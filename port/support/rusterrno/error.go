// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped SDK708c94 Unix format_error/CStr::display adapter, not a std-file port.
package rusterrno

import (
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const PanicStrerrorRFailure = "std::sys::io::error::unix::format_error.strerror_r"

// Binding witnesses the actual XSI strerror_r operation. It borrows the zeroed
// temporary buffer only during Call, writes a NUL-terminated result, and has no
// owned destructor. Its complete callback implements the source libc call;
// plain Go panics inside it retain Go-local cleanup semantics. The buffer is
// exactly the SDK's 128 bytes for Unix (this adapter does not claim WASI).
type Binding struct {
	Call func(code int32, buffer *[128]byte, context rustfmt.Context) int32
}

// MissingBinding identifies an unavailable host FFI configuration, rather than
// an invented Rust errno message. Supply a Binding explicitly without cgo.
type MissingBinding struct{}

func (MissingBinding) Error() string {
	return "rusterrno: native strerror_r requires cgo or an explicit Binding"
}

func NativeAvailable() bool { return nativeBinding() != nil }

// FormatError follows SDK Unix format_error: a zeroed fixed buffer, failure
// only for a negative return, CStr's lossy chunks, and fresh inner arguments.
// Caller options stay unchanged. The binding and its borrowed state outlive
// this operation; no construction-time Context or runtime scope is captured.
func FormatError(code int32, f *rustfmt.Formatter, binding *Binding) error {
	if binding == nil {
		binding = nativeBinding()
	}
	if binding == nil || binding.Call == nil {
		f.Context.Panic(MissingBinding{})
	}
	var buffer [128]byte
	if binding.Call(code, &buffer, f.Context) < 0 {
		f.Context.PanicAt(rustfmt.SourcePanicInfo{Payload: "strerror_r failure", Kind: rustfmt.PanicStaticStr,
			Location: f.Context.SourcePanicLocation(PanicStrerrorRFailure), CanUnwind: true})
	}
	length := 0
	for length < len(buffer) && buffer[length] != 0 {
		length++
	}
	if length == len(buffer) {
		// A non-terminated result violates the unsafe source FFI precondition;
		// it is outside defined Rust CStr::from_ptr behavior.
		f.Context.Panic("rusterrno: strerror_r binding violated NUL termination")
	}
	return FormatCStrBytes(buffer[:length], f)
}

// FormatCStrBytes is the scoped formatting operation reached by Unix
// format_error's fresh `write!(f, "{}", cstr.display())` argument. Bytes have
// already been bounded at the C NUL. It intentionally ignores caller padding.
func FormatCStrBytes(bytes []byte, f *rustfmt.Formatter) error {
	for len(bytes) != 0 {
		valid := 0
		for valid < len(bytes) {
			value, size := utf8.DecodeRune(bytes[valid:])
			if value == utf8.RuneError && size == 1 {
				break
			}
			valid += size
		}
		if err := f.WriteString(string(bytes[:valid])); err != nil {
			return err
		}
		bytes = bytes[valid:]
		if len(bytes) == 0 {
			break
		}
		invalid := invalidPrefix(bytes)
		if err := f.WriteString("\ufffd"); err != nil {
			return err
		}
		bytes = bytes[invalid:]
	}
	return nil
}

// The prefix length is Utf8Chunks' single invalid subsequence. Invalid leading
// bytes consume one; a valid leading/continuation prefix consumes that prefix
// when the next byte is missing or invalid. Scalar/overlong restrictions apply
// to byte two before any later bytes are consumed.
func invalidPrefix(bytes []byte) int {
	first := bytes[0]
	count := 0
	low, high := byte(0x80), byte(0xbf)
	switch {
	case first >= 0xc2 && first <= 0xdf:
		count = 2
	case first >= 0xe0 && first <= 0xef:
		count = 3
		if first == 0xe0 {
			low = 0xa0
		}
		if first == 0xed {
			high = 0x9f
		}
	case first >= 0xf0 && first <= 0xf4:
		count = 4
		if first == 0xf0 {
			low = 0x90
		}
		if first == 0xf4 {
			high = 0x8f
		}
	default:
		return 1
	}
	if len(bytes) < 2 || bytes[1] < low || bytes[1] > high {
		return 1
	}
	for i := 2; i < count; i++ {
		if i == len(bytes) || bytes[i] < 0x80 || bytes[i] > 0xbf {
			return i
		}
	}
	return count
}
