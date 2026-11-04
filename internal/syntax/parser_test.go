package syntax

import (
	"strings"
	"testing"
)

func TestParseProgram(t *testing.T) {
	p, err := Parse(`/* outer /* nested */ comment */
fn double(n: i64) -> i64 { n * 2 }
fn main() {
    let mut x: i64 = 0x10_i64;
    while x > 0 { x -= 1; if x == 3 { continue; } }
    if x == 0 { println!("value: {}", double(x)); }
    else if x < 0 { return; } else { assert!(false); }
}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Functions) != 2 {
		t.Fatalf("functions = %d", len(p.Functions))
	}
	double := p.Functions[0]
	if double.Name != "double" || double.ReturnType != "i64" || len(double.Params) != 1 || double.Params[0].Type != "i64" {
		t.Fatalf("function = %#v", double)
	}
	if double.Body.Tail == nil || double.Body.Tail.Op != "*" {
		t.Fatal("missing function tail expression")
	}
	main := p.Functions[1]
	if main.ReturnType != "()" {
		t.Fatalf("main type = %q", main.ReturnType)
	}
	let := main.Body.Statements[0]
	if let.Kind != "let" || !let.Mutable || let.Expr.Text != "0x10_i64" {
		t.Fatalf("let = %#v", let)
	}
	loop := main.Body.Statements[1]
	if loop.Kind != "while" || loop.Body.Statements[0].Op != "-=" {
		t.Fatalf("loop = %#v", loop)
	}
	conditional := main.Body.Tail
	if conditional == nil || conditional.Kind != "if" || conditional.Else == nil || conditional.Else.Tail.Kind != "if" {
		t.Fatal("else if was not nested")
	}
	macro := conditional.Body.Statements[0].Expr
	if macro.Kind != "macro" || macro.Text != "println" || len(macro.Args) != 2 {
		t.Fatalf("macro = %#v", macro)
	}
}

func TestPrecedenceAndGrouping(t *testing.T) {
	p, err := Parse(`fn main() { let result = 1 + 2 * 3 == 7 && !false || false; let b = (1 < 2) == true; let c = true == (1 < 2); }`)
	if err != nil {
		t.Fatal(err)
	}
	e := p.Functions[0].Body.Statements[0].Expr
	if e.Op != "||" || e.Left.Op != "&&" || e.Left.Left.Op != "==" || e.Left.Left.Left.Op != "+" || e.Left.Left.Left.Right.Op != "*" {
		t.Fatalf("wrong precedence: %#v", e)
	}
	if e.Left.Right.Kind != "unary" || e.Left.Right.Op != "!" {
		t.Fatal("unary expression missing")
	}
}

func TestStringsAndPositions(t *testing.T) {
	p, err := Parse("fn main() {\n    println!(\"hi\\n\\x41\\u{1f980}\\\\\\\"\\0\");\n}")
	if err != nil {
		t.Fatal(err)
	}
	e := p.Functions[0].Body.Statements[0].Expr
	if e.Span.Line != 2 || e.Span.Column != 5 {
		t.Fatalf("span = %#v", e.Span)
	}
	if e.Args[0].Text != "hi\nA🦀\\\"\x00" {
		t.Fatalf("decoded string = %q", e.Args[0].Text)
	}
}

func TestFunctionAndNestedBlockTails(t *testing.T) {
	p, err := Parse(`fn truth() -> bool { true } fn main() { { 2 + 3 } println!("{}", truth()) }`)
	if err != nil {
		t.Fatal(err)
	}
	if p.Functions[0].Body.Tail.Kind != "bool" {
		t.Fatal("boolean tail missing")
	}
	main := p.Functions[1]
	if main.Body.Statements[0].Kind != "expr" || main.Body.Statements[0].Type != "()" || main.Body.Statements[0].Expr.Kind != "block" || main.Body.Statements[0].Expr.Body.Tail.Op != "+" {
		t.Fatal("nested block tail missing")
	}
	if main.Body.Tail == nil || main.Body.Tail.Kind != "macro" {
		t.Fatal("macro tail missing")
	}
}

func TestIntegerForms(t *testing.T) {
	for _, literal := range []string{"0", "123_456", "1i32", "9i64", "0xff", "0xFF_i32", "0o777", "0b101_01i64", "2147483648"} {
		t.Run(literal, func(t *testing.T) {
			if _, err := Parse("fn main() { let x = " + literal + "; }"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRejectUnsupportedAndMalformedSyntax(t *testing.T) {
	cases := []struct{ source, message string }{
		{"fn main() { let x = ; }", "expected identifier"},
		{"fn main() { let x = 1 }", "expected ;"},
		{"fn main() { let x = 1.0; }", "floating-point"},
		{"fn main() { let x = 1u32; }", "integer literal"},
		{"fn main() { let x = 0b2; }", "integer literal"},
		{"fn main() { let x = 1i64i32; }", "integer literal"},
		{"fn main() { let x = 0x; }", "has no digits"},
		{"fn main() { let x = +1; }", "expected identifier"},
		{"fn main() { let x = 1 < 2 < 3; }", "cannot be chained"},
		{"fn main() { let x = true == true == true; }", "cannot be chained"},
		{"fn main() { let x = true == 1 < 2; }", "cannot be chained"},
		{"fn main() { let x; }", "expected ="},
		{"fn main() { for x in y {} }", "expected identifier"},
		{"pub const VALUE: i32 = 1;", "expected fn"},
		{"fn true() {}", "expected identifier"},
		{"fn main(mut x: i32) {}", "expected identifier"},
		{"fn main() { let _ = 1; }", "expected identifier"},
		{"fn main() {", "expected }"},
		{"/* comment", "unterminated block comment"},
		{`fn main() { println!("unterminated); }`, "unterminated string"},
		{`fn main() { println!("\q"); }`, "unsupported string escape"},
		{`fn main() { println!("\x80"); }`, "ASCII byte"},
		{`fn main() { println!("\u{d800}"); }`, "invalid Unicode escape"},
		{`fn main() { println!("\u{110000}"); }`, "invalid Unicode escape"},
		{"fn main() { let café = 1; }", "unsupported character"},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			_, err := Parse(tc.source)
			if err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("error = %v, want %q", err, tc.message)
			}
			if err != nil && !strings.Contains(err.Error(), ":") {
				t.Fatalf("error has no position: %v", err)
			}
		})
	}
}

