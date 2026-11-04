package syntax

import (
	"fmt"
	"strings"
)

type parser struct {
	tokens          []token
	index           int
	grouped         map[*Expr]bool
	noStructLiteral bool
	rangeBoundary   bool
}

// Parse parses the explicitly supported bootstrap Rust language. Unsupported
// syntax is an error; no token is discarded to recover from a parse error.
func Parse(source string) (*Program, error) {
	tokens, err := lex(source)
	if err != nil {
		return nil, err
	}
	p := &parser{tokens: tokens, grouped: make(map[*Expr]bool)}
	return p.items("eof")
}

// items parses the item-only namespace of a crate or inline module. Function
// bodies use block(), so namespace items are never silently hoisted from them.
func (p *parser) items(end string) (*Program, error) {
	program := &Program{}
	for !p.at(end) {
		start := p.current()
		if p.at("eof") {
			return nil, sourceError(start.span, "expected %s, found end of input", end)
		}
		derived := false
		if p.at("#") {
			if err := p.copyDerive(); err != nil {
				return nil, err
			}
			derived = true
			if p.at("#") {
				return nil, sourceError(p.current().span, "multiple attributes are not supported")
			}
		}
		public := p.take("pub")
		if public && p.at("(") {
			return nil, sourceError(p.current().span, "restricted visibility is not supported")
		}
		if derived && !p.at("struct") {
			return nil, sourceError(start.span, "Copy/Clone derive is only supported on named structs")
		}
		switch {
		case p.take("struct"):
			if !derived {
				return nil, sourceError(start.span, "structs require #[derive(Copy, Clone)] in this bootstrap")
			}
			declaration, err := p.structDeclaration(start, public)
			if err != nil {
				return nil, err
			}
			program.Structs = append(program.Structs, declaration)
		case p.at("fn"):
			f, err := p.function()
			if err != nil {
				return nil, err
			}
			f.Public = public
			f.Span = start.span
			program.Functions = append(program.Functions, f)
		case p.take("mod"):
			name, err := p.identifier()
			if err != nil {
				return nil, err
			}
			module := &Module{Name: name.text, Public: public, Span: start.span}
			if p.take(";") {
				module.External = true
			} else {
				if _, err := p.expect("{"); err != nil {
					return nil, err
				}
				module.Program, err = p.items("}")
				if err != nil {
					return nil, err
				}
				if _, err := p.expect("}"); err != nil {
					return nil, err
				}
			}
			program.Modules = append(program.Modules, module)
		case p.take("use"):
			_, path, err := p.path()
			if err != nil {
				return nil, err
			}
			item := Import{Path: path, Public: public, Span: start.span}
			if p.take("as") {
				alias, err := p.identifier()
				if err != nil {
					return nil, err
				}
				item.Alias = alias.text
			}
			segments := strings.Split(item.Path, "::")
			last := segments[len(segments)-1]
			if item.Alias == "" && (last == "crate" || last == "self" || last == "super") {
				return nil, sourceError(start.span, "imports of crate/self/super must have an explicit alias")
			}
			if _, err := p.expect(";"); err != nil {
				return nil, err
			}
			program.Imports = append(program.Imports, item)
		default:
			return nil, sourceError(p.current().span, "expected fn, mod, use, or derived struct item, found %s", describe(p.current()))
		}
	}
	return program, nil
}

// copyDerive recognizes only the semantic marker implemented by this compiler.
func (p *parser) copyDerive() error {
	start, _ := p.expect("#")
	if _, err := p.expect("["); err != nil {
		return err
	}
	if !p.take("derive") {
		return sourceError(start.span, "only #[derive(Copy, Clone)] attributes are supported")
	}
	if _, err := p.expect("("); err != nil {
		return err
	}
	seen := map[string]bool{}
	for !p.at(")") {
		name, err := p.identifier()
		if err != nil {
			return err
		}
		if (name.text != "Copy" && name.text != "Clone") || seen[name.text] {
			return sourceError(name.span, "derive must contain Copy and Clone exactly once")
		}
		seen[name.text] = true
		if !p.take(",") {
			break
		}
	}
	if _, err := p.expect(")"); err != nil {
		return err
	}
	if _, err := p.expect("]"); err != nil {
		return err
	}
	if !seen["Copy"] || !seen["Clone"] {
		return sourceError(start.span, "derive must contain both Copy and Clone")
	}
	return nil
}

