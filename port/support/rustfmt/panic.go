// SPDX-License-Identifier: MIT OR Apache-2.0
// Explicit used panic-operation contracts. This is neither a Go-global panic
// interceptor nor a claim to port the standard-library panic runtime.
package rustfmt

import "os"

type PanicPayloadKind string

const (
	PanicStaticStr   PanicPayloadKind = "static_str"
	PanicOwnedString PanicPayloadKind = "owned_string"
	PanicOpaque      PanicPayloadKind = "opaque"
)

// SourceLocation is the actual selected Rust operation's location. The source
// adapter supplies it explicitly, including tracked callers when applicable;
// no Go stack coordinates or fabricated Rust locations are inferred.
type SourceLocation struct {
	File         string
	Line, Column uint32
}

// SourcePanicInfo borrows the actual payload and source metadata for one hook
// invocation. Kind represents Rust's dynamic Any type independently of Go's
// representation (both Rust string variants can use a Go string). The value
// passed to recover remains exactly Payload; metadata never wraps that value.
type SourcePanicInfo struct {
	Payload          any
	Kind             PanicPayloadKind
	Location         *SourceLocation
	CanUnwind        bool
	ForceNoBacktrace bool
}

// PanicOperations is borrowed operation metadata, with no implicit owning
// destructor or captured construction-time caller. Hook is a complete borrowed
// callback witness: contextual source panic sites in its body use the supplied
// Context. A raw Go panic retains Go-local defer timing before the escape guard
// aborts. CleanupAbort must identify the ACTUAL selected compiler/std profile
// before a cleanup diagnostic can be emitted; nil never invents such an event.
type PanicOperations struct {
	Hook         func(SourcePanicInfo, Context)
	CleanupAbort *SourcePanicInfo
	// Locations is caller-owned immutable profile metadata, keyed by semantic
	// source site IDs. File spellings are supplied verbatim, never normalized.
	Locations map[string]SourceLocation
	// LocationCells is the primary borrowed immutable source-cell profile.
	// Entries retain their actual identity through hook invocation. An explicit
	// nil entry is authoritative; absent keys fall back to Locations. Callers
	// preserve these cells and their immutable data for the operation lifetime.
	LocationCells map[string]*SourceLocation
}

func WithPanicOperations(context Context, operations *PanicOperations) Context {
	if context.Runtime != nil && context.Runtime.Panic == operations {
		return context
	}
	metadata := RuntimeMetadata{}
	if context.Runtime != nil {
		metadata = *context.Runtime
	}
	metadata.Panic = operations
	context.Runtime = &metadata
	return context
}

func (c Context) panicOperations() *PanicOperations {
	if c.Runtime == nil {
		return nil
	}
	return c.Runtime.Panic
}

// SourcePanicLocation resolves a selected-profile source site. Primary cells
// preserve identity, including explicit nil. The Locations convenience returns
// scalar copies and grants no source-cell identity. Missing sites stay nil;
// neither route infers filenames or grants mutable access to profile metadata.
func (c Context) SourcePanicLocation(site string) *SourceLocation {
	ops := c.panicOperations()
	if ops == nil {
		return nil
	}
	if location, ok := ops.LocationCells[site]; ok {
		return location
	}
	location, ok := ops.Locations[site]
	if !ok {
		return nil
	}
	return &location
}

func (c Context) processingContext() Context {
	c.ProcessingPanic = true
	c.Unwinding = true
	return c
}

func (c Context) invokePanicHook(info SourcePanicInfo) {
	ops := c.panicOperations()
	if ops == nil || ops.Hook == nil {
		return
	}
	defer func() {
		if recover() != nil {
			os.Exit(134)
		}
	}()
	ops.Hook(info, c.processingContext())
}

// PanicAt is an ordinary source raise: the hook runs before local unwinding.
// A panic while already processing a panic aborts before invoking another hook
// or unwinding callback locals. An un-unwindable source panic aborts after its
// hook returns. Ambient Unwinding alone remains a catchable nested operation.
func (c Context) PanicAt(info SourcePanicInfo) {
	if c.ProcessingPanic {
		os.Exit(134)
	}
	c.invokePanicHook(info)
	if !info.CanUnwind {
		os.Exit(134)
	}
	panic(info.Payload)
}

// PanicFormattedAt starts processing BEFORE rendering the panic payload. This
// preserves expect/assert formatting and local destruction before the hook,
// including immediate abort if a translated Debug operation panics. Render is
// borrowed static operation metadata, not an invented consumed FnOnce value.
func (c Context) PanicFormattedAt(info SourcePanicInfo, render func(Context) any) {
	if c.ProcessingPanic {
		os.Exit(134)
	}
	info.Payload = func() (payload any) {
		defer func() {
			if recover() != nil {
				os.Exit(134)
			}
		}()
		return render(c.processingContext())
	}()
	c.PanicAt(info)
}

// ResumeUnwind propagates an already raised payload without dispatching the
// hook again. Source recover/cleanup forwarding paths use this operation.
func (c Context) ResumeUnwind(payload any) {
	if c.ProcessingPanic {
		os.Exit(134)
	}
	panic(payload)
}

// AbortPanicAt preserves a real hook-then-abort operation. Its metadata must be
// the observed selected runtime operation; this helper invents no diagnostic.
func (c Context) AbortPanicAt(info SourcePanicInfo) {
	if c.ProcessingPanic {
		os.Exit(134)
	}
	c.invokePanicHook(info)
	os.Exit(134)
}

// AbortCleanup is used only at a proven compiler-generated cleanup-abort
// boundary after a second panic escapes required cleanup. The actual boundary
// owns the diagnostic; a callback witness must not emit that same event too.
func (c Context) AbortCleanup() {
	ops := c.panicOperations()
	if ops != nil && ops.CleanupAbort != nil {
		c.AbortPanicAt(*ops.CleanupAbort)
	}
	os.Exit(134)
}

// CleanupPanicInfoNightly20261104 is bound to the exact official nightly
// 1.101.0 (708c94ef093ecc42fb8851e17d8383d999727abc), independently observed by
// tools/port-rust-panic-runtime/prototype.rs. It is not a version-agnostic
// runtime location. Other source profiles supply their actual metadata.
func CleanupPanicInfoNightly20261104() SourcePanicInfo {
	return SourcePanicInfo{
		Payload: "panic in a destructor during cleanup", Kind: PanicStaticStr,
		Location:  &SourceLocation{File: "/rustc/708c94ef093ecc42fb8851e17d8383d999727abc/library/core/src/panicking.rs", Line: 233, Column: 5},
		CanUnwind: false, ForceNoBacktrace: true,
	}
}