func TestErrorLocationAfterNestedComment(t *testing.T) {
	_, err := Parse("/* one\n/* two */\n*/\nfn main() {\n  @\n}")
	if err == nil || !strings.HasPrefix(err.Error(), "5:3:") {
		t.Fatalf("error = %v", err)
	}
}

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"", "fn main() {}", "fn main() { println!(\"hello\"); }", "/* nested /* comments */ */", "fn f(n:i64)->i64 { n+1 }", "fn f(n:i32)->i32 { if n < 0 { 0 } else { {n+1} } }", "fn main() { let x = (); { (); } () }", "fn main(){print!(\"a\r\nb\");}", "//\xff\nfn main(){}", "pub mod child { pub fn value() -> i32 { 1 } } use crate::child::value as alias; fn main() { alias(); }", "mod external; mod child { use super::super as ancestor; }"} {
		f.Add(seed)
	}
	f.Add(`#[derive(Copy, Clone)] struct P {x:i32} fn main(){let mut p=P{x:1};p.x+=2;}`)
	f.Add(`#[derive(Clone,Copy,)] pub struct Empty {} fn main(){if check(Empty{}) {}}`)
	f.Fuzz(func(t *testing.T, source string) { _, _ = Parse(source) })
}

func TestIfExpressionsAndElseIfTails(t *testing.T) {
	p, err := Parse(`
fn choose(n: i32) -> i64 {
 let value: i64 = if n < 0 { -1 } else if n == 0 { 0 } else { 1 };
 if value == 0 { 100 } else { value }
}
fn main() { println!("{}", if true { choose(0) } else { 7 }); }
`)
	if err != nil {
		t.Fatal(err)
	}
	choose := p.Functions[0]
	initializer := choose.Body.Statements[0].Expr
	if initializer.Kind != "if" || initializer.Left.Op != "<" || initializer.Body.Tail.Kind != "unary" {
		t.Fatalf("if initializer = %#v", initializer)
	}
	if initializer.Else == nil || initializer.Else.Tail == nil || initializer.Else.Tail.Kind != "if" {
		t.Fatal("else-if not encoded as branch tail")
	}
	nested := initializer.Else.Tail
	if nested.Left.Op != "==" || nested.Body.Tail.Text != "0" || nested.Else.Tail.Text != "1" {
		t.Fatalf("nested conditional = %#v", nested)
	}
	if choose.Body.Tail.Kind != "if" || choose.Body.Tail.Body.Tail.Text != "100" {
		t.Fatal("if expression function tail missing")
	}
	argument := p.Functions[1].Body.Statements[0].Expr.Args[1]
	if argument.Kind != "if" || argument.Body.Tail.Kind != "call" {
		t.Fatal("if expression argument missing")
	}
}

