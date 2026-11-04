package compiler

import (
	"strings"
	"testing"
)

func TestCheckBuiltinForLoops(t *testing.T) {
	cases := []struct{ name, source string }{
		{"integer ranges", `fn main(){let mut total=0;for n in 0..4{total+=n;}for n in 2..=4{total+=n;}println!("{}",total);}`},
		{"typed wide bounds", `fn main(){for n in -9223372036854775808i64..=-9223372036854775807i64{println!("{}",n);}for n in 18446744073709551615u64..=18446744073709551615u64{println!("{}",n);}}`},
		{"typed endpoints unify", `fn main(){for n in 0..3u8{println!("{}",n);}for n in -2i16..2{println!("{}",n);}}`},
		{"usize indexing", `fn main(){let values=[1,2,3];for n in 0..values.len(){println!("{}",values[n]);}}`},
		{"mut pattern", `fn main(){for mut n in 0u8..3{n+=1;println!("{}",n);}}`},
		{"wildcard and zero length", `fn main(){for _ in [1,2]{}let empty:[i32;0]=[];for _ in empty{println!("unreachable");}}`},
		{"owned array and mutable records", `#[derive(Copy,Clone)]struct Value{n:i32}fn main(){let values=[Value{n:1};2];for mut value in values{value.n+=1;println!("{}",value.n);}println!("{}",values[0].n);}`},
		{"nested arrays and loops", `fn main(){for mut row in [[1,2],[3,4]]{row[0]+=1;for n in row{if n==2{continue;}if n==4{break;}println!("{}",n);}}}`},
		{"header outside pattern scope", `fn main(){let value=[1,2];for value in value{println!("{}",value);}println!("{}",value.len());}`},
		{"header expressions", `fn low()->i32{0}fn high()->i32{3}fn values()->[i32;2]{[4,5]}fn main(){for n in low()..high(){println!("{}",n);}for n in values(){println!("{}",n);}}`},
		{"pattern cast inference barrier", `fn main(){for n in 0..256{let value=n as u8;}for n in [256]{let value=n as u8;}let values=[256];for n in values{let alias=n;let value=alias as u8;}}`},
		{"header diverges with explicit result", `fn value()->i32{for n in ({return 7;0i32})..1i32{}0}fn array()->i32{for n in ({return 8;[0]}){}0}fn main(){println!("{} {}",value(),array());}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := Check(tt.source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckRejectsInvalidForLoops(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"immutable pattern", `fn main(){for n in 0..2{n=1;}}`, "immutable variable"},
		{"immutable record pattern", `#[derive(Copy,Clone)]struct Value{n:i32}fn main(){for value in [Value{n:1}]{value.n=2;}}`, "requires a mutable local"},
		{"unknown after scope", `fn main(){for n in 0..2{}println!("{}",n);}`, "unknown variable"},
		{"unknown in header", `fn main(){for n in n..2{}}`, "unknown variable"},
		{"wrong item iterable", `fn main(){for n in 1{}}`, "owned Copy array"},
		{"bool range", `fn main(){for n in false..true{}}`, "supported integers"},
		{"mixed widths", `fn main(){for n in 0i8..2i16{}}`, "expected i8, found i16"},
		{"mixed pointer type", `fn main(){for n in 0u64..2usize{}}`, "expected u64, found usize"},
		{"unsigned negative", `fn main(){for n in -1..2u8{}}`, "unsigned integer u8"},
		{"body requires unit", `fn main(){for n in 0..2{n}}`, "expected ()"},
		{"no guaranteed return", `fn value()->i32{for n in 0..1{return 1;}}fn main(){}`, "block result"},
		{"diverging header still unit", `fn value()->i32{for n in ({return 7;0i32})..1i32{}}fn main(){}`, "block result"},
		{"diverging array header still unit", `fn value()->i32{for n in ({return 7;[0]}){}}fn main(){}`, "block result"},
		{"pattern shadows function", `fn value(){}fn main(){for value in 0..2{value();}}`, "variable and cannot be called"},
		{"out of range default bounds", `fn main(){for n in 0..18446744073709551615{let value=n as u64;}}`, "out of range for i32"},
		{"explicit narrow overflow", `fn main(){for n in 0u8..256{}}`, "out of range for u8"},
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

func TestForLoopMetadataAndScope(t *testing.T) {
	c, err := analyze(`fn main(){let items=[1u8,2];for mut items in items{items+=1;}for _ in 0..2usize{}}`)
	if err != nil {
		t.Fatal(err)
	}
	stmts := c.funcs["main"].Body.Statements
	first, second := &stmts[1], &stmts[2]
	if c.forTypes[first] != "u8" || c.forTypes[second] != "usize" {
		t.Fatalf("for item types %q %q", c.forTypes[first], c.forTypes[second])
	}
	if c.names[first.Expr] != c.decls[&stmts[0]] || c.decls[first] == c.decls[&stmts[0]] {
		t.Fatal("iterable was rebound before header analysis")
	}
	if c.decls[&first.Body.Statements[0]] != c.decls[first] {
		t.Fatal("body mutation did not use scoped pattern binding")
	}
}
