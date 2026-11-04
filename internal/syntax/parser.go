package syntax

import "fmt"

type parser struct {
	tokens  []token
	index   int
	grouped map[*Expr]bool
}

// Parse parses the explicitly supported bootstrap Rust language. Unsupported
// syntax is an error; no token is discarded to recover from a parse error.
func Parse(source string) (*Program, error) {
	tokens, err := lex(source)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens, grouped: make(map[*Expr]bool)}
	program := &Program{}
	for !p.at("eof") {
		f, err := p.function()
		if err != nil {
			return nil, err
		}
		program.Functions = append(program.Functions, f)
	}
	return program, nil
}

func (p *parser) current() token { return p.tokens[p.index] }
func (p *parser) at(kind string) bool {
	t := p.current()
	return t.kind == kind || t.kind == "ident" && t.text == kind
}
func (p *parser) take(kind string) bool {
	if p.at(kind) {
		p.index++
		return true
	}
	return false
}
func (p *parser) expect(kind string) (token, error) {
	t := p.current()
	if !p.take(kind) {
		return token{}, sourceError(t.span, "expected %s, found %s", kind, describe(t))
	}
	return t, nil
}
func describe(t token) string {
	if t.kind == "eof" {
		return "end of input"
	}
	return fmt.Sprintf("%q", t.text)
}

var reserved = map[string]bool{
	"as": true, "async": true, "await": true, "break": true, "const": true, "continue": true, "crate": true, "dyn": true, "else": true, "enum": true, "extern": true, "false": true, "fn": true, "for": true, "if": true, "impl": true, "in": true, "let": true, "loop": true, "match": true, "mod": true, "move": true, "mut": true, "pub": true, "ref": true, "return": true, "self": true, "Self": true, "static": true, "struct": true, "super": true, "trait": true, "true": true, "type": true, "unsafe": true, "use": true, "where": true, "while": true, "abstract": true, "become": true, "box": true, "do": true, "final": true, "macro": true, "override": true, "priv": true, "typeof": true, "unsized": true, "virtual": true, "yield": true, "try": true, "gen": true,
}

func (p *parser) identifier() (token, error) {
	t := p.current()
	if t.kind != "ident" || reserved[t.text] || t.text == "_" {
		return token{}, sourceError(t.span, "expected identifier, found %s", describe(t))
	}
	p.index++
	return t, nil
}

func (p *parser) function() (*Function, error) {
	start, err := p.expect("fn")
	if err != nil {
		return nil, err
	}
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if _, err = p.expect("("); err != nil {
		return nil, err
	}
	f := &Function{Name: name.text, ReturnType: "()", Span: start.span}
	for !p.at(")") {
		param, err := p.identifier()
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(":"); err != nil {
			return nil, err
		}
		typ, err := p.typeName()
		if err != nil {
			return nil, err
		}
		f.Params = append(f.Params, Param{Name: param.text, Type: typ, Span: param.span})
		if !p.take(",") {
			break
		}
	}
	if _, err = p.expect(")"); err != nil {
		return nil, err
	}
	if p.take("->") {
		f.ReturnType, err = p.typeName()
		if err != nil {
			return nil, err
		}
	}
	f.Body, err = p.block()
	return f, err
}

func (p *parser) typeName() (string, error) {
	start := p.current()
	if p.take("(") {
		if _, err := p.expect(")"); err != nil {
			return "", err
		}
		return "()", nil
	}
	if p.take("&") {
		if _, err := p.expect("str"); err != nil {
			return "", err
		}
		return "&str", nil
	}
	for _, typ := range []string{"i32", "i64", "bool"} {
		if p.take(typ) {
			return typ, nil
		}
	}
	return "", sourceError(start.span, "unsupported type %s (supported: i32, i64, bool, &str, ())", describe(start))
}

func (p *parser) block() (*Block, error) {
	start, err := p.expect("{")
	if err != nil {
		return nil, err
	}
	b := &Block{Span: start.span}
	for !p.at("}") {
		if p.at("eof") {
			return nil, sourceError(p.current().span, "expected }, found end of input")
		}
		if p.take(";") {
			continue
		}
		s, isStatement, err := p.statement()
		if err != nil {
			return nil, err
		}
		if isStatement {
			b.Statements = append(b.Statements, s)
			continue
		}
		e, err := p.statementExpression()
		if err != nil {
			return nil, err
		}
		if e.Kind == "name" && isAssignment(p.current().kind) {
			op := p.current()
			p.index++
			rhs, err := p.expression(1)
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(";"); err != nil {
				return nil, err
			}
			b.Statements = append(b.Statements, Stmt{Kind: "assign", Name: e.Text, Op: op.kind, Expr: rhs, Span: e.Span})
			continue
		}
		if p.take(";") {
			b.Statements = append(b.Statements, Stmt{Kind: "expr", Expr: e, Span: e.Span})
			continue
		}
		if p.at("}") {
			b.Tail = e
			break
		}
		if (e.Kind == "if" || e.Kind == "block") && !p.grouped[e] {
			b.Statements = append(b.Statements, Stmt{Kind: "expr", Type: "()", Expr: e, Span: e.Span})
			continue
		}
		return nil, sourceError(p.current().span, "expected ; or }, found %s", describe(p.current()))
	}
	if _, err := p.expect("}"); err != nil {
		return nil, err
	}
	return b, nil
}

