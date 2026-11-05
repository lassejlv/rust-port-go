// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type fingerprintDecodeMetadataManifest struct {
	Records      int
	ActualAborts int    `json:"actual_aborts"`
	FixtureSHA   string `json:"fixture_sha256"`
	RawSHA       string `json:"raw_sha256"`
	Locations    map[string]rustfmt.SourceLocation
}

func fingerprintDecodeMetadataDirectory() string {
	return filepath.Join("..", "..", "..", "..", "..", "tools", "port-rust-fingerprint", "decode-metadata")
}
func fingerprintMetadataHook(info rustfmt.SourcePanicInfo, current rustfmt.Context) string {
	location := info.Location
	if location == nil {
		panic("missing explicit reference location")
	}
	text, ok := info.Payload.(string)
	if !ok {
		panic("unexpected hook payload")
	}
	return fmt.Sprintf("hook:%s:%s:%s:%d:%d:%t:%t:%t", info.Kind, hex.EncodeToString([]byte(text)), hex.EncodeToString([]byte(location.File)), location.Line, location.Column, info.CanUnwind, info.ForceNoBacktrace, current.Panicking())
}

type fingerprintMetadataDecoder struct {
	length  int
	context rustfmt.Context
	emit    func(string)
	bytes   [32]byte
}

