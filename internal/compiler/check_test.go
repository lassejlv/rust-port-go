package compiler

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/internal/syntax"
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

func TestLibraryAnalysisDoesNotRequireEntrypoint(t *testing.T) {
	cases := []string{
		``,
		`pub fn value()->i32{42}`,
		`pub fn main(x:i32)->i32{x+1}`,
		`#[derive(Copy,Clone)]pub struct Value{pub x:i32}pub fn value()->Value{Value{x:42}}`,
	}
	for _, source := range cases {
		program, err := syntax.Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := analyzeProgramMode(program, false); err != nil {
			t.Fatalf("library %q: %v", source, err)
		}
	}
}

func TestLibraryAnalysisStillChecksBodiesAndLayouts(t *testing.T) {
	cases := []struct{ source, want string }{
		{`pub fn value()->i32{unknown()}`, "unknown function"},
		{`pub fn value()->i32{true}`, "expected i32"},
		{`#[derive(Copy,Clone)]struct Loop{next:Loop}`, "recursive by-value"},
		{`pub fn main(x:())->i32{1}`, "unsupported parameter"},
	}
	for _, tt := range cases {
		program, err := syntax.Parse(tt.source)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := analyzeProgramMode(program, false); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("got %v, want %q", err, tt.want)
		}
	}
}

func TestBinaryAnalysisRetainsEntrypointRequirement(t *testing.T) {
	program, err := syntax.Parse(`pub fn value()->i32{42}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := analyzeProgramMode(program, true); err == nil || !strings.Contains(err.Error(), "missing main") {
		t.Fatalf("got %v", err)
	}
}

func TestModulePrivacyKeepsCrateRootsSeparate(t *testing.T) {
	cases := []struct {
		owner, current string
		want           bool
	}{
		{"", "", true}, {"", "child::nested", true}, {"", "@crate1", false}, {"", "@crate1::nested", false},
		{"@crate1", "@crate1", true}, {"@crate1", "@crate1::nested", true}, {"@crate1", "@crate10", false},
		{"@crate1::a", "@crate1::a::child", true}, {"@crate1::a", "@crate1::b", false}, {"@crate1", "", false},
	}
	for _, tt := range cases {
		if got := moduleContains(tt.owner, tt.current); got != tt.want {
			t.Fatalf("owner=%q current=%q got %v", tt.owner, tt.current, got)
		}
	}
	c := &checker{checked: &checked{structs: map[string]*syntax.Struct{
		"struct:Private":          {Name: "Private"},
		"struct:Public":           {Name: "Public", Public: true},
		"struct:@crate1::Private": {Name: "@crate1::Private"},
	}}, currentModule: "@crate1"}
	span := syntax.Span{Line: 1, Column: 1}
	if err := c.visibleType("struct:Private", span); err == nil {
		t.Fatal("external crate accessed selected root's private type")
	}
	if err := c.visibleType("struct:Public", span); err != nil {
		t.Fatal(err)
	}
	c.currentModule = ""
	if err := c.visibleType("struct:@crate1::Private", span); err == nil {
		t.Fatal("selected root accessed dependency's private type")
	}
}

func checkerCrate(t *testing.T, id, source string, externs map[string]*syntax.Program) *syntax.Program {
	t.Helper()
	program, err := syntax.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	program.CrateID, program.Externs = id, externs
	return program
}

func TestDependencyMainIsAnOrdinaryLibraryFunction(t *testing.T) {
	library := checkerCrate(t, "@crate1", `pub fn main(n:i32)->i32{n+1}`, nil)
	root := checkerCrate(t, "", `fn main(){println!("{}",math::main(4));}`, map[string]*syntax.Program{"math": library})
	c, err := analyzeProgram(root)
	if err != nil {
		t.Fatal(err)
	}
	if c.funcs["main"] == nil || c.funcs["@crate1::main"] == nil || len(c.funcs["@crate1::main"].Params) != 1 {
		t.Fatal("dependency main collided with selected binary main")
	}
	root = checkerCrate(t, "", `pub fn value()->i32{math::main(4)}`, map[string]*syntax.Program{"math": library})
	if _, err := analyzeProgramMode(root, false); err != nil {
		t.Fatalf("library root depending on ordinary main: %v", err)
	}
}

func TestDependencyMainCannotSatisfyBinaryEntrypoint(t *testing.T) {
	library := checkerCrate(t, "@crate1", `pub fn main(){}`, nil)
	root := checkerCrate(t, "", `pub fn value()->i32{42}`, map[string]*syntax.Program{"math": library})
	if _, err := analyzeProgram(root); err == nil || !strings.Contains(err.Error(), "missing main") {
		t.Fatalf("got %v", err)
	}
}

func TestSharedDependencyStructIdentity(t *testing.T) {
	common := checkerCrate(t, "@crate3", `#[derive(Copy,Clone)]pub struct Value{pub n:i32}pub fn make(n:i32)->Value{Value{n}}`, nil)
	left := checkerCrate(t, "@crate1", `pub fn make()->common::Value{common::make(10)}`, map[string]*syntax.Program{"common": common})
	right := checkerCrate(t, "@crate2", `pub fn read(value:renamed::Value)->i32{value.n}`, map[string]*syntax.Program{"renamed": common})
	root := checkerCrate(t, "", `fn main(){println!("{}",right::read(left::make()));}`, map[string]*syntax.Program{"left": left, "right": right})
	c, err := analyzeProgram(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.structs) != 1 || c.structs["struct:@crate3::Value"] == nil {
		t.Fatalf("shared library type identity lost: %v", c.structs)
	}
}

func TestCrossCrateStructPrivacyAndIdentity(t *testing.T) {
	cases := []struct{ name, library, root, want string }{
		{"private returned type", `#[derive(Copy,Clone)]struct Hidden{pub n:i32}pub fn make()->Hidden{Hidden{n:1}}`, `fn main(){dep::make();}`, "type \"@crate1::Hidden\" is private"},
		{"private field", `#[derive(Copy,Clone)]pub struct Value{n:i32}pub fn make()->Value{Value{n:1}}`, `fn main(){println!("{}",dep::make().n);}`, "field \"n\" of struct \"@crate1::Value\" is private"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			library := checkerCrate(t, "@crate1", tt.library, nil)
			root := checkerCrate(t, "", tt.root, map[string]*syntax.Program{"dep": library})
			if _, err := analyzeProgram(root); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v,want %q", err, tt.want)
			}
		})
	}
	left := checkerCrate(t, "@crate1", `#[derive(Copy,Clone)]pub struct Value{pub n:i32}pub fn make()->Value{Value{n:1}}`, nil)
	right := checkerCrate(t, "@crate2", `#[derive(Copy,Clone)]pub struct Value{pub n:i32}pub fn read(value:Value)->i32{value.n}`, nil)
	root := checkerCrate(t, "", `fn main(){right::read(left::make());}`, map[string]*syntax.Program{"left": left, "right": right})
	if _, err := analyzeProgram(root); err == nil || !strings.Contains(err.Error(), "expected struct:@crate2::Value, found struct:@crate1::Value") {
		t.Fatalf("separate library type identities unified: %v", err)
	}
}

