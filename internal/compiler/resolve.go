package compiler

import (
	"strings"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

// moduleScope uses one bounded item namespace. Rust's simultaneous function and
// module names require separate namespaces and are rejected by this bootstrap.
type moduleScope struct {
	parent  *moduleScope
	path    string
	program *syntax.Program
	items   map[string]*moduleItem
}

type moduleItem struct {
	name       string
	owner      *moduleScope
	public     bool
	span       syntax.Span
	module     *moduleScope
	function   *syntax.Function
	importDecl *syntax.Import
	canonical  string
	state      int
	target     *moduleItem
}

type moduleResolver struct {
	root   *moduleScope
	scopes []*moduleScope
	active map[*syntax.Program]bool
}

// resolveModules flattens module items while keeping source call spelling in
// Expr.Text. Expr.Target carries the canonical callee, so an imported function
// never bypasses lexical variable shadowing in the type checker.
func resolveModules(program *syntax.Program) (*syntax.Program, error) {
	r := &moduleResolver{active: make(map[*syntax.Program]bool)}
	root, err := r.collect(program, nil, "")
	if err != nil {
		return nil, err
	}
	r.root = root
	for _, scope := range r.scopes {
		for i := range scope.program.Imports {
			decl := &scope.program.Imports[i]
			item := scope.items[importName(decl)]
			if _, err := r.resolveImport(item); err != nil {
				return nil, err
			}
		}
	}
	flat := &syntax.Program{}
	for _, scope := range r.scopes {
		for _, f := range scope.program.Functions {
			copyFunction := *f
			copyFunction.Name = qualified(scope.path, f.Name)
			if err := r.rewriteBlock(scope, f.Body); err != nil {
				return nil, err
			}
			flat.Functions = append(flat.Functions, &copyFunction)
		}
	}
	if item := root.items["main"]; item != nil && item.importDecl != nil && item.target != nil && item.target.function != nil {
		// Rust permits the crate entrypoint to be a function brought into scope
		// by use. A fresh wrapper preserves the resolved target's identity and
		// avoids checking/emitting the same body under two function identities.
		target := item.target.function
		entry := &syntax.Function{Name: "main", Params: append([]syntax.Param(nil), target.Params...), ReturnType: target.ReturnType, Span: item.span}
		call := &syntax.Expr{Kind: "call", Text: "main", Target: item.target.canonical, Span: item.span}
		for _, param := range target.Params {
			call.Args = append(call.Args, &syntax.Expr{Kind: "name", Text: param.Name, Span: param.Span})
		}
		entry.Body = &syntax.Block{Tail: call, Span: item.span}
		flat.Functions = append(flat.Functions, entry)
	}
	return flat, nil
}

func qualified(module, name string) string {
	if module == "" {
		return name
	}
	return module + "::" + name
}

func importName(decl *syntax.Import) string {
	if decl.Alias != "" {
		return decl.Alias
	}
	parts := strings.Split(decl.Path, "::")
	return parts[len(parts)-1]
}

func (r *moduleResolver) collect(program *syntax.Program, parent *moduleScope, path string) (*moduleScope, error) {
	if program == nil {
		return nil, fail(syntax.Span{Line: 1, Column: 1}, "missing module syntax")
	}
	if r.active[program] {
		return nil, fail(syntax.Span{Line: 1, Column: 1}, "cyclic module syntax")
	}
	r.active[program] = true
	defer delete(r.active, program)
	scope := &moduleScope{parent: parent, path: path, program: program, items: make(map[string]*moduleItem)}
	r.scopes = append(r.scopes, scope)
	add := func(item *moduleItem) error {
		if prior, exists := scope.items[item.name]; exists {
			if item.function != nil && prior.function != nil {
				return fail(item.span, "duplicate function %q", item.name)
			}
			return fail(item.span, "conflicting item %q (shared function/module/import names are unsupported)", item.name)
		}
		scope.items[item.name] = item
		return nil
	}
	for _, f := range program.Functions {
		if err := add(&moduleItem{name: f.Name, owner: scope, public: f.Public, span: f.Span, function: f, canonical: qualified(path, f.Name)}); err != nil {
			return nil, err
		}
	}
	for _, m := range program.Modules {
		if m.Program == nil {
			return nil, fail(m.Span, "external module %q requires the source-file API and an existing module file", m.Name)
		}
		item := &moduleItem{name: m.Name, owner: scope, public: m.Public, span: m.Span}
		if err := add(item); err != nil {
			return nil, err
		}
		child, err := r.collect(m.Program, scope, qualified(path, m.Name))
		if err != nil {
			return nil, err
		}
		item.module = child
	}
	for i := range program.Imports {
		decl := &program.Imports[i]
		if decl.Public {
			return nil, fail(decl.Span, "public use reexports are unsupported")
		}
		name := importName(decl)
		if name == "crate" || name == "self" || name == "super" || name == "" {
			return nil, fail(decl.Span, "use path requires an item name or explicit alias")
		}
		if err := add(&moduleItem{name: name, owner: scope, span: decl.Span, importDecl: decl}); err != nil {
			return nil, err
		}
	}
	return scope, nil
}

func descendant(scope, ancestor *moduleScope) bool {
	for scope != nil {
		if scope == ancestor {
			return true
		}
		scope = scope.parent
	}
	return false
}

func (r *moduleResolver) resolveImport(item *moduleItem) (*moduleItem, error) {
	if item.importDecl == nil {
		return item, nil
	}
	if item.state == 2 {
		return item.target, nil
	}
	if item.state == 1 {
		return nil, fail(item.span, "cyclic use import involving %q", item.name)
	}
	item.state = 1
	target, err := r.path(item.owner, item.importDecl.Path, item.span)
	if err != nil {
		return nil, err
	}
	item.target, item.state = target, 2
	return target, nil
}

func (r *moduleResolver) path(requester *moduleScope, path string, span syntax.Span) (*moduleItem, error) {
	parts := strings.Split(path, "::")
	scope := requester
	if len(parts) > 0 {
		switch parts[0] {
		case "crate":
			scope = r.root
			parts = parts[1:]
		case "self":
			parts = parts[1:]
		case "super":
			for len(parts) > 0 && parts[0] == "super" {
				if scope.parent == nil {
					return nil, fail(span, "super path escapes the crate root")
				}
				scope = scope.parent
				parts = parts[1:]
			}
		}
	}
	if len(parts) == 0 {
		// Imports may alias an explicitly named module root, but calling a root
		// directly is rejected by the call resolver below.
		return &moduleItem{module: scope, owner: scope, public: true}, nil
	}
	var item *moduleItem
	for i, part := range parts {
		if part == "" || part == "crate" || part == "self" || part == "super" {
			return nil, fail(span, "invalid module path %q", path)
		}
		var exists bool
		item, exists = scope.items[part]
		if !exists {
			return nil, fail(span, "unresolved module path %q", path)
		}
		if !item.public && !descendant(requester, item.owner) {
			return nil, fail(span, "item %q is private in module path %q", part, path)
		}
		var err error
		item, err = r.resolveImport(item)
		if err != nil {
			return nil, err
		}
		if i < len(parts)-1 {
			if item.module == nil {
				return nil, fail(span, "%q is not a module in path %q", part, path)
			}
			scope = item.module
		}
	}
	return item, nil
}

func (r *moduleResolver) rewriteBlock(scope *moduleScope, b *syntax.Block) error {
	if b == nil {
		return nil
	}
	for i := range b.Statements {
		s := &b.Statements[i]
		if err := r.rewriteExpr(scope, s.Expr); err != nil {
			return err
		}
		if err := r.rewriteBlock(scope, s.Body); err != nil {
			return err
		}
		if err := r.rewriteBlock(scope, s.Else); err != nil {
			return err
		}
	}
	return r.rewriteExpr(scope, b.Tail)
}

func (r *moduleResolver) rewriteExpr(scope *moduleScope, e *syntax.Expr) error {
	if e == nil {
		return nil
	}
	if e.Kind == "call" {
		// A missing bare item may be a lexical variable; the type checker owns
		// that diagnostic. A qualified fallback prevents accidental lookup of
		// a same-named function in the crate root from a child module.
		_, bareItem := scope.items[e.Text]
		if !strings.Contains(e.Text, "::") && !bareItem {
			e.Target = qualified(scope.path, e.Text)
		} else {
			item, err := r.path(scope, e.Text, e.Span)
			if err != nil {
				return err
			}
			if item.function == nil {
				return fail(e.Span, "cannot call module %q as a function", e.Text)
			}
			e.Target = item.canonical
		}
	}
	if err := r.rewriteExpr(scope, e.Left); err != nil {
		return err
	}
	if err := r.rewriteExpr(scope, e.Right); err != nil {
		return err
	}
	for _, arg := range e.Args {
		if err := r.rewriteExpr(scope, arg); err != nil {
			return err
		}
	}
	if err := r.rewriteBlock(scope, e.Body); err != nil {
		return err
	}
	return r.rewriteBlock(scope, e.Else)
}
