// Mirrors all workloads in semver 1.0.28 benches/parse.rs.
package semver_test

import (
	"testing"

	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
)

var prereleaseSink semver.Prerelease
var versionSink semver.Version
var requirementSink semver.VersionReq

var originalBenchmarks = []struct {
	name string
	text string
}{
	{"parse Prerelease", "x.7.z.92"},
	{"parse Version", "1.0.2021-beta+exp.sha.5114f85"},
	{"parse VersionReq", ">=1.2.3, <2.0.0"},
}

func parseBenchmark(index int, text string) error {
	var err error
	switch index {
	case 0:
		prereleaseSink, err = semver.NewPrerelease(text)
	case 1:
		versionSink, err = semver.ParseVersion(text)
	case 2:
		requirementSink, err = semver.ParseVersionReq(text)
	default:
		panic("missing original benchmark")
	}
	return err
}

func TestOriginalBenchmarkInputs(t *testing.T) {
	for i, benchmark := range originalBenchmarks {
		if err := parseBenchmark(i, benchmark.text); err != nil {
			t.Fatal(err)
		}
	}
}

func BenchmarkParse(b *testing.B) {
	for index, benchmark := range originalBenchmarks {
		b.Run(benchmark.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if err := parseBenchmark(index, benchmark.text); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
