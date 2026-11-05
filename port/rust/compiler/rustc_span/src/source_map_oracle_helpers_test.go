// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"runtime"
	"strings"
)

// These builders construct supplemental oracle fixtures only. The translated
// original test bodies and their helpers live in source_map/tests_test.go.
func sourceMapOracleFixturePath(value string) RustPath {
	if runtime.GOOS == "windows" {
		value = strings.ReplaceAll(value, "/", `\`)
		if rest, ok := strings.CutPrefix(value, `\`); ok {
			value = `X:\` + rest
		}
	}
	return NewRustPath(value)
}

func sourceMapTestEntries(pairs ...string) []RustPathMappingEntry {
	out := make([]RustPathMappingEntry, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, RustPathMappingEntry{sourceMapOracleFixturePath(pairs[i]), sourceMapOracleFixturePath(pairs[i+1])})
	}
	return out
}
