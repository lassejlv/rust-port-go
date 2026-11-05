// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type libTraitFailureRecord struct {
	Profile, Op, Ambient, Stdout string
	Abort                        bool
}
type libTraitLengthDecoder struct {
	*SpanMemDecoder
	target, lengths int
	log             func(string)
}

func (d *libTraitLengthDecoder) read(name string) { d.log("read:" + name) }
func (d *libTraitLengthDecoder) ReadU8() uint8    { d.read("read_u8"); return d.SpanMemDecoder.ReadU8() }
func (d *libTraitLengthDecoder) ReadU16() uint16 {
	d.read("read_u16")
	return d.SpanMemDecoder.ReadU16()
}
func (d *libTraitLengthDecoder) ReadU32() uint32 {
	d.read("read_u32")
	return d.SpanMemDecoder.ReadU32()
}
func (d *libTraitLengthDecoder) ReadU64() uint64 {
	d.read("read_u64")
	return d.SpanMemDecoder.ReadU64()
}
func (d *libTraitLengthDecoder) ReadU128() rustcserialize.Uint128 {
	d.read("read_u128")
	return d.SpanMemDecoder.ReadU128()
}
func (d *libTraitLengthDecoder) ReadI16() int16 {
	d.read("read_i16")
	return d.SpanMemDecoder.ReadI16()
}
func (d *libTraitLengthDecoder) ReadI32() int32 {
	d.read("read_i32")
	return d.SpanMemDecoder.ReadI32()
}
func (d *libTraitLengthDecoder) ReadI64() int64 {
	d.read("read_i64")
	return d.SpanMemDecoder.ReadI64()
}
func (d *libTraitLengthDecoder) ReadI128() rustcserialize.Int128 {
	d.read("read_i128")
	return d.SpanMemDecoder.ReadI128()
}
func (d *libTraitLengthDecoder) ReadIsize() int {
	d.read("read_isize")
	return d.SpanMemDecoder.ReadIsize()
}
func (d *libTraitLengthDecoder) ReadUsize() uint {
	d.read("read_usize")
	d.lengths++
	value := d.SpanMemDecoder.ReadUsize()
	if d.lengths == d.target {
		return ^uint(0)
	}
	return value
}
func (d *libTraitLengthDecoder) ReadRawBytes(n uint) []byte {
	d.read("read_raw_bytes")
	return d.SpanMemDecoder.ReadRawBytes(n)
}
func (d *libTraitLengthDecoder) ReadStr() string     { return rustcserialize.DefaultReadStr(d) }
func (d *libTraitLengthDecoder) ReadByteStr() []byte { return rustcserialize.DefaultReadByteStr(d) }
func (d *libTraitLengthDecoder) ReadBool() bool      { return rustcserialize.DefaultReadBool(d) }
func (d *libTraitLengthDecoder) ReadChar() rune      { return rustcserialize.DefaultReadChar(d) }
func (d *libTraitLengthDecoder) ReadI8() int8        { return rustcserialize.DefaultReadI8(d) }

