// SPDX-License-Identifier: MIT OR Apache-2.0
// Expansion data and closed variants from the complete pinned hygiene module.
// Source/API inventory and native comparisons live in tools/port-rust-hygiene.
package rustcspan

import (
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	hashmap "github.com/lassejlv/rust-port-go/port/support/rusthashmap"
)

type ExpnId struct {
	Krate   CrateNum
	LocalID ExpnIndex
}

func RootExpnId() ExpnId                    { return ExpnId{Krate: LocalCrate(), LocalID: ExpnIndexZero()} }
func (v ExpnId) Clone() ExpnId              { return v }
func (v *ExpnId) CloneFrom(other ExpnId)    { *v = other }
func (v ExpnId) Equal(other ExpnId) bool    { return v == other }
func (v ExpnId) NotEqual(other ExpnId) bool { return v != other }
func (v ExpnId) AsLocal() Option[LocalExpnId] {
	if v.Krate == LocalCrate() {
		return Some(localExpnIdFromRaw(v.LocalID))
	}
	return None[LocalExpnId]()
}
func (v ExpnId) ExpectLocal() LocalExpnId { return v.ExpectLocalWithContext(rustfmt.Context{}) }
func (v ExpnId) ExpectLocalWithContext(ctx rustfmt.Context) LocalExpnId {
	return v.expectLocalWithHygieneSite(ctx, "rustc_span::hygiene::ExpnId::expect_local")
}
func (v ExpnId) expectLocalWithHygieneSite(ctx rustfmt.Context, site string) LocalExpnId {
	out, ok := v.AsLocal().Get()
	if !ok {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "called `Option::unwrap()` on a `None` value", Kind: rustfmt.PanicStaticStr, CanUnwind: true, Location: ctx.SourcePanicLocation(site)})
	}
	return out
}
func (v LocalExpnId) ToExpnId() ExpnId { return ExpnId{Krate: LocalCrate(), LocalID: v.AsRaw()} }
func (v ExpnId) Hash(hasher hashmap.Hasher) {
	hashmap.HashU32(hasher, v.Krate.AsU32())
	hashmap.HashU32(hasher, v.LocalID.AsU32())
}

type ExpnHash struct{ fingerprint datastructures.Fingerprint }

func newExpnHash(crate StableCrateId, local rustchashes.Hash64) ExpnHash {
	return ExpnHash{datastructures.NewFingerprint(crate.AsHash(), local)}
}
func (v ExpnHash) StableCrateId() StableCrateId {
	first, _ := v.fingerprint.Split()
	return StableCrateId{hash: first}
}
func (v ExpnHash) LocalHash() rustchashes.Hash64 { _, second := v.fingerprint.Split(); return second }
func (v ExpnHash) IsRoot() bool                  { return v.fingerprint == datastructures.ZeroFingerprint() }
func (v ExpnHash) Clone() ExpnHash               { return v }
func (v *ExpnHash) CloneFrom(other ExpnHash)     { *v = other }
func (v ExpnHash) Equal(other ExpnHash) bool     { return v == other }
func (v ExpnHash) NotEqual(other ExpnHash) bool  { return v != other }
func (v ExpnHash) Hash(hasher hashmap.Hasher) {
	if typed, ok := hasher.(*datastructures.Unhasher); ok {
		v.fingerprint.Hash(typed)
		return
	}
	a, b := v.fingerprint.Split()
	hashmap.HashU64(hasher, a.AsU64())
	hashmap.HashU64(hasher, b.AsU64())
}
func (v ExpnHash) StableHash(hcx datastructures.StableHashCtxt, hasher *datastructures.StableHasher) {
	v.fingerprint.StableHash(hcx, hasher)
}

type Transparency uint8

const (
	Transparent Transparency = iota
	SemiOpaque
	Opaque
)

func TransparencyFallback(macroRules bool) Transparency {
	if macroRules {
		return SemiOpaque
	}
	return Opaque
}
func (v Transparency) Clone() Transparency           { return v }
func (v Transparency) Equal(other Transparency) bool { return v == other }
func (v Transparency) PartialCompare(other Transparency) Option[int] {
	return Some(compareDefIndexValues(uint32(v), uint32(other)))
}
func (v Transparency) Hash(hasher hashmap.Hasher) { hashmap.HashIsize(hasher, int(v)) }
func (v Transparency) StableHash(_ datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	h.WriteU8(uint8(v))
}