func TestBlockExpressionOperandsAndBindings(t *testing.T) {
	p, err := Parse(`
fn calculate(n:i32) -> i32 {
 let x = { let temporary = n * 2; temporary + 1 };
 ({ x + 3 }) * { let y = 2; y }
}
fn main() { let answer = calculate({ 4 }); }
`)
	if err != nil {
		t.Fatal(err)
	}
	body := p.Functions[0].Body
	initializer := body.Statements[0].Expr
	if initializer.Kind != "block" || initializer.Body.Statements[0].Name != "temporary" || initializer.Body.Tail.Op != "+" {
		t.Fatal("block binding shape is wrong")
	}
	if body.Tail.Op != "*" || body.Tail.Left.Kind != "block" || body.Tail.Right.Kind != "block" {
		t.Fatal("block operands shape is wrong")
	}
	if p.Functions[1].Body.Statements[0].Expr.Args[0].Kind != "block" {
		t.Fatal("block call argument missing")
	}
}

func TestUnitExpressionsAndReturns(t *testing.T) {
	p, err := Parse(`fn unit() -> () { return (); } fn another() { (()) } fn main() { let value: () = (); unit(); () }`)
	if err != nil {
		t.Fatal(err)
	}
	returned := p.Functions[0].Body.Statements[0]
	if returned.Kind != "return" || returned.Expr.Kind != "unit" {
		t.Fatal("unit return expression missing")
	}
	if p.Functions[1].Body.Tail.Kind != "unit" {
		t.Fatal("parenthesized unit missing")
	}
	main := p.Functions[2].Body
	if main.Statements[0].Type != "()" || main.Statements[0].Expr.Kind != "unit" || main.Tail.Kind != "unit" {
		t.Fatal("unit binding or tail missing")
	}
}

func TestExpressionStatementUnitMarkers(t *testing.T) {
	p, err := Parse(`fn main() {
 if true {} else {} let a = 1;
 {} let b = 2;
 if true { 1 } else { 2 };
 { 3 };
 if false {}
}`)
	if err != nil {
		t.Fatal(err)
	}
	body := p.Functions[0].Body
	for _, index := range []int{0, 2} {
		stmt := body.Statements[index]
		if stmt.Kind != "expr" || stmt.Type != "()" {
			t.Fatalf("no-semicolon expression = %#v", stmt)
		}
	}
	for _, index := range []int{4, 5} {
		stmt := body.Statements[index]
		if stmt.Kind != "expr" || stmt.Type != "" {
			t.Fatalf("semicolon expression = %#v", stmt)
		}
	}
	if body.Tail.Kind != "if" || body.Tail.Else != nil {
		t.Fatal("if without else tail missing")
	}
}

func TestMalformedExpressionBlocks(t *testing.T) {
	for _, source := range []string{
		`fn main() { let x = if true 1 else { 2 }; }`,
		`fn main() { let x = if true { 1 } else 2; }`,
		`fn main() { let x = if true { 1 } else if { 2 }; }`,
		`fn main() { let x = { let a = 1; a; ; }`,
		`fn main() { let x = (1,); }`,
		`fn main() { () let a = 1; }`,
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := Parse(source); err == nil {
				t.Fatal("malformed expression accepted")
			}
		})
	}
}

func TestBareBlockAndIfStatementBoundaries(t *testing.T) {
	for _, source := range []string{
		`fn main() { {1} + 2; }`,
		`fn f() -> i32 { {1} + 2 } fn main() {}`,
		`fn main() { if true {1} else {2} + 3; }`,
		`fn f() -> i32 { if true {1} else {2} + 3 } fn main() {}`,
		`fn main() { ({}) let x = 1; }`,
		`fn main() { (if true {}) let x = 1; }`,
		`fn main() { {1} == 1; }`,
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := Parse(source); err == nil {
				t.Fatal("Rust-invalid statement boundary accepted")
			}
		})
	}
}

func TestGroupedAndRHSBlockArithmetic(t *testing.T) {
	for _, source := range []string{
		`fn main() { ({1}) + 2; }`,
		`fn main() { (if true {1} else {2}) + 3; }`,
		`fn f() -> i32 { ({1}) + 2 } fn main() {}`,
		`fn f() -> i32 { (if true {1} else {2}) + 3 } fn main() {}`,
		`fn main() { let x = {1} + 2; }`,
		`fn main() { let x = if true {1} else {2} + 3; }`,
		`fn f() -> i32 { return {1} + 2; } fn main() {}`,
		`fn main() { println!("{}", {1} + 2); }`,
		`fn main() { ({}); let x = 1; }`,
		`fn main() { (if true {}); let x = 1; }`,
		`fn f() -> i32 { ({1}) } fn main() {}`,
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := Parse(source); err != nil {
				t.Fatalf("Rust-valid supported syntax rejected: %v", err)
			}
		})
	}
}

