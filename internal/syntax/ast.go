// Package syntax describes Ferrugo's bootstrap Rust syntax tree.
package syntax

// Span identifies a source location. Line and Column are one-based.
type Span struct {
	Offset, Line, Column int
	File                 string
}

type Program struct {
	// Compiler-loaded crate metadata; parser leaves these fields empty.
	CrateID   string
	Externs   map[string]*Program
	Functions []*Function
	Modules   []*Module
	Imports   []Import
	Structs   []*Struct
}

// Struct is a named Copy record. Only the supported Copy+Clone derive pair
// sets Copy; move-only and other struct shapes are outside this milestone.
type Struct struct {
	Name         string
	Public, Copy bool
	Fields       []StructField
	Span         Span
}

type StructField struct {
	Name, Type string
	Public     bool
	Span       Span
}

// FieldValue preserves constructor evaluation order, including shorthand fields.
type FieldValue struct {
	Name  string
	Value *Expr
	Span  Span
}

// Module declares an inline module (Program set) or an external source file.
type Module struct {
	Name             string
	Public, External bool
	Program          *Program
	Span             Span
}

// Import binds a simple namespace path, optionally with an explicit alias.
// An empty Alias means the last path segment; public reexports are parsed but
// may be rejected by the resolver until their semantics are implemented.
type Import struct {
	Path, Alias string
	Public      bool
	Span        Span
}
type Function struct {
	Public     bool
	Name       string
	Params     []Param
	ReturnType string
	Body       *Block
	Span       Span
}
type Param struct {
	Name, Type string
	Span       Span
}
type Block struct {
	Statements []Stmt
	Tail       *Expr
	Span       Span
}

// Stmt.Kind: let, assign, expr, return, if, while, break, continue, block.
// assign uses Target for a field place, otherwise Name; Expr is the RHS.
// Assignment Op preserves =, arithmetic, bitwise, or shift compound spelling.
// For legacy if/while, Expr is the condition and Body is the body. Else is optional.
// An expr statement with Type="()" has no semicolon and must evaluate to unit.
type Stmt struct {
	Kind, Name, Type, Op string
	Target               *Expr
	Mutable              bool
	Expr                 *Expr
	Body, Else           *Block
	Span                 Span
}

// Expr.Kind: int, bool, string, unit, name, unary, binary, call, macro, if, block, struct, field, cast.
// struct uses Text source type path, Target canonical type, and ordered Fields.
// field uses Left receiver and Text field name.
// cast uses Left operand, Text source target type, and Target resolved type.
// Text holds literal spelling (decoded for string), identifier or callee path.
// Qualified name/call paths retain their :: separators, including crate/self/super.
// Target optionally holds a resolver-selected canonical function; Text stays source spelling.
// Op holds unary/binary operator; Left/Right are operands; Args holds arguments.
// For if, Left is the condition and Body/Else are branches. For block, Body is its block.
type Expr struct {
	Kind, Text, Op string
	Target         string
	Left, Right    *Expr
	Args           []*Expr
	Fields         []FieldValue
	Body, Else     *Block
	Span           Span
}
