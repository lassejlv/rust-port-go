// SPDX-License-Identifier: MIT OR Apache-2.0
// Compile the original whole interval and index storage modules and retain only their
// output. Go output is compared with the reference; it is never a fixture source.
package main

import (
	"archive/tar"
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

	probe "github.com/lassejlv/rust-port-go/tools/port-rust-interval/compat/validation"
)

const pin = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"

//go:embed oracle.rs
var driver []byte

type source struct {
	Path   string `json:"path"`
	Blob   string `json:"blob"`
	SHA256 string `json:"sha256"`
}
type abortRecord struct {
	Request probe.Request
	Trace   []string
	Class   string
}
type manifest struct {
	Aborts          int               `json:"aborts"`
	AbortSHA256     string            `json:"abort_sha256"`
	SmallvecArchive string            `json:"smallvec_archive_sha256"`
	SmallvecSource  string            `json:"smallvec_source_sha256"`
	Revision        string            `json:"revision"`
	Rustc           string            `json:"rustc"`
	Go              string            `json:"go"`
	Profiles        []string          `json:"profiles"`
	Records         int               `json:"records"`
	RustTests       int               `json:"original_tests_per_profile"`
	Sources         []source          `json:"sources"`
	Inputs          map[string]string `json:"inputs"`
	Decoded         string            `json:"decoded_sha256"`
	Compressed      string            `json:"compressed_sha256"`
}

