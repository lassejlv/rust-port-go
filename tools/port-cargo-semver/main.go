// Original Cargo semver extension oracle generator. SPDX-License-Identifier: MIT OR Apache-2.0
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
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
	"sort"
	"strconv"
	"strings"
	"time"
)

const revision = "24de4721844c7a4d839072bab6fb66d41c9bc1f4"
const rustVersion = "rustc 1.99.1 (58cf8f9c3 2026-10-13)"
const toolPath = "tools/port-cargo-semver"
const targetPath = "port/cargo/src/util"

type dependency struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Checksum string `json:"checksum"`
}
type sourcePin struct {
	Path string `json:"path"`
	Blob string `json:"git_blob"`
	Hash string `json:"sha256"`
}

var sourcePins = []sourcePin{
	{"src/util/semver_ext.rs", "39d0f5659d6e68115e4b5d07eea7a840b290297c", "ec29b287daf33cf7ce867a7e0981df607e7e84fa4bc2984c7f42f8c64468d3d0"},
	{"src/util/semver_eval_ext.rs", "4cb09e9fb53d3f12212a0abcd712f2e11a1d181e", "49d93e6907b452ac732f559aeb4a96a7eca720c3a33240be7e23d47686bcd0c5"},
	{"Cargo.lock", "8af3a6d2fbbc40a8c7aa0d97045eebc29c9a6253", "f394b1d1e68c7f7cd11261b1623493c3da407a88f0f50519a403c339b750b5c7"},
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
func absolute(root, path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(root, path)
}
func selected(root, variable, flagValue, local, fallback string, explicit bool) (string, error) {
	name := flagValue
	if !explicit {
		var found bool
		name, found = os.LookupEnv(variable)
		if !found {
			name = local
			if info, err := os.Stat(name); err != nil || info.IsDir() {
				name = fallback
			}
		}
	}
	if name == "" {
		return "", fmt.Errorf("%s executable was explicitly empty", variable)
	}
	if strings.ContainsAny(name, `/\`) {
		name = absolute(root, name)
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("%s executable %q: %w", variable, name, err)
	}
	return filepath.Abs(path)
}

func selectedDirectory(root, variable, value, fallback string, explicit bool) (string, error) {
	if !explicit {
		var present bool
		value, present = os.LookupEnv(variable)
		if !present {
			return filepath.Join(root, fallback), nil
		}
	}
	if value == "" {
		return "", fmt.Errorf("%s directory was explicitly empty", variable)
	}
	return absolute(root, value), nil
}

func environment(overrides map[string]string) []string {
	drop := map[string]bool{"RUSTFLAGS": true, "CARGO_ENCODED_RUSTFLAGS": true, "RUSTC_WRAPPER": true, "RUSTC_WORKSPACE_WRAPPER": true, "RUSTC_BOOTSTRAP": true, "UPDATE_EXPECT": true, "CARGO_BUILD_TARGET": true}
	var result []string
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if _, changed := overrides[key]; !changed && !drop[key] {
			result = append(result, entry)
		}
	}
	for key, value := range overrides {
		result = append(result, key+"="+value)
	}
	return result
}

func command(ctx context.Context, dir string, env []string, exe string, input []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Env = env
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s %v: %w\n%s", exe, args, err, stderr.Bytes())
	}
	return output, nil
}

func lockDependencies(data []byte) []dependency {
	var result []dependency
	for _, block := range strings.Split(string(data), "[[package]]") {
		var dep dependency
		registry := false
		for _, line := range strings.Split(block, "\n") {
			key, value, ok := strings.Cut(line, " = ")
			if !ok {
				continue
			}
			text, err := strconv.Unquote(value)
			if err != nil {
				continue
			}
			switch key {
			case "name":
				dep.Name = text
			case "version":
				dep.Version = text
			case "checksum":
				dep.Checksum = text
			case "source":
				registry = text == "registry+https://github.com/rust-lang/crates.io-index"
			}
		}
		if registry {
			result = append(result, dep)
		}
	}
	sort.Slice(result, func(a, b int) bool {
		return result[a].Name < result[b].Name || result[a].Name == result[b].Name && result[a].Version < result[b].Version
	})
	return result
}

func verifyArchives(metadata []byte, cargoHome string, deps []dependency) error {
	var decoded struct {
		Packages []struct {
			Name     string  `json:"name"`
			Version  string  `json:"version"`
			Source   *string `json:"source"`
			Manifest string  `json:"manifest_path"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(metadata, &decoded); err != nil {
		return err
	}
	wanted := map[string]string{}
	seen := map[string]bool{}
	for _, dep := range deps {
		wanted[dep.Name+"@"+dep.Version] = dep.Checksum
	}
	for _, pkg := range decoded.Packages {
		if pkg.Source == nil {
			continue
		}
		if *pkg.Source != "registry+https://github.com/rust-lang/crates.io-index" {
			return fmt.Errorf("unexpected registry source: %s", *pkg.Source)
		}
		key := pkg.Name + "@" + pkg.Version
		seen[key] = true
		expected, ok := wanted[key]
		if !ok {
			return fmt.Errorf("unlocked registry package %s", key)
		}
		packageDir := filepath.Dir(pkg.Manifest)
		registry := filepath.Base(filepath.Dir(packageDir))
		archive := filepath.Join(cargoHome, "registry/cache", registry, pkg.Name+"-"+pkg.Version+".crate")
		data, err := os.ReadFile(archive)
		if err != nil {
			return err
		}
		if digest(data) != expected {
			return fmt.Errorf("registry archive checksum mismatch: %s", archive)
		}
		compressed, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return err
		}
		tarReader := tar.NewReader(compressed)
		for {
			header, err := tarReader.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				compressed.Close()
				return err
			}
			if header.Typeflag != tar.TypeReg {
				continue
			}
			prefix := pkg.Name + "-" + pkg.Version + "/"
			if !strings.HasPrefix(header.Name, prefix) {
				compressed.Close()
				return fmt.Errorf("unexpected archive path %s", header.Name)
			}
			name := strings.TrimPrefix(header.Name, prefix)
			if filepath.IsAbs(name) || strings.Contains(name, "..") {
				compressed.Close()
				return fmt.Errorf("invalid archive member %s", name)
			}
			original, err := io.ReadAll(tarReader)
			if err != nil {
				compressed.Close()
				return err
			}
			actual, err := os.ReadFile(filepath.Join(packageDir, filepath.FromSlash(name)))
			if err != nil {
				compressed.Close()
				return err
			}
			if !bytes.Equal(actual, original) {
				compressed.Close()
				return fmt.Errorf("expanded registry source changed: %s/%s", key, name)
			}
		}
		if err := compressed.Close(); err != nil {
			return err
		}
	}
	if len(seen) != len(wanted) {
		return fmt.Errorf("metadata selected %d registry packages, expected %d", len(seen), len(wanted))
	}
	return nil
}

