// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

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
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unsafe"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/ruststr"
)

type ownedPathNativeRow struct {
	ID       int      `json:"id"`
	Factory  string   `json:"factory"`
	Capacity uint     `json:"capacity"`
	Initial  string   `json:"initial"`
	Ops      []string `json:"ops"`
	Windows  bool     `json:"windows"`
	Want     string   `json:"want"`
}

func ownedPathUnhex(s string) []byte {
	result, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return result
}
func ownedPathHex(s string) string { return hex.EncodeToString([]byte(s)) }
func ownedPathSnapshot(p *RustPathBuf) string {
	cell := p.Borrow()
	return fmt.Sprintf("%d:%d:%s", cell.Len(), cell.Capacity(), ownedPathHex(cell.AsPath().raw))
}
func ownedPathStringSnapshot(s *ruststr.OwnedString) string {
	cell := s.Borrow()
	return fmt.Sprintf("%d:%d:%s", cell.Len(), cell.Capacity(), ownedPathHex(cell.AsStr()))
}
func ownedPathInput(value string, windows bool) *RustOsString {
	data := ownedPathUnhex(value)
	if !windows {
		return RustOsStringFromUnixBuffer(data)
	}
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[2*i:])
	}
	return RustOsStringFromUTF16(units, rustfmt.Context{})
}
func ownedPathRun(q ownedPathNativeRow) string {
	context := rustfmt.Context{}
	flavor := RustPathUnix
	if q.Windows {
		flavor = RustPathWindows
	}
	var path *RustPathBuf
	if q.Factory == "string" {
		s := ruststr.NewOwnedStringWithCapacity(q.Capacity, context)
		s.Borrow().PushString(string(ownedPathUnhex(q.Initial)))
		before := s.Borrow().BufferIdentity()
		path = RustPathBufFromOwnedString(s, flavor, context)
		if before != path.Borrow().BufferIdentity() {
			panic("String->PathBuf did not move the original allocation")
		}
	} else {
		input := ownedPathInput(q.Initial, q.Windows)
		switch q.Factory {
		case "from":
			path = RustPathBufFromPath(input.AsPath(), context)
			input.Close()
		case "wide":
			path = RustPathBufFromOsString(input, context)
		case "cap":
			path = NewRustPathBufWithCapacity(q.Capacity, flavor, context)
			path.Borrow().AsMutOsString().Push(input.AsPath())
			input.Close()
		}
	}
	defer func() { path.Close() }()
	rows := []string{ownedPathSnapshot(path)}
	for _, operation := range q.Ops {
		parts := strings.Split(operation, ",")
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}
		event := "-"
		number := func() uint {
			v, err := strconv.ParseUint(arg, 10, 64)
			if err != nil {
				panic(err)
			}
			return uint(v)
		}
		input := func() *RustOsString { return ownedPathInput(arg, q.Windows) }
		func() {
			defer func() {
				if failure := recover(); failure != nil {
					if !strings.HasPrefix(event, "P:") {
						panic(failure)
					}
				}
			}()
			panicOps := &rustfmt.PanicOperations{Hook: func(info rustfmt.SourcePanicInfo, _ rustfmt.Context) {
				kind := "opaque"
				switch info.Kind {
				case rustfmt.PanicStaticStr:
					kind = "static"
				case rustfmt.PanicOwnedString:
					kind = "owned"
				}
				event = "P:" + kind + ":" + ownedPathHex(fmt.Sprint(info.Payload))
			}}
			cx := rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Panic: panicOps}}
			switch parts[0] {
			case "push", "ospush", "name", "ext", "add", "join":
				value := input()
				defer value.Close()
				switch parts[0] {
				case "push":
					path.Borrow().PushWithContext(value.AsPath(), cx)
				case "ospush":
					path.Borrow().AsMutOsString().PushWithContext(value.AsPath(), cx)
				case "name":
					path.Borrow().SetFileNameWithContext(value.AsPath(), cx)
				case "ext":
					event = fmt.Sprint(path.Borrow().SetExtensionWithContext(value.AsPath(), cx))
				case "add":
					event = fmt.Sprint(path.Borrow().AddExtensionWithContext(value.AsPath(), cx))
				case "join":
					next := path.AsPath().JoinOwned(value.AsPath(), cx)
					path.Close()
					path = next
				}
			case "pop":
				event = fmt.Sprint(path.Borrow().Pop())
			case "clear":
				path.Borrow().Clear()
			case "reserve":
				path.Borrow().AsMutOsString().ReserveWithContext(number(), cx)
			case "exact":
				path.Borrow().AsMutOsString().ReserveExactWithContext(number(), cx)
			case "shrink":
				path.Borrow().AsMutOsString().ShrinkToWithContext(number(), cx)
			case "fit":
				path.Borrow().AsMutOsString().ShrinkToFit()
			case "lower":
				path.Borrow().AsMutOsString().MakeASCIILowercase()
			case "upper":
				path.Borrow().AsMutOsString().MakeASCIIUppercase()
			case "clone":
				copy := path.CloneWithContext(cx)
				event = fmt.Sprintf("C:%s:%t", ownedPathSnapshot(copy), copy.Borrow().BufferIdentity() == path.Borrow().BufferIdentity())
				copy.Close()
			case "clonefrom":
				copy := NewRustPathBufWithCapacity(number(), flavor, cx)
				copy.Borrow().AsMutOsString().Push(path.AsPath().view("123"))
				copy.Borrow().CloneFromWithContext(path.Borrow(), cx)
				event = "F:" + ownedPathSnapshot(copy)
				copy.Close()
			case "move":
				before := path.Borrow().BufferIdentity()
				path = path.MoveWithContext(cx)
				event = fmt.Sprintf("M:%t", before == path.Borrow().BufferIdentity())
			case "round":
				before := path.Borrow().BufferIdentity()
				path = RustPathBufFromOsString(path.IntoOsStringWithContext(cx), cx)
				event = fmt.Sprintf("R:%t", before == path.Borrow().BufferIdentity())
			case "intostring":
				before := path.Borrow().BufferIdentity()
				s, err := path.IntoOsStringWithContext(cx).IntoStringWithContext(cx)
				if err == nil {
					event = fmt.Sprintf("S:%t", before == s.Borrow().BufferIdentity())
					path = RustPathBufFromOwnedString(s, flavor, cx)
				} else {
					event = fmt.Sprintf("E:%t", before == err.Borrow().BufferIdentity())
					path = RustPathBufFromOsString(err, cx)
				}
			case "cowborrow":
				before := path.Borrow().BufferIdentity()
				cow := BorrowedRustCowPath(path.AsPath())
				clone := cow.CloneWithContext(cx)
				if !clone.IsBorrowed() {
					panic("Cow borrowed clone allocated")
				}
				clone.Close()
				own := cow.ToMutWithContext(cx)
				event = fmt.Sprintf("B:%d:%d:%s:%t", own.Len(), own.Capacity(), ownedPathHex(own.AsPath().raw), own.BufferIdentity() == before)
				next := cow.IntoOwnedWithContext(cx)
				path.Close()
				path = next
			case "cowowned":
				before := path.Borrow().BufferIdentity()
				cow := OwnedRustCowPath(path, cx)
				clone := cow.CloneWithContext(cx)
				copy := clone.IntoOwnedWithContext(cx)
				event = "O:" + ownedPathSnapshot(copy)
				copy.Close()
				path = cow.IntoOwnedWithContext(cx)
				event += fmt.Sprintf(":%t", before == path.Borrow().BufferIdentity())
			case "cowclonefrom":
				copy := NewRustPathBufWithCapacity(number(), flavor, cx)
				copy.Borrow().AsMutOsString().Push(path.AsPath().view("123"))
				dest := OwnedRustCowPath(copy, cx)
				source := OwnedRustCowPath(path.CloneWithContext(cx), cx)
				dest.CloneFromWithContext(source, cx)
				result := dest.IntoOwnedWithContext(cx)
				event = "D:" + ownedPathSnapshot(result)
				result.Close()
				source.Close()
			case "lossy":
				cow := path.AsPath().ToStringLossyOwned(cx)
				text := cow.String()
				var identity *byte
				if unsafe.StringData(text) == nil {
					identity = ruststr.OwnedByteBufferIdentity(nil)
				} else {
					identity = unsafe.StringData(text)
				}
				owned, alias := cow.IsOwned(), identity == path.Borrow().BufferIdentity()
				value := cow.IntoOwnedWithContext(cx)
				event = fmt.Sprintf("L:%t:%t:%s", owned, alias, ownedPathStringSnapshot(value))
				value.Close()
			case "cowtrans":
				results := []string{}
				for _, destOwned := range []bool{false, true} {
					for _, sourceOwned := range []bool{false, true} {
						var dest, source *RustCowPath
						if destOwned {
							p := NewRustPathBufWithCapacity(64, flavor, cx)
							p.Borrow().AsMutOsString().Push(path.AsPath().view("old"))
							dest = OwnedRustCowPath(p, cx)
						} else {
							dest = BorrowedRustCowPath(path.AsPath().view("old"))
						}
						if sourceOwned {
							source = OwnedRustCowPath(path.CloneWithContext(cx), cx)
						} else {
							source = BorrowedRustCowPath(path.AsPath())
						}
						dest.CloneFromWithContext(source, cx)
						owned := dest.IsOwned()
						p := dest.IntoOwnedWithContext(cx)
						results = append(results, fmt.Sprintf("%t:%s", owned, ownedPathSnapshot(p)))
						p.Close()
						source.Close()
					}
				}
				event = "V:" + strings.Join(results, "~")
			case "cowstrtrans":
				text := path.AsPath().ToStringLossyOwned(cx)
				results := []string{}
				for _, destOwned := range []bool{false, true} {
					for _, sourceOwned := range []bool{false, true} {
						var dest, source *RustCowString
						if destOwned {
							s := ruststr.NewOwnedStringWithCapacity(64, cx)
							s.Borrow().PushString("old")
							dest = OwnedRustCowString(s, cx)
						} else {
							dest = BorrowedRustCowString("old")
						}
						if sourceOwned {
							source = OwnedRustCowString(ruststr.OwnedStringFromGo(text.String(), cx), cx)
						} else {
							source = BorrowedRustCowString(text.String())
						}
						dest.CloneFromWithContext(source, cx)
						owned := dest.IsOwned()
						s := dest.IntoOwnedWithContext(cx)
						results = append(results, fmt.Sprintf("%t:%s", owned, ownedPathStringSnapshot(s)))
						s.Close()
						source.Close()
					}
				}
				text.Close()
				event = "W:" + strings.Join(results, "~")
			case "lossymut":
				cow := path.AsPath().ToStringLossyOwned(cx)
				cow.ToMutWithContext(cx).PushString(string(ownedPathUnhex(arg)))
				value := cow.IntoOwnedWithContext(cx)
				event = "T:" + ownedPathStringSnapshot(value)
				value.Close()
			case "cowstr":
				value := path.AsPath().ToStringLossyOwned(cx).IntoOwnedWithContext(cx)
				value.Borrow().Reserve(number())
				before := value.Borrow().BufferIdentity()
				cow := OwnedRustCowString(value, cx)
				clone := cow.CloneWithContext(cx).IntoOwnedWithContext(cx)
				event = "U:" + ownedPathStringSnapshot(clone)
				clone.Close()
				value = cow.IntoOwnedWithContext(cx)
				event += fmt.Sprintf(":%s:%t", ownedPathStringSnapshot(value), before == value.Borrow().BufferIdentity())
				value.Close()
			case "stable":
				before := path.Borrow().BufferIdentity()
				path.Borrow().AsMutOsString().MakeASCIILowercase()
				event = fmt.Sprintf("I:%t", before == path.Borrow().BufferIdentity())
			default:
				panic("unknown owning path operation")
			}
		}()
		rows = append(rows, event+"/"+ownedPathSnapshot(path))
	}
	return strings.Join(rows, "|")
}

