// SPDX-License-Identifier: MIT OR Apache-2.0
package rusthashmap

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustsort"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type guardRequest struct {
	Mode, Action, Hook string
	Resume             bool
	Outcome            string
	Code               int `json:"native_exit_code"`
	Stdout             string
}
type guardEnvelope struct {
	Request   guardRequest
	Locations map[string]rustfmt.SourceLocation
}
type guardWorld struct {
	request   guardRequest
	locations map[string]rustfmt.SourceLocation
}

func (w *guardWorld) log(format string, args ...any) { fmt.Printf(format+"\n", args...) }
func (w *guardWorld) info(message, site string) rustfmt.SourcePanicInfo {
	var location *rustfmt.SourceLocation
	if value, ok := w.locations[site]; ok {
		copy := value
		location = &copy
	}
	return rustfmt.SourcePanicInfo{Payload: message, Kind: rustfmt.PanicStaticStr, Location: location, CanUnwind: true}
}
func (w *guardWorld) raise(c Context, message string) {
	if w.request.Resume {
		c.ResumeUnwind(message)
	}
	c.PanicAt(w.info(message, message))
}
func (w *guardWorld) local(name string, c Context, body func()) {
	defer func() {
		p := recover()
		drop := c
		drop.Unwinding = drop.Unwinding || p != nil
		w.log("drop:%s:%t", name, drop.Panicking())
		if p != nil {
			c.ResumeUnwind(p)
		}
	}()
	body()
}
func (w *guardWorld) report(body func()) {
	var payload any
	func() { defer func() { payload = recover() }(); body() }()
	if payload == nil {
		w.log("caught:none:%t", w.request.Mode != "normal")
	} else {
		w.log("caught:%s:%t", hex.EncodeToString([]byte(fmt.Sprint(payload))), w.request.Mode != "normal")
	}
}

type guardKey struct{ key, id uint }
type guardValue struct{ id uint }
type guardHasher struct{ key uint }

func (h *guardHasher) Write(value []byte) {
	if len(value) == 8 {
		h.key = uint(binary.LittleEndian.Uint64(value))
	}
}
func (h *guardHasher) Finish64() uint64 { return 0 }

type guardBuilder struct{ world *guardWorld }

