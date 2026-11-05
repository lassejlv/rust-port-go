package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestLibOriginalFlagRadixFormatters(t *testing.T) {
	values := make(map[string]func(*rustfmt.Formatter) error)
	for bits := 0; bits < 256; bits++ {
		value := RemapPathScopeComponents(bits)
		values[fmt.Sprintf("radix/%d/0", bits)] = value.WriteBinaryFormatter
		values[fmt.Sprintf("radix/%d/1", bits)] = value.WriteOctalFormatter
		values[fmt.Sprintf("radix/%d/2", bits)] = value.WriteLowerHexFormatter
		values[fmt.Sprintf("radix/%d/3", bits)] = value.WriteUpperHexFormatter
	}
	libReplayOriginalFormatters(t, "testdata/lib-flags-format.tsv.gz", values, 7528)
}
func TestLibOriginalFlagIteratorsAndOperations(t *testing.T) {
	f, err := os.Open("testdata/lib-flags-iter.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	raw, err := io.ReadAll(io.LimitReader(g, 16<<20+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 16<<20 {
		t.Fatal("flags bound")
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	count := 0
	for scan.Scan() {
		count++
		p := strings.Split(scan.Text(), "\t")
		parse := func(s string) uint8 {
			v, err := strconv.ParseUint(s, 10, 8)
			if err != nil {
				t.Fatal(err)
			}
			return uint8(v)
		}
		got := ""
		switch p[0] {
		case "iter":
			if len(p) != 6 {
				t.Fatal("iter shape")
			}
			value := RemapPathScopeComponents(parse(p[1]))
			names := value.IterNames()
			rows := []string{}
			for i := 0; i < 9; i++ {
				v, ok := names.Next()
				if ok {
					rows = append(rows, fmt.Sprintf("%s:%d:%d", v.Name, v.Value, names.Remaining()))
				} else {
					rows = append(rows, fmt.Sprintf("none:%d", names.Remaining()))
				}
			}
			iter := value.Iter()
			values := []string{}
			for i := 0; i < 9; i++ {
				v, ok := iter.Next()
				if ok {
					values = append(values, strconv.Itoa(int(v)))
				} else {
					values = append(values, "none")
				}
			}
			got = strings.Join([]string{strings.Join(rows, ","), strings.Join(values, ","), strconv.FormatBool(value.ContainsUnknownBits()), strconv.FormatBool(RemapPathScopeComponentsFromBits(uint8(value)).IsSome())}, "\t")
			if got != strings.Join(p[2:], "\t") {
				t.Fatalf("%v Go=%s", p, got)
			}
		case "defined":
			iter := RemapPathScopeDefinedNames()
			values := []string{}
			for {
				flag, ok := iter.Next()
				if !ok {
					break
				}
				values = append(values, fmt.Sprintf("%s:%d", flag.Name, flag.Value))
			}
			got = strings.Join(values, ",")
			if got != p[1] {
				t.Fatal(got, p[1])
			}
		case "name":
			name, err := hex.DecodeString(p[1])
			if err != nil {
				t.Fatal(err)
			}
			got = "none"
			if value, ok := RemapPathScopeComponentsFromName(string(name)).Get(); ok {
				got = strconv.Itoa(int(value))
			}
			if got != p[2] {
				t.Fatal(p, got)
			}
		case "ops":
			if len(p) != 12 {
				t.Fatal("ops shape")
			}
			a, b := RemapPathScopeComponents(parse(p[1])), RemapPathScopeComponents(parse(p[2]))
			truncate, clear := a, a
			truncate.Truncate()
			clear.Clear()
			got = fmt.Sprintf("%d\t%d\t%d\t%d\t%d\t%t\t%t\t%d\t%d", a.Union(b), a.Intersection(b), a.SymmetricDifference(b), a.Difference(b), a.Complement(), a.Intersects(b), a.Contains(b), truncate, clear)
			if got != strings.Join(p[3:], "\t") {
				t.Fatal(p, got)
			}
		default:
			t.Fatal(p[0])
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 65807 {
		t.Fatal(count)
	}
}