func TestAdjacentBlockAndUnaryOrUnitStatements(t *testing.T) {
	p, err := Parse(`fn main() { {} -1; if true {} !false; {} (); }`)
	if err != nil {
		t.Fatal(err)
	}
	statements := p.Functions[0].Body.Statements
	if len(statements) != 6 {
		t.Fatalf("statements = %d, want 6", len(statements))
	}
	for _, index := range []int{0, 2, 4} {
		if statements[index].Type != "()" {
			t.Fatalf("statement %d lost unit marker", index)
		}
	}
	if statements[1].Expr.Kind != "unary" || statements[1].Expr.Op != "-" || statements[3].Expr.Kind != "unary" || statements[3].Expr.Op != "!" || statements[5].Expr.Kind != "unit" {
		t.Fatal("adjacent expression was consumed as a postfix/operator")
	}
}

func TestBareBlockCallTokensAreSeparateExpression(t *testing.T) {
	p, err := Parse(`fn f() {} fn main() { {f}(); if true {f} else {f}(); }`)
	if err != nil {
		t.Fatal(err)
	}
	statements := p.Functions[1].Body.Statements
	if len(statements) != 4 || statements[0].Expr.Kind != "block" || statements[0].Type != "()" || statements[1].Expr.Kind != "unit" || statements[2].Expr.Kind != "if" || statements[2].Type != "()" || statements[3].Expr.Kind != "unit" {
		t.Fatal("bare block call tokens did not split at Rust statement boundary")
	}
}

func TestPostfixFeaturesRemainExplicitlyUnsupported(t *testing.T) {
	for _, source := range []string{
		`fn main() { {1i32}.abs(); }`,
		`fn main() { if true {1i32} else {2i32}.abs(); }`,
		`fn main() { {1}[0]; }`,
		`fn f() -> i32 { let x = {1}?; x } fn main() {}`,
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := Parse(source); err == nil {
				t.Fatal("unsupported postfix expression accepted")
			}
		})
	}
}

func TestCRLFStringNormalizationAndContinuation(t *testing.T) {
	cases := []struct{ name, literal, want string }{
		{"raw CRLF", "\"first\r\nsecond\"", "first\nsecond"},
		{"mixed raw line endings", "\"a\r\nb\nc\r\nd\"", "a\nb\nc\nd"},
		{"CRLF continuation", "\"a\\\r\n  \tb\"", "ab"},
		{"LF continuation", "\"a\\\n  b\"", "ab"},
		{"continuation skips bare CR whitespace", "\"a\\\n \r b\"", "ab"},
		{"continuation skips CRLF whitespace", "\"a\\\r\n \r\n b\"", "ab"},
		{"escaped CR preserved", `"a\rb"`, "a\rb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Parse("fn main() { print!(" + tc.literal + "); }")
			if err != nil {
				t.Fatal(err)
			}
			got := p.Functions[0].Body.Statements[0].Expr.Args[0].Text
			if got != tc.want {
				t.Fatalf("decoded string=%q, want %q", got, tc.want)
			}
		})
	}
}

func TestRawAndEscapedBareCRRejected(t *testing.T) {
	for _, source := range []string{
		"fn main() { print!(\"a\rb\"); }",
		"fn main() { print!(\"a\\\rb\"); }",
		"fn main() { print!(\"a\r\r\nb\"); }",
	} {
		t.Run(source, func(t *testing.T) {
			_, err := Parse(source)
			if err == nil || !strings.Contains(err.Error(), "bare CR") {
				t.Fatalf("error=%v, want bare CR error", err)
			}
		})
	}
}

func TestCRLFNormalizationPreservesSourceLocations(t *testing.T) {
	source := "fn main() {\r\n  println!(\"first\r\nsecond\");\r\n  println!(\"end\");\r\n}"
	p, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	e := p.Functions[0].Body.Statements[1].Expr
	if e.Span.Line != 4 || e.Span.Column != 3 || e.Span.Offset != strings.Index(source, `println!("end")`) {
		t.Fatalf("span after normalized string=%#v", e.Span)
	}
}

