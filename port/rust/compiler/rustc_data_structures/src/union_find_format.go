// SPDX-License-Identifier: MIT OR Apache-2.0
// Debug trait bridge for the complete pinned union_find.rs implementation at
// 2d20b42b883dcfc014f3c2c56081eeb6091c2179. Struct and padding dispatch follows
// that pin's library/core/src/fmt/builders.rs. No Rust layout ABI is assumed.
package datastructures

import (
	"strings"

	rustcindex "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_index/src"
)

// FormatDebug is the primary derived Debug implementation. The key witness is
// invoked at the real entry.parent cell, with the complete mutable formatter,
// source panic context, and independently fallible string/character writers.
// As with the source shared borrow, formatting must not mutate this forest.
func (sets *UnionFind[Key]) FormatDebug(f *rustcindex.IndexDebugFormatter, formatKey func(*Key, *rustcindex.IndexDebugFormatter) error) error {
	b := newUnionFindDebugStruct(f, "UnionFind")
	b.field("table", func(f *rustcindex.IndexDebugFormatter) error {
		ops := rustcindex.ValueOps[unionFindEntry[Key]]{FormatDebug: func(entry *unionFindEntry[Key], f *rustcindex.IndexDebugFormatter) error {
			entryBuilder := newUnionFindDebugStruct(f, "UnionFindEntry")
			entryBuilder.field("parent", func(f *rustcindex.IndexDebugFormatter) error { return formatKey(&entry.parent, f) })
			entryBuilder.field("rank", func(f *rustcindex.IndexDebugFormatter) error {
				return rustcindex.FormatDebugUint(uint64(entry.rank), 32, f)
			})
			return entryBuilder.finish()
		}}
		idx := rustcindex.NewIdx(sets.index.FromIndex, sets.index.ToIndex)
		return rustcindex.IndexSliceFromRaw(sets.table, idx, ops).FormatDebug(f)
	})
	return b.finish()
}

// Debug is the existing compact string convenience. Use FormatDebug for Debug
// implementations whose borrowing, flags, writer errors, or panic state matter.
func (sets *UnionFind[Key]) Debug(formatKey func(Key) string) string {
	var text strings.Builder
	f := rustcindex.IndexDebugFormatter{Options: rustcindex.DefaultIndexFormatOptions(), Write: func(s string) error { _, _ = text.WriteString(s); return nil }}
	err := sets.FormatDebug(&f, func(key *Key, f *rustcindex.IndexDebugFormatter) error { return f.WriteString(formatKey(*key)) })
	if err != nil {
		panic("UnionFind: Debug implementation returned an error for a string sink")
	}
	return text.String()
}

type unionFindDebugStruct struct {
	f      *rustcindex.IndexDebugFormatter
	err    error
	fields bool
}

func newUnionFindDebugStruct(f *rustcindex.IndexDebugFormatter, name string) unionFindDebugStruct {
	return unionFindDebugStruct{f: f, err: f.WriteString(name)}
}
func (b *unionFindDebugStruct) field(name string, format func(*rustcindex.IndexDebugFormatter) error) {
	defer func() { b.fields = true }()
	if b.err != nil {
		return
	}
	if b.f.Options.Alternate {
		if !b.fields {
			b.err = b.f.WriteString(" {\n")
			if b.err != nil {
				return
			}
		}
		newline := true
		child := unionFindPadFormatter(b.f, &newline)
		if b.err = child.WriteString(name); b.err != nil {
			return
		}
		if b.err = child.WriteString(": "); b.err != nil {
			return
		}
		if b.err = format(&child); b.err != nil {
			return
		}
		b.err = child.WriteString(",\n")
	} else {
		prefix := " { "
		if b.fields {
			prefix = ", "
		}
		if b.err = b.f.WriteString(prefix); b.err != nil {
			return
		}
		if b.err = b.f.WriteString(name); b.err != nil {
			return
		}
		if b.err = b.f.WriteString(": "); b.err != nil {
			return
		}
		b.err = format(b.f)
	}
}
func (b *unionFindDebugStruct) finish() error {
	if b.err != nil {
		return b.err
	}
	if !b.fields {
		return nil
	}
	if b.f.Options.Alternate {
		return b.f.WriteString("}")
	}
	return b.f.WriteString(" }")
}

func unionFindPadFormatter(f *rustcindex.IndexDebugFormatter, newline *bool) rustcindex.IndexDebugFormatter {
	child := *f
	child.Write = func(text string) error {
		for len(text) != 0 {
			n := strings.IndexByte(text, '\n')
			if n < 0 {
				n = len(text)
			} else {
				n++
			}
			part := text[:n]
			if *newline {
				if err := f.WriteString("    "); err != nil {
					return err
				}
			}
			*newline = part[len(part)-1] == '\n'
			if err := f.WriteString(part); err != nil {
				return err
			}
			text = text[n:]
		}
		return nil
	}
	child.WriteChar = func(c rune) error {
		if *newline {
			if err := f.WriteString("    "); err != nil {
				return err
			}
		}
		*newline = c == '\n'
		return f.WriteRune(c)
	}
	return child
}