func TestIntegerMetadataAndLiteralBounds(t *testing.T) {
	for _, name := range integerSuffixes {
		t.Run(name, func(t *testing.T) {
			info, ok := integerInfo(name)
			if !ok || info.Bits <= 0 || info.CType == "" || info.PrintMacro == "" {
				t.Fatalf("bad integer metadata for %s: %#v", name, info)
			}
			if (name == "isize" || name == "usize") && info.Bits != pointerBits {
				t.Fatal("pointer integer width differs from host")
			}
			limit := new(big.Int).Lsh(big.NewInt(1), uint(info.Bits))
			minimum := big.NewInt(0)
			if info.Signed {
				limit.Rsh(limit, 1)
				minimum.Neg(new(big.Int).Set(limit))
			}
			maximum := new(big.Int).Sub(limit, big.NewInt(1))
			for _, value := range []*big.Int{minimum, maximum} {
				source := fmt.Sprintf("fn main(){let value:%s=%s%s;println!(\"{}\",value);}", name, value.String(), name)
				if err := Check(source); err != nil {
					t.Fatalf("boundary %s: %v", source, err)
				}
			}
			tooBig := new(big.Int).Add(maximum, big.NewInt(1))
			if err := Check(fmt.Sprintf("fn main(){let value=%s%s;}", tooBig.String(), name)); err == nil || !strings.Contains(err.Error(), "out of range") {
				t.Fatalf("out of range %s accepted: %v", name, err)
			}
			if info.Signed {
				tooSmall := new(big.Int).Sub(minimum, big.NewInt(1))
				if err := Check(fmt.Sprintf("fn main(){let value=%s%s;}", tooSmall.String(), name)); err == nil || !strings.Contains(err.Error(), "out of range") {
					t.Fatalf("negative overflow %s accepted: %v", name, err)
				}
			} else if err := Check(fmt.Sprintf("fn main(){let value=-0%s;}", name)); err == nil || !strings.Contains(err.Error(), "unsigned") {
				t.Fatalf("unsigned negation %s accepted: %v", name, err)
			}
		})
	}
	for _, name := range []string{"i128", "u128", "f64", "bool"} {
		if _, ok := integerInfo(name); ok {
			t.Fatalf("unsupported integer metadata %q", name)
		}
	}
}

