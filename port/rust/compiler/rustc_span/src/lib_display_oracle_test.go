package rustcspan

import (
	"fmt"
	"testing"

	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestLibOriginalDisplayFormatters(t *testing.T) {
	real := RealFileNameFromVirtualPath(RustPathFromUnixBytes([]byte("/réel/\"file.rs")))
	names := []FileName{RealFileNameFileName(real), CfgSpecFileName(rustchashes.NewHash64(31)), AnonFileName(rustchashes.ZeroHash64()), MacroExpansionFileName(rustchashes.NewHash64(26)), ProcMacroSourceCodeFileName(rustchashes.NewHash64(10)), CliCrateAttrFileName(rustchashes.NewHash64(99)), CustomFileName("é\n\x00\U000323af"), DocTestSourceCode(real, -26), InlineAsmFileName(rustchashes.NewHash64(^uint64(0))), RealFileNameFileName(EmptyRealFileName()), CustomFileName(""), RealFileNameFileName(RealFileNameFromVirtualPath(RustPathFromUnixBytes([]byte{'/', 'r', 'a', 'w', '/', 255, '.', 'r', 's'})))}
	values := make(map[string]func(*rustfmt.Formatter) error)
	for i := range names {
		value := &names[i]
		for name, display := range map[string]FileNameDisplay{"remapped": value.PreferRemappedUnconditionally(), "local": value.PreferLocalUnconditionally(), "short": value.Short(), "macro": value.Display(RemapPathScopeMacro), "diagnostics": value.Display(RemapPathScopeDiagnostics), "coverage": value.Display(RemapPathScopeCoverage)} {
			values[fmt.Sprintf("file/%d/%s", i, name)] = display.WriteDisplayFormatter
		}
	}
	for i, kind := range []SourceFileHashAlgorithm{SourceFileHashMd5, SourceFileHashSha1, SourceFileHashSha256, SourceFileHashBlake3} {
		values[fmt.Sprintf("algorithm/%d", i)] = kind.WriteDisplayFormatter
		for j, data := range [][]byte{{}, []byte("source\x00\xff"), []byte("a")} {
			value := NewSourceFileHashInMemory(kind, data)
			values[fmt.Sprintf("hash/%d/%d", i, j)] = value.WriteDisplayFormatter
		}
	}
	libReplayOriginalFormatters(t, "testdata/lib-display.tsv.gz", values, 12504)
}