func TestUnicodeEscapeUnderscoreRules(t *testing.T) {
	for _, escape := range []string{`\u{41_}`, `\u{41__________}`, `\u{4__1}`, `\u{0_0_0_0_4_1}`} {
		t.Run(escape, func(t *testing.T) {
			p, err := Parse(`fn main() { print!("` + escape + `"); }`)
			if err != nil {
				t.Fatal(err)
			}
			if got := p.Functions[0].Body.Statements[0].Expr.Args[0].Text; got != "A" {
				t.Fatalf("decoded Unicode escape=%q", got)
			}
		})
	}
	for _, escape := range []string{`\u{_41}`, `\u{__41}`, `\u{}`, `\u{_}`, `\u{0000041}`, `\u{+41}`} {
		t.Run(escape, func(t *testing.T) {
			_, err := Parse(`fn main() { print!("` + escape + `"); }`)
			if err == nil || !strings.Contains(err.Error(), "Unicode escape") {
				t.Fatalf("error=%v, want Unicode escape error", err)
			}
		})
	}
}

func TestInvalidUTF8RejectedAcrossWholeSource(t *testing.T) {
	cases := []struct{ name, source, position string }{
		{"first byte", "\xfffn main() {}", "1:1:"},
		{"line comment", "//é🦀\xff\nfn main() {}", "1:5:"},
		{"block comment", "/*é\n  \xff*/\nfn main() {}", "2:3:"},
		{"CRLF comment", "// valid\r\n//\xff\r\nfn main() {}", "2:3:"},
		{"string", "fn main() { print!(\"\xff\"); }", "1:21:"},
		{"truncated rune", "fn main() {} //\xe2\x82", "1:16:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.source)
			if err == nil || !strings.Contains(err.Error(), "invalid UTF-8 in source") || !strings.HasPrefix(err.Error(), tc.position) {
				t.Fatalf("error=%v, want %s invalid UTF-8 in source", err, tc.position)
			}
		})
	}
	// The encoded replacement character is valid UTF-8, unlike an invalid byte.
	if _, err := Parse("//é🦀�\nfn main() { print!(\"�\"); }"); err != nil {
		t.Fatal(err)
	}
}

func TestModuleAndImportItems(t *testing.T) {
	source := `use crate::arithmetic::sum as add;
pub use self::arithmetic as exported;
pub mod arithmetic {
 use super::helper as base;
 pub fn sum(n: i32) -> i32 { base(n) + 1 }
 mod nested {
  use super::super::helper;
  pub fn value() -> i32 { super::sum(crate::helper(1)) }
 }
 mod external;
}
pub fn main() { println!("{}", add(2)); }
fn helper(n:i32) -> i32 { n }
`
	p, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Functions) != 2 || len(p.Modules) != 1 || len(p.Imports) != 2 {
		t.Fatalf("root item counts = %#v", p)
	}
	if got := p.Imports[0]; got.Path != "crate::arithmetic::sum" || got.Alias != "add" || got.Public {
		t.Fatalf("import = %#v", got)
	}
	if got := p.Imports[1]; got.Path != "self::arithmetic" || got.Alias != "exported" || !got.Public {
		t.Fatalf("public import = %#v", got)
	}
	module := p.Modules[0]
	if module.Name != "arithmetic" || !module.Public || module.External || module.Program == nil {
		t.Fatalf("module = %#v", module)
	}
	if len(module.Program.Functions) != 1 || len(module.Program.Modules) != 2 || len(module.Program.Imports) != 1 {
		t.Fatal("inline module contents missing")
	}
	if !module.Program.Functions[0].Public || module.Program.Imports[0].Alias != "base" {
		t.Fatal("module function visibility or alias missing")
	}
	nested := module.Program.Modules[0]
	if nested.Name != "nested" || nested.Public || nested.External || nested.Program.Imports[0].Path != "super::super::helper" || nested.Program.Imports[0].Alias != "" {
		t.Fatalf("nested module = %#v", nested)
	}
	call := nested.Program.Functions[0].Body.Tail
	if call.Kind != "call" || call.Text != "super::sum" || call.Args[0].Text != "crate::helper" || call.Target != "" {
		t.Fatalf("qualified call = %#v", call)
	}
	external := module.Program.Modules[1]
	if external.Name != "external" || !external.External || external.Public || external.Program != nil {
		t.Fatalf("external module = %#v", external)
	}
	if !p.Functions[0].Public || p.Functions[1].Public {
		t.Fatal("function visibility is wrong")
	}
}

