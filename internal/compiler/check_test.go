package compiler

import (
	"strings"
	"testing"
)

func TestCheckAcceptedCrates(t *testing.T) {
	cases := []struct{ name, source string }{
		{"empty main", `fn main() {}`},
		{"recursion", `fn fib(n: i64) -> i64 { if n < 2 { return n; } fib(n - 1) + fib(n - 2) } fn main() { println!("{}", fib(10)); }`},
		{"branches return", `fn sign(x: i32) -> bool { if x < 0 { return false; } else { return true; } } fn main() { assert!(sign(1)); }`},
		{"shadowing", `fn main() { let x = 1; let x = x + 2; { let x = true; assert!(x); } println!("{}", x); }`},
		{"loop control", `fn main() { let mut n = 0; while n < 10 { n += 1; if n == 3 { continue; } if n == 7 { break; } } println!("{}", n); }`},
		{"contextual integer", `fn add(x: i64) -> i64 { 1 + x } fn main() { let a: i64 = 2; let b = 1 + a; println!("{} {}", add(4), b); }`},
		{"literal bases", `fn main() { let a = 0xff_i32; let b: i64 = 0b1010; let c = 0o77i64; println!("{} {} {}", a,b,c); }`},
		{"signed minimum", `fn main() { let a = -2147483648; let b: i64 = -9223372036854775808; println!("{} {}",a,b); }`},
		{"format braces", `fn main() { println!("{{{}}} {{}}", 42); println!(); print!("{{}}"); }`},
		{"strings", `fn equal(a: &str, b: &str) -> bool { a == b } fn main() { assert!(equal("hi", "hi")); }`},
		{"if values", `fn choose(x: bool) -> i64 { if x { 1 } else { 2 } } fn main() { let x = if true { 4 } else { 5 }; println!("{} {}", x, choose(true)); }`},
		{"branch inference", `fn main() { let x = if true { 1 } else { 2i64 }; let y = if true { 1 } else { let z: i64 = 3; z }; println!("{} {}", x, y); }`},
		{"block values", `fn main() { let x: i64 = { let y = 2i64; y + 1 }; println!("{}", { let z = x; z * 2 }); }`},
		{"unit values", `fn noop() -> () { return (); } fn main() { (); noop(); if true { () } else { () }; () }`},
		{"discard values", `fn main() { if true { 1 } else { 2 }; { 3 }; }`},
		{"returning branch", `fn choose(x: bool) -> i64 { if x { return 7; } else { 8 } } fn main() { let y = if true { return; } else { 2 }; println!("{}", y); }`},
		{"block return", `fn value() -> i64 { { return 9; } } fn main() { println!("{}", value()); }`},
		{"divergent expression statement", `fn value() -> i64 { { return 9; }; } fn main() { println!("{}", value()); }`},
		{"returning initializer", `fn value() -> i32 { let x: i32 = { return 9; }; x } fn main() { println!("{}",value()); }`},
		{"returning call argument", `fn id(x: i32) -> i32 { x } fn value() -> i32 { id({ return 9; }); } fn main() { println!("{}",value()); }`},
		{"returning arithmetic", `fn value() -> i32 { ({ return 9; 0 }) + 2 } fn main() { println!("{}",value()); }`},
		{"elided string lifetime", `fn choose(flag: bool, source: &str) -> &str { if flag { source } else { "fallback" } } fn main() { println!("{}", choose(true, "value")); }`},
		{"divergent boolean operands", `fn a() -> bool { !({ return false; }) } fn b() -> bool { true && { return false; } } fn c() -> bool { false || { return true; } } fn main() { println!("{} {} {}", a(), b(), c()); }`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := Check(tt.source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckRejectsInvalidCrates(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"missing main", `fn helper() {}`, "missing main"},
		{"main arguments", `fn main(x:i32) {}`, "main must"},
		{"main return", `fn main()->i32 { 1 }`, "main must"},
		{"duplicate function", `fn main() {} fn main() {}`, "duplicate function"},
		{"duplicate parameter", `fn f(x:i32,x:i32){} fn main(){}`, "duplicate parameter"},
		{"undefined variable", `fn main(){ println!("{}", absent); }`, "unknown variable"},
		{"undefined function", `fn main(){ absent(); }`, "unknown function"},
		{"wrong arity", `fn f(x:i32){} fn main(){f();}`, "expects 1 arguments"},
		{"immutable assignment", `fn main(){ let x = 1; x = 2; }`, "immutable variable"},
		{"bad assignment type", `fn main(){let mut x=1; x=true;}`, "expected i32, found bool"},
		{"bad condition", `fn main(){if 1 {}}`, "expected bool"},
		{"break outside loop", `fn main(){break;}`, "break outside"},
		{"continue outside loop", `fn main(){continue;}`, "continue outside"},
		{"wrong return", `fn f()->i32{return true;} fn main(){}`, "expected i32"},
		{"missing return", `fn f(x:bool)->i32{if x{return 1;}} fn main(){}`, "expected i32"},
		{"out of scope", `fn main(){{let x=1;} println!("{}",x);}`, "unknown variable"},
		{"function shadow", `fn f(){} fn main(){let f=1;f();}`, "cannot be called"},
		{"bad arithmetic", `fn main(){let a=true+false;}`, "arithmetic requires"},
		{"integer mismatch", `fn main(){let a=1i64+2i32;}`, "expected i64"},
		{"integer overflow", `fn main(){let a=2147483648;}`, "out of range"},
		{"negative overflow", `fn main(){let a=-2147483649;}`, "out of range"},
		{"format missing", `fn main(){println!("{} {}",1);}`, "format expects 2"},
		{"format extra", `fn main(){println!("hello",1);}`, "format expects 0"},
		{"format unsupported", `fn main(){println!("{:?}",1);}`, "only {}"},
		{"format unmatched", `fn main(){println!("}");}`, "unmatched }"},
		{"format variable", `fn main(){let s="hello";println!(s);}`, "string literal"},
		{"assert wrong type", `fn main(){assert!(1);}`, "expected bool"},
		{"assert unsupported message", `fn main(){assert!(true,"message");}`, "one boolean"},
		{"unsupported macro", `fn main(){panic!("no");}`, "unsupported macro"},
		{"unit format", `fn noop(){} fn main(){println!("{}",noop());}`, "unit values"},
		{"nonunit branch", `fn main(){if true {1}}`, "expected ()"},
		{"branch mismatch", `fn main(){let x=if true {1}else{false};}`, "expected"},
		{"nonunit no semicolon", `fn main(){if true {1}else{2} println!("next");}`, "expected ()"},
		{"nonunit block no semicolon", `fn main(){{1} println!("next");}`, "expected ()"},
		{"unit binding", `fn main(){let x=();}`, "unit-valued bindings"},
		{"unit parameter", `fn f(x:()){} fn main(){}`, "unsupported parameter"},
		{"static type despite divergence", `fn f(x:i32)->bool{true} fn main(){let x:i32=f({return;});}`, "expected i32, found bool"},
		{"unreachable tail type", `fn main(){let x:i32={return; true};}`, "expected i32, found bool"},
		{"missing return lifetime", `fn value()->&str{"literal"} fn main(){}`, "exactly one &str parameter"},
		{"ambiguous return lifetime", `fn value(a:&str,b:&str)->&str{a} fn main(){}`, "exactly one &str parameter"},
		{"divergent arithmetic left", `fn value()->i32{({return 21;})+1} fn main(){}`, "divergent operands"},
		{"divergent arithmetic right", `fn value()->i32{1+{return 22;}} fn main(){}`, "divergent operands"},
		{"divergent arithmetic typed", `fn main(){let x:i32=1i32+{return;};}`, "divergent operands"},
		{"divergent unary negation", `fn value()->i32{-({return 21;})} fn main(){}`, "divergent operands"},
		{"divergent comparison", `fn main(){let x=({return;})==1;}`, "divergent operands"},
		{"divergent ordering", `fn main(){let x=({return;})<1;}`, "divergent operands"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := Check(tt.source)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v; want error containing %q", err, tt.want)
			}
		})
	}
}

func TestCheckResolvesShadowedVariablesToDistinctNames(t *testing.T) {
	c, err := analyze(`fn main(){let x=1;let x=x+1;println!("{}",x);}`)
	if err != nil {
		t.Fatal(err)
	}
	stmts := c.program.Functions[0].Body.Statements
	first, second := &stmts[0], &stmts[1]
	if c.decls[first] == c.decls[second] || c.names[second.Expr.Left] != c.decls[first] {
		t.Fatal("shadow initializer did not resolve to the earlier binding")
	}
	arg := stmts[2].Expr.Args[1]
	if c.names[arg] != c.decls[second] {
		t.Fatal("later use did not resolve to the new binding")
	}
}

func TestCheckErrorsIncludeSourceLocation(t *testing.T) {
	err := Check("fn main() {\n  let a: bool = 1;\n}")
	if err == nil || !strings.Contains(err.Error(), "2:") {
		t.Fatalf("missing line location: %v", err)
	}
}