func (p *parser) structDeclaration(start token, public bool) (*Struct, error) {
	name, err := p.identifier()
	if err != nil {
		return nil, err
	}
	if p.at("(") || p.at(";") {
		return nil, sourceError(p.current().span, "tuple and unit structs are not supported; use named fields")
	}
	if p.at("<") {
		return nil, sourceError(p.current().span, "generic structs are not supported")
	}
	if _, err := p.expect("{"); err != nil {
		return nil, err
	}
	declaration := &Struct{Name: name.text, Public: public, Copy: true, Span: start.span}
	for !p.at("}") {
		fieldStart := p.current()
		public := p.take("pub")
		if public && p.at("(") {
			return nil, sourceError(p.current().span, "restricted field visibility is not supported")
		}
		field, err := p.identifier()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(":"); err != nil {
			return nil, err
		}
		typ, err := p.typeName()
		if err != nil {
			return nil, err
		}
		declaration.Fields = append(declaration.Fields, StructField{Name: field.text, Type: typ, Public: public, Span: fieldStart.span})
		if !p.take(",") {
			break
		}
	}
	if _, err := p.expect("}"); err != nil {
		return nil, err
	}
	return declaration, nil
}

func (p *parser) structLiteral(e *Expr) (*Expr, error) {
	p.index++ // opening brace, already inspected by atom
	e.Kind = "struct"
	for !p.at("}") {
		if p.rangeOperator() {
			return nil, sourceError(p.current().span, "struct update syntax is not supported")
		}
		field, err := p.identifier()
		if err != nil {
			return nil, err
		}
		value := &Expr{Kind: "name", Text: field.text, Span: field.span}
		if p.take(":") {
			value, err = p.fullExpression(1)
			if err != nil {
				return nil, err
			}
		}
		e.Fields = append(e.Fields, FieldValue{Name: field.text, Value: value, Span: field.span})
		if !p.take(",") {
			break
		}
	}
	if _, err := p.expect("}"); err != nil {
		return nil, err
	}
	return e, nil
}

func (p *parser) postfix(e *Expr, allowIndex bool) (*Expr, error) {
	for {
		if allowIndex && p.take("[") {
			if p.rangeOperator() {
				return nil, sourceError(p.current().span, "slices and ranges are not supported")
			}
			if p.at("]") {
				return nil, sourceError(p.current().span, "array indexing requires an expression")
			}
			index, err := p.fullExpression(1)
			if err != nil {
				return nil, err
			}
			if p.rangeOperator() {
				return nil, sourceError(p.current().span, "slices and ranges are not supported")
			}
			if _, err := p.expect("]"); err != nil {
				return nil, err
			}
			e = &Expr{Kind: "index", Left: e, Right: index, Span: e.Span}
			continue
		}
		if !p.take(".") {
			break
		}
		if p.current().kind == "int" {
			return nil, sourceError(p.current().span, "tuple fields and floating-point literals are not supported")
		}
		field, err := p.identifier()
		if err != nil {
			return nil, err
		}
		if p.take("(") {
			if field.text != "len" {
				return nil, sourceError(field.span, "methods and callable fields are not supported")
			}
			if !p.take(")") {
				return nil, sourceError(p.current().span, "array .len() takes no arguments")
			}
			e = &Expr{Kind: "len", Left: e, Span: e.Span}
		} else {
			e = &Expr{Kind: "field", Text: field.text, Left: e, Span: e.Span}
		}
		// Dot projection makes a bare block/if an ordinary expression; subsequent
		// indexing is then permitted even in statement position.
		allowIndex = true
	}
	return e, nil
}

