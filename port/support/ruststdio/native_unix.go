//go:build !windows

// SPDX-License-Identifier: MIT OR Apache-2.0
package ruststdio

import (
	"errors"
	"syscall"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func nativeWrite(bytes []byte, _ rustfmt.Context) (int, error) { return syscall.Write(2, bytes) }
func nativeBadFD(err error) bool                               { return errors.Is(err, syscall.EBADF) }
func nativeInterrupted(err error) bool                         { return errors.Is(err, syscall.EINTR) }
