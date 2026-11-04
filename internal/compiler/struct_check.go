package compiler

import (
	"strings"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

func structType(typ string) bool { return strings.HasPrefix(typ, "struct:") }

func itemModule(canonical string) string {
	canonical = strings.TrimPrefix(canonical, "struct:")
	if index := strings.LastIndex(canonical, "::"); index >= 0 {
		return canonical[:index]
	}
	return ""
}

func moduleContains(owner, current string) bool {
	// An empty owner denotes the selected crate's root, not the root of
	// another crate flattened under an inaccessible @crateN prefix.
	if owner == "" {
		return !strings.HasPrefix(current, "@")
	}
	return current == owner || strings.HasPrefix(current, owner+"::")
}

func (c *checker) supportedType(typ string) bool {
	return supportedType(typ) || c.structs[typ] != nil
}

func (c *checker) collectStructs() error {
	for _, s := range c.program.Structs {
		key := "struct:" + s.Name
		if c.structs[key] != nil {
			return fail(s.Span, "duplicate struct %q", s.Name)
		}
		if !s.Copy {
			return fail(s.Span, "struct %q requires #[derive(Copy, Clone)]; move-only structs are unsupported", s.Name)
		}
		c.structs[key] = s
	}
	for _, s := range c.program.Structs {
		names := make(map[string]bool)
		for _, field := range s.Fields {
			if names[field.Name] {
				return fail(field.Span, "duplicate field %q in struct %q", field.Name, s.Name)
			}
			names[field.Name] = true
			if field.Type == "&str" {
				return fail(field.Span, "&str fields require explicit lifetimes and are unsupported")
			}
			if field.Type == "()" || !c.supportedType(field.Type) {
				return fail(field.Span, "unsupported Copy field type %q", field.Type)
			}
		}
	}
	state := make(map[string]int)
	var visit func(string) error
	visit = func(key string) error {
		if state[key] == 2 {
			return nil
		}
		s := c.structs[key]
		if state[key] == 1 {
			return fail(s.Span, "recursive by-value struct layout involving %q", s.Name)
		}
		state[key] = 1
		for _, field := range s.Fields {
			if structType(field.Type) {
				if err := visit(field.Type); err != nil {
					return err
				}
			}
		}
		state[key] = 2
		c.structOrder = append(c.structOrder, key)
		return nil
	}
	for _, s := range c.program.Structs {
		if err := visit("struct:" + s.Name); err != nil {
			return err
		}
	}
	return nil
}

func (c *checker) visibleType(typ string, span syntax.Span) error {
	s := c.structs[typ]
	if s != nil && !s.Public && !moduleContains(itemModule(s.Name), c.currentModule) {
		return fail(span, "type %q is private", s.Name)
	}
	return nil
}

func (c *checker) field(typ, name string, span syntax.Span) (*syntax.StructField, error) {
	s := c.structs[typ]
	if s == nil {
		return nil, fail(span, "type %s has no named fields", typ)
	}
	if err := c.visibleType(typ, span); err != nil {
		return nil, err
	}
	for i := range s.Fields {
		field := &s.Fields[i]
		if field.Name != name {
			continue
		}
		if !field.Public && !moduleContains(itemModule(s.Name), c.currentModule) {
			return nil, fail(span, "field %q of struct %q is private", name, s.Name)
		}
		if err := c.visibleType(field.Type, span); err != nil {
			return nil, err
		}
		return field, nil
	}
	return nil, fail(span, "unknown field %q on struct %q", name, s.Name)
}

func (c *checker) structLiteral(e *syntax.Expr) (string, error) {
	typ := e.Target
	s := c.structs[typ]
	if s == nil {
		return "", fail(e.Span, "unknown struct type %q", e.Text)
	}
	if err := c.visibleType(typ, e.Span); err != nil {
		return "", err
	}
	seen := make(map[string]bool)
	for _, value := range e.Fields {
		if seen[value.Name] {
			return "", fail(value.Span, "duplicate field %q in struct literal", value.Name)
		}
		seen[value.Name] = true
		field, err := c.field(typ, value.Name, value.Span)
		if err != nil {
			return "", err
		}
		if _, err := c.expr(value.Value, field.Type); err != nil {
			return "", err
		}
		c.diverges[e] = c.diverges[e] || c.diverges[value.Value]
	}
	for _, field := range s.Fields {
		if !seen[field.Name] {
			return "", fail(e.Span, "missing field %q in struct %q", field.Name, s.Name)
		}
	}
	return typ, nil
}

func (c *checker) fieldExpression(e *syntax.Expr) (string, error) {
	receiver, err := c.expr(e.Left, "")
	if err != nil {
		return "", err
	}
	field, err := c.field(receiver, e.Text, e.Span)
	if err != nil {
		return "", err
	}
	c.diverges[e] = c.diverges[e.Left]
	return field.Type, nil
}

// place validates a field assignment without turning its root into a copied
// value. The backend uses checked.names on that root to write the original.
func (c *checker) place(e *syntax.Expr) (string, binding, error) {
	if e == nil {
		return "", binding{}, fail(syntax.Span{Line: 1, Column: 1}, "missing assignment place")
	}
	if e.Kind == "name" {
		root, ok := c.lookup(e.Text)
		if !ok {
			return "", binding{}, fail(e.Span, "unknown variable %q", e.Text)
		}
		if err := c.visibleType(root.typ, e.Span); err != nil {
			return "", binding{}, err
		}
		c.names[e], c.types[e] = root.cname, root.typ
		return root.typ, root, nil
	}
	if e.Kind != "field" {
		return "", binding{}, fail(e.Span, "field assignment must be rooted in a mutable local variable")
	}
	typ, root, err := c.place(e.Left)
	if err != nil {
		return "", binding{}, err
	}
	field, err := c.field(typ, e.Text, e.Span)
	if err != nil {
		return "", binding{}, err
	}
	c.types[e] = field.Type
	return field.Type, root, nil
}

func (c *checker) fieldAssignment(s *syntax.Stmt) (bool, error) {
	typ, root, err := c.place(s.Target)
	if err != nil {
		return false, err
	}
	if !root.mutable {
		return false, fail(s.Span, "field assignment requires a mutable local variable")
	}
	if s.Op != "=" && s.Op != "+=" && s.Op != "-=" && s.Op != "*=" && s.Op != "/=" && s.Op != "%=" {
		return false, fail(s.Span, "unsupported assignment operator %q", s.Op)
	}
	if s.Op != "=" && !integerType(typ) {
		return false, fail(s.Span, "compound assignment requires an integer field")
	}
	if _, err := c.expr(s.Expr, typ); err != nil {
		return false, err
	}
	return !c.diverges[s.Expr], nil
}
