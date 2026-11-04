// SPDX-License-Identifier: MIT OR Apache-2.0
// Shared adapter for the scoped Rust std::fmt debug-builder contracts exercised
// by source ports. This supporting package grants no standard-library file credit.

// Package rustfmt implements fallible streaming Debug builders. Numeric flags
// and mutable formatter options belong to the source package's callback layer.
package rustfmt

import (
	"io"
	"strings"
	"unicode/utf8"
)

// Value formats an already-evaluated source value into the supplied writer.
// A builder invokes it only while its remembered formatting result is successful.
type Value func(io.Writer) error

type Field struct {
	Name  string
	Value Value
}
type Entry struct{ Key, Value Value }

// CharWriter preserves Rust fmt::Write's independently overridable write_char.
// Other writers receive the default UTF-8 write_str operation instead.
type CharWriter interface{ WriteChar(rune) error }

func WriteString(writer io.Writer, text string) error {
	count, err := io.WriteString(writer, text)
	if err == nil && count != len(text) {
		return io.ErrShortWrite
	}
	return err
}

func WriteChar(writer io.Writer, char rune) error {
	if !utf8.ValidRune(char) {
		panic("rustfmt: character must be a Unicode scalar")
	}
	if custom, ok := writer.(CharWriter); ok {
		return custom.WriteChar(char)
	}
	return WriteString(writer, string(char))
}

// IndentWriter is the pretty debug builder's PadAdapter. Each nesting level
// emits its own four-space writes, drops empty string writes, and forwards
// character operations separately while tracking newlines from both methods.
type IndentWriter struct {
	writer  io.Writer
	newline bool
}

func NewIndentWriter(writer io.Writer) *IndentWriter {
	return &IndentWriter{writer: writer, newline: true}
}
func (writer *IndentWriter) Write(text []byte) (int, error) { return writer.WriteString(string(text)) }
func (writer *IndentWriter) WriteString(text string) (int, error) {
	accepted := 0
	for text != "" {
		end := strings.IndexByte(text, '\n') + 1
		if end == 0 {
			end = len(text)
		}
		part := text[:end]
		if writer.newline {
			if err := WriteString(writer.writer, "    "); err != nil {
				return accepted, err
			}
		}
		writer.newline = strings.HasSuffix(part, "\n")
		if err := WriteString(writer.writer, part); err != nil {
			return accepted, err
		}
		accepted += len(part)
		text = text[end:]
	}
	return accepted, nil
}
func (writer *IndentWriter) WriteChar(char rune) error {
	if writer.newline {
		if err := WriteString(writer.writer, "    "); err != nil {
			return err
		}
	}
	writer.newline = char == '\n'
	return WriteChar(writer.writer, char)
}

type builder struct {
	writer io.Writer
	pretty bool
	count  int
	err    error
}

func (builder *builder) text(writer io.Writer, text string) {
	if builder.err == nil {
		builder.err = WriteString(writer, text)
	}
}
func (builder *builder) value(writer io.Writer, value Value) {
	if builder.err == nil {
		builder.err = value(writer)
	}
}

// StructBuilder remembers errors without stopping its caller's expression
// evaluation. Field's value argument is evaluated before the inert method call
// even after an earlier failure, just as with Rust's builder method chains.
type StructBuilder struct{ builder }

func BeginStruct(writer io.Writer, name string, pretty bool) *StructBuilder {
	b := &StructBuilder{builder: builder{writer: writer, pretty: pretty}}
	b.text(writer, name)
	return b
}
func (b *StructBuilder) Field(name string, value Value) *StructBuilder {
	if b.err != nil {
		return b
	}
	if b.pretty {
		if b.count == 0 {
			b.text(b.writer, " {\n")
		}
		padded := NewIndentWriter(b.writer)
		b.text(padded, name)
		b.text(padded, ": ")
		b.value(padded, value)
		b.text(padded, ",\n")
	} else {
		separator := ", "
		if b.count == 0 {
			separator = " { "
		}
		b.text(b.writer, separator)
		b.text(b.writer, name)
		b.text(b.writer, ": ")
		b.value(b.writer, value)
	}
	b.count++
	return b
}
func (b *StructBuilder) Finish() error {
	if b.count > 0 {
		ending := " }"
		if b.pretty {
			ending = "}"
		}
		b.text(b.writer, ending)
	}
	return b.err
}

