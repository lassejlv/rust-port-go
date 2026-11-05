// SPDX-License-Identifier: MIT OR Apache-2.0
// Filename blocks of compiler/rustc_span/src/lib.rs at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. Whole-file integration is in progress.
package rustcspan

import (
	"math/bits"
	"strconv"
	"strings"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type RemapPathScopeComponents uint8

const (
	RemapPathScopeMacro         RemapPathScopeComponents = 1 << 0
	RemapPathScopeDiagnostics   RemapPathScopeComponents = 1 << 1
	RemapPathScopeDebugInfo     RemapPathScopeComponents = 1 << 3
	RemapPathScopeCoverage      RemapPathScopeComponents = 1 << 4
	RemapPathScopeDocumentation RemapPathScopeComponents = 1 << 5
	RemapPathScopeObject                                 = RemapPathScopeMacro | RemapPathScopeDocumentation | RemapPathScopeDebugInfo | RemapPathScopeCoverage
)

func EmptyRemapPathScopeComponents() RemapPathScopeComponents { return 0 }
func AllRemapPathScopeComponents() RemapPathScopeComponents {
	return RemapPathScopeObject | RemapPathScopeDiagnostics
}
func RemapPathScopeComponentsFromBits(value uint8) Option[RemapPathScopeComponents] {
	if value & ^uint8(AllRemapPathScopeComponents()) != 0 {
		return None[RemapPathScopeComponents]()
	}
	return Some(RemapPathScopeComponents(value))
}
func RemapPathScopeComponentsFromBitsTruncate(value uint8) RemapPathScopeComponents {
	return RemapPathScopeComponents(value) & AllRemapPathScopeComponents()
}
func RemapPathScopeComponentsFromBitsRetain(value uint8) RemapPathScopeComponents {
	return RemapPathScopeComponents(value)
}
func (value RemapPathScopeComponents) Bits() uint8   { return uint8(value) }
func (value RemapPathScopeComponents) IsEmpty() bool { return value == 0 }
func (value RemapPathScopeComponents) IsAll() bool {
	return value.Contains(AllRemapPathScopeComponents())
}
func (value RemapPathScopeComponents) Contains(other RemapPathScopeComponents) bool {
	return value&other == other
}
func (value RemapPathScopeComponents) Intersects(other RemapPathScopeComponents) bool {
	return value&other != 0
}
func (value RemapPathScopeComponents) Union(other RemapPathScopeComponents) RemapPathScopeComponents {
	return value | other
}
func (value RemapPathScopeComponents) Intersection(other RemapPathScopeComponents) RemapPathScopeComponents {
	return value & other
}
func (value RemapPathScopeComponents) Difference(other RemapPathScopeComponents) RemapPathScopeComponents {
	return value &^ other
}
func (value RemapPathScopeComponents) SymmetricDifference(other RemapPathScopeComponents) RemapPathScopeComponents {
	return value ^ other
}
func (value RemapPathScopeComponents) Complement() RemapPathScopeComponents {
	return ^value & AllRemapPathScopeComponents()
}
func (value *RemapPathScopeComponents) Insert(other RemapPathScopeComponents) { *value |= other }
func (value *RemapPathScopeComponents) Remove(other RemapPathScopeComponents) { *value &^= other }
func (value *RemapPathScopeComponents) Toggle(other RemapPathScopeComponents) { *value ^= other }
func (value *RemapPathScopeComponents) Set(other RemapPathScopeComponents, enabled bool) {
	if enabled {
		value.Insert(other)
	} else {
		value.Remove(other)
	}
}
func (value RemapPathScopeComponents) Clone() RemapPathScopeComponents    { return value }
func (value RemapPathScopeComponents) Hash(h interface{ WriteU8(uint8) }) { h.WriteU8(uint8(value)) }

func (value RemapPathScopeComponents) WriteDebugFormatter(f *rustfmt.Formatter) error {
	p := f.Options.Alternate
	builder := rustfmt.BeginTuple(f.Writer, "RemapPathScopeComponents", p)
	builder.Field(f.Value(p, func(child *rustfmt.Formatter) error { return value.writeInternalDebug(child) }))
	return builder.Finish()
}
func (value RemapPathScopeComponents) writeInternalDebug(f *rustfmt.Formatter) error {
	remaining := value
	wrote := false
	for _, entry := range []struct {
		bits RemapPathScopeComponents
		name string
	}{{RemapPathScopeMacro, "MACRO"}, {RemapPathScopeDiagnostics, "DIAGNOSTICS"}, {RemapPathScopeDebugInfo, "DEBUGINFO"}, {RemapPathScopeCoverage, "COVERAGE"}, {RemapPathScopeDocumentation, "DOCUMENTATION"}, {RemapPathScopeObject, "OBJECT"}} {
		if value.Contains(entry.bits) && remaining.Intersects(entry.bits) {
			if wrote {
				if err := f.WriteString(" | "); err != nil {
					return err
				}
			}
			if err := f.WriteString(entry.name); err != nil {
				return err
			}
			remaining &^= entry.bits
			wrote = true
		}
	}
	if remaining != 0 || !wrote {
		if wrote {
			if err := f.WriteString(" | "); err != nil {
				return err
			}
		}
		if err := f.WriteString("0x"); err != nil {
			return err
		}
		if err := f.WriteString(strconv.FormatUint(uint64(remaining), 16)); err != nil {
			return err
		}
	}
	return nil
}

type innerRealFileName struct{ name, workingDirectory, embeddableName RustPath }
type RealFileName struct {
	local         Option[innerRealFileName]
	maybeRemapped innerRealFileName
	scopes        RemapPathScopeComponents
}

func EmptyRealFileName() RealFileName {
	inner := innerRealFileName{name: NewRustPath(""), workingDirectory: NewRustPath(""), embeddableName: NewRustPath("")}
	return RealFileName{local: Some(inner), maybeRemapped: inner}
}
func RealFileNameFromVirtualPath(path RustPath) RealFileName {
	return RealFileName{maybeRemapped: innerRealFileName{name: path.Clone(), embeddableName: path.Clone(), workingDirectory: NewRustPath("")}, scopes: AllRemapPathScopeComponents()}
}
func (value innerRealFileName) Clone() innerRealFileName {
	return innerRealFileName{name: value.name.Clone(), workingDirectory: value.workingDirectory.Clone(), embeddableName: value.embeddableName.Clone()}
}
func (value RealFileName) Clone() RealFileName {
	out := RealFileName{scopes: value.scopes}
	if local, ok := value.local.Get(); ok {
		out.local = Some(local.Clone())
	}
	out.maybeRemapped = value.maybeRemapped.Clone()
	return out
}
func (value RealFileName) WasRemapped() bool      { return !value.scopes.IsEmpty() }
func (value RealFileName) wasFullyRemapped() bool { return value.scopes.IsAll() }
func (value RealFileName) wasNotRemapped() bool   { return value.scopes.IsEmpty() }
func (value RealFileName) Path(scope RemapPathScopeComponents) RustPath {
	return value.PathWithContext(scope, rustfmt.Context{})
}
func (value RealFileName) PathWithContext(scope RemapPathScopeComponents, context rustfmt.Context) RustPath {
	if bits.OnesCount8(scope.Bits()) != 1 {
		spanFormattedPanicAt(context, spanLibLocation(354, 9), func(processing rustfmt.Context) any {
			var text strings.Builder
			text.WriteString("one and only one scope should be passed to `RealFileName::path`: ")
			_ = scope.WriteDebugFormatter(&rustfmt.Formatter{Writer: &text, Options: rustfmt.DefaultOptions(), Context: processing})
			return text.String()
		})
	}
	if !value.scopes.Contains(scope) {
		if local, ok := value.local.Get(); ok {
			return local.name
		}
	}
	return value.maybeRemapped.name
}
func (value RealFileName) EmbeddableName(scope RemapPathScopeComponents) (RustPath, RustPath) {
	return value.EmbeddableNameWithContext(scope, rustfmt.Context{})
}
func (value RealFileName) EmbeddableNameWithContext(scope RemapPathScopeComponents, context rustfmt.Context) (RustPath, RustPath) {
	if bits.OnesCount8(scope.Bits()) != 1 {
		spanFormattedPanicAt(context, spanLibLocation(379, 9), func(processing rustfmt.Context) any {
			var text strings.Builder
			text.WriteString("one and only one scope should be passed to `RealFileName::embeddable_path`: ")
			_ = scope.WriteDebugFormatter(&rustfmt.Formatter{Writer: &text, Options: rustfmt.DefaultOptions(), Context: processing})
			return text.String()
		})
	}
	if !value.scopes.Contains(scope) {
		if local, ok := value.local.Get(); ok {
			return local.workingDirectory, local.embeddableName
		}
	}
	return value.maybeRemapped.workingDirectory, value.maybeRemapped.embeddableName
}
func (value RealFileName) LocalPath() Option[RustPath] {
	if value.wasNotRemapped() {
		return Some(value.maybeRemapped.name)
	}
	if local, ok := value.local.Get(); ok {
		return Some(local.name)
	}
	return None[RustPath]()
}
func (value RealFileName) IntoLocalPath() Option[RustPath] { return value.LocalPath() }
func (value *RealFileName) UpdateForCrateMetadata() {
	if value.wasFullyRemapped() || value.wasNotRemapped() {
		value.local = None[innerRealFileName]()
	}
}
func (value innerRealFileName) Equal(other innerRealFileName) bool {
	return value.name.Equal(other.name) && value.workingDirectory.Equal(other.workingDirectory) && value.embeddableName.Equal(other.embeddableName)
}
func (value innerRealFileName) Compare(other innerRealFileName) int {
	if c := value.name.Compare(other.name); c != 0 {
		return c
	}
	if c := value.workingDirectory.Compare(other.workingDirectory); c != 0 {
		return c
	}
	return value.embeddableName.Compare(other.embeddableName)
}
func (value RealFileName) Equal(other RealFileName) bool {
	a, ap := value.local.Get()
	b, bp := other.local.Get()
	return ap == bp && (!ap || a.Equal(b)) && value.maybeRemapped.Equal(other.maybeRemapped) && value.scopes == other.scopes
}
func (value RealFileName) Compare(other RealFileName) int {
	a, ap := value.local.Get()
	b, bp := other.local.Get()
	if ap != bp {
		if ap {
			return 1
		}
		return -1
	}
	if ap {
		if c := a.Compare(b); c != 0 {
			return c
		}
	}
	if c := value.maybeRemapped.Compare(other.maybeRemapped); c != 0 {
		return c
	}
	if value.scopes < other.scopes {
		return -1
	}
	if value.scopes > other.scopes {
		return 1
	}
	return 0
}
func (value innerRealFileName) StableHash(hcx datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	value.name.StableHash(hcx, h)
	value.workingDirectory.StableHash(hcx, h)
	value.embeddableName.StableHash(hcx, h)
}
func (value RealFileName) StableHash(hcx datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	if !value.wasFullyRemapped() {
		if local, ok := value.local.Get(); ok {
			h.WriteU8(1)
			local.StableHash(hcx, h)
		} else {
			h.WriteU8(0)
		}
	}
	value.maybeRemapped.StableHash(hcx, h)
	h.WriteU8(uint8(value.scopes))
}

type fileNameKind uint8

const (
	fileNameReal fileNameKind = iota
	fileNameCfgSpec
	fileNameAnon
	fileNameMacroExpansion
	fileNameProcMacroSourceCode
	fileNameCliCrateAttr
	fileNameCustom
	fileNameDocTest
	fileNameInlineAsm
)

// FileName has a canonical active alternative; constructors validate Rust str
// inputs. Raw paths retain the OS encoding handled by RustPath.
type FileName struct {
	kind   fileNameKind
	real   RealFileName
	hash   rustchashes.Hash64
	custom string
	line   int
}

func CfgSpecFileName(hash rustchashes.Hash64) FileName {
	return FileName{kind: fileNameCfgSpec, hash: hash}
}
func AnonFileName(hash rustchashes.Hash64) FileName { return FileName{kind: fileNameAnon, hash: hash} }
func MacroExpansionFileName(hash rustchashes.Hash64) FileName {
	return FileName{kind: fileNameMacroExpansion, hash: hash}
}
func ProcMacroSourceCodeFileName(hash rustchashes.Hash64) FileName {
	return FileName{kind: fileNameProcMacroSourceCode, hash: hash}
}
func CliCrateAttrFileName(hash rustchashes.Hash64) FileName {
	return FileName{kind: fileNameCliCrateAttr, hash: hash}
}
func InlineAsmFileName(hash rustchashes.Hash64) FileName {
	return FileName{kind: fileNameInlineAsm, hash: hash}
}

func RealFileNameFileName(path RealFileName) FileName {
	return FileName{kind: fileNameReal, real: path}
}
func CustomFileName(text string) FileName {
	requireSpanUTF8(text, rustfmt.Context{})
	return FileName{kind: fileNameCustom, custom: text}
}
func DocTestSourceCode(path RealFileName, line int) FileName {
	return FileName{kind: fileNameDocTest, real: path, line: line}
}
func sourceCodeFileName(kind fileNameKind, text string) FileName {
	requireSpanUTF8(text, rustfmt.Context{})
	h := datastructures.NewStableHasher()
	stablehash.HashString(h, text)
	return FileName{kind: kind, hash: rustchashes.Hash64FromStableHash(rustchashes.StableHasherHash(h.FinishHash()))}
}
func MacroExpansionSourceCode(text string) FileName {
	return sourceCodeFileName(fileNameMacroExpansion, text)
}
func AnonSourceCode(text string) FileName { return sourceCodeFileName(fileNameAnon, text) }
func ProcMacroSourceCode(text string) FileName {
	return sourceCodeFileName(fileNameProcMacroSourceCode, text)
}
func CfgSpecSourceCode(text string) FileName { return sourceCodeFileName(fileNameCfgSpec, text) }
func CliCrateAttrSourceCode(text string) FileName {
	return sourceCodeFileName(fileNameCliCrateAttr, text)
}
func InlineAsmSourceCode(text string) FileName { return sourceCodeFileName(fileNameInlineAsm, text) }
func (value FileName) Clone() FileName {
	switch value.kind {
	case fileNameReal, fileNameDocTest:
		value.real = value.real.Clone()
	case fileNameCustom:
		value.custom = strings.Clone(value.custom)
	}
	return value
}
func (value FileName) IsReal() bool { return value.kind == fileNameReal }
func (value FileName) IntoLocalPath() Option[RustPath] {
	if value.kind == fileNameReal || value.kind == fileNameDocTest {
		return value.real.IntoLocalPath()
	}
	return None[RustPath]()
}
func (value FileName) Equal(other FileName) bool {
	if value.kind != other.kind {
		return false
	}
	switch value.kind {
	case fileNameReal:
		return value.real.Equal(other.real)
	case fileNameDocTest:
		return value.real.Equal(other.real) && value.line == other.line
	case fileNameCustom:
		return value.custom == other.custom
	default:
		return value.hash == other.hash
	}
}
func (value FileName) Compare(other FileName) int {
	if value.kind < other.kind {
		return -1
	}
	if value.kind > other.kind {
		return 1
	}
	switch value.kind {
	case fileNameReal:
		return value.real.Compare(other.real)
	case fileNameDocTest:
		if c := value.real.Compare(other.real); c != 0 {
			return c
		}
		if value.line < other.line {
			return -1
		}
		if value.line > other.line {
			return 1
		}
		return 0
	case fileNameCustom:
		return strings.Compare(value.custom, other.custom)
	default:
		return value.hash.Compare(other.hash)
	}
}
func (value FileName) StableHash(hcx datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	h.WriteU8(uint8(value.kind))
	switch value.kind {
	case fileNameReal:
		value.real.StableHash(hcx, h)
	case fileNameDocTest:
		value.real.StableHash(hcx, h)
		h.WriteIsize(value.line)
	case fileNameCustom:
		h.WriteUsize(uint(len(value.custom)))
		h.Write([]byte(value.custom))
	default:
		value.hash.Hash(h)
	}
}

type fileNameDisplayPreference uint8

const (
	displayRemapped fileNameDisplayPreference = iota
	displayLocal
	displayShort
	displayScope
)

type FileNameDisplay struct {
	inner      *FileName
	preference fileNameDisplayPreference
	scope      RemapPathScopeComponents
}

func (value *FileName) PreferRemappedUnconditionally() FileNameDisplay {
	return FileNameDisplay{inner: value, preference: displayRemapped}
}
func (value *FileName) PreferLocalUnconditionally() FileNameDisplay {
	return FileNameDisplay{inner: value, preference: displayLocal}
}
func (value *FileName) Short() FileNameDisplay {
	return FileNameDisplay{inner: value, preference: displayShort}
}
func (value *FileName) Display(scope RemapPathScopeComponents) FileNameDisplay {
	return FileNameDisplay{inner: value, preference: displayScope, scope: scope}
}
func (value RealFileName) toStringLossy(pref fileNameDisplayPreference, scope RemapPathScopeComponents) CowString {
	return value.toStringLossyWithContext(pref, scope, rustfmt.Context{})
}
func (value RealFileName) toStringLossyWithContext(pref fileNameDisplayPreference, scope RemapPathScopeComponents, context rustfmt.Context) CowString {
	switch pref {
	case displayRemapped:
		return value.maybeRemapped.name.ToStringLossy()
	case displayLocal:
		if local, ok := value.local.Get(); ok {
			return local.name.ToStringLossy()
		}
		return value.maybeRemapped.name.ToStringLossy()
	case displayShort:
		if name, ok := value.maybeRemapped.name.FileName().Get(); ok {
			return name.ToStringLossy()
		}
		return BorrowedCowString("")
	case displayScope:
		return value.PathWithContext(scope, context).ToStringLossy()
	}
	context.Panic("rustcspan: invalid display preference")
	return CowString{}
}
func (value FileNameDisplay) WriteDisplayFormatter(f *rustfmt.Formatter) error {
	v := value.inner
	switch v.kind {
	case fileNameReal, fileNameDocTest:
		return f.WriteString(v.real.toStringLossyWithContext(value.preference, value.scope, f.Context).String())
	case fileNameCustom:
		if err := f.WriteString("<"); err != nil {
			return err
		}
		if err := f.WriteString(v.custom); err != nil {
			return err
		}
		return f.WriteString(">")
	default:
		labels := map[fileNameKind]string{fileNameCfgSpec: "<cfgspec>", fileNameMacroExpansion: "<macro expansion>", fileNameAnon: "<anon>", fileNameProcMacroSourceCode: "<proc-macro source code>", fileNameCliCrateAttr: "<crate attribute>", fileNameInlineAsm: "<inline asm>"}
		return f.WriteString(labels[v.kind])
	}
}
func (value FileNameDisplay) String() string { return rustfmt.String(value.WriteDisplayFormatter) }
func (value FileNameDisplay) ToStringLossy() CowString {
	return value.ToStringLossyWithContext(rustfmt.Context{})
}
func (value FileNameDisplay) ToStringLossyWithContext(context rustfmt.Context) CowString {
	if value.inner.kind == fileNameReal {
		return value.inner.real.toStringLossyWithContext(value.preference, value.scope, context)
	}
	var text strings.Builder
	if err := value.WriteDisplayFormatter(&rustfmt.Formatter{Writer: &text, Options: rustfmt.DefaultOptions(), Context: context}); err != nil {
		context.Panic("a formatting trait implementation returned an error when the underlying stream did not: Error")
	}
	return OwnedCowString(text.String())
}
