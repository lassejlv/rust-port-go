// SPDX-License-Identifier: MIT OR Apache-2.0
package astutil_test

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	stablehash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	rustcast "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_ast/src"
	astutil "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_ast/src/util"
	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	serialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	span "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
)

const astProfileEnv = "FERRUGO_AST_UTIL_PROFILE_DIR"
const astEvidenceLimit = 32 << 20

type astFixture struct {
	SHA256    string `json:"sha256"`
	RawSHA256 string `json:"raw_sha256"`
	Bytes     int    `json:"bytes"`
	RawBytes  int    `json:"raw_bytes"`
}
type astProfile struct {
	Debug    bool `json:"debug_assertions"`
	Overflow bool `json:"overflow_checks"`
	Rows     int  `json:"rows"`
	Tags     int  `json:"tags"`
	Aborts   int  `json:"aborts"`
	Fixtures map[string]astFixture
}

type astManifest struct {
	Schema         int
	Revision       string
	Target         string
	UnicodeVersion [3]int            `json:"unicode_version"`
	SourceBindings map[string]string `json:"source_bindings"`
	Profiles       map[string]astProfile
}
type astEvidence struct {
	Root, Dir string
	Manifest  astManifest
	SHA       string
}

func astRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		next := filepath.Dir(dir)
		if next == dir {
			t.Fatal("repository root unavailable")
		}
		dir = next
	}
}
func astRead(path string, limit int) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > int64(limit) {
		return nil, fmt.Errorf("unsafe or oversized AST evidence: %s", path)
	}
	return os.ReadFile(path)
}
func astDirectory(root string, lookup func(string) (string, bool)) (string, error) {
	name, selected := lookup(astProfileEnv)
	if !selected {
		name = filepath.Join(root, "port/rust/compiler/rustc_ast/src/util/testdata")
	} else if strings.TrimSpace(name) == "" {
		return "", errors.New("explicit AST profile directory is empty")
	}
	dir, err := filepath.Abs(name)
	if err != nil {
		return "", err
	}
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", errors.New("AST profile directory is not a directory")
	}
	return dir, nil
}
func loadAstEvidence(t *testing.T) astEvidence {
	t.Helper()
	root := astRoot(t)
	dir, err := astDirectory(root, os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	body, err := astRead(filepath.Join(dir, "ast-util-manifest.json"), 2<<20)
	if err != nil {
		t.Fatal(err)
	}
	var manifest astManifest
	if err = json.Unmarshal(body, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Schema != 1 || manifest.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || manifest.Target != "x86_64-unknown-linux-gnu" || manifest.UnicodeVersion != [3]int{18, 0, 0} || len(manifest.Profiles) != 4 || len(manifest.SourceBindings) < 10 {
		t.Fatal("invalid AST source profile")
	}
	for _, name := range []string{"port/rust/compiler/rustc_ast/src/token_comment_kind.go", "port/rust/compiler/rustc_ast/src/util/case.go", "port/rust/compiler/rustc_ast/src/util/case_types/case.go", "port/rust/compiler/rustc_ast/src/util/comments.go", "port/rust/compiler/rustc_ast/src/util/unicode.go", "tools/port-rust-ast-util/oracle.rs", "tools/port-rust-ast-util/reference.py", "tools/port-rust-ast-util/reference-build/prepare.py"} {
		if manifest.SourceBindings[name] == "" {
			t.Fatalf("missing required AST source binding: %s", name)
		}
	}
	for name, want := range manifest.SourceBindings {
		clean := filepath.Clean(name)
		if filepath.IsAbs(name) || clean != name || strings.HasPrefix(clean, "..") {
			t.Fatal("unsafe AST source binding")
		}
		source, err := astRead(filepath.Join(root, name), astEvidenceLimit)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(source)) != want {
			t.Fatalf("AST source binding drift: %s", name)
		}
	}
	for _, name := range []string{"d0o0", "d0o1", "d1o0", "d1o1"} {
		profile, ok := manifest.Profiles[name]
		if !ok || profile.Debug != (name[1] == '1') || profile.Overflow != (name[3] == '1') || len(profile.Fixtures) != 3 || profile.Tags != 264 || profile.Aborts != 2 {
			t.Fatalf("invalid AST profile %s", name)
		}
	}
	return astEvidence{root, dir, manifest, fmt.Sprintf("%x", sha256.Sum256(body))}
}
func (e astEvidence) fixture(t *testing.T, profile, name string) []byte {
	t.Helper()
	proof, ok := e.Manifest.Profiles[profile].Fixtures[name]
	if !ok || filepath.Base(name) != name || !strings.HasPrefix(name, "ast-util-"+profile) || proof.RawBytes < 0 || proof.RawBytes > astEvidenceLimit || proof.Bytes < 0 || proof.Bytes > astEvidenceLimit {
		t.Fatal("invalid AST fixture declaration")
	}
	body, err := astRead(filepath.Join(e.Dir, name), astEvidenceLimit)
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != proof.Bytes || fmt.Sprintf("%x", sha256.Sum256(body)) != proof.SHA256 {
		t.Fatal("AST compressed evidence differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(reader, astEvidenceLimit+1))
	if err != nil {
		t.Fatal(err)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	if len(raw) != proof.RawBytes || fmt.Sprintf("%x", sha256.Sum256(raw)) != proof.RawSHA256 {
		t.Fatal("AST raw evidence differs")
	}
	return raw
}
func astNumber(t *testing.T, text string) uint64 {
	t.Helper()
	v, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func astUnhex(t *testing.T, text string) string {
	t.Helper()
	v, err := hex.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return string(v)
}
func astBit(value bool) string {
	if value {
		return "1"
	}
	return "0"
}
func astKind(i uint64) rustcast.CommentKind {
	if i == 0 {
		return rustcast.CommentKindLine()
	}
	return rustcast.CommentKindBlock()
}
func astStyle(i uint64) astutil.CommentStyle {
	return []astutil.CommentStyle{astutil.Isolated(), astutil.Trailing(), astutil.Mixed(), astutil.BlankLine()}[i]
}
func astCase(i uint64) astutil.Case {
	return []astutil.Case{astutil.Sensitive(), astutil.Insensitive()}[i]
}

type astRawHasher struct{ Events []string }

func (h *astRawHasher) Write(b []byte)   { h.Events = append(h.Events, "bytes:"+hex.EncodeToString(b)) }
func (h *astRawHasher) Finish64() uint64 { return 0 }

type astTypedHasher struct{ astRawHasher }

func (h *astTypedHasher) WriteIsize(v int) { h.Events = append(h.Events, fmt.Sprintf("isize:%d", v)) }

type astUsizeHasher struct{ astRawHasher }

func (h *astUsizeHasher) WriteUsize(v uint) { h.Events = append(h.Events, fmt.Sprintf("usize:%d", v)) }

var _ stablehash.Hasher = (*astRawHasher)(nil)

type astSink struct {
	Calls []string
	Fail  bool
}

func (s *astSink) Write(b []byte) (int, error) {
	s.Calls = append(s.Calls, "str:"+hex.EncodeToString(b))
	if s.Fail {
		return 0, errors.New("fmt::Error")
	}
	return len(b), nil
}
func (s *astSink) WriteChar(r rune) error {
	s.Calls = append(s.Calls, fmt.Sprintf("char:%x", r))
	if s.Fail {
		return errors.New("fmt::Error")
	}
	return nil
}
func astOptions(index uint64) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	switch index {
	case 1:
		o.Alternate = true
	case 2:
		o.Fill = '*'
		o.Align = rustfmt.AlignCenter
		o.Width = 20
		o.HasWidth = true
		o.Precision = 1
		o.HasPrecision = true
	case 3:
		o.SignPlus = true
		o.Alternate = true
		o.ZeroPad = true
		o.Width = 20
		o.HasWidth = true
		o.DebugUpperHex = true
	}
	return o
}

func astTraitRow(t *testing.T, f []string) string {
	t.Helper()
	i := uint64(0)
	if f[0] != "format" {
		i = astNumber(t, f[1])
	}
	switch f[0] {
	case "comment_layout":
		size, alignment := astutil.CommentStringElementLayout()
		return fmt.Sprintf("comment_layout\t%d\t%d", size, alignment)
	case "format":
		i = astNumber(t, f[2])
		sink := &astSink{Fail: f[4] == "1"}
		formatter := &rustfmt.Formatter{Options: astOptions(astNumber(t, f[3])), Writer: sink}
		before := formatter.Options
		var err error
		switch f[1] {
		case "case":
			err = astCase(i).WriteDebugFormatter(formatter)
		case "style":
			err = astStyle(i).WriteDebugFormatter(formatter)
		case "kind":
			err = astKind(i).WriteDebugFormatter(formatter)
		default:
			t.Fatal("unknown enum")
		}
		if before != formatter.Options {
			t.Fatal("unit Debug mutated formatter state")
		}
		return strings.Join(append(f[:5], astBit(err == nil), strings.Join(sink.Calls, ",")), "\t")
	case "hash":
		raw := &astRawHasher{}
		typed := &astTypedHasher{}
		usize := &astUsizeHasher{}
		v := astKind(i)
		v.Hash(raw)
		v.Hash(typed)
		v.Hash(usize)
		return strings.Join([]string{f[0], f[1], strings.Join(raw.Events, ","), strings.Join(typed.Events, ","), strings.Join(usize.Events, ",")}, "\t")
	case "hash_slice":
		raw := &astRawHasher{}
		typed := &astTypedHasher{}
		v := []rustcast.CommentKind{astKind(i), astKind(i), astKind(1 - i)}
		rustcast.HashCommentKindSlice(v, raw)
		rustcast.HashCommentKindSlice(v, typed)
		return strings.Join([]string{f[0], f[1], strings.Join(raw.Events, ","), strings.Join(typed.Events, ",")}, "\t")
	case "stable":
		h := datastructures.NewStableHasher()
		h.Write([]byte(astUnhex(t, f[2])))
		astKind(i).StableHash(nil, h)
		hash := h.FinishHash()
		return fmt.Sprintf("stable\t%d\t%s\t%d\t%d", i, f[2], hash[0], hash[1])
	case "encode":
		e := span.NewSpanMemEncoder(nil)
		astKind(i).Encode(e)
		position := e.Position()
		body := e.Finish()
		return fmt.Sprintf("encode\t%d\t%d\t%x", i, position, body)
	case "kind_eq":
		j := astNumber(t, f[2])
		return strings.Join(append(f[:3], astBit(astKind(i).Equal(astKind(j))), astBit(astKind(i).NotEqual(astKind(j)))), "\t")
	case "style_eq":
		j := astNumber(t, f[2])
		return strings.Join(append(f[:3], astBit(astStyle(i).Equal(astStyle(j))), astBit(astStyle(i).NotEqual(astStyle(j)))), "\t")
	case "case_eq":
		j := astNumber(t, f[2])
		return strings.Join(append(f[:3], astBit(astCase(i).Equal(astCase(j))), astBit(astCase(i).NotEqual(astCase(j)))), "\t")
	case "kind_clone":
		v := astKind(1 - i)
		v.CloneFrom(astKind(i))
		return strings.Join(append(f[:2], astBit(v.Equal(astKind(i))), astBit(astKind(i).Clone().Equal(astKind(i)))), "\t")
	case "style_clone":
		v := astStyle((i + 1) % 4)
		v.CloneFrom(astStyle(i))
		return strings.Join(append(f[:2], astBit(v.Equal(astStyle(i))), astBit(astStyle(i).Clone().Equal(astStyle(i)))), "\t")
	case "case_clone":
		v := astCase(0)
		v.CloneFrom(astCase(i))
		return strings.Join(append(f[:2], astBit(v.Equal(astCase(i))), astBit(astCase(i).Clone().Equal(astCase(i)))), "\t")
	}
	t.Fatalf("unknown native AST trait row: %s", f[0])
	return ""
}

func TestAstUtilOriginalFullCrateReference(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("retained native trait/hash observations use x86_64-unknown-linux-gnu")
	}
	e := loadAstEvidence(t)
	for _, profile := range []string{"d0o0", "d0o1", "d1o0", "d1o1"} {
		t.Run(profile, func(t *testing.T) {
			session := span.NewSessionGlobals(span.DefaultEdition, nil, span.None[span.SourceMapInputs]())
			defer session.Close()
			body := e.fixture(t, profile, "ast-util-"+profile+".tsv.gz")
			scanner := bufio.NewScanner(bytes.NewReader(body))
			scanner.Buffer(make([]byte, 4096), 1<<20)
			row := 0
			for scanner.Scan() {
				expected := scanner.Text()
				f := strings.Split(expected, "\t")
				actual := ""
				switch f[0] {
				case "B":
					text := astUnhex(t, f[2])
					v := span.InternSymbol(session, text)
					before := span.InternSymbol(session, fmt.Sprintf("__ferrugo_ast_before_%d", row))
					result := astutil.BeautifyDocString(session, v, astKind(astNumber(t, f[1])))
					after := span.InternSymbol(session, fmt.Sprintf("__ferrugo_ast_after_%d", row))
					actual = fmt.Sprintf("B\t%s\t%s\t%x\t%s\t%d\t%s\t%s", f[1], f[2], result.AsStr(session), astBit(result.Equal(v)), after.AsU32()-before.AsU32(), astBit(astutil.MayHaveDocLinks(text)), astBit(astutil.ContainsTextFlowControlChars(text)))
				case "C":
					count := int(astNumber(t, f[3]))
					lines := make([]ruststr.StringCell, count, 17)
					if count > 0 {
						for index, part := range strings.Split(f[15], ",") {
							text := astUnhex(t, part)
							owner := ruststr.NewOwnedStringWithCapacity(uint(len(text)+13), rustfmt.Context{})
							owner.Borrow().PushString(text)
							lines[index] = *owner.TakeCellWithContext(rustfmt.Context{})
						}
					}
					source := astutil.Comment{Style: astStyle(astNumber(t, f[1])), Lines: lines, Pos: span.BytePos(astNumber(t, f[2]))}
					clone := source.Clone()
					replaced := astutil.Comment{Style: astutil.Mixed(), Lines: make([]ruststr.StringCell, 0, 37), Pos: 42}
					replaced.CloneFrom(&source)
					separate, replacedSeparate := true, true
					caps := []string{}
					texts := []string{}
					for index := range clone.Lines {
						line := clone.Lines[index].AsStr()
						if len(line) > 0 && unsafe.StringData(line) == unsafe.StringData(source.Lines[index].AsStr()) {
							separate = false
						}
						if len(line) > 0 && unsafe.StringData(replaced.Lines[index].AsStr()) == unsafe.StringData(source.Lines[index].AsStr()) {
							replacedSeparate = false
						}
						caps = append(caps, strconv.FormatUint(uint64(clone.Lines[index].Capacity()), 10))
						texts = append(texts, hex.EncodeToString([]byte(line)))
					}
					pointerSeparate := unsafe.SliceData(lines) != unsafe.SliceData(clone.Lines)
					actual = fmt.Sprintf("C\t%s\t%s\t%d\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%s", f[1], f[2], len(clone.Lines), cap(clone.Lines), cap(replaced.Lines), astBit(pointerSeparate), astBit(separate), astBit(replacedSeparate), strings.Join(caps, ","), strings.Join(texts, ","), astBit(clone.Style.Equal(source.Style)), astBit(replaced.Style.Equal(source.Style)), clone.Pos.ToU32(), replaced.Pos.ToU32(), f[15])
					if len(lines) > 0 {
						source.Lines[0].PushString("mutated")
						for index := range clone.Lines {
							if clone.Lines[index].AsStr() != replaced.Lines[index].AsStr() {
								t.Fatal("Clone/CloneFrom alias source String cell")
							}
						}
					}
					source.Close()
					clone.Close()
					replaced.Close()
				case "flow_constants":
					parts := []string{}
					for _, c := range astutil.TextFlowControlCharsBorrowed() {
						parts = append(parts, fmt.Sprintf("%x", c))
					}
					actual = "flow_constants\t" + strings.Join(parts, ",")
				default:
					actual = astTraitRow(t, f)
				}
				if actual != expected {
					t.Fatalf("native row %d\ngot  %s\nwant %s", row, actual, expected)
				}
				row++
			}
			if err := scanner.Err(); err != nil {
				t.Fatal(err)
			}
			if row != e.Manifest.Profiles[profile].Rows {
				t.Fatalf("native row count %d", row)
			}
		})
	}
}

func TestAstUtilExhaustiveFlowScalars(t *testing.T) {
	e := loadAstEvidence(t)
	for _, profile := range []string{"d0o0", "d0o1", "d1o0", "d1o1"} {
		raw := e.fixture(t, profile, "ast-util-"+profile+"-flow.bin.gz")
		if len(raw) != 0x110000 {
			t.Fatal("scalar slot count")
		}
		valid := 0
		for n, want := range raw {
			if 0xd800 <= n && n <= 0xdfff {
				if want != 255 {
					t.Fatal("surrogate oracle marker")
				}
				continue
			}
			valid++
			one := string(rune(n))
			texts := [...]string{one, "x" + one, one + "\u2000", "\u2000" + one + "\x00", "\u2000\u2603" + one + "\u202f"}
			var got byte
			for i, text := range texts {
				if astutil.ContainsTextFlowControlChars(text) {
					got |= 1 << i
				}
			}
			if got != want {
				t.Fatalf("%s scalar U+%04X got %02x want %02x", profile, n, got, want)
			}
		}
		if valid != 1112064 {
			t.Fatal(valid)
		}
	}
	first := astutil.TextFlowControlCharsBorrowed()
	if first != astutil.TextFlowControlCharsBorrowed() {
		t.Fatal("constant shared borrow identity")
	}
	copy := astutil.TextFlowControlChars()
	copy[0] = 0
	if first[0] != 0x202a {
		t.Fatal("copy convenience aliases immutable source constant")
	}
}

type astTag struct {
	Tag    uint8    `json:"tag"`
	Mode   string   `json:"mode"`
	Abort  bool     `json:"abort"`
	Events []string `json:"events"`
}

func astTagRows(t *testing.T, e astEvidence, profile string) []astTag {
	t.Helper()
	raw := e.fixture(t, profile, "ast-util-"+profile+"-tags.jsonl.gz")
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	rows := []astTag{}
	for scanner.Scan() {
		var row astTag
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, row)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 264 {
		t.Fatal("tag case count")
	}
	return rows
}
func astTagLocation(t *testing.T, event string) *rustfmt.SourceLocation {
	t.Helper()
	fields := strings.Split(event, ":")
	if len(fields) != 7 || fields[0] != "hook" {
		t.Fatal("invalid native panic event")
	}
	return &rustfmt.SourceLocation{File: astUnhex(t, fields[3]), Line: uint32(astNumber(t, fields[4])), Column: uint32(astNumber(t, fields[5]))}
}
func astReplayTag(t *testing.T, row astTag, emit func(string)) {
	t.Helper()
	events := []string{}
	event := func(s string) { events = append(events, s); emit(s) }
	var tagLocation, outerLocation *rustfmt.SourceLocation
	for _, e := range row.Events {
		if strings.HasPrefix(e, "hook:") {
			if row.Mode != "normal" && outerLocation == nil {
				outerLocation = astTagLocation(t, e)
			} else {
				tagLocation = astTagLocation(t, e)
			}
		}
	}
	ops := &rustfmt.PanicOperations{LocationCells: map[string]*rustfmt.SourceLocation{rustcast.CommentKindDecodeInvalidTagSite: tagLocation}}
	ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, ops)
	var action func(rustfmt.Context)
	ops.Hook = func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
		kind := "opaque"
		if info.Kind == rustfmt.PanicOwnedString {
			kind = "String"
		}
		if info.Kind == rustfmt.PanicStaticStr {
			kind = "str"
		}
		if info.Location == nil {
			t.Fatal("missing explicit native panic location")
		}
		event(fmt.Sprintf("hook:%s:%x:%x:%d:%d:%s", kind, fmt.Sprint(info.Payload), info.Location.File, info.Location.Line, info.Location.Column, astBit(current.Panicking())))
		if row.Mode == "processing" {
			action(current)
		}
	}
	action = func(current rustfmt.Context) {
		body := append([]byte{row.Tag}, []byte(serialize.MagicEndBytes)...)
		decoder, err := span.NewSpanMemDecoderWithContext(nil, body, 0, current)
		if err != nil {
			t.Fatal(err)
		}
		outcome := "panic"
		func() {
			defer func() { _ = recover() }()
			value := rustcast.DecodeCommentKindWithContext(decoder, current)
			outcome = "ok:" + value.RustDebug()
		}()
		event(fmt.Sprintf("result:%s:position:%d:panicking:%s", outcome, decoder.Position(), astBit(current.Panicking())))
	}
	if row.Mode == "normal" {
		action(ctx)
	} else {
		func() {
			defer func() { _ = recover() }()
			if row.Mode == "inherited" {
				defer func() { during := ctx; during.Unwinding = true; action(during) }()
			}
			ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "outer", Kind: rustfmt.PanicStaticStr, Location: outerLocation, CanUnwind: true})
		}()
	}
	if !row.Abort && !reflect.DeepEqual(events, row.Events) {
		t.Fatalf("tag %d/%s\ngot %#v\nwant %#v", row.Tag, row.Mode, events, row.Events)
	}
}
func TestAstUtilCommentKindPanicReference(t *testing.T) {
	e := loadAstEvidence(t)
	if selected := os.Getenv("FERRUGO_AST_UTIL_TAG_CHILD"); selected != "" {
		parts := strings.Split(selected, ":")
		if len(parts) != 2 || os.Getenv("FERRUGO_AST_UTIL_MANIFEST_SHA") != e.SHA {
			t.Fatal("invalid AST child producer selection")
		}
		index := astNumber(t, parts[1])
		rows := astTagRows(t, e, parts[0])
		if index >= uint64(len(rows)) || !rows[index].Abort {
			t.Fatal("invalid AST abort case")
		}
		astReplayTag(t, rows[index], func(s string) { fmt.Fprintln(os.Stderr, "EVENT:"+s) })
		t.Fatal("expected source processing panic abort")
	}
	for _, profile := range []string{"d0o0", "d0o1", "d1o0", "d1o1"} {
		for index, row := range astTagRows(t, e, profile) {
			if !row.Abort {
				astReplayTag(t, row, func(string) {})
				continue
			}
			command := exec.Command(os.Args[0], "-test.run=^TestAstUtilCommentKindPanicReference$")
			command.Env = append(os.Environ(), astProfileEnv+"="+e.Dir, "FERRUGO_AST_UTIL_TAG_CHILD="+profile+":"+strconv.Itoa(index), "FERRUGO_AST_UTIL_MANIFEST_SHA="+e.SHA)
			output, err := command.CombinedOutput()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("expected isolated abort134, got %v: %s", err, output)
			}
			events := []string{}
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "EVENT:") {
					events = append(events, strings.TrimPrefix(line, "EVENT:"))
				}
			}
			if !reflect.DeepEqual(events, row.Events) {
				t.Fatalf("%s tag %d abort events\ngot %#v\nwant %#v", profile, row.Tag, events, row.Events)
			}
		}
	}
}

