// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

func (e *Entry[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	e.check(f.Context)
	pretty := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "Entry", pretty)
	b.Field(f.Value(pretty, func(child *rustfmt.Formatter) error {
		if e.kind == Occupied {
			return (&OccupiedEntry[K, V]{entry: e}).WriteDebugFormatter(child)
		}
		return (&VacantEntry[K, V]{entry: e}).WriteDebugFormatter(child)
	}))
	return b.Finish()
}
func (o *OccupiedEntry[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	o.check(f.Context)
	m := o.entry.owner
	if m.keys.Debug == nil || m.values.Debug == nil {
		f.Context.Panic("rusthashmap: missing Entry Debug implementation")
	}
	pretty := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "OccupiedEntry", pretty)
	key := o.Key(f.Context)
	b.Field("key", f.Value(pretty, func(child *rustfmt.Formatter) error { return m.keys.Debug(key, child) }))
	value := o.Get(f.Context)
	b.Field("value", f.Value(pretty, func(child *rustfmt.Formatter) error { return m.values.Debug(value, child) }))
	return b.FinishNonExhaustive()
}
func (v *VacantEntry[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	v.check(f.Context)
	if v.entry.owner.keys.Debug == nil {
		f.Context.Panic("rusthashmap: missing key Debug implementation")
	}
	pretty := f.Options.Alternate
	b := rustfmt.BeginTuple(f.Writer, "VacantEntry", pretty)
	key := v.Key(f.Context)
	b.Field(f.Value(pretty, func(child *rustfmt.Formatter) error { return v.entry.owner.keys.Debug(key, child) }))
	return b.Finish()
}
func (e *OccupiedError[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	if e == nil || !e.live {
		f.Context.Panic("rusthashmap: use of consumed OccupiedError")
	}
	m := e.Entry.entry.owner
	if m.keys.Debug == nil || m.values.Debug == nil {
		f.Context.Panic("rusthashmap: missing OccupiedError Debug implementation")
	}
	pretty := f.Options.Alternate
	b := rustfmt.BeginStruct(f.Writer, "OccupiedError", pretty)
	key := e.Entry.Key(f.Context)
	b.Field("key", f.Value(pretty, func(child *rustfmt.Formatter) error { return m.keys.Debug(key, child) }))
	b.Field("uninserted_key", f.Value(pretty, func(child *rustfmt.Formatter) error { return m.keys.Debug(e.Key.Borrow(), child) }))
	old := e.Entry.Get(f.Context)
	b.Field("old_value", f.Value(pretty, func(child *rustfmt.Formatter) error { return m.values.Debug(old, child) }))
	b.Field("new_value", f.Value(pretty, func(child *rustfmt.Formatter) error { return m.values.Debug(e.Value.Borrow(), child) }))
	return b.FinishNonExhaustive()
}