// A bare block/if in statement position ends before infix operators. This is
// how Rust separates `{} -1;` into a unit block and a unary-minus statement.
// Parentheses opt back into ordinary expression parsing, as do all RHS contexts.
func (p *parser) statementExpression() (*Expr, error) {
	if p.at("if") || p.at("{") {
		return p.prefix()
	}
	return p.expression(1)
}

func isAssignment(kind string) bool {
	return kind == "=" || kind == "+=" || kind == "-=" || kind == "*=" || kind == "/=" || kind == "%="
}

func (p *parser) statement() (Stmt, bool, error) {
	start := p.current()
	s := Stmt{Span: start.span}
	var err error
	switch {
	case p.take("let"):
		s.Kind = "let"
		s.Mutable = p.take("mut")
		var name token
		name, err = p.identifier()
		if err != nil {
			return s, true, err
		}
		s.Name = name.text
		if p.take(":") {
			s.Type, err = p.typeName()
			if err != nil {
				return s, true, err
			}
		}
		if _, err = p.expect("="); err != nil {
			return s, true, err
		}
		s.Expr, err = p.expression(1)
		if err != nil {
			return s, true, err
		}
		_, err = p.expect(";")
	case p.take("return"):
		s.Kind = "return"
		if !p.at(";") {
			s.Expr, err = p.expression(1)
			if err != nil {
				return s, true, err
			}
		}
		_, err = p.expect(";")
	case p.take("while"):
		s.Kind = "while"
		s.Expr, err = p.expression(1)
		if err != nil {
			return s, true, err
		}
		s.Body, err = p.block()
	case p.take("break"):
		s.Kind = "break"
		_, err = p.expect(";")
	case p.take("continue"):
		s.Kind = "continue"
		_, err = p.expect(";")
	default:
		return s, false, nil
	}
	return s, true, err
}

func (p *parser) ifExpression(start token) (*Expr, error) {
	e := &Expr{Kind: "if", Span: start.span}
	var err error
	e.Left, err = p.expression(1)
	if err != nil {
		return nil, err
	}
	e.Body, err = p.block()
	if err != nil {
		return nil, err
	}
	if p.take("else") {
		if p.at("if") {
			nestedStart := p.current()
			p.index++
			nested, err := p.ifExpression(nestedStart)
			if err != nil {
				return nil, err
			}
			e.Else = &Block{Tail: nested, Span: nestedStart.span}
		} else {
			e.Else, err = p.block()
			if err != nil {
				return nil, err
			}
		}
	}
	return e, nil
}

var precedence = map[string]int{"||": 1, "&&": 2, "==": 3, "!=": 3, "<": 4, "<=": 4, ">": 4, ">=": 4, "+": 5, "-": 5, "*": 6, "/": 6, "%": 6}

func (p *parser) expression(min int) (*Expr, error) {
	left, err := p.prefix()
	if err != nil {
		return nil, err
	}
	for {
		op := p.current()
		prec := precedence[op.kind]
		if prec < min {
			break
		}
		if prec == 3 || prec == 4 {
			previous := precedence[left.Op]
			if left.Kind == "binary" && (previous == 3 || previous == 4) && !p.grouped[left] {
				return nil, sourceError(op.span, "comparison operators cannot be chained")
			}
		}
		p.index++
		right, err := p.expression(prec + 1)
		if err != nil {
			return nil, err
		}
		if prec == 3 || prec == 4 {
			next := precedence[right.Op]
			if right.Kind == "binary" && (next == 3 || next == 4) && !p.grouped[right] {
				return nil, sourceError(op.span, "comparison operators cannot be chained")
			}
		}
		left = &Expr{Kind: "binary", Op: op.kind, Left: left, Right: right, Span: left.Span}
	}
	return left, nil
}

func (p *parser) prefix() (*Expr, error) {
	t := p.current()
	if p.take("-") || p.take("!") {
		rhs, err := p.expression(7)
		if err != nil {
			return nil, err
		}
		return &Expr{Kind: "unary", Op: t.kind, Right: rhs, Span: t.span}, nil
	}
	if p.take("if") {
		return p.ifExpression(t)
	}
	if p.at("{") {
		body, err := p.block()
		if err != nil {
			return nil, err
		}
		return &Expr{Kind: "block", Body: body, Span: t.span}, nil
	}
	if p.take("(") {
		if p.take(")") {
			return &Expr{Kind: "unit", Span: t.span}, nil
		}
		e, err := p.expression(1)
		if err != nil {
			return nil, err
		}
		if _, err = p.expect(")"); err != nil {
			return nil, err
		}
		p.grouped[e] = true
		return e, nil
	}
	if t.kind == "int" || t.kind == "string" {
		p.index++
		return &Expr{Kind: t.kind, Text: t.text, Span: t.span}, nil
	}
	if p.take("true") || p.take("false") {
		return &Expr{Kind: "bool", Text: t.text, Span: t.span}, nil
	}
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	e := &Expr{Kind: "name", Text: name.text, Span: name.span}
	macro := p.take("!")
	if macro || p.at("(") {
		if _, err = p.expect("("); err != nil {
			return nil, err
		}
		if macro {
			e.Kind = "macro"
		} else {
			e.Kind = "call"
		}
		for !p.at(")") {
			arg, err := p.expression(1)
			if err != nil {
				return nil, err
			}
			e.Args = append(e.Args, arg)
			if !p.take(",") {
				break
			}
		}
		if _, err = p.expect(")"); err != nil {
			return nil, err
		}
	}
	return e, nil
}
