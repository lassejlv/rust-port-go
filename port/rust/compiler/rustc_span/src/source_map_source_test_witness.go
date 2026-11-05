// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// SourceMapByteposToFileCharposSourceTestWitness forwards the private operation
// used by the original child tests. It borrows the actual SourceFile cell and
// uses the supplied current context/build profile without copying the algorithm.
// This Go-only child-test bridge grants no new Rust public API or file credit.
func SourceMapByteposToFileCharposSourceTestWitness(file *SourceFile, pos RelativeBytePos, context rustfmt.Context, config SpanEncodingConfig) CharPos {
	return file.byteposToFileCharpos(pos, context, config)
}

// SourceMapRealFileNameSourceTestWitness constructs the original tests' expected
// private fields directly. Its paths are explicit fixture values; expected
// values are never generated with the FilePathMapping implementation under test.
// It is a Go-only test bridge, not an additional Rust public constructor.
func SourceMapRealFileNameSourceTestWitness(localName, localWorkingDir, localEmbeddedName, remappedName, remappedWorkingDir, remappedEmbeddedName RustPath, scopes RemapPathScopeComponents) RealFileName {
	return RealFileName{
		local:         Some(innerRealFileName{name: localName, workingDirectory: localWorkingDir, embeddableName: localEmbeddedName}),
		maybeRemapped: innerRealFileName{name: remappedName, workingDirectory: remappedWorkingDir, embeddableName: remappedEmbeddedName},
		scopes:        scopes,
	}
}
