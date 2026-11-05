//go:build linux

// SPDX-License-Identifier: MIT OR Apache-2.0
// Linux default-C-locale OS-error used-contract proof, not a std io file port.
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"github.com/lassejlv/rust-port-go/port/support/rusterrno"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestSourceMapLinuxNativeIOErrors(t *testing.T) {
	if !rusterrno.NativeAvailable() {
		t.Skip("native libc fixture requires cgo; explicit binding is tested separately")
	}
	if runtime.GOOS != "linux" {
		t.Skip("native Linux/glibc error message profile")
	}
	file, err := os.Open("testdata/source_map_io_errors_linux.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	scan := bufio.NewScanner(reader)
	n := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 4 {
			t.Fatal("invalid native IO error fixture")
		}
		code, err := strconv.ParseInt(p[0], 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		failure := sourceMapWrapIOError(syscall.Errno(uintptr(uint32(int32(code)))))
		if got, want := failure.Error(), sourceMapOracleString(p[2]); got != want {
			t.Fatalf("errno%d display%q want%q", code, got, want)
		}
		got := rustfmt.String(func(f *rustfmt.Formatter) error { return FormatSourceIOErrorDebug(failure, f) })
		want := sourceMapOracleString(p[3])
		if got != want {
			t.Fatalf("errno%d debug%q want%q", code, got, want)
		}
		CloseSourceIOError(failure, rustfmt.Context{})
		n++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d original std native OS-error cases", n)
}
