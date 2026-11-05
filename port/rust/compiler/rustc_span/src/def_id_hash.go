// SPDX-License-Identifier: MIT OR Apache-2.0
// Hash operations and StableCrateId::new from def_id.rs.
package rustcspan

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// DefIdHasher supplies the precise primitive operations used by native DefId
// Hash. A source Hasher's typed defaults can be supplied through HasherRef or
// equivalent caller-owned adapters; Hash never calls Finish or owns the hasher.
type DefIdHasher interface {
	WriteU32(uint32)
	WriteU64(uint64)
}

func (value DefId) Hash(hasher DefIdHasher) {
	if strconv.IntSize == 64 {
		hasher.WriteU64(uint64(value.Krate.AsU32())<<32 | uint64(value.Index.AsU32()))
	} else {
		value.Index.Hash(hasher)
		value.Krate.Hash(hasher)
	}
}
func (value ModId) Hash(hasher DefIdHasher)        { value.defId.Hash(hasher) }
func (value LocalModId) Hash(hasher spanU32Hasher) { value.localDefId.Hash(hasher) }

func (value DefId) ToStableHashKeyWithContext(context datastructures.StableHashCtxt, ambient rustfmt.Context) DefPathHash {
	return DefPathHash{datastructures.StableHashDefPathHashWithContext(context, value.ToRawDefId(), ambient)}
}
func (value LocalDefId) ToStableHashKeyWithContext(context datastructures.StableHashCtxt, ambient rustfmt.Context) DefPathHash {
	return value.ToDefId().ToStableHashKeyWithContext(context, ambient)
}
func (value DefId) StableHashWithContext(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ambient rustfmt.Context) {
	hasher.SetContext(ambient)
	value.ToStableHashKeyWithContext(context, ambient).StableHash(context, hasher)
}
func (value LocalDefId) StableHashWithContext(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ambient rustfmt.Context) {
	hasher.SetContext(ambient)
	value.ToStableHashKeyWithContext(context, ambient).LocalHash().Hash(hasher)
}
func (value CrateNum) StableHashWithContext(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ambient rustfmt.Context) {
	hasher.SetContext(ambient)
	value.AsDefId().ToStableHashKeyWithContext(context, ambient).StableCrateId().StableHash(context, hasher)
}
func (value ModId) StableHashWithContext(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ambient rustfmt.Context) {
	value.defId.StableHashWithContext(context, hasher, ambient)
}
func (value LocalModId) StableHashWithContext(context datastructures.StableHashCtxt, hasher *datastructures.StableHasher, ambient rustfmt.Context) {
	value.localDefId.StableHashWithContext(context, hasher, ambient)
}

// NewStableCrateId hashes the actual interned text immediately. metadata models
// the consumed concrete Vec<String>; its sort/dedup storage is private to this
// call. cfgVersion models the source static str without inventing version data.
func NewStableCrateId(session *SessionGlobals, crateName Symbol, isExe bool, metadata []string, cfgVersion string, ambient ...rustfmt.Context) StableCrateId {
	context := rustfmt.Context{}
	if len(ambient) > 0 {
		context = ambient[0]
	}
	name := crateName.AsStrWithContext(session, context)
	if !utf8.ValidString(cfgVersion) {
		context.Panic("def_id: invalid UTF-8 in Rust version string")
	}
	return newStableCrateIdWithVersion(name, isExe, metadata, func() string {
		if version, forced := os.LookupEnv("RUSTC_FORCE_RUSTC_VERSION"); forced {
			return defIdLossyUTF8(version)
		}
		return cfgVersion
	}, context)
}

func newStableCrateIdText(name string, isExe bool, metadata []string, version string, context rustfmt.Context) StableCrateId {
	return newStableCrateIdWithVersion(name, isExe, metadata, func() string { return version }, context)
}
func newStableCrateIdWithVersion(name string, isExe bool, metadata []string, version func() string, context rustfmt.Context) StableCrateId {
	if !utf8.ValidString(name) {
		context.Panic("def_id: invalid UTF-8 in Rust string input")
	}
	hasher := stablehash.NewStableSipHasher128()
	hasher.SetContext(context)
	stablehash.HashString(hasher, name)
	values := make([]string, len(metadata))
	copy(values, metadata)
	for _, text := range values {
		if !utf8.ValidString(text) {
			context.Panic("def_id: invalid UTF-8 in metadata string")
		}
	}
	sort.Strings(values)
	hasher.Write([]byte("metadata"))
	for i, text := range values {
		if i > 0 && text == values[i-1] {
			continue
		}
		hasher.WriteUsize(uint(len(text)))
		hasher.Write([]byte(text))
	}
	if isExe {
		hasher.Write([]byte("exe"))
	} else {
		hasher.Write([]byte("lib"))
	}
	versionText := version()
	if !utf8.ValidString(versionText) {
		context.Panic("def_id: invalid UTF-8 in Rust version string")
	}
	hasher.Write([]byte(versionText))
	return StableCrateId{rustchashes.Hash64FromStableHash(rustchashes.StableHasherHash(hasher.FinishHash()))}
}

// defIdLossyUTF8 is the Unix OS-string conversion used by this source call.
// Invalid UTF-8 is replaced per Rust Utf8Error's valid-prefix/error-length
// boundaries: incomplete valid prefixes form one replacement, while distinct
// invalid lead bytes remain distinct. Go's ToValidUTF8 groups too broadly.
func defIdLossyUTF8(text string) string {
	var out strings.Builder
	for len(text) > 0 {
		r, n := utf8.DecodeRuneInString(text)
		if r != utf8.RuneError || n != 1 {
			out.WriteString(text[:n])
			text = text[n:]
			continue
		}
		n = 1
		lead := text[0]
		length := 0
		low, high := byte(0x80), byte(0xbf)
		switch {
		case lead >= 0xc2 && lead <= 0xdf:
			length = 2
		case lead >= 0xe0 && lead <= 0xef:
			length = 3
			if lead == 0xe0 {
				low = 0xa0
			}
			if lead == 0xed {
				high = 0x9f
			}
		case lead >= 0xf0 && lead <= 0xf4:
			length = 4
			if lead == 0xf0 {
				low = 0x90
			}
			if lead == 0xf4 {
				high = 0x8f
			}
		}
		if length > 1 && len(text) > 1 && text[1] >= low && text[1] <= high {
			n = 2
			for n < length && n < len(text) && text[n] >= 0x80 && text[n] <= 0xbf {
				n++
			}
		}
		out.WriteRune(utf8.RuneError)
		text = text[n:]
	}
	return out.String()
}
