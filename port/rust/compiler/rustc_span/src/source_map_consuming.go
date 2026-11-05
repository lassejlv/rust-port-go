// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// IntoCowPathOperation is borrowed metadata for a source Into<Cow<Path>>::into
// implementation. Into receives the complete owned Self, including its actual
// state cell. It may borrow, move, take, retain or forget Self. Captured owned
// fields and their complete destruction belong in the receiver's ValueOps;
// additional destructible captures must not be hidden in this Go function.
// Returned borrowed paths obey the original Rust lifetime/aliasing contract.
type IntoCowPathOperation[E any] struct {
	Into func(*rustvec.Owned[E], rustfmt.Context) CowPath
}

// ToRealFilenameConsuming is the general by-value conversion entry point for
// source_map.rs::FilePathMapping::to_real_filename. The existing *E conversion
// adapters are conveniences for implementations that retain Self for automatic
// destruction. This primary also permits legal manual consuming implementations.
func ToRealFilenameConsuming[E any](mapping *FilePathMapping, workingDirectory *RealFileName, receiver *rustvec.Owned[E], operation IntoCowPathOperation[E], context rustfmt.Context) RealFileName {
	if operation.Into == nil {
		context.Panic("rustc_span: missing consuming Into<Cow<Path>> operation")
	}
	path := rustvec.ConsumeValue(receiver, context, func(self *rustvec.Owned[E]) CowPath {
		return operation.Into(self, context)
	})
	// Complete conversion/remaining-Self cleanup finishes before any mapping or
	// tracing. The original body clones local_path into its actual filename
	// fields; transferring the transparent RustPath view adds no clone here.
	return mapping.ToRealFilenameWithContext(workingDirectory, path.Path, context)
}