type MacroKind uint8

const (
	BangMacro MacroKind = iota
	AttrMacro
	DeriveMacro
)

func (v MacroKind) Descr() string {
	switch v {
	case BangMacro:
		return "macro"
	case AttrMacro:
		return "attribute macro"
	case DeriveMacro:
		return "derive macro"
	}
	panic("invalid Go-only MacroKind")
}
func (v MacroKind) DescrExpected() string {
	if v == AttrMacro {
		return "attribute"
	}
	return v.Descr()
}
func (v MacroKind) Article() string {
	if v == AttrMacro {
		return "an"
	}
	return "a"
}
func (v MacroKind) Clone() MacroKind           { return v }
func (v MacroKind) Equal(other MacroKind) bool { return v == other }
func (v MacroKind) Compare(other MacroKind) int {
	return compareDefIndexValues(uint32(v), uint32(other))
}
func (v MacroKind) PartialCompare(other MacroKind) Option[int] { return Some(v.Compare(other)) }
func (v MacroKind) Hash(hasher hashmap.Hasher)                 { hashmap.HashIsize(hasher, int(v)) }
func (v MacroKind) StableHash(_ datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	h.WriteU8(uint8(v))
}

type AstPass uint8

const (
	StdImports AstPass = iota
	TestHarness
	ProcMacroHarness
)

func (v AstPass) Descr() string {
	switch v {
	case StdImports:
		return "standard library imports"
	case TestHarness:
		return "test harness"
	case ProcMacroHarness:
		return "proc macro harness"
	}
	panic("invalid Go-only AstPass")
}
func (v AstPass) Clone() AstPass           { return v }
func (v AstPass) Equal(other AstPass) bool { return v == other }
func (v AstPass) StableHash(_ datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	h.WriteU8(uint8(v))
}

type DesugaringTag uint8

const (
	DesugarQuestionMark DesugaringTag = iota
	DesugarTryBlock
	DesugarYeetExpr
	DesugarOpaqueTy
	DesugarAsync
	DesugarAwait
	DesugarForLoop
	DesugarWhileLoop
	DesugarBoundModifier
	DesugarContract
	DesugarPatTyRange
	DesugarFormatLiteral
	DesugarRangeExpr
)

// Source is active only for FormatLiteral; other variants keep it false.
type DesugaringKind struct {
	Tag    DesugaringTag
	Source bool
}

func (v DesugaringKind) Clone() DesugaringKind           { return v }
func (v DesugaringKind) Equal(other DesugaringKind) bool { return v == other }
func (v DesugaringKind) Descr() string {
	switch v.Tag {
	case DesugarQuestionMark:
		return "operator `?`"
	case DesugarTryBlock:
		return "`try` block"
	case DesugarYeetExpr:
		return "`do yeet` expression"
	case DesugarOpaqueTy:
		return "`impl Trait`"
	case DesugarAsync:
		return "`async` block or function"
	case DesugarAwait:
		return "`await` expression"
	case DesugarForLoop:
		return "`for` loop"
	case DesugarWhileLoop:
		return "`while` loop"
	case DesugarBoundModifier:
		return "trait bound modifier"
	case DesugarContract:
		return "contract check"
	case DesugarPatTyRange:
		return "pattern type"
	case DesugarFormatLiteral:
		if v.Source {
			return "format string literal"
		}
		return "expression that expanded into a format string literal"
	case DesugarRangeExpr:
		return "range expression"
	}
	panic("invalid Go-only DesugaringKind")
}
func (v DesugaringKind) Matches(value string) bool {
	names := [...]string{"QuestionMark", "TryBlock", "YeetExpr", "OpaqueTy", "Async", "Await", "ForLoop", "WhileLoop", "BoundModifier", "Contract", "PatTyRange", "FormatLiteral", "RangeExpr"}
	if int(v.Tag) >= len(names) {
		panic("invalid Go-only DesugaringKind")
	}
	return value == names[v.Tag]
}
func (v DesugaringKind) StableHash(_ datastructures.StableHashCtxt, h *datastructures.StableHasher) {
	h.WriteU8(uint8(v.Tag))
	if v.Tag == DesugarFormatLiteral {
		if v.Source {
			h.WriteU8(1)
		} else {
			h.WriteU8(0)
		}
	}
}

