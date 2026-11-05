// SPDX-License-Identifier: MIT OR Apache-2.0
package ruststr

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type ownedStringNativeRow struct {
	SDK, Profile string
	Request      struct {
		Factory, Text string
		Capacity      uint
		Ops           []string
	}
	Want string
}

func ownedStringHex(s string) string { return hex.EncodeToString([]byte(s)) }
func ownedStringUnhex(s string) string {
	b, e := hex.DecodeString(s)
	if e != nil {
		panic(e)
	}
	return string(b)
}
func ownedStringRun(q ownedStringNativeRow) string {
	context := rustfmt.Context{}
	var initial *OwnedString
	if q.Request.Factory == "from" {
		initial = OwnedStringFromGo(q.Request.Text, context)
	} else {
		initial = NewOwnedStringWithCapacity(q.Request.Capacity, context)
		initial.Borrow().PushString(q.Request.Text)
	}
	slots := []*StringCell{initial.TakeCellWithContext(context), nil, nil}
	defer func() {
		for _, cell := range slots {
			if cell != nil {
				cell.CloseWithContext(context)
			}
		}
	}()
	snapshot := func() string {
		parts := []string{}
		for _, cell := range slots {
			if cell == nil {
				parts = append(parts, "-")
			} else {
				parts = append(parts, fmt.Sprintf("%d:%d:%s", cell.Len(), cell.Capacity(), ownedStringHex(cell.AsStr())))
			}
		}
		return strings.Join(parts, ";")
	}
	output := []string{snapshot()}
	for _, op := range q.Request.Ops {
		fields := strings.Split(op, ":")
		slot, _ := strconv.Atoi(fields[1])
		identity := "-"
		hooks := []string{}
		ops := &rustfmt.PanicOperations{Locations: map[string]rustfmt.SourceLocation{}}
		// Real selected SDK origin comes from this record; the operation still
		// computes its own payload/kind/failure state at the actual raise site.
		for _, line := range strings.Split(q.Want, "\n") {
			parts := strings.SplitN(line, ",", 4)
			if len(parts) != 4 || parts[2] == "" {
				continue
			}
			for _, hook := range strings.Split(parts[2], "|") {
				p := strings.Split(hook, ":")
				if len(p) == 5 {
					line, _ := strconv.ParseUint(p[3], 10, 32)
					column, _ := strconv.ParseUint(p[4], 10, 32)
					ops.Locations[OwnedStringCapacityPanicSite] = rustfmt.SourceLocation{File: ownedStringUnhex(p[2]), Line: uint32(line), Column: uint32(column)}
				}
			}
		}
		ops.Hook = func(info rustfmt.SourcePanicInfo, c rustfmt.Context) {
			file := ""
			var line, col uint32
			if info.Location != nil {
				file = info.Location.File
				line = info.Location.Line
				col = info.Location.Column
			}
			hooks = append(hooks, fmt.Sprintf("%s:%s:%s:%d:%d", info.Kind, ownedStringHex(fmt.Sprint(info.Payload)), ownedStringHex(file), line, col))
		}
		c := rustfmt.WithPanicOperations(context, ops)
		caught := func() (panicked bool) {
			defer func() { panicked = recover() != nil }()
			switch fields[0] {
			case "push", "char", "reserve", "exact":
				cell := slots[slot]
				before := cell.BufferIdentity()
				var needed uint
				if fields[0] == "push" {
					text := ownedStringUnhex(fields[2])
					needed = uint(len(text))
					stable := cell.Capacity()-cell.Len() >= needed
					cell.PushStringWithContext(text, c)
					if stable {
						identity = strconv.FormatBool(before == cell.BufferIdentity())
					}
				}
				if fields[0] == "char" {
					value, _ := strconv.ParseInt(fields[2], 10, 32)
					needed = uint(len(string(rune(value))))
					stable := cell.Capacity()-cell.Len() >= needed
					cell.PushCharWithContext(rune(value), c)
					if stable {
						identity = strconv.FormatBool(before == cell.BufferIdentity())
					}
				}
				if fields[0] == "reserve" || fields[0] == "exact" {
					n, _ := strconv.ParseUint(fields[2], 10, 64)
					needed = uint(n)
					stable := cell.Capacity()-cell.Len() >= needed
					if fields[0] == "reserve" {
						cell.ReserveWithContext(needed, c)
					} else {
						cell.ReserveExactWithContext(needed, c)
					}
					if stable {
						identity = strconv.FormatBool(before == cell.BufferIdentity())
					}
				}
			case "clone":
				src, _ := strconv.Atoi(fields[2])
				value := slots[src].CloneWithContext(c)
				next := value.TakeCellWithContext(c)
				identity = strconv.FormatBool(next.BufferIdentity() == slots[src].BufferIdentity())
				if slots[slot] != nil {
					slots[slot].CloseWithContext(c)
				}
				slots[slot] = next
			case "clonefrom":
				src, _ := strconv.Atoi(fields[2])
				before := slots[slot].BufferIdentity()
				stable := slots[slot].Capacity() >= slots[src].Len()
				slots[slot].CloneFromWithContext(slots[src], c)
				if stable {
					identity = strconv.FormatBool(before == slots[slot].BufferIdentity())
				}
			case "move":
				src, _ := strconv.Atoi(fields[2])
				before := slots[src].BufferIdentity()
				next := slots[src].MoveWithContext(c)
				slots[src] = nil
				if slots[slot] != nil {
					slots[slot].CloseWithContext(c)
				}
				slots[slot] = next
				identity = strconv.FormatBool(before == slots[slot].BufferIdentity())
			case "drop":
				if slots[slot] != nil {
					slots[slot].CloseWithContext(c)
					slots[slot] = nil
				}
			default:
				panic("invalid std String request")
			}
			return false
		}()
		output = append(output, fmt.Sprintf("%t,%s,%s,%s", caught, identity, strings.Join(hooks, "|"), snapshot()))
	}
	return strings.Join(output, "\n")
}
func TestOwnedStringOriginalSDKs(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	body, err := os.ReadFile(filepath.Join(root, "tools/port-cargo-time-span/string-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Records, Operations int
		RawSHA              string `json:"raw_sha256"`
		FixtureSHA          string `json:"fixture_sha256"`
		Files               map[string]string
	}
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatal(err)
	}
	if m.Records != 1344 || m.Operations != 45696 || len(m.Files) == 0 {
		t.Fatal("invalid String proof profile")
	}
	for path, digest := range m.Files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != digest {
			t.Fatal("String source/proof binding differs", path, err)
		}
	}
	compressed, err := os.ReadFile("testdata/owned-string.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(compressed)) != m.FixtureSHA {
		t.Fatal("String fixture hash")
	}
	g, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(g, 16<<20+1))
	if err != nil || len(raw) > 16<<20 {
		t.Fatal("String raw bound", err)
	}
	if err := g.Close(); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != m.RawSHA {
		t.Fatal("String raw hash")
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 64<<10), 1<<20)
	count := 0
	for scan.Scan() {
		count++
		var row ownedStringNativeRow
		if err := json.Unmarshal(scan.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		if got := ownedStringRun(row); got != row.Want {
			t.Fatalf("case%d SDK%s %s\nGo=%s\nRust=%s", count, row.SDK, row.Profile, got, row.Want)
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != m.Records {
		t.Fatal(count)
	}
}
