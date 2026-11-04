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
		{"missing return", `fn f(x:bool)->i32{if x{return 1;}} fn main(){}`, "block result"},
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