func TestOwnedPathOriginalSDK101(t *testing.T) {
	directory := filepath.Join("..", "..", "..", "..", "..", "tools", "port-rust-owned-path")
	compressed, err := os.ReadFile(filepath.Join(directory, "native.jsonl.gz"))
	if err != nil {
		t.Fatal(err)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(directory, "native-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Records    int    `json:"records"`
		Operations int    `json:"operations"`
		RawSHA     string `json:"raw_sha256"`
		GzipSHA    string `json:"gzip_sha256"`
	}
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != manifest.GzipSHA {
		t.Fatal("owning path gzip checksum differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != manifest.RawSHA {
		t.Fatal("owning path raw checksum differs")
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 4096), 1<<20)
	count, operations := 0, 0
	for scanner.Scan() {
		var row ownedPathNativeRow
		if err = json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		if got := ownedPathRun(row); got != row.Want {
			left, right := strings.Split(got, "|"), strings.Split(row.Want, "|")
			for i := range left {
				if i >= len(right) || left[i] != right[i] {
					op := "initial"
					if i > 0 {
						op = row.Ops[i-1]
					}
					t.Fatalf("Windows=%t script %d op %d %s\ngot  %s\nwant %s", row.Windows, row.ID, i, op, left[i], right[i])
				}
			}
			t.Fatalf("Windows=%t script %d trace differs", row.Windows, row.ID)
		}
		count++
		operations += len(row.Ops)
	}
	if scanner.Err() != nil {
		t.Fatal(scanner.Err())
	}
	if count != manifest.Records || operations != manifest.Operations {
		t.Fatal("owning path proof count differs")
	}
}