func (d *fingerprintMetadataDecoder) ReadRawBytes(n int) []byte {
	d.emit(fmt.Sprintf("read:%d:%d:%t", n, d.length, d.context.Panicking()))
	return d.bytes[:d.length]
}
func fingerprintMetadataDecode(length int, packed bool, current rustfmt.Context, emit func(string)) {
	d := fingerprintMetadataDecoder{length: length, context: current, emit: emit}
	for i := range d.bytes {
		d.bytes[i] = byte(i)
	}
	defer func() {
		p := recover()
		if p != nil && current.ProcessingPanic {
			current.ResumeUnwind(p)
		}
		emit(fmt.Sprintf("decoder_drop:%t", current.Panicking() || p != nil))
		if p != nil {
			current.ResumeUnwind(p)
		}
	}()
	var value Fingerprint
	if packed {
		value = DecodePackedFingerprintWithContext(current, &d).Fingerprint()
	} else {
		value = DecodeFingerprintWithContext(current, &d)
	}
	a, b := value.Split()
	emit(fmt.Sprintf("value:%d:%d", a.AsU64(), b.AsU64()))
}
func fingerprintMetadataProbe(ambient string, packed bool, length int, locations map[string]rustfmt.SourceLocation, emit func(string)) {
	attempt := func(current rustfmt.Context) {
		caught := false
		func() {
			defer func() { caught = recover() != nil }()
			fingerprintMetadataDecode(length, packed, current, emit)
		}()
		emit(fmt.Sprintf("caught:%t", caught))
	}
	context := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: locations, Hook: func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
		emit(fingerprintMetadataHook(info, current))
		if ambient == "processing" && info.Payload == "ambient" {
			emit("ambient_hook")
			attempt(current)
			emit("hook_return")
		}
	}})
	trigger := func() {
		context.PanicAt(rustfmt.SourcePanicInfo{Payload: "ambient", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation("decode-metadata::ambient"), CanUnwind: true})
	}
	if ambient == "normal" {
		attempt(context)
		return
	}
	if ambient == "inherited" {
		func() {
			defer func() { emit(fmt.Sprintf("ambient_caught:%t", recover() != nil)) }()
			defer func() {
				p := recover()
				if p != nil {
					emit("ambient_drop")
					current := context
					current.Unwinding = true
					attempt(current)
					context.ResumeUnwind(p)
				}
			}()
			trigger()
		}()
		return
	}
	if ambient == "processing" {
		func() { defer func() { emit(fmt.Sprintf("ambient_caught:%t", recover() != nil)) }(); trigger() }()
		return
	}
	panic(ambient)
}
func fingerprintDecodeMetadataLoad(t *testing.T) fingerprintDecodeMetadataManifest {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(fingerprintDecodeMetadataDirectory(), "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest fingerprintDecodeMetadataManifest
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	return manifest
}
func TestFingerprintDecodeMetadataChild(t *testing.T) {
	query := os.Getenv("FERRUGO_FINGERPRINT_DECODE_METADATA_CHILD")
	if query == "" {
		return
	}
	p := strings.Split(query, ":")
	if len(p) != 3 {
		t.Fatal("invalid child query")
	}
	packed, _ := strconv.ParseBool(p[1])
	length, _ := strconv.Atoi(p[2])
	manifest := fingerprintDecodeMetadataLoad(t)
	fingerprintMetadataProbe(p[0], packed, length, manifest.Locations, func(event string) { _, _ = fmt.Fprintln(os.Stderr, "EVENT:"+event) })
}
func TestFingerprintOriginalDecodeMetadata(t *testing.T) {
	manifest := fingerprintDecodeMetadataLoad(t)
	compressed, err := os.ReadFile(filepath.Join(fingerprintDecodeMetadataDirectory(), "cases.tsv.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) > 1<<20 || fmt.Sprintf("%x", sha256.Sum256(compressed)) != manifest.FixtureSHA {
		t.Fatal("decode metadata fixture hash/bound")
	}
	g, err := gzip.NewReader(strings.NewReader(string(compressed)))
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	raw, err := io.ReadAll(io.LimitReader(g, 1<<20+1))
	if err != nil || len(raw) > 1<<20 {
		t.Fatal("decode metadata decompression", err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != manifest.RawSHA {
		t.Fatal("decode metadata raw hash")
	}
	scan := bufio.NewScanner(strings.NewReader(string(raw)))
	records, aborts := 0, 0
	for scan.Scan() {
		records++
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 5 {
			t.Fatal("decode metadata row")
		}
		events := []string{}
		if p[3] == "true" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^TestFingerprintDecodeMetadataChild$")
			command.Env = append(os.Environ(), "FERRUGO_FINGERPRINT_DECODE_METADATA_CHILD="+strings.Join(p[:3], ":"))
			output, err := command.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("decode abort%v: %v %s", p[:3], err, output)
			}
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "EVENT:") {
					events = append(events, strings.TrimPrefix(line, "EVENT:"))
				}
			}
		} else {
			packed, err := strconv.ParseBool(p[1])
			if err != nil {
				t.Fatal(err)
			}
			length, err := strconv.Atoi(p[2])
			if err != nil {
				t.Fatal(err)
			}
			fingerprintMetadataProbe(p[0], packed, length, manifest.Locations, func(event string) { events = append(events, event) })
		}
		want := []string{}
		for _, event := range strings.Split(p[4], "|") {
			if strings.HasPrefix(event, "hook:") {
				parts := strings.Split(event, ":")
				if len(parts) != 10 {
					t.Fatal("retained native hook shape")
				}
				if _, err := hex.DecodeString(parts[9]); err != nil {
					t.Fatal("raw native filename")
				}
				event = strings.Join(parts[:9], ":")
			}
			want = append(want, event)
		}
		if strings.Join(events, "|") != strings.Join(want, "|") {
			t.Fatalf("decode%v\nGo: %s\nRust: %s", p[:4], strings.Join(events, "|"), strings.Join(want, "|"))
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if records != 24 || aborts != 6 || records != manifest.Records || aborts != manifest.ActualAborts {
		t.Fatal("decode metadata counts", records, aborts)
	}
}
func TestFingerprintDecodeMetadataWithoutLocation(t *testing.T) {
	var observed rustfmt.SourcePanicInfo
	context := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, _ rustfmt.Context) { observed = info }})
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected decode failure")
			}
		}()
		DecodeFingerprintWithContext(context, &fingerprintMetadataDecoder{length: 0, context: context, emit: func(string) {}})
	}()
	if observed.Kind != rustfmt.PanicOwnedString || observed.Location != nil || !observed.CanUnwind || observed.ForceNoBacktrace {
		t.Fatal("default decode metadata", observed)
	}
}