func (p *parser) arrayLengthLiteral() (string, error) {
	length := p.current()
	if length.kind != "int" {
		return "", sourceError(length.span, "array lengths must be unsuffixed or usize integer literals; constant expressions are not supported")
	}
	for _, suffix := range []string{"i8", "i16", "i32", "i64", "isize", "u8", "u16", "u32", "u64"} {
		if strings.HasSuffix(length.text, suffix) {
			return "", sourceError(length.span, "array lengths must be unsuffixed or usize integer literals")
		}
	}
	p.index++
	return length.text, nil
}

func (p *parser) arrayLiteral(start token) (*Expr, error) {
	array := &Expr{Kind: "array", Span: start.span}
	if p.take("]") {
		return array, nil
	}
	if p.rangeOperator() {
		return nil, sourceError(p.current().span, "array spread and range syntax are not supported")
	}
	first, err := p.fullExpression(1)
	if err != nil {
		return nil, err
	}
	if p.take(";") {
		length, err := p.arrayLengthLiteral()
		if err != nil {
			return nil, err
		}
		if !p.at("]") {
			return nil, sourceError(p.current().span, "array repeat counts must be a single integer literal")
		}
		p.index++
		return &Expr{Kind: "repeat", Left: first, Text: length, Span: start.span}, nil
	}
	array.Args = append(array.Args, first)
	for p.take(",") {
		if p.at("]") {
			break
		}
		value, err := p.fullExpression(1)
		if err != nil {
			return nil, err
		}
		array.Args = append(array.Args, value)
	}
	if p.rangeOperator() {
		return nil, sourceError(p.current().span, "array spread and range syntax are not supported")
	}
	if _, err := p.expect("]"); err != nil {
		return nil, err
	}
	return array, nil
}

func (p *parser) fullExpression(min int) (*Expr, error) {
	previous, previousRange := p.noStructLiteral, p.rangeBoundary
	p.noStructLiteral, p.rangeBoundary = false, false
	defer func() { p.noStructLiteral, p.rangeBoundary = previous, previousRange }()
	return p.expression(min)
}

func (p *parser) condition() (*Expr, error) {
	previous := p.noStructLiteral
	p.noStructLiteral = true
	defer func() { p.noStructLiteral = previous }()
	return p.expression(1)
}

// path keeps source spelling for resolution. Keywords may lead a path, with
// repeated super segments allowed only before the first ordinary identifier.
func (p *parser) path() (token, string, error) {
	start := p.current()
	if p.at("{") {
		return token{}, "", sourceError(start.span, "group imports are not supported")
	}
	if p.at("*") {
		return token{}, "", sourceError(start.span, "glob imports are not supported")
	}
	if p.at("::") {
		return token{}, "", sourceError(start.span, "absolute paths are not supported")
	}
	var first token
	var err error
	if p.at("crate") || p.at("self") || p.at("super") {
		first = p.current()
		p.index++
	} else {
		first, err = p.identifier()
		if err != nil {
			return token{}, "", err
		}
	}
	parts := []string{first.text}
	superPrefix := first.text == "super"
	for p.take("::") {
		if p.at("{") {
			return token{}, "", sourceError(p.current().span, "group imports are not supported")
		}
		if p.at("*") {
			return token{}, "", sourceError(p.current().span, "glob imports are not supported")
		}
		if superPrefix && p.at("super") {
			parts = append(parts, p.current().text)
			p.index++
			continue
		}
		segment, err := p.identifier()
		if err != nil {
			return token{}, "", sourceError(p.current().span, "expected path segment, found %s", describe(p.current()))
		}
		parts = append(parts, segment.text)
		superPrefix = false
	}
	return start, strings.Join(parts, "::"), nil
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
	if p.take("[") {
		element, err := p.typeName()
		if err != nil {
			return "", err
		}
		if p.at("]") {
			return "", sourceError(start.span, "slices are not supported; array types require [T; N]")
		}
		if _, err := p.expect(";"); err != nil {
			return "", err
		}
		length, err := p.arrayLengthLiteral()
		if err != nil {
			return "", err
		}
		if !p.at("]") {
			return "", sourceError(p.current().span, "array type lengths must be a single integer literal")
		}
		p.index++
		return "[" + element + ";" + length + "]", nil
	}
	if p.take("(") {
		if p.take(")") {
			return "()", nil
		}
		typ, err := p.typeName()
		if err != nil {
			return "", err
		}
		if _, err := p.expect(")"); err != nil {
			return "", err
		}
		return typ, nil
	}
	if p.take("&") {
		if p.at("[") {
			return "", sourceError(start.span, "array references and slices are not supported")
		}
		if _, err := p.expect("str"); err != nil {
			return "", err
		}
		return "&str", nil
	}
	if start.kind == "ident" || p.at("::") {
		_, path, err := p.path()
		if err != nil {
			return "", err
		}
		if p.at("<") {
			return "", sourceError(p.current().span, "generic type arguments are not supported")
		}
		return path, nil
	}
	return "", sourceError(start.span, "unsupported type %s", describe(start))
}

