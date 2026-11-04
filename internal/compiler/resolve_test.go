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
