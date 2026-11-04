package compiler

import (
	"fmt"
	"github.com/lassejlv/rust-port-go/internal/syntax"
)

func (g *generator) zeroValue(typ string) string { return g.temp(typ, "("+cType(typ)+"){0}") }

func (g *generator) checkIndex(index string, length, line int) {
	if length == 0 {
		g.line("fr_panic(\"array index out of bounds at line %d\");", line)
		return
	}
	g.line("if ((%s) >= ((uintptr_t)%d)) fr_panic(\"array index out of bounds at line %d\");", index, length, line)
}

func isArrayPlace(expression *syntax.Expr) bool {
	if expression == nil {
		return false
	}
	switch expression.Kind {
	case "name":
		return true
	case "field", "index":
		return isArrayPlace(expression.Left)
	}
	return false
}

func (g *generator) indexValue(expression *syntax.Expr) string {
	var receiver string
	if isArrayPlace(expression.Left) {
		receiver = g.place(expression.Left)
	} else {
		receiver = g.expr(expression.Left)
	}
	if receiver == "" || g.checked.diverges[expression.Left] {
		return ""
	}
	index := g.expr(expression.Right)
	if g.checked.diverges[expression.Right] {
		return ""
	}
	array := g.checked.arrays[g.checked.types[expression.Left]]
	g.checkIndex(index, array.Length, expression.Span.Line)
	if array.Length == 0 {
		return "(*((" + cType(array.Element) + " *)0))"
	}
	if g.checked.zeroSized[array.Element] {
		return g.zeroValue(array.Element)
	}
	return g.temp(g.checked.types[expression], "("+receiver+").fr_data["+index+"]")
}

func (g *generator) arrayValue(expression *syntax.Expr) string {
	typ := g.checked.types[expression]
	array := g.checked.arrays[typ]
	if expression.Kind == "repeat" {
		value := g.expr(expression.Left)
		if g.checked.diverges[expression.Left] {
			return ""
		}
		target := g.zeroValue(typ)
		if g.checked.zeroSized[typ] {
			return target
		}
		g.counter++
		index := fmt.Sprintf("fr_repeat%d", g.counter)
		g.line("for (size_t %s = 0; %s < %d; ++%s) {", index, index, array.Length, index)
		g.indent++
		g.line("%s.fr_data[%s] = %s;", target, index, value)
		g.indent--
		g.line("}")
		return target
	}
	target := g.temp(typ, "("+cType(typ)+"){0}")
	for i, element := range expression.Args {
		value := g.expr(element)
		if g.checked.diverges[element] {
			return ""
		}
		if !g.checked.zeroSized[typ] {
			g.line("%s.fr_data[%d] = %s;", target, i, value)
		}
	}
	return target
}