func pinnedSource(ctx context.Context, root string, pin sourcePin) ([]byte, error) {
	env := environment(map[string]string{"GIT_NO_LAZY_FETCH": "1"})
	blob, err := command(ctx, root, env, "git", nil, "-C", root, "rev-parse", revision+":"+pin.Path)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(string(blob)) != pin.Blob {
		return nil, fmt.Errorf("Git blob differs: %s", pin.Path)
	}
	data, err := command(ctx, root, env, "git", nil, "-C", root, "show", revision+":"+pin.Path)
	if err != nil {
		return nil, err
	}
	if digest(data) != pin.Hash {
		return nil, fmt.Errorf("source SHA-256 differs: %s", pin.Path)
	}
	return data, nil
}

func write32(w io.Writer, n uint32) {
	if err := binary.Write(w, binary.LittleEndian, n); err != nil {
		panic(err)
	}
}
func text(w io.Writer, s string) {
	write32(w, uint32(len(s)))
	if _, err := io.WriteString(w, s); err != nil {
		panic(err)
	}
}

func retainGzip(raw, existing []byte, check bool) ([]byte, error) {
	if len(existing) != 0 {
		r, err := gzip.NewReader(bytes.NewReader(existing))
		if err == nil {
			data, readErr := io.ReadAll(io.LimitReader(r, int64(len(raw))+1))
			closeErr := r.Close()
			if readErr == nil && closeErr == nil && bytes.Equal(data, raw) {
				return existing, nil
			}
		}
	}
	if check {
		return nil, fmt.Errorf("committed gzip absent, invalid, or differs from original output")
	}
	var result bytes.Buffer
	w, _ := gzip.NewWriterLevel(&result, gzip.BestCompression)
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

type comparator struct {
	Op           byte
	Major        uint64
	Minor, Patch *uint64
	Pre          string
}
type action struct {
	Op    byte
	Value string
}
type request struct {
	Kind, Constructor byte
	Req, Version      string
	Raw               []comparator
	Actions           []action
}

func number(n uint64) *uint64 { return &n }
func requests() []request {
	var cases []request
	add := func(r request) { cases = append(cases, r) }
	prereleases := []string{"", "-0", "-1", "-alpha", "-alpha.1", "-beta.2", "-rc.0"}
	var versions []string
	for _, major := range []uint64{0, 1, 2, 4} {
		for _, minor := range []uint64{0, 1, 2, 24} {
			for _, patch := range []uint64{0, 1, 3} {
				for _, pre := range prereleases {
					for _, build := range []string{"", "+build.1"} {
						versions = append(versions, fmt.Sprintf("%d.%d.%d%s%s", major, minor, patch, pre, build))
					}
				}
			}
		}
	}
	versions = append(versions, "0.0.18446744073709551615", "0.0.18446744073709551615-0", "0.18446744073709551615.0-alpha", "1.18446744073709551615.0-alpha", "18446744073709551615.0.0-alpha", "18446744073709551615.18446744073709551615.18446744073709551615")
	var requirements []string
	for _, operation := range []string{"=", ">", ">=", "<", "<=", "~", "^"} {
		for _, major := range []uint64{0, 1, 2, 4} {
			for _, suffix := range []string{"", ".0", ".2", ".0.0", ".0.3", ".2.1", ".0.0-0", ".0.3-alpha.1", ".2.1-rc.0"} {
				requirements = append(requirements, fmt.Sprintf("%s%d%s", operation, major, suffix))
			}
		}
	}
	requirements = append(requirements, "*", "1.*", "1.*.*", "0.0.*", ">=1.2.3, <2.0.0", ">=1.2.3-0, <2", ">=1.2.3-0, <2.0", ">=1.2.3-0, <2.0.0", ">=2.0.0-0, <2", ">=1.2.3-2, <1.2.3-4", ">5.0.0, <2.0.0", ">5.0.0-0, <2.0.0", "=18446744073709551615", "^18446744073709551615", "~0.18446744073709551615", "=0.0.18446744073709551615")
	for _, req := range requirements {
		for _, version := range versions {
			add(request{Kind: 2, Req: req, Version: version})
		}
	}
	// Public Comparator fields permit absent minor/present patch combinations
	// that the parser cannot produce. Exercise the original algorithms on them.
	optional := []*uint64{nil, number(0), number(1), number(^uint64(0))}
	for operation := byte(0); operation < 8; operation++ {
		for _, major := range []uint64{0, 1, ^uint64(0)} {
			for _, minor := range optional {
				for _, patch := range optional {
					for _, pre := range []string{"", "0", "alpha.1"} {
						cmp := comparator{operation, major, minor, patch, pre}
						for _, version := range []string{"0.0.0", "0.0.1-0", "0.1.0-alpha", "1.0.0-0", "1.1.3-beta", "2.0.0-alpha", "18446744073709551615.0.0-0", "18446744073709551615.18446744073709551615.18446744073709551615"} {
							add(request{Kind: 2, Version: version, Raw: []comparator{cmp}})
						}
					}
				}
			}
		}
	}
	// Opposing/composed bounds and comparator iteration order influence both
	// prerelease admission and whether a later bound overflows.
	for _, pair := range [][2]comparator{
		{{0, 0, number(0), number(^uint64(0)), ""}, {1, 1, nil, nil, ""}},
		{{1, 1, nil, nil, ""}, {0, 0, number(0), number(^uint64(0)), ""}},
		{{3, 2, nil, nil, ""}, {2, 1, number(0), number(0), "alpha"}},
		{{2, 1, number(0), number(0), "alpha"}, {3, 2, nil, nil, ""}},
	} {
		for _, version := range versions {
			add(request{Kind: 2, Version: version, Raw: []comparator{pair[0], pair[1]}})
		}
	}
	scriptReqs := []string{"*", "=1.2.3", "^1.2.3", "^1.2.3-alpha", "~0.2", ">=1.2.3-0, <2.0.0", "=0.0.18446744073709551615"}
	for constructor := byte(0); constructor < 7; constructor++ {
		for _, req := range scriptReqs {
			for _, version := range []string{"1.2.3", "1.2.3+foo", "1.2.3-alpha", "0.0.18446744073709551615-0"} {
				actions := []action{{0, ""}, {1, version}, {2, version}, {3, version}, {4, version}, {5, version}, {1, "1.2.3+bar"}, {6, "2.0.0+precise"}, {1, "2.0.0+other"}, {2, "2.0.0+precise"}, {6, "2.0.0"}, {1, "2.0.0+other"}, {7, ""}, {2, "2.0.0-alpha"}, {5, "1.2.3-alpha"}, {9, "1.2.3"}, {10, "1.2.3+locked"}, {1, "1.2.3+other"}, {11, ""}}
				for k := 0; k < 4; k++ {
					actions = append(actions, action{12, fmt.Sprintf("%d|%s", k, version)})
				}
				add(request{Constructor: constructor, Req: req, Version: version, Actions: actions})
			}
		}
	}
	for constructor := byte(0); constructor < 7; constructor++ {
		for _, req := range []string{"*", "=1.2.3-rc.0", ">=1.2, <2"} {
			var actions []action
			for n := 0; n < 32; n++ {
				actions = append(actions, action{8, strconv.Itoa(n)})
			}
			add(request{Constructor: constructor, Req: req, Version: "1.2.3-rc.0+build.007", Actions: actions})
		}
	}
	for variant := byte(0); variant < 4; variant++ {
		for _, mask := range []byte{0, 1, 2, 4, 8, 16, 31} {
			for operation := 0; operation < 8; operation++ {
				for _, version := range []string{"1.2.3", "0.0.0-alpha+build.007"} {
					add(request{Kind: 1, Constructor: variant, Req: strconv.Itoa(int(mask)), Version: version, Actions: []action{{0, ""}, {1, strconv.Itoa(operation)}, {2, ""}, {3, ""}, {4, "1.2.2-alpha"}, {4, "1.2.3+foo"}}})
				}
			}
		}
	}
	for _, req := range []string{"", "garbage", "1.2.3 || 2", "^18446744073709551616", "=1.2.3-00", "<1.2.3+?"} {
		add(request{Req: req, Version: "1.2.3"})
	}
	for _, version := range []string{"", "1", "1.2", "1.2.3.4", "18446744073709551616.0.0", "1.2.3-00", "1.2.3+?"} {
		add(request{Req: "*", Version: version})
	}
	return cases
}
func encodeRequests(cases []request, checked bool) []byte {
	var b bytes.Buffer
	b.WriteString("CGSR")
	write32(&b, uint32(len(cases)))
	flag := byte(0)
	if checked {
		flag = 1
	}
	for _, r := range cases {
		b.WriteByte(r.Kind)
		b.WriteByte(flag)
		b.WriteByte(r.Constructor)
		text(&b, r.Req)
		text(&b, r.Version)
		if r.Raw != nil {
			b.WriteByte(1)
		} else {
			b.WriteByte(0)
		}
		write32(&b, uint32(len(r.Raw)))
		for _, c := range r.Raw {
			b.WriteByte(c.Op)
			binary.Write(&b, binary.LittleEndian, c.Major)
			for _, n := range []*uint64{c.Minor, c.Patch} {
				if n == nil {
					b.WriteByte(0)
				} else {
					b.WriteByte(1)
					binary.Write(&b, binary.LittleEndian, *n)
				}
			}
			text(&b, c.Pre)
		}
		write32(&b, uint32(len(r.Actions)))
		for _, a := range r.Actions {
			b.WriteByte(a.Op)
			text(&b, a.Value)
		}
	}
	return b.Bytes()
}

type manifest struct {
	Format             int               `json:"format"`
	Revision           string            `json:"revision"`
	Sources            []sourcePin       `json:"sources"`
	Dependencies       []dependency      `json:"registry_dependencies"`
	Rustc              string            `json:"rustc"`
	Go                 string            `json:"generation_go"`
	Host               string            `json:"generation_host"`
	PointerBits        int               `json:"pointer_bits"`
	Cases              int               `json:"cases"`
	Profiles           map[string]int    `json:"profiles"`
	Kinds              map[string]int    `json:"case_kinds"`
	OriginalTests      int               `json:"original_tests"`
	FormattingCases    int               `json:"formatting_cases"`
	DebugWriterCases   int               `json:"debug_writer_cases"`
	DebugDispatchCases int               `json:"debug_dispatch_cases"`
	DebugStateCases    int               `json:"debug_state_cases"`
	Hashes             map[string]string `json:"sha256"`
}

func run(root, sourceRoot, cargoHome, cargo, rustc string, check bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	if strconv.IntSize != 64 {
		return fmt.Errorf("original regeneration requires 64-bit host")
	}
	version, err := command(ctx, root, nil, rustc, nil, "--version")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(version)) != rustVersion {
		return fmt.Errorf("expected %s, found %s", rustVersion, version)
	}
	originals := make([][]byte, len(sourcePins))
	for i, pin := range sourcePins {
		originals[i], err = pinnedSource(ctx, sourceRoot, pin)
		if err != nil {
			return err
		}
	}
	lock, err := os.ReadFile(filepath.Join(root, toolPath, "Cargo.lock"))
	if err != nil {
		return err
	}
	deps := lockDependencies(lock)
	if len(deps) != 1 || deps[0].Name != "semver" || deps[0].Version != "1.0.28" {
		return fmt.Errorf("expected only original semver1.0.28 dependency: %+v", deps)
	}
	found := false
	for _, d := range lockDependencies(originals[2]) {
		if d == deps[0] {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("semver differs from original Cargo.lock")
	}
	temp, err := os.MkdirTemp("", "cargo-semver-oracle-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	if err := os.MkdirAll(filepath.Join(temp, "util"), 0700); err != nil {
		return err
	}
	hashes := map[string]string{}
	for _, name := range []string{"Cargo.toml", "Cargo.lock", "oracle.rs", "format_oracle.rs", "util/cargo_debug_probe.rs", "util/cargo_debug_dispatch.rs", "util/cargo_debug_state.rs", "util/mod.rs", "main.go"} {
		data, err := os.ReadFile(filepath.Join(root, toolPath, name))
		if err != nil {
			return err
		}
		hashes[name] = digest(data)
		if name != "main.go" {
			if err := os.WriteFile(filepath.Join(temp, name), data, 0600); err != nil {
				return err
			}
		}
	}
	for i, name := range []string{"semver_ext.rs", "semver_eval_ext.rs"} {
		if err := os.WriteFile(filepath.Join(temp, "util", name), originals[i], 0600); err != nil {
			return err
		}
	}
	baseEnv := map[string]string{"RUSTC": rustc, "CARGO_HOME": cargoHome, "CARGO_TARGET_DIR": filepath.Join(temp, "checked"), "CARGO_NET_OFFLINE": "true", "RUSTUP_TOOLCHAIN": "1.99.1", "CARGO_PROFILE_DEV_OVERFLOW_CHECKS": "true", "FERRUGO_EXPECT_CHECKED": "true"}
	meta, err := command(ctx, temp, environment(baseEnv), cargo, nil, "metadata", "--locked", "--offline", "--format-version=1")
	if err != nil {
		return err
	}
	if err := verifyArchives(meta, cargoHome, deps); err != nil {
		return err
	}
	tests, err := command(ctx, temp, environment(baseEnv), cargo, nil, "test", "--locked", "--offline", "--bin", "oracle")
	if err != nil {
		return err
	}
	if !bytes.Contains(tests, []byte("11 passed; 0 failed")) {
		return fmt.Errorf("not all11 original inline tests ran: %s", tests)
	}
	cases := requests()
	kinds := map[string]int{"matching": 0, "mutators_hash": 0, "custom_traits": 0}
	for _, r := range cases {
		switch r.Kind {
		case 0:
			kinds["mutators_hash"]++
		case 1:
			kinds["custom_traits"]++
		case 2:
			kinds["matching"]++
		}
	}
	var corpus bytes.Buffer
	var formatRaw, debugRaw, debugDispatchRaw, debugStateRaw []byte
	corpus.WriteString("CGSC")
	write32(&corpus, 2)
	for _, checked := range []bool{true, false} {
		profile := "checked"
		if !checked {
			profile = "wrapping"
		}
		baseEnv["CARGO_TARGET_DIR"] = filepath.Join(temp, profile)
		baseEnv["CARGO_PROFILE_DEV_OVERFLOW_CHECKS"] = strconv.FormatBool(checked)
		baseEnv["FERRUGO_EXPECT_CHECKED"] = strconv.FormatBool(checked)
		env := environment(baseEnv)
		if _, err := command(ctx, temp, env, cargo, nil, "build", "--locked", "--offline", "--bins"); err != nil {
			return err
		}
		request := encodeRequests(cases, checked)
		response, err := command(ctx, temp, env, filepath.Join(temp, profile, "debug", "oracle"), request)
		if err != nil {
			return err
		}
		if len(response) < 8 || string(response[:4]) != "CGSO" || binary.LittleEndian.Uint32(response[4:8]) != uint32(len(cases)) {
			return fmt.Errorf("bad original response")
		}
		hashes[profile+"_requests.raw"] = digest(request)
		hashes[profile+"_responses.raw"] = digest(response)
		write32(&corpus, uint32(len(request)))
		corpus.Write(request)
		write32(&corpus, uint32(len(response)))
		corpus.Write(response)
		if checked {
			formatRaw, err = command(ctx, temp, env, filepath.Join(temp, profile, "debug", "format-oracle"), nil)
			if err != nil {
				return err
			}
			debugRaw, err = command(ctx, temp, env, filepath.Join(temp, profile, "debug", "debug-oracle"), nil)
			if err != nil {
				return err
			}
			debugDispatchRaw, err = command(ctx, temp, env, filepath.Join(temp, profile, "debug", "debug-dispatch-oracle"), nil)
			if err != nil {
				return err
			}
			debugStateRaw, err = command(ctx, temp, env, filepath.Join(temp, profile, "debug", "debug-state-oracle"), nil)
			if err != nil {
				return err
			}
		}
	}
	if len(formatRaw) < 8 || string(formatRaw[:4]) != "CGSF" {
		return fmt.Errorf("bad original format stream")
	}
	formatCount := int(binary.LittleEndian.Uint32(formatRaw[4:8]))
	debugCount := bytes.Count(debugRaw, []byte{'\n'})
	if debugCount != 8668 {
		return fmt.Errorf("expected8668 original Debug writer records, got%d", debugCount)
	}
	debugDispatchCount := bytes.Count(debugDispatchRaw, []byte{'\n'})
	if debugDispatchCount != 8668 {
		return fmt.Errorf("expected8668 original Debug dispatch records, got%d", debugDispatchCount)
	}
	debugStateCount := bytes.Count(debugStateRaw, []byte{'\n'})
	if debugStateCount != 8668 {
		return fmt.Errorf("expected8668 original mutable Debug state records, got%d", debugStateCount)
	}
	raw := corpus.Bytes()
	name := "testdata/semver_cases.bin.gz"
	path := filepath.Join(root, targetPath, name)
	old, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	compressed, err := retainGzip(raw, old, check)
	if err != nil {
		return err
	}
	hashes[name] = digest(compressed)
	hashes[name+".raw"] = digest(raw)
	files := map[string][]byte{name: compressed}
	for name, raw := range map[string][]byte{"testdata/semver_format_cases.bin.gz": formatRaw, "testdata/semver_debug_probes.tsv.gz": debugRaw, "testdata/semver_debug_dispatch.tsv.gz": debugDispatchRaw, "testdata/semver_debug_state.tsv.gz": debugStateRaw} {
		previous, readErr := os.ReadFile(filepath.Join(root, targetPath, name))
		if readErr != nil && !os.IsNotExist(readErr) {
			return readErr
		}
		data, err := retainGzip(raw, previous, check)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		files[name] = data
		hashes[name] = digest(data)
		hashes[name+".raw"] = digest(raw)
	}
	m := manifest{1, revision, sourcePins, deps, rustVersion, runtime.Version(), runtime.GOOS + "/" + runtime.GOARCH, 64, len(cases) * 2, map[string]int{"checked": len(cases), "wrapping": len(cases)}, kinds, 11, formatCount, debugCount, debugDispatchCount, debugStateCount, hashes}
	manifestPath := filepath.Join(root, toolPath, "manifest.json")
	if check {
		old, err := os.ReadFile(manifestPath)
		if err != nil {
			return err
		}
		var previous manifest
		if err := json.Unmarshal(old, &previous); err != nil {
			return err
		}
		m.Go, m.Host = previous.Go, previous.Host
	}
	metadata, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	metadata = append(metadata, '\n')
	if check {
		previous, err := os.ReadFile(manifestPath)
		if err != nil || !bytes.Equal(metadata, previous) {
			return fmt.Errorf("manifest differs")
		}
	} else {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return err
		}
		for name, data := range files {
			if err := os.WriteFile(filepath.Join(root, targetPath, name), data, 0644); err != nil {
				return err
			}
		}
		if err := os.WriteFile(manifestPath, metadata, 0644); err != nil {
			return err
		}
	}
	fmt.Printf("validated11 original tests and %d requests (%d per checked/wrapping profile),%d formatting cases,%d Debug writer+%d char-dispatch+%d mutable-state records,%s\n", len(cases)*2, len(cases), formatCount, debugCount, debugDispatchCount, debugStateCount, runtime.Version())
	return nil
}
func main() {
	rootFlag := flag.String("root", ".", "repository root")
	sourceFlag := flag.String("cargo-source", "", "pinned Cargo source Git cache (FERRUGO_CARGO_SOURCE)")
	homeFlag := flag.String("cargo-home", "", "Cargo registry cache (CARGO_HOME)")
	cargoFlag := flag.String("cargo", "", "Cargo executable (FERRUGO_CARGO)")
	rustcFlag := flag.String("rustc", "", "Rust executable (FERRUGO_RUSTC)")
	check := flag.Bool("check", false, "verify original-source fixtures without changing files")
	flag.Parse()
	if *rootFlag == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "root must be nonempty and positional arguments unsupported")
		os.Exit(1)
	}
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	root, err := filepath.Abs(*rootFlag)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cargo, err := selected(root, "FERRUGO_CARGO", *cargoFlag, filepath.Join(root, ".toolchain/rust/bin/cargo"), "cargo", explicit["cargo"])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	rustc, err := selected(root, "FERRUGO_RUSTC", *rustcFlag, filepath.Join(root, ".toolchain/rust/bin/rustc"), "rustc", explicit["rustc"])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	source, err := selectedDirectory(root, "FERRUGO_CARGO_SOURCE", *sourceFlag, ".toolchain/upstream-cargo", explicit["cargo-source"])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	home, err := selectedDirectory(root, "CARGO_HOME", *homeFlag, ".toolchain/port-cargo-home", explicit["cargo-home"])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := run(root, source, home, cargo, rustc, *check); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
