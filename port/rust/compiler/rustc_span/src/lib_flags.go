// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual bitflags! generated operations for the lib.rs scope type, following
// locked bitflags 2.10.0 public.rs/traits.rs/iter.rs. This is a scoped dependency
// boundary and does not claim any whole bitflags source file.
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type RemapPathScopeNamedFlag struct {
	Name  string
	Value RemapPathScopeComponents
}

func remapPathScopeDefinitions() [6]RemapPathScopeNamedFlag {
	return [6]RemapPathScopeNamedFlag{{"MACRO", RemapPathScopeMacro}, {"DIAGNOSTICS", RemapPathScopeDiagnostics}, {"DEBUGINFO", RemapPathScopeDebugInfo}, {"COVERAGE", RemapPathScopeCoverage}, {"DOCUMENTATION", RemapPathScopeDocumentation}, {"OBJECT", RemapPathScopeObject}}
}
func RemapPathScopeComponentsFromName(name string) Option[RemapPathScopeComponents] {
	for _, flag := range remapPathScopeDefinitions() {
		if name == flag.Name {
			return Some(flag.Value)
		}
	}
	return None[RemapPathScopeComponents]()
}
func (value RemapPathScopeComponents) ContainsUnknownBits() bool {
	return !AllRemapPathScopeComponents().Contains(value)
}
func (value *RemapPathScopeComponents) Truncate() {
	*value = RemapPathScopeComponentsFromBitsTruncate(uint8(*value))
}
func (value *RemapPathScopeComponents) Clear() { *value = EmptyRemapPathScopeComponents() }
func (value *RemapPathScopeComponents) IntersectionAssign(other RemapPathScopeComponents) {
	*value = value.Intersection(other)
}
func (value RemapPathScopeComponents) WriteBinaryFormatter(f *rustfmt.Formatter) error {
	return rustfmt.FormatUint(uint64(value), 8, 2, false, f)
}
func (value RemapPathScopeComponents) WriteOctalFormatter(f *rustfmt.Formatter) error {
	return rustfmt.FormatUint(uint64(value), 8, 8, false, f)
}
func (value RemapPathScopeComponents) WriteLowerHexFormatter(f *rustfmt.Formatter) error {
	return rustfmt.FormatUint(uint64(value), 8, 16, false, f)
}
func (value RemapPathScopeComponents) WriteUpperHexFormatter(f *rustfmt.Formatter) error {
	return rustfmt.FormatUint(uint64(value), 8, 16, true, f)
}

// These iterators own Copy flags and a borrowed static declaration dictionary.
// The source supplies only Iterator::next (default size_hint is 0,None), not
// DoubleEndedIterator/ExactSizeIterator/Clone implementations.
type RemapPathScopeIterNames struct {
	source, remaining RemapPathScopeComponents
	index             uint
}

func (value RemapPathScopeComponents) IterNames() *RemapPathScopeIterNames {
	return &RemapPathScopeIterNames{source: value, remaining: value}
}
func (iter *RemapPathScopeIterNames) Remaining() RemapPathScopeComponents { return iter.remaining }
func (iter *RemapPathScopeIterNames) SizeHint() (uint, Option[uint])      { return 0, None[uint]() }
func (iter *RemapPathScopeIterNames) Next() (RemapPathScopeNamedFlag, bool) {
	flags := remapPathScopeDefinitions()
	for iter.index < uint(len(flags)) {
		if iter.remaining.IsEmpty() {
			return RemapPathScopeNamedFlag{}, false
		}
		flag := flags[iter.index]
		iter.index++
		if iter.source.Contains(flag.Value) && iter.remaining.Intersects(flag.Value) {
			iter.remaining.Remove(flag.Value)
			return flag, true
		}
	}
	return RemapPathScopeNamedFlag{}, false
}

type RemapPathScopeIter struct {
	inner RemapPathScopeIterNames
	done  bool
}

func (value RemapPathScopeComponents) Iter() *RemapPathScopeIter {
	return &RemapPathScopeIter{inner: *value.IterNames()}
}
func (iter *RemapPathScopeIter) SizeHint() (uint, Option[uint]) { return 0, None[uint]() }
func (iter *RemapPathScopeIter) Next() (RemapPathScopeComponents, bool) {
	if flag, ok := iter.inner.Next(); ok {
		return flag.Value, true
	}
	if !iter.done {
		iter.done = true
		if !iter.inner.remaining.IsEmpty() {
			return iter.inner.remaining, true
		}
	}
	return 0, false
}

type RemapPathScopeIterDefinedNames struct{ index uint }

func RemapPathScopeDefinedNames() *RemapPathScopeIterDefinedNames {
	return &RemapPathScopeIterDefinedNames{}
}
func (iter *RemapPathScopeIterDefinedNames) Next() (RemapPathScopeNamedFlag, bool) {
	flags := remapPathScopeDefinitions()
	if iter.index >= uint(len(flags)) {
		return RemapPathScopeNamedFlag{}, false
	}
	value := flags[iter.index]
	iter.index++
	return value, true
}
func (iter *RemapPathScopeIterDefinedNames) SizeHint() (uint, Option[uint]) { return 0, None[uint]() }

// Extend consumes the selected Iterator environment, borrowing its real state
// on each next call. Its complete destructor runs after iteration or unwind.
func ExtendRemapPathScopeComponentsOwned[S any](value *RemapPathScopeComponents, environment *SpanOwned[S], next func(*S, rustfmt.Context) (RemapPathScopeComponents, bool), ctx rustfmt.Context) {
	owned := environment.MoveWithContext(ctx)
	defer spanCloseOwnedCallback(ctx, owned.DropWithContext)
	for {
		flag, ok := next(owned.Borrow(), ctx)
		if !ok {
			return
		}
		value.Insert(flag)
	}
}
func CollectRemapPathScopeComponentsOwned[S any](environment *SpanOwned[S], next func(*S, rustfmt.Context) (RemapPathScopeComponents, bool), ctx rustfmt.Context) RemapPathScopeComponents {
	value := EmptyRemapPathScopeComponents()
	ExtendRemapPathScopeComponentsOwned(&value, environment, next, ctx)
	return value
}

// Extend/Collect are the generated generic Extend/FromIterator primaries.
// IntoIterator conversion is independent of Iterator::next and happens once;
// its owner and the resulting iterator retain their genuine Drop contracts.
func (value *RemapPathScopeComponents) Extend(input rustvec.IntoIterator[RemapPathScopeComponents], ctx rustfmt.Context) {
	defer spanCloseOwnedCallback(ctx, input.CloseWithContext)
	iterator := input.IntoIter(ctx)
	defer spanCloseOwnedCallback(ctx, iterator.CloseWithContext)
	for {
		flag, ok := iterator.Next(ctx)
		if !ok {
			return
		}
		value.Insert(flag)
	}
}
func CollectRemapPathScopeComponents(input rustvec.IntoIterator[RemapPathScopeComponents], ctx rustfmt.Context) RemapPathScopeComponents {
	value := EmptyRemapPathScopeComponents()
	value.Extend(input, ctx)
	return value
}
