// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
)

//go:embed testdata/fingerprint_oracle.tsv.gz
var fingerprintFixture []byte

//go:embed fingerprint_oracle_manifest.json
var fingerprintManifestBytes []byte

type fingerprintManifest struct {
	Records    int    `json:"records"`
	RawSHA     string `json:"raw_sha256"`
	FixtureSHA string `json:"fixture_sha256"`
	Formats    []struct {
		Pattern string
		Options rustchashes.FormatOptions
		Debug   bool
	}
	Aborts []struct {
		Read       uint8    `json:"read"`
		Drop       uint8    `json:"drop"`
		Trace      []string `json:"trace"`
		Inherited  bool     `json:"inherited"`
		CatchInner bool     `json:"catch_inner"`
		Processing bool     `json:"processing"`
	} `json:"owned_aborts"`
}

func readFingerprintManifest(t *testing.T) fingerprintManifest {
	t.Helper()
	var metadata fingerprintManifest
	if err := json.Unmarshal(fingerprintManifestBytes, &metadata); err != nil {
		t.Fatal(err)
	}
	return metadata
}
func fpUint(text string) uint64 {
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		panic(err)
	}
	return value
}
func fpInt(text string) int {
	value, err := strconv.Atoi(text)
	if err != nil {
		panic(err)
	}
	return value
}
func fpBool(text string) bool {
	value, err := strconv.ParseBool(text)
	if err != nil {
		panic(err)
	}
	return value
}
func fpHex(bytes []byte) string { return hex.EncodeToString(bytes) }
func fpUnhex(text string) []byte {
	value, err := hex.DecodeString(text)
	if err != nil {
		panic(err)
	}
	return value
}
func fpPair(value Fingerprint) string {
	a, b := value.Split()
	return fmt.Sprintf("%d,%d", a.AsU64(), b.AsU64())
}
func fpOrder(value int) string {
	if value < 0 {
		return "Less"
	}
	if value > 0 {
		return "Greater"
	}
	return "Equal"
}

type fpTrace []string

func (trace *fpTrace) WriteU64(value uint64) { *trace = append(*trace, strconv.FormatUint(value, 10)) }

type fpWrappedUnhasher struct{ *Unhasher }

type fpEncoder struct {
	bytes      []byte
	operations []string
}

func (encoder *fpEncoder) EmitRawBytes(value []byte) {
	encoder.operations = append(encoder.operations, "raw:"+fpHex(value))
	encoder.bytes = append(encoder.bytes, value...)
}

type fpSink struct {
	events []string
	fail   int
	chars  bool
}

func (sink *fpSink) record(kind string, text string) error {
	index := len(sink.events)
	sink.events = append(sink.events, kind+":"+fpHex([]byte(text)))
	if index == sink.fail {
		return errors.New("original fmt::Error witness")
	}
	return nil
}
func (sink *fpSink) Write(value []byte) (int, error) { return sink.WriteString(string(value)) }
func (sink *fpSink) WriteString(value string) (int, error) {
	err := sink.record("S", value)
	if err != nil {
		return 0, err
	}
	return len(value), nil
}
func (sink *fpSink) WriteChar(value rune) error {
	if sink.chars {
		return sink.record("C", string(value))
	}
	return sink.record("S", string(value))
}
func fpCapture(call func()) (value any) { defer func() { value = recover() }(); call(); return nil }

type fpOwnedCell struct {
	id     uint8
	value  uint64
	reads  uint64
	borrow *fpOwnedCell
}