// FinishNonExhaustive is DebugStruct::finish_non_exhaustive. The marker is a
// separate fallible write, and pretty output uses a fresh PadAdapter before
// writing the closing brace through the original formatter.
func (b *StructBuilder) FinishNonExhaustive() error {
	if b.count == 0 {
		b.text(b.writer, " { .. }")
	} else if b.pretty {
		b.text(NewIndentWriter(b.writer), "..\n")
		b.text(b.writer, "}")
	} else {
		b.text(b.writer, ", .. }")
	}
	return b.err
}
func WriteStruct(writer io.Writer, name string, pretty bool, fields []Field) error {
	b := BeginStruct(writer, name, pretty)
	for _, field := range fields {
		b.Field(field.Name, field.Value)
	}
	return b.Finish()
}

type TupleBuilder struct {
	builder
	name string
}

func BeginTuple(writer io.Writer, name string, pretty bool) *TupleBuilder {
	b := &TupleBuilder{builder: builder{writer: writer, pretty: pretty}, name: name}
	b.text(writer, name)
	return b
}
func (b *TupleBuilder) Field(value Value) *TupleBuilder {
	if b.err != nil {
		return b
	}
	if b.pretty {
		if b.count == 0 {
			b.text(b.writer, "(\n")
		}
		padded := NewIndentWriter(b.writer)
		b.value(padded, value)
		b.text(padded, ",\n")
	} else {
		separator := ", "
		if b.count == 0 {
			separator = "("
		}
		b.text(b.writer, separator)
		b.value(b.writer, value)
	}
	b.count++
	return b
}
func (b *TupleBuilder) Finish() error {
	if b.count > 0 {
		if !b.pretty && b.name == "" && b.count == 1 {
			b.text(b.writer, ",")
		}
		b.text(b.writer, ")")
	}
	return b.err
}
func WriteTuple(writer io.Writer, name string, pretty bool, fields []Value) error {
	b := BeginTuple(writer, name, pretty)
	for _, value := range fields {
		b.Field(value)
	}
	return b.Finish()
}

type ListBuilder struct{ builder }

func BeginList(writer io.Writer, pretty bool) *ListBuilder {
	b := &ListBuilder{builder: builder{writer: writer, pretty: pretty}}
	b.text(writer, "[")
	return b
}
func (b *ListBuilder) Entry(value Value) *ListBuilder {
	if b.err != nil {
		return b
	}
	if b.pretty {
		if b.count == 0 {
			b.text(b.writer, "\n")
		}
		padded := NewIndentWriter(b.writer)
		b.value(padded, value)
		b.text(padded, ",\n")
	} else {
		if b.count > 0 {
			b.text(b.writer, ", ")
		}
		b.value(b.writer, value)
	}
	b.count++
	return b
}
func (b *ListBuilder) Finish() error { b.text(b.writer, "]"); return b.err }
func WriteList(writer io.Writer, pretty bool, values []Value) error {
	b := BeginList(writer, pretty)
	for _, value := range values {
		b.Entry(value)
	}
	return b.Finish()
}

type MapBuilder struct{ builder }

func BeginMap(writer io.Writer, pretty bool) *MapBuilder {
	b := &MapBuilder{builder: builder{writer: writer, pretty: pretty}}
	b.text(writer, "{")
	return b
}
func (b *MapBuilder) Entry(key, value Value) *MapBuilder {
	if b.err != nil {
		return b
	}
	if b.pretty {
		if b.count == 0 {
			b.text(b.writer, "\n")
		}
		padded := NewIndentWriter(b.writer)
		b.value(padded, key)
		b.text(padded, ": ")
		b.value(padded, value)
		b.text(padded, ",\n")
	} else {
		if b.count > 0 {
			b.text(b.writer, ", ")
		}
		b.value(b.writer, key)
		b.text(b.writer, ": ")
		b.value(b.writer, value)
	}
	b.count++
	return b
}
func (b *MapBuilder) Finish() error { b.text(b.writer, "}"); return b.err }
func WriteMap(writer io.Writer, pretty bool, entries []Entry) error {
	b := BeginMap(writer, pretty)
	for _, entry := range entries {
		b.Entry(entry.Key, entry.Value)
	}
	return b.Finish()
}
