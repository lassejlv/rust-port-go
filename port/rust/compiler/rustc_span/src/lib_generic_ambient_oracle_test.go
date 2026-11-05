package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLibOriginalGenericAmbient(t *testing.T) {
	f, err := os.Open("testdata/lib-generic-ambient.tsv.gz")
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
		t.Fatal("ambient fixture bound")
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	count, aborts := 0, 0
	for scan.Scan() {
		count++
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 6 {
			t.Fatal("ambient shape")
		}
		ctx := rustfmt.Context{Unwinding: p[3] == "unwinding", ProcessingPanic: p[3] == "processing"}
		got := ""
		if p[4] == "true" {
			aborts++
			cmd := exec.Command(os.Args[0], "-test.run=^TestLibGenericAbortChild$")
			cmd.Env = append(os.Environ(), "FERRUGO_LIB_GENERIC_ABORT="+p[0], "FERRUGO_LIB_GENERIC_CALL="+p[1], "FERRUGO_LIB_GENERIC_DROP="+p[2], "FERRUGO_LIB_GENERIC_AMBIENT="+p[3])
			output, err := cmd.CombinedOutput()
			status, ok := err.(*exec.ExitError)
			if !ok || status.ExitCode() != 134 {
				t.Fatalf("%v child %v: %s", p[:5], err, output)
			}
			events := []string{}
			for _, line := range strings.Split(string(output), "\n") {
				if strings.HasPrefix(line, "EVENT:") {
					events = append(events, strings.TrimPrefix(line, "EVENT:"))
				}
			}
			got = strings.Join(events, ",")
		} else {
			got = libSessionObservedWithContext(p[0], p[1] == "true", p[2] == "true", false, ctx, p[3] != "processing")
		}
		if got != p[5] {
			t.Fatalf("row %d %v\nGo: %s\nRust: %s", count, p[:5], got, p[5])
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 72 || aborts != 42 {
		t.Fatalf("counts %d %d", count, aborts)
	}
}
