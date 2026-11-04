package compiler

import (
	"fmt"
	"math/big"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

func (g *generator) forStatement(statement *syntax.Stmt) {
	item := g.checked.forTypes[statement]
	iterable := statement.Expr
	if iterable.Kind == "range" {
		start := g.expr(iterable.Left)
		if g.checked.diverges[iterable.Left] {
			return
		}
		end := g.expr(iterable.Right)
		if g.checked.diverges[iterable.Right] {
			return
		}
		cursor := g.temp(item, start)
		step := g.arithmetic("+", item, cursor, integerC(big.NewInt(1), item))
		if iterable.Op == "..=" {
			finished := g.temp("bool", "false")
			g.line("for (; !%s && %s <= %s; %s = (%s == %s), %s = %s ? %s : %s) {", finished, cursor, end, finished, cursor, end, cursor, finished, cursor, step)
		} else {
			g.line("for (; %s < %s; %s = %s) {", cursor, end, cursor, step)
		}
		g.indent++
		if statement.Name != "_" {
			g.line("%s %s = %s;", cType(item), g.checked.decls[statement], cursor)
		}
		g.block(statement.Body, false, "()")
		g.indent--
		g.line("}")
		return
	}
	value := g.expr(iterable)
	if g.checked.diverges[iterable] {
		return
	}
	array := g.checked.arrays[g.checked.types[iterable]]
	if array.Length == 0 {
		return
	}
	cursor := g.temp("usize", "((uintptr_t)0)")
	g.line("for (; %s < ((uintptr_t)%d); %s = fr_add_usize(%s, (uintptr_t)1)) {", cursor, array.Length, cursor, cursor)
	g.indent++
	if statement.Name != "_" {
		element := fmt.Sprintf("(%s).fr_data[%s]", value, cursor)
		if g.checked.zeroSized[item] {
			element = "(" + cType(item) + "){0}"
		}
		g.line("%s %s = %s;", cType(item), g.checked.decls[statement], element)
	}
	g.block(statement.Body, false, "()")
	g.indent--
	g.line("}")
}
