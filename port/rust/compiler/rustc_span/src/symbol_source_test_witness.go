// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// SymbolInternerSourceTestWitness is a Go-only bridge for the original child
// tests, which Rust permits to access private Interner methods. It delegates to
// the actual implementation with the supplied prefills; it is not a Session,
// does not synthesize IDs and earns no additional upstream public API credit.
type SymbolInternerSourceTestWitness struct{ interner *symbolInterner }

func NewSymbolInternerSourceTestWitness(initial, extra []string) SymbolInternerSourceTestWitness {
	return SymbolInternerSourceTestWitness{prefillSymbolInterner(initial, extra, rustfmt.Context{})}
}
func (witness SymbolInternerSourceTestWitness) InternStr(text string) Symbol {
	return NewSymbol(witness.interner.intern(text, rustfmt.Context{}))
}
func (witness SymbolInternerSourceTestWitness) InternByteStr(bytes []byte) ByteSymbol {
	return NewByteSymbol(witness.interner.intern(string(bytes), rustfmt.Context{}))
}
func (witness SymbolInternerSourceTestWitness) GetStr(symbol Symbol) string {
	return witness.interner.get(symbol.value, rustfmt.Context{})
}
func (witness SymbolInternerSourceTestWitness) GetByteStr(symbol ByteSymbol) string {
	return witness.interner.get(symbol.value, rustfmt.Context{})
}
