// SPDX-License-Identifier: MIT OR Apache-2.0
package rustchashes

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
)

type hashOracleManifest struct {
	RawSHA     string `json:"raw_sha256"`
	FixtureSHA string `json:"fixture_sha256"`
	Records    int    `json:"records"`
	Formats    []struct {
		Pattern  string
		Options  FormatOptions
		LowerHex bool
	}
}

func oracleU64(t *testing.T, text string) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func oracleU128(t *testing.T, text string) Uint128 {
	t.Helper()
	v, ok := new(big.Int).SetString(text, 10)
	if !ok || v.Sign() < 0 || v.BitLen() > 128 {
		t.Fatalf("invalid u128 %q", text)
	}
	lo := v.Uint64()
	hi := new(big.Int).Rsh(v, 64).Uint64()
	return Uint128{Lo: lo, Hi: hi}
}

type recordHasher struct{ words []uint64 }

func (h *recordHasher) WriteU64(v uint64) { h.words = append(h.words, v) }

type recordingSink struct {
	events []string
	fail   int
}

var errSink = errors.New("writer failed")

func (s *recordingSink) Write(p []byte) (int, error) {
	n := len(s.events)
	s.events = append(s.events, fmt.Sprintf("S:%d:%x", len(p), p))
	if n == s.fail {
		return 0, errSink
	}
	return len(p), nil
}

type charRecordingSink struct{ *recordingSink }

func (s charRecordingSink) WriteChar(ch rune) error {
	n := len(s.events)
	text := []byte(string(ch))
	s.events = append(s.events, fmt.Sprintf("C:%d:%x", len(text), text))
	if n == s.fail {
		return errSink
	}
	return nil
}

func TestOriginalHashOracle(t *testing.T) {
	data, err := os.ReadFile("testdata/oracle.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile("oracle_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest hashOracleManifest
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	checkHash := func(data []byte, want string) {
		t.Helper()
		h := sha256.Sum256(data)
		if hex.EncodeToString(h[:]) != want {
			t.Fatal("retained fixture checksum mismatch")
		}
	}
	checkHash(data, manifest.FixtureSHA)
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(gz, 128<<20))
	if err != nil {
		t.Fatal(err)
	}
	if err = gz.Close(); err != nil {
		t.Fatal(err)
	}
	checkHash(raw, manifest.RawSHA)
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count := 0
	for scanner.Scan() {
		count++
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) < 5 {
			t.Fatalf("record %d malformed", count)
		}
		lo, hi := oracleU64(t, fields[1]), oracleU64(t, fields[2])
		a := NewHash64(lo)
		a128 := NewHash128(Uint128{Lo: lo, Hi: hi})
		switch fields[0] {
		case "V":
			if len(fields) != 14 {
				t.Fatalf("record %d has %d fields", count, len(fields))
			}
			b := NewHash64(oracleU64(t, fields[3]))
			b128 := NewHash128(Uint128{Lo: b.AsU64(), Hi: oracleU64(t, fields[4])})
			xor := a
			xor.XorAssign(b.AsU64())
			compare := func(v int) string {
				if v < 0 {
					return "Less"
				}
				if v > 0 {
					return "Greater"
				}
				return "Equal"
			}
			if a.AsU64() != oracleU64(t, fields[5]) || a.WrappingAdd(b).AsU64() != oracleU64(t, fields[6]) || xor.AsU64() != oracleU64(t, fields[7]) || compare(a.Compare(b)) != fields[8] || a128.AsU128() != oracleU128(t, fields[9]) || a128.WrappingAdd(b128).AsU128() != oracleU128(t, fields[10]) || compare(a128.Compare(b128)) != fields[11] || a128.Truncate().AsU64() != oracleU64(t, fields[12]) {
				t.Fatalf("arithmetic/order mismatch record %d: %s", count, scanner.Text())
			}
			var hasher recordHasher
			a.Hash(&hasher)
			a128.Hash(&hasher)
			trace := fmt.Sprintf("%d,%d", hasher.words[0], hasher.words[1])
			if len(hasher.words) != 2 || trace != fields[13] {
				t.Fatalf("typed hash mismatch record %d", count)
			}
			var clone Hash64
			clone.CloneFrom(a)
			var clone128 Hash128
			clone128.CloneFrom(a128)
			if clone != a.Clone() || clone128 != a128.Clone() || ZeroHash64() != (Hash64{}) || ZeroHash128() != (Hash128{}) || Hash64FromStableHash(StableHasherHash{lo, hi}) != a || Hash128FromStableHash(StableHasherHash{lo, hi}) != a128 {
				t.Fatalf("clone/default/conversion mismatch record %d", count)
			}
		case "F":
			if len(fields) != 10 {
				t.Fatalf("format record %d malformed", count)
			}
			index, err := strconv.Atoi(fields[3])
			if err != nil || index < 0 || index >= len(manifest.Formats) {
				t.Fatalf("bad format index %q", fields[3])
			}
			fail, err := strconv.Atoi(fields[6])
			if err != nil {
				t.Fatal(err)
			}
			sink := &recordingSink{fail: fail}
			pattern := manifest.Formats[index]
			var writer io.Writer = sink
			if fields[5] == "true" {
				writer = charRecordingSink{sink}
			}
			f := NumericFormatter{Writer: writer, Options: pattern.Options}
			var value FormatterValue = a
			if fields[4] == "128" {
				value = a128
			}
			if pattern.LowerHex {
				err = value.WriteLowerHex(&f)
			} else {
				err = value.WriteDebug(&f)
			}
			got := strings.Join(sink.events, "|")
			fill := f.Options.Fill
			if fill == 0 && !f.Options.FillSet {
				fill = ' '
			}
			state := fmt.Sprintf("%d:%d", fill, f.Options.Alignment)
			if strconv.FormatBool(err == nil) != fields[7] || got != fields[8] || state != fields[9] {
				t.Fatalf("format mismatch record %d (%s, bits%s, fail%d, char%s):\ngot %v %s state%s\nwant %s %s state%s", count, pattern.Pattern, fields[4], fail, fields[5], err, got, state, fields[7], fields[8], fields[9])
			}
		default:
			t.Fatalf("unknown operation %q", fields[0])
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != manifest.Records {
		t.Fatalf("read %d records, want %d", count, manifest.Records)
	}
}
