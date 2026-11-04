// SPDX-License-Identifier: MIT OR Apache-2.0
package validation

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type OwnedCase struct {
	Ambient, Borrow, DropPanic, Processing bool
	Fail, Input, Operation                 int
}

func DecodeOwned(id int, literal bool) OwnedCase {
	var c OwnedCase
	c.Ambient = id%2 != 0
	id /= 2
	c.Borrow = id%2 != 0
	id /= 2
	c.DropPanic = id%2 != 0
	id /= 2
	c.Fail = []int{0, 1, 2, 99}[id%4]
	id /= 4
	if literal {
		c.Input = id % 9
		c.Operation = id / 9
	} else {
		c.Input = id % 7
		c.Operation = id / 7
	}
	return c
}

var LiteralInputs = []string{"", "abc", "a\\z", "🦀\r\x00", "\\\n \n🦀z", "\\x80\\xff\\u{1f980}", "\\\n \u00a0x", "\x00", "\\"}
var PredicateInputs = []string{"", "abc", "a\x00b", "🦀🦀x", "a!b", "\x00", "!!!"}

type OwnedState struct {
	Case  OwnedCase
	Calls int
	Emit  func(string)
	Seen  bool
}

func (s *OwnedState) Call(context rustfmt.Context, value string) {
	s.Calls++
	same := "none"
	if s.Seen {
		same = "same"
	}
	s.Seen = true
	s.Emit(fmt.Sprintf("call:%d:%t:%s:%s", s.Calls, context.Panicking(), same, value))
	if s.Calls == s.Case.Fail {
		context.Panic("callback")
	}
}
func (s *OwnedState) Drop(context rustfmt.Context) {
	defer func() {
		p := recover()
		s.Emit(fmt.Sprintf("field:%t", context.Panicking() || p != nil))
		if p != nil {
			panic(p)
		}
	}()
	same := "none"
	if s.Calls != 0 && !s.Case.Borrow {
		same = "same"
	}
	s.Emit(fmt.Sprintf("drop:%d:%t:%s", s.Calls, context.Panicking(), same))
	if s.Case.DropPanic {
		context.Panic("environment Drop")
	}
}
func RequiredCleanup(f func()) {
	defer func() {
		if recover() != nil {
			os.Exit(134)
		}
	}()
	f()
}

// BorrowScope is the validation caller's implicit local destruction; the
// borrowed library call never owns this Drop. Source field glue lives in Drop.
func BorrowScope(context rustfmt.Context, drop func(rustfmt.Context), body func()) {
	defer func() {
		if p := recover(); p != nil {
			context.Unwinding = true
			RequiredCleanup(func() { drop(context) })
			panic(p)
		}
		drop(context)
	}()
	body()
}
func Caught(body func()) (caught bool) { defer func() { caught = recover() != nil }(); body(); return }
func Ambient(c OwnedCase, emit func(string), body func(rustfmt.Context)) {
	if c.Processing {
		emit("processing:true")
		body(rustfmt.Context{ProcessingPanic: true})
		panic("expected native abort")
	}
	if c.Ambient {
		emit("ambient:true")
		caught := Caught(func() { body(rustfmt.Context{Unwinding: true}) })
		emit(fmt.Sprintf("inner_caught:%t", caught))
		emit("caught:true")
	} else {
		caught := Caught(func() { body(rustfmt.Context{}) })
		emit(fmt.Sprintf("caught:%t", caught))
	}
}
func ProcessingCase(id int, scope string) OwnedCase {
	c := OwnedCase{Processing: true, DropPanic: id%2 != 0, Input: 2, Operation: id / 2}
	if id%2 == 0 {
		c.Fail = 1
	}
	if scope == "lexer" {
		c.Input = 1
		c.Operation = 0
	}
	if scope == "iterator" {
		c.Input = 4
	}
	return c
}
func ReplayOwned(t *testing.T, directory, testName string, run func(int, func(string))) {
	ReplayOwnedNamed(t, directory, "owned", testName, run)
}
func ReplayOwnedNamed(t *testing.T, directory, stem, testName string, run func(int, func(string))) {
	t.Helper()
	if value, ok := os.LookupEnv("FERRUGO_OWNED_CONTRACT_CASE"); ok {
		id, e := strconv.Atoi(value)
		if e != nil {
			t.Fatal(e)
		}
		run(id, func(line string) { fmt.Println(line) })
		os.Exit(0)
	}
	CheckEvidence(t, directory, stem)
	compressed, e := os.ReadFile(filepath.Join(directory, stem+".tsv.gz"))
	if e != nil {
		t.Fatal(e)
	}
	meta, e := os.ReadFile(filepath.Join(directory, stem+"-manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var info struct {
		Records    int    `json:"records"`
		Raw        string `json:"raw_sha256"`
		Compressed string `json:"fixture_sha256"`
	}
	if e = json.Unmarshal(meta, &info); e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(compressed)
	if hex.EncodeToString(sum[:]) != info.Compressed {
		t.Fatal("owned compressed fixture changed")
	}
	reader, e := gzip.NewReader(bytes.NewReader(compressed))
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	raw := new(bytes.Buffer)
	if _, e = raw.ReadFrom(reader); e != nil {
		t.Fatal(e)
	}
	sum = sha256.Sum256(raw.Bytes())
	if hex.EncodeToString(sum[:]) != info.Raw {
		t.Fatal("owned fixture changed")
	}
	scanner := bufio.NewScanner(raw)
	count, aborts := 0, 0
	for scanner.Scan() {
		count++
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) != 3 {
			t.Fatal("invalid owned record")
		}
		id, e := strconv.Atoi(parts[0])
		if e != nil {
			t.Fatal(e)
		}
		expected, e := hex.DecodeString(parts[2])
		if e != nil {
			t.Fatal(e)
		}
		var actual []byte
		if parts[1] == "abort" {
			aborts++
			command := exec.Command(os.Args[0], "-test.run=^"+testName+"$")
			command.Env = append(os.Environ(), "FERRUGO_OWNED_CONTRACT_CASE="+strconv.Itoa(id))
			actual, e = command.Output()
			if status, ok := e.(*exec.ExitError); !ok || status.ExitCode() != 134 {
				t.Fatalf("case%d: required abort, got%v\n%s", id, e, actual)
			}
		} else if parts[1] == "ok" {
			var b strings.Builder
			run(id, func(line string) { fmt.Fprintln(&b, line) })
			actual = []byte(b.String())
		} else {
			t.Fatal("invalid original process status")
		}
		if !bytes.Equal(actual, expected) {
			t.Fatalf("owned case%d\nGo:\n%s\nRust:\n%s", id, actual, expected)
		}
	}
	if e = scanner.Err(); e != nil {
		t.Fatal(e)
	}
	if count != info.Records {
		t.Fatal("truncated ownership fixture")
	}
	t.Logf("compared%d ownership records including%d isolated aborts", count, aborts)
}
