//go:build !cgo || (!aix && !android && !darwin && !dragonfly && !freebsd && !illumos && !linux && !netbsd && !openbsd && !solaris)

// SPDX-License-Identifier: MIT OR Apache-2.0
package rusterrno

func nativeBinding() *Binding { return nil }
