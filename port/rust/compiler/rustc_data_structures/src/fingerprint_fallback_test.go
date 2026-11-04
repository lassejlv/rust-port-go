// SPDX-License-Identifier: MIT OR Apache-2.0
package datastructures

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

type fingerprintFallbackCase struct {
	Name                   string
	Read, Drop             uint8
	Contextual, LocalDefer bool
}

var fingerprintFallbackCases = []fingerprintFallbackCase{
	{Name: "plain_read_first", Read: 0},
	{Name: "plain_read_second", Read: 1},
	{Name: "plain_close_second", Read: 2, Drop: 2},
	{Name: "plain_close_first", Read: 2, Drop: 1},
	{Name: "plain_read_local_defer", Read: 0, LocalDefer: true},
	{Name: "plain_close_local_defer", Read: 2, Drop: 2, LocalDefer: true},
	{Name: "contextual_raw_read_local_defer", Read: 0, Contextual: true, LocalDefer: true},
	{Name: "contextual_raw_close_local_defer", Read: 2, Drop: 2, Contextual: true, LocalDefer: true},
}

func fingerprintRawFallback(test fingerprintFallbackCase, emit func(string)) Fingerprint {
	ambient := FingerprintDropContext{ProcessingPanic: true}
	component := func(id uint8, value uint64) OwnedFingerprintComponent[fpOwnedCell] {
		read := func(cell *fpOwnedCell, context FingerprintDropContext) uint64 {
			cell.reads++
			emit(fmt.Sprintf("read:%d:%d:%t", cell.id, cell.reads, context.Panicking()))
			if cell.id == test.Read {
				if test.LocalDefer {
					defer emit(fmt.Sprintf("go-local-read:%d", cell.id))
				}
				panic("read") // deliberately raw Go, rather than Context.Panic
			}
			return cell.value
		}
		close := func(cell *fpOwnedCell, context FingerprintDropContext) {
			emit(fmt.Sprintf("drop:%d:%d:%t", cell.id, cell.reads, context.Panicking()))
			if test.Drop&(1<<cell.id) != 0 {
				if test.LocalDefer {
					defer emit(fmt.Sprintf("go-local-close:%d", cell.id))
				}
				panic("drop") // raw Go callback-local defers must run first
			}
		}
		result := OwnedFingerprintComponent[fpOwnedCell]{Value: fpOwnedCell{id: id, value: value}}
		if test.Contextual {
			result.AsU64WithContext = read
			result.CloseWithContext = close
		} else {
			result.AsU64 = func(cell *fpOwnedCell) uint64 { return read(cell, ambient) }
			result.Close = func(cell *fpOwnedCell) { close(cell, ambient) }
		}
		return result
	}
	return NewFingerprintOwnedWithContext(ambient, component(0, 7), component(1, 11))
}
func TestFingerprintRawFallbackAbortHelper(t *testing.T) {
	request, ok := os.LookupEnv("FERRUGO_FINGERPRINT_RAW_FALLBACK")
	if !ok {
		t.Skip("isolated raw Go processing-mode callback helper")
	}
	var test fingerprintFallbackCase
	if err := json.Unmarshal([]byte(request), &test); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if value := recover(); value != nil {
			fmt.Fprintln(os.Stdout, "outer-recovered:"+fmt.Sprint(value))
		}
	}()
	emit := func(text string) { fmt.Fprintln(os.Stdout, text) }
	emit("hook-enter:true")
	fingerprintRawFallback(test, emit)
	t.Fatal("processing-mode raw panic returned")
}
func TestFingerprintProcessingPanicRawFallback(t *testing.T) {
	metadata := readFingerprintManifest(t)
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range fingerprintFallbackCases {
		t.Run(test.Name, func(t *testing.T) {
			var expected []string
			for _, native := range metadata.Aborts {
				if native.Processing && native.Read == test.Read && native.Drop == test.Drop {
					expected = append([]string(nil), native.Trace...)
					break
				}
			}
			if len(expected) == 0 {
				t.Fatal("missing retained native processing-panic trace")
			}
			if test.LocalDefer {
				if test.Read != 2 {
					expected = append(expected, fmt.Sprintf("go-local-read:%d", test.Read))
				} else {
					id := 0
					if test.Drop&2 != 0 {
						id = 1
					}
					expected = append(expected, fmt.Sprintf("go-local-close:%d", id))
				}
			}
			encoded, err := json.Marshal(test)
			if err != nil {
				t.Fatal(err)
			}
			deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			command := exec.CommandContext(deadline, binary, "-test.run=^TestFingerprintRawFallbackAbortHelper$")
			command.Env = append(os.Environ(), "FERRUGO_FINGERPRINT_RAW_FALLBACK="+string(encoded))
			var stdout, stderr bytes.Buffer
			command.Stdout = &stdout
			command.Stderr = &stderr
			err = command.Run()
			var failure *exec.ExitError
			if !errors.As(err, &failure) || failure.ExitCode() != 134 {
				t.Fatalf("raw fallback must abort134: %v\n%s\n%s", err, stdout.String(), stderr.String())
			}
			if got, want := strings.TrimSuffix(stdout.String(), "\n"), strings.Join(expected, "\n"); got != want {
				t.Fatalf("component-boundary fallback trace differs:\ngot%s\nwant%s", got, want)
			}
		})
	}
}
