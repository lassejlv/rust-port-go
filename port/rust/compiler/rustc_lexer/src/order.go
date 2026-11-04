// SPDX-License-Identifier: MIT OR Apache-2.0
// Source-derived total ordering. Inputs describe lawful Rust enum variants;
// inactive Go union fields and Go-only discriminants are not Rust enum states.
package rustclexer

func orderU32(a, b uint32) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
func orderBool(a, b bool) int {
	if a == b {
		return 0
	}
	if !a {
		return -1
	}
	return 1
}
func orderOption(a, b uint32, hasA, hasB bool) int {
	if n := orderBool(hasA, hasB); n != 0 {
		return n
	}
	if !hasA {
		return 0
	}
	return orderU32(a, b)
}

func (k LiteralKind) Compare(other LiteralKind) int {
	if n := orderU32(uint32(k.Tag), uint32(other.Tag)); n != 0 {
		return n
	}
	switch k.Tag {
	case Int:
		if n := orderU32(uint32(k.Base), uint32(other.Base)); n != 0 {
			return n
		}
		return orderBool(k.EmptyInt, other.EmptyInt)
	case Float:
		if n := orderU32(uint32(k.Base), uint32(other.Base)); n != 0 {
			return n
		}
		return orderBool(k.EmptyExponent, other.EmptyExponent)
	case Char, Byte, Str, ByteStr, CStr:
		return orderBool(k.Terminated, other.Terminated)
	case RawStr, RawByteStr, RawCStr:
		return orderOption(uint32(k.NHashes), uint32(other.NHashes), k.HasNHashes, other.HasNHashes)
	default:
		panic("rustclexer: invalid source LiteralKind")
	}
}
func (k LiteralKind) PartialCompare(other LiteralKind) (int, bool) { return k.Compare(other), true }
func (g GuardedStr) Compare(other GuardedStr) int {
	if n := orderU32(g.NHashes, other.NHashes); n != 0 {
		return n
	}
	if n := orderBool(g.Terminated, other.Terminated); n != 0 {
		return n
	}
	return orderU32(g.TokenLen, other.TokenLen)
}
func (g GuardedStr) PartialCompare(other GuardedStr) (int, bool) { return g.Compare(other), true }
func (e RawStrError) Compare(other RawStrError) int {
	if n := orderU32(uint32(e.Tag), uint32(other.Tag)); n != 0 {
		return n
	}
	switch e.Tag {
	case InvalidStarter:
		return orderU32(uint32(e.BadChar), uint32(other.BadChar))
	case NoTerminator:
		if n := orderU32(e.Expected, other.Expected); n != 0 {
			return n
		}
		if n := orderU32(e.Found, other.Found); n != 0 {
			return n
		}
		return orderOption(e.PossibleTerminatorOffset, other.PossibleTerminatorOffset, e.HasPossibleTerminatorOffset, other.HasPossibleTerminatorOffset)
	case TooManyDelimiters:
		return orderU32(e.Found, other.Found)
	default:
		panic("rustclexer: invalid source RawStrError")
	}
}
func (e RawStrError) PartialCompare(other RawStrError) (int, bool) { return e.Compare(other), true }
