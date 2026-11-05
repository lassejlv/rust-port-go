// SPDX-License-Identifier: MIT OR Apache-2.0
// Explicit Bucket/Core/IndexMap Clone and CloneFrom operations from the locked
// source. Core::clone constructs an empty core and invokes clone_from; its Vec
// reserve/overwrite behavior is intentionally different from UnordMap's derive.
package rustindexmap

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

func (inner *core[K, V]) cloneKey(source *K, c Context) K {
	return cloneKeyValue(inner.config.Keys, source, c)
}
func cloneKeyValue[K any](ops KeyOps[K], source *K, c Context) K {
	if ops.TrivialClone {
		return *source
	}
	if ops.Clone == nil {
		c.Panic("rustindexmap: missing key Clone witness")
	}
	return ops.Clone(source, c)
}
func (inner *core[K, V]) cloneValue(source *V, c Context) V {
	return cloneValueValue(inner.config.Values, source, c)
}
func cloneValueValue[V any](ops ValueOps[V], source *V, c Context) V {
	if ops.TrivialClone {
		return *source
	}
	if ops.Clone == nil {
		c.Panic("rustindexmap: missing value Clone witness")
	}
	return ops.Clone(source, c)
}
func (inner *core[K, V]) cloneBucket(source *bucket[K, V], c Context) (result bucket[K, V]) {
	return cloneBucketValue(inner.config, source, c)
}
func cloneBucketValue[K, V any](config Config[K, V], source *bucket[K, V], c Context) (result bucket[K, V]) {
	result.hash = source.hash
	result.key = cloneKeyValue(config.Keys, &source.key, c)
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if config.Keys.Drop != nil {
				unwind := c
				unwind.Unwinding = true
				cleanupUnwind(unwind, func() { config.Keys.Drop(&result.key, unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	result.value = cloneValueValue(config.Values, &source.value, c)
	return result
}
func (inner *core[K, V]) cloneKeyFrom(destination, source *K, c Context) {
	if inner.config.Keys.CloneFrom != nil {
		inner.config.Keys.CloneFrom(destination, source, c)
		return
	}
	replacement := inner.cloneKey(source, c)
	defer func() {
		p := recover()
		if p != nil && c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		*destination = replacement
		if p != nil {
			c.ResumeUnwind(p)
		}
	}()
	if inner.config.Keys.Drop != nil {
		inner.config.Keys.Drop(destination, c)
	}
}
func (inner *core[K, V]) cloneValueFrom(destination, source *V, c Context) {
	if inner.config.Values.CloneFrom != nil {
		inner.config.Values.CloneFrom(destination, source, c)
		return
	}
	replacement := inner.cloneValue(source, c)
	defer func() {
		p := recover()
		if p != nil && c.ProcessingPanic {
			c.ResumeUnwind(p)
		}
		*destination = replacement
		if p != nil {
			c.ResumeUnwind(p)
		}
	}()
	if inner.config.Values.Drop != nil {
		inner.config.Values.Drop(destination, c)
	}
}
func (inner *core[K, V]) cloneBucketFrom(destination, source *bucket[K, V], c Context) {
	destination.hash = source.hash
	inner.cloneKeyFrom(&destination.key, &source.key, c)
	inner.cloneValueFrom(&destination.value, &source.value, c)
}
func (inner *core[K, V]) cloneFrom(source *core[K, V], c Context) {
	inner.indices.CloneFrom(source.indices, c)
	if cap(inner.entries) < len(source.entries) {
		inner.reserveEntries(uint(len(source.entries)-len(inner.entries)), c)
	}
	// Vec::clone_from first truncates; each surviving Bucket then receives
	// its own clone_from, and the remaining source tail is newly cloned.
	if len(inner.entries) > len(source.entries) {
		tail := inner.entries[len(source.entries):]
		inner.entries = inner.entries[:len(source.entries)]
		dropBuckets(inner.config, tail, c)
		clear(tail)
	}
	for i := range inner.entries {
		inner.cloneBucketFrom(&inner.entries[i], &source.entries[i], c)
	}
	for len(inner.entries) < len(source.entries) {
		value := inner.cloneBucket(&source.entries[len(inner.entries)], c)
		inner.entries = append(inner.entries, value)
	}
}
func (inner *core[K, V]) clone(c Context) *core[K, V] {
	next := newCore(0, inner.config, c)
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupUnwind(unwind, func() { next.close(unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	next.cloneFrom(inner, c)
	complete = true
	return next
}
func (m *Map[K, V]) Clone(contexts ...Context) *Map[K, V] {
	c := current(contexts)
	m.check(c)
	m.requireCloneTraits(c)
	// Source field expression evaluation clones Core before the builder.
	inner := m.core.clone(c)
	complete := false
	defer func() {
		if p := recover(); p != nil {
			if c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if !complete {
				unwind := c
				unwind.Unwinding = true
				cleanupUnwind(unwind, func() { inner.close(unwind) })
			}
			c.ResumeUnwind(p)
		}
	}()
	builder := m.builder.Clone(c)
	if builder == nil {
		c.Panic("rustindexmap: source Clone returned nil builder")
	}
	complete = true
	return &Map[K, V]{core: inner, builder: builder, live: true}
}
func (m *Map[K, V]) CloneFrom(source *Map[K, V], contexts ...Context) {
	c := current(contexts)
	m.check(c)
	source.check(c)
	m.requireCloneTraits(c)
	source.requireCloneTraits(c)
	if m == source {
		c.Panic("rustindexmap: CloneFrom requires disjoint source/destination borrows")
	}
	if m.core.config.Profile != source.core.config.Profile || m.core.config.BucketLayout != source.core.config.BucketLayout || m.core.config.DebugAssertions != source.core.config.DebugAssertions {
		c.Panic("rustindexmap: incompatible source target dictionaries")
	}
	m.core.cloneFrom(source.core, c)
	m.builder.CloneFrom(source.builder, c)
}
func (m *Map[K, V]) Equal(other *Map[K, V], contexts ...Context) bool {
	c := current(contexts)
	if m.core.config.Values.Equal == nil {
		c.Panic("rustindexmap: missing value PartialEq witness")
	}
	return EqualTo(m, other, m.core.config.Values.Equal, c)
}
func EqualTo[K, V, W any](left *Map[K, V], right *Map[K, W], equal func(*V, *W, Context) bool, contexts ...Context) bool {
	c := current(contexts)
	left.check(c)
	right.check(c)
	if left.Len(c) != right.Len(c) {
		return false
	}
	if equal == nil {
		c.Panic("rustindexmap: missing cross-value PartialEq witness")
	}
	for i := range left.core.entries {
		entry := &left.core.entries[i]
		value, found := right.Get(&entry.key, c)
		if !found || !equal(&entry.value, value, c) {
			return false
		}
	}
	return true
}
func (m *Map[K, V]) NotEqual(other *Map[K, V], contexts ...Context) bool {
	return !m.Equal(other, contexts...)
}
func (m *Map[K, V]) WriteDebugFormatter(f *rustfmt.Formatter) error {
	m.check(f.Context)
	if m.core.config.Keys.Debug == nil || m.core.config.Values.Debug == nil {
		f.Context.Panic("rustindexmap: missing Debug witness")
	}
	pretty := f.Options.Alternate
	builder := rustfmt.BeginMap(f.Writer, pretty)
	for i := range m.core.entries {
		entry := &m.core.entries[i]
		builder.Entry(f.Value(pretty, func(child *rustfmt.Formatter) error { return m.core.config.Keys.Debug(&entry.key, child) }), f.Value(pretty, func(child *rustfmt.Formatter) error { return m.core.config.Values.Debug(&entry.value, child) }))
	}
	return builder.Finish()
}

func (m *Map[K, V]) requireCloneTraits(context Context) {
	if !m.core.config.Keys.TrivialClone && m.core.config.Keys.Clone == nil {
		context.Panic("rustindexmap: unavailable source key Clone trait")
	}
	if !m.core.config.Values.TrivialClone && m.core.config.Values.Clone == nil {
		context.Panic("rustindexmap: unavailable source value Clone trait")
	}
	requireSourceBuilderClone(m.builder, context)
}
