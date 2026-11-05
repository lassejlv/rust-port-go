// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

// Clone preserves bucket positions, deleted tags and spare capacity. Hashbrown
// does not reinsert cloned keys or invoke their Hash/Eq implementations.
func (t *RawTable[T]) Clone(c Context) *RawTable[T] {
	t.check(c)
	next := NewRawTable[T](t.layout, t.profile, t.ops)
	if len(t.values) == 0 {
		return next
	}
	next.allocate(uint(len(t.values)), c)
	next.cloneElements(t, c)
	return next
}

func (t *RawTable[T]) cloneElements(source *RawTable[T], c Context) {
	copy(t.ctrl, source.ctrl)
	if t.ops.TrivialClone {
		copy(t.values, source.values)
	} else {
		if t.ops.Clone == nil {
			c.Panic("rusthashmap: missing source Clone implementation")
		}
		through, success := 0, false
		defer func() {
			if success {
				return
			}
			p := recover()
			if p != nil && c.ProcessingPanic {
				c.ResumeUnwind(p)
			}
			if t.ops.Drop != nil {
				unwind := c
				unwind.Unwinding = true
				for i := 0; i < through; i++ {
					if t.ctrl[i]&0x80 == 0 {
						cleanupOnPanic(unwind, func() { t.ops.Drop(&t.values[i], unwind) })
					}
				}
			}
			if p != nil {
				c.ResumeUnwind(p)
			}
		}()
		for i := range source.values {
			if source.ctrl[i]&0x80 == 0 {
				t.values[i] = t.ops.Clone(&source.values[i], c)
				through = i + 1
			}
		}
		success = true
	}
	t.items, t.growthLeft = source.items, source.growthLeft
}

// CloneFrom drops old elements before cloning, and retains/replaces the
// allocation according to the source bucket count. A throwing old destructor
// leaves the destination empty; already cloned values are destroyed on failure.
func (t *RawTable[T]) CloneFrom(source *RawTable[T], c Context) {
	t.check(c)
	source.check(c)
	if t == source {
		c.Panic("rusthashmap: CloneFrom requires disjoint source and destination borrows")
	}
	if len(source.values) == 0 {
		old := *t
		t.ctrl, t.values, t.items, t.growthLeft = nil, nil, 0, 0
		old.Close(c)
		return
	}
	success := false
	defer func() {
		if !success {
			t.clearMetadata()
		}
	}()
	if t.ops.Drop != nil {
		for i := range t.values {
			if t.ctrl[i]&0x80 == 0 {
				t.ops.Drop(&t.values[i], c)
			}
		}
	}
	if len(t.values) != len(source.values) {
		t.allocate(uint(len(source.values)), c)
	} else {
		clear(t.values)
	}
	t.cloneElements(source, c)
	success = true
}
