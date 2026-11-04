package compiler

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

// EmitC validates a bootstrap Rust binary crate and emits a C11 translation unit.
// Generated programs need libc, but do not need Rust or Go installed.
func EmitC(source string) (string, error) {
	c, err := analyze(source)
	if err != nil {
		return "", err
	}
	return emitChecked(c), nil
}

func emitChecked(c *checked) string {
	g := &generator{checked: c}
	g.WriteString(runtimeC)
	g.structDefinitions()
	for _, f := range c.program.Functions {
		g.line("%s;", g.signature(f))
	}
	for _, f := range c.program.Functions {
		g.line("%s", g.signature(f))
		g.block(f.Body, true, f.ReturnType)
	}
	g.line("int main(void) { %s(); return 0; }", c.fnNames[c.funcs["main"]])
	return g.String()
}

type generator struct {
	strings.Builder
	checked         *checked
	indent, counter int
}

func (g *generator) line(format string, args ...any) {
	g.WriteString(strings.Repeat("    ", g.indent))
	fmt.Fprintf(&g.Builder, format, args...)
	g.WriteByte('\n')
}
func cType(t string) string {
	if strings.HasPrefix(t, "struct:") {
		return "fr_struct_" + hex.EncodeToString([]byte(strings.TrimPrefix(t, "struct:")))
	}
	switch t {
	case "i32":
		return "int32_t"
	case "i64":
		return "int64_t"
	case "bool":
		return "bool"
	case "&str":
		return "fr_str"
	case "()":
		return "void"
	}
	panic("unchecked type: " + t)
}
func cField(name string) string { return "fr_field_" + name }
func (g *generator) structDefinitions() {
	for _, key := range g.checked.structOrder {
		name := cType(key)
		g.line("typedef struct %s %s;", name, name)
	}
	for _, key := range g.checked.structOrder {
		structure := g.checked.structs[key]
		g.line("struct %s {", cType(key))
		g.indent++
		if len(structure.Fields) == 0 {
			g.line("unsigned char fr_empty;")
		}
		for _, field := range structure.Fields {
			g.line("%s %s;", cType(field.Type), cField(field.Name))
		}
		g.indent--
		g.line("};")
	}
}
func (g *generator) place(e *syntax.Expr) string {
	if e.Kind == "name" {
		return g.checked.names[e]
	}
	if e.Kind == "field" {
		return "(" + g.place(e.Left) + ")." + cField(e.Text)
	}
	panic("unchecked assignment place: " + e.Kind)
}
func (g *generator) signature(f *syntax.Function) string {
	var params []string
	for i, p := range f.Params {
		params = append(params, cType(p.Type)+" "+g.checked.params[f][i])
	}
	if len(params) == 0 {
		params = append(params, "void")
	}
	return cType(f.ReturnType) + " " + g.checked.fnNames[f] + "(" + strings.Join(params, ", ") + ")"
}
func (g *generator) temp(t, value string) string {
	g.counter++
	name := fmt.Sprintf("fr_t%d", g.counter)
	g.line("%s %s = %s;", cType(t), name, value)
	return name
}
func (g *generator) block(b *syntax.Block, function bool, result string) {
	g.line("{")
	g.indent++
	for i := range b.Statements {
		g.statement(&b.Statements[i])
	}
	if b.Tail != nil {
		value := g.expr(b.Tail)
		if function && result != "()" && !g.checked.diverges[b.Tail] {
			g.line("return %s;", value)
		} else if value != "" {
			g.line("(void)(%s);", value)
		}
	}
	if function && result == "()" {
		g.line("return;")
	}
	g.indent--
	g.line("}")
}
func (g *generator) resultTarget(e *syntax.Expr) string {
	if g.checked.types[e] == "()" || g.checked.diverges[e] {
		return ""
	}
	g.counter++
	name := fmt.Sprintf("fr_t%d", g.counter)
	g.line("%s %s;", cType(g.checked.types[e]), name)
	return name
}
func (g *generator) valueBlock(b *syntax.Block, target string) {
	g.line("{")
	g.indent++
	for i := range b.Statements {
		g.statement(&b.Statements[i])
	}
	if b.Tail != nil {
		value := g.expr(b.Tail)
		if !g.checked.diverges[b.Tail] && value != "" {
			if target != "" {
				g.line("%s = %s;", target, value)
			} else {
				g.line("(void)(%s);", value)
			}
		}
	}
	g.indent--
	g.line("}")
}
func (g *generator) statement(s *syntax.Stmt) {
	switch s.Kind {
	case "let":
		value := g.expr(s.Expr)
		if value == "" {
			g.line("%s %s;", cType(g.checked.types[s.Expr]), g.checked.decls[s])
		} else {
			g.line("%s %s = %s;", cType(g.checked.types[s.Expr]), g.checked.decls[s], value)
		}
	case "assign":
		value := g.expr(s.Expr)
		name := g.checked.decls[s]
		if s.Target != nil {
			name = g.place(s.Target)
		}
		if g.checked.diverges[s.Expr] {
			return
		}
		if s.Op != "=" {
			value = g.arithmetic(strings.TrimSuffix(s.Op, "="), g.checked.types[s.Expr], name, value)
		}
		g.line("%s = %s;", name, value)
	case "expr":
		value := g.expr(s.Expr)
		if value != "" {
			g.line("(void)(%s);", value)
		}
	case "return":
		if s.Expr == nil {
			g.line("return;")
		} else {
			value := g.expr(s.Expr)
			if g.checked.diverges[s.Expr] {
				return
			}
			if value == "" {
				g.line("return;")
			} else {
				g.line("return %s;", value)
			}
		}
	case "if":
		condition := g.expr(s.Expr)
		g.line("if (%s)", condition)
		g.block(s.Body, false, "()")
		if s.Else != nil {
			g.line("else")
			g.block(s.Else, false, "()")
		}
	case "while":
		g.line("while (true) {")
		g.indent++
		condition := g.expr(s.Expr)
		if !g.checked.diverges[s.Expr] {
			g.line("if (!(%s)) break;", condition)
			g.block(s.Body, false, "()")
		}
		g.indent--
		g.line("}")
	case "block":
		g.block(s.Body, false, "()")
	case "break", "continue":
		g.line("%s;", s.Kind)
	default:
		panic("unchecked statement: " + s.Kind)
	}
}
func (g *generator) arithmetic(op, t, a, b string) string {
	name := map[string]string{"+": "add", "-": "sub", "*": "mul", "/": "div", "%": "rem"}[op]
	return fmt.Sprintf("fr_%s_%s(%s, %s)", name, t, a, b)
}
func (g *generator) expr(e *syntax.Expr) string {
	t := g.checked.types[e]
	switch e.Kind {
	case "field":
		value := g.expr(e.Left)
		if g.checked.diverges[e.Left] {
			return ""
		}
		return g.temp(t, "("+value+")."+cField(e.Text))
	case "struct":
		fields := make([]string, 0, len(e.Fields))
		for _, field := range e.Fields {
			value := g.expr(field.Value)
			if g.checked.diverges[field.Value] {
				return ""
			}
			fields = append(fields, "."+cField(field.Name)+" = "+value)
		}
		if len(fields) == 0 {
			fields = append(fields, ".fr_empty = 0")
		}
		return g.temp(t, "("+cType(t)+"){"+strings.Join(fields, ", ")+"}")
	case "unit":
		return ""
	case "block":
		target := g.resultTarget(e)
		g.valueBlock(e.Body, target)
		return target
	case "if":
		condition := g.expr(e.Left)
		if g.checked.diverges[e.Left] {
			return ""
		}
		target := g.resultTarget(e)
		g.line("if (%s)", condition)
		g.valueBlock(e.Body, target)
		if e.Else != nil {
			g.line("else")
			g.valueBlock(e.Else, target)
		}
		return target
	case "int":
		n, _, _ := parseInteger(e.Text)
		return integerC(n, t)
	case "bool":
		return e.Text
	case "string":
		return cString(e.Text)
	case "name":
		// Snapshot variables before evaluating a later operand or argument.
		return g.temp(t, g.checked.names[e])
	case "unary":
		if e.Op == "-" && e.Right.Kind == "int" {
			n, _, _ := parseInteger(e.Right.Text)
			return integerC(new(big.Int).Neg(n), t)
		}
		a := g.expr(e.Right)
		if g.checked.diverges[e.Right] {
			return ""
		}
		if e.Op == "!" {
			return g.temp(t, "!("+a+")")
		}
		return g.temp(t, fmt.Sprintf("fr_neg_%s(%s)", t, a))
	case "binary":
		a := g.expr(e.Left)
		if g.checked.diverges[e.Left] {
			return ""
		}
		if e.Op == "&&" || e.Op == "||" {
			name := g.temp("bool", a)
			condition := name
			if e.Op == "||" {
				condition = "!" + name
			}
			g.line("if (%s) {", condition)
			g.indent++
			b := g.expr(e.Right)
			if !g.checked.diverges[e.Right] {
				g.line("%s = %s;", name, b)
			}
			g.indent--
			g.line("}")
			return name
		}
		b := g.expr(e.Right)
		if g.checked.diverges[e.Right] {
			return ""
		}
		if strings.Contains("+-*/%", e.Op) {
			return g.temp(t, g.arithmetic(e.Op, t, a, b))
		}
		if g.checked.types[e.Left] == "&str" {
			value := fmt.Sprintf("fr_str_eq(%s, %s)", a, b)
			if e.Op == "!=" {
				value = "!" + value
			}
			return g.temp("bool", value)
		}
		return g.temp(t, fmt.Sprintf("(%s %s %s)", a, e.Op, b))
	case "call":
		args := make([]string, len(e.Args))
		for i, a := range e.Args {
			args[i] = g.expr(a)
			if g.checked.diverges[a] {
				return ""
			}
		}
		call := g.checked.calls[e] + "(" + strings.Join(args, ", ") + ")"
		if t == "()" {
			g.line("%s;", call)
			return ""
		}
		return g.temp(t, call)
	case "macro":
		return g.macro(e)
	}
	panic("unchecked expression: " + e.Kind)
}
func integerC(n *big.Int, t string) string {
	bits := 32
	if t == "i64" {
		bits = 64
	}
	min := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), uint(bits-1)))
	if n.Cmp(min) == 0 {
		return fmt.Sprintf("INT%d_MIN", bits)
	}
	if n.Sign() < 0 {
		return fmt.Sprintf("(-INT%d_C(%s))", bits, new(big.Int).Abs(n).String())
	}
	return fmt.Sprintf("INT%d_C(%s)", bits, n.String())
}
func cString(s string) string {
	// Octal byte escapes avoid C's variable-length hex escapes and preserve UTF-8/NUL.
	var b strings.Builder
	b.WriteString("(fr_str){(const unsigned char *)\"")
	for i := 0; i < len(s); i++ {
		fmt.Fprintf(&b, "\\%03o", s[i])
	}
	fmt.Fprintf(&b, "\", %d}", len(s))
	return b.String()
}
func (g *generator) macro(e *syntax.Expr) string {
	if e.Text == "assert" {
		condition := g.expr(e.Args[0])
		if g.checked.diverges[e.Args[0]] {
			return ""
		}
		g.line("if (!(%s)) fr_panic(\"assertion failed at line %d\");", condition, e.Span.Line)
		return ""
	}
	if len(e.Args) == 0 {
		g.line("fputc('\\n', stdout);")
		return ""
	}
	args := make([]string, len(e.Args)-1)
	for i, a := range e.Args[1:] {
		args[i] = g.expr(a)
		if g.checked.diverges[a] {
			return ""
		}
	}
	var literal strings.Builder
	flush := func() {
		if literal.Len() > 0 {
			g.line("fr_write(%s);", cString(literal.String()))
			literal.Reset()
		}
	}
	format := e.Args[0].Text
	index := 0
	for i := 0; i < len(format); i++ {
		switch {
		case format[i] == '{' && format[i+1] == '{':
			literal.WriteByte('{')
			i++
		case format[i] == '}' && format[i+1] == '}':
			literal.WriteByte('}')
			i++
		case format[i] == '{':
			flush()
			arg := e.Args[index+1]
			value := args[index]
			switch g.checked.types[arg] {
			case "i32":
				g.line("printf(\"%%\" PRId32, %s);", value)
			case "i64":
				g.line("printf(\"%%\" PRId64, %s);", value)
			case "bool":
				g.line("fputs(%s ? \"true\" : \"false\", stdout);", value)
			case "&str":
				g.line("fr_write(%s);", value)
			}
			index++
			i++
		default:
			literal.WriteByte(format[i])
		}
	}
	flush()
	if e.Text == "println" {
		g.line("fputc('\\n', stdout);")
	}
	return ""
}
