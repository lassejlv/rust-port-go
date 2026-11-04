// ferrugo compiles the documented bootstrap Rust subset using a Go front end.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lassejlv/rust-port-go/internal/compiler"
)

const version = "0.1.0"
const help = `ferrugo 0.1.0 — Rust in Go, one tested step at a time

Usage: ferrugo [options] <source.rs>

  --check          Parse and type-check without generating files
  --crate-type=lib Check a library instead of a binary (requires --check)
  --emit=c         Emit C11 source instead of an executable
  -o <path>        Output path (use - for C source on stdout)
  --version, -V    Show version
  --help, -h       Show help

Executable output defaults to the source basename in the current directory.
C output defaults to <basename>.c. The native backend requires a system C11
compiler (cc, or an executable selected by CC). No rustc or Cargo is invoked.
Only the bootstrap subset documented in docs/COMPATIBILITY.md is supported.
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	fail := func(err error) int { fmt.Fprintf(stderr, "error: %v\n", err); return 1 }
	if len(args) == 0 {
		fmt.Fprint(stdout, help)
		return 0
	}
	var input, output string
	check, emit := false, false
	crateType := "bin"
	typeSeen := false
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--help", "-h":
			fmt.Fprint(stdout, help)
			return 0
		case "--version", "-V":
			fmt.Fprintf(stdout, "ferrugo %s\n", version)
			return 0
		case "--check":
			check = true
		case "--crate-type", "--crate-type=lib", "--crate-type=bin":
			if typeSeen {
				return fail(fmt.Errorf("duplicate --crate-type"))
			}
			typeSeen = true
			if arg == "--crate-type" {
				if i+1 >= len(args) {
					return fail(fmt.Errorf("--crate-type requires bin or lib"))
				}
				i++
				crateType = args[i]
			} else {
				crateType = strings.TrimPrefix(arg, "--crate-type=")
			}
			if crateType != "bin" && crateType != "lib" {
				return fail(fmt.Errorf("--crate-type supports only bin or lib"))
			}
		case "--emit=c":
			emit = true
		case "--emit":
			if i+1 >= len(args) || args[i+1] != "c" {
				return fail(fmt.Errorf("--emit supports only c"))
			}
			i++
			emit = true
		case "-o":
			if i+1 >= len(args) || args[i+1] == "" {
				return fail(fmt.Errorf("-o requires an output path"))
			}
			if output != "" {
				return fail(fmt.Errorf("duplicate output path"))
			}
			i++
			output = args[i]
		default:
			if strings.HasPrefix(arg, "-") {
				return fail(fmt.Errorf("unsupported option %q", arg))
			}
			if input != "" {
				return fail(fmt.Errorf("expected one source file"))
			}
			input = arg
		}
	}
	if input == "" {
		return fail(fmt.Errorf("source file required"))
	}
	if crateType == "lib" && !check {
		return fail(fmt.Errorf("library output artifacts are unsupported; use --crate-type=lib with --check"))
	}
	if check && (emit || output != "") {
		return fail(fmt.Errorf("--check cannot be combined with output options"))
	}
	if output == "-" && !emit {
		return fail(fmt.Errorf("-o - is supported only with --emit=c"))
	}
	if check || emit {
		if check {
			var err error
			if crateType == "lib" {
				err = compiler.CheckLibraryFile(input)
			} else {
				err = compiler.CheckFile(input)
			}
			if err != nil {
				return fail(err)
			}
			return 0
		}
		if output != "-" {
			if output == "" {
				output = base(input) + ".c"
			}
			if err := compiler.WriteC(input, output); err != nil {
				return fail(err)
			}
			return 0
		}
		code, err := compiler.EmitCFile(input)
		if err != nil {
			return fail(err)
		}
		if _, err = io.WriteString(stdout, code); err != nil {
			return fail(err)
		}
		return 0
	}
	if output == "" {
		output = base(input)
	}
	if err := compiler.Build(input, output); err != nil {
		return fail(err)
	}
	return 0
}
func base(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}