func (p *parser) block() (*Block, error) {
	previous, previousRange := p.noStructLiteral, p.rangeBoundary
	p.noStructLiteral, p.rangeBoundary = false, false
	defer func() { p.noStructLiteral, p.rangeBoundary = previous, previousRange }()
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
		if (e.Kind == "name" || e.Kind == "field" || e.Kind == "index") && isAssignment(p.current().kind) {
			op := p.current()
			p.index++
			rhs, err := p.expression(1)
			if err != nil {
				return nil, err
			}
			if _, err = p.expect(";"); err != nil {
				return nil, err
			}
			assignment := Stmt{Kind: "assign", Op: op.kind, Expr: rhs, Span: e.Span}
			if e.Kind == "field" || e.Kind == "index" {
				assignment.Target = e
			} else {
				assignment.Name = e.Text
			}
			b.Statements = append(b.Statements, assignment)
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
		e, err := p.atom()
		if err != nil {
			return nil, err
		}
		e, err = p.postfix(e, false)
		if err != nil {
			return nil, err
		}
		if e.Kind == "if" || e.Kind == "block" {
			return e, nil
		}
		return p.expressionRest(e, 1)
	}
	return p.expression(1)
}

func isAssignment(kind string) bool {
	return kind == "=" || kind == "+=" || kind == "-=" || kind == "*=" || kind == "/=" || kind == "%=" || kind == "&=" || kind == "|=" || kind == "^=" || kind == "<<=" || kind == ">>="
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
	case p.take("for"):
		s.Kind = "for"
		s.Mutable = p.take("mut")
		if p.take("_") {
			if s.Mutable {
				return s, true, sourceError(start.span, "mut must be followed by a named binding; wildcard _ cannot be mutable")
			}
			s.Name = "_"
		} else {
			name, patternErr := p.identifier()
			if patternErr != nil {
				return s, true, sourceError(p.current().span, "for patterns support only a name, mut name, or _")
			}
			s.Name = name.text
		}
		if _, err = p.expect("in"); err != nil {
			return s, true, err
		}
		s.Expr, err = p.forIterable(true)
		if err != nil {
			return s, true, err
		}
		s.Body, err = p.block()
	case p.take("while"):
		s.Kind = "while"
		s.Expr, err = p.condition()
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
	case p.at("fn") || p.at("mod") || p.at("use") || p.at("pub") || p.at("struct") || p.at("#"):
		return s, true, sourceError(start.span, "function-local items and imports are not supported")
	default:
		return s, false, nil
	}
	return s, true, err
}

func (p *parser) rangeOperator() bool { return p.at("..") || p.at("..=") }

func (p *parser) forBound(restrictStruct bool) (*Expr, error) {
	previousRange, previousStruct := p.rangeBoundary, p.noStructLiteral
	p.rangeBoundary, p.noStructLiteral = true, restrictStruct
	defer func() { p.rangeBoundary, p.noStructLiteral = previousRange, previousStruct }()
	return p.expression(1)
}

