package rustcserialize

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func fileOracleDigest(data []byte) string { return fmt.Sprintf("sha256=%x", sha256.Sum256(data)) }
func fileOraclePanic(call func()) (panicked bool) {
	defer func() { panicked = recover() != nil }()
	call()
	return
}
func TestOriginalFileEncoder(t *testing.T) {
	f, err := os.Open("testdata/file.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	raw, err := io.ReadAll(io.LimitReader(g, 1<<20+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 1<<20 {
		t.Fatal("file fixture too large")
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	scan.Buffer(make([]byte, 4096), 1<<20)
	directory := t.TempDir()
	count := 0
	for scan.Scan() {
		count++
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 7 {
			t.Fatalf("row %d shape", count)
		}
		profile := p[0]
		p = p[1:]
		if p[0] == "full" && runtime.GOOS != "linux" {
			continue
		}
		path := filepath.Join(directory, "file")
		if p[0] == "full" {
			path = "/dev/full"
		}
		fail, err := strconv.Atoi(p[2])
		if err != nil {
			t.Fatal(err)
		}
		events := []string{}
		calls := 0
		e, err := NewFileEncoderWithFlushStrategy(path, func(data []byte) {
			n := calls
			calls++
			events = append(events, fmt.Sprintf("flush:%d:%d:%s", n, len(data), fileOracleDigest(data)))
			if n == fail {
				panic("strategy")
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		e.profile = Profile{DebugAssertions: profile[1] == '1', OverflowChecks: profile[3] == '1'}
		for _, op := range strings.Split(p[1], ",") {
			part := strings.Split(op, ":")
			number := func(i int) uint64 {
				v, err := strconv.ParseUint(part[i], 10, 64)
				if err != nil {
					t.Fatal(err)
				}
				return v
			}
			panicked := fileOraclePanic(func() {
				switch part[0] {
				case "raw":
					e.EmitRawBytes(bytes.Repeat([]byte{byte(number(2))}, int(number(1))))
				case "u64":
					e.EmitU64(number(1))
				case "u16":
					e.EmitU16(uint16(number(1)))
				case "with":
					n, written, fill := uint(number(1)), uint(number(2)), byte(number(3))
					e.WriteWith(n, func(window []byte) uint {
						events = append(events, fmt.Sprintf("window:%d:%s", n, fileOracleDigest(window)))
						for i := range window {
							window[i] = fill
						}
						return written
					})
				case "flush":
					e.Flush()
				case "finish":
					position, err := e.Finish()
					if err == nil {
						events = append(events, fmt.Sprintf("finish:ok:%d", position))
					} else {
						events = append(events, "finish:error")
					}
				default:
					t.Fatal(part[0])
				}
			})
			events = append(events, fmt.Sprintf("step:%t:%d", panicked, e.Position()))
		}
		dropped := fileOraclePanic(e.Close)
		events = append(events, fmt.Sprintf("drop:%t", dropped))
		if got := strings.Join(events, ","); got != p[3] {
			t.Fatalf("row %d (%s %s fail=%s)\nGo: %s\nRust: %s", count, p[0], p[1], p[2], got, p[3])
		}
		if p[0] == "file" {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strconv.Itoa(len(data)) != p[4] || fileOracleDigest(data) != p[5] {
				t.Fatalf("row %d file content differs", count)
			}
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 332 {
		t.Fatalf("records=%d", count)
	}
}