func TestQualifiedNameAndCallPaths(t *testing.T) {
	paths := []string{"value", "module::value", "crate::module::value", "self::value", "super::value", "super::super::value", "super::super::module::value"}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			p, err := Parse("fn main() { let value = " + path + "; " + path + "(); }")
			if err != nil {
				t.Fatal(err)
			}
			statements := p.Functions[0].Body.Statements
			if statements[0].Expr.Kind != "name" || statements[0].Expr.Text != path || statements[1].Expr.Kind != "call" || statements[1].Expr.Text != path {
				t.Fatal("qualified expression lost source path")
			}
		})
	}
}

func TestRootNamespaceAliasesAndExternalModuleVisibility(t *testing.T) {
	p, err := Parse(`use crate as root; mod child { use self as local; use super as parent; use super::super as ancestor; } pub mod external; fn main() {}`)
	if err != nil {
		t.Fatal(err)
	}
	if p.Imports[0].Path != "crate" || p.Imports[0].Alias != "root" {
		t.Fatal("crate namespace alias missing")
	}
	if len(p.Modules[0].Program.Imports) != 3 {
		t.Fatal("relative namespace aliases missing")
	}
	if !p.Modules[1].External || !p.Modules[1].Public || p.Modules[1].Program != nil {
		t.Fatal("public external module shape wrong")
	}
}

func TestNamespaceSourceSpans(t *testing.T) {
	source := "\n  pub mod outer {\n    use super::helper as alias;\n    pub fn function() { crate::helper(); }\n  }\n  fn main() {}"
	p, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	module := p.Modules[0]
	if module.Span.Line != 2 || module.Span.Column != 3 || module.Span.Offset != strings.Index(source, "pub mod") || module.Span.File != "" {
		t.Fatalf("module span=%#v", module.Span)
	}
	item := module.Program.Imports[0]
	if item.Span.Line != 3 || item.Span.Column != 5 || item.Span.Offset != strings.Index(source, "use super") {
		t.Fatalf("import span=%#v", item.Span)
	}
	function := module.Program.Functions[0]
	if function.Span.Line != 4 || function.Span.Column != 5 || function.Span.Offset != strings.Index(source, "pub fn") {
		t.Fatalf("public function span=%#v", function.Span)
	}
	call := function.Body.Statements[0].Expr
	if call.Span.Line != 4 || call.Span.Column != 25 || call.Span.Offset != strings.Index(source, "crate::helper") {
		t.Fatalf("qualified call span=%#v", call.Span)
	}
	if got := sourceError(Span{Line: 2, Column: 3, File: "module.rs"}, "example").Error(); got != "module.rs:2:3: example" {
		t.Fatalf("filename error=%q", got)
	}
}

