package compiler

import (
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

func TestResolveModulesAccepted(t *testing.T) {
	cases := []struct{ name, source string }{
		{"qualified child", `mod math { pub fn value()->i32 { 42 } } fn main(){println!("{}", math::value());}`},
		{"canonical duplicate basenames", `mod a {pub fn value()->i32{1}} mod b {pub fn value()->i32{2}} fn main(){println!("{} {}",a::value(),b::value());}`},
		{"ancestor private", `mod outer { fn value()->i32{3} mod inner {pub fn call()->i32{super::value()}} pub fn call()->i32{inner::call()} } fn main(){println!("{}",outer::call());}`},
		{"sibling root private module", `mod a {pub fn value()->i32{4}} mod b {pub fn call()->i32{crate::a::value()}} fn main(){println!("{}",b::call());}`},
		{"self and repeated super", `fn value()->i32{5} mod a {pub mod b {pub fn call()->i32{super::super::value()} pub fn again()->i32{self::call()}}} fn main(){println!("{}",a::b::again());}`},
		{"function alias", `use crate::math::value as answer; mod math{pub fn value()->i32{6}} fn main(){println!("{}",answer());}`},
		{"module alias and variable namespaces", `mod math{pub fn value()->i32{7}} use crate::math as numbers; fn main(){let numbers=1;println!("{} {}",numbers,numbers::value());}`},
		{"import chains", `mod a{pub fn value()->i32{8}} use crate::a::value as first; use self::first as second; fn main(){println!("{}",second());}`},
		{"relative child import", `mod a{mod b{pub fn value()->i32{9}} use b::value; pub fn call()->i32{value()}} fn main(){println!("{}",a::call());}`},
		{"descendant uses private alias", `mod a{use crate::value as alias;mod b{use super::alias;pub fn call()->i32{alias()}}pub fn call()->i32{b::call()}} fn value()->i32{10} fn main(){println!("{}",a::call());}`},
		{"module root aliases", `use crate as root; mod a{use super as parent; pub fn call()->i32{parent::value()}} fn value()->i32{11} fn main(){println!("{}",root::a::call());}`},
		{"imported recursion", `mod a{use self::fib as recurse;pub fn fib(n:i32)->i32{if n<2{n}else{recurse(n-1)+recurse(n-2)}}} fn main(){println!("{}",a::fib(8));}`},
		{"imported main", `mod a{pub fn run(){println!("main");}}use a::run as main;`},
		{"imported main chain", `fn run(){println!("main");}use self::run as first;use first as main;`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := Check(tt.source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestResolveModulesRejectsInvalidPaths(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"private function", `mod a{fn value()->i32{1}} fn main(){a::value();}`, "private"},
		{"private nested module", `mod a{mod b{pub fn value()->i32{1}}} fn main(){a::b::value();}`, "private"},
		{"private sibling function", `mod a{fn value()->i32{1}} mod b{pub fn call()->i32{crate::a::value()}} fn main(){b::call();}`, "private"},
		{"unknown import", `use crate::absent::value;fn main(){}`, "unresolved module path"},
		{"unknown qualified call", `fn main(){absent::value();}`, "unresolved module path"},
		{"root escape", `fn main(){super::value();}`, "escapes the crate root"},
		{"import cycle", `use crate::second as first;use crate::first as second;fn main(){}`, "cyclic use"},
		{"self import cycle", `use self::a;fn main(){}`, "cyclic use"},
		{"duplicate module", `mod a{} mod a{} fn main(){}`, "conflicting item"},
		{"duplicate function", `mod a{fn value(){} fn value(){}} fn main(){}`, "duplicate function"},
		{"conflicting alias", `mod a{pub fn value(){}}use a::value as x;use a::value as x;fn main(){}`, "conflicting item"},
		{"public reexport", `mod a{pub fn value(){}}pub use a::value;fn main(){}`, "public use reexports"},
		{"call module", `mod a{} fn main(){a();}`, "cannot call module"},
		{"function in module path", `fn f(){} fn main(){f::g();}`, "not a module"},
		{"external source string", `mod a;fn main(){}`, "requires the source-file API"},
		{"no implicit root functions", `fn value()->i32{1} mod a{pub fn call()->i32{value()}}fn main(){a::call();}`, "unknown function"},
		{"no implicit parent imports", `mod a{use crate::value;mod b{pub fn call()->i32{value()}}pub fn call()->i32{b::call()}}fn value()->i32{1}fn main(){a::call();}`, "unknown function"},
		{"private import alias", `mod a{use crate::value as alias;}fn value()->i32{1}fn main(){a::alias();}`, "private"},
		{"shadowed imported function", `mod a{pub fn value()->i32{1}}use a::value as alias;fn main(){let alias=1;alias();}`, "variable and cannot be called"},
		{"shadowed local function", `mod a{fn value()->i32{1}pub fn call(){let value=2;value();}}fn main(){a::call();}`, "variable and cannot be called"},
		{"imported main argument", `mod a{pub fn run(x:i32){}}use a::run as main;`, "main must have no parameters"},
		{"imported main return", `mod a{pub fn run()->i32{1}}use a::run as main;`, "main must have no parameters and return ()"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := Check(tt.source)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestResolveModulesKeepsSourceCalleeAndOriginalFunctionNames(t *testing.T) {
	program, err := syntax.Parse(`mod a{pub fn value()->i32{1}}use a::value as alias;fn main(){alias();}`)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		flat, err := resolveModules(program)
		if err != nil {
			t.Fatal(err)
		}
		if len(flat.Functions) != 2 || flat.Functions[0].Name != "main" || flat.Functions[1].Name != "a::value" {
			t.Fatalf("unexpected flattened functions: %#v", flat.Functions)
		}
		call := flat.Functions[0].Body.Statements[0].Expr
		if call.Text != "alias" || call.Target != "a::value" {
			t.Fatalf("callee source %q target %q", call.Text, call.Target)
		}
		if program.Modules[0].Program.Functions[0].Name != "value" {
			t.Fatal("resolver changed source function name")
		}
	}
}

func TestResolveModulesRetainsFilenameInErrors(t *testing.T) {
	program, err := syntax.Parse("mod a{fn value(){}}\nfn main(){a::value();}")
	if err != nil {
		t.Fatal(err)
	}
	stampProgram(program, "src/main.rs")
	_, err = analyzeProgram(program)
	if err == nil || !strings.Contains(err.Error(), "src/main.rs:2:") {
		t.Fatalf("filename/line missing: %v", err)
	}
}

func TestResolveModulesRejectsCyclicSyntax(t *testing.T) {
	program := &syntax.Program{}
	program.Modules = []*syntax.Module{{Name: "cycle", Program: program, Span: syntax.Span{Line: 1, Column: 1}}}
	if _, err := resolveModules(program); err == nil || !strings.Contains(err.Error(), "cyclic module syntax") {
		t.Fatalf("got %v", err)
	}
}

func TestImportedMainHasIndependentWrapperBody(t *testing.T) {
	program, err := syntax.Parse(`mod a{pub fn run(){println!("main");}}use a::run as main;`)
	if err != nil {
		t.Fatal(err)
	}
	flat, err := resolveModules(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(flat.Functions) != 2 {
		t.Fatalf("expected target and wrapper, got %d functions", len(flat.Functions))
	}
	target, entry := flat.Functions[0], flat.Functions[1]
	if target.Name != "a::run" || entry.Name != "main" || target.Body == entry.Body {
		t.Fatal("entrypoint reused target function body")
	}
	if entry.Body.Tail == nil || entry.Body.Tail.Target != "a::run" {
		t.Fatal("entrypoint wrapper does not call canonical target")
	}
	result := runNative(t, `mod a{pub fn run(){println!("main");}}use a::run as main;`)
	if result.status != 0 || result.stdout != "main\n" || result.stderr != "" {
		t.Fatalf("native entrypoint result: %#v", result)
	}
}

func TestResolveStructureTypesAcrossModulesAndAliases(t *testing.T) {
	source := `
mod geometry {
 #[derive(Copy, Clone)] pub struct Point {pub x:i32}
 pub fn identity(p:Point)->Point{p}
}
use geometry::Point as P;
use geometry as geo;
fn pass(p:P)->geo::Point {let q:P=p; q}
fn main(){let p:P=P{x:4}; println!("{}",pass(p).x);}
`
	program, err := syntax.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		flat, err := resolveModules(program)
		if err != nil {
			t.Fatal(err)
		}
		if len(flat.Structs) != 1 || flat.Structs[0].Name != "geometry::Point" {
			t.Fatalf("structures=%#v", flat.Structs)
		}
		pass := flat.Functions[0]
		if pass.Name != "pass" || pass.Params[0].Type != "struct:geometry::Point" || pass.ReturnType != "struct:geometry::Point" {
			t.Fatalf("resolved signature=%#v", pass)
		}
		if pass.Body.Statements[0].Type != "struct:geometry::Point" {
			t.Fatal("let type not canonical")
		}
		main := flat.Functions[1]
		constructor := main.Body.Statements[0].Expr
		if constructor.Text != "P" || constructor.Target != "struct:geometry::Point" {
			t.Fatalf("constructor source=%q target=%q", constructor.Text, constructor.Target)
		}
		if program.Functions[0].Params[0].Type != "P" || program.Functions[0].ReturnType != "geo::Point" {
			t.Fatal("resolver mutated source function signature")
		}
		if program.Modules[0].Program.Structs[0].Name != "Point" {
			t.Fatal("resolver mutated source structure name")
		}
	}
}

func TestResolveStructureFieldTypesAndExpressions(t *testing.T) {
	source := `
#[derive(Clone,Copy)] struct Outer {inner:m::Inner}
mod m {#[derive(Copy,Clone)] pub struct Inner {pub value:i64}}
fn main(){let mut p=Outer{inner:m::Inner{value:4}}; p.inner.value += 2; println!("{}",p.inner.value);}
`
	program, err := syntax.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	flat, err := resolveModules(program)
	if err != nil {
		t.Fatal(err)
	}
	if len(flat.Structs) != 2 || flat.Structs[0].Fields[0].Type != "struct:m::Inner" {
		t.Fatalf("wrong field type: %#v", flat.Structs)
	}
	if program.Structs[0].Fields[0].Type != "m::Inner" {
		t.Fatal("resolver mutated source structure fields")
	}
	outer := flat.Functions[0].Body.Statements[0].Expr
	if outer.Target != "struct:Outer" || outer.Fields[0].Value.Target != "struct:m::Inner" {
		t.Fatal("nested constructor was not resolved")
	}
	place := flat.Functions[0].Body.Statements[1].Target
	if place == nil || place.Kind != "field" || place.Left.Kind != "field" || place.Left.Left.Text != "p" {
		t.Fatalf("field place=%#v", place)
	}
}

func TestResolveStructureAcceptedPaths(t *testing.T) {
	cases := []string{
		`mod a{#[derive(Copy,Clone)]pub struct S{pub x:i32}}use a::S as First;use self::First as S;fn main(){let p:S=S{x:1};println!("{}",p.x);}`,
		`#[derive(Copy,Clone)]struct Point{x:i32}fn main(){let Point=2;let p=Point{x:Point};println!("{}",p.x);}`,
		`mod a{#[derive(Copy,Clone)]struct Private{x:i32}mod b{pub fn call()->i32{let p:super::Private=super::Private{x:3};p.x}}pub fn call()->i32{b::call()}}fn main(){println!("{}",a::call());}`,
		`mod a{#[derive(Copy,Clone)]struct Private{x:i32}pub fn make()->Private{Private{x:1}}}fn main(){}`,
	}
	for i, source := range cases {
		program, err := syntax.Parse(source)
		if err != nil {
			t.Fatalf("case%d parse: %v", i, err)
		}
		if _, err := resolveModules(program); err != nil {
			t.Fatalf("case%d resolve: %v", i, err)
		}
	}
}

func TestResolveStructureRejectsInvalidTypes(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"private constructor", `mod a{#[derive(Copy,Clone)]struct S{pub x:i32}}fn main(){let p=a::S{x:1};}`, "private"},
		{"private signature type", `mod a{#[derive(Copy,Clone)]struct S{pub x:i32}}fn use_it(p:a::S){}fn main(){}`, "private"},
		{"private import", `mod a{#[derive(Copy,Clone)]struct S{pub x:i32}}use a::S;fn main(){}`, "private"},
		{"unknown annotation", `fn main(){let p:Absent=1;}`, "unresolved module path"},
		{"function type", `fn f(){}fn use_it(p:f){}fn main(){}`, "not a structure type"},
		{"module type", `mod a{}fn use_it(p:a){}fn main(){}`, "not a structure type"},
		{"function constructor", `fn f(){}fn main(){let p=f{};}`, "not a structure type"},
		{"shared item namespace", `#[derive(Copy,Clone)]struct S{x:i32}fn S(){}fn main(){}`, "shared function/module/import/structure"},
		{"duplicate structures", `#[derive(Copy,Clone)]struct S{x:i32}#[derive(Copy,Clone)]struct S{x:i32}fn main(){}`, "duplicate structure"},
		{"primitive shadow", `#[derive(Copy,Clone)]struct i32{x:bool}fn main(){}`, "shadowing primitive type"},
		{"unknown field type", `#[derive(Copy,Clone)]struct S{x:Absent}fn main(){}`, "unresolved module path"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			program, err := syntax.Parse(tt.source)
			if err != nil {
				t.Fatal(err)
			}
			_, err = resolveModules(program)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v, want %q", err, tt.want)
			}
		})
	}
}

func TestResolveImportedMainRetainsResolvedStructureSignature(t *testing.T) {
	program, err := syntax.Parse(`mod a{#[derive(Copy,Clone)]pub struct S{pub x:i32}pub fn make()->S{S{x:1}}}use a::make as main;`)
	if err != nil {
		t.Fatal(err)
	}
	flat, err := resolveModules(program)
	if err != nil {
		t.Fatal(err)
	}
	entry := flat.Functions[len(flat.Functions)-1]
	if entry.Name != "main" || entry.ReturnType != "struct:a::S" {
		t.Fatalf("entry signature=%#v", entry)
	}
}

func TestResolveRejectsStructureAliasesShadowingPrimitiveTypes(t *testing.T) {
	cases := []struct{ name, source string }{
		{"direct i32", `mod a{#[derive(Copy,Clone)]pub struct P{pub x:i32}}use a::P as i32;fn f(x:i32)->i32{x+1}fn main(){println!("{}",f(2));}`},
		{"direct bool", `mod a{#[derive(Copy,Clone)]pub struct P{pub x:i32}}use a::P as bool;fn f(x:bool)->bool{!x}fn main(){println!("{}",f(false));}`},
		{"chained bool", `mod a{#[derive(Copy,Clone)]pub struct P{pub x:i32}}use a::P as Alias;use Alias as bool;fn f(x:bool)->bool{!x}fn main(){println!("{}",f(false));}`},
		{"chained i64", `mod a{#[derive(Copy,Clone)]pub struct P{pub x:i32}}use a::P as First;use self::First as Second;use Second as i64;fn f(x:i64)->i64{x+1}fn main(){println!("{}",f(2));}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := Check(tt.source)
			if err == nil || !strings.Contains(err.Error(), "shadowing primitive type") || !strings.Contains(err.Error(), "structure alias") {
				t.Fatalf("got %v", err)
			}
		})
	}
}

func TestResolveAllowsPrimitiveNamedFunctionAndModuleAliases(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"function i32", `mod a{pub fn value()->i32{2}}use a::value as i32;fn f(x:i32)->i32{x+1}fn main(){println!("{} {}",f(2),i32());}`, "3 2\n"},
		{"module i32", `mod a{pub fn value()->i32{2}}use a as i32;fn f(x:i32)->i32{x+1}fn main(){println!("{} {}",f(2),i32::value());}`, "3 2\n"},
		{"function bool", `mod a{pub fn value()->bool{true}}use a::value as bool;fn f(x:bool)->bool{!x}fn main(){println!("{} {}",f(false),bool());}`, "true true\n"},
		{"module bool", `mod a{pub fn value()->bool{true}}use a as bool;fn f(x:bool)->bool{!x}fn main(){println!("{} {}",f(false),bool::value());}`, "true true\n"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result := runNative(t, tt.source)
			if result.status != 0 || result.stdout != tt.want {
				t.Fatalf("status=%d stdout=%q stderr=%q", result.status, result.stdout, result.stderr)
			}
		})
	}
}
