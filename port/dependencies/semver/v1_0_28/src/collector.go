// SPDX-License-Identifier: MIT OR Apache-2.0
// The consuming IntoIterator boundary and used Vec::from_iter contract of
// semver 1.0.28 src/impls.rs. Supporting collection grants no std source credit.
package semver

import (
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
	"runtime"
	"strconv"
)

// CollectorConfig selects explicit ambient context and reference std profile.
// ElementSize carries the source Comparator layout for capacity overflow and
// growth buckets; it is not a claim that the Go value has the same Rust ABI.
type CollectorConfig = rustvec.Config
type ComparatorIterator = rustvec.Iterator[Comparator]
type ComparatorIntoIterator = rustvec.IntoIterator[Comparator]

func DefaultCollectorConfig() CollectorConfig {
	size := uint(56)
	if strconv.IntSize == 32 {
		size = 48
		if runtime.GOARCH == "386" {
			size = 40
		}
	}
	return rustvec.DefaultConfig(size)
}

// FromOwnedIterator is the primary source FromIterator implementation. Build a
// typed owned input with rustvec.NewIntoIterator, or use rustvec.FromIterator
// for an already owned iterator. Input and iterator callbacks retain actual
// state cells, source Drop order, and the Vec collection specialization.
func FromOwnedIterator(source ComparatorIntoIterator, configs ...CollectorConfig) VersionReq {
	config := DefaultCollectorConfig()
	if len(configs) > 0 {
		config = configs[0]
		if config.ElementSize == 0 {
			config.ElementSize = DefaultCollectorConfig().ElementSize
		}
	}
	return VersionReq{Comparators: rustvec.Collect(source, config, rustvec.ElementOps[Comparator]{})}
}
