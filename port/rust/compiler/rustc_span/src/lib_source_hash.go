// SPDX-License-Identifier: MIT OR Apache-2.0
// SourceFileHash blocks of pinned compiler/rustc_span/src/lib.rs.
package rustcspan

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"

	blake3 "github.com/lassejlv/rust-port-go/port/dependencies/blake3/v1_8_2/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type SourceFileHashAlgorithm uint8

const (
	SourceFileHashMd5 SourceFileHashAlgorithm = iota
	SourceFileHashSha1
	SourceFileHashSha256
	SourceFileHashBlake3
)

func (value SourceFileHashAlgorithm) hashLen() int {
	switch value {
	case SourceFileHashMd5:
		return 16
	case SourceFileHashSha1:
		return 20
	case SourceFileHashSha256, SourceFileHashBlake3:
		return 32
	}
	panic("rustcspan: invalid SourceFileHashAlgorithm")
}
func (value SourceFileHashAlgorithm) String() string {
	switch value {
	case SourceFileHashMd5:
		return "md5"
	case SourceFileHashSha1:
		return "sha1"
	case SourceFileHashSha256:
		return "sha256"
	case SourceFileHashBlake3:
		return "blake3"
	}
	panic("rustcspan: invalid SourceFileHashAlgorithm")
}
func ParseSourceFileHashAlgorithm(text string) (SourceFileHashAlgorithm, bool) {
	switch text {
	case "md5":
		return SourceFileHashMd5, true
	case "sha1":
		return SourceFileHashSha1, true
	case "sha256":
		return SourceFileHashSha256, true
	case "blake3":
		return SourceFileHashBlake3, true
	}
	return 0, false
}
func (value SourceFileHashAlgorithm) WriteDisplayFormatter(f *rustfmt.Formatter) error {
	return f.WriteString(value.String())
}

type SourceFileHash struct {
	Kind  SourceFileHashAlgorithm
	value [32]byte
}

// SourceFileHashFromRaw retains the entire array, including inactive bytes,
// which derived equality/hash intentionally observe.
func SourceFileHashFromRaw(kind SourceFileHashAlgorithm, value [32]byte) SourceFileHash {
	kind.hashLen()
	return SourceFileHash{Kind: kind, value: value}
}
func NewSourceFileHashInMemory(kind SourceFileHashAlgorithm, data []byte) SourceFileHash {
	out := SourceFileHash{Kind: kind}
	switch kind {
	case SourceFileHashMd5:
		v := md5.Sum(data)
		copy(out.value[:], v[:])
	case SourceFileHashSha1:
		v := sha1.Sum(data)
		copy(out.value[:], v[:])
	case SourceFileHashSha256:
		out.value = sha256.Sum256(data)
	case SourceFileHashBlake3:
		out.value = blake3.Hash(data)
	default:
		kind.hashLen()
	}
	return out
}

// NewSourceFileHashInMemoryOwned consumes the source impl AsRef<[u8]> value.
// The callback borrows its real cell and its returned slice for the digest.
func NewSourceFileHashInMemoryOwned[S any](kind SourceFileHashAlgorithm, source *SpanOwned[S], asRef func(*S, rustfmt.Context) []byte, context rustfmt.Context) SourceFileHash {
	owned := source.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, owned.DropWithContext)
	kind.hashLen()
	data := asRef(owned.Borrow(), context)
	return NewSourceFileHashInMemory(kind, data)
}
func (value SourceFileHash) Matches(src string) bool {
	return value.MatchesWithContext(src, rustfmt.Context{})
}
func (value SourceFileHash) MatchesWithContext(src string, context rustfmt.Context) bool {
	requireSpanUTF8(src, context)
	return NewSourceFileHashInMemory(value.Kind, []byte(src)) == value
}

