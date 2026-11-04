// SPDX-License-Identifier: MIT OR Apache-2.0
// Standard Hash test-input adapters are checked by original-tests/input_dispatch.rs.
package stablehash_test

import (
	"encoding/binary"
	"fmt"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
)

func sourceProfile() (stablehash.ArithmeticProfile, bool) {
	profile := stablehash.DefaultProfile()
	explicit := false
	for _, flag := range []struct {
		name   string
		target *bool
	}{
		{"FERRUGO_STABLE_HASH_TEST_DEBUG", &profile.DebugAssertions},
		{"FERRUGO_STABLE_HASH_TEST_CHECKED", &profile.OverflowChecks},
	} {
		if value, ok := os.LookupEnv(flag.name); ok {
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				panic(err)
			}
			*flag.target = parsed
			explicit = true
		}
	}
	return profile, explicit
}
func newSip(k0, k1 uint64) *stablehash.SipHasher128 {
	if profile, explicit := sourceProfile(); explicit {
		return stablehash.NewSipHasher128WithProfile(k0, k1, profile)
	}
	return stablehash.NewSipHasher128WithKeys(k0, k1)
}
func newStable() *stablehash.StableSipHasher128 {
	if profile, explicit := sourceProfile(); explicit {
		return stablehash.NewStableSipHasher128WithProfile(profile)
	}
	return stablehash.NewStableSipHasher128()
}
func assert_eq[T comparable](t *testing.T, left, right T) {
	t.Helper()
	if left != right {
		t.Fatalf("assert_eq: %#v != %#v", left, right)
	}
}
func assert_ne[T comparable](t *testing.T, left, right T, arguments ...any) {
	t.Helper()
	if left == right {
		if len(arguments) != 0 {
			text := arguments[0].(string)
			text = strings.ReplaceAll(text, "{a}", fmt.Sprint(arguments[1]))
			text = strings.ReplaceAll(text, "{b}", fmt.Sprint(arguments[2]))
			t.Fatal(text)
		}
		t.Fatalf("assert_ne: %#v == %#v", left, right)
	}
}
func assert_true(t *testing.T, condition bool) {
	t.Helper()
	if !condition {
		t.Fatal("source assertion failed")
	}
}

// Bytes mirrors the source's Hash implementation: no slice prefix, one typed
// u8 operation for each byte. Other source Hash inputs below retain prefixes.
type Bytes []byte

func (value Bytes) hash(state stablehash.Hasher) {
	for _, byte := range value {
		stablehash.HashU8(state, byte)
	}
}
func writeHashInput(state stablehash.Hasher, value any) {
	switch value := value.(type) {
	case Bytes:
		value.hash(state)
	case uint8:
		stablehash.HashU8(state, value)
	case uint16:
		stablehash.HashU16(state, value)
	case uint32:
		stablehash.HashU32(state, value)
	case uint64:
		stablehash.HashU64(state, value)
	case uint:
		stablehash.HashUsize(state, value)
	case stablehash.UInt128:
		stablehash.HashU128(state, value)
	case int8:
		stablehash.HashI8(state, value)
	case int16:
		stablehash.HashI16(state, value)
	case int32:
		stablehash.HashI32(state, value)
	case int64:
		stablehash.HashI64(state, value)
	case int:
		stablehash.HashIsize(state, value)
	case stablehash.Int128:
		stablehash.HashI128(state, value)
	case [2]string:
		for _, field := range value {
			stablehash.HashString(state, field)
		}
	case [2]int:
		for _, field := range value {
			stablehash.HashIsize(state, field)
		}
	case [3][]int32:
		for _, field := range value {
			stablehash.HashLengthPrefix(state, uint(len(field)))
			// Rust's integer Hash::hash_slice specialization performs one
			// native-endian bulk write, including empty writes.
			bytes := make([]byte, len(field)*4)
			for index, element := range field {
				binary.NativeEndian.PutUint32(bytes[index*4:], uint32(element))
			}
			state.Write(bytes)
		}
	default:
		panic(fmt.Sprintf("unmapped original Hash test input %T", value))
	}
}
func hash_with(state *stablehash.SipHasher128, value any) stablehash.SipHasher128Hash {
	writeHashInput(state, value)
	return state.Finish128()
}
func sipHash(value any) stablehash.SipHasher128Hash { return hash_with(newSip(0, 0), value) }

const low128Mask = (1 << 64) - 1

func native128(value stablehash.UInt128) []byte {
	bytes := make([]byte, 16)
	var probe [2]byte
	binary.NativeEndian.PutUint16(probe[:], 1)
	if probe[0] == 1 {
		binary.NativeEndian.PutUint64(bytes[:8], value[0])
		binary.NativeEndian.PutUint64(bytes[8:], value[1])
	} else {
		binary.NativeEndian.PutUint64(bytes[:8], value[1])
		binary.NativeEndian.PutUint64(bytes[8:], value[0])
	}
	return bytes
}
func nativeBytes(value any) []byte {
	switch value := value.(type) {
	case uint8:
		return []byte{value}
	case int8:
		return []byte{uint8(value)}
	case uint16:
		var bytes [2]byte
		binary.NativeEndian.PutUint16(bytes[:], value)
		return bytes[:]
	case int16:
		return nativeBytes(uint16(value))
	case uint32:
		var bytes [4]byte
		binary.NativeEndian.PutUint32(bytes[:], value)
		return bytes[:]
	case int32:
		return nativeBytes(uint32(value))
	case uint64:
		var bytes [8]byte
		binary.NativeEndian.PutUint64(bytes[:], value)
		return bytes[:]
	case int64:
		return nativeBytes(uint64(value))
	case uint:
		if strconv.IntSize == 64 {
			return nativeBytes(uint64(value))
		}
		return nativeBytes(uint32(value))
	case int:
		return nativeBytes(uint(value))
	case stablehash.UInt128:
		return native128(value)
	case stablehash.Int128:
		return native128(stablehash.UInt128(value))
	default:
		panic("unmapped original to_ne_bytes test input")
	}
}

type TestHash struct{ Value stablehash.SipHasher128Hash }

func stableHash(value any) TestHash {
	state := newStable()
	writeHashInput(state, value)
	return TestHash{state.FinishHash()}
}

func sliceTupleEqual(a, b [3][]int32) bool {
	for index := range a {
		if !slices.Equal(a[index], b[index]) {
			return false
		}
	}
	return true
}
