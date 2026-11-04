package compiler

import (
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

func TestArrayTypeHelpers(t *testing.T) {
	for _, tt := range []struct {
		typ, element string
		length       int
	}{
		{"array:0:i32", "i32", 0}, {"array:65536:bool", "bool", 65536},
		{"array:2:array:3:struct:@crate1::Value", "array:3:struct:@crate1::Value", 2},
	} {
		spec, ok := arrayInfo(tt.typ)
		if !ok || spec.Element != tt.element || spec.Length != tt.length || arrayType(spec.Element, spec.Length) != tt.typ {
			t.Fatalf("bad array metadata %q: %#v %v", tt.typ, spec, ok)
		}
	}
	for _, typ := range []string{"i32", "array:", "array:1:", "array:-1:i32", "array:01:i32", "array:65537:i32", "array:1:array:65537:i32"} {
		if _, ok := arrayInfo(typ); ok {
			t.Fatalf("invalid canonical array %q", typ)
		}
	}
	span := syntax.Span{Line: 2, Column: 3}
	for _, tt := range []struct {
		text string
		want int
	}{{"0", 0}, {"0xffusize", 255}, {"65_536", 65536}, {"0b10", 2}, {"0o10", 8}} {
		length, err := parseArrayLength(tt.text, span)
		if err != nil || length != tt.want {
			t.Fatalf("%s: %d %v", tt.text, length, err)
		}
	}
	for _, text := range []string{"-1", "1u8", "1i32", "65537", "1+1", "unknown", "18446744073709551615usize"} {
		if _, err := parseArrayLength(text, span); err == nil {
			t.Fatalf("invalid array length %q", text)
		}
	}
}

func TestCheckCopyArrays(t *testing.T) {
	cases := []struct{ name, source string }{
		{"literal inference", `fn main(){let a=[1,2,3];println!("{} {}",a[0],a.len());}`},
		{"typed literal", `fn main(){let a:[u8;2]=[1,255];println!("{}",a[1]);}`},
		{"repeat and zero", `fn value()->i32{3}fn main(){let a=[value();0];let b:[bool;0]=[];println!("{} {}",a.len(),b.len());}`},
		{"nested arrays", `fn main(){let a:[[i16;2];2]=[[1,2],[3,4]];println!("{}",a[1][0]);}`},
		{"contextual indexed literal", `fn main(){let a:u8=[1][0];let b:u8=[[1]][0][0];let c:u8=[[[1]]][0][0][0];println!("{} {} {}",a,b,c);}`},
		{"array values and returns", `fn reverse(a:[i32;2])->[i32;2]{[a[1],a[0]]}fn main(){let a=[1,2];let b=reverse(a);println!("{} {}",a[0],b[0]);}`},
		{"array fields and places", `#[derive(Copy,Clone)]struct Value{items:[u8;2]}fn main(){let mut value=Value{items:[1,2]};value.items[0]+=1;value.items[1]<<=1u16;println!("{}",value.items[0]);}`},
		{"array records and nested places", `#[derive(Copy,Clone)]struct Value{n:i32}fn main(){let mut a=[[Value{n:1};2];2];a[0][1].n=3;println!("{}",a[0][1].n);}`},
		{"array aliases", `mod a{#[derive(Copy,Clone)]pub struct Value{pub n:i32}}use a::Value as V;fn main(){let a:[V;1]=[V{n:1}];println!("{}",a[0].n);}`},
		{"array branch values", `fn main(){let a=if true{[1,2]}else{[3,4]};println!("{}",a[0]);}`},
		{"divergent index target", `fn value()->i32{let mut a=[0];a[{return 7;0usize}]=1;}fn main(){println!("{}",value());}`},
		{"divergent projected target", `#[derive(Copy,Clone)]struct Value{n:i32}fn value()->i32{let mut a=[Value{n:0}];a[{return 7;0usize}].n=1;}fn main(){println!("{}",value());}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if err := Check(tt.source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCheckRejectsInvalidArrays(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"empty inference", `fn main(){let a=[];}`, "explicit element type"},
		{"literal length", `fn main(){let a:[i32;2]=[1];}`, "array length"},
		{"repeat length", `fn main(){let a:[i32;2]=[1;3];}`, "array length"},
		{"mixed element types", `fn main(){let a=[1,true];}`, "expected bool"},
		{"string elements", `fn main(){let a=["text"];}`, "unsupported Copy array element"},
		{"unit elements", `fn main(){let a=[()];}`, "unsupported Copy array element"},
		{"string type", `fn value(a:[&str;1]){}fn main(){}`, "unsupported parameter type"},
		{"scalar index", `fn main(){let a=1;let b=a[0];}`, "receiver must be an array"},
		{"wrong index type", `fn main(){let a=[1];let b=a[0u8];}`, "expected usize"},
		{"negative index", `fn main(){let a=[1];let b=a[-1];}`, "unsigned integer usize"},
		{"immutable place", `fn main(){let a=[1];a[0]=2;}`, "requires a mutable local"},
		{"rvalue place", `fn values()->[i32;1]{[1]}fn main(){values()[0]=2;}`, "rooted in a mutable local"},
		{"wrong element assignment", `fn main(){let mut a=[1];a[0]=true;}`, "expected i32"},
		{"scalar len", `fn main(){let a=1;let b=a.len();}`, "len receiver must be an array"},
		{"recursive array layout", `#[derive(Copy,Clone)]struct Value{items:[Value;1]}fn main(){}`, "recursive by-value"},
		{"zero recursive array layout", `#[derive(Copy,Clone)]struct Value{items:[Value;0]}fn main(){}`, "recursive by-value"},
		{"array formatting", `fn main(){println!("{}",[1]);}`, "array values cannot be formatted"},
		{"array invalid repeat value", `fn main(){let a=[unknown();0];}`, "unknown function"},
		{"untyped divergent index", `fn value()->i32{let a=[0];a[{return 7;}]}fn main(){}`, "value-less divergent indexes"},
		{"untyped divergent place", `fn value()->i32{let mut a=[0];a[{return 7;}]=1;}fn main(){}`, "value-less divergent indexes"},
		{"private element return", `mod a{#[derive(Copy,Clone)]struct Hidden{n:i32}pub fn make()->[Hidden;0]{[]}}fn main(){a::make();}`, "type \"a::Hidden\" is private"},
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

func TestMixedArrayRecordLayoutOrder(t *testing.T) {
	c, err := analyze(`#[derive(Copy,Clone)]struct Outer{items:[Inner;2]}#[derive(Copy,Clone)]struct Inner{n:i32}fn main(){let inferred=[true;3];}`)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(c.typeOrder, ",") != "struct:Inner,array:2:struct:Inner,struct:Outer,array:3:bool" {
		t.Fatalf("mixed layout %v", c.typeOrder)
	}
	if strings.Join(c.structOrder, ",") != "struct:Inner,struct:Outer" {
		t.Fatalf("record layout %v", c.structOrder)
	}
}

func TestArrayIndexCastInferenceBarrier(t *testing.T) {
	for _, source := range []string{
		`fn main(){let n=[256][0] as u8;}`,
		`fn main(){let a=[256];let n=a[0] as u8;}`,
		`fn main(){let a=[256];let n=a[0];let value=n as u8;}`,
		`fn main(){let n=[256;1][0] as u8;}`,
	} {
		if err := Check(source); err != nil {
			t.Fatalf("index cast barrier %s: %v", source, err)
		}
	}
	for _, source := range []string{
		`fn main(){let n=[18446744073709551615][0] as u64;}`,
		`fn main(){let a=[18446744073709551615];let n=a[0] as u64;}`,
	} {
		if err := Check(source); err == nil {
			t.Fatalf("cast target leaked into array element inference: %s", source)
		}
	}
}

func TestIndexedPlaceMetadataAndDivergence(t *testing.T) {
	c, err := analyze(`fn value()->i32{let mut a=[[0u8;2];2];a[0][{return 7;0usize}]<<=1u64;}fn main(){}`)
	if err != nil {
		t.Fatal(err)
	}
	stmts := c.funcs["value"].Body.Statements
	place := stmts[1].Target
	if c.assignTypes[&stmts[1]] != "u8" || !c.diverges[place] || c.names[place.Left.Left] != c.decls[&stmts[0]] {
		t.Fatal("indexed place lost root, LHS type, or divergence")
	}
}

func TestZeroSizedAggregateMetadata(t *testing.T) {
	c, err := analyze(`
#[derive(Copy,Clone)]struct Empty{}
#[derive(Copy,Clone)]struct ZeroFields{empty:Empty,bytes:[u8;0]}
#[derive(Copy,Clone)]struct Nonzero{empty:Empty,n:u8}
#[derive(Copy,Clone)]struct Nested{values:[ZeroFields;7]}
fn main(){
 let huge=[[[0u8;0];4096];4096];
 let empty=[Empty{};65536];
 let absent:[Nonzero;0]=[];
 let present=[Nonzero{empty:Empty{},n:1};2];
}
`)
	if err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{
		"struct:Empty", "struct:ZeroFields", "struct:Nested",
		"array:0:u8", "array:4096:array:0:u8", "array:4096:array:4096:array:0:u8",
		"array:65536:struct:Empty", "array:0:struct:Nonzero", "array:7:struct:ZeroFields",
	} {
		if !c.zeroSized[typ] {
			t.Fatalf("logical zero-sized aggregate not marked: %s", typ)
		}
	}
	for _, typ := range []string{"u8", "bool", "i64", "struct:Nonzero", "array:2:struct:Nonzero"} {
		if c.zeroSized[typ] {
			t.Fatalf("nonzero type marked zero-sized: %s", typ)
		}
	}
	for _, typ := range c.typeOrder {
		if _, exists := c.zeroSized[typ]; !exists {
			t.Fatalf("aggregate missing size metadata: %s", typ)
		}
	}
}
