// SPDX-License-Identifier: MIT OR Apache-2.0
// Inherited concrete PartialEq::ne and Copy Clone::clone_from from pinned lib.rs.
package rustcspan

func (value RemapPathScopeComponents) NotEqual(other RemapPathScopeComponents) bool {
	return !value.Equal(other)
}
func (value innerRealFileName) NotEqual(other innerRealFileName) bool   { return !value.Equal(other) }
func (value RealFileName) NotEqual(other RealFileName) bool             { return !value.Equal(other) }
func (value FileName) NotEqual(other FileName) bool                     { return !value.Equal(other) }
func (value SpanData) NotEqual(other SpanData) bool                     { return !value.Equal(other) }
func (value MultiByteChar) NotEqual(other MultiByteChar) bool           { return !value.Equal(other) }
func (value NormalizedPos) NotEqual(other NormalizedPos) bool           { return !value.Equal(other) }
func (value ExternalSourceKind) NotEqual(other ExternalSourceKind) bool { return !value.Equal(other) }
func (value ExternalSource) NotEqual(other ExternalSource) bool         { return !value.Equal(other) }
func (value SourceFileHashAlgorithm) NotEqual(other SourceFileHashAlgorithm) bool {
	return !value.Equal(other)
}
func (value SourceFileHash) NotEqual(other SourceFileHash) bool         { return !value.Equal(other) }
func (value StableSourceFileId) NotEqual(other StableSourceFileId) bool { return !value.Equal(other) }
func (value BytePos) NotEqual(other BytePos) bool                       { return !value.Equal(other) }
func (value RelativeBytePos) NotEqual(other RelativeBytePos) bool       { return !value.Equal(other) }
func (value CharPos) NotEqual(other CharPos) bool                       { return !value.Equal(other) }
func (value LineInfo) NotEqual(other LineInfo) bool                     { return !value.Equal(other) }
func (value DistinctSources) NotEqual(other DistinctSources) bool       { return !value.Equal(other) }
func (value MalformedSourceMapPositions) NotEqual(other MalformedSourceMapPositions) bool {
	return !value.Equal(other)
}
func (value InnerSpan) NotEqual(other InnerSpan) bool                 { return !value.Equal(other) }
func (value *SpanLinesError) NotEqual(other *SpanLinesError) bool     { return !value.Equal(other) }
func (value *SpanSnippetError) NotEqual(other *SpanSnippetError) bool { return !value.Equal(other) }
func (value *MultiByteChar) CloneFrom(other MultiByteChar)            { *value = other.Clone() }
func (value *NormalizedPos) CloneFrom(other NormalizedPos)            { *value = other.Clone() }
func (value *RemapPathScopeComponents) CloneFrom(other RemapPathScopeComponents) {
	*value = other.Clone()
}
func (value *innerRealFileName) CloneFrom(other innerRealFileName) { *value = other.Clone() }
