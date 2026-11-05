//go:build windows

// SPDX-License-Identifier: MIT OR Apache-2.0
package ruststdio

import (
	"errors"
	"syscall"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// Used host boundary for the ASCII Graph/SourceMap statements. Embeddings that
// select a Windows console Unicode conversion profile supply that raw Write
// operation explicitly; redirected handles already write their bytes directly.
func nativeWrite(bytes []byte, _ rustfmt.Context) (int, error) {
	return syscall.Write(syscall.Stderr, bytes)
}
func nativeBadFD(err error) bool   { return errors.Is(err, syscall.Errno(6)) } // ERROR_INVALID_HANDLE
func nativeInterrupted(error) bool { return false }
