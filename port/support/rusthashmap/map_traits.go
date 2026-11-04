// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

func (m *Map[K, V]) Clone(c Context) *Map[K, V] {
	m.check(c)
	next := &Map[K, V]{keys: m.keys, values: m.values, builder: m.builder.Clone(c), live: true}
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupOnPanic(func() { next.builder.Close(unwind) })
			}
			panic(p)
		}
	}()
	next.table = m.table.Clone(c)
	next.table.ops = next.elementOps()
	complete = true
	return next
}
func (m *Map[K, V]) CloneFrom(source *Map[K, V], c Context) {
	m.check(c)
	source.check(c)
	m.table.CloneFrom(source.table, c)
	m.builder.CloneFrom(source.builder, c)
}
func (m *Map[K, V]) Equal(other *Map[K, V], c Context) bool {
	m.check(c)
	other.check(c)
	if m.Len() != other.Len() {
		return false
	}
	if m.values.Equal == nil {
		c.Panic("rusthashmap: missing value PartialEq implementation")
	}
	i := m.table.Iter(c)
	for {
		pair, ok := i.Next(c)
		if !ok {
			return true
		}
		value, found := other.Get(&pair.Key, c)
		if !found || !m.values.Equal(&pair.Value, value, c) {
			return false
		}
	}
}
func (m *Map[K, V]) NotEqual(other *Map[K, V], c Context) bool { return !m.Equal(other, c) }

func (m *Map[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	m.check(f.Context)
	if m.keys.Debug == nil || m.values.Debug == nil {
		f.Context.Panic("rusthashmap: missing Debug implementation")
	}
	pretty := f.Options.Alternate
	b := rustfmt.BeginMap(f.Writer, pretty)
	i := m.table.Iter(f.Context)
	for {
		pair, ok := i.Next(f.Context)
		if !ok {
			break
		}
		b.Entry(f.Value(pretty, func(child *rustfmt.Formatter) error { return m.keys.Debug(&pair.Key, child) }), f.Value(pretty, func(child *rustfmt.Formatter) error { return m.values.Debug(&pair.Value, child) }))
	}
	return b.Finish()
}
