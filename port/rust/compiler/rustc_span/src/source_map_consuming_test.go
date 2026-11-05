// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type consumingIntoRecord struct {
	Mode   string   `json:"mode"`
	Action string   `json:"action"`
	Abort  bool     `json:"abort"`
	Events []string `json:"events"`
}
type consumingIntoProof struct {
	Records   int                                `json:"records"`
	Aborts    int                                `json:"actual_SIGABRT"`
	Locations map[string]*rustfmt.SourceLocation `json:"locations"`
	SHA       string                             `json:"gzip_sha256"`
	Bindings  map[string]string                  `json:"go_bindings"`
}

func loadConsumingIntoProof(t *testing.T) consumingIntoProof {
	t.Helper()
	data, err := os.ReadFile("../../../../../tools/port-rust-source-map/consuming/VALIDATION.json")
	if err != nil {
		t.Fatal(err)
	}
	var p consumingIntoProof
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatal(err)
	}
	for path, want := range p.Bindings {
		data, err := os.ReadFile("../../../../../" + path)
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) != want {
			t.Fatalf("consuming source binding changed:%s", path)
		}
	}
	return p
}

type consumingIntoInput struct {
	Path    CowPath
	Value   uint
	Counter Shared[*atomic.Uintptr]
	Action  string
}
type consumingIntoState struct {
	request  consumingIntoRecord
	proof    consumingIntoProof
	retained *rustvec.Owned[consumingIntoInput]
	emit     func(string)
}

