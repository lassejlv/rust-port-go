package compiler

import "github.com/lassejlv/rust-port-go/internal/syntax"

func (c *checker) forStatement(s *syntax.Stmt) (bool, error) {
	if s.Name == "_" && s.Mutable {
		return false, fail(s.Span, "mut wildcard patterns are unsupported")
	}
	if s.Expr == nil {
		return false, fail(s.Span, "for loop requires an iterable")
	}
	item := ""
	if s.Expr.Kind == "range" {
		rangeExpr := s.Expr
		if rangeExpr.Left == nil || rangeExpr.Right == nil || (rangeExpr.Op != ".." && rangeExpr.Op != "..=") {
			return false, fail(rangeExpr.Span, "for loops require a bounded integer range")
		}
		context := ""
		if c.fixedCastSource(rangeExpr.Left) {
			context = c.hint(rangeExpr.Left)
		}
		if context == "" && c.fixedCastSource(rangeExpr.Right) {
			context = c.hint(rangeExpr.Right)
		}
		if context == "" {
			context = c.hint(rangeExpr.Left)
			if context == "" {
				context = c.hint(rangeExpr.Right)
			}
		}
		if context == "" {
			context = "i32"
		}
		if !integerType(context) {
			return false, fail(rangeExpr.Span, "range bounds must be supported integers")
		}
		left, err := c.expr(rangeExpr.Left, context)
		if err != nil {
			return false, err
		}
		if _, err := c.expr(rangeExpr.Right, left); err != nil {
			return false, err
		}
		if c.bottom[rangeExpr.Left] || c.bottom[rangeExpr.Right] {
			return false, fail(rangeExpr.Span, "value-less divergent range bounds are unsupported")
		}
		item = left
		c.types[rangeExpr] = item
		c.diverges[rangeExpr] = c.diverges[rangeExpr.Left] || c.diverges[rangeExpr.Right]
	} else {
		typ, err := c.expr(s.Expr, "")
		if err != nil {
			return false, err
		}
		array, ok := arrayInfo(typ)
		if !ok {
			return false, fail(s.Expr.Span, "for loop iterable must be a bounded integer range or owned Copy array")
		}
		item = array.Element
		if !c.copyElement(item) {
			return false, fail(s.Expr.Span, "unsupported for-loop array element %s", item)
		}
		if err := c.visibleType(item, s.Expr.Span); err != nil {
			return false, err
		}
	}
	c.forTypes[s] = item
	name := c.newVariable()
	c.decls[s] = name
	c.scopes = append(c.scopes, map[string]binding{})
	defer func() { c.scopes = c.scopes[:len(c.scopes)-1] }()
	if s.Name != "_" {
		c.scopes[len(c.scopes)-1][s.Name] = binding{typ: item, cname: name, mutable: s.Mutable}
	}
	c.loopDepth++
	_, err := c.block(s.Body, "()", false)
	c.loopDepth--
	if err != nil {
		return false, err
	}
	// A for statement retains unit static typing even when evaluating its
	// header always exits. Keep header divergence for lowering, while treating
	// the statement as potentially falling through for function-result checks.
	return true, nil
}
