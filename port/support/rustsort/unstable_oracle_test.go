// SPDX-License-Identifier: MIT OR Apache-2.0
package rustsort

import "testing"

func TestOriginalUnstableSort(t *testing.T) {
	replayOriginalSort(t, "unstable_manifest.json", "unstable.tsv.gz", "unstable_oracle.rs", "generate_unstable.py", "UNSTABLE-SOURCES.json", Unstable[sortValue], true)
}