func fpConstruct(read, drop uint8, ambient FingerprintDropContext, emit func(string)) Fingerprint {
	component := func(id uint8, value uint64) OwnedFingerprintComponent[fpOwnedCell] {
		return OwnedFingerprintComponent[fpOwnedCell]{Value: fpOwnedCell{id: id, value: value}, AsU64WithContext: func(cell *fpOwnedCell, context FingerprintDropContext) uint64 {
			cell.borrow = cell
			cell.reads++
			emit(fmt.Sprintf("read:%d:%d:%t", cell.id, cell.reads, context.Panicking()))
			if read == cell.id {
				context.Panic("read")
			}
			return cell.value
		}, CloseWithContext: func(cell *fpOwnedCell, context FingerprintDropContext) {
			if cell.reads != 0 && cell.borrow != cell {
				panic("destructor did not borrow actual local cell")
			}
			emit(fmt.Sprintf("drop:%d:%d:%t", cell.id, cell.reads, context.Panicking()))
			if drop&(1<<cell.id) != 0 {
				context.Panic("drop")
			}
		}}
	}
	return NewFingerprintOwnedWithContext(ambient, component(0, 7), component(1, 11))
}
func fpOwned(read, drop uint8, emit func(string)) string {
	var result Fingerprint
	panicked := fpCapture(func() { result = fpConstruct(read, drop, FingerprintDropContext{}, emit) })
	if panicked != nil {
		return "panic:" + fpHex([]byte(fmt.Sprint(panicked)))
	}
	return "ok:" + fpPair(result)
}
func fpInherited(read, drop uint8, catchInner bool, emit func(string)) string {
	panicked := fpCapture(func() {
		defer func() {
			original := recover()
			if original == nil {
				return
			}
			cleanup := fpCapture(func() {
				ambient := FingerprintDropContext{Unwinding: true}
				emit("outer-drop:true")
				if catchInner {
					var value Fingerprint
					inner := fpCapture(func() { value = fpConstruct(read, drop, ambient, emit) })
					if inner != nil {
						emit("inner:panic:" + fpHex([]byte(fmt.Sprint(inner))))
					} else {
						emit("inner:ok:" + fpPair(value))
					}
				} else {
					value := fpConstruct(read, drop, ambient, emit)
					emit("inner:ok:" + fpPair(value))
				}
				emit("outer-return:true")
			})
			if cleanup != nil {
				os.Exit(134)
			}
			panic(original)
		}()
		panic("outer")
	})
	return "panic:" + fpHex([]byte(fmt.Sprint(panicked)))
}

func fpProcessing(read, drop uint8, emit func(string)) string {
	panicked := fpCapture(func() {
		emit("hook-enter:true")
		fpConstruct(read, drop, FingerprintDropContext{ProcessingPanic: true}, emit)
		emit("hook-return:true")
		panic("outer")
	})
	return "panic:" + fpHex([]byte(fmt.Sprint(panicked)))
}

type fpDecoder struct {
	bytes       []byte
	position    int
	override    []byte
	overrideSet bool
	operations  []string
}

func (decoder *fpDecoder) ReadRawBytes(length int) []byte {
	decoder.operations = append(decoder.operations, fmt.Sprintf("raw:%d", length))
	if decoder.overrideSet {
		return decoder.override
	}
	if length > len(decoder.bytes)-decoder.position {
		panic("MemDecoder exhausted")
	}
	start := decoder.position
	decoder.position += length
	return decoder.bytes[start:decoder.position]
}
func fpDecode(fields []string) string {
	if !fpBool(fields[3]) {
		return "error"
	} // actual MemDecoder footer contract
	data := fpUnhex(fields[1])
	position := fpInt(fields[2])
	if position > len(data) {
		return "constructor-panic:" + fpHex([]byte(fmt.Sprintf("range start index %d out of range for slice of length %d", position, len(data))))
	}
	decoder := fpDecoder{bytes: data, position: position, overrideSet: fields[4] != "-"}
	if decoder.overrideSet {
		decoder.override = fpUnhex(fields[4])
	}
	var value Fingerprint
	panicked := fpCapture(func() {
		if fpBool(fields[5]) {
			value = DecodePackedFingerprint(&decoder).Fingerprint()
		} else {
			value = DecodeFingerprint(&decoder)
		}
	})
	status := "ok:" + fpPair(value)
	if panicked != nil {
		status = "panic:" + fpHex([]byte(fmt.Sprint(panicked)))
	}
	return fmt.Sprintf("%s:%d:%s", status, decoder.position, strings.Join(decoder.operations, "|"))
}

