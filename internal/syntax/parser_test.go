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
		{"fn main() { let x = 1.0; }", "unsupported character"},
		{"fn main() { let x = 1u32; }", "integer literal"},
		{"fn main() { let x = 0b2; }", "integer literal"},
		{"fn main() { let x = 1i64i32; }", "integer literal"},
		{"fn main() { let x = 0x; }", "has no digits"},
		{"fn main() { let x = +1; }", "expected identifier"},
		{"fn main() { let x = 1 < 2 < 3; }", "cannot be chained"},
		{"fn main() { let x = true == true == true; }", "cannot be chained"},
		{"fn main() { let x = true == 1 < 2; }", "cannot be chained"},
		{"fn main() { let x: usize = 1; }", "unsupported type"},
		{"fn main() { let x; }", "expected ="},
		{"fn main() { for x in y {} }", "expected identifier"},
		{"pub fn main() {}", "expected fn"},
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
	for _, seed := range []string{"", "fn main() {}", "fn main() { println!(\"hello\"); }", "/* nested /* comments */ */", "fn f(n:i64)->i64 { n+1 }", "fn f(n:i32)->i32 { if n < 0 { 0 } else { {n+1} } }", "fn main() { let x = (); { (); } () }"} {
		f.Add(seed)
	}
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