func TestCheckIntegerAndBitOperations(t *testing.T) {
	cases := []struct{ name, source string }{
		{"narrow arithmetic", `fn add(a:i8,b:i8)->i8{a+b}fn main(){let a:i16=-32768;let b=add(4,5);println!("{} {}",a,b);}`},
		{"unsigned extrema and bases", `fn main(){let a=255u8;let b=0xffffu16;let c=0xffff_ffffu32;let d=18446744073709551615u64;println!("{} {} {} {}",a,b,c,d);}`},
		{"bitwise integers", `fn mask(a:u16)->u16{!a&0xffu16|2u16^3u16}fn main(){println!("{} {}",mask(9),!3i8);}`},
		{"bitwise booleans", `fn main(){let mut b=true;b&=false;b|=true;b^=false;println!("{} {} {} {}",b,true&false,true|false,true^false);}`},
		{"independent shift types", `fn main(){let a:u8=1<<2i64;let b=1i8<<1u64;let c=4u64>>1u8;let d:isize=2<<1usize;println!("{} {} {} {}",a,b,c,d);}`},
		{"integer compound operators", `fn main(){let mut a=15u8;a&=7;a|=8;a^=3;a<<=1u64;a>>=1i8;a+=1;a-=1;a*=1;a/=1;a%=15;println!("{}",a);}`},
		{"integer record fields", `#[derive(Copy,Clone)]struct Bits{n:u8,flag:bool}fn main(){let mut b=Bits{n:15,flag:true};b.n&=7;b.n<<=1usize;b.n>>=1i16;b.flag^=true;println!("{} {}",b.n,b.flag);}`},
		{"pointer nominal values", `fn id(n:usize)->usize{n}fn signed(n:isize)->isize{n}fn main(){println!("{} {}",id(12),signed(-12));}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := Check(tt.source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckRejectsIntegerOperatorMismatches(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"mixed widths", `fn main(){let a=1u8+2u16;}`, "expected u8, found u16"},
		{"mixed signs", `fn main(){let a=1i8+2u8;}`, "expected i8, found u8"},
		{"isize nominal", `fn main(){let a:isize=1i64;}`, "expected isize, found i64"},
		{"usize nominal", `fn main(){let a:usize=1u64;}`, "expected usize, found u64"},
		{"unsigned unary negation variable", `fn main(){let a=1u8;let b=-a;}`, "unsigned"},
		{"bitwise mixed types", `fn main(){let a=true&1;}`, "expected bool"},
		{"bitwise string", `fn main(){let a="a"|"b";}`, "matching integers or booleans"},
		{"shift bool lhs", `fn main(){let a=true<<1;}`, "must be integers"},
		{"shift bool rhs", `fn main(){let a=1u8<<true;}`, "must be integers"},
		{"arithmetic compound bool", `fn main(){let mut a=true;a+=false;}`, "requires an integer"},
		{"shift compound bool lhs", `fn main(){let mut a=true;a<<=1;}`, "requires an integer"},
		{"shift compound bool rhs", `fn main(){let mut a=1u8;a<<=true;}`, "shift count must be an integer"},
		{"field bitwise mismatch", `#[derive(Copy,Clone)]struct Bits{n:u8}fn main(){let mut a=Bits{n:1};a.n|=2u16;}`, "expected u8, found u16"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			err := Check(tt.source)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("got %v,want %q", err, tt.want)
			}
		})
	}
}

func TestShiftAndAssignmentMetadataPreservesOperandTypes(t *testing.T) {
	c, err := analyze(`#[derive(Copy,Clone)]struct Bits{n:u8}fn main(){let inferred=1<<2u64;let mut a=1u8;a<<=2i16;let mut b=Bits{n:1};b.n>>=1usize;}`)
	if err != nil {
		t.Fatal(err)
	}
	stmts := c.funcs["main"].Body.Statements
	if c.types[stmts[0].Expr] != "i32" || c.types[stmts[0].Expr.Right] != "u64" {
		t.Fatal("shift count changed inferred left type")
	}
	if c.assignTypes[&stmts[2]] != "u8" || c.types[stmts[2].Expr] != "i16" {
		t.Fatal("simple shift assignment lost distinct LHS/RHS types")
	}
	if c.assignTypes[&stmts[4]] != "u8" || c.types[stmts[4].Expr] != "usize" {
		t.Fatal("field shift assignment lost distinct LHS/RHS types")
	}
}
