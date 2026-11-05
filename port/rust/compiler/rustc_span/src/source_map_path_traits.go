// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped Path/OsStr formatting and hashing contracts used by pinned rustc_span.
package rustcspan

import (
	"math/bits"
	"strconv"
	"unicode/utf8"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func hashRustOsStr(path RustPath, h stablehash.Hasher) {
	stablehash.HashLengthPrefix(h, uint(len(path.raw)))
	h.Write(path.EncodedBytes())
}
func hashPathPrefix(prefix rustPathPrefix, path RustPath, h stablehash.Hasher) {
	stablehash.HashIsize(h, int(prefix.kind))
	switch prefix.kind {
	case pathVerbatim, pathDeviceNS:
		hashRustOsStr(path.view(prefix.a), h)
	case pathVerbatimUNC, pathUNC:
		hashRustOsStr(path.view(prefix.a), h)
		hashRustOsStr(path.view(prefix.b), h)
	case pathVerbatimDisk, pathDisk:
		stablehash.HashU8(h, prefix.drive)
	}
}

// Hash preserves std Path::hash, including its distinct chunk-bit boundary
// stream and independent Hasher overrides. Writes borrow the actual encoded
// bytes; callbacks must not mutate or retain them beyond their source borrow.
func (path RustPath) Hash(h stablehash.Hasher) {
	start := 0
	verbatim := false
	if prefix, ok := path.parsePrefix(); ok {
		hashPathPrefix(prefix, path, h)
		start = prefix.length()
		verbatim = prefix.verbatim()
	}
	bytes := path.EncodedBytes()[start:]
	componentStart := 0
	chunkBits := uint(0)
	for i := 0; i < len(bytes); i++ {
		isSeparator := path.sep(bytes[i])
		if verbatim {
			isSeparator = bytes[i] == '\\'
		}
		if isSeparator {
			if i > componentStart {
				toHash := bytes[componentStart:i]
				chunkBits = bits.RotateLeft(chunkBits+uint(len(toHash)), -2)
				h.Write(toHash)
			}
			componentStart = i + 1
			if !verbatim && componentStart < len(bytes) && bytes[componentStart] == '.' && (componentStart+1 == len(bytes) || path.sep(bytes[componentStart+1])) {
				componentStart++
			}
		}
	}
	if componentStart < len(bytes) {
		toHash := bytes[componentStart:]
		chunkBits = bits.RotateLeft(chunkBits+uint(len(toHash)), -2)
		h.Write(toHash)
	}
	stablehash.HashUsize(h, chunkBits)
}

// StableHash is the compiler's custom Path implementation in stable_hash.rs,
// not std Path::hash or stable str hashing. Its tags are source derive's u8s.
func (path RustPath) StableHash(_ datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	h.WriteU8(0) // StartOfPath
	components := path.Components()
	for index := uint(0); ; index++ {
		component, ok := components.Next()
		if !ok {
			break
		}
		h.WriteUsize(index)
		switch component.Kind {
		case RustPathPrefix:
			prefix := component.prefix
			h.WriteU8(uint8(prefix.kind) + 1)
			switch prefix.kind {
			case pathVerbatim, pathDeviceNS:
				hashRustOsStr(path.view(prefix.a), h)
			case pathVerbatimUNC, pathUNC:
				hashRustOsStr(path.view(prefix.a), h)
				hashRustOsStr(path.view(prefix.b), h)
			case pathVerbatimDisk, pathDisk:
				h.WriteU8(prefix.drive)
			}
		case RustPathRootDir:
			h.WriteU8(7)
		case RustPathCurDir:
			h.WriteU8(8)
		case RustPathParentDir:
			h.WriteU8(9)
		case RustPathNormal:
			h.WriteU8(10)
			hashRustOsStr(component.Path, h)
		}
	}
	h.WriteU8(11) // EndOfPath
}

func (path RustPath) WriteDebugFormatter(f *rustfmt.Formatter) error {
	if path.Flavor() == RustPathWindows {
		return path.writeWTF8Debug(f)
	}
	return path.writeUnixDebug(f)
}
func (path RustPath) RustDebug() string { return rustfmt.String(path.WriteDebugFormatter) }
func (path RustPath) writeUnixDebug(f *rustfmt.Formatter) error {
	if err := f.WriteChar('"'); err != nil {
		return err
	}
	remaining := path.raw
	for remaining != "" {
		validEnd := 0
		for validEnd < len(remaining) {
			ch, n := utf8.DecodeRuneInString(remaining[validEnd:])
			if ch == utf8.RuneError && n == 1 {
				break
			}
			validEnd += n
		}
		valid := remaining[:validEnd]
		from := 0
		for index, ch := range valid {
			escape := EscapeDebugPinnedUnicode18(ch, false, true)
			if escape != string(ch) {
				if err := f.WriteString(valid[from:index]); err != nil {
					return err
				}
				for _, part := range escape {
					if err := f.WriteChar(part); err != nil {
						return err
					}
				}
				from = index + utf8.RuneLen(ch)
			}
		}
		if err := f.WriteString(valid[from:]); err != nil {
			return err
		}
		remaining = remaining[validEnd:]
		if remaining != "" {
			n := rustInvalidUTF8Length(remaining)
			for _, ch := range []byte(remaining[:n]) {
				if err := f.WriteString(`\x`); err != nil {
					return err
				}
				child := &rustfmt.Formatter{Writer: f.Writer, Context: f.Context, Options: rustfmt.DefaultOptions()}
				child.Options.DebugUpperHex = true
				child.Options.ZeroPad = true
				child.Options.HasWidth = true
				child.Options.Width = 2
				if err := rustfmt.FormatDebugUint(uint64(ch), 8, child); err != nil {
					return err
				}
			}
			remaining = remaining[n:]
		}
	}
	return f.WriteChar('"')
}
func (path RustPath) writeWTF8Debug(f *rustfmt.Formatter) error {
	if err := f.WriteChar('"'); err != nil {
		return err
	}
	for i := 0; i < len(path.raw); {
		if surrogate, n := pathSurrogateAt(path.raw[i:]); n != 0 {
			if err := f.WriteString(`\u{`); err != nil {
				return err
			}
			child := &rustfmt.Formatter{Writer: f.Writer, Context: f.Context, Options: rustfmt.DefaultOptions()}
			child.Options.DebugLowerHex = true
			if err := rustfmt.FormatDebugUint(uint64(surrogate), 16, child); err != nil {
				return err
			}
			if err := f.WriteString("}"); err != nil {
				return err
			}
			i += n
			continue
		}
		ch, n := utf8.DecodeRuneInString(path.raw[i:])
		if ch == utf8.RuneError && n == 1 {
			f.Context.Panic("source-map: invalid Windows WTF-8 path")
		}
		for _, part := range EscapeDebugPinnedUnicode18(ch, false, true) {
			if err := f.WriteChar(part); err != nil {
				return err
			}
		}
		i += n
	}
	return f.WriteChar('"')
}
func pathSurrogateAt(raw string) (uint16, int) {
	if len(raw) >= 3 && raw[0] == 0xED && raw[1] >= 0xA0 && raw[1] <= 0xBF && raw[2] >= 0x80 && raw[2] <= 0xBF {
		return uint16(raw[0]&0x0F)<<12 | uint16(raw[1]&0x3F)<<6 | uint16(raw[2]&0x3F), 3
	}
	return 0, 0
}

// sourcePointerFormat is the standard pointer-shaped field used by SourceMap
// Debug. Host addresses are opaque identities, not Rust ABI addresses.
func sourcePointerFormat(pointer uintptr, f *rustfmt.Formatter) error {
	child := &rustfmt.Formatter{Writer: f.Writer, Context: f.Context, Options: rustfmt.DefaultOptions()}
	child.Options.DebugLowerHex = true
	child.Options.Alternate = true
	return rustfmt.FormatDebugUint(uint64(pointer), uint(strconv.IntSize), child)
}