func (s *consumingIntoState) event(format string, args ...any) { s.emit(fmt.Sprintf(format, args...)) }
func (s *consumingIntoState) drop(value *consumingIntoInput, c rustfmt.Context) {
	s.event("self-drop:%t:%d:%t", value.Path.Path.IsEmpty(), value.Value, c.Panicking())
	s.event("token-drop:%d:%t", value.Value, c.Panicking())
	(*value.Counter.Borrow()).Add(1)
	value.Counter.CloseWithContext(c)
}
func (s *consumingIntoState) convert(mapping *FilePathMapping, working *RealFileName, token Shared[*atomic.Uintptr], c rustfmt.Context) RealFileName {
	receiver := rustvec.NewOwned(consumingIntoInput{Path: OwnedCowPath(NewRustPath("src/input.rs")), Value: 7, Counter: token, Action: s.request.Action}, rustvec.ValueOps[consumingIntoInput]{Drop: s.drop})
	return ToRealFilenameConsuming(mapping, working, receiver, IntoCowPathOperation[consumingIntoInput]{Into: func(self *rustvec.Owned[consumingIntoInput], current rustfmt.Context) CowPath {
		value := self.Borrow()
		s.event("into:%s:%d:%t", value.Action, value.Value, current.Panicking())
		value.Value += 100
		result := value.Path
		value.Path = BorrowedCowPath(BorrowedRustPath(""))
		s.event("prepared:%t:%t", result.IsOwned(), current.Panicking())
		switch value.Action {
		case "take":
			s.event("take-self")
			s.retained = self.MoveWithContext(current)
		case "forget", "forget_panic":
			s.event("forget-self")
			_ = self.Take()
			if s.request.Action == "forget_panic" {
				s.raise("convert", current)
			}
		case "drop":
		default:
			panic("invalid Into action")
		}
		return result
	}}, c)
}
func (s *consumingIntoState) raise(tag string, c rustfmt.Context) {
	site := tag
	if tag == "outer" && s.request.Mode == "processing" {
		site = "outer_processing"
	}
	c.PanicAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicStaticStr, Payload: tag, Location: c.SourcePanicLocation(site), CanUnwind: true})
}
func (s *consumingIntoState) operation(c rustfmt.Context) {
	counter := NewShared(new(atomic.Uintptr), nil)
	defer counter.CloseWithContext(c)
	mapping := NewFilePathMapping([]RustPathMappingEntry{{From: NewRustPath("src"), To: NewRustPath("V")}}, AllRemapPathScopeComponents())
	empty := EmptyFilePathMapping()
	initial := EmptyRealFileName()
	working := empty.ToRealFilename(&initial, NewRustPath("/wd"))
	caught := false
	func() {
		defer func() {
			if recover() != nil {
				caught = true
			}
		}()
		result := s.convert(&mapping, &working, counter.CloneWithContext(c), c)
		text := rustfmt.String(func(f *rustfmt.Formatter) error { f.Context = c; return result.WriteDebugFormatter(f) })
		s.event("result:%s:%t", text, c.Panicking())
	}()
	s.event("caught:%t:%t", caught, c.Panicking())
	s.event("state:%d:%d", (*counter.Borrow()).Load(), counter.StrongCount())
	if s.retained != nil {
		s.retained.DropWithContext(c)
		s.retained = nil
	}
	s.event("final:%d:%d", (*counter.Borrow()).Load(), counter.StrongCount())
}
func (s *consumingIntoState) escaped(c rustfmt.Context) {
	token := NewShared(new(atomic.Uintptr), nil)
	mapping := EmptyFilePathMapping()
	initial := EmptyRealFileName()
	working := mapping.ToRealFilename(&initial, NewRustPath("/wd"))
	result := s.convert(&mapping, &working, token, c)
	text := rustfmt.String(func(f *rustfmt.Formatter) error { f.Context = c; return result.WriteDebugFormatter(f) })
	s.event("escaped-result:%s", text)
	if s.retained != nil {
		s.retained.DropWithContext(c)
		s.retained = nil
	}
}
func (s *consumingIntoState) run() {
	cleanup := rustfmt.CleanupPanicInfoNightly20261104()
	if site := s.proof.Locations["panic in a destructor during cleanup"]; site != nil {
		cleanup.Location = site
	}
	c := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{LocationCells: s.proof.Locations, CleanupAbort: &cleanup, Hook: func(info rustfmt.SourcePanicInfo, current rustfmt.Context) {
		if info.Location == nil {
			panic("missing current source profile location")
		}
		s.event("hook:%v:%t:%s:%d:%d", info.Payload, current.Panicking(), info.Location.File, info.Location.Line, info.Location.Column)
		if s.request.Mode == "processing" {
			s.operation(current)
		}
	}})
	switch s.request.Mode {
	case "normal":
		s.operation(c)
	case "processing":
		func() { defer func() { recover() }(); s.raise("outer", c) }()
		s.event("outer-end")
	case "inherited_caught", "inherited_escape":
		caught := false
		func() {
			defer func() {
				if recover() != nil {
					caught = true
				}
			}()
			defer spanCloseOwnedCallback(c, func(current rustfmt.Context) {
				s.event("outer-drop:%t", current.Panicking())
				if s.request.Mode == "inherited_caught" {
					s.operation(current)
				} else {
					s.escaped(current)
				}
			})
			s.raise("outer", c)
		}()
		s.event("outer-caught:%t", caught)
	}
}
func TestSourceMapConsumingIntoChild(t *testing.T) {
	encoded, ok := os.LookupEnv("FERRUGO_CONSUMING_INTO_CHILD")
	if !ok {
		return
	}
	var q consumingIntoRecord
	if err := json.Unmarshal([]byte(encoded), &q); err != nil {
		t.Fatal(err)
	}
	s := consumingIntoState{request: q, proof: loadConsumingIntoProof(t), emit: func(line string) { fmt.Println(line) }}
	s.run()
	os.Exit(0)
}
func TestSourceMapConsumingIntoNativeOracle(t *testing.T) {
	p := loadConsumingIntoProof(t)
	if p.Records != 16 || p.Aborts != 2 {
		t.Fatalf("incomplete consuming proof:%d/%d", p.Records, p.Aborts)
	}
	data, err := os.ReadFile("testdata/source_map_consuming.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != p.SHA {
		t.Fatal("consuming fixture identity differs")
	}
	z, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scan := bufio.NewScanner(z)
	count, aborts := 0, 0
	for scan.Scan() {
		var q consumingIntoRecord
		if err := json.Unmarshal(scan.Bytes(), &q); err != nil {
			t.Fatal(err)
		}
		var events []string
		if q.Abort {
			encoded, _ := json.Marshal(q)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSourceMapConsumingIntoChild$")
			cmd.Env = append(os.Environ(), "FERRUGO_CONSUMING_INTO_CHILD="+string(encoded))
			var out, stderr bytes.Buffer
			cmd.Stdout = &out
			cmd.Stderr = &stderr
			err := cmd.Run()
			cancel()
			exit, ok := err.(*exec.ExitError)
			if !ok || exit.ExitCode() != 134 {
				t.Fatalf("record%d exit%v stderr%s", count, err, stderr.String())
			}
			events = strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
			aborts++
		} else {
			s := consumingIntoState{request: q, proof: p, emit: func(line string) { events = append(events, line) }}
			s.run()
		}
		if !reflect.DeepEqual(events, q.Events) {
			t.Fatalf("%s/%s got%q want%q", q.Mode, q.Action, events, q.Events)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 16 || aborts != 2 {
		t.Fatalf("consuming records:%d/%d", count, aborts)
	}
	t.Logf("%d original consuming Into cases / %d isolated aborts", count, aborts)
}
