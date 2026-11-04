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
	conditional := main.Body.Statements[2]
	if conditional.Else == nil || conditional.Else.Statements[0].Kind != "if" {
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
	if main.Body.Statements[0].Kind != "block" || main.Body.Statements[0].Body.Tail.Op != "+" {
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
	for _, seed := range []string{"", "fn main() {}", "fn main() { println!(\"hello\"); }", "/* nested /* comments */ */", "fn f(n:i64)->i64 { n+1 }"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) { _, _ = Parse(source) })
}
