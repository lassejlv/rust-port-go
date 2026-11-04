// SPDX-License-Identifier: MIT OR Apache-2.0
package rustclexer

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	validation "github.com/lassejlv/rust-port-go/tools/port-lexer-debug/validation"
)

var contractInputs = []string{"", "a", "a\x00🦀\n", "r#\"a\"#tail", "##\"abc\"#suffix", "---\nx: 1\n---\nfn main() {}", "\n---\n---", " /* hi */ 1u8 + 'a'", "\"unterminated", "\x00", "λ🦀\r\n", "r###\"abc\"#x\"##", " abc!tail"}

func contractCursorState(c *Cursor) string {
	return fmt.Sprintf("%s:%d:%d:%d:%d:%d:%t", hex.EncodeToString([]byte(c.AsStr())), c.Prev(), c.First(), c.Second(), c.Third(), c.PosWithinToken(), c.IsEOF())
}
func contractPanicText(f func() string) (out string) {
	defer func() {
		if recover() != nil {
			out = "panic"
		}
	}()
	return f()
}
func contractRune(c rune, ok bool) string {
	if !ok {
		return "None"
	}
	return "Some(" + rustfmt.String(func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugChar(c, f) }) + ")"
}
func contractToken(token Token, ok bool) string {
	if !ok {
		return "None"
	}
	return "Some(" + token.RustDebug() + ")"
}
func contractProfileRecord(parts []string) string {
	config := CursorConfig{OverflowChecks: parts[0] == "true", DebugAssertions: parts[1] == "true"}
	number := func(i int) uint64 {
		n, e := strconv.ParseUint(parts[i], 10, 64)
		if e != nil {
			panic(e)
		}
		return n
	}
	front := FrontmatterNo
	if len(parts) > 4 && parts[4] == "true" {
		front = FrontmatterYes
	}
	switch parts[2] {
	case "cursor":
		c := NewCursorWithConfig(contractInputs[number(3)], front, config)
		events := []string{contractCursorState(c)}
		for action := 0; action < 9; action++ {
			text := contractPanicText(func() string {
				switch action {
				case 0, 4:
					return contractRune(c.Bump())
				case 1:
					c.BumpBytes(0)
				case 2:
					c.EatUntil('!')
				case 3:
					c.ResetPosWithinToken()
				case 5:
					c.EatWhile(func(x rune) bool { return x != '\n' })
				case 6:
					c.BumpBytes(1)
				default:
					return c.AdvanceToken().RustDebug()
				}
				return "unit"
			})
			events = append(events, text, contractCursorState(c))
		}
		return hex.EncodeToString([]byte(strings.Join(events, "|")))
	case "guard":
		c := NewCursorWithConfig(contractInputs[number(3)], FrontmatterNo, config)
		for i := uint64(0); i < number(4); i++ {
			c.Bump()
		}
		value := contractPanicText(func() string {
			g, ok := c.GuardedDoubleQuotedString()
			if !ok {
				return "None"
			}
			return "Some(" + g.RustDebug() + ")"
		})
		return hex.EncodeToString([]byte(value + "|" + contractCursorState(c)))
	case "raw":
		return hex.EncodeToString([]byte(contractPanicText(func() string {
			err := ValidateRawStrWithConfig(contractInputs[number(3)], uint32(number(4)), config)
			if err != nil {
				return "Err(" + err.RustDebug() + ")"
			}
			return "Ok(())"
		})))
	case "arith":
		c := NewCursorWithConfig("", FrontmatterNo, config)
		a, b, op := uint32(number(3)), uint32(number(4)), number(5)
		return contractPanicText(func() string {
			var v uint32
			switch op {
			case 0:
				v = c.addU32(a, b)
			case 1:
				v = c.subU32(a, b)
			case 2:
				v = c.addU32(c.subU32(c.subU32(a, b), b), 1)
			case 3:
				v = c.addU32(a, 1)
			default:
				panic("invalid native arithmetic")
			}
			return fmt.Sprint(v)
		})
	case "cast":
		n := ^uint(0)
		return fmt.Sprint(uint32(n))
	case "until", "bytes":
		c := NewCursorWithConfig(contractInputs[number(3)], FrontmatterNo, config)
		value := contractPanicText(func() string {
			if parts[2] == "until" {
				c.EatUntil(byte(number(4)))
			} else {
				c.BumpBytes(int(number(4)))
			}
			return "unit"
		})
		return hex.EncodeToString([]byte(value + "|" + contractCursorState(c)))
	case "iterator":
		it := NewTokenizerWithConfig(contractInputs[number(3)], front, config)
		events := []string{"(0, None)"}
		script := number(5)
		push := func(token Token, ok bool) { events = append(events, contractToken(token, ok)) }
		switch script {
		case 0:
			for i := 0; i < 20; i++ {
				push(it.Next())
				low, high, present := it.SizeHint()
				if low != 0 || high != 0 || present {
					panic("invalid size_hint")
				}
				events = append(events, "(0, None)")
			}
		case 1:
			push(it.Nth(0))
			push(it.Next())
			push(it.Nth(1))
			push(it.Next())
			push(it.Nth(100))
			for i := 0; i < 3; i++ {
				push(it.Next())
			}
		case 2:
			push(it.Nth(^uint(0)))
			push(it.Next())
		case 3:
			push(it.Next())
			events = append(events, fmt.Sprint(it.Count()))
		case 4:
			push(it.Next())
			push(it.Last())
		case 5:
		default:
			panic("invalid native iterator script")
		}
		return hex.EncodeToString([]byte(strings.Join(events, "|")))
	default:
		panic("invalid native profile case")
	}
}
func TestOriginalCursorAndLazyProfiles(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("native source profile records64-bit usize")
	}
	directory := contractDirectory()
	validation.CheckEvidence(t, directory, "profiles")
	compressed, e := os.ReadFile(filepath.Join(directory, "profiles.tsv.gz"))
	if e != nil {
		t.Fatal(e)
	}
	meta, e := os.ReadFile(filepath.Join(directory, "profiles-manifest.json"))
	if e != nil {
		t.Fatal(e)
	}
	var info struct {
		Records    int    `json:"records"`
		Raw        string `json:"raw_sha256"`
		Compressed string `json:"fixture_sha256"`
	}
	if e = json.Unmarshal(meta, &info); e != nil {
		t.Fatal(e)
	}
	sum := sha256.Sum256(compressed)
	if hex.EncodeToString(sum[:]) != info.Compressed {
		t.Fatal("profile fixture changed")
	}
	reader, e := gzip.NewReader(bytes.NewReader(compressed))
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	raw := new(bytes.Buffer)
	if _, e = raw.ReadFrom(reader); e != nil {
		t.Fatal(e)
	}
	sum = sha256.Sum256(raw.Bytes())
	if hex.EncodeToString(sum[:]) != info.Raw {
		t.Fatal("profile observations changed")
	}
	lines := strings.Split(strings.TrimSuffix(raw.String(), "\n"), "\n")
	for i, line := range lines {
		parts := strings.Split(line, "\t")
		got := contractProfileRecord(parts)
		if got != parts[len(parts)-1] {
			t.Fatalf("profile line%d %s\nGo:%s\nRust:%s", i+1, strings.Join(parts[:len(parts)-1], "/"), got, parts[len(parts)-1])
		}
	}
	if len(lines) != info.Records {
		t.Fatal("truncated native profile records")
	}
	t.Logf("compared%d independent checked/debug cursor/iterator/arithmetic records", len(lines))
}
