// SPDX-License-Identifier: MIT OR Apache-2.0
// Used std Path/OsStr contracts of pinned rustc_span source_map.rs/lib.rs.
// This helper does not port or claim the complete std path source file.
package rustcspan

import (
	"runtime"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

// CowString retains the observable borrowed/owned alternative of Cow<str>.
// Its immutable string is borrowed for the lifetime of the surrounding view.
type CowString struct {
	text  string
	owned bool
}

func BorrowedCowString(text string) CowString { return CowString{text: text} }
func OwnedCowString(text string) CowString    { return CowString{text: text, owned: true} }
func (text CowString) String() string         { return text.text }
func (text CowString) IsOwned() bool          { return text.owned }
func (text CowString) IsBorrowed() bool       { return !text.owned }

// RustPathFlavor selects the source host's path grammar, not a compilation
// target's grammar. Native is the ordinary public constructor's host profile.
type RustPathFlavor uint8

const (
	RustPathNative RustPathFlavor = iota
	RustPathUnix
	RustPathWindows
)

// RustPath is an immutable OsStr/Path view. Unix encodings preserve arbitrary
// bytes; Windows encodings are WTF-8, retaining unpaired UTF-16 surrogates.
// Callers must not replace this with filepath.Clean or filepath.Join.
type RustPath struct {
	raw    string
	flavor RustPathFlavor
}

// NewRustPath creates owned native PathBuf backing from a borrowed string.
// BorrowedRustPath is the distinct no-copy Path view constructor.
func NewRustPath(raw string) RustPath {
	if runtime.GOOS == "windows" && !utf8.ValidString(raw) {
		panic("source-map: Rust str requires valid UTF-8; use RustPathFromUTF16 for Windows OS strings")
	}
	return RustPath{raw: strings.Clone(raw)}
}
func BorrowedRustPath(raw string) RustPath {
	if runtime.GOOS == "windows" && !utf8.ValidString(raw) {
		panic("source-map: Rust str requires valid UTF-8; use RustPathFromUTF16 for Windows OS strings")
	}
	return RustPath{raw: raw}
}

// BorrowedRustPathFromUnixBytes is OsStr::from_bytes/Path::new's shared borrow.
// The byte backing must remain live and immutable for the entire view lifetime.
// Clone before transferring it to an API that consumes an owned PathBuf.
func BorrowedRustPathFromUnixBytes(raw []byte) RustPath {
	return RustPath{raw: unsafe.String(unsafe.SliceData(raw), len(raw)), flavor: RustPathUnix}
}

// Clone is PathBuf's owned clone, rather than a copied borrowed Path view.
// Nonempty OS bytes receive independent backing; encoded borrows retain their
// actual source-buffer identity when passed to custom Hasher::write callbacks.
func (path RustPath) Clone() RustPath               { return path.view(strings.Clone(path.raw)) }
func (path RustPath) ownedView(raw string) RustPath { return path.view(strings.Clone(raw)) }
func RustPathFromUnixBytes(raw []byte) RustPath {
	return RustPath{raw: string(raw), flavor: RustPathUnix}
}
func RustPathFromUTF16(raw []uint16) RustPath {
	var b strings.Builder
	for i := 0; i < len(raw); i++ {
		ch := rune(raw[i])
		if ch >= 0xD800 && ch <= 0xDBFF && i+1 < len(raw) && raw[i+1] >= 0xDC00 && raw[i+1] <= 0xDFFF {
			b.WriteRune(utf16.DecodeRune(ch, rune(raw[i+1])))
			i++
		} else if ch >= 0xD800 && ch <= 0xDFFF {
			b.WriteByte(byte(0xE0 | (ch >> 12)))
			b.WriteByte(byte(0x80 | ((ch >> 6) & 0x3F)))
			b.WriteByte(byte(0x80 | (ch & 0x3F)))
		} else {
			b.WriteRune(ch)
		}
	}
	return RustPath{raw: b.String(), flavor: RustPathWindows}
}
func (path RustPath) Flavor() RustPathFlavor {
	if path.flavor != RustPathNative {
		return path.flavor
	}
	if runtime.GOOS == "windows" {
		return RustPathWindows
	}
	return RustPathUnix
}
func (path RustPath) EncodedString() string { return path.raw }

// EncodedBytes is a shared read-only borrow, matching OsStr::as_encoded_bytes.
// The caller must never mutate it or retain it past the path's source lifetime.
func (path RustPath) EncodedBytes() []byte {
	return unsafe.Slice(unsafe.StringData(path.raw), len(path.raw))
}
func (path RustPath) CopyEncodedBytes() []byte { return []byte(path.raw) }
func (path RustPath) IsEmpty() bool            { return path.raw == "" }
func (path RustPath) sep(ch byte) bool {
	return ch == '/' || (path.Flavor() == RustPathWindows && ch == '\\')
}
func (path RustPath) separator() string {
	if path.Flavor() == RustPathWindows {
		return "\\"
	}
	return "/"
}
func (path RustPath) view(raw string) RustPath { return RustPath{raw: raw, flavor: path.flavor} }
func (path RustPath) compatible(other RustPath) {
	if path.Flavor() != other.Flavor() {
		panic("source-map: paths belong to different host grammars")
	}
}
func (path RustPath) ToStringLossy() CowString {
	if utf8.ValidString(path.raw) {
		return BorrowedCowString(path.raw)
	}
	var out strings.Builder
	for i := 0; i < len(path.raw); {
		ch, n := utf8.DecodeRuneInString(path.raw[i:])
		if ch != utf8.RuneError || n != 1 {
			out.WriteString(path.raw[i : i+n])
			i += n
			continue
		}
		if path.Flavor() == RustPathWindows && i+2 < len(path.raw) && path.raw[i] == 0xED && path.raw[i+1] >= 0xA0 && path.raw[i+1] <= 0xBF && path.raw[i+2] >= 0x80 && path.raw[i+2] <= 0xBF {
			out.WriteRune(utf8.RuneError)
			i += 3
			continue
		}
		// Rust from_utf8_lossy consumes a whole incomplete suffix as one error,
		// and a malformed sequence's valid continuation prefix as one error.
		errLen := rustInvalidUTF8Length(path.raw[i:])
		out.WriteRune(utf8.RuneError)
		i += errLen
	}
	return OwnedCowString(out.String())
}
func rustInvalidUTF8Length(text string) int {
	first := text[0]
	need := 0
	switch {
	case first >= 0xC2 && first <= 0xDF:
		need = 2
	case first >= 0xE0 && first <= 0xEF:
		need = 3
	case first >= 0xF0 && first <= 0xF4:
		need = 4
	default:
		return 1
	}
	for i := 1; i < need; i++ {
		if i == len(text) {
			return i
		}
		ch := text[i]
		if ch < 0x80 || ch > 0xBF {
			return i
		}
		if i == 1 && ((first == 0xE0 && ch < 0xA0) || (first == 0xED && ch >= 0xA0) || (first == 0xF0 && ch < 0x90) || (first == 0xF4 && ch > 0x8F)) {
			return 1
		}
	}
	return 1
}

type RustPathComponentKind uint8

const (
	RustPathPrefix RustPathComponentKind = iota
	RustPathRootDir
	RustPathCurDir
	RustPathParentDir
	RustPathNormal
)

type rustPathPrefixKind uint8

const (
	pathVerbatim rustPathPrefixKind = iota
	pathVerbatimUNC
	pathVerbatimDisk
	pathDeviceNS
	pathUNC
	pathDisk
)

type rustPathPrefix struct {
	kind  rustPathPrefixKind
	a, b  string
	drive byte
}

func (p rustPathPrefix) verbatim() bool { return p.kind <= pathVerbatimDisk }
func (p rustPathPrefix) isDrive() bool  { return p.kind == pathDisk }
func (p rustPathPrefix) length() int {
	switch p.kind {
	case pathVerbatim:
		return 4 + len(p.a)
	case pathVerbatimUNC:
		if p.b != "" {
			return 9 + len(p.a) + len(p.b)
		}
		return 8 + len(p.a)
	case pathVerbatimDisk:
		return 6
	case pathDeviceNS:
		return 4 + len(p.a)
	case pathUNC:
		if p.b != "" {
			return 3 + len(p.a) + len(p.b)
		}
		return 2 + len(p.a)
	default:
		return 2
	}
}
func (path RustPath) parsePrefix() (rustPathPrefix, bool) {
	if path.Flavor() != RustPathWindows {
		return rustPathPrefix{}, false
	}
	raw := path.raw
	var leading [8]byte
	for i := 0; i < len(raw) && i < len(leading); i++ {
		leading[i] = raw[i]
		if leading[i] == '/' {
			leading[i] = '\\'
		}
	}
	if len(raw) >= 2 && string(leading[:2]) == "\\\\" {
		if len(raw) >= 4 && string(leading[2:4]) == "?\\" && !strings.Contains(raw[:4], "/") {
			if len(raw) >= 8 && string(leading[4:8]) == "UNC\\" {
				a, rest := path.nextPrefixPart(raw[8:], true)
				b, _ := path.nextPrefixPart(rest, true)
				return rustPathPrefix{kind: pathVerbatimUNC, a: a, b: b}, true
			}
			rest := raw[4:]
			if drive, ok := pathDrive(rest); ok && (len(rest) == 2 || rest[2] == '\\') {
				return rustPathPrefix{kind: pathVerbatimDisk, drive: drive}, true
			}
			a, _ := path.nextPrefixPart(rest, true)
			return rustPathPrefix{kind: pathVerbatim, a: a}, true
		}
		if len(raw) >= 4 && string(leading[2:4]) == ".\\" {
			a, _ := path.nextPrefixPart(raw[4:], false)
			return rustPathPrefix{kind: pathDeviceNS, a: a}, true
		}
		a, rest := path.nextPrefixPart(raw[2:], false)
		b, _ := path.nextPrefixPart(rest, false)
		if a != "" && b != "" {
			return rustPathPrefix{kind: pathUNC, a: a, b: b}, true
		}
		return rustPathPrefix{}, false
	}
	if drive, ok := pathDrive(raw); ok {
		return rustPathPrefix{kind: pathDisk, drive: drive}, true
	}
	return rustPathPrefix{}, false
}
func pathDrive(raw string) (byte, bool) {
	if len(raw) < 2 || raw[1] != ':' {
		return 0, false
	}
	ch := raw[0]
	if ch >= 'a' && ch <= 'z' {
		return ch - 'a' + 'A', true
	}
	return ch, ch >= 'A' && ch <= 'Z'
}
func (path RustPath) nextPrefixPart(raw string, verbatim bool) (string, string) {
	for i := 0; i < len(raw); i++ {
		if raw[i] == '\\' || (!verbatim && raw[i] == '/') {
			return raw[:i], raw[i+1:]
		}
	}
	return raw, ""
}

type RustPathComponent struct {
	Kind   RustPathComponentKind
	Path   RustPath
	prefix rustPathPrefix
}

func (c RustPathComponent) Equal(other RustPathComponent) bool {
	return c.Kind == other.Kind && (c.Kind == RustPathPrefix && c.prefix == other.prefix || c.Kind == RustPathNormal && c.Path.raw == other.Path.raw || c.Kind != RustPathPrefix && c.Kind != RustPathNormal)
}

type rustPathState uint8

const (
	pathStatePrefix rustPathState = iota
	pathStateStartDir
	pathStateBody
	pathStateDone
)

type RustPathComponents struct {
	path                    RustPath
	prefix                  rustPathPrefix
	hasPrefix, physicalRoot bool
	front, back             rustPathState
}

func (path RustPath) Components() *RustPathComponents {
	p, ok := path.parsePrefix()
	offset := 0
	if ok {
		offset = p.length()
	}
	front := pathStateStartDir
	if path.Flavor() == RustPathWindows {
		front = pathStatePrefix
	}
	return &RustPathComponents{path: path, prefix: p, hasPrefix: ok, physicalRoot: offset < len(path.raw) && path.sep(path.raw[offset]), front: front, back: pathStateBody}
}
func (c *RustPathComponents) Clone() *RustPathComponents { copy := *c; return &copy }
func (c *RustPathComponents) prefixLen() int {
	if c.hasPrefix {
		return c.prefix.length()
	}
	return 0
}
func (c *RustPathComponents) prefixRemaining() int {
	if c.front == pathStatePrefix {
		return c.prefixLen()
	}
	return 0
}
func (c *RustPathComponents) hasRoot() bool {
	return c.physicalRoot || (c.hasPrefix && !c.prefix.isDrive())
}
func (c *RustPathComponents) sep(ch byte) bool {
	if c.hasPrefix && c.prefix.verbatim() {
		return ch == '\\'
	}
	return c.path.sep(ch)
}
func (c *RustPathComponents) includeCurDir() bool {
	if c.hasRoot() {
		return false
	}
	raw := c.path.raw[c.prefixRemaining():]
	return raw == "." || (len(raw) > 1 && raw[0] == '.' && c.sep(raw[1]))
}
func (c *RustPathComponents) lenBeforeBody() int {
	n := c.prefixRemaining()
	if c.front <= pathStateStartDir && c.physicalRoot {
		n++
	}
	if c.front <= pathStateStartDir && c.includeCurDir() {
		n++
	}
	return n
}
func (c *RustPathComponents) parseSingle(raw string) (RustPathComponent, bool) {
	switch raw {
	case "":
		return RustPathComponent{}, false
	case ".":
		if c.hasPrefix && c.prefix.verbatim() {
			return RustPathComponent{Kind: RustPathCurDir, Path: c.path.view(raw)}, true
		}
		return RustPathComponent{}, false
	case "..":
		return RustPathComponent{Kind: RustPathParentDir, Path: c.path.view(raw)}, true
	default:
		return RustPathComponent{Kind: RustPathNormal, Path: c.path.view(raw)}, true
	}
}
func (c *RustPathComponents) nextPart(back bool) (int, RustPathComponent, bool) {
	raw := c.path.raw
	if back {
		start := c.lenBeforeBody()
		split := -1
		for i := len(raw) - 1; i >= start; i-- {
			if c.sep(raw[i]) {
				split = i
				break
			}
		}
		if split < 0 {
			v, ok := c.parseSingle(raw[start:])
			return len(raw) - start, v, ok
		}
		v, ok := c.parseSingle(raw[split+1:])
		return len(raw) - split, v, ok
	}
	for i := 0; i < len(raw); i++ {
		if c.sep(raw[i]) {
			v, ok := c.parseSingle(raw[:i])
			return i + 1, v, ok
		}
	}
	v, ok := c.parseSingle(raw)
	return len(raw), v, ok
}
func (c *RustPathComponents) Next() (RustPathComponent, bool) {
	for c.front != pathStateDone && c.back != pathStateDone && c.front <= c.back {
		switch c.front {
		case pathStateBody:
			if c.path.raw == "" {
				c.front = pathStateDone
				continue
			}
			n, v, ok := c.nextPart(false)
			c.path.raw = c.path.raw[n:]
			if ok {
				return v, true
			}
		case pathStateStartDir:
			c.front = pathStateBody
			if c.physicalRoot {
				c.path.raw = c.path.raw[1:]
				return RustPathComponent{Kind: RustPathRootDir, Path: c.path.view(c.path.separator())}, true
			}
			if c.hasPrefix {
				if !c.prefix.isDrive() && !c.prefix.verbatim() {
					return RustPathComponent{Kind: RustPathRootDir, Path: c.path.view(c.path.separator())}, true
				}
			} else if c.includeCurDir() {
				c.path.raw = c.path.raw[1:]
				return RustPathComponent{Kind: RustPathCurDir, Path: c.path.view(".")}, true
			}
		case pathStatePrefix:
			c.front = pathStateStartDir
			if n := c.prefixLen(); n != 0 {
				raw := c.path.raw[:n]
				c.path.raw = c.path.raw[n:]
				return RustPathComponent{Kind: RustPathPrefix, Path: c.path.view(raw), prefix: c.prefix}, true
			}
		}
	}
	return RustPathComponent{}, false
}
func (c *RustPathComponents) NextBack() (RustPathComponent, bool) {
	for c.front != pathStateDone && c.back != pathStateDone && c.front <= c.back {
		switch c.back {
		case pathStateBody:
			if len(c.path.raw) <= c.lenBeforeBody() {
				c.back = pathStateStartDir
				continue
			}
			n, v, ok := c.nextPart(true)
			c.path.raw = c.path.raw[:len(c.path.raw)-n]
			if ok {
				return v, true
			}
		case pathStateStartDir:
			c.back = pathStateDone
			if c.path.Flavor() == RustPathWindows {
				c.back = pathStatePrefix
			}
			if c.physicalRoot {
				c.path.raw = c.path.raw[:len(c.path.raw)-1]
				return RustPathComponent{Kind: RustPathRootDir, Path: c.path.view(c.path.separator())}, true
			}
			if c.hasPrefix {
				if !c.prefix.isDrive() && !c.prefix.verbatim() {
					return RustPathComponent{Kind: RustPathRootDir, Path: c.path.view(c.path.separator())}, true
				}
			} else if c.includeCurDir() {
				c.path.raw = c.path.raw[:len(c.path.raw)-1]
				return RustPathComponent{Kind: RustPathCurDir, Path: c.path.view(".")}, true
			}
		case pathStatePrefix:
			c.back = pathStateDone
			if c.prefixLen() != 0 {
				return RustPathComponent{Kind: RustPathPrefix, Path: c.path, prefix: c.prefix}, true
			}
		}
	}
	return RustPathComponent{}, false
}
func (c *RustPathComponents) AsPath() RustPath {
	copy := c.Clone()
	if copy.front == pathStateBody {
		for copy.path.raw != "" {
			n, _, ok := copy.nextPart(false)
			if ok {
				break
			}
			copy.path.raw = copy.path.raw[n:]
		}
	}
	if copy.back == pathStateBody {
		for len(copy.path.raw) > copy.lenBeforeBody() {
			n, _, ok := copy.nextPart(true)
			if ok {
				break
			}
			copy.path.raw = copy.path.raw[:len(copy.path.raw)-n]
		}
	}
	return copy.path
}
func (path RustPath) HasRoot() bool { return path.Components().hasRoot() }
func (path RustPath) IsAbsolute() bool {
	c := path.Components()
	return c.hasRoot() && (path.Flavor() != RustPathWindows || c.hasPrefix)
}
func (path RustPath) FileName() Option[RustPath] {
	v, ok := path.Components().NextBack()
	if !ok || v.Kind != RustPathNormal {
		return None[RustPath]()
	}
	return Some(v.Path)
}
func (path RustPath) StripPrefix(base RustPath) (RustPath, bool) {
	path.compatible(base)
	left, right := path.Components(), base.Components()
	for {
		next := left.Clone()
		a, okA := next.Next()
		b, okB := right.Next()
		if !okB {
			return left.AsPath(), true
		}
		if !okA || !a.Equal(b) {
			return RustPath{}, false
		}
		left = next
	}
}
func (path RustPath) Compare(other RustPath) int {
	path.compatible(other)
	left, right := path.Components(), other.Components()
	for {
		a, okA := left.Next()
		b, okB := right.Next()
		if !okA {
			if !okB {
				return 0
			}
			return -1
		}
		if !okB {
			return 1
		}
		if a.Kind < b.Kind {
			return -1
		}
		if a.Kind > b.Kind {
			return 1
		}
		switch a.Kind {
		case RustPathNormal:
			if n := strings.Compare(a.Path.raw, b.Path.raw); n != 0 {
				return n
			}
		case RustPathPrefix:
			if a.prefix.kind < b.prefix.kind {
				return -1
			}
			if a.prefix.kind > b.prefix.kind {
				return 1
			}
			if n := strings.Compare(a.prefix.a, b.prefix.a); n != 0 {
				return n
			}
			if n := strings.Compare(a.prefix.b, b.prefix.b); n != 0 {
				return n
			}
			if a.prefix.drive < b.prefix.drive {
				return -1
			}
			if a.prefix.drive > b.prefix.drive {
				return 1
			}
		}
	}
}
func (path RustPath) Equal(other RustPath) bool { return path.Compare(other) == 0 }
func (path RustPath) Join(other RustPath) RustPath {
	path.compatible(other)
	needSep := len(path.raw) > 0 && !path.sep(path.raw[len(path.raw)-1])
	comps := path.Components()
	if comps.hasPrefix && comps.prefixLen() == len(path.raw) && comps.prefix.isDrive() {
		needSep = false
	}
	if _, hasPrefix := other.parsePrefix(); other.IsAbsolute() || hasPrefix {
		return path.ownedView(other.raw)
	}
	if comps.hasPrefix && comps.prefix.verbatim() && other.raw != "" {
		var all []RustPathComponent
		for {
			v, ok := comps.Next()
			if !ok {
				break
			}
			all = append(all, v)
		}
		more := other.Components()
		for {
			v, ok := more.Next()
			if !ok {
				break
			}
			switch v.Kind {
			case RustPathRootDir:
				all = append(all[:1], v)
			case RustPathCurDir:
			case RustPathParentDir:
				if len(all) > 0 && all[len(all)-1].Kind == RustPathNormal {
					all = all[:len(all)-1]
				}
			default:
				all = append(all, v)
			}
		}
		var out strings.Builder
		needSep = false
		for _, v := range all {
			if needSep && v.Kind != RustPathRootDir {
				out.WriteString(path.separator())
			}
			out.WriteString(v.Path.raw)
			switch v.Kind {
			case RustPathRootDir:
				needSep = false
			case RustPathPrefix:
				needSep = !v.prefix.isDrive() && v.prefix.length() > 0
			default:
				needSep = true
			}
		}
		return path.ownedView(out.String())
	}
	if other.HasRoot() {
		return path.ownedView(path.raw[:comps.prefixRemaining()] + other.raw)
	}
	if needSep {
		return path.ownedView(path.raw + path.separator() + other.raw)
	}
	return path.ownedView(path.raw + other.raw)
}
