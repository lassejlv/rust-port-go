package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func gzipFixture(t *testing.T, value []byte, level int) []byte {
	t.Helper()
	var output bytes.Buffer
	writer, err := gzip.NewWriterLevel(&output, level)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write(value); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}
func decodedFixture(t *testing.T, compressed []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	result, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestOracleFixturePreservesValidEncoding(t *testing.T) {
	oracle := bytes.Repeat([]byte("pinned original Rust output\n"), 64)
	// NoCompression provides a different valid representation independent of the
	// Go version's BestCompression implementation.
	existing := gzipFixture(t, oracle, gzip.NoCompression)
	path := filepath.Join(t.TempDir(), "oracle.gz")
	if err := os.WriteFile(path, existing, 0644); err != nil {
		t.Fatal(err)
	}
	for _, check := range []bool{false, true} {
		result, err := oracleFixture(path, oracle, check)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(result, existing) {
			t.Fatal("valid committed encoding was recompressed")
		}
	}
}
func TestOracleFixtureRejectsAndRepairsBadStreams(t *testing.T) {
	oracle := []byte("exact original Rust output")
	valid := gzipFixture(t, oracle, gzip.BestCompression)
	badCRC := bytes.Clone(valid)
	badCRC[len(badCRC)-8] ^= 1
	for name, existing := range map[string][]byte{
		"invalid header":    []byte("invalid gzip"),
		"bad CRC":           badCRC,
		"truncated trailer": valid[:len(valid)-1],
		"different payload": gzipFixture(t, []byte("other Rust output"), gzip.BestCompression),
		"oversized payload": gzipFixture(t, bytes.Repeat(oracle, 10000), gzip.BestCompression),
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "oracle.gz")
			if err := os.WriteFile(path, existing, 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := oracleFixture(path, oracle, true); err == nil {
				t.Fatal("check accepted a nonmatching or corrupt stream")
			}
			repaired, err := oracleFixture(path, oracle, false)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(decodedFixture(t, repaired), oracle) {
				t.Fatal("generation did not rebuild the exact oracle payload")
			}
		})
	}
}
func TestOracleFixtureMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.gz")
	if _, err := oracleFixture(path, []byte("oracle"), true); err == nil {
		t.Fatal("check silently accepted a missing fixture")
	}
	generated, err := oracleFixture(path, []byte("oracle"), false)
	if err != nil {
		t.Fatal(err)
	}
	if string(decodedFixture(t, generated)) != "oracle" {
		t.Fatal("new fixture content differs")
	}
}
