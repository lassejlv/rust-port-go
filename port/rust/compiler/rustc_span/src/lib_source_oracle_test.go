package rustcspan

import (
	"bufio"
	"compress/gzip"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"

	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func libSourceHex(text string) string { return hex.EncodeToString([]byte(text)) }
func libSourceList[T any](values []T, debug func(T, *rustfmt.Formatter) error) string {
	return rustfmt.String(func(f *rustfmt.Formatter) error {
		b := rustfmt.BeginList(f.Writer, false)
		for _, value := range values {
			b.Entry(f.Value(false, func(child *rustfmt.Formatter) error { return debug(value, child) }))
		}
		return b.Finish()
	})
}
func libSourceObserved(src string, kind SourceFileHashAlgorithm) string {
	sf, err := NewSourceFile(AnonSourceCode(src), src, kind, Some(kind))
	if err != nil {
		panic(err)
	}
	defer sf.CloseWithContext(rustfmt.Context{})
	normalized, _ := sf.Src.Get()
	text := fmt.Sprintf("%d;%d;%s;%s;%s;%s;%s;%s;%d;%d", sf.UnnormalizedSourceLen, sf.NormalizedSourceLen, libSourceHex(normalized.String()), sf.SrcHash.String(), rustfmt.String(sf.StableId.WriteDebugFormatter), libSourceList(sf.LinesWithContext(rustfmt.Context{}), func(v RelativeBytePos, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) }), libSourceList(sf.MultibyteChars, func(v MultiByteChar, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) }), libSourceList(sf.NormalizedPos, func(v NormalizedPos, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) }), cap(sf.MultibyteChars), cap(sf.NormalizedPos))
	for i := uint(0); i < sf.CountLines()+2; i++ {
		if line, ok := sf.GetLine(i).Get(); ok {
			text += fmt.Sprintf(";L%d:%t:%s", i, line.IsOwned(), libSourceHex(line.String()))
		} else {
			text += fmt.Sprintf(";L%d:-", i)
		}
	}
	source := normalized.String()
	for pos := 0; pos <= len(source); pos++ {
		if pos == len(source) || utf8.RuneStart(source[pos]) {
			line, col, display := sf.LookupFilePosWithColDisplay(BytePos(pos))
			text += fmt.Sprintf(";P%d:%d:%d:%d", pos, line, col, display)
		}
	}
	clone := sf.Clone()
	defer clone.CloseWithContext(rustfmt.Context{})
	text += fmt.Sprintf(";C%d:%d:%d", cap(clone.MultibyteChars), cap(clone.NormalizedPos), normalized.StrongCount())
	encoder := NewSpanMemEncoder(nil)
	sf.Encode(encoder)
	raw := encoder.Finish()
	text += ";E" + hex.EncodeToString(raw)
	frame := append(raw, []byte(rustcserialize.MagicEndBytes)...)
	decoder, decodeError := NewSpanMemDecoder(nil, frame, 0)
	if decodeError != nil {
		panic(decodeError)
	}
	decoded := DecodeSourceFile(decoder)
	defer decoded.CloseWithContext(rustfmt.Context{})
	wasFrozen := decoded.Lines.IsFrozen()
	text += fmt.Sprintf(";D%t:%t:%d:%d:%t:%s", !decoded.Src.IsSome(), wasFrozen, cap(decoded.MultibyteChars), cap(decoded.NormalizedPos), decoded.ExternalSrc.IsFrozen(), libSourceList(decoded.LinesWithContext(rustfmt.Context{}), func(v RelativeBytePos, f *rustfmt.Formatter) error { return v.WriteDebugFormatter(f) }))
	return text
}
func TestLibOriginalSourceFileMetadata(t *testing.T) {
	file, err := os.Open("testdata/lib-source.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	raw, err := io.ReadAll(io.LimitReader(gz, 32*1024*1024+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 32*1024*1024 {
		t.Fatal("source fixture exceeds bound")
	}
	scan := bufio.NewScanner(strings.NewReader(string(raw)))
	scan.Buffer(make([]byte, 4096), 1024*1024)
	count := 0
	for scan.Scan() {
		parts := strings.Split(scan.Text(), "\t")
		if len(parts) != 4 {
			t.Fatalf("invalid source record %d", count)
		}
		kind, ok := ParseSourceFileHashAlgorithm(parts[1])
		if !ok {
			t.Fatal(parts[1])
		}
		input, err := hex.DecodeString(parts[2])
		if err != nil {
			t.Fatal(err)
		}
		expected, err := hex.DecodeString(parts[3])
		if err != nil {
			t.Fatal(err)
		}
		got := libSourceObserved(string(input), kind)
		if got != string(expected) {
			t.Fatalf("record %s/%s input %q\nGo: %s\nRust: %s", parts[0], parts[1], input, got, expected)
		}
		count++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 8232 {
		t.Fatalf("source records %d", count)
	}
}