func libTraitFailureSite(payload string) string {
	switch payload {
	case "outer":
		return "lib-default-fixture.outer"
	case "capacity overflow":
		return rustvec.PanicCapacityOverflowSite
	case "assertion failed: min <= max":
		return LibOrdClampPanicSite
	case "attempt to add with overflow":
		return "source:rustc_index/src/idx.rs:22:19"
	case "assertion failed: value <= (0xFFFF_FF00 as usize)":
		return LibPanicSite(1370, 1)
	}
	panic("unknown source inherited failure payload")
}
func libTraitFailureReplay(record libTraitFailureRecord, writer io.Writer) {
	log := func(line string) { fmt.Fprintln(writer, line) }
	operations := &rustfmt.PanicOperations{Locations: map[string]rustfmt.SourceLocation{}}
	for _, line := range strings.Split(record.Stdout, "\n") {
		if !strings.HasPrefix(line, "hook:") {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) != 9 {
			panic("invalid inherited failure source hook")
		}
		payload, err := hex.DecodeString(fields[2])
		if err != nil {
			panic(err)
		}
		file, err := hex.DecodeString(fields[3])
		if err != nil {
			panic(err)
		}
		row, err := strconv.ParseUint(fields[4], 10, 32)
		if err != nil {
			panic(err)
		}
		column, err := strconv.ParseUint(fields[5], 10, 32)
		if err != nil {
			panic(err)
		}
		operations.Locations[libTraitFailureSite(string(payload))] = rustfmt.SourceLocation{File: string(file), Line: uint32(row), Column: uint32(column)}
	}
	var operate func(rustfmt.Context)
	operations.Hook = func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
		if info.Location == nil {
			panic("missing declared inherited failure profile")
		}
		log(fmt.Sprintf("hook:%s:%s:%s:%d:%d:%t:%t:%t", info.Kind, hex.EncodeToString([]byte(info.Payload.(string))), hex.EncodeToString([]byte(info.Location.File)), info.Location.Line, info.Location.Column, info.CanUnwind, info.ForceNoBacktrace, current.Panicking()))
		if info.Payload == "outer" && record.Ambient == "processing" {
			operate(current)
		}
	}
	context := rustfmt.WithPanicOperations(rustfmt.Context{}, operations)
	config := SpanEncodingConfig{DebugAssertions: record.Profile[1] == '1', OverflowChecks: record.Profile[3] == '1'}
	catch := func(call func()) (caught bool) {
		defer func() {
			if recover() != nil {
				caught = true
			}
		}()
		call()
		return false
	}
	operate = func(current rustfmt.Context) {
		parts := strings.Split(record.Op, "_")
		switch parts[0] {
		case "vec":
			source, err := NewSourceFile(CustomFileName(""), "", SourceFileHashMd5, None[SourceFileHashAlgorithm]())
			if err != nil {
				panic(err)
			}
			defer func() {
				if payload := recover(); payload != nil {
					if current.ProcessingPanic {
						current.ResumeUnwind(payload)
					}
					unwind := current
					unwind.Unwinding = true
					spanCleanupDuringUnwindWithContext(unwind, func() { source.CloseWithContext(unwind) })
					current.ResumeUnwind(payload)
				}
				source.CloseWithContext(current)
			}()
			encoder := NewSpanMemEncoderWithContext(nil, current, config)
			encoder.SetRuntimeContext(current)
			source.Encode(encoder)
			data := append(encoder.Finish(), []byte(rustcserialize.MagicEndBytes)...)
			base, err := NewSpanMemDecoderWithContext(nil, data, 0, current, config)
			if err != nil {
				panic(err)
			}
			base.SetRuntimeContext(current)
			target, _ := strconv.Atoi(parts[1])
			decoder := &libTraitLengthDecoder{SpanMemDecoder: base, target: target, log: log}
			defer func() {
				if payload := recover(); payload != nil {
					if current.ProcessingPanic {
						current.ResumeUnwind(payload)
					}
					log(fmt.Sprintf("decoder_drop:%d:%d:true", decoder.Position(), decoder.lengths))
					current.ResumeUnwind(payload)
				}
				log(fmt.Sprintf("decoder_drop:%d:%d:%t", decoder.Position(), decoder.lengths, current.Panicking()))
			}()
			value := DecodeSourceFile(decoder)
			value.CloseWithContext(current)
			log("decoded")
		case "idx":
			input, _ := strconv.ParseUint(parts[1], 10, 32)
			amount, _ := strconv.ParseUint(parts[2], 10, 64)
			value := AttrIdFromU32WithContext(uint32(input), current)
			var payload any
			func() {
				defer func() { payload = recover() }()
				AttrIdIdxWithContext(current, config).IncrementBy(&value, uint(amount))
			}()
			log(fmt.Sprintf("idx_after:%d", value.AsU32()))
			if payload != nil {
				current.ResumeUnwind(payload)
			}
		case "clamp":
			SourceFileHashSha1.ClampWithContext(SourceFileHashSha256, SourceFileHashMd5, current)
		default:
			panic("unknown inherited failure operation")
		}
	}
	raiseOuter := func() {
		context.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: context.SourcePanicLocation("lib-default-fixture.outer"), CanUnwind: true})
	}
	caughtOperation := func(current rustfmt.Context) {
		caught := catch(func() { operate(current) })
		log(fmt.Sprintf("caught:%t:%t", caught, current.Panicking()))
	}
	switch record.Ambient {
	case "normal":
		caughtOperation(context)
	case "unwinding":
		caught := catch(func() {
			defer func() {
				if payload := recover(); payload != nil {
					unwind := context
					unwind.Unwinding = true
					caughtOperation(unwind)
					context.ResumeUnwind(payload)
				}
			}()
			raiseOuter()
		})
		log(fmt.Sprintf("outer_caught:%t", caught))
	case "processing":
		log(fmt.Sprintf("outer_caught:%t", catch(raiseOuter)))
	default:
		panic("unknown inherited ambient")
	}
}

func TestLibTraitFailuresNativeChild(t *testing.T) {
	if value := os.Getenv("FERRUGO_LIB_TRAIT_FAILURE_CHILD"); value != "" {
		var record libTraitFailureRecord
		if err := json.Unmarshal([]byte(value), &record); err != nil {
			t.Fatal(err)
		}
		libTraitFailureReplay(record, os.Stdout)
		os.Exit(0)
	}
}
func TestLibOriginalTraitFailures(t *testing.T) {
	file, err := os.Open("testdata/lib-trait-failures.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	data, err := io.ReadAll(io.LimitReader(gz, 4*1024*1024+1))
	if err != nil || len(data) > 4*1024*1024 {
		t.Fatal("invalid inherited failure fixture", err)
	}
	scan := bufio.NewScanner(strings.NewReader(string(data)))
	count, aborts := 0, 0
	for scan.Scan() {
		var record libTraitFailureRecord
		if err := json.Unmarshal(scan.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		var output strings.Builder
		if record.Abort {
			payload, _ := json.Marshal(record)
			command := exec.Command(os.Args[0], "-test.run=^TestLibTraitFailuresNativeChild$")
			command.Env = append(os.Environ(), "FERRUGO_LIB_TRAIT_FAILURE_CHILD="+string(payload))
			var stderr strings.Builder
			command.Stdout = &output
			command.Stderr = &stderr
			if err := command.Run(); err == nil || command.ProcessState.ExitCode() != 134 {
				t.Fatalf("%s/%s/%s expectedabort134:%v/%s", record.Profile, record.Op, record.Ambient, err, stderr.String())
			}
			aborts++
		} else {
			libTraitFailureReplay(record, &output)
		}
		if output.String() != record.Stdout {
			t.Fatalf("%s/%s/%s differs\ngot %s\nwant%s", record.Profile, record.Op, record.Ambient, output.String(), record.Stdout)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 84 || aborts != 20 {
		t.Fatalf("inherited failure corpus%d/%d", count, aborts)
	}
}
