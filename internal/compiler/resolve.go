package compiler

import (
	"sort"
	"strings"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

// moduleScope uses one bounded item namespace. Rust's separate type/value
// namespaces are not yet fully modeled; shared item names reject explicitly.
type moduleScope struct {
	parent  *moduleScope
	path    string
	program *syntax.Program
	items   map[string]*moduleItem
	crate   *moduleScope
	externs map[string]*moduleScope
	public  bool
}

type moduleItem struct {
	name       string
	owner      *moduleScope
	public     bool
	span       syntax.Span
	module     *moduleScope
	function   *syntax.Function
	structure  *syntax.Struct
	importDecl *syntax.Import
	canonical  string
	state      int
	target     *moduleItem
}

type moduleResolver struct {
	scopes      []*moduleScope
	active      map[*syntax.Program]bool
	types       map[string]*moduleItem
	crateRoots  map[*syntax.Program]*moduleScope
	crateActive map[*syntax.Program]bool
	identities  map[string]*syntax.Program
}

// resolveModules flattens module items while keeping source call spelling in
// Expr.Text. Expr.Target carries the canonical callee, so an imported function
// never bypasses lexical variable shadowing in the type checker.
func resolveModules(program *syntax.Program) (*syntax.Program, error) {
	return resolveModulesMode(program, true)
}

func resolveModulesMode(program *syntax.Program, requireMain bool) (*syntax.Program, error) {
	r := &moduleResolver{
		active: make(map[*syntax.Program]bool), types: make(map[string]*moduleItem),
		crateRoots: make(map[*syntax.Program]*moduleScope), crateActive: make(map[*syntax.Program]bool),
		identities: make(map[string]*syntax.Program),
	}
	root, err := r.collectCrate(program)
	if err != nil {
		return nil, err
	}
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
	functions := make(map[string]*syntax.Function)
	for _, scope := range r.scopes {
		for _, declaration := range scope.program.Structs {
			copyStruct := *declaration
			copyStruct.Name = qualified(scope.path, declaration.Name)
			copyStruct.Fields = append([]syntax.StructField(nil), declaration.Fields...)
			for i := range copyStruct.Fields {
				field := &copyStruct.Fields[i]
				field.Type, err = r.resolveType(scope, field.Type, field.Span)
				if err != nil {
					return nil, err
				}
			}
			flat.Structs = append(flat.Structs, &copyStruct)
		}
		for _, f := range scope.program.Functions {
			copyFunction := *f
			copyFunction.Name = qualified(scope.path, f.Name)
			copyFunction.Params = append([]syntax.Param(nil), f.Params...)
			for i := range copyFunction.Params {
				param := &copyFunction.Params[i]
				param.Type, err = r.resolveType(scope, param.Type, param.Span)
				if err != nil {
					return nil, err
				}
			}
			copyFunction.ReturnType, err = r.resolveType(scope, f.ReturnType, f.Span)
			if err != nil {
				return nil, err
			}
			if err := r.rewriteBlock(scope, f.Body); err != nil {
				return nil, err
			}
			flat.Functions = append(flat.Functions, &copyFunction)
			functions[copyFunction.Name] = &copyFunction
		}
	}
	if item := root.items["main"]; requireMain && item != nil && item.importDecl != nil && item.target != nil && item.target.function != nil {
		// Rust permits the crate entrypoint to be a function brought into scope
		// by use. A fresh wrapper preserves the resolved target's identity and
		// avoids checking/emitting the same body under two function identities.
		target := functions[item.target.canonical]
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

// collectCrate interns a dependency Program once, so a diamond or multiple
// extern aliases preserve one function/record identity instead of flattening
// copies. Crate roots deliberately have no parent: super and private visibility
// can never cross the dependency boundary.
func (r *moduleResolver) collectCrate(program *syntax.Program) (*moduleScope, error) {
	if program == nil {
		return nil, fail(syntax.Span{Line: 1, Column: 1}, "missing crate syntax")
	}
	if r.crateActive[program] {
		return nil, fail(syntax.Span{Line: 1, Column: 1}, "cyclic crate dependency graph")
	}
	if root := r.crateRoots[program]; root != nil {
		return root, nil
	}
	if prior := r.identities[program.CrateID]; prior != nil && prior != program {
		return nil, fail(syntax.Span{Line: 1, Column: 1}, "conflicting crate identity %q", program.CrateID)
	}
	r.identities[program.CrateID] = program
	r.crateActive[program] = true
	defer delete(r.crateActive, program)
	root, err := r.collect(program, nil, program.CrateID)
	if err != nil {
		return nil, err
	}
	r.crateRoots[program] = root
	names := make([]string, 0, len(program.Externs))
	for name := range program.Externs {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if name == "" || name == "crate" || name == "self" || name == "super" || strings.Contains(name, "::") {
			return nil, fail(syntax.Span{Line: 1, Column: 1}, "invalid extern crate name %q", name)
		}
		external, err := r.collectCrate(program.Externs[name])
		if err != nil {
			return nil, err
		}
		root.externs[name] = external
	}
	return root, nil
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
	if parent == nil {
		scope.crate = scope
		scope.externs = make(map[string]*moduleScope)
	} else {
		scope.crate = parent.crate
	}
	r.scopes = append(r.scopes, scope)
	add := func(item *moduleItem) error {
		if prior, exists := scope.items[item.name]; exists {
			if item.function != nil && prior.function != nil {
				return fail(item.span, "duplicate function %q", item.name)
			}
			if item.structure != nil && prior.structure != nil {
				return fail(item.span, "duplicate structure %q", item.name)
			}
			return fail(item.span, "conflicting item %q (shared function/module/import/structure names are unsupported)", item.name)
		}
		scope.items[item.name] = item
		return nil
	}
	for _, f := range program.Functions {
		if err := add(&moduleItem{name: f.Name, owner: scope, public: f.Public, span: f.Span, function: f, canonical: qualified(path, f.Name)}); err != nil {
			return nil, err
		}
	}
	for _, structure := range program.Structs {
		if primitiveTypeName(structure.Name) {
			return nil, fail(structure.Span, "shadowing primitive type %q with a structure is unsupported", structure.Name)
		}
		item := &moduleItem{name: structure.Name, owner: scope, public: structure.Public, span: structure.Span, structure: structure, canonical: qualified(path, structure.Name)}
		if err := add(item); err != nil {
			return nil, err
		}
		r.types[item.canonical] = item
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
		child.public = m.Public
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
	if target.structure != nil && primitiveTypeName(item.name) {
		return nil, fail(item.span, "shadowing primitive type %q with a structure alias is unsupported", item.name)
	}
	item.target, item.state = target, 2
	return target, nil
}

func (r *moduleResolver) path(requester *moduleScope, path string, span syntax.Span) (*moduleItem, error) {
	parts := strings.Split(path, "::")
	scope := requester
	explicitRoot := false
	if len(parts) > 0 {
		switch parts[0] {
		case "crate":
			explicitRoot = true
			scope = requester.crate
			parts = parts[1:]
		case "self":
			explicitRoot = true
			parts = parts[1:]
		case "super":
			explicitRoot = true
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
		if !exists && i == 0 && !explicitRoot {
			if external := requester.crate.externs[part]; external != nil {
				item = &moduleItem{name: part, owner: requester.crate, public: true, module: external}
				exists = true
			}
		}
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
		var err error
		s.Type, err = r.resolveType(scope, s.Type, s.Span)
		if err != nil {
			return err
		}
		if err := r.rewriteExpr(scope, s.Target); err != nil {
			return err
		}
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
	if e.Kind == "struct" {
		item, err := r.path(scope, e.Text, e.Span)
		if err != nil {
			return err
		}
		if item.structure == nil {
			return fail(e.Span, "%q is not a structure type", e.Text)
		}
		e.Target = "struct:" + item.canonical
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
				if item.structure != nil {
					return fail(e.Span, "cannot call structure %q as a function; tuple constructors are unsupported", e.Text)
				}
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
	for i := range e.Fields {
		if err := r.rewriteExpr(scope, e.Fields[i].Value); err != nil {
			return err
		}
	}
	if err := r.rewriteBlock(scope, e.Body); err != nil {
		return err
	}
	return r.rewriteBlock(scope, e.Else)
}

// resolveType accepts primitive spellings and resolves named record types in the
// declaration's lexical module. Canonical local annotations are revalidated so
// resolving the same parsed syntax tree again remains stable and private paths
// never bypass visibility just because a previous pass populated their key.
func (r *moduleResolver) resolveType(scope *moduleScope, name string, span syntax.Span) (string, error) {
	switch name {
	case "", "i32", "i64", "bool", "&str", "()":
		return name, nil
	}
	if strings.HasPrefix(name, "struct:") {
		canonical := strings.TrimPrefix(name, "struct:")
		item := r.types[canonical]
		if item == nil {
			return "", fail(span, "unknown structure type %q", canonical)
		}
		if !item.public && !descendant(scope, item.owner) {
			return "", fail(span, "type %q is private", canonical)
		}
		for owner := item.owner; owner.parent != nil; owner = owner.parent {
			if !owner.public && !descendant(scope, owner.parent) {
				return "", fail(span, "module containing type %q is private", canonical)
			}
		}
		return name, nil
	}
	item, err := r.path(scope, name, span)
	if err != nil {
		return "", err
	}
	if item.structure == nil {
		return "", fail(span, "%q is not a structure type", name)
	}
	return "struct:" + item.canonical, nil
}

func primitiveTypeName(name string) bool {
	switch name {
	case "bool", "str", "char", "i8", "i16", "i32", "i64", "i128", "isize", "u8", "u16", "u32", "u64", "u128", "usize", "f32", "f64":
		return true
	}
	return false
}
