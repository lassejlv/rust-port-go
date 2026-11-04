// SPDX-License-Identifier: MIT OR Apache-2.0
// Full-scalar char Debug evidence for the neutral supporting formatter.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func main() {
	_, file, _, _ := runtime.Caller(0)
	root := flag.String("root", filepath.Clean(filepath.Join(filepath.Dir(file), "../../..")), "repository root")
	compiler := flag.String("rustc", "", "explicit Rust compiler")
	check := flag.Bool("check", false, "verify original output and retain equivalent gzip bytes")
	flag.Parse()
	explicitEmpty := false
	flag.Visit(func(f *flag.Flag) {
		if (f.Name == "rustc" && *compiler == "") || (f.Name == "root" && *root == "") {
			explicitEmpty = true
		}
	})
	if explicitEmpty || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "empty explicit path or unexpected positional argument")
		os.Exit(2)
	}
	if err := run(*root, *compiler, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(root, compiler string, check bool) error {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	if compiler == "" {
		if explicit, ok := os.LookupEnv("FERRUGO_RUSTC"); ok {
			if explicit == "" {
				return fmt.Errorf("FERRUGO_RUSTC is empty")
			}
			compiler = explicit
		} else {
			compiler = filepath.Join(root, ".toolchain/rust/bin/rustc")
		}
	}
	if !filepath.IsAbs(compiler) {
		if strings.ContainsAny(compiler, "/\\") {
			compiler = filepath.Join(root, compiler)
		} else {
			compiler, err = exec.LookPath(compiler)
			if err != nil {
				return err
			}
		}
	}
	version, err := exec.Command(compiler, "--version").Output()
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(version)) != "rustc 1.99.1 (58cf8f9c3 2026-10-13)" {
		return fmt.Errorf("unexpected primitive formatter profile: %s", version)
	}
	dir := filepath.Join(root, "tools/port-lexer-debug/string")
	source := filepath.Join(dir, "oracle.rs")
	body, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "rust-string-debug-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	binary := filepath.Join(tmp, "oracle")
	if output, err := exec.Command(compiler, "--edition=2024", "-C", "opt-level=2", source, "-o", binary).CombinedOutput(); err != nil {
		return fmt.Errorf("compile original primitive oracle: %w\n%s", err, output)
	}
	raw, err := exec.Command(binary).Output()
	if err != nil {
		return err
	}
	if len(raw) != 20+4*4344*8 || string(raw[:4]) != "RDS1" || binaryWord(raw, 4) != 17 || binaryWord(raw, 8) != 1112064 || binaryWord(raw, 12) != 2 || binaryWord(raw, 16) != 4344 {
		return fmt.Errorf("invalid original primitive stream")
	}
	path := filepath.Join(dir, "oracle.bin.gz")
	var compressed []byte
	if prior, e := os.ReadFile(path); e == nil {
		reader, e := gzip.NewReader(bytes.NewReader(prior))
		if e == nil {
			decoded, e := io.ReadAll(reader)
			reader.Close()
			if e == nil && bytes.Equal(decoded, raw) {
				compressed = prior
			}
		}
	}
	if compressed == nil {
		var out bytes.Buffer
		writer, _ := gzip.NewWriterLevel(&out, gzip.BestCompression)
		writer.Write(raw)
		writer.Close()
		compressed = out.Bytes()
	}
	meta, _ := json.MarshalIndent(struct {
		Rustc                             string   `json:"rustc"`
		Unicode                           [3]uint8 `json:"unicode"`
		Scalars, Forms, BlockSize, Blocks int
		Source, Raw, Compressed           string
	}{strings.TrimSpace(string(version)), [3]uint8{17, 0, 0}, 1112064, 2, 256, 4344, digest(body), digest(raw), digest(compressed)}, "", "  ")
	meta = append(meta, '\n')
	outputs := map[string][]byte{path: compressed, filepath.Join(dir, "manifest.json"): meta}
	for path, data := range outputs {
		if check {
			prior, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(prior, data) {
				return fmt.Errorf("original char Debug output differs: %s", path)
			}
		} else {
			if err := os.WriteFile(path, data, 0644); err != nil {
				return err
			}
		}
	}
	fmt.Printf("original str Debug: %d scalars, 2 forms, %d trace blocks per C/S mode\n", 1112064, 4344)
	return nil
}
func binaryWord(raw []byte, offset int) uint32 { return binary.LittleEndian.Uint32(raw[offset:]) }
