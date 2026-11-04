package main

import (
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func encodeTestFixture(t *testing.T, payload []byte) []byte {
	t.Helper()
	var encoded bytes.Buffer
	writer, err := gzip.NewWriterLevel(&encoded, gzip.BestSpeed)
	if err != nil {
		t.Fatal(err)
	}
	writer.Name = "an existing encoding from another Go release"
	if _, err := writer.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return encoded.Bytes()
}

func decodeTestFixture(t *testing.T, encoded []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	payload, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestOracleFixturePreservesExistingEncoding(t *testing.T) {
	payload := bytes.Repeat([]byte("exact Rust Unicode reference\n"), 100)
	existing := encodeTestFixture(t, payload)
	path := filepath.Join(t.TempDir(), "oracle.bin.gz")
	if err := os.WriteFile(path, existing, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, check := range []bool{true, false} {
		got, err := oracleFixture(path, payload, check)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, existing) {
			t.Fatalf("check=%t rewrote an existing exact-payload gzip stream", check)
		}
	}
}

func TestOracleFixtureRejectsAndRebuildsInvalidReference(t *testing.T) {
	payload := bytes.Repeat([]byte("exact Rust Unicode reference\n"), 100)
	valid := encodeTestFixture(t, payload)
	badCRC := append([]byte(nil), valid...)
	badCRC[len(badCRC)-8] ^= 0xff
	cases := []struct {
		name    string
		encoded []byte
	}{
		{"different_payload", encodeTestFixture(t, append(append([]byte(nil), payload...), 'x'))},
		{"bad_crc", badCRC},
		{"truncated", valid[:len(valid)-1]},
		{"invalid_gzip", []byte("not gzip")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "oracle.bin.gz")
			if err := os.WriteFile(path, tc.encoded, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := oracleFixture(path, payload, true); err == nil {
				t.Fatal("check accepted an invalid or different oracle payload")
			}
			rebuilt, err := oracleFixture(path, payload, false)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(decodeTestFixture(t, rebuilt), payload) {
				t.Fatal("regeneration did not rebuild the exact oracle payload")
			}
		})
	}
}

func TestOracleFixtureCreatesMissingReference(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.bin.gz")
	payload := []byte("new oracle")
	if _, err := oracleFixture(path, payload, true); err == nil {
		t.Fatal("check accepted a missing fixture")
	}
	created, err := oracleFixture(path, payload, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decodeTestFixture(t, created), payload) {
		t.Fatal("new fixture does not contain the exact oracle")
	}
}
