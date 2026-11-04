// SPDX-License-Identifier: Apache-2.0 OR MIT
// Derived Debug and its primitive witnesses from the complete original crate.
package literalescaper

import rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"

type DebugFormatter = rustfmt.Formatter
type DebugOptions = rustfmt.Options
type DebugContext = rustfmt.Context

func (e *EscapeError) FormatDebug(f *DebugFormatter) error { return f.WriteString(e.String()) }
func (e EscapeError) RustDebug() string                    { return rustfmt.String(e.FormatDebug) }
func (m *Mode) FormatDebug(f *DebugFormatter) error {
	names := [...]string{"Char", "Byte", "Str", "RawStr", "ByteStr", "RawByteStr", "CStr", "RawCStr"}
	if int(*m) >= len(names) {
		panic("literal escaper: invalid source Mode")
	}
	return f.WriteString(names[*m])
}
func (m Mode) RustDebug() string { return rustfmt.String(m.FormatDebug) }
func (c *NonZeroChar) FormatDebug(f *DebugFormatter) error {
	return rustfmt.FormatDebugChar(c.Get(), f)
}
func (c NonZeroChar) RustDebug() string { return rustfmt.String(c.FormatDebug) }
func (b *NonZeroByte) FormatDebug(f *DebugFormatter) error {
	return rustfmt.FormatDebugUint(uint64(b.Get()), 8, f)
}
func (b NonZeroByte) RustDebug() string { return rustfmt.String(b.FormatDebug) }
func (u *MixedUnit) FormatDebug(f *DebugFormatter) error {
	name := "Char"
	var format func(*DebugFormatter) error
	switch u.kind {
	case CharUnit:
		format = func(child *DebugFormatter) error {
			c, ok := u.Char()
			if !ok {
				panic("invalid char unit")
			}
			return c.FormatDebug(child)
		}
	case HighByteUnit:
		name = "HighByte"
		format = func(child *DebugFormatter) error {
			b, ok := u.HighByte()
			if !ok {
				panic("invalid high byte unit")
			}
			return b.FormatDebug(child)
		}
	default:
		panic("literal escaper: invalid source MixedUnit")
	}
	return rustfmt.WriteTuple(f.Writer, name, f.Options.Alternate, []rustfmt.Value{f.Value(f.Options.Alternate, format)})
}
func (u MixedUnit) RustDebug() string { return rustfmt.String(u.FormatDebug) }