func TestRejectUnsupportedNamespaceSyntax(t *testing.T) {
	cases := []struct{ source, want string }{
		{`pub(crate) fn main() {}`, "restricted visibility"},
		{`pub(super) mod child {}`, "restricted visibility"},
		{`pub(in crate) fn main() {}`, "restricted visibility"},
		{`use crate::module::*; fn main() {}`, "glob imports"},
		{`use crate::module::{a,b}; fn main() {}`, "group imports"},
		{`use {a,b}; fn main() {}`, "group imports"},
		{`use ::external::value; fn main() {}`, "absolute paths"},
		{`use crate; fn main() {}`, "explicit alias"},
		{`use self; fn main() {}`, "explicit alias"},
		{`mod child { use super; } fn main() {}`, "explicit alias"},
		{`use module::value as _; fn main() {}`, "expected identifier"},
		{`use module::value as self; fn main() {}`, "expected identifier"},
		{`use module::value fn main() {}`, "expected ;"},
		{`mod child fn main() {}`, "expected {"},
		{`mod child { fn value() {}`, "expected }"},
		{`fn main() { use crate::helper; }`, "function-local"},
		{`fn main() { mod child {} }`, "function-local"},
		{`fn main() { fn nested() {} }`, "function-local"},
		{`fn main() { module::super::value(); }`, "expected path segment"},
		{`fn main() { crate::self::value(); }`, "expected path segment"},
		{`fn main() { self::crate::value(); }`, "expected path segment"},
		{`fn main() { super::module::super::value(); }`, "expected path segment"},
		{`fn main() { crate::module::; }`, "expected path segment"},
		{`fn main() { ::module::value(); }`, "absolute paths"},
		{`fn main() { module::println!("hi"); }`, "qualified macros"},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			_, err := Parse(tc.source)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestCopyStructDeclarationsAndNamedTypes(t *testing.T) {
	p, err := Parse(`
#[derive(Clone, Copy,)]
pub struct Point { pub x:i32, y:i64, }
mod geometry {
 #[derive(Copy,Clone)] pub struct Shape { pub point:crate::Point, flag:bool }
}
#[derive(Copy, Clone)] struct Empty {}
fn identity(point:crate::Point) -> self::Point { let result:Point=point; result }
fn main() {}
`)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Structs) != 2 || len(p.Modules[0].Program.Structs) != 1 {
		t.Fatal("struct declarations missing")
	}
	point := p.Structs[0]
	if point.Name != "Point" || !point.Public || !point.Copy || len(point.Fields) != 2 || point.Fields[0].Name != "x" || point.Fields[0].Type != "i32" || !point.Fields[0].Public || point.Fields[1].Public {
		t.Fatalf("point declaration=%#v", point)
	}
	shape := p.Modules[0].Program.Structs[0]
	if shape.Name != "Shape" || !shape.Copy || !shape.Public || shape.Fields[0].Type != "crate::Point" || shape.Fields[1].Type != "bool" {
		t.Fatalf("shape declaration=%#v", shape)
	}
	if len(p.Structs[1].Fields) != 0 || !p.Structs[1].Copy {
		t.Fatal("empty record struct missing")
	}
	function := p.Functions[0]
	if function.Params[0].Type != "crate::Point" || function.ReturnType != "self::Point" || function.Body.Statements[0].Type != "Point" {
		t.Fatal("named type paths not preserved")
	}
}

func TestStructLiteralsShorthandAndFieldAssignments(t *testing.T) {
	p, err := Parse(`
#[derive(Copy,Clone)] struct Point {x:i32,y:i32}
#[derive(Copy,Clone)] struct Outer {point:Point}
fn main() {
 let x=1;
 let mut outer=Outer { point:crate::Point { y:2,x, }, };
 let value=outer.point.x;
 outer.point.x += { 3 };
 outer.point.y = Point { x:4, y:5 }.x;
}
`)
	if err != nil {
		t.Fatal(err)
	}
	statements := p.Functions[0].Body.Statements
	outer := statements[1].Expr
	if outer.Kind != "struct" || outer.Text != "Outer" || outer.Target != "" || len(outer.Fields) != 1 {
		t.Fatalf("outer constructor=%#v", outer)
	}
	point := outer.Fields[0].Value
	if point.Kind != "struct" || point.Text != "crate::Point" || len(point.Fields) != 2 || point.Fields[0].Name != "y" || point.Fields[1].Name != "x" {
		t.Fatal("constructor source field order lost")
	}
	if shorthand := point.Fields[1].Value; shorthand.Kind != "name" || shorthand.Text != "x" {
		t.Fatal("shorthand field not expanded to name expression")
	}
	projection := statements[2].Expr
	if projection.Kind != "field" || projection.Text != "x" || projection.Left.Kind != "field" || projection.Left.Text != "point" || projection.Left.Left.Text != "outer" {
		t.Fatal("nested projection shape wrong")
	}
	assignment := statements[3]
	if assignment.Kind != "assign" || assignment.Name != "" || assignment.Op != "+=" || assignment.Target == nil || assignment.Target.Text != "x" || assignment.Expr.Kind != "block" {
		t.Fatalf("field compound assignment=%#v", assignment)
	}
	assignment = statements[4]
	if assignment.Kind != "assign" || assignment.Target.Text != "y" || assignment.Expr.Kind != "field" || assignment.Expr.Left.Kind != "struct" {
		t.Fatal("field assignment RHS constructor projection missing")
	}
}

func TestFieldPostfixOnAllValueReceiverShapes(t *testing.T) {
	for _, expression := range []string{
		`value.field`, `make().field`, `(value).field`, `({value}).field`,
		`Point { x:1 }.x`, `(if true {value} else {value}).field`,
		`if true {value} else {value}.field`, `{value}.field`,
	} {
		t.Run(expression, func(t *testing.T) {
			p, err := Parse("fn main() { let result=" + expression + "; }")
			if err != nil {
				t.Fatal(err)
			}
			e := p.Functions[0].Body.Statements[0].Expr
			if e.Kind != "field" || e.Left == nil {
				t.Fatalf("field expression=%#v", e)
			}
		})
	}
	for _, source := range []string{
		`fn main() { {value}.field + 1; }`,
		`fn main() { if true {value} else {value}.field + 1; }`,
	} {
		p, err := Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		if p.Functions[0].Body.Statements[0].Expr.Op != "+" {
			t.Fatal("dotted block expression lost binary continuation")
		}
	}
}

func TestConditionStructLiteralRestrictions(t *testing.T) {
	for _, source := range []string{
		`fn main() { if flag {} }`,
		`fn main() { while flag {} }`,
		`fn main() { if point.flag {} }`,
		`fn main() { if (Point { flag:true }).flag {} }`,
		`fn main() { while (Point { flag:true }.flag) {} }`,
		`fn main() { if test(Point {flag:true}) {} }`,
		`fn main() { while test(Point {flag:true}) {} }`,
		`fn main() { if {Point {flag:true}.flag} {} }`,
		`fn main() { if flag && test(Point {flag:true}) {} }`,
		`fn main() { if true { let p=Point {flag:true}; } }`,
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := Parse(source); err != nil {
				t.Fatalf("valid condition syntax rejected: %v", err)
			}
		})
	}
	for _, source := range []string{
		`fn main() { if Point {flag:true}.flag {} }`,
		`fn main() { while Point {flag:true}.flag {} }`,
		`fn main() { if flag && Point {flag:true}.flag {} }`,
		`fn main() { if !Point {flag:true}.flag {} }`,
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := Parse(source); err == nil {
				t.Fatal("ungrouped condition constructor accepted")
			}
		})
	}
}

