package compiler

import "strconv"

const pointerBits = strconv.IntSize

type intSpec struct {
	Bits              int
	Signed            bool
	CType, PrintMacro string
}

// integerInfo keeps nominal Rust types distinct even where their C width agrees.
func integerInfo(name string) (intSpec, bool) {
	switch name {
	case "i8":
		return intSpec{8, true, "int8_t", "PRId8"}, true
	case "i16":
		return intSpec{16, true, "int16_t", "PRId16"}, true
	case "i32":
		return intSpec{32, true, "int32_t", "PRId32"}, true
	case "i64":
		return intSpec{64, true, "int64_t", "PRId64"}, true
	case "isize":
		return intSpec{pointerBits, true, "intptr_t", "PRIdPTR"}, true
	case "u8":
		return intSpec{8, false, "uint8_t", "PRIu8"}, true
	case "u16":
		return intSpec{16, false, "uint16_t", "PRIu16"}, true
	case "u32":
		return intSpec{32, false, "uint32_t", "PRIu32"}, true
	case "u64":
		return intSpec{64, false, "uint64_t", "PRIu64"}, true
	case "usize":
		return intSpec{pointerBits, false, "uintptr_t", "PRIuPTR"}, true
	default:
		return intSpec{}, false
	}
}

var integerSuffixes = []string{"isize", "usize", "i16", "i32", "i64", "u16", "u32", "u64", "i8", "u8"}
