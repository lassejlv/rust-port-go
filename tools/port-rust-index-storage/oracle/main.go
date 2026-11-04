// SPDX-License-Identifier: MIT OR Apache-2.0
// Build and execute the original storage sources; never substitute Go output.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const revision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"

type request struct {
	Nightly, Checked, Debug bool
	Kind                    string
}
type record struct {
	Request  json.RawMessage `json:"request"`
	Response json.RawMessage `json:"response"`
}
type manifest struct {
	Revision         string            `json:"rust_revision"`
	Requests         int               `json:"requests"`
	Steps            int               `json:"steps"`
	Profiles         []string          `json:"profiles"`
	Go               string            `json:"go"`
	Rustc            string            `json:"rustc"`
	Cargo            string            `json:"cargo"`
	Files            map[string]string `json:"sha256"`
	DecodedSHA256    string            `json:"decoded_sha256"`
	CompressedSHA256 string            `json:"compressed_sha256"`
}

func hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func rootDefault() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../.."))
}
func resolveTool(root, value string) (string, error) {
	if value == "" {
		return "", fmt.Errorf("empty explicit tool")
	}
	if !strings.ContainsAny(value, "/\\") {
		return exec.LookPath(value)
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(root, value)
	}
	p, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(p)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("tool is a directory: %s", p)
	}
	return p, nil
}
func selected(root, key, fallback string) (string, error) {
	if value, ok := os.LookupEnv(key); ok {
		return resolveTool(root, value)
	}
	return resolveTool(root, fallback)
}
func envWith(base []string, patch map[string]string) []string {
	out := make([]string, 0, len(base)+len(patch))
	for _, s := range base {
		key, _, _ := strings.Cut(s, "=")
		if _, ok := patch[key]; !ok {
			out = append(out, s)
		}
	}
	for k, v := range patch {
		out = append(out, k+"="+v)
	}
	return out
}
func command(root, bin string, args []string, env []string, input []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%s %v: %w\n%s", bin, args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
func version(root, tool string, args ...string) (string, error) {
	out, err := command(root, tool, args, os.Environ(), nil)
	return strings.TrimSpace(string(out)), err
}
func profile(r request) string {
	a, b, c := "normal", "wrapping", "nodebug"
	if r.Nightly {
		a = "nightly"
	}
	if r.Checked {
		b = "checked"
	}
	if r.Debug {
		c = "debug"
	}
	return a + "-" + b + "-" + c
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	root := flag.String("root", rootDefault(), "repository root")
	goFlag := flag.String("go", "", "Go binary (otherwise FERRUGO_GO or local toolchain)")
	cargoFlag := flag.String("cargo", "", "Cargo binary (otherwise FERRUGO_CARGO or local toolchain)")
	rustFlag := flag.String("rustc", "", "rustc binary (otherwise FERRUGO_RUSTC or local toolchain)")
	check := flag.Bool("check", false, "compare regenerated decoded records and preserve committed gzip bytes")
	offline := flag.Bool("offline", true, "forbid Cargo network access")
	flag.Parse()
	provided := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { provided[f.Name] = true })
	for _, name := range []string{"root", "go", "cargo", "rustc"} {
		if provided[name] && flag.Lookup(name).Value.String() == "" {
			return fmt.Errorf("explicit -%s is empty", name)
		}
	}
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	abs, err := filepath.Abs(*root)
	if err != nil {
		return err
	}
	*root = abs
	choose := func(explicit, key, fallback string) (string, error) {
		if explicit != "" {
			return resolveTool(*root, explicit)
		}
		return selected(*root, key, fallback)
	}
	goBin, err := choose(*goFlag, "FERRUGO_GO", filepath.Join(runtime.GOROOT(), "bin", "go"))
	if err != nil {
		return err
	}
	cargo, err := choose(*cargoFlag, "FERRUGO_CARGO", ".toolchain/rust/bin/cargo")
	if err != nil {
		return err
	}
	rustc, err := choose(*rustFlag, "FERRUGO_RUSTC", ".toolchain/rust/bin/rustc")
	if err != nil {
		return err
	}
	goVersion, err := version(*root, goBin, "version")
	if err != nil {
		return err
	}
	if !supportedGo(goVersion) {
		return fmt.Errorf("selected Go is not a Go toolchain: %s", goVersion)
	}
	rustVersion, err := version(*root, rustc, "-vV")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(rustVersion, "rustc 1.99.1 ") {
		return fmt.Errorf("oracle requires recorded rustc 1.99.1, got %s", rustVersion)
	}
	cargoVersion, err := version(*root, cargo, "--version")
	if err != nil {
		return err
	}
	if !strings.HasPrefix(cargoVersion, "cargo 1.99.1 ") {
		return fmt.Errorf("oracle requires recorded cargo 1.99.1, got %s", cargoVersion)
	}
	home := filepath.Join(*root, ".toolchain/port-cargo-home")
	if value, ok := os.LookupEnv("CARGO_HOME"); ok {
		if value == "" {
			return fmt.Errorf("CARGO_HOME is empty")
		}
		if !filepath.IsAbs(value) {
			value = filepath.Join(*root, value)
		}
		home = value
	}
	base := envWith(os.Environ(), map[string]string{"CARGO_HOME": home, "RUSTC": rustc, "RUSTC_BOOTSTRAP": "1", "RUSTFLAGS": "", "CARGO_ENCODED_RUSTFLAGS": "", "GOTOOLCHAIN": "local", "GIT_NO_LAZY_FETCH": "1"})
	args := []string{"tools/port-rust-index-storage/reference-build/prepare.py", "--root", *root, "--cargo", cargo, "--rustc", rustc}
	if *offline {
		args = append(args, "--offline")
	}
	if _, err := command(*root, "python3", args, base, nil); err != nil {
		return err
	}
	dir := filepath.Join(*root, "tools/port-rust-index-storage/oracle")
	if !*offline {
		if _, err := command(*root, cargo, []string{"fetch", "--locked", "--manifest-path", filepath.Join(dir, "Cargo.toml")}, base, nil); err != nil {
			return err
		}
	}
	registry, err := command(*root, "python3", []string{filepath.Join(dir, "verify.py"), *root, cargo}, base, nil)
	if err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "ferrugo-storage-oracle-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	requestPath := filepath.Join(tmp, "requests.jsonl")
	dumpEnv := envWith(base, map[string]string{"FERRUGO_STORAGE_DUMP_REQUESTS": requestPath})
	if _, err := command(*root, goBin, []string{"test", "./port/rust/compiler/rustc_index/src/vec", "-run", "^TestDumpStorageOracleRequests$", "-count=1"}, dumpEnv, nil); err != nil {
		return err
	}
	requests, err := os.ReadFile(requestPath)
	if err != nil {
		return err
	}
	groups := map[string][][]byte{}
	abortGroups := map[string][][]byte{}
	var ordered [][]byte
	scanner := bufio.NewScanner(bytes.NewReader(requests))
	scanner.Buffer(make([]byte, 65536), 4<<20)
	for scanner.Scan() {
		line := append([]byte{}, scanner.Bytes()...)
		var r request
		if err := json.Unmarshal(line, &r); err != nil {
			return err
		}
		p := profile(r)
		if strings.HasPrefix(r.Kind, "abort") {
			abortGroups[p] = append(abortGroups[p], line)
		} else {
			groups[p] = append(groups[p], line)
		}
		ordered = append(ordered, line)
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(ordered) == 0 {
		return fmt.Errorf("request builder produced no requests")
	}
	var profiles []string
	for p := range groups {
		profiles = append(profiles, p)
	}
	sort.Strings(profiles)
	responses := map[string][][]byte{}
	abortResponses := map[string][]byte{}
	for _, p := range profiles {
		fmt.Fprintln(os.Stderr, "original storage profile", p)
		checks := "false"
		if strings.Contains(p, "-checked-") {
			checks = "true"
		}
		debug := "false"
		if strings.HasSuffix(p, "-debug") {
			debug = "true"
		}
		flags := ""
		if strings.HasPrefix(p, "nightly-") {
			flags = "-Zcrate-attr=feature(never_type)"
		}
		target := filepath.Join(*root, ".toolchain/oracles/storage-broad", p)
		env := envWith(base, map[string]string{"CARGO_TARGET_DIR": target, "CARGO_PROFILE_DEV_OVERFLOW_CHECKS": checks, "CARGO_PROFILE_DEV_DEBUG_ASSERTIONS": debug, "RUSTFLAGS": flags, "CARGO_ENCODED_RUSTFLAGS": flags})
		buildArgs := []string{"build", "--locked", "--manifest-path", filepath.Join(dir, "Cargo.toml"), "--message-format=json"}
		if *offline {
			buildArgs = append(buildArgs, "--offline")
		}
		if strings.HasPrefix(p, "nightly-") {
			buildArgs = append(buildArgs, "--features", "nightly")
		}
		build, err := command(*root, cargo, buildArgs, env, nil)
		if err != nil {
			return err
		}
		binary := ""
		s := bufio.NewScanner(bytes.NewReader(build))
		s.Buffer(make([]byte, 65536), 4<<20)
		for s.Scan() {
			var event struct {
				Reason     string `json:"reason"`
				Executable string `json:"executable"`
			}
			if json.Unmarshal(s.Bytes(), &event) == nil && event.Reason == "compiler-artifact" && event.Executable != "" {
				binary = event.Executable
			}
		}
		if err := s.Err(); err != nil {
			return err
		}
		if binary == "" {
			return fmt.Errorf("Cargo did not report original oracle binary")
		}
		input := bytes.Join(groups[p], []byte{'\n'})
		input = append(input, '\n')
		result, err := command(*root, binary, nil, env, input)
		if err != nil {
			return err
		}
		s = bufio.NewScanner(bytes.NewReader(result))
		s.Buffer(make([]byte, 65536), 4<<20)
		for s.Scan() {
			if !json.Valid(s.Bytes()) {
				return fmt.Errorf("original oracle emitted invalid JSON")
			}
			responses[p] = append(responses[p], append([]byte{}, s.Bytes()...))
		}
		if err := s.Err(); err != nil {
			return err
		}
		if len(responses[p]) != len(groups[p]) {
			return fmt.Errorf("profile %s returned %d of %d responses", p, len(responses[p]), len(groups[p]))
		}
		for _, line := range abortGroups[p] {
			response, err := runAbort(*root, binary, env, line)
			if err != nil {
				return err
			}
			abortResponses[string(line)] = response
		}

	}
	var decoded bytes.Buffer
	offsets := map[string]int{}
	steps := 0
	for _, line := range ordered {
		var r request
		_ = json.Unmarshal(line, &r)
		p := profile(r)
		var response []byte
		if strings.HasPrefix(r.Kind, "abort") {
			response = abortResponses[string(line)]
		} else {
			response = responses[p][offsets[p]]
			offsets[p]++
		}
		var state struct {
			Steps []json.RawMessage `json:"steps"`
		}
		if err := json.Unmarshal(response, &state); err != nil {
			return err
		}
		steps += len(state.Steps)
		row, err := json.Marshal(record{Request: line, Response: response})
		if err != nil {
			return err
		}
		decoded.Write(row)
		decoded.WriteByte('\n')
	}
	var compressed bytes.Buffer
	gz, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := gz.Write(decoded.Bytes()); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	files := []string{"tools/port-rust-index-storage/oracle/main.go", "tools/port-rust-index-storage/oracle/main_test.go", "tools/port-rust-index-storage/oracle/oracle.rs", "tools/port-rust-index-storage/oracle/codec.rs", "tools/port-rust-index-storage/oracle/format.rs", "tools/port-rust-index-storage/oracle/verify.py", "tools/port-rust-index-storage/oracle/Cargo.toml", "tools/port-rust-index-storage/oracle/Cargo.lock", "tools/port-rust-index-storage/reference-build/prepare.py", "tools/port-rust-index-storage/reference-build/Cargo.toml", "tools/port-rust-index-storage/reference-build/rustc_index_macros.Cargo.toml", "tools/port-rust-index-storage/reference-build/rustc_serialize.Cargo.toml", "tools/port-rust-index-storage/reference-build/rustc_hashes.Cargo.toml", "tools/port-rust-index-storage/reference-build/lib.rs", "tools/port-rust-index-storage/reference-build/Cargo.lock", "port/rust/compiler/rustc_index/src/vec/oracle_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_options_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_zst_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_invert_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_custom_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_format_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_iter_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_abort_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_codec_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_integrity_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_codec_default_test.go", "port/rust/compiler/rustc_index/src/vec/oracle_codec_nightly_test.go"}
	hashes := map[string]string{}
	for _, p := range files {
		data, err := os.ReadFile(filepath.Join(*root, p))
		if err != nil {
			return err
		}
		hashes[p] = hash(data)
	}
	provenance, err := os.ReadFile(filepath.Join(*root, ".toolchain/oracles/index-storage-reference/source-provenance.json"))
	if err != nil {
		return err
	}
	hashes["verified-original-source-provenance"] = hash(provenance)
	hashes["verified-registry-provenance"] = hash(registry)
	m := manifest{Revision: revision, Requests: len(ordered), Steps: steps, Profiles: profiles, Go: goVersion, Rustc: rustVersion, Cargo: cargoVersion, Files: hashes, DecodedSHA256: hash(decoded.Bytes()), CompressedSHA256: hash(compressed.Bytes())}
	fixture := filepath.Join(dir, "cases.jsonl.gz")
	manifestPath := filepath.Join(dir, "manifest.json")
	if *check {
		old, err := os.ReadFile(fixture)
		if err != nil {
			return err
		}
		reader, err := gzip.NewReader(bytes.NewReader(old))
		if err != nil {
			return err
		}
		contents, err := io.ReadAll(reader)
		closeErr := reader.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
		if !bytes.Equal(contents, decoded.Bytes()) {
			return fmt.Errorf("storage corpus differs; regenerate without -check")
		}
		oldManifest, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		var prior manifest
		if err := json.Unmarshal(oldManifest, &prior); err != nil {
			return err
		}
		m.Go = prior.Go
		m.CompressedSHA256 = hash(old)
		next, _ := json.MarshalIndent(m, "", "  ")
		next = append(next, '\n')
		if !bytes.Equal(next, oldManifest) {
			return fmt.Errorf("storage provenance differs; regenerate without -check")
		}
	} else {
		if err := os.WriteFile(fixture, compressed.Bytes(), 0644); err != nil {
			return err
		}
		data, _ := json.MarshalIndent(m, "", "  ")
		data = append(data, '\n')
		if err := os.WriteFile(manifestPath, data, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("original storage: %d requests, %d steps, %d profiles\n", m.Requests, m.Steps, len(m.Profiles))
	return nil
}

func runAbort(root, binary string, env []string, line []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary)
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(append(append([]byte{}, line...), '\n'))
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exit, ok := err.(*exec.ExitError)
	if !ok || !(exit.ExitCode() == 134 || exit.ExitCode() == -1 && strings.Contains(exit.ProcessState.String(), "aborted")) {
		return nil, fmt.Errorf("original double panic did not abort: %v\nrequest=%s\n%s\n%s", err, line, stdout.String(), stderr.String())
	}
	events := []string{}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.HasPrefix(line, "ORACLE_EVENT:") {
			var event string
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "ORACLE_EVENT:")), &event); err != nil {
				return nil, err
			}
			events = append(events, event)
		}
	}
	return json.Marshal(map[string]any{"abort": true, "events": events})
}

func supportedGo(version string) bool {
	var minor int
	_, err := fmt.Sscanf(version, "go version go1.%d", &minor)
	return err == nil && minor >= 23 && strings.HasPrefix(version, "go version go1.")
}
