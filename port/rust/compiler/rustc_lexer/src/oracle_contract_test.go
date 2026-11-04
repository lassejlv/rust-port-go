// SPDX-License-Identifier: MIT OR Apache-2.0
package rustclexer

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	validation "github.com/lassejlv/rust-port-go/tools/port-lexer-debug/validation"
)

func contractDirectory() string {
	_, source, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../../tools/port-lexer-debug"))
}
func contractLiterals() []LiteralKind {
	var values []LiteralKind
	for _, base := range []Base{Binary, Octal, Decimal, Hexadecimal} {
		for _, b := range []bool{false, true} {
			values = append(values, LiteralKind{Tag: Int, Base: base, EmptyInt: b}, LiteralKind{Tag: Float, Base: base, EmptyExponent: b})
		}
	}
	for _, b := range []bool{false, true} {
		for _, tag := range []LiteralTag{Char, Byte, Str, ByteStr, CStr} {
			values = append(values, LiteralKind{Tag: tag, Terminated: b})
		}
	}
	for _, h := range []struct {
		present bool
		value   uint8
	}{{false, 0}, {true, 0}, {true, 1}, {true, 26}, {true, 255}} {
		for _, tag := range []LiteralTag{RawStr, RawByteStr, RawCStr} {
			values = append(values, LiteralKind{Tag: tag, HasNHashes: h.present, NHashes: h.value})
		}
	}
	return values
}
func contractErrors() []RawStrError {
	var values []RawStrError
	for _, c := range []rune{0, '\n', '\'', '\\', 'a', '🦀', '\u0301', '\x7f', '\U0010ffff'} {
		values = append(values, RawStrError{Tag: InvalidStarter, BadChar: c})
	}
	for _, n := range []uint32{0, 26, ^uint32(0)} {
		for _, h := range []struct {
			present bool
			value   uint32
		}{{false, 0}, {true, 0}, {true, 26}, {true, ^uint32(0)}} {
			values = append(values, RawStrError{Tag: NoTerminator, Expected: n, Found: ^uint32(0) - n, HasPossibleTerminatorOffset: h.present, PossibleTerminatorOffset: h.value})
		}
		values = append(values, RawStrError{Tag: TooManyDelimiters, Found: n})
	}
	return values
}
func contractGuards() []GuardedStr {
	var values []GuardedStr
	for _, n := range []uint32{0, 26, ^uint32(0)} {
		for _, b := range []bool{false, true} {
			for _, length := range []uint32{0, 26, ^uint32(0)} {
				values = append(values, GuardedStr{n, b, length})
			}
		}
	}
	return values
}
func contractDebugValues() []validation.Value {
	var values []validation.Value
	add := func(value validation.Value) { values = append(values, value) }
	for _, b := range []Base{Binary, Octal, Decimal, Hexadecimal} {
		b := b
		add(&b)
	}
	for _, d := range []DocStyle{Outer, Inner} {
		d := d
		add(&d)
	}
	for _, d := range []DocStyle{NoDocStyle, Outer, Inner} {
		add(&TokenKind{Tag: LineComment, DocStyle: d})
		for _, b := range []bool{false, true} {
			add(&TokenKind{Tag: BlockComment, DocStyle: d, Terminated: b})
		}
	}
	for _, a := range []bool{false, true} {
		for _, b := range []bool{false, true} {
			add(&TokenKind{Tag: Frontmatter, HasInvalidPrecedingWhitespace: a, InvalidInfostring: b})
		}
		add(&TokenKind{Tag: Lifetime, StartsWithNumber: a})
	}
	for _, tag := range []TokenTag{Whitespace, Ident, InvalidIdent, RawIdent, ForcedKeywordIdent, UnknownPrefix, UnknownPrefixLifetime, RawLifetime, GuardedStrPrefix, Semi, Comma, Dot, OpenParen, CloseParen, OpenBrace, CloseBrace, OpenBracket, CloseBracket, At, Pound, Tilde, Question, Colon, Dollar, Eq, Bang, Lt, Gt, Minus, And, Or, Plus, Star, Slash, Caret, Percent, Unknown, Eof} {
		add(&TokenKind{Tag: tag})
		add(&Token{Kind: TokenKind{Tag: tag}, Len: ^uint32(0)})
	}
	for _, literal := range contractLiterals() {
		literal := literal
		add(&literal)
		add(&TokenKind{Tag: Literal, Literal: literal, SuffixStart: 26})
		add(&Token{Kind: TokenKind{Tag: Literal, Literal: literal, SuffixStart: ^uint32(0)}, Len: 26})
	}
	for _, e := range contractErrors() {
		e := e
		add(&e)
	}
	for _, g := range contractGuards() {
		g := g
		add(&g)
	}
	return values
}
func TestOriginalLexerFullDebug(t *testing.T) {
	validation.Replay(t, contractDirectory(), contractDebugValues())
}

func TestOriginalStructuredOrdering(t *testing.T) {
	directory := contractDirectory()
	validation.CheckEvidence(t, directory, "order")
	compressed, err := os.ReadFile(filepath.Join(directory, "order.tsv.gz"))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile(filepath.Join(directory, "order-manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var info struct {
		Records    int    `json:"records"`
		Raw        string `json:"raw_sha256"`
		Compressed string `json:"fixture_sha256"`
	}
	if err = json.Unmarshal(metadata, &info); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(compressed)
	if hex.EncodeToString(sum[:]) != info.Compressed {
		t.Fatal("ordering fixture digest changed")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	raw := new(bytes.Buffer)
	if _, err = raw.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	sum = sha256.Sum256(raw.Bytes())
	if hex.EncodeToString(sum[:]) != info.Raw {
		t.Fatal("ordering observations digest changed")
	}
	literals, errors, guards := contractLiterals(), contractErrors(), contractGuards()
	scanner := bufio.NewScanner(raw)
	count := 0
	for scanner.Scan() {
		count++
		var name string
		var a, b, want int
		var some bool
		if _, err = fmt.Fscanf(strings.NewReader(scanner.Text()), "%s\t%d\t%d\t%d\t%t", &name, &a, &b, &want, &some); err != nil {
			t.Fatal(err)
		}
		var got, partial int
		var present bool
		switch name {
		case "literal":
			got = literals[a].Compare(literals[b])
			partial, present = literals[a].PartialCompare(literals[b])
		case "error":
			got = errors[a].Compare(errors[b])
			partial, present = errors[a].PartialCompare(errors[b])
		case "guard":
			got = guards[a].Compare(guards[b])
			partial, present = guards[a].PartialCompare(guards[b])
		default:
			t.Fatal("invalid source ordering type")
		}
		if got != want || partial != want || !present || !some {
			t.Fatalf("%s[%d,%d]: Go%d/%d Rust%d", name, a, b, got, partial, want)
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != info.Records {
		t.Fatal("truncated ordering fixture")
	}
}
