package compiler

import "github.com/lassejlv/rust-port-go/internal/syntax"

func (c *checker) castExpression(e *syntax.Expr) (string, error) {
	if !integerType(e.Target) {
		return "", fail(e.Span, "cast target must be a supported integer type")
	}
	fixed := c.fixedCastSource(e.Left)
	context := ""
	if !fixed && castForwardsExpectation(e.Left) {
		context = e.Target
	}
	source, err := c.expr(e.Left, context)
	if err != nil {
		return "", err
	}
	if c.bottom[e.Left] {
		return "", fail(e.Span, "value-less divergent cast sources are unsupported")
	}
	if !integerType(source) && source != "bool" {
		return "", fail(e.Span, "cast source must be an integer or boolean, found %s", source)
	}
	c.diverges[e] = c.diverges[e.Left]
	return e.Target, nil
}

// Rust forwards a cast's type into a syntactic literal through unary operators
// and block tails. Binary and conditional expressions form an inference
// boundary: their source type defaults independently before conversion.
func castForwardsExpectation(e *syntax.Expr) bool {
	if e == nil {
		return false
	}
	switch e.Kind {
	case "int":
		return true
	case "unary":
		return castForwardsExpectation(e.Right)
	case "block":
		return e.Body != nil && castForwardsExpectation(e.Body.Tail)
	default:
		return false
	}
}

// fixedCastSource tracks whether a source's type is fixed independently of a
// surrounding cast. Rust constrains unsuffixed cast operands with the target;
// blindly defaulting them to i32 would turn invalid literals into valid wraps.
func (c *checker) fixedCastSource(e *syntax.Expr) bool {
	if e == nil {
		return false
	}
	switch e.Kind {
	case "int":
		_, suffix, err := parseInteger(e.Text)
		return err == nil && suffix != ""
	case "name":
		if name := c.names[e]; name != "" {
			return true
		}
		_, ok := c.lookup(e.Text)
		return ok
	case "cast", "call", "field", "index", "len", "array", "repeat", "bool", "string", "struct", "unit", "macro":
		return true
	case "unary":
		return c.fixedCastSource(e.Right)
	case "binary":
		if e.Op == "==" || e.Op == "!=" || e.Op == "<" || e.Op == "<=" || e.Op == ">" || e.Op == ">=" || e.Op == "&&" || e.Op == "||" {
			return true
		}
		if e.Op == "<<" || e.Op == ">>" {
			return c.fixedCastSource(e.Left)
		}
		return c.fixedCastSource(e.Left) || c.fixedCastSource(e.Right)
	case "if":
		return c.fixedCastBlock(e.Body) || c.fixedCastBlock(e.Else)
	case "block":
		return c.fixedCastBlock(e.Body)
	}
	return false
}

func (c *checker) fixedCastBlock(block *syntax.Block) bool {
	if block == nil {
		return false
	}
	c.scopes = append(c.scopes, map[string]binding{})
	defer func() { c.scopes = c.scopes[:len(c.scopes)-1] }()
	for _, stmt := range block.Statements {
		if stmt.Kind != "let" {
			continue
		}
		typ := stmt.Type
		if typ == "" {
			typ = c.hint(stmt.Expr)
		}
		c.scopes[len(c.scopes)-1][stmt.Name] = binding{typ: typ}
	}
	return c.fixedCastSource(block.Tail)
}
