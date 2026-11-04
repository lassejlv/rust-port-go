//go:build !semver_no_std

package semver

// FeatureStd identifies the source's std feature. Source maps std::error::Error
// chaining; Error() itself maps Display in every profile, as Go requires.
const FeatureStd = true

func (err *Error) Source() error { return nil }
