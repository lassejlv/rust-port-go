// Command port-core-unicode translates the complete pinned core Unicode file.
package main

import (
	"bytes"
	"compress/gzip"
	"crypto/sha1"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const revision = "2d20b42b883dcfc014f3c2c56081eeb6091c2179"
const sourceBlob = "84414a72df88c4718d040cb93777d7a4b265e8d4"
const sourcePath = "library/core/src/unicode/unicode_data.rs"

//go:embed algorithms.go.tmpl
var algorithms []byte

//go:embed oracle.rs
var oracleShim []byte

type options struct {
	root, source, rustc string
	check               bool
}

type staticTable struct {
	Module       string `json:"module"`
	Name         string `json:"rust_name"`
	GoName       string `json:"go_name"`
	Entries      int    `json:"entries"`
	SourceSHA256 string `json:"source_definition_sha256"`
	type_, value string
}

type manifest struct {
	Format           int           `json:"format"`
	RustRevision     string        `json:"rust_revision"`
	SourcePath       string        `json:"source_path"`
	SourceBlob       string        `json:"source_blob"`
	SourceSHA256     string        `json:"source_sha256"`
	SourceLines      int           `json:"source_lines"`
	UnicodeVersion   [3]uint8      `json:"unicode_version"`
	ScalarValues     int           `json:"scalar_values"`
	Predicates       []string      `json:"predicates"`
	Conversions      []string      `json:"conversions"`
	OracleEncoding   string        `json:"oracle_encoding"`
	OracleShimSHA256 string        `json:"oracle_shim_sha256"`
	OracleSHA256     string        `json:"oracle_sha256"`
	FixtureSHA256    string        `json:"fixture_gzip_sha256"`
	GoFileSHA256     string        `json:"go_file_sha256"`
	License          string        `json:"license"`
	Copyright        string        `json:"copyright"`
	LicenseSHA256    string        `json:"license_sha256"`
	ReuseAnnotation  string        `json:"reuse_annotation"`
	Tables           []staticTable `json:"tables"`
}

type moduleSpec struct{ rust, goName string }

var modules = []moduleSpec{
	{"alphabetic", "AlphabeticLookup"}, {"case_ignorable", "CaseIgnorableLookup"},
	{"cf", "CFLookup"}, {"cn_planes_0_3", "CNPlanes03Lookup"},
	{"default_ignorable_code_point", "DefaultIgnorableCodePointLookup"},
	{"grapheme_extend", "GraphemeExtendLookup"}, {"lowercase", "LowercaseLookup"},
	{"lt", "LTLookup"}, {"n", "NLookup"}, {"uppercase", "UppercaseLookup"},
	{"white_space", "WhiteSpaceLookup"}, {"xid_continue", "XIDContinueLookup"}, {"xid_start", "XIDStartLookup"},
}

func main() {
	var opts options
	flag.StringVar(&opts.root, "root", ".", "repository root")
	flag.StringVar(&opts.source, "source", "", "optional original Rust source (verified against the pinned Git blob)")
	flag.StringVar(&opts.rustc, "rustc", os.Getenv("FERRUGO_RUSTC"), "Rust compiler (FERRUGO_RUSTC, local toolchain, then PATH)")
	flag.BoolVar(&opts.check, "check", false, "verify outputs without changing tracked files")
	flag.Parse()
	if err := run(opts); err != nil {
		fmt.Fprintln(os.Stderr, "port-core-unicode:", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	root, err := filepath.Abs(opts.root)
	if err != nil {
		return err
	}
	var source []byte
	if opts.source != "" {
		path := opts.source
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		source, err = os.ReadFile(path)
	} else {
		source, err = gitFile(root, sourceBlob)
	}
	if err != nil {
		return err
	}
	gitHash := sha1.New()
	fmt.Fprintf(gitHash, "blob %d\x00", len(source))
	gitHash.Write(source)
	if got := hex.EncodeToString(gitHash.Sum(nil)); got != sourceBlob {
		return fmt.Errorf("source Git blob %s, expected %s", got, sourceBlob)
	}
	generated, tables, err := translate(source)
	if err != nil {
		return err
	}
	rustc := opts.rustc
	if rustc == "" {
		rustc = filepath.Join(root, ".toolchain/rust/bin/rustc")
		if _, err := os.Stat(rustc); err != nil {
			rustc = "rustc"
		}
	}
	if !strings.ContainsAny(rustc, `/\`) {
		rustc, err = exec.LookPath(rustc)
		if err != nil {
			return err
		}
	} else if !filepath.IsAbs(rustc) {
		rustc = filepath.Join(root, rustc)
	}
	oracleDir := filepath.Join(root, ".toolchain/oracles/core-unicode")
	for name, data := range map[string][]byte{"main.rs": oracleShim, "unicode_data.rs": source} {
		if err := writeFile(filepath.Join(oracleDir, name), data); err != nil {
			return err
		}
	}
	binaryPath := filepath.Join(oracleDir, "oracle")
	cmd := exec.Command(rustc, "--edition=2024", "-O", "-C", "debug-assertions=yes", "--crate-name", "ferrugo_core_unicode_oracle", filepath.Join(oracleDir, "main.rs"), "-o", binaryPath)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compile untouched core Unicode source: %w", err)
	}
	cmd = exec.Command(binaryPath)
	cmd.Stderr = os.Stderr
	oracle, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("run untouched core Unicode source: %w", err)
	}
	if len(oracle) < 16 || string(oracle[:8]) != "FERRUGOC" || !bytes.Equal(oracle[8:12], []byte{18, 0, 0, 0}) {
		return fmt.Errorf("invalid core Unicode oracle output")
	}
	packageDir := filepath.Join(root, "port/rust/library/core/src/unicode")
	fixturePath := filepath.Join(packageDir, "testdata/core_unicode_oracle.bin.gz")
	compressed, err := oracleFixture(fixturePath, oracle, opts.check)
	if err != nil {
		return err
	}
	license, err := gitFile(root, revision+":LICENSES/Unicode-3.0.txt")
	if err != nil {
		return err
	}
	reuse, err := gitFile(root, revision+":REUSE.toml")
	if err != nil {
		return err
	}
	annotation := ""
	for _, section := range strings.Split(string(reuse), "[[annotations]]") {
		if strings.Contains(section, "path = \""+sourcePath+"\"") {
			annotation = strings.TrimSpace(section)
		}
	}
	if !strings.Contains(annotation, "SPDX-License-Identifier = \"Unicode-3.0\"") || !strings.Contains(annotation, "1991-2024 Unicode, Inc.") {
		return fmt.Errorf("pinned REUSE Unicode annotation missing")
	}
	m := manifest{
		Format: 1, RustRevision: revision, SourcePath: sourcePath, SourceBlob: sourceBlob,
		SourceSHA256: digest(source), SourceLines: bytes.Count(source, []byte{'\n'}),
		UnicodeVersion: [3]uint8{18, 0, 0}, ScalarValues: 1_112_064,
		Conversions:      []string{"ToLower", "ToUpper", "ToTitle", "ToCasefold"},
		OracleEncoding:   "FERRUGOC header, version [u8;4], scalar count u32 LE; each scalar in ascending order excluding surrogates: predicate flags u16 LE, changed-conversion mask u8, then [u32 LE;3] for each changed conversion; unchanged conversion is [input,0,0]; predicates are non-ASCII only",
		OracleShimSHA256: digest(oracleShim), OracleSHA256: digest(oracle), FixtureSHA256: digest(compressed),
		GoFileSHA256: digest(generated), License: "Unicode-3.0", Copyright: "1991-2024 Unicode, Inc.", LicenseSHA256: digest(license), ReuseAnnotation: annotation,
		Tables: tables,
	}
	for _, spec := range modules {
		m.Predicates = append(m.Predicates, spec.goName)
	}
	jsonManifest, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	outputs := map[string][]byte{
		"unicode_data.go":                     generated,
		"unicode_data_manifest.json":          append(jsonManifest, '\n'),
		"testdata/core_unicode_oracle.bin.gz": compressed,
		"LICENSE-UNICODE":                     license,
	}
	for name, data := range outputs {
		path := filepath.Join(packageDir, name)
		if opts.check {
			actual, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !bytes.Equal(actual, data) {
				return fmt.Errorf("generated output differs: %s", path)
			}
		} else if err := writeFile(path, data); err != nil {
			return err
		}
	}
	verb := "generated"
	if opts.check {
		verb = "verified"
	}
	fmt.Printf("%s complete core Unicode 18.0.0 file: 13 predicates, 4 conversions, %d source tables, 1112064 scalar oracle rows\n", verb, len(tables))
	fmt.Printf("oracle SHA-256 %s; fixture %d bytes\n", m.OracleSHA256, len(compressed))
	return nil
}

var lineComments = regexp.MustCompile(`(?m)//[^\n]*`)
var blockComments = regexp.MustCompile(`(?s)/\*.*?\*/`)
var modulePattern = regexp.MustCompile(`pub mod ([a-z_0-9]+) \{`)
var staticPattern = regexp.MustCompile(`(?m)^\s*static ([A-Z_]+): ([^\n]+?) = `)
var numberPattern = regexp.MustCompile(`\b(?:0b[01]+|0x[0-9a-fA-F]+|[0-9]+)\b`)
var minimumPattern = regexp.MustCompile(`\(c as u32\) >= (0x[0-9a-f]+)`)
var headerPattern = regexp.MustCompile(`ShortOffsetRunHeader::new\(([0-9]+),\s*([0-9]+)\)`)
var singlePattern = regexp.MustCompile(`\(Range::(singleton|step_by_1|step_by_2)\((0x[0-9a-f]+)(?:\.\.=(0x[0-9a-f]+))?\),\s*(-?[0-9]+)\)`)
var multiPattern = regexp.MustCompile(`\((0x[0-9a-f]+),\s*\[(0x[0-9a-f]+),\s*(0x[0-9a-f]+),\s*(0x[0-9a-f]+)\]\)`)
var exceptionPattern = regexp.MustCompile(`\((0x[0-9a-f]+),\s*\[('(?:\\u\{[0-9a-f]+\}|[^'])'),\s*('(?:\\u\{[0-9a-f]+\}|[^'])'),\s*('(?:\\u\{[0-9a-f]+\}|[^'])')\]\)`)

func translate(source []byte) (generated []byte, tables []staticTable, err error) {
	defer func() {
		if problem := recover(); problem != nil {
			err = fmt.Errorf("translate pinned Unicode source: %v", problem)
		}
	}()
	clean := lineComments.ReplaceAllString(blockComments.ReplaceAllString(string(source), ""), "")
	moduleMatches := modulePattern.FindAllStringSubmatchIndex(clean, -1)
	if len(moduleMatches) != 14 {
		panic("expected thirteen property modules and conversions")
	}
	var output bytes.Buffer
	output.Write(algorithms)
	for i, matched := range moduleMatches {
		name := clean[matched[2]:matched[3]]
		end := len(clean)
		if i+1 < len(moduleMatches) {
			end = moduleMatches[i+1][0]
		}
		body := clean[matched[1]:end]
		if i < len(modules) && name != modules[i].rust {
			panic("unexpected module " + name)
		}
		if i == len(modules) && name != "conversions" {
			panic("missing conversion module")
		}
		var names []string
		for _, matchedStatic := range staticPattern.FindAllStringSubmatchIndex(body, -1) {
			rustName, typ := body[matchedStatic[2]:matchedStatic[3]], body[matchedStatic[4]:matchedStatic[5]]
			start := matchedStatic[1]
			for body[start] != '[' && body[start] != '{' {
				start++
			}
			value := composite(body, start)
			goName := lowerCamel(name) + upperCamel(strings.ToLower(rustName))
			if name == "conversions" {
				goName = map[string]string{"LOWERCASE_LUT": "lowercaseLUT", "UPPERCASE_LUT": "uppercaseLUT", "TITLECASE_LUT": "titlecaseLUT", "CASEFOLD_LUT": "casefoldLUT"}[rustName]
				if goName == "" {
					panic("unknown conversion LUT " + rustName)
				}
			}
			table := staticTable{Module: name, Name: rustName, GoName: goName, SourceSHA256: digest([]byte(rustName + ":" + typ + "=" + value)), type_: typ, value: value}
			if name == "conversions" {
				table.Entries = 2
				emitLUT(&output, table)
			} else {
				emitPropertyTable(&output, &table)
			}
			tables = append(tables, table)
			names = append(names, rustName)
		}
		if name != "conversions" {
			prefix := lowerCamel(name)
			switch name {
			case "lowercase", "uppercase":
				if strings.Join(names, ",") != "BITSET_CHUNKS_MAP,BITSET_INDEX_CHUNKS,BITSET_CANONICAL,BITSET_MAPPING" {
					panic("unexpected bitset tables")
				}
				minimum := minimumPattern.FindStringSubmatch(body)[1]
				fmt.Fprintf(&output, "func %s(c rune) bool { return validScalar(c) && c >= %s && bitsetSearch(uint32(c), %sBitsetChunksMap[:], %sBitsetIndexChunks[:], %sBitsetCanonical[:], %sBitsetMapping[:]) }\n\n", modules[i].goName, minimum, prefix, prefix, prefix, prefix)
			case "white_space":
				if strings.Join(names, ",") != "WHITESPACE_MAP" {
					panic("unexpected whitespace table")
				}
				fmt.Fprint(&output, `func WhiteSpaceLookup(c rune) bool {
	if !validScalar(c) { return false }
	switch uint32(c) >> 8 {
	case 0: return whiteSpaceWhitespaceMap[int(c)&0xff]&1 != 0
	case 22: return c == 0x1680
	case 32: return whiteSpaceWhitespaceMap[int(c)&0xff]&2 != 0
	case 48: return c == 0x3000
	default: return false
	}
}

`)
			default:
				if strings.Join(names, ",") != "SHORT_OFFSET_RUNS,OFFSETS" {
					panic("unexpected skiplist tables")
				}
				minimum := minimumPattern.FindStringSubmatch(body)[1]
				fmt.Fprintf(&output, "func %s(c rune) bool { return validScalar(c) && c >= %s && skipSearch(c, %sShortOffsetRuns[:], %sOffsets[:]) }\n\n", modules[i].goName, minimum, prefix, prefix)
			}
		} else if strings.Join(names, ",") != "LOWERCASE_LUT,UPPERCASE_LUT,TITLECASE_LUT,CASEFOLD_LUT" {
			panic("unexpected conversion LUTs")
		}
	}
	if len(tables) != 33 {
		panic(fmt.Sprintf("expected all 33 original static tables, got %d", len(tables)))
	}
	generated, err = format.Source(output.Bytes())
	return generated, tables, err
}

func emitPropertyTable(output *bytes.Buffer, table *staticTable) {
	values := numbers(table.value)
	dimensions := numbers(table.type_)
	if len(dimensions) < 1 {
		panic("missing table dimension")
	}
	table.Entries = int(dimensions[len(dimensions)-1])
	fmt.Fprintf(output, "// %s::%s, copied from the original compressed table.\n", table.Module, table.Name)
	switch table.Name {
	case "SHORT_OFFSET_RUNS":
		matches := headerPattern.FindAllStringSubmatch(table.value, -1)
		if len(matches) != table.Entries {
			panic("header count mismatch")
		}
		fmt.Fprintf(output, "var %s = [%d]shortOffsetRunHeader{\n", table.GoName, table.Entries)
		for _, m := range matches {
			fmt.Fprintf(output, "newShortOffsetRunHeader(%s,%s),\n", m[1], m[2])
		}
	case "BITSET_MAPPING", "BITSET_INDEX_CHUNKS":
		width := 2
		if table.Name == "BITSET_INDEX_CHUNKS" {
			width = int(dimensions[0])
		}
		if width != 2 && width != 16 {
			panic("unexpected compressed row width")
		}
		if len(values) != table.Entries*width {
			panic("compressed row count mismatch")
		}
		fmt.Fprintf(output, "var %s = [%d][%d]uint8{\n", table.GoName, table.Entries, width)
		for i := 0; i < len(values); i += width {
			fmt.Fprint(output, "{")
			for _, v := range values[i : i+width] {
				if v > 255 {
					panic("byte overflow")
				}
				fmt.Fprintf(output, "%d,", v)
			}
			fmt.Fprint(output, "},\n")
		}
	case "OFFSETS", "BITSET_CHUNKS_MAP", "WHITESPACE_MAP", "BITSET_CANONICAL":
		if len(values) != table.Entries {
			panic("table count mismatch")
		}
		typeName := "uint8"
		if table.Name == "BITSET_CANONICAL" {
			typeName = "uint64"
		}
		fmt.Fprintf(output, "var %s = [%d]%s{\n", table.GoName, table.Entries, typeName)
		for i, v := range values {
			if typeName == "uint64" {
				fmt.Fprintf(output, "0x%016x,", v)
			} else {
				if v > 255 {
					panic("byte overflow")
				}
				fmt.Fprintf(output, "%d,", v)
			}
			if i%16 == 15 {
				fmt.Fprintln(output)
			}
		}
		fmt.Fprintln(output)
	default:
		panic("unknown table " + table.Name)
	}
	fmt.Fprint(output, "}\n\n")
}

func emitLUT(output *bytes.Buffer, table staticTable) {
	fmt.Fprintf(output, "// conversions::%s, preserving both original planes and all three table kinds.\nvar %s = l1LUT{l2LUTs: [2]l2LUT{\n", table.Name, table.GoName)
	positions := regexp.MustCompile(`L2Lut \{`).FindAllStringIndex(table.value, -1)
	if len(positions) != 2 {
		panic("expected two conversion planes")
	}
	for _, pos := range positions {
		body := composite(table.value, pos[1]-1)
		fmt.Fprint(output, "{singles: []singleConversion{\n")
		value := fieldArray(body, "singles")
		for _, m := range matchedRows(value, singlePattern) {
			end := m[2]
			if m[3] != "" {
				end = m[3]
			}
			parity := m[1] == "step_by_2"
			fmt.Fprintf(output, "{newConversionRange(%s,%s,%t),%s},\n", m[2], end, parity, m[4])
		}
		fmt.Fprint(output, "}, multis: []multiConversion{\n")
		for _, m := range matchedRows(fieldArray(body, "multis"), multiPattern) {
			fmt.Fprintf(output, "{%s,[3]uint16{%s,%s,%s}},\n", m[1], m[2], m[3], m[4])
		}
		fmt.Fprint(output, "}, exceptions: []conversionException{\n")
		for _, m := range matchedRows(fieldArray(body, "exceptions"), exceptionPattern) {
			for i := 2; i <= 4; i++ {
				if strings.HasPrefix(m[i], `'\u{`) {
					m[i] = "0x" + strings.TrimSuffix(strings.TrimPrefix(m[i], `'\u{`), "}'")
				}
			}
			fmt.Fprintf(output, "{%s,[3]rune{%s,%s,%s}},\n", m[1], m[2], m[3], m[4])
		}
		fmt.Fprint(output, "}},\n")
	}
	fmt.Fprint(output, "}}\n\n")
}

func matchedRows(value string, pattern *regexp.Regexp) [][]string {
	var result [][]string
	position := 1
	for _, indices := range pattern.FindAllStringSubmatchIndex(value, -1) {
		if strings.Trim(value[position:indices[0]], " \r\n\t,") != "" {
			panic("unparsed table row: " + value[position:indices[0]])
		}
		var match []string
		for i := 0; i < len(indices); i += 2 {
			if indices[i] < 0 {
				match = append(match, "")
			} else {
				match = append(match, value[indices[i]:indices[i+1]])
			}
		}
		result = append(result, match)
		position = indices[1]
	}
	if strings.Trim(value[position:len(value)-1], " \r\n\t,") != "" {
		panic("unparsed final table row: " + value[position:len(value)-1])
	}
	return result
}

func fieldArray(body, name string) string {
	start := strings.Index(body, name+":")
	if start < 0 {
		panic("missing LUT field " + name)
	}
	start += strings.Index(body[start:], "[")
	return composite(body, start)
}

func composite(text string, start int) string {
	var stack []byte
	for i := start; i < len(text); i++ {
		c := text[i]
		if c == '\'' {
			for i++; i < len(text); i++ {
				if text[i] == '\\' {
					i++
					continue
				}
				if text[i] == '\'' {
					break
				}
			}
			continue
		}
		switch c {
		case '[':
			stack = append(stack, ']')
		case '{':
			stack = append(stack, '}')
		case '(':
			stack = append(stack, ')')
		case ']', '}', ')':
			if len(stack) == 0 || stack[len(stack)-1] != c {
				panic("unbalanced Rust data literal")
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return text[start : i+1]
			}
		}
	}
	panic("unterminated Rust data literal")
}

func numbers(text string) []uint64 {
	var values []uint64
	for _, s := range numberPattern.FindAllString(text, -1) {
		value, err := strconv.ParseUint(s, 0, 64)
		if err != nil {
			panic(err)
		}
		values = append(values, value)
	}
	return values
}

func upperCamel(name string) string {
	var b strings.Builder
	for _, piece := range strings.Split(name, "_") {
		if piece != "" {
			b.WriteString(strings.ToUpper(piece[:1]))
			b.WriteString(piece[1:])
		}
	}
	return b.String()
}

func lowerCamel(name string) string { s := upperCamel(name); return strings.ToLower(s[:1]) + s[1:] }

func gitFile(root, object string) ([]byte, error) {
	cmd := exec.Command("git", "-C", filepath.Join(root, ".toolchain/upstream-rust"), "show", object)
	data, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("read pinned Git object %s: %w", object, err)
	}
	return data, nil
}

func digest(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Go versions can emit different DEFLATE streams for the same input. Preserve
// the existing stream when its checked payload matches; the manifest separately
// verifies the SHA-256 of these exact compressed bytes.
func oracleFixture(path string, oracle []byte, check bool) ([]byte, error) {
	existing, err := os.ReadFile(path)
	if err == nil {
		reader, openErr := gzip.NewReader(bytes.NewReader(existing))
		if openErr == nil {
			decoded, readErr := io.ReadAll(io.LimitReader(reader, int64(len(oracle))+1))
			closeErr := reader.Close()
			if readErr == nil && closeErr == nil && bytes.Equal(decoded, oracle) {
				return existing, nil
			}
		}
		if check {
			return nil, fmt.Errorf("compressed fixture does not contain the exact Rust oracle: %s", path)
		}
	} else if !os.IsNotExist(err) || check {
		return nil, err
	}
	var compressed bytes.Buffer
	writer, err := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(oracle); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return compressed.Bytes(), nil
}