func TestFingerprintOriginalOracle(t *testing.T) {
	metadata := readFingerprintManifest(t)
	if fmt.Sprintf("%x", sha256.Sum256(fingerprintFixture)) != metadata.FixtureSHA {
		t.Fatal("compressed fixture hash differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(fingerprintFixture))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != metadata.RawSHA {
		t.Fatal("original oracle hash differs")
	}
	debug, records := false, 0
	for _, line := range strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n") {
		records++
		fields := strings.Split(line, "\t")
		fail := func(name string, got, want any) {
			t.Helper()
			if fmt.Sprint(got) != fmt.Sprint(want) {
				t.Fatalf("record%d %s: got%v want%v\n%s", records, name, got, want, line)
			}
		}
		switch fields[0] {
		case "P":
			debug = fpBool(fields[1])
		case "L":
			fail("size", unsafe.Sizeof(Fingerprint{}), fields[1])
			fail("packed size", unsafe.Sizeof(PackedFingerprint{}), fields[3])
			fail("packed alignment", unsafe.Alignof(PackedFingerprint{}), fields[4])
			if strconv.IntSize == 64 {
				fail("alignment", unsafe.Alignof(Fingerprint{}), fields[2])
			}
		case "V":
			a := FingerprintFromU64(fpUint(fields[1]), fpUint(fields[2]))
			b := NewFingerprint(FingerprintU64(fpUint(fields[3])), FingerprintU64(fpUint(fields[4])))
			packed := PackFingerprint(a)
			fail("combine", fpPair(a.Combine(b)), fields[5])
			fail("commutative", fpPair(a.CombineCommutative(b)), fields[6])
			fail("smaller", a.ToSmallerHash().AsU64(), fields[7])
			fail("order", fpOrder(a.Compare(b)), fields[8])
			fail("packed order", fpOrder(packed.Compare(PackFingerprint(b))), fields[9])
			fail("hex", a.ToHex(), fields[10])
			fail("display", a.String(), fields[11])
			fail("packed display", packed.String(), fields[11])
			fail("Debug", fpHex([]byte(a.DebugString())), fields[12])
			fail("packed Debug", fpHex([]byte(packed.DebugString())), fields[13])
			var encoder fpEncoder
			a.Encode(&encoder)
			packed.Encode(&encoder)
			fail("encoded bytes", fpHex(encoder.bytes), fields[14])
			fail("codec operations", strings.Join(encoder.operations, "|"), fields[17])
			var trace fpTrace
			a.Hash(&trace)
			packed.Hash(&trace)
			fail("hash operations", strings.Join(trace, ","), fields[15])
			unhash := NewUnhasherWithDebugAssertions(debug)
			a.Hash(unhash)
			fail("specialization", unhash.Finish(), fields[16])
			bytes := a.ToLEBytes()
			if FingerprintFromLEBytes(bytes) != a || packed.Fingerprint() != a || FingerprintFromStableHash(rustchashes.StableHasherHash{a.first, a.second}) != a || !a.Clone().Equal(a) || !packed.Clone().Equal(packed) || a.StableCompare(b) != a.Compare(b) || !FingerprintCanUseUnstableSort {
				t.Fatal("source conversion/derived traits differ")
			}
			var cloned Fingerprint
			cloned.CloneFrom(a)
			var packedClone PackedFingerprint
			packedClone.CloneFrom(packed)
			if cloned != a || packedClone != packed || ZeroFingerprint() != (Fingerprint{}) || a.asU128() != (rustchashes.Uint128{Lo: a.first, Hi: a.second}) {
				t.Fatal("source Clone/ZERO/u128 differs")
			}
		case "F":
			value := FingerprintFromU64(fpUint(fields[1]), fpUint(fields[2]))
			format := metadata.Formats[fpInt(fields[3])]
			sink := fpSink{fail: fpInt(fields[6]), chars: fpBool(fields[5])}
			formatter := rustchashes.NumericFormatter{Writer: &sink, Options: format.Options}
			var err error
			if fpBool(fields[4]) {
				packed := PackFingerprint(value)
				if format.Debug {
					err = packed.WriteDebug(&formatter)
				} else {
					err = packed.WriteDisplay(&formatter)
				}
			} else {
				if format.Debug {
					err = value.WriteDebug(&formatter)
				} else {
					err = value.WriteDisplay(&formatter)
				}
			}
			fail("format result", err == nil, fields[7])
			fail("format writes", strings.Join(sink.events, "|"), fields[8])
			if len(fields) > 9 {
				fill := formatter.Options.Fill
				if fill == 0 && !formatter.Options.FillSet {
					fill = ' '
				}
				fail("formatter state", fmt.Sprintf("%d:%d", fill, formatter.Options.Alignment), fields[9])
			}
		case "O":
			var events []string
			result := fpOwned(uint8(fpInt(fields[1])), uint8(fpInt(fields[2])), func(text string) { events = append(events, text) })
			fail("owned result", result, fields[3])
			fail("owned cleanup", strings.Join(events, "|"), fields[4])
		case "I":
			var events []string
			result := fpInherited(uint8(fpInt(fields[1])), uint8(fpInt(fields[2])), fpBool(fields[3]), func(text string) { events = append(events, text) })
			fail("inherited result", result, fields[4])
			fail("inherited cleanup", strings.Join(events, "|"), fields[5])
		case "Q":
			var events []string
			result := fpProcessing(uint8(fpInt(fields[1])), uint8(fpInt(fields[2])), func(text string) { events = append(events, text) })
			fail("processing result", result, fields[3])
			fail("processing cleanup", strings.Join(events, "|"), fields[4])
		case "D":
			fail("raw decoder", fpDecode(fields), fields[6])
		case "U":
			hasher := NewUnhasherWithDebugAssertions(debug)
			panicked := fpCapture(func() {
				for _, value := range strings.Split(fields[1], ",") {
					hasher.WriteU64(fpUint(value))
				}
			})
			status := "ok"
			if panicked != nil {
				status = "panic:" + fpHex([]byte(fmt.Sprint(panicked)))
			}
			fail("Unhasher status", status, fields[2])
			fail("Unhasher state", hasher.Finish(), fields[3])
		case "W":
			hasher := fpWrappedUnhasher{NewUnhasherWithDebugAssertions(debug)}
			value := FingerprintFromU64(fpUint(fields[1]), fpUint(fields[2]))
			panicked := fpCapture(func() { value.Hash(&hasher) })
			status := "ok"
			if panicked != nil {
				status = "panic:" + fpHex([]byte(fmt.Sprint(panicked)))
			}
			fail("wrapped hasher status", status, fields[3])
			fail("wrapped hasher state", hasher.Finish(), fields[4])
		case "B":
			hasher := NewUnhasherWithDebugAssertions(debug)
			panicked := fpCapture(func() { hasher.Write([]byte{1, 2, 3}) })
			status := "ok"
			if panicked != nil {
				status = "panic:" + fpHex([]byte(fmt.Sprint(panicked)))
			}
			fail("untyped bytes rejected", status, fields[1])
			fail("rejected bytes preserve state", hasher.Finish(), fields[2])
		case "H":
			value := FingerprintFromU64(fpUint(fields[1]), fpUint(fields[2]))
			var hash, stable fpTrace
			value.Hash(&hash)
			value.StableHash(struct{}{}, &stable)
			fail("stateful proof Hash callbacks", strings.Join(hash, ","), fields[4])
			fail("stateful proof StableHash callbacks", strings.Join(stable, ","), fields[5])
		default:
			t.Fatalf("unknown oracle record: %s", line)
		}
	}
	if records != metadata.Records {
		t.Fatalf("records%d want%d", records, metadata.Records)
	}
}

