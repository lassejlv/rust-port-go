//go:build cgo && (aix || android || darwin || dragonfly || freebsd || illumos || linux || netbsd || openbsd || solaris)

// SPDX-License-Identifier: MIT OR Apache-2.0
package rusterrno

/*
#define _POSIX_C_SOURCE 200112L
#include <stddef.h>
#include <string.h>

// Select glibc's distinct XSI symbol rather than its GNU-return-pointer
// strerror_r API. Musl and the other supported Unix C libraries expose the
// XSI integer-return interface under strerror_r itself.
#if defined(__GLIBC__)
extern int __xpg_strerror_r(int, char *, size_t);
static int ferrugo_strerror_r(int code, char *buffer, size_t size) {
    return __xpg_strerror_r(code, buffer, size);
}
#else
static int ferrugo_strerror_r(int code, char *buffer, size_t size) {
    return strerror_r(code, buffer, size);
}
#endif
*/
import "C"

import (
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

var systemBinding = Binding{Call: func(code int32, buffer *[128]byte, _ rustfmt.Context) int32 {
	return int32(C.ferrugo_strerror_r(C.int(code), (*C.char)(unsafe.Pointer(&buffer[0])), C.size_t(len(buffer))))
}}

func nativeBinding() *Binding { return &systemBinding }
