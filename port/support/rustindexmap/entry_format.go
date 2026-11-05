// SPDX-License-Identifier: MIT OR Apache-2.0
// Exact entry Debug struct/tuple shapes from indexmap2.14.0 src/map/entry.rs.
package rustindexmap

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

func requireEntryDebug[K, V any](config Config[K, V], value bool, c Context) {
	if config.Keys.Debug == nil || value && config.Values.Debug == nil {
		c.Panic("rustindexmap: unavailable source entry Debug traits")
	}
}
func (e *Entry[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	e.check(f.Context)
	requireEntryDebug(e.owner.core.config, true, f.Context)
	pretty := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "Entry", pretty)
	b.Field(f.Value(pretty, func(child *rustfmt.Formatter) error {
		if e.kind == Vacant {
			return (&VacantEntry[K, V]{e}).WriteDebugFormatter(child)
		}
		return (&OccupiedEntry[K, V]{e}).WriteDebugFormatter(child)
	}))
	return b.Finish()
}
func (o *OccupiedEntry[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	o.entry.check(f.Context)
	config := o.entry.owner.core.config
	requireEntryDebug(config, true, f.Context)
	pretty := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "OccupiedEntry", pretty)
	key := o.Key(f.Context)
	b.Field("key", f.Value(pretty, func(child *rustfmt.Formatter) error { return config.Keys.Debug(key, child) }))
	value := o.Get(f.Context)
	b.Field("value", f.Value(pretty, func(child *rustfmt.Formatter) error { return config.Values.Debug(value, child) }))
	return b.Finish()
}
func (v *VacantEntry[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	v.entry.check(f.Context)
	config := v.entry.owner.core.config
	requireEntryDebug(config, false, f.Context)
	pretty := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "VacantEntry", pretty)
	key := v.Key(f.Context)
	b.Field(f.Value(pretty, func(child *rustfmt.Formatter) error { return config.Keys.Debug(key, child) }))
	return b.Finish()
}
func (e *IndexedEntry[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	e.check(f.Context)
	config := e.owner.core.config
	requireEntryDebug(config, true, f.Context)
	pretty := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "IndexedEntry", pretty)
	b.Field("index", f.Value(pretty, func(child *rustfmt.Formatter) error {
		return rustfmt.FormatDebugUint(uint64(e.index), config.Profile.PointerBits, child)
	}))
	key := e.Key(f.Context)
	b.Field("key", f.Value(pretty, func(child *rustfmt.Formatter) error { return config.Keys.Debug(key, child) }))
	value := e.Get(f.Context)
	b.Field("value", f.Value(pretty, func(child *rustfmt.Formatter) error { return config.Values.Debug(value, child) }))
	return b.Finish()
}
