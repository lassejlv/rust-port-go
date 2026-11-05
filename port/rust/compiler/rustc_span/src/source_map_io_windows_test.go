//go:build windows

// SPDX-License-Identifier: MIT OR Apache-2.0
// Exact std Windows Error formatter bridge; actual PE replay is under Wine.
package rustcspan

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestSourceMapWindowsIOErrorChild(t *testing.T) {
	if os.Getenv("FERRUGO_WINDOWS_IO_CHILD") != "1" {
		return
	}
	scan := bufio.NewScanner(os.Stdin)
	scan.Buffer(make([]byte, 65536), 4<<20)
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		code, _ := strconv.ParseInt(p[1], 10, 32)
		style, _ := strconv.Atoi(p[3])
		fail, _ := strconv.Atoi(p[4])
		value := sourceMapWrapIOError(syscall.Errno(uint32(code)))
		sink := &sourceMapFormatSink{fail: fail, address: "<unused-native-pointer>"}
		f := &rustfmt.Formatter{Writer: sink, Options: sourceMapFormatOptions(style)}
		var failure error
		if p[2] == "display" {
			failure = FormatSourceIOErrorDisplay(value, f)
		} else {
			failure = FormatSourceIOErrorDebug(value, f)
		}
		fmt.Printf("%s\t%x\t%t\t%x\t%x\n", p[0], sourceMapWindowsErrorKind(int32(code)), failure == nil, sink.out.String(), strings.Join(sink.calls, "|"))
		CloseSourceIOError(value, rustfmt.Context{})
	}
	if err := scan.Err(); err != nil {
		panic(err)
	}
	os.Exit(0)
}

func TestSourceMapWindowsFilesChild(t *testing.T) {
	if os.Getenv("FERRUGO_WINDOWS_FILES_CHILD") != "1" {
		return
	}
	scan := bufio.NewScanner(os.Stdin)
	scan.Buffer(make([]byte, 65536), 4<<20)
	reprError := func(err error) string {
		defer CloseSourceIOError(err, rustfmt.Context{})
		return fmt.Sprintf("err:%x", rustfmt.String(func(f *rustfmt.Formatter) error { return FormatSourceIOErrorDebug(err, f) }))
	}

	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		b, err := hex.DecodeString(p[2])
		if err != nil || len(b)%2 != 0 {
			panic("invalid UTF16 file request")
		}
		wide := make([]uint16, len(b)/2)
		for i := range wide {
			wide[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
		}
		path := RustPathFromUTF16(wide)
		out := ""
		switch p[1] {
		case "exists":
			out = strconv.FormatBool(sourceMapNativeFileExists(path))
		case "open_meta":
			file, err := sourceMapNativeOpen(path)
			if err != nil {
				out = reprError(err)
			} else {
				length, err := sourceMapNativeMetadataLen(file)
				file.Close()
				if err != nil {
					out = reprError(err)
				} else {
					out = fmt.Sprintf("ok:%d", length)
				}
			}
		case "open_read":
			file, err := sourceMapNativeOpen(path)
			if err != nil {
				out = reprError(err)
			} else {
				bytes, err := io.ReadAll(file)
				file.Close()
				if err != nil {
					out = reprError(sourceMapWrapIOError(err))
				} else {
					out = fmt.Sprintf("ok:%x", bytes)
				}
			}
		case "text":
			text, err := (RealFileLoader{}).ReadFile(path)
			if err != nil {
				out = reprError(err)
			} else {
				out = fmt.Sprintf("ok:%x", text)
			}
		case "cwd":
			old, err := sourceMapNativeCurrentDirectory()
			if err != nil {
				panic(err)
			}
			expected := old.Join(path)
			wide, err := sourceMapWindowsLongPath(path)
			if err != nil {
				out = reprError(err)
			} else if err := syscall.SetCurrentDirectory(&wide[0]); err != nil {
				out = reprError(sourceMapWrapIOError(err))
			} else {
				actual, err := sourceMapNativeCurrentDirectory()
				if err != nil {
					panic(err)
				}
				out = fmt.Sprintf("%t:%t:%t", actual.Equal(expected), actual.HasRoot(), actual.IsAbsolute())
				back, err := sourceMapWindowsLongPath(old)
				if err != nil {
					panic(err)
				}
				if err := syscall.SetCurrentDirectory(&back[0]); err != nil {
					panic(err)
				}
			}
		default:
			panic("unknown Windows file operation")
		}
		fmt.Printf("%s\t%x\n", p[0], out)
	}
	if err := scan.Err(); err != nil {
		panic(err)
	}
	os.Exit(0)
}
