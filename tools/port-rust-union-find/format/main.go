// SPDX-License-Identifier: MIT OR Apache-2.0
// Compile the original whole UF and index storage modules and retain only their
// output. Go output is compared with the reference; it is never a fixture source.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	probe "github.com/lassejlv/rust-port-go/tools/port-rust-union-find/format/validation"
)

const pin = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"

//go:embed oracle.rs
var driver []byte

type source struct {
	Path   string `json:"path"`
	Blob   string `json:"blob"`
	SHA256 string `json:"sha256"`
}
type manifest struct {
	Revision   string            `json:"revision"`
	Rustc      string            `json:"rustc"`
	Go         string            `json:"go"`
	Profiles   []string          `json:"profiles"`
	Records    int               `json:"records"`
	RustTests  int               `json:"original_tests_per_profile"`
	Sources    []source          `json:"sources"`
	Inputs     map[string]string `json:"inputs"`
	Decoded    string            `json:"decoded_sha256"`
	Compressed string            `json:"compressed_sha256"`
}

func digest(data []byte) string { d := sha256.Sum256(data); return hex.EncodeToString(d[:]) }
func main() {
	root := flag.String("root", ".", "repository root")
	rustc := flag.String("rustc", os.Getenv("FERRUGO_RUSTC"), "reference compiler path")
	check := flag.Bool("check", false, "rebuild originals and verify retained output")
	flag.Parse()
	if err := run(*root, *rustc, *check); err != nil {
		fmt.Fprintln(os.Stderr, "union-find-format:", err)
		os.Exit(1)
	}
}
func command(path string, args ...string) ([]byte, error) {
	c := exec.Command(path, args...)
	c.Env = append(os.Environ(), "RUSTC_BOOTSTRAP=1")
	var stderr bytes.Buffer
	c.Stderr = &stderr
	output, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %w\n%s", path, args, err, stderr.Bytes())
	}
	return output, nil
}

