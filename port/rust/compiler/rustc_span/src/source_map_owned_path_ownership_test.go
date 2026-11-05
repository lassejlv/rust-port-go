// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"testing"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
)

func ownedPathMustPanic(t *testing.T, operation func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("consumed owner remained usable")
		}
	}()
	operation()
}
func TestOwnedPathMovesPreserveBufferAndConsumeAliases(t *testing.T) {
	cx := rustfmt.Context{}
	for _, text := range []string{"", "a", "β", "/some/file.rs"} {
		s := ruststr.NewOwnedStringWithCapacity(53, cx)
		s.Borrow().PushString(text)
		cell := s.Borrow()
		before := cell.BufferIdentity()
		alias := *s
		os := RustOsStringFromOwnedString(s, RustPathUnix, cx)
		if os.Borrow().BufferIdentity() != before || os.Borrow().Capacity() != 53 {
			t.Fatal("String buffer/capacity not moved")
		}
		ownedPathMustPanic(t, func() { alias.Borrow() })
		ownedPathMustPanic(t, func() { cell.AsStr() })
		path := RustPathBufFromOsString(os, cx)
		ownedPathMustPanic(t, func() { os.Borrow() })
		borrow := path.AsPath()
		if len(text) > 0 && unsafe.StringData(borrow.raw) != before {
			t.Fatal("Path borrow copied bytes")
		}
		pathAlias := *path
		moved := path.MoveWithContext(cx)
		ownedPathMustPanic(t, func() { pathAlias.Borrow() })
		if moved.Borrow().BufferIdentity() != before || moved.AsPath().raw != text {
			t.Fatal("PathBuf move changed allocation")
		}
		s2, failure := moved.IntoOsStringWithContext(cx).IntoStringWithContext(cx)
		if failure != nil || s2.Borrow().BufferIdentity() != before || s2.Borrow().Capacity() != 53 {
			t.Fatal("OsString into_string lost allocation")
		}
		s2.Close()
		ownedPathMustPanic(t, func() { s2.Borrow() })
	}
}
func TestOwnedPathCowCloneFromTransitionsAndUniqueMutation(t *testing.T) {
	cx := rustfmt.Context{}
	for _, destOwned := range []bool{false, true} {
		for _, sourceOwned := range []bool{false, true} {
			var dest, source *RustCowPath
			if destOwned {
				p := NewRustPathBufWithCapacity(64, RustPathUnix, cx)
				p.Borrow().AsMutOsString().Push(RustPathFromUnixBytes([]byte("old")))
				dest = OwnedRustCowPath(p, cx)
			} else {
				dest = BorrowedRustCowPath(RustPathFromUnixBytes([]byte("old")))
			}
			if sourceOwned {
				p := NewRustPathBufWithCapacity(37, RustPathUnix, cx)
				p.Borrow().AsMutOsString().Push(RustPathFromUnixBytes([]byte("new")))
				source = OwnedRustCowPath(p, cx)
			} else {
				source = BorrowedRustCowPath(RustPathFromUnixBytes([]byte("new")))
			}
			dest.CloneFromWithContext(source, cx)
			if dest.IsOwned() != sourceOwned || dest.AsPath().raw != "new" {
				t.Fatal("Cow<Path> clone_from branch")
			}
			if destOwned && sourceOwned && dest.ToMutWithContext(cx).Capacity() != 64 {
				t.Fatal("owned Cow clone_from discarded spare capacity")
			}
			dest.ToMutWithContext(cx).AsMutOsString().Push(RustPathFromUnixBytes([]byte("!")))
			if source.AsPath().raw != "new" {
				t.Fatal("Cow mutation changed source")
			}
			dest.Close()
			source.Close()
		}
	}
	for _, destOwned := range []bool{false, true} {
		for _, sourceOwned := range []bool{false, true} {
			makeCow := func(text string, owned bool, capacity uint) *RustCowString {
				if !owned {
					return BorrowedRustCowString(text)
				}
				s := ruststr.NewOwnedStringWithCapacity(capacity, cx)
				s.Borrow().PushString(text)
				return OwnedRustCowString(s, cx)
			}
			dest, source := makeCow("old", destOwned, 64), makeCow("new", sourceOwned, 37)
			dest.CloneFromWithContext(source, cx)
			if dest.IsOwned() != sourceOwned || dest.String() != "new" {
				t.Fatal("Cow<str> clone_from branch")
			}
			if destOwned && sourceOwned && dest.ToMutWithContext(cx).Capacity() != 64 {
				t.Fatal("owned String Cow clone_from discarded spare capacity")
			}
			dest.ToMutWithContext(cx).PushString("!")
			if source.String() != "new" {
				t.Fatal("String Cow mutation changed source")
			}
			dest.Close()
			source.Close()
		}
	}
}
func TestOwnedPathInvalidUnicodeReturnsSameOwnerAllocation(t *testing.T) {
	cx := rustfmt.Context{}
	for _, value := range []*RustOsString{RustOsStringFromUnixBuffer(append(make([]byte, 0, 41), 'x', 0xff)), RustOsStringFromUTF16([]uint16{0xd800}, cx)} {
		before, capacity := value.Borrow().BufferIdentity(), value.Borrow().Capacity()
		s, failure := value.IntoStringWithContext(cx)
		if s != nil || failure == nil || failure.Borrow().BufferIdentity() != before || failure.Borrow().Capacity() != capacity {
			t.Fatal("invalid into_string did not return original buffer")
		}
		ownedPathMustPanic(t, func() { value.Borrow() })
		failure.Close()
	}
}

func TestOwnedPathPanicUsesCurrentOperationContext(t *testing.T) {
	path := RustPathBufFromPath(RustPathFromUnixBytes([]byte("file.old")), rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "construction"}})
	defer path.Close()
	for _, extension := range []bool{false, true} {
		calls := 0
		location := rustfmt.SourceLocation{File: "selected/native/source", Line: 17, Column: 3}
		operations := &rustfmt.PanicOperations{Locations: map[string]rustfmt.SourceLocation{OwnedPathCapacityPanicSite: location, OwnedPathExtensionPanicSite: location}}
		operations.Hook = func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
			calls++
			if current.Runtime.Value != "operation" || !current.ProcessingPanic || info.Location == nil || *info.Location != location {
				t.Fatal("source panic captured stale context/location")
			}
			kind := rustfmt.PanicStaticStr
			if extension {
				kind = rustfmt.PanicOwnedString
			}
			if info.Kind != kind {
				t.Fatal("source panic payload kind changed")
			}
		}
		context := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: "operation", Panic: operations}}
		before, capacity := path.AsPath().raw, path.Borrow().Capacity()
		ownedPathMustPanic(t, func() {
			if extension {
				path.Borrow().SetExtensionWithContext(RustPathFromUnixBytes([]byte("/bad")), context)
			} else {
				path.Borrow().AsMutOsString().ReserveWithContext(^uint(0), context)
			}
		})
		if calls != 1 || path.AsPath().raw != before || path.Borrow().Capacity() != capacity {
			t.Fatal("source panic changed path before validation")
		}
	}
}
