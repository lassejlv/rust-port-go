package blake3

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"testing"
)

// Expectations are produced only by the actual locked Rust crate. Each case
// checks empty/prefix/full digests, repeated nonconsuming finalize, resumed
// updates, borrowed-input lifetime, value-state independence and one-shot hash.
func TestNativeUnkeyedHashOperations(t *testing.T) {
	file, err := os.Open("testdata/native.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	z, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	scanner := bufio.NewScanner(io.LimitReader(z, 16<<20))
	count := 0
	for scanner.Scan() {
		var length, seed, stride int
		var prefix, full, once, empty string
		if n, err := fmt.Sscan(scanner.Text(), &length, &seed, &stride, &prefix, &full, &once, &empty); err != nil || n != 7 {
			t.Fatalf("bad native record: %d, %v", n, err)
		}
		if length < 0 || length > 2<<20 || stride < 0 || seed < 0 || seed > 255 {
			t.Fatal("native request exceeds bounds")
		}
		data := make([]byte, length)
		for i := range data {
			data[i] = byte(i%251) ^ byte(seed)
		}
		h := NewHasher()
		check := func(expected string, actual [32]byte) {
			if got := hex.EncodeToString(actual[:]); got != expected {
				t.Fatalf("case %d (%d/%d/%d): got %s, expected %s", count, length, seed, stride, got, expected)
			}
		}
		check(empty, h.Finalize())
		position := 0
		var prefixState Hasher
		for step, end := range []int{length / 2, length} {
			for position < end {
				n := end - position
				if stride > 0 {
					n = min(n, stride)
				}
				borrow := append([]byte(nil), data[position:position+n]...)
				h.Update(borrow)
				for i := range borrow {
					borrow[i] ^= 0xff
				}
				position += n
			}
			h.Update(nil)
			want := full
			if step == 0 {
				want = prefix
				prefixState = *h
			}
			check(want, h.Finalize())
			check(want, h.Finalize())
		}
		check(prefix, prefixState.Finalize())
		check(once, Hash(data))
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 6040 {
		t.Fatalf("native request count %d", count)
	}
}