func run(root, rustc string, check bool) error {
	var err error
	root, err = filepath.Abs(root)
	if err != nil {
		return err
	}
	if rustc == "" {
		rustc = filepath.Join(root, ".toolchain/rust/bin/rustc")
	} else if !filepath.IsAbs(rustc) && strings.ContainsAny(rustc, `/\`) {
		rustc = filepath.Join(root, rustc)
	}
	version, err := command(rustc, "--version", "--verbose")
	if err != nil {
		return err
	}
	cache := filepath.Join(root, ".toolchain/upstream-rust")
	head, err := command("git", "-C", cache, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != pin {
		return fmt.Errorf("upstream HEAD differs from %s", pin)
	}
	dir := filepath.Join(root, "tools/port-rust-union-find/format")
	m := manifest{Revision: pin, Rustc: strings.TrimSpace(string(version)), Go: runtime.Version(), Profiles: []string{"wrapping-nodebug", "wrapping-debug", "checked-nodebug", "checked-debug"}, RustTests: 2, Inputs: map[string]string{}}
	for _, rel := range []string{"main.go", "oracle.rs", "validation/validation.go"} {
		b, e := os.ReadFile(filepath.Join(dir, rel))
		if e != nil {
			return e
		}
		m.Inputs[rel] = digest(b)
	}
	temp, err := os.MkdirTemp("", "ferrugo-uf-format-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err = os.MkdirAll(filepath.Join(temp, "index"), 0755); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(temp, "upstream/union_find"), 0755); err != nil {
		return err
	}
	for _, file := range []struct{ path, local, blob string }{
		{"compiler/rustc_index/src/idx.rs", "index/idx.rs", "60c4dd90d8f666fc855b5a5e6e22f07209dfd1f8"},
		{"compiler/rustc_index/src/slice.rs", "index/slice.rs", "415fe370b702c4dea8f511f1faacb128af4cc6be"},
		{"compiler/rustc_index/src/vec.rs", "index/vec.rs", "97aad8e6e8c048812f27d1b4df46ed26604be2fa"},
		{"compiler/rustc_data_structures/src/union_find.rs", "upstream/union_find.rs", "ef73cd7ab40abb07d1281315625d312632c5bbae"},
		{"compiler/rustc_data_structures/src/union_find/tests.rs", "upstream/union_find/tests.rs", "34a4e4f8e6edc36719eb5ad996252420957cfc22"},
	} {
		blob, e := command("git", "-C", cache, "rev-parse", pin+":"+file.path)
		if e != nil {
			return e
		}
		if strings.TrimSpace(string(blob)) != file.blob {
			return fmt.Errorf("source blob changed: %s", file.path)
		}
		data, e := command("git", "-C", cache, "show", file.blob)
		if e != nil {
			return e
		}
		m.Sources = append(m.Sources, source{file.path, file.blob, digest(data)})
		if e = os.WriteFile(filepath.Join(temp, file.local), data, 0644); e != nil {
			return e
		}
	}
	// This wrapper only binds whole original modules. It does not implement an
	// index, vector, slice, codec, or UF operation in place of original source.
	storage := []byte("mod idx;pub use idx::{Idx,IntoSliceIdx};mod slice;pub use slice::IndexSlice;mod vec;pub use vec::IndexVec;\n")
	if err = os.WriteFile(filepath.Join(temp, "index/lib.rs"), storage, 0644); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(temp, "driver.rs"), driver, 0644); err != nil {
		return err
	}
	requests := probe.Requests()
	m.Records = len(requests)
	var records bytes.Buffer
	writer := bufio.NewWriter(&records)
	for profile := uint(0); profile < 4; profile++ {
		overflow, debug := "no", "no"
		if profile&2 != 0 {
			overflow = "yes"
		}
		if profile&1 != 0 {
			debug = "yes"
		}
		flags := []string{"--edition=2024", "-Copt-level=2", "-Coverflow-checks=" + overflow, "-Cdebug-assertions=" + debug}
		indexlib := filepath.Join(temp, "liboriginal_index.rlib")
		args := append([]string{}, flags...)
		args = append(args, "--crate-name=original_index", "--crate-type=rlib", filepath.Join(temp, "index/lib.rs"), "-o", indexlib)
		if _, err = command(rustc, args...); err != nil {
			return err
		}
		binary := filepath.Join(temp, "oracle")
		args = append([]string{}, flags...)
		args = append(args, filepath.Join(temp, "driver.rs"), "--extern", "original_index="+indexlib, "-o", binary)
		if _, err = command(rustc, args...); err != nil {
			return err
		}
		unit := filepath.Join(temp, "original-tests")
		args = append([]string{}, flags...)
		args = append(args, "--test", filepath.Join(temp, "driver.rs"), "--extern", "original_index="+indexlib, "-o", unit)
		if _, err = command(rustc, args...); err != nil {
			return err
		}
		proof, e := command(unit, "--test-threads=1")
		if e != nil {
			return e
		}
		if !bytes.Contains(proof, []byte("2 passed; 0 failed")) {
			return fmt.Errorf("original tests did not run: %s", proof)
		}
		var input strings.Builder
		var selected []probe.Request
		for _, r := range requests {
			if r.Profile == profile {
				selected = append(selected, r)
				input.WriteString(r.Protocol())
				input.WriteByte('\n')
			}
		}
		cmd := exec.Command(binary)
		cmd.Stdin = strings.NewReader(input.String())
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		stdout, e := cmd.StdoutPipe()
		if e != nil {
			return e
		}
		if e = cmd.Start(); e != nil {
			return e
		}
		scan := bufio.NewScanner(stdout)
		scan.Buffer(make([]byte, 64*1024), 4*1024*1024)
		n := 0
		for scan.Scan() {
			if n >= len(selected) {
				return fmt.Errorf("extra source output")
			}
			response, e := probe.DecodeResponse(scan.Text())
			if e != nil {
				return e
			}
			if check && !reflect.DeepEqual(probe.Replay(selected[n]), response) {
				return fmt.Errorf("Go/reference mismatch profile%d case%d request%+v\nGo:%+v\nRust:%+v", profile, n, selected[n], probe.Replay(selected[n]), response)
			}
			line, e := json.Marshal(probe.Record{Request: selected[n], Response: response})
			if e != nil {
				return e
			}
			writer.Write(line)
			writer.WriteByte('\n')
			n++
		}
		if e = scan.Err(); e != nil {
			return e
		}
		if e = cmd.Wait(); e != nil {
			return fmt.Errorf("reference runtime: %w: %s", e, stderr.String())
		}
		if n != len(selected) {
			return fmt.Errorf("reference output count %d want%d", n, len(selected))
		}
	}
	if err = writer.Flush(); err != nil {
		return err
	}
	m.Decoded = digest(records.Bytes())
	fixture := filepath.Join(dir, "cases.jsonl.gz")
	if check {
		data, e := os.ReadFile(fixture)
		if e != nil {
			return e
		}
		gz, e := gzip.NewReader(bytes.NewReader(data))
		if e != nil {
			return e
		}
		old, e := io.ReadAll(gz)
		if e != nil {
			return e
		}
		if e = gz.Close(); e != nil {
			return e
		}
		if !bytes.Equal(old, records.Bytes()) {
			return fmt.Errorf("original reference output changed; regenerate")
		}
		m.Compressed = digest(data)
		oldJSON, e := os.ReadFile(filepath.Join(dir, "manifest.json"))
		if e != nil {
			return e
		}
		var oldManifest manifest
		if e = json.Unmarshal(oldJSON, &oldManifest); e != nil {
			return e
		}
		m.Go = oldManifest.Go
		if !reflect.DeepEqual(m, oldManifest) {
			return fmt.Errorf("reference manifest changed; regenerate")
		}
	} else {
		var compressed bytes.Buffer
		gz := gzip.NewWriter(&compressed)
		if _, err = gz.Write(records.Bytes()); err != nil {
			return err
		}
		if err = gz.Close(); err != nil {
			return err
		}
		m.Compressed = digest(compressed.Bytes())
		if err = os.WriteFile(fixture, compressed.Bytes(), 0644); err != nil {
			return err
		}
		data, e := json.MarshalIndent(m, "", "  ")
		if e != nil {
			return e
		}
		data = append(data, '\n')
		if e = os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0644); e != nil {
			return e
		}
	}
	fmt.Printf("union-find formatter: %d original records across4profiles; unchanged2tests/profile pass\n", len(requests))
	return nil
}