type ExpnKindTag uint8

const (
	ExpnRoot ExpnKindTag = iota
	ExpnMacro
	ExpnAstPass
	ExpnDesugaring
)

// Only the selected variant's fields are active; others retain zero values.
type ExpnKind struct {
	Tag        ExpnKindTag
	MacroKind  MacroKind
	Name       Symbol
	AstPass    AstPass
	Desugaring DesugaringKind
}

func (v ExpnKind) Clone() ExpnKind           { return v }
func (v ExpnKind) Equal(other ExpnKind) bool { return v == other }
func (v ExpnKind) Descr(session *SessionGlobals) string {
	return v.DescrWithContext(session, rustfmt.Context{})
}
func (v ExpnKind) DescrWithContext(session *SessionGlobals, ctx rustfmt.Context) string {
	switch v.Tag {
	case ExpnRoot:
		return KwPathRoot().AsStrWithContext(session, ctx)
	case ExpnMacro:
		name := v.Name.AsStrWithContext(session, ctx)
		switch v.MacroKind {
		case BangMacro:
			return name + "!"
		case AttrMacro:
			return "#[" + name + "]"
		case DeriveMacro:
			return "#[derive(" + name + ")]"
		}
	case ExpnAstPass:
		return v.AstPass.Descr()
	case ExpnDesugaring:
		return "desugaring of " + v.Desugaring.Descr()
	}
	ctx.Panic("invalid Go-only ExpnKind")
	return ""
}

// disambiguator follows the original private field: only the hygiene allocator
// changes it. Source ExpnData intentionally has neither equality nor Hash.
type ExpnData struct {
	Kind                  ExpnKind
	Parent                ExpnId
	CallSite              Span
	disambiguator         uint32
	DefSite               Span
	AllowInternalUnstable Option[Shared[[]Symbol]]
	Edition               Edition
	MacroDefID            Option[DefId]
	ParentModule          Option[ModId]
	AllowInternalUnsafe   bool
	LocalInnerMacros      bool
	CollapseDebuginfo     bool
	DiagnosticOpaque      bool
}

func NewExpnData(kind ExpnKind, parent ExpnId, callSite, defSite Span, unstable Option[Shared[[]Symbol]], edition Edition, macroDef Option[DefId], parentModule Option[ModId], unsafe, inner, collapse, diagnosticOpaque bool) ExpnData {
	return ExpnData{Kind: kind, Parent: parent, CallSite: callSite, DefSite: defSite, AllowInternalUnstable: unstable, Edition: edition, MacroDefID: macroDef, ParentModule: parentModule, AllowInternalUnsafe: unsafe, LocalInnerMacros: inner, CollapseDebuginfo: collapse, DiagnosticOpaque: diagnosticOpaque}
}
func DefaultExpnData(kind ExpnKind, callSite Span, edition Edition, macroDef Option[DefId], parentModule Option[ModId]) ExpnData {
	return NewExpnData(kind, RootExpnId(), callSite, DummySpan(), None[Shared[[]Symbol]](), edition, macroDef, parentModule, false, false, false, false)
}
func ExpnDataAllowUnstable(kind ExpnKind, callSite Span, edition Edition, unstable Shared[[]Symbol], macroDef Option[DefId], parentModule Option[ModId]) ExpnData {
	data := DefaultExpnData(kind, callSite, edition, macroDef, parentModule)
	data.AllowInternalUnstable = Some(unstable)
	return data
}
func (v *ExpnData) IsRoot() bool    { return v.Kind.Tag == ExpnRoot }
func (v *ExpnData) Clone() ExpnData { return v.CloneWithContext(rustfmt.Context{}) }
func (v *ExpnData) CloneWithContext(ctx rustfmt.Context) ExpnData {
	clone := *v
	if symbols, ok := v.AllowInternalUnstable.Get(); ok {
		clone.AllowInternalUnstable = Some(symbols.CloneWithContext(ctx))
	}
	return clone
}
func (v *ExpnData) Close() { v.CloseWithContext(rustfmt.Context{}) }
func (v *ExpnData) CloseWithContext(ctx rustfmt.Context) {
	if symbols, ok := v.AllowInternalUnstable.Get(); ok {
		v.AllowInternalUnstable = None[Shared[[]Symbol]]()
		symbols.CloseWithContext(ctx)
	}
}
