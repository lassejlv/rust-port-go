// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func sourceMapIntoPanic(ctx rustfmt.Context, tag string) {
	line, column := uint32(7), uint32(17)
	if tag == "env" {
		line, column = 8, 16
	} else if tag == "outer" {
		line, column = 9, 18
	}
	ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: tag, Kind: rustfmt.PanicStaticStr, Location: &rustfmt.SourceLocation{File: "owned_into.rs", Line: line, Column: column}, CanUnwind: true})
}
func sourceMapIntoInvoke(path int, owned bool, action string, ctx rustfmt.Context) {
	paths := []string{"abc/in.rs", "abc", "other", ""}
	value := BorrowedCowPath(BorrowedRustPath(paths[path]))
	if owned {
		value = OwnedCowPath(NewRustPath(paths[path]))
	}
	mapping := NewFilePathMapping([]RustPathMappingEntry{{From: NewRustPath("abc"), To: NewRustPath("REMAPPED")}}, AllRemapPathScopeComponents())
	empty := EmptyRealFileName()
	wd := EmptyFilePathMapping()
	working := wd.ToRealFilename(&empty, NewRustPath("/wd"))
	type input struct {
		path   CowPath
		action string
		tag    []byte
	}
	var address uintptr
	state := NewSpanOwned(input{value, action, []byte{1, 2}}, func(value *input, context rustfmt.Context) {
		at := uintptr(unsafe.Pointer(value))
		fmt.Printf("input_drop:%t:%t:%t:%t\n", address == at, value.path.IsOwned(), value.path.Path.IsEmpty(), context.Panicking())
		if value.action == "env" || value.action == "both" {
			sourceMapIntoPanic(context, "env")
		}
	})
	out := FilePathMappingToRealFilenameOwned(&mapping, &working, state, func(value *input, context rustfmt.Context) CowPath {
		address = uintptr(unsafe.Pointer(value))
		fmt.Printf("into:%t:%t\n", value.path.IsOwned(), context.Panicking())
		if value.action == "body" || value.action == "both" {
			sourceMapIntoPanic(context, "body")
		}
		result := value.path
		value.path = OwnedCowPath(NewRustPath(""))
		fmt.Printf("prepared:%t\n", result.IsOwned())
		return result
	}, ctx)
	rendered := rustfmt.String(func(f *rustfmt.Formatter) error { f.Context = ctx; return out.WriteDebugFormatter(f) })
	fmt.Printf("result:%s:%t\n", rendered, ctx.Panicking())
}
func sourceMapIntoChild(profile, mode, action string, path int, owned bool) {
	cleanup := rustfmt.CleanupPanicInfoNightly20261104()
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{CleanupAbort: &cleanup, Hook: func(info rustfmt.SourcePanicInfo, context rustfmt.Context) {
		fmt.Printf("hook:%s:%s:%d:%d:%v\n", info.Kind, info.Location.File, info.Location.Line, info.Location.Column, info.Payload)
		if mode == "processing" && info.Payload == "outer" {
			sourceMapIntoInvoke(path, owned, action, context)
			fmt.Printf("hook_return:%t\n", context.Panicking())
		}
	}})
	switch mode {
	case "normal":
		sourceMapExtractReport(ctx, func() { sourceMapIntoInvoke(path, owned, action, ctx) })
	case "processing":
		sourceMapExtractReport(ctx, func() { sourceMapIntoPanic(ctx, "outer") })
	case "inherited_caught", "inherited_escape":
		sourceMapExtractReport(ctx, func() {
			defer spanCloseOwnedCallback(ctx, func(unwind rustfmt.Context) {
				fmt.Printf("outer_drop:%t\n", unwind.Panicking())
				if mode == "inherited_caught" {
					sourceMapExtractReport(unwind, func() { sourceMapIntoInvoke(path, owned, action, unwind) })
				} else {
					sourceMapIntoInvoke(path, owned, action, unwind)
				}
			})
			sourceMapIntoPanic(ctx, "outer")
		})
	default:
		panic("invalid Into context")
	}
	_ = profile // path manipulation has no independent arithmetic-profile operations.
}
func TestSourceMapOwnedIntoChild(t *testing.T) {
	if os.Getenv("FERRUGO_SOURCE_MAP_INTO_CHILD") != "1" {
		return
	}
	path, _ := strconv.Atoi(os.Getenv("FERRUGO_SOURCE_MAP_INTO_PATH"))
	sourceMapIntoChild(os.Getenv("FERRUGO_SOURCE_MAP_INTO_PROFILE"), os.Getenv("FERRUGO_SOURCE_MAP_INTO_MODE"), os.Getenv("FERRUGO_SOURCE_MAP_INTO_ACTION"), path, os.Getenv("FERRUGO_SOURCE_MAP_INTO_OWNED") == "1")
	os.Exit(0)
}
func TestSourceMapOwnedIntoNativeOracle(t *testing.T) {
	f, err := os.Open("testdata/source_map_owned_into.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	z, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	scan := bufio.NewScanner(z)
	count, aborts := 0, 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 7 {
			t.Fatal("invalid Into fixture")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		command := exec.CommandContext(ctx, binary, "-test.run=^TestSourceMapOwnedIntoChild$")
		command.Env = append(os.Environ(), "FERRUGO_SOURCE_MAP_INTO_CHILD=1", "FERRUGO_SOURCE_MAP_INTO_PROFILE="+p[0], "FERRUGO_SOURCE_MAP_INTO_MODE="+p[1], "FERRUGO_SOURCE_MAP_INTO_ACTION="+p[2], "FERRUGO_SOURCE_MAP_INTO_PATH="+p[3], "FERRUGO_SOURCE_MAP_INTO_OWNED="+p[4])
		out, err := command.Output()
		cancel()
		kind := "ok"
		if err != nil {
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("unexpected Into host failure %v %s", err, out)
			}
			kind = "abort"
			aborts++
		}
		want := sourceMapOracleString(p[6])
		if kind != p[5] || string(out) != want {
			t.Fatalf("%s got%s/%q want%s/%q", strings.Join(p[:5], "/"), kind, out, p[5], want)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	t.Logf("%d actual public Into ownership scripts, %d isolated aborts", count, aborts)
}
