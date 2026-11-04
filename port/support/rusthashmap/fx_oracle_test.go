// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestOriginalFxHasher(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("retained actual Rust fixture selects the x86-64 usize/isize profile")
	}
	metadata, err := os.ReadFile("../../../tools/port-rust-unord/table/fx_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases     int    `json:"cases"`
		Raw       string `json:"raw_sha256"`
		Fixture   string `json:"fixture_sha256"`
		Oracle    string `json:"oracle_sha256"`
		Generator string `json:"generator_sha256"`
		Support   string `json:"support_generator_sha256"`
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{"../../../tools/port-rust-unord/table/fx_oracle.rs": manifest.Oracle, "../../../tools/port-rust-unord/table/fx_generate.py": manifest.Generator, "../../../tools/port-rust-unord/table/generate.py": manifest.Support} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("Fx reference binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/fx.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	if digest(compressed) != manifest.Fixture || manifest.Cases != 5548 {
		t.Fatal("Fx reference fixture/coverage differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(reader, 4<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("Fx reference body differs:%v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	texts := []string{"", "a", "\x00", "é", "中文", "🦀", "a\x00é🦀", "\n\r\t"}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	cases := 0
	for scanner.Scan() {
		cases++
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) != 5 {
			t.Fatalf("malformed Fx case%d", cases)
		}
		seed, _ := strconv.ParseUint(fields[0], 10, 64)
		n, _ := strconv.Atoi(fields[1])
		mode, _ := strconv.Atoi(fields[2])
		want, _ := strconv.ParseUint(fields[3], 10, 64)
		wantClone, _ := strconv.ParseUint(fields[4], 10, 64)
		data := make([]byte, n)
		for i := range data {
			data[i] = byte(seed + uint64(i)*131)
		}
		h := NewFxHasherProfile(64, seed)
		switch mode {
		case 0:
			h.Write(data)
		case 1:
			HashLengthPrefix(h, uint(n))
			h.Write(data)
		case 2:
			h.Write(data[:n/2])
			h.Write(data[n/2:])
		case 3:
			h.WriteU8(uint8(seed))
		case 4:
			h.WriteU16(uint16(seed))
		case 5:
			h.WriteU32(uint32(seed))
		case 6:
			h.WriteU64(seed)
		case 7:
			h.WriteU128(UInt128{uint64(n), seed})
		case 8:
			h.WriteUsize(uint(seed))
		case 9:
			HashI8(h, int8(seed))
		case 10:
			HashI16(h, int16(seed))
		case 11:
			HashI32(h, int32(seed))
		case 12:
			HashI64(h, int64(seed))
		case 13:
			HashI128(h, Int128{uint64(n), seed})
		case 14:
			HashIsize(h, int(seed))
		case 15:
			HashString(h, texts[n%len(texts)])
		case 16:
			h.WriteLengthPrefix(uint(n))
		case 17:
			h.Write(nil)
			h.WriteU64(seed)
			h.Write(data)
		case 18:
			h.WriteString(texts[n%len(texts)])
			h.WriteU128(UInt128{seed, uint64(n)})
		default:
			t.Fatal("unknown native Fx operation")
		}
		clone := NewFxHasher()
		clone.CloneFrom(h.Clone())
		clone.WriteU64(uint64(n))
		if got, gotClone := h.Finish64(), clone.Finish64(); got != want || gotClone != wantClone {
			t.Fatalf("Fx case%d (%s):got%d/%d want%d/%d", cases, strings.Join(fields[:3], "\t"), got, gotClone, want, wantClone)
		}
	}
	if err := scanner.Err(); err != nil || cases != manifest.Cases {
		t.Fatalf("Fx proof incomplete:%d:%v", cases, err)
	}
}
