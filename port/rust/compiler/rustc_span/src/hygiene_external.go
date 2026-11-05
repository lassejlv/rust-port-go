// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

func (c SyntaxContext) InExternalMacro(session *SessionGlobals, sourceMap *SourceMap) bool {
	return c.InExternalMacroWithContext(session, sourceMap, rustfmt.Context{})
}
func (c SyntaxContext) InExternalMacroWithContext(session *SessionGlobals, sourceMap *SourceMap, ctx rustfmt.Context) bool {
	if c.IsRoot() {
		return false
	}
	data := c.OuterExpnDataWithContext(session, ctx)
	defer hygieneDropOnReturn(data.CloseWithContext, ctx)
	switch data.Kind.Tag {
	case ExpnRoot:
		return false
	case ExpnDesugaring:
		switch data.Kind.Desugaring.Tag {
		case DesugarForLoop, DesugarWhileLoop, DesugarOpaqueTy, DesugarAsync, DesugarAwait:
			return false
		}
		return true
	case ExpnAstPass:
		return true
	case ExpnMacro:
		if data.Kind.MacroKind != BangMacro {
			return true
		}
		// Preserve the short-circuit path: no SourceMap access for dummy spans.
		if data.DefSite.IsDummyWithContext(session, ctx) {
			return true
		}
		return sourceMap.IsImportedWithContext(session, data.DefSite, ctx)
	}
	ctx.Panic("invalid Go-only ExpnKind")
	return false
}
