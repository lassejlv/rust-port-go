// Package syntax describes Ferrugo's bootstrap Rust syntax tree.
package syntax

// Span identifies a source location. Line and Column are one-based.
type Span struct{ Offset, Line, Column int }

type Program struct{ Functions []*Function }
type Function struct {
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
// For assign, Name is the target and Op is =, +=, -=, *=, /= or %=.
// For legacy if/while, Expr is the condition and Body is the body. Else is optional.
// An expr statement with Type="()" has no semicolon and must evaluate to unit.
type Stmt struct {
	Kind, Name, Type, Op string
	Mutable              bool
	Expr                 *Expr
	Body, Else           *Block
	Span                 Span
}

// Expr.Kind: int, bool, string, unit, name, unary, binary, call, macro, if, block.
// Text holds literal spelling (decoded for string), identifier or callee name.
// Op holds unary/binary operator; Left/Right are operands; Args holds arguments.
// For if, Left is the condition and Body/Else are branches. For block, Body is its block.
type Expr struct {
	Kind, Text, Op string
	Left, Right    *Expr
	Args           []*Expr
	Body, Else     *Block
	Span           Span
}