func TestAstUtilProducerDirectoryFailClosed(t *testing.T) {
	root := astRoot(t)
	if _, err := astDirectory(root, func(string) (string, bool) { return "", true }); err == nil {
		t.Fatal("explicit empty directory accepted")
	}
	if _, err := astDirectory(root, func(string) (string, bool) { return filepath.Join(t.TempDir(), "absent"), true }); err == nil {
		t.Fatal("missing directory accepted")
	}
	dir := t.TempDir()
	resolved, err := astDirectory(root, func(string) (string, bool) { return dir, true })
	if err != nil || resolved != dir {
		t.Fatalf("relocated directory rejected: %s %v", resolved, err)
	}
	target := filepath.Join(dir, "source")
	if err = os.WriteFile(target, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, filepath.Join(dir, "link")); err == nil {
		if _, err = astRead(filepath.Join(dir, "link"), 100); err == nil {
			t.Fatal("evidence symlink accepted")
		}
	}
}

func TestAstUtilProducerRelocationAndCorruption(t *testing.T) {
	if os.Getenv("FERRUGO_AST_UTIL_INTEGRITY_CHILD") == "1" {
		e := loadAstEvidence(t)
		e.fixture(t, "d0o0", "ast-util-d0o0.tsv.gz")
		return
	}
	e := loadAstEvidence(t)
	dir := t.TempDir()
	files := []string{"ast-util-manifest.json"}
	for _, profile := range e.Manifest.Profiles {
		for filename := range profile.Fixtures {
			files = append(files, filename)
		}
	}
	for _, filename := range files {
		body, err := astRead(filepath.Join(e.Dir, filename), astEvidenceLimit)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(dir, filename), body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	run := func(wantSuccess bool) {
		t.Helper()
		command := exec.Command(os.Args[0], "-test.run=^TestAstUtilProducerRelocationAndCorruption$")
		command.Env = append(os.Environ(), astProfileEnv+"="+dir, "FERRUGO_AST_UTIL_INTEGRITY_CHILD=1")
		body, err := command.CombinedOutput()
		if (err == nil) != wantSuccess {
			t.Fatalf("relocated producer success=%v, want %v: %v\n%s", err == nil, wantSuccess, err, body)
		}
	}
	run(true)
	path := filepath.Join(dir, "ast-util-d0o0.tsv.gz")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	corrupt := append([]byte(nil), original...)
	corrupt[len(corrupt)-8] ^= 1
	if err = os.WriteFile(path, corrupt, 0600); err != nil {
		t.Fatal(err)
	}
	run(false)
	if err = os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(dir, "ast-util-manifest.json")
	manifestBody, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest astManifest
	if err = json.Unmarshal(manifestBody, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest.SourceBindings["port/rust/compiler/rustc_ast/src/util/comments.go"] = strings.Repeat("0", 64)
	modified, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(manifestPath, modified, 0600); err != nil {
		t.Fatal(err)
	}
	run(false)
	if err = os.Remove(manifestPath); err != nil {
		t.Fatal(err)
	}
	run(false)
}

func TestAstUtilLiveNativeRegeneration(t *testing.T) {
	if os.Getenv("FERRUGO_AST_UTIL_ORACLE") != "1" {
		t.Skip("opt-in full original Rust reference regeneration")
	}
	root := astRoot(t)
	arguments := []string{"tools/port-rust-ast-util/generate.py", "--check"}
	if dir, ok := os.LookupEnv(astProfileEnv); ok {
		if strings.TrimSpace(dir) == "" {
			t.Fatal("empty producer selection")
		}
		arguments = append(arguments, "--output-dir", dir)
	}
	command := exec.Command("python3", arguments...)
	command.Dir = root
	command.Env = append(os.Environ(), "CARGO_BUILD_JOBS=1")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("native AST regeneration: %v\n%s", err, output)
	}
}
