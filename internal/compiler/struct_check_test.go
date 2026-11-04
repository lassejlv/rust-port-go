package compiler

import (
	"strings"
	"testing"
)

func TestCheckCopyStructs(t *testing.T) {
	cases := []struct{ name, source string }{
		{"construct fields", `#[derive(Copy,Clone)] struct Point{x:i32,y:i64} fn main(){let p=Point{y:2,x:1};println!("{} {}",p.x,p.y);}`},
		{"empty record", `#[derive(Copy,Clone)]struct Empty{} fn identity(v:Empty)->Empty{v} fn main(){let empty=Empty{};identity(empty);identity(empty);}`},
		{"shorthand and copying", `#[derive(Clone,Copy)]struct Point{x:i32,y:i32} fn main(){let x=3;let y=4;let p=Point{x,y};let q=p;println!("{} {}",p.x,q.y);}`},
		{"nested record", `#[derive(Copy,Clone)]struct Outer{inner:Inner,flag:bool} #[derive(Copy,Clone)]struct Inner{n:i64} fn main(){let mut o=Outer{flag:true,inner:Inner{n:1}};o.inner.n+=2;println!("{} {}",o.inner.n,o.flag);}`},
		{"parameters and results", `#[derive(Copy,Clone)]struct Point{x:i32} fn change(p:Point)->Point{Point{x:p.x+1}} fn main(){let p=Point{x:1};println!("{} {}",change(p).x,p.x);}`},
		{"branch record value", `#[derive(Copy,Clone)]struct Point{x:i32} fn choose(flag:bool)->Point{if flag{Point{x:1}}else{Point{x:2}}} fn main(){println!("{}",choose(false).x);}`},
		{"record assignment", `#[derive(Copy,Clone)]struct Inner{n:i32} #[derive(Copy,Clone)]struct Outer{inner:Inner} fn main(){let mut o=Outer{inner:Inner{n:1}};o.inner=Inner{n:2};println!("{}",o.inner.n);}`},
		{"private fields in owner", `mod a{#[derive(Copy,Clone)]pub struct Point{x:i32}pub fn make()->Point{Point{x:4}}pub fn read(p:Point)->i32{p.x}} fn main(){let p=a::make();println!("{}",a::read(p));}`},
		{"private fields in descendant", `mod a{#[derive(Copy,Clone)]pub struct Point{x:i32}pub mod b{pub fn make()->super::Point{super::Point{x:5}}pub fn read(p:super::Point)->i32{p.x}}}fn main(){println!("{}",a::b::read(a::b::make()));}`},
		{"named imports", `mod a{#[derive(Copy,Clone)]pub struct Point{pub x:i32}}use a::Point as Position;fn main(){let p:Position=Position{x:6};println!("{}",p.x);}`},
		{"public inferred unnameable type", `mod a{mod hidden{#[derive(Copy,Clone)]pub struct Point{pub x:i32}}pub fn make()->hidden::Point{hidden::Point{x:7}}}fn main(){println!("{}",a::make().x);}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := Check(tt.source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckRejectsInvalidCopyStructs(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"move only", `struct Point{x:i32}fn main(){}`, "#[derive(Copy, Clone)]"},
		{"string field", `#[derive(Copy,Clone)]struct Point{x:&str}fn main(){}`, "&str fields require explicit lifetimes"},
		{"unit field", `#[derive(Copy,Clone)]struct Point{x:()}fn main(){}`, "unsupported Copy field type"},
		{"recursive layout", `#[derive(Copy,Clone)]struct Point{x:Point}fn main(){}`, "recursive by-value"},
		{"mutual recursive layout", `#[derive(Copy,Clone)]struct A{x:B}#[derive(Copy,Clone)]struct B{x:A}fn main(){}`, "recursive by-value"},
		{"duplicate declaration field", `#[derive(Copy,Clone)]struct Point{x:i32,x:i32}fn main(){}`, "duplicate field"},
		{"duplicate constructor field", `#[derive(Copy,Clone)]struct Point{x:i32}fn main(){let p=Point{x:1,x:2};}`, "duplicate field"},
		{"missing constructor field", `#[derive(Copy,Clone)]struct Point{x:i32,y:i32}fn main(){let p=Point{x:1};}`, "missing field"},
		{"unknown constructor field", `#[derive(Copy,Clone)]struct Point{x:i32}fn main(){let p=Point{x:1,y:2};}`, "unknown field"},
		{"wrong constructor type", `#[derive(Copy,Clone)]struct Point{x:i32}fn main(){let p=Point{x:true};}`, "expected i32, found bool"},
		{"wrong named type", `#[derive(Copy,Clone)]struct A{x:i32}#[derive(Copy,Clone)]struct B{x:i32}fn main(){let p:A=B{x:1};}`, "expected struct:A, found struct:B"},
		{"scalar field access", `fn main(){let p=1;println!("{}",p.x);}`, "has no named fields"},
		{"unknown field access", `#[derive(Copy,Clone)]struct Point{x:i32}fn main(){let p=Point{x:1};println!("{}",p.y);}`, "unknown field"},
		{"immutable field mutation", `#[derive(Copy,Clone)]struct Point{x:i32}fn main(){let p=Point{x:1};p.x=2;}`, "requires a mutable local"},
		{"temporary field mutation", `#[derive(Copy,Clone)]struct Point{x:i32}fn value()->Point{Point{x:1}}fn main(){value().x=2;}`, "rooted in a mutable local"},
		{"wrong field assignment type", `#[derive(Copy,Clone)]struct Point{x:i32}fn main(){let mut p=Point{x:1};p.x=true;}`, "expected i32, found bool"},
		{"private field read", `mod a{#[derive(Copy,Clone)]pub struct Point{x:i32}pub fn make()->Point{Point{x:1}}}fn main(){println!("{}",a::make().x);}`, "field \"x\" of struct \"a::Point\" is private"},
		{"private field construction", `mod a{#[derive(Copy,Clone)]pub struct Point{x:i32}}fn main(){let p=a::Point{x:1};}`, "is private"},
		{"private return type", `mod a{#[derive(Copy,Clone)]struct Point{pub x:i32}pub fn make()->Point{Point{x:1}}}fn main(){a::make();}`, "type \"a::Point\" is private"},
		{"private nested field type", `mod a{#[derive(Copy,Clone)]struct Inner{pub x:i32}#[derive(Copy,Clone)]pub struct Outer{pub inner:Inner}pub fn make()->Outer{Outer{inner:Inner{x:1}}}}fn main(){a::make().inner;}`, "type \"a::Inner\" is private"},
		{"format unsupported", `#[derive(Copy,Clone)]struct Point{x:i32}fn main(){println!("{}",Point{x:1});}`, "without a Display implementation"},
		{"comparison unsupported", `#[derive(Copy,Clone)]struct Point{x:i32}fn main(){let p=Point{x:1};let b=p==p;}`, "does not support struct:Point"},
		{"arithmetic unsupported", `#[derive(Copy,Clone)]struct Point{x:i32}fn main(){let p=Point{x:1};let q=p+p;}`, "arithmetic requires integers"},
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

func TestStructLayoutTopologicalOrder(t *testing.T) {
	c, err := analyze(`#[derive(Copy,Clone)]struct Outer{inner:Inner}#[derive(Copy,Clone)]struct Inner{n:i32}fn main(){}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(c.structOrder, ",") != "struct:Inner,struct:Outer" {
		t.Fatalf("layout order %v", c.structOrder)
	}
}

func TestStructFieldPlaceUsesOriginalLocal(t *testing.T) {
	c, err := analyze(`#[derive(Copy,Clone)]struct Outer{inner:Inner}#[derive(Copy,Clone)]struct Inner{n:i32}fn main(){let mut o=Outer{inner:Inner{n:1}};o.inner.n=2;}`)
	if err != nil {
		t.Fatal(err)
	}
	stmts := c.funcs["main"].Body.Statements
	place := stmts[1].Target
	if place == nil || c.names[place.Left.Left] != c.decls[&stmts[0]] || c.types[place] != "i32" || c.types[place.Left] != "struct:Inner" {
		t.Fatal("assignment place lost root binding or field types")
	}
}