func TestFingerprintAbortHelper(t *testing.T) {
	request, ok := os.LookupEnv("FERRUGO_FINGERPRINT_GO_ABORT")
	if !ok {
		t.Skip("isolated double-panic helper")
	}
	fields := strings.Split(request, ",")
	emit := func(text string) { fmt.Fprintln(os.Stdout, text) }
	if len(fields) == 3 && fields[2] == "processing" {
		fpProcessing(uint8(fpInt(fields[0])), uint8(fpInt(fields[1])), emit)
	} else if len(fields) == 3 {
		fpInherited(uint8(fpInt(fields[0])), uint8(fpInt(fields[1])), fpBool(fields[2]), emit)
	} else {
		fpOwned(uint8(fpInt(fields[0])), uint8(fpInt(fields[1])), emit)
	}
	t.Fatal("double panic returned")
}
func TestFingerprintOwnedAbortParity(t *testing.T) {
	metadata := readFingerprintManifest(t)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range metadata.Aborts {
		command := exec.Command(binary, "-test.run=^TestFingerprintAbortHelper$")
		request := fmt.Sprintf("%d,%d", test.Read, test.Drop)
		if test.Processing {
			request += ",processing"
		} else if test.Inherited {
			request += fmt.Sprintf(",%t", test.CatchInner)
		}
		command.Env = append(os.Environ(), "FERRUGO_FINGERPRINT_GO_ABORT="+request)
		var stdout, stderr bytes.Buffer
		command.Stdout = &stdout
		command.Stderr = &stderr
		err := command.Run()
		var failure *exec.ExitError
		if !errors.As(err, &failure) || failure.ExitCode() != 134 {
			t.Fatalf("owned destructor double panic must exit134: %v\n%s", err, stderr.String())
		}
		if got, want := strings.TrimSuffix(stdout.String(), "\n"), strings.Join(test.Trace, "\n"); got != want {
			t.Fatalf("before-abort cleanup differs:\ngot%s\nwant%s", got, want)
		}
	}
}

func TestFingerprintOriginalOracleRegeneration(t *testing.T) {
	if os.Getenv("FERRUGO_RUN_FINGERPRINT_ORACLE") != "1" {
		t.Skip("set FERRUGO_RUN_FINGERPRINT_ORACLE=1 for unchanged Rust regeneration")
	}
	root, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("python3", filepath.Join(root, "tools/port-rust-fingerprint/generate.py"), "--root", root, "--offline", "--check")
	command.Dir = root
	command.Env = os.Environ()
	if os.Getenv("FERRUGO_GO") == "" {
		command.Env = append(command.Env, "FERRUGO_GO="+filepath.Join(runtime.GOROOT(), "bin", "go"))
	}
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fingerprint original regeneration: %v\n%s", err, output)
	}
}