// HashBytes borrows the active prefix; it may not be mutated or retained beyond
// the source borrow. A pointer receiver avoids a misleading view of a copy.
func (value *SourceFileHash) HashBytes() []byte              { return value.value[:value.Kind.hashLen()] }
func (value SourceFileHash) RawValue() [32]byte              { return value.value }
func (value SourceFileHash) Clone() SourceFileHash           { return value }
func (value SourceFileHash) Equal(other SourceFileHash) bool { return value == other }

// SourceHashRead is the Rust Read::read result contract: an error carries no
// count. The borrowed buffer has length 16KiB for every source read call.
type SourceHashRead interface{ ReadSourceHash([]byte) (uint, error) }

type ownedSourceHashReader[S any] struct {
	source  *SpanOwned[S]
	read    func(*S, []byte, rustfmt.Context) (uint, error)
	context rustfmt.Context
}

func (reader *ownedSourceHashReader[S]) ReadSourceHash(buf []byte) (uint, error) {
	return reader.read(reader.source.Borrow(), buf, reader.context)
}

// Reader errors transfer intact on a successful return. The original leaves
// the pending digest return-place error undestroyed if the consumed reader's
// destructor panics before that return completes; do not invent error cleanup.
func NewSourceFileHashFromOwnedRead[S any](kind SourceFileHashAlgorithm, source *SpanOwned[S], read func(*S, []byte, rustfmt.Context) (uint, error), context rustfmt.Context) (result SourceFileHash, failure error) {
	owned := source.MoveWithContext(context)
	defer spanCloseOwnedCallback(context, owned.DropWithContext)
	return newSourceFileHashRead(kind, &ownedSourceHashReader[S]{source: owned, read: read, context: context}, context)
}

func NewSourceFileHashFromRead(kind SourceFileHashAlgorithm, src SourceHashRead) (SourceFileHash, error) {
	return newSourceFileHashRead(kind, src, rustfmt.Context{})
}
func newSourceFileHashRead(kind SourceFileHashAlgorithm, src SourceHashRead, context rustfmt.Context) (SourceFileHash, error) {
	result := SourceFileHash{Kind: kind}
	buf := make([]byte, 16*1024)
	var h hash.Hash
	var b *blake3.Hasher
	switch kind {
	case SourceFileHashMd5:
		h = md5.New()
	case SourceFileHashSha1:
		h = sha1.New()
	case SourceFileHashSha256:
		h = sha256.New()
	case SourceFileHashBlake3:
		b = blake3.NewHasher()
	default:
		kind.hashLen()
	}
	for {
		n, err := src.ReadSourceHash(buf)
		if err != nil {
			return SourceFileHash{}, err
		}
		if n == 0 {
			break
		}
		if n > uint(len(buf)) {
			spanFormattedPanicAt(context, spanLibLocation(1860, 41), func(rustfmt.Context) any {
				return fmt.Sprintf("range end index %d out of range for slice of length %d", n, len(buf))
			})
		}
		if b != nil {
			b.Update(buf[:n])
		} else {
			_, _ = h.Write(buf[:n])
		}
	}
	if b != nil {
		result.value = b.Finalize()
	} else {
		copy(result.value[:], h.Sum(nil))
	}
	return result, nil
}

// The io.Reader adapter retains a pending Go (n>0,err) until the next source
// read, since Rust Read cannot return bytes and an error simultaneously.
type sourceHashGoReader struct {
	reader  io.Reader
	pending error
}

func (r *sourceHashGoReader) ReadSourceHash(buf []byte) (uint, error) {
	if r.pending != nil {
		err := r.pending
		r.pending = nil
		if err == io.EOF {
			return 0, nil
		}
		return 0, err
	}
	n, err := r.reader.Read(buf)
	if n < 0 || n > len(buf) {
		panic("rustcspan: invalid Go Reader count")
	}
	if n > 0 {
		r.pending = err
		return uint(n), nil
	}
	if err == io.EOF {
		return 0, nil
	}
	return 0, err
}
func NewSourceFileHashFromReader(kind SourceFileHashAlgorithm, reader io.Reader) (SourceFileHash, error) {
	return NewSourceFileHashFromRead(kind, &sourceHashGoReader{reader: reader})
}
