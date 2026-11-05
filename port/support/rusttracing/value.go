// SPDX-License-Identifier: MIT OR Apache-2.0
// Scoped tracing 0.1.44 / tracing-core 0.1.36 used-contract adapter. This is
// not a complete port of either dependency and earns no dependency-file credit.
package rusttracing

import "github.com/lassejlv/rust-port-go/port/support/rustfmt"

type Context struct {
	Scope *Scope
	Panic rustfmt.Context
}
type Level uint8

const (
	Off Level = iota
	Error
	Warn
	Info
	Debug
	Trace
)

type Kind uint8

const (
	SpanKind Kind = iota
	EventKind
)

type Metadata struct {
	Name, Target, ModulePath, File string
	Line                           uint32
	Level                          Level
	Kind                           Kind
	Fields                         []string
}

// Field identity is its callsite and position. Names and metadata are borrowed
// static source data; callers must not mutate them after registration.
type Field struct {
	Name     string
	Index    uint
	Callsite *Callsite
}
type FieldValue struct {
	Name  string
	Field *Field
	Value Value
}
type Value interface {
	Record(*Field, Visitor, Context)
}
type DebugValue struct {
	Format func(*rustfmt.Formatter) error
}

func (v DebugValue) WriteDebugFormatter(f *rustfmt.Formatter) error { return v.Format(f) }
func (v DebugValue) Record(field *Field, visitor Visitor, ctx Context) {
	visitor.RecordDebug(field, v, ctx)
}
func DebugField(format func(*rustfmt.Formatter) error) Value { return DebugValue{format} }

// DisplayField is tracing::field::display. Its Debug implementation delegates
// to Display on the same mutable formatter, without pre-rendering a string.
func DisplayField(format func(*rustfmt.Formatter) error) Value { return DebugValue{format} }

// MessageField models fmt::Arguments. Format arguments install their own
// format-spec options instead of inheriting the visitor's outer Debug flags.
func MessageField(format func(*rustfmt.Formatter) error) Value {
	return DebugValue{func(f *rustfmt.Formatter) error {
		child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}
		return format(child)
	}}
}

// Visitor's one required method and independently overridable typed methods
// mirror tracing_core::field::Visit. Fallbacks preserve the original scalar
// type's Debug implementation and the typed method's copied argument value.
type Visitor interface {
	RecordDebug(*Field, DebugValue, Context)
}
type boolVisitor interface{ RecordBool(*Field, bool, Context) }
type intVisitor interface{ RecordI64(*Field, int64, Context) }
type uintVisitor interface{ RecordU64(*Field, uint64, Context) }
type floatVisitor interface {
	RecordF64(*Field, float64, Context)
}
type strVisitor interface{ RecordStr(*Field, string, Context) }
type bytesVisitor interface{ RecordBytes(*Field, []byte, Context) }
type boolValue bool

func BoolField(v bool) Value { return boolValue(v) }
func (v boolValue) Record(f *Field, visitor Visitor, ctx Context) {
	value := bool(v)
	if typed, ok := visitor.(boolVisitor); ok {
		typed.RecordBool(f, value, ctx)
		return
	}
	visitor.RecordDebug(f, DebugValue{func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugBool(value, f) }}, ctx)
}

type intValue int64

func I64Field(v int64) Value { return intValue(v) }
func (v intValue) Record(f *Field, visitor Visitor, ctx Context) {
	value := int64(v)
	if typed, ok := visitor.(intVisitor); ok {
		typed.RecordI64(f, value, ctx)
		return
	}
	visitor.RecordDebug(f, DebugValue{func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugInt(value, 64, f) }}, ctx)
}

type uintValue uint64

func U64Field(v uint64) Value { return uintValue(v) }
func (v uintValue) Record(f *Field, visitor Visitor, ctx Context) {
	value := uint64(v)
	if typed, ok := visitor.(uintVisitor); ok {
		typed.RecordU64(f, value, ctx)
		return
	}
	visitor.RecordDebug(f, DebugValue{func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(value, 64, f) }}, ctx)
}

type strValue string

func StrField(v string) Value { return strValue(v) }
func (v strValue) Record(f *Field, visitor Visitor, ctx Context) {
	value := string(v)
	if typed, ok := visitor.(strVisitor); ok {
		typed.RecordStr(f, value, ctx)
		return
	}
	visitor.RecordDebug(f, DebugValue{func(f *rustfmt.Formatter) error { return rustfmt.FormatDebugStringUnicode18(value, f) }}, ctx)
}

type ParentKind uint8

const (
	ContextualParent ParentKind = iota
	RootParent
	ExplicitParent
)

type Parent struct {
	Kind ParentKind
	ID   ID
}
type ID struct{ value uint64 }

func NewID(value uint64) ID { return NewIDWithContext(value, rustfmt.Context{}) }
func NewIDWithContext(value uint64, ctx rustfmt.Context) ID {
	if value == 0 {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Payload: "span IDs must be > 0", Location: ctx.SourcePanicLocation(PanicSiteZeroID), CanUnwind: true})
	}
	return ID{value}
}

// PanicSiteZeroID selects the original tracing-core span::Id constructor site.
// Missing caller profile metadata remains honestly unlocated.
const PanicSiteZeroID = "tracing_core::span::Id::from_u64.nonzero"

func (id ID) AsU64() uint64 { return id.value }

type CurrentKind uint8

const (
	UnknownCurrent CurrentKind = iota
	NoCurrent
	KnownCurrent
)

type Current struct {
	Kind     CurrentKind
	ID       ID
	Metadata *Metadata
}

type EventRecord struct {
	Metadata *Metadata
	Parent   Parent
	Values   []FieldValue
	site     *Callsite
}
type Attributes struct {
	Metadata *Metadata
	Parent   Parent
	Values   []FieldValue
	site     *Callsite
}

func recordFields(site *Callsite, values []FieldValue, visitor Visitor, ctx Context) {
	for i, value := range values {
		if value.Value == nil {
			continue
		}
		field := value.Field
		if field == nil {
			if i < len(site.fields) && site.fields[i].Name == value.Name {
				field = &site.fields[i]
			} else {
				field = site.Field(value.Name)
			}
		}
		if field != nil && field.Callsite == site {
			value.Value.Record(field, visitor, ctx)
		}
	}
}
func (e *EventRecord) Record(visitor Visitor, ctx Context) {
	recordFields(e.site, e.Values, visitor, ctx)
}
func (a *Attributes) Record(visitor Visitor, ctx Context) {
	recordFields(a.site, a.Values, visitor, ctx)
}