func (p *parser) forIterable(restrictStruct bool) (*Expr, error) {
	if p.rangeOperator() {
		return nil, sourceError(p.current().span, "for ranges require both start and end bounds; open ranges are not supported")
	}
	start, err := p.forBound(restrictStruct)
	if err != nil {
		return nil, err
	}
	if !p.rangeOperator() {
		return start, nil
	}
	operator := p.current()
	p.index++
	if (restrictStruct && p.at("{")) || p.at("}") || p.at(")") || p.at(";") || p.at("eof") {
		return nil, sourceError(p.current().span, "for ranges require an end bound; open ranges are not supported")
	}
	end, err := p.forBound(restrictStruct)
	if err != nil {
		return nil, err
	}
	if p.rangeOperator() {
		return nil, sourceError(p.current().span, "chained range bounds are not supported")
	}
	return &Expr{Kind: "range", Left: start, Right: end, Op: operator.kind, Span: start.Span}, nil
}

func (p *parser) ifExpression(start token) (*Expr, error) {
	e := &Expr{Kind: "if", Span: start.span}
	var err error
	e.Left, err = p.condition()
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

var precedence = map[string]int{
	"||": 1, "&&": 2,
	"==": 3, "!=": 3, "<": 3, "<=": 3, ">": 3, ">=": 3,
	"|": 4, "^": 5, "&": 6, "<<": 7, ">>": 7,
	"+": 8, "-": 8, "*": 9, "/": 9, "%": 9,
}

const castPrecedence = 10

func comparisonOperator(op string) bool {
	return op == "==" || op == "!=" || op == "<" || op == "<=" || op == ">" || op == ">="
}

func (p *parser) expression(min int) (*Expr, error) {
	left, err := p.prefix()
	if err != nil {
		return nil, err
	}
	return p.expressionRest(left, min)
}

func (p *parser) expressionRest(left *Expr, min int) (*Expr, error) {
	for {
		if p.rangeOperator() {
			if p.rangeBoundary {
				break
			}
			return nil, sourceError(p.current().span, "range syntax is supported only as direct bounded for headers")
		}
		if p.at("as") {
			if castPrecedence < min {
				break
			}
			operator := p.current()
			p.index++
			if p.at("_") {
				return nil, sourceError(p.current().span, "inferred cast targets are not supported")
			}
			groupedType := p.at("(")
			target, err := p.typeName()
			if err != nil {
				return nil, err
			}
			if !groupedType && (p.at("<") || p.at("<<")) {
				return nil, sourceError(operator.span, "casts followed by < or << require parentheses around the cast or target type")
			}
			left = &Expr{Kind: "cast", Left: left, Text: target, Span: left.Span}
			continue
		}
		op := p.current()
		prec := precedence[op.kind]
		if prec < min {
			break
		}
		if comparisonOperator(op.kind) {
			if left.Kind == "binary" && comparisonOperator(left.Op) && !p.grouped[left] {
				return nil, sourceError(op.span, "comparison operators cannot be chained")
			}
		}
		p.index++
		right, err := p.expression(prec + 1)
		if err != nil {
			return nil, err
		}
		if comparisonOperator(op.kind) {
			if right.Kind == "binary" && comparisonOperator(right.Op) && !p.grouped[right] {
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
		rhs, err := p.expression(castPrecedence + 1)
		if err != nil {
			return nil, err
		}
		return &Expr{Kind: "unary", Op: t.kind, Right: rhs, Span: t.span}, nil
	}
	e, err := p.atom()
	if err != nil {
		return nil, err
	}
	return p.postfix(e, true)
}

func (p *parser) atom() (*Expr, error) {
	t := p.current()
	if p.rangeOperator() {
		return nil, sourceError(t.span, "open ranges are not supported; for ranges require start and end bounds")
	}
	if p.at("&") || p.at("&&") {
		return nil, sourceError(t.span, "references, borrows and slices are not supported")
	}
	if p.take("[") {
		return p.arrayLiteral(t)
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
		var e *Expr
		var err error
		if p.rangeBoundary {
			e, err = p.forIterable(false)
		} else {
			e, err = p.fullExpression(1)
		}
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
	name, path, err := p.path()
	if err != nil {
		return nil, err
	}
	e := &Expr{Kind: "name", Text: path, Span: name.span}
	if p.at("{") && !p.noStructLiteral {
		return p.structLiteral(e)
	}
	macro := p.take("!")
	if macro && strings.Contains(path, "::") {
		return nil, sourceError(name.span, "qualified macros are not supported")
	}
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
			arg, err := p.fullExpression(1)
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
