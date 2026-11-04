package stablehash

import (
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
)

func TestOriginalArithmeticProfiles(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("retained arithmetic input fixture is x86_64")
	}
	for _, debug := range []bool{false, true} {
		for _, checked := range []bool{false, true} {
			func() {
				file, e := os.Open(fmt.Sprintf("testdata/arith_debug%t_overflow%t.jsonl.gz", debug, checked))
				if e != nil {
					t.Fatal(e)
				}
				defer file.Close()
				reader, e := gzip.NewReader(file)
				if e != nil {
					t.Fatal(e)
				}
				defer reader.Close()
				scan := bufio.NewScanner(reader)
				count := 0
				for scan.Scan() {
					count++
					var row struct {
						Request struct {
							Op   string
							A, B uint64
						}
						Result struct {
							Value *uint64
							Panic string
						}
					}
					if e := json.Unmarshal(scan.Bytes(), &row); e != nil {
						t.Fatal(e)
					}
					var value uint
					var p any
					func() {
						defer func() { p = recover() }()
						if row.Request.Op == "add" {
							value = debugStrictAdd(uint(row.Request.A), uint(row.Request.B), ArithmeticProfile{debug, checked}, DropContext{})
						} else {
							value = debugStrictSub(uint(row.Request.A), uint(row.Request.B), ArithmeticProfile{debug, checked}, DropContext{})
						}
					}()
					if row.Result.Value != nil {
						if p != nil || uint64(value) != *row.Result.Value {
							t.Fatalf("%+v debug%v checked%v:value%d panic%v", row.Request, debug, checked, value, p)
						}
					} else if p == nil || fmt.Sprint(p) != row.Result.Panic {
						t.Fatalf("%+v panic%v want%s", row.Request, p, row.Result.Panic)
					}
				}
				if e := scan.Err(); e != nil {
					t.Fatal(e)
				}
				if count != 882 {
					t.Fatal(count)
				}
			}()
		}
	}
}