func digest(data []byte) string { d := sha256.Sum256(data); return hex.EncodeToString(d[:]) }
func main() {
	root := flag.String("root", ".", "repository root")
	rustc := flag.String("rustc", os.Getenv("FERRUGO_RUSTC"), "reference compiler path")
	check := flag.Bool("check", false, "rebuild originals and verify retained output")
	flag.Parse()
	if err := run(*root, *rustc, *check); err != nil {
		fmt.Fprintln(os.Stderr, "interval-compat:", err)
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
	dir := filepath.Join(root, "tools/port-rust-interval/compat")
	m := manifest{Revision: pin, Rustc: strings.TrimSpace(string(version)), Go: runtime.Version(), Profiles: []string{"wrapping-nodebug", "wrapping-debug", "checked-nodebug", "checked-debug"}, RustTests: 7, Inputs: map[string]string{}}
	for _, rel := range []string{"main.go", "oracle.rs", "validation/validation.go", "witness/main.go"} {
		b, e := os.ReadFile(filepath.Join(dir, rel))
		if e != nil {
			return e
		}
		m.Inputs[rel] = digest(b)
	}
	temp, err := os.MkdirTemp("", "ferrugo-interval-compat-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err = os.MkdirAll(filepath.Join(temp, "index"), 0755); err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Join(temp, "interval"), 0755); err != nil {
		return err
	}
	for _, file := range []struct{ path, local, blob string }{
		{"compiler/rustc_index/src/idx.rs", "index/idx.rs", "60c4dd90d8f666fc855b5a5e6e22f07209dfd1f8"},
		{"compiler/rustc_index/src/slice.rs", "index/slice.rs", "415fe370b702c4dea8f511f1faacb128af4cc6be"},
		{"compiler/rustc_index/src/vec.rs", "index/vec.rs", "97aad8e6e8c048812f27d1b4df46ed26604be2fa"},
		{"compiler/rustc_index/src/interval.rs", "interval.rs", "475c81e66ab09229137350775db615e39df24eee"},
		{"compiler/rustc_index/src/interval/tests.rs", "interval/tests.rs", "cf3222e6c6572303939f620858a4d98a90ba2bbe"},
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
	// index, vector, slice, codec, or interval operation in place of original source.
	storage := []byte("mod idx;pub use idx::{Idx,IntoSliceIdx};mod slice;pub use slice::IndexSlice;mod vec;pub use vec::IndexVec;\n")
	if err = os.WriteFile(filepath.Join(temp, "index/lib.rs"), storage, 0644); err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(temp, "driver.rs"), driver, 0644); err != nil {
		return err
	}
	archive, e := os.ReadFile(filepath.Join(root, ".toolchain/port-rust-interval-deps/smallvec-1.15.1.crate"))
	if e != nil {
		return e
	}
	const smallvecHash = "67b1b7a3b5fe4f1376887184045fcf45c69e92af734b7aaddc05fb777b6fbd03"
	if digest(archive) != smallvecHash {
		return fmt.Errorf("SmallVec archive differs from pinned root Cargo.lock")
	}
	lock, e := command("git", "-C", cache, "show", pin+":Cargo.lock")
	if e != nil {
		return e
	}
	if !bytes.Contains(lock, []byte("name = \"smallvec\"\nversion = \"1.15.1\"\nsource = \"registry+https://github.com/rust-lang/crates.io-index\"\nchecksum = \""+smallvecHash+"\"")) {
		return fmt.Errorf("pinned SmallVec lock entry changed")
	}
	m.SmallvecArchive = smallvecHash
	if e = unpackSmallvec(archive, temp); e != nil {
		return e
	}
	smallvecSource := filepath.Join(temp, "smallvec-1.15.1/src/lib.rs")
	sourceBytes, e := os.ReadFile(smallvecSource)
	if e != nil {
		return e
	}
	m.SmallvecSource = digest(sourceBytes)
	smallvecLib := filepath.Join(temp, "libsmallvec.rlib")
	requests := probe.Requests()
	m.Records = len(requests)
	var aborts []abortRecord
	var records bytes.Buffer
	writer := bufio.NewWriter(&records)
	goWitness := filepath.Join(temp, "go-witness")
	if check {
		goBin := filepath.Join(runtime.GOROOT(), "bin/go")
		if custom, ok := os.LookupEnv("FERRUGO_GO"); ok {
			if custom == "" {
				return fmt.Errorf("FERRUGO_GO cannot be empty")
			}
			goBin = custom
		}
		if !filepath.IsAbs(goBin) && strings.ContainsAny(goBin, `/\`) {
			goBin = filepath.Join(root, goBin)
		}
		build := exec.Command(goBin, "build", "-o", goWitness, "./tools/port-rust-interval/compat/witness")
		build.Dir = root
		if output, e := build.CombinedOutput(); e != nil {
			return fmt.Errorf("build isolated Go child: %w\n%s", e, output)
		}
	}
	for profile := uint(0); profile < 4; profile++ {
		overflow, debug := "no", "no"
		if profile&2 != 0 {
			overflow = "yes"
		}
		if profile&1 != 0 {
			debug = "yes"
		}
		flags := []string{"--edition=2024", "-Copt-level=2", "-Coverflow-checks=" + overflow, "-Cdebug-assertions=" + debug}
		if _, err = command(rustc, "--edition=2018", "--crate-name=smallvec", "--crate-type=rlib", "-Copt-level=2", "-Coverflow-checks="+overflow, "-Cdebug-assertions="+debug, smallvecSource, "-o", smallvecLib); err != nil {
			return err
		}
		indexlib := filepath.Join(temp, "liboriginal_index.rlib")
		args := append([]string{}, flags...)
		args = append(args, "--crate-name=original_index", "--crate-type=rlib", filepath.Join(temp, "index/lib.rs"), "-o", indexlib)
		if _, err = command(rustc, args...); err != nil {
			return err
		}
		binary := filepath.Join(temp, "oracle")
		args = append([]string{}, flags...)
		args = append(args, "--crate-name=oracle", filepath.Join(temp, "driver.rs"), "--extern", "original_index="+indexlib, "--extern", "smallvec="+smallvecLib, "-o", binary)
		if _, err = command(rustc, args...); err != nil {
			return err
		}
		unit := filepath.Join(temp, "original-tests")
		args = append([]string{}, flags...)
		args = append(args, "--test", "--crate-name=oracle", filepath.Join(temp, "driver.rs"), "--extern", "original_index="+indexlib, "--extern", "smallvec="+smallvecLib, "-o", unit)
		if _, err = command(rustc, args...); err != nil {
			return err
		}
		proof, e := command(unit, "--test-threads=1")
		if e != nil {
			return e
		}
		if !bytes.Contains(proof, []byte("7 passed; 0 failed")) {
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
			if check && !reflect.DeepEqual(probe.Replay(selected[n], nil), response) {
				return fmt.Errorf("Go/reference mismatch profile%d case%d request%+v\nGo:%+v\nRust:%+v", profile, n, selected[n], probe.Replay(selected[n], nil), response)
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
		for _, request := range probe.AbortRequests() {
			if request.Profile != profile {
				continue
			}
			// Disable core files only in this owned reference child, then replace sh
			// with the Rust executable. Arguments remain structured, never interpolated.
			cmd := exec.Command("sh", "-c", "ulimit -c 0\nexec \"$@\"", "interval-abort", binary, "--abort")
			cmd.Stdin = strings.NewReader(request.Protocol() + "\n")
			output, e := cmd.Output()
			native, ok := e.(*exec.ExitError)
			if !ok || native.ExitCode() != -1 || !strings.Contains(e.Error(), "aborted") {
				return fmt.Errorf("expected native abort for%+v, got%v\n%s", request, e, output)
			}
			events, e := abortEvents(output)
			if e != nil {
				return e
			}
			record := abortRecord{Request: request, Trace: events, Class: "abort"}
			if check {
				encoded, _ := json.Marshal(request)
				goCmd := exec.Command(goWitness)
				goCmd.Stdin = bytes.NewReader(encoded)
				got, e := goCmd.Output()
				status, ok := e.(*exec.ExitError)
				if !ok || status.ExitCode() != 134 {
					return fmt.Errorf("expected Go abort for%+v, got%v\n%s", request, e, got)
				}
				actual, e := abortEvents(got)
				if e != nil {
					return e
				}
				if !reflect.DeepEqual(actual, events) {
					return fmt.Errorf("abort trace mismatch request%+v\nGo:%v\nRust:%v", request, actual, events)
				}
			}
			aborts = append(aborts, record)
		}

	}
	if err = writer.Flush(); err != nil {
		return err
	}
	m.Decoded = digest(records.Bytes())
	m.Aborts = len(aborts)
	abortJSON, e := json.MarshalIndent(aborts, "", "  ")
	if e != nil {
		return e
	}
	abortJSON = append(abortJSON, '\n')
	m.AbortSHA256 = digest(abortJSON)
	abortPath := filepath.Join(dir, "aborts.json")
	if check {
		saved, e := os.ReadFile(abortPath)
		if e != nil {
			return e
		}
		if !bytes.Equal(saved, abortJSON) {
			return fmt.Errorf("native abort records changed; regenerate")
		}
	} else {
		if e = os.WriteFile(abortPath, abortJSON, 0644); e != nil {
			return e
		}
	}
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
	fmt.Printf("interval compatibility: %d original records across 4 profiles; unchanged 7 tests/profile pass\n", len(requests))
	return nil
}

func unpackSmallvec(archive []byte, dir string) error {
	gz, e := gzip.NewReader(bytes.NewReader(archive))
	if e != nil {
		return e
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, e := tr.Next()
		if e == io.EOF {
			return nil
		}
		if e != nil {
			return e
		}
		p := filepath.Clean(filepath.FromSlash(h.Name))
		if h.Typeflag == tar.TypeDir && p == "smallvec-1.15.1" {
			continue
		}
		if filepath.IsAbs(p) || !strings.HasPrefix(p, "smallvec-1.15.1"+string(filepath.Separator)) {
			return fmt.Errorf("invalid archive member %s", p)
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		if h.Typeflag != tar.TypeReg {
			return fmt.Errorf("unsupported archive member %s", p)
		}
		target := filepath.Join(dir, p)
		if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
			return e
		}
		b, e := io.ReadAll(tr)
		if e != nil {
			return e
		}
		if e = os.WriteFile(target, b, 0644); e != nil {
			return e
		}
	}
}

func abortEvents(data []byte) ([]string, error) {
	events := []string{}
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "event:") {
			return nil, fmt.Errorf("unexpected isolated output: %s", line)
		}
		raw, e := hex.DecodeString(strings.TrimPrefix(line, "event:"))
		if e != nil {
			return nil, e
		}
		events = append(events, string(raw))
	}
	return events, nil
}