func TestStructSourceSpans(t *testing.T) {
	source := "\n #[derive(Copy,Clone)]\n pub struct Point {\n  pub x:i32,\n }\n fn main() { let x=1; let p=Point{x}; p.x=2; }"
	p, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	declaration := p.Structs[0]
	if declaration.Span.Line != 2 || declaration.Span.Column != 2 || declaration.Span.Offset != strings.Index(source, "#[derive") {
		t.Fatalf("struct span=%#v", declaration.Span)
	}
	field := declaration.Fields[0]
	if field.Span.Line != 4 || field.Span.Column != 3 || field.Span.Offset != strings.Index(source, "pub x") {
		t.Fatalf("field declaration span=%#v", field.Span)
	}
	literal := p.Functions[0].Body.Statements[1].Expr
	if literal.Span.Offset != strings.Index(source, "Point{x}") || literal.Fields[0].Span.Offset != strings.Index(source, "{x}")+1 || literal.Fields[0].Value.Span != literal.Fields[0].Span {
		t.Fatal("constructor or shorthand span lost")
	}
	assignment := p.Functions[0].Body.Statements[2]
	if assignment.Target.Span.Offset != strings.Index(source, "p.x=2") || assignment.Span != assignment.Target.Span {
		t.Fatal("assignment field span lost")
	}
}

func TestUnsupportedStructureForms(t *testing.T) {
	cases := []struct{ source, want string }{
		{`struct Point{x:i32} fn main(){}`, "require #[derive(Copy, Clone)]"},
		{`#[derive(Copy)] struct Point{x:i32} fn main(){}`, "both Copy and Clone"},
		{`#[derive(Clone)] struct Point{x:i32} fn main(){}`, "both Copy and Clone"},
		{`#[derive(Copy,Clone,Debug)] struct Point{x:i32} fn main(){}`, "exactly once"},
		{`#[derive(Copy,Copy,Clone)] struct Point{x:i32} fn main(){}`, "exactly once"},
		{`#[repr(C)] struct Point{x:i32} fn main(){}`, "only #[derive"},
		{`#[derive(Copy,Clone)] #[repr(C)] struct Point{x:i32} fn main(){}`, "multiple attributes"},
		{`#[derive(Copy,Clone)] fn main(){}`, "only supported on named structs"},
		{`#[derive(Copy,Clone)] struct Point(i32); fn main(){}`, "tuple and unit structs"},
		{`#[derive(Copy,Clone)] struct Point; fn main(){}`, "tuple and unit structs"},
		{`#[derive(Copy,Clone)] struct Point<T>{x:T} fn main(){}`, "generic structs"},
		{`#[derive(Copy,Clone)] struct Point{pub(crate) x:i32} fn main(){}`, "restricted field visibility"},
		{`#[derive(Copy,Clone)] struct Point{x:Vec<i32>} fn main(){}`, "generic type arguments"},
		{`#[derive(Copy,Clone)] struct Point{x:i32 y:i32} fn main(){}`, "expected }"},
		{`fn main(){let point=Point{x:1,..old};}`, "struct update"},
		{`fn main(){let point=Point{..old};}`, "struct update"},
		{`fn main(){let point=Point{x:1 y:2};}`, "expected }"},
		{`fn main(){let point=Point{x:};}`, "expected identifier"},
		{`fn main(){point.method();}`, "methods and callable fields"},
		{`fn main(){point.0;}`, "tuple fields"},
		{`fn main(){struct Point{x:i32}}`, "function-local"},
		{`fn main(){#[derive(Copy,Clone)] struct Point{x:i32}}`, "function-local"},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			_, err := Parse(tc.source)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}
