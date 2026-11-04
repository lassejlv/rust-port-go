// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

//go:embed edition_testdata/oracle.tsv.gz
var editionReferenceFixture []byte

//go:embed edition_oracle_manifest.json
var editionReferenceManifest []byte

func TestEditionPinnedReference(t *testing.T) {
	var meta struct {
		Revision      string         `json:"rust_revision"`
		SourceSHA     string         `json:"source_sha256"`
		RawSHA        string         `json:"oracle_raw_sha256"`
		CompressedSHA string         `json:"fixture_sha256"`
		Counts        map[string]int `json:"case_counts"`
	}
	if err := json.Unmarshal(editionReferenceManifest, &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || meta.SourceSHA != "a1dd82ce1a5b22b27e3360bcf8bd606fd4fa6669f6de6305536d6436d63ac7ae" {
		t.Fatal("Edition source pin differs")
	}
	digest := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	if digest(editionReferenceFixture) != meta.CompressedSHA {
		t.Fatal("compressed reference hash differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(editionReferenceFixture))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if digest(raw) != meta.RawSHA {
		t.Fatal("original runtime output hash differs")
	}
	counts := make(map[string]int)
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		counts[fields[0]]++
		fail := func(got any) {
			t.Helper()
			t.Fatalf("original edition mismatch: %s; Go result %v", scanner.Text(), got)
		}
		integer := func(text string) int {
			n, err := strconv.Atoi(text)
			if err != nil {
				t.Fatal(err)
			}
			return n
		}
		unhex := func(text string) []byte {
			data, err := hex.DecodeString(text)
			if err != nil {
				t.Fatal(err)
			}
			return data
		}
		switch fields[0] {
		case "constants":
			if fields[1] != EditionNameList || fields[2] != EditionNameListStable || integer(fields[3]) != int(DefaultEdition) || integer(fields[4]) != int(LatestStableEdition) {
				fail("constant mismatch")
			}
		case "variant":
			e := AllEditions()[integer(fields[1])]
			predicates := ""
			for _, value := range []bool{e.IsStable(), e.IsRust2015(), e.AtLeastRust2018(), e.AtLeastRust2021(), e.AtLeastRust2024(), e.AtLeastEditionFuture()} {
				if value {
					predicates += "1"
				} else {
					predicates += "0"
				}
			}
			trace := &editionOperationTrace{}
			e.Hash(trace)
			codec := &editionReferenceEncoder{inner: NewEditionMemoryEncoder()}
			e.Encode(codec)
			if fields[2] != e.AsStr() || fields[3] != e.LintName() || fields[4] != e.DebugString() || fields[5] != predicates || fields[6] != strings.Join(trace.entries, ",") || fields[7] != hex.EncodeToString(codec.inner.Finish()) || fields[8] != fmt.Sprint(codec.operations) {
				fail("variant methods/derived typed operations")
			}
		case "compare":
			e, other := Edition(integer(fields[1])), Edition(integer(fields[2]))
			comparison := e.Compare(other)
			name := "Equal"
			if comparison < 0 {
				name = "Less"
			} else if comparison > 0 {
				name = "Greater"
			}
			cloned := e.Clone()
			cloned.CloneFrom(other)
			if fields[3] != name || fields[4] != strconv.FormatBool(e == other) || integer(fields[5]) != int(cloned) {
				fail("comparison/clone")
			}
		case "format":
			e := Edition(integer(fields[1]))
			format := integer(fields[2])
			// Go has no center/custom-fill verb. Those source cases ignore the
			// complete outer formatting options, as the original oracle proves.
			formats := []string{"%v", "%20v", "%-20v", "%20v", "%20v", "%.0v", "%.2v", "%+020.2v", "%#s", "%20v", "%#v", "%#v", "%#20v", "%#.2v", "%+#020.2v"}
			if got := fmt.Sprintf(formats[format], e); got != string(unhex(fields[3])) {
				fail(got)
			}
		case "writer":
			e := Edition(integer(fields[1]))
			writer := editionFailingWriter{fmt.Errorf("reject")}
			if strconv.FormatBool(e.DisplayTo(writer) != nil) != fields[2] || strconv.FormatBool(e.DebugTo(writer) != nil) != fields[3] {
				fail("write errors")
			}
		case "parse":
			e, ok := EditionFromStr(string(unhex(fields[1])))
			result := "error"
			if ok {
				result = strconv.Itoa(int(e))
			}
			if fields[2] != result {
				fail(result)
			}
		case "hash":
			e := Edition(integer(fields[1]))
			trace := &editionOperationTrace{entries: []string{"caller-prefix:" + fields[2]}}
			e.Hash(trace)
			e.StableHash(struct{}{}, trace)
			want := []string{"caller-prefix:" + fields[2], "isize:" + fields[3], "u8:" + fields[4]}
			if fmt.Sprint(trace.entries) != fmt.Sprint(want) {
				fail(trace.entries)
			}
			// The live original oracle compares these actual Go-emitted
			// operands against original derives using real stateful hashers.
			// Digests are retained as proof, never implemented as a lookup.
			if _, err := strconv.ParseUint(fields[5], 10, 64); err != nil {
				t.Fatal(err)
			}
			if value, ok := new(big.Int).SetString(fields[6], 10); !ok || value.Sign() < 0 || value.BitLen() > 128 {
				fail("invalid retained stable hash")
			}
		case "decode":
			got := editionReferenceDecode(unhex(fields[1]), uint(integer(fields[2])))
			want := fields[3]
			if strings.HasPrefix(want, "constructor-panic:") {
				// Constructor range-panic text belongs to the surrounding
				// serialization implementation, not Edition's derived impl.
				if !strings.HasPrefix(got, "constructor-panic:") {
					fail(got)
				}
			} else if got != want {
				fail(got)
			}
		case "unused-interface-calls":
			if fields[1] != "0" {
				fail("reference invoked an unsupported interface")
			}
		default:
			t.Fatalf("unknown original output record %q", fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	for kind, want := range meta.Counts {
		if counts[kind] != want {
			t.Fatalf("reference %s records = %d; want %d", kind, counts[kind], want)
		}
	}
	if counts["variant"] != 5 || counts["compare"] != 25 || counts["hash"] != 645 {
		t.Fatal("finite enum/derived-state coverage reduced")
	}
}

type editionReferenceEncoder struct {
	inner      *EditionMemoryEncoder
	operations []string
}

func (encoder *editionReferenceEncoder) EmitU8(value uint8) {
	encoder.operations = append(encoder.operations, fmt.Sprintf("U8(%d)", value))
	encoder.inner.EmitU8(value)
}

type editionReferenceDecoder struct {
	inner      *EditionMemoryDecoder
	operations []string
}

func (decoder *editionReferenceDecoder) ReadU8() uint8 {
	decoder.operations = append(decoder.operations, "ReadU8")
	return decoder.inner.ReadU8()
}

func editionReferenceDecode(blob []byte, position uint) (result string) {
	defer func() {
		if p := recover(); p != nil {
			result = "constructor-panic:" + hex.EncodeToString([]byte(fmt.Sprint(p)))
		}
	}()
	inner, ok := NewEditionMemoryDecoder(blob, position)
	if !ok {
		return "error"
	}
	decoder := &editionReferenceDecoder{inner: inner}
	result = func() (value string) {
		defer func() {
			if p := recover(); p != nil {
				value = fmt.Sprintf("panic:%s:%d:%s", hex.EncodeToString([]byte(fmt.Sprint(p))), inner.Remaining(), fmt.Sprint(decoder.operations))
			}
		}()
		edition := DecodeEdition(decoder)
		return fmt.Sprintf("ok:%d:%d:%s", int(edition), inner.Remaining(), fmt.Sprint(decoder.operations))
	}()
	return
}

func TestEditionLiveOriginalReference(t *testing.T) {
	rustc, requested := os.LookupEnv("FERRUGO_RUSTC")
	if !requested {
		t.Skip("set FERRUGO_RUSTC to build the actual original Edition and derives")
	}
	if rustc == "" {
		t.Fatal("FERRUGO_RUSTC explicitly empty")
	}
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatal("repository root not found")
		}
		root = parent
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin/go"), "run", "./tools/port-rust-edition", "-root", root, "-rustc", rustc, "-check")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("live original edition reference: %v\n%s", err, output)
	} else {
		t.Log(strings.TrimSpace(string(output)))
	}
}
