package rustcspan

import (
	"bufio"
	"compress/gzip"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"io"
	"os"
	"strconv"
	"strings"
	"testing"
)

func libOwnedHashObserved(operation string, kind SourceFileHashAlgorithm, mode string) []string {
	events := []string{}
	ctx := rustfmt.Context{}
	panicked := false
	var result SourceFileHash
	var failure error
	func() {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		if operation == "as_ref" {
			type input struct{ calls int }
			owned := NewSpanOwned(input{}, func(value *input, ctx rustfmt.Context) {
				spanDropFields(ctx, func(ctx rustfmt.Context) {
					events = append(events, "input_drop:"+strconv.Itoa(value.calls)+":"+strconv.FormatBool(ctx.Panicking()))
				}, func(ctx rustfmt.Context) { events = append(events, "token:"+strconv.FormatBool(ctx.Panicking())) })
			})
			result = NewSourceFileHashInMemoryOwned(kind, owned, func(value *input, ctx rustfmt.Context) []byte {
				events = append(events, "as_ref")
				value.calls++
				if mode == "true" {
					ctx.Panic("as_ref")
				}
				return []byte{0xff, 0, 0x80}
			}, ctx)
		} else {
			number, err := strconv.Atoi(mode)
			if err != nil {
				panic(err)
			}
			type reader struct{ calls int }
			owned := NewSpanOwned(reader{}, func(value *reader, ctx rustfmt.Context) {
				spanDropFields(ctx, func(ctx rustfmt.Context) {
					events = append(events, "reader_drop:"+strconv.Itoa(value.calls)+":"+strconv.FormatBool(ctx.Panicking()))
					if number == 3 {
						ctx.Panic("drop")
					}
				}, func(ctx rustfmt.Context) { events = append(events, "token:"+strconv.FormatBool(ctx.Panicking())) })
			})
			result, failure = NewSourceFileHashFromOwnedRead(kind, owned, func(value *reader, buf []byte, ctx rustfmt.Context) (uint, error) {
				events = append(events, "read:"+strconv.Itoa(value.calls)+":"+strconv.Itoa(len(buf)))
				value.calls++
				if number == 5 {
					return uint(len(buf) + 1), nil
				}
				if value.calls == 1 {
					copy(buf, []byte{0xff, 0, 0x80})
					return 3, nil
				}
				switch number {
				case 0:
					return 0, nil
				case 1, 3:
					return 0, NewSourceIOError(struct{}{}, SourceIOErrorOps[struct{}]{Display: func(*struct{}, *rustfmt.Formatter) error { panic("unexpected display") }, Debug: func(*struct{}, *rustfmt.Formatter) error { panic("unexpected debug") }, Drop: func(_ *struct{}, ctx rustfmt.Context) {
						events = append(events, "error_drop:"+strconv.FormatBool(ctx.Panicking()))
					}})
				case 2:
					ctx.Panic("read")
				}
				panic("unexpected read mode")
			}, ctx)
		}
	}()
	if panicked {
		events = append(events, "panic")
	} else if failure != nil {
		events = append(events, "err")
		CloseSourceIOError(failure, ctx)
	} else {
		events = append(events, "ok:"+result.String())
	}
	return events
}
func TestLibOriginalOwnedHashInputs(t *testing.T) {
	file, err := os.Open("testdata/lib-owned-hash.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	raw, err := io.ReadAll(io.LimitReader(gz, 128*1024+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 128*1024 {
		t.Fatal("owned hash fixture exceeds bound")
	}
	scan := bufio.NewScanner(strings.NewReader(string(raw)))
	count := 0
	for scan.Scan() {
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 4 {
			t.Fatal("malformed owned hash record")
		}
		kind, ok := ParseSourceFileHashAlgorithm(p[1])
		if !ok {
			t.Fatal(p[1])
		}
		got := strings.Join(libOwnedHashObserved(p[0], kind, p[2]), ",")
		if got != p[3] {
			t.Fatalf("%v\nGo: %s\nRust: %s", p[:3], got, p[3])
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 28 {
		t.Fatal(count)
	}
}