func (b guardBuilder) Build(c Context) *OwnedHasher {
	h := &guardHasher{}
	return NewOwnedHasher(h, func(_ Hasher, c Context) {
		b.world.log("drop:hasher:%t", c.Panicking())
		if h.key == 1 && b.world.request.Action == "map_hasher_double" {
			b.world.raise(c, "hasher_drop")
		}
	})
}
func (b guardBuilder) Clone(Context) Builder      { return b }
func (b guardBuilder) CloneFrom(Builder, Context) {}
func (b guardBuilder) Close(Context)              {}
func (w *guardWorld) mapOps() (KeyOps[guardKey], ValueOps[guardValue]) {
	keys := KeyOps[guardKey]{
		Hash: func(k *guardKey, h Hasher, c Context) {
			w.log("hash:%d:%t", k.id, c.Panicking())
			HashUsize(h, k.key)
			if k.id == 1 && strings.HasPrefix(w.request.Action, "map_hash") {
				w.local("hash_local", c, func() { w.raise(c, "hash") })
			}
		},
		Equal: func(a, b *guardKey, c Context) bool {
			w.log("eq:%d:%d:%t", a.id, b.id, c.Panicking())
			if a.id == 1 && w.request.Action == "map_eq" {
				w.raise(c, "eq")
			}
			return a.key == b.key
		},
		Drop: func(k *guardKey, c Context) {
			w.log("drop:key%d:%t", k.id, c.Panicking())
			if k.id == 1 && w.request.Action == "map_hash_key_double" {
				w.raise(c, "key_drop")
			}
		},
	}
	values := ValueOps[guardValue]{Drop: func(v *guardValue, c Context) {
		w.log("drop:value%d:%t", v.id, c.Panicking())
		if v.id == 1 && w.request.Action == "map_hash_value_double" {
			w.raise(c, "value_drop")
		}
	}}
	return keys, values
}
func (w *guardWorld) action(c Context) {
	action := w.request.Action
	w.log("call:%s:%t", action, c.Panicking())
	if action == "map_capacity" {
		keys, values := w.mapOps()
		w.report(func() {
			NewMapWithCapacity(^uint(0), Layout{24, 8}, DefaultProfile(), keys, values, guardBuilder{w}, c)
		})
	} else if strings.HasPrefix(action, "map_") {
		keys, values := w.mapOps()
		m := NewMap[guardKey, guardValue](Layout{24, 8}, DefaultProfile(), keys, values, guardBuilder{w})
		m.Insert(guardKey{0, 0}, guardValue{0}, c)
		w.report(func() {
			if action == "map_entry" {
				environment := rustvec.NewOwned(struct{}{}, rustvec.ValueOps[struct{}]{Drop: func(_ *struct{}, c Context) { w.log("drop:entry_env:%t", c.Panicking()) }})
				entry := m.Entry(guardKey{1, 1}, c)
				entry.OrInsertWithOwned(NewOnce(environment, func(_ *struct{}, _ struct{}, c Context) *rustvec.Owned[guardValue] {
					w.local("entry_local", c, func() { w.raise(c, "entry") })
					panic("unreachable")
				}), c)
			} else {
				m.Insert(guardKey{1, 1}, guardValue{1}, c)
			}
		})
		m.Close(c)
	} else if strings.HasPrefix(action, "vec_") {
		type state struct{ next uint }
		ops := rustvec.IteratorOps[state, guardValue]{
			Next: func(s rustvec.Ref[state]) (guardValue, bool) {
				w.log("next:%d:%t", s.Value.next, s.Context.Panicking())
				if s.Value.next == 3 {
					return guardValue{}, false
				}
				index := s.Value.next
				s.Value.next++
				if index == 1 && action == "vec_next" {
					w.local("next_local", s.Context, func() { w.raise(s.Context, "next") })
				}
				return guardValue{index}, true
			},
			SizeHint: func(s rustvec.Ref[state]) rustvec.Hint {
				w.log("hint:%d:%t", s.Value.next, s.Context.Panicking())
				if action == "vec_capacity" {
					return rustvec.Hint{Lower: ^uint(0)}
				}
				n := 3 - s.Value.next
				return rustvec.Hint{Lower: n, Upper: n, HasUpper: true}
			},
			Drop: func(s *state, c Context) {
				w.log("drop:iterator:%d:%t", s.next, c.Panicking())
				if strings.HasPrefix(action, "vec_iter_drop") {
					w.raise(c, "iter_drop")
				}
			},
		}
		var output []guardValue
		w.report(func() {
			cfg := rustvec.DefaultConfig(8)
			cfg.Context = c
			output = rustvec.Collect(rustvec.FromIterator(rustvec.NewIterator(state{}, ops)), cfg, rustvec.ElementOps[guardValue]{Drop: func(value *guardValue, c Context) {
				w.log("drop:collected%d:%t", value.id, c.Panicking())
				if value.id == 0 && action == "vec_iter_drop_double" {
					w.raise(c, "collected_drop")
				}
			}})
		})
		for _, value := range output {
			w.log("drop:collected%d:%t", value.id, c.Panicking())
		}
	} else if action == "sort_compare" {
		values := []guardValue{{2}, {1}, {0}}
		w.report(func() {
			rustsort.Stable(values, rustsort.Type{Size: 8, Align: 8, Freeze: true}, func(a, b *guardValue, c Context) bool {
				w.local("compare_local", c, func() { w.raise(c, "compare") })
				return false
			}, c)
		})
		for _, value := range values {
			w.log("drop:value%d:%t", value.id, c.Panicking())
		}
	} else if action == "sort_invalid" {
		type value struct {
			key int64
			id  int
		}
		var seed uint64 = 2
		values := make([]value, 21)
		for i := range values {
			seed ^= seed << 13
			seed ^= seed >> 7
			seed ^= seed << 17
			values[i] = value{int64(seed % 13), i}
		}
		w.report(func() {
			rustsort.Stable(values, rustsort.Type{Size: 16, Align: 8, Freeze: true, Copy: true}, func(a, b *value, c Context) bool {
				w.log("compare:%d:%d:%t", a.id, b.id, c.Panicking())
				x, y := a.key%3, b.key%3
				return x != y && (x+1)%3 == y
			}, c)
		})
	} else {
		panic("invalid guard action")
	}
	w.log("return:%t", c.Panicking())
}
func TestPanicGuardChild(t *testing.T) {
	data := os.Getenv("FERRUGO_GUARD_PANIC_CHILD")
	if data == "" {
		t.Skip("isolated panic guard child")
	}
	var envelope guardEnvelope
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		t.Fatal(err)
	}
	w := &guardWorld{envelope.Request, envelope.Locations}
	locations := map[string]rustfmt.SourceLocation{}
	for message, id := range map[string]string{"Hash table capacity overflow": PanicCapacityOverflowSite, "capacity overflow": rustvec.PanicCapacityOverflowSite, "user-provided comparison function does not correctly implement a total order": rustsort.PanicOrderViolationSite} {
		locations[id] = w.locations[message]
	}
	cleanup := rustfmt.CleanupPanicInfoNightly20261104()
	operations := &rustfmt.PanicOperations{CleanupAbort: &cleanup, Locations: locations}
	operations.Hook = func(info rustfmt.SourcePanicInfo, c Context) {
		func() {
			file := ""
			var line, column uint32
			if info.Location != nil {
				file, line, column = info.Location.File, info.Location.Line, info.Location.Column
			}
			w.log("HOOK:%s:%s:%t:%t:%t:%s:%d:%d", info.Kind, hex.EncodeToString([]byte(fmt.Sprint(info.Payload))), c.Panicking(), info.CanUnwind, info.ForceNoBacktrace, hex.EncodeToString([]byte(file)), line, column)
			defer func() {
				w.log("drop:hook_local:%t", c.Panicking())
				if w.request.Hook == "drop_panic" {
					w.raise(c, "hook_drop")
				}
			}()
			switch w.request.Hook {
			case "panic":
				w.raise(c, "hook_panic")
			case "resume":
				c.ResumeUnwind("hook_resume")
			}
		}()
		if w.request.Mode == "processing" {
			w.action(c)
		}
	}
	c := rustfmt.WithPanicOperations(Context{}, operations)
	switch w.request.Mode {
	case "normal":
		w.action(c)
	case "unwind":
		func() {
			defer func() { recover() }()
			defer func() {
				p := recover()
				unwind := c
				unwind.Unwinding = true
				w.log("outer_drop:%t", unwind.Panicking())
				w.action(unwind)
				c.ResumeUnwind(p)
			}()
			c.PanicAt(w.info("outer", "outer.unwind"))
		}()
		w.log("outer_end:false")
	case "processing":
		func() { defer func() { recover() }(); c.PanicAt(w.info("outer", "outer.processing")) }()
		w.log("outer_end:false")
	default:
		t.Fatal("invalid guard mode")
	}
	os.Exit(0)
}
func TestOriginalPanicGuards(t *testing.T) {
	const root = "../../../tools/port-rust-unord/panic/"
	data, err := os.ReadFile(root + "manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Cases, Aborts int
		Locations     map[string]rustfmt.SourceLocation
		Fixture       string `json:"fixture_sha256"`
		Raw           string `json:"raw_sha256"`
		Oracle        string `json:"oracle_sha256"`
		Generator     string `json:"generator_sha256"`
		SDK           string `json:"sdk_helper_sha256"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	for path, want := range map[string]string{root + "oracle.rs": manifest.Oracle, root + "generate.py": manifest.Generator, root + "../sort/generate.py": manifest.SDK} {
		b, err := os.ReadFile(path)
		if err != nil || digest(b) != want {
			t.Fatalf("original guard proof binding differs:%s:%v", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/panic_context.jsonl.gz")
	if err != nil || digest(compressed) != manifest.Fixture || manifest.Cases != 312 || manifest.Aborts != 258 {
		t.Fatalf("original guard fixture differs:%v", err)
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(z, 32<<20))
	if err != nil || digest(raw) != manifest.Raw {
		t.Fatalf("original guard fixture body differs:%v", err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	cases, aborts := 0, 0
	for scanner.Scan() {
		var request guardRequest
		if err := json.Unmarshal(scanner.Bytes(), &request); err != nil {
			t.Fatal(err)
		}
		if (request.Outcome == "abort" && request.Code != -6) || (request.Outcome == "completed" && request.Code != 0) {
			t.Fatal("original guard process class differs")
		}
		envelope, _ := json.Marshal(guardEnvelope{request, manifest.Locations})
		cmd := exec.Command(executable, "-test.run=^TestPanicGuardChild$")
		cmd.Env = append(os.Environ(), "FERRUGO_GUARD_PANIC_CHILD="+string(envelope), "GORACE="+os.Getenv("GORACE")+" atexit_sleep_ms=0")
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		outcome := "completed"
		if err != nil {
			if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 134 {
				outcome = "abort"
			} else {
				t.Fatalf("guard child failed:%v:%s", err, stderr.String())
			}
		}
		if outcome != request.Outcome || stdout.String() != request.Stdout {
			t.Fatalf("native guard%d differs mode%s/action%s/resume%t/hook%s\nclass%s/want%s\n got:%s\nwant:%s", cases, request.Mode, request.Action, request.Resume, request.Hook, outcome, request.Outcome, stdout.String(), request.Stdout)
		}
		cases++
		if outcome == "abort" {
			aborts++
		}
	}
	if err := scanner.Err(); err != nil || cases != manifest.Cases || aborts != manifest.Aborts {
		t.Fatalf("guard coverage differs:%d/%d:%v", cases, aborts, err)
	}
}
