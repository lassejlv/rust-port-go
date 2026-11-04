package compiler

import (
	"fmt"
	"strings"
	"testing"
)

func TestCheckAllSupportedCastPairs(t *testing.T) {
	for _, source := range integerSuffixes {
		for _, target := range integerSuffixes {
			program := fmt.Sprintf("fn main(){let value:%s=1%s as %s;}", target, source, target)
			if err := Check(program); err != nil {
				t.Fatalf("%s -> %s: %v", source, target, err)
			}
		}
	}
	for _, target := range integerSuffixes {
		if err := Check(fmt.Sprintf("fn main(){let a=true as %s;let b=false as %s;}", target, target)); err != nil {
			t.Fatalf("bool -> %s: %v", target, err)
		}
	}
}

func TestCastLiteralSourceInference(t *testing.T) {
	cases := []struct{ source, wantSource, wantTarget string }{
		{`fn main(){let n=127 as i8;}`, "i8", "i8"},
		{`fn main(){let n=255 as u8;}`, "u8", "u8"},
		{`fn main(){let n=0xffff_ffff as u32;}`, "u32", "u32"},
		{`fn main(){let n=18446744073709551615 as u64;}`, "u64", "u64"},
		{`fn main(){let n=-128 as i8;}`, "i8", "i8"},
		{`fn main(){let n=256i16 as u8;}`, "i16", "u8"},
		{`fn main(){let n=-1i32 as u64;}`, "i32", "u64"},
		{`fn main(){let n=(1<<2u64) as u8;}`, "i32", "u8"},
		{`fn main(){let n=(1+2) as i16;}`, "i32", "i16"},
	}
	for _, tt := range cases {
		c, err := analyze(tt.source)
		if err != nil {
			t.Fatalf("%s: %v", tt.source, err)
		}
		cast := c.funcs["main"].Body.Statements[0].Expr
		if c.types[cast.Left] != tt.wantSource || c.types[cast] != tt.wantTarget {
			t.Fatalf("%s: source=%s target=%s", tt.source, c.types[cast.Left], c.types[cast])
		}
	}
	for _, source := range []string{
		`fn main(){let n=128 as i8;}`,
		`fn main(){let n=256 as u8;}`,
		`fn main(){let n=300 as u8;}`,
		`fn main(){let n=-1 as u8;}`,
		`fn main(){let n=-129 as i8;}`,
	} {
		if err := Check(source); err == nil {
			t.Fatalf("invalid source literal accepted: %s", source)
		}
	}
}

func TestFixedCastSourcesAndDefaultedBindingRestriction(t *testing.T) {
	for _, source := range []string{
		`fn main(){let n:i32=300;let value=n as u8;}`,
		`fn main(){let n=300i32;let alias=n;let value=alias as u8;}`,
		`fn value()->i32{300}fn main(){let n=value();let result=n as u8;}`,
		`#[derive(Copy,Clone)]struct Value{n:i32}fn main(){let value=Value{n:300};let result=value.n as u8;}`,
		`fn main(){let n=300;let result=(n+0i32) as u8;}`,
		`fn main(){let result=({let n:i32=300;n}) as u8;}`,
		`fn main(){let result=(if true{1u16}else{2}) as u8;}`,
		`fn main(){let result=((1<2) as u8) as u64;}`,
	} {
		if err := Check(source); err != nil {
			t.Fatalf("fixed cast source %s: %v", source, err)
		}
	}
	for _, source := range []string{
		`fn main(){let n=256;let result=n as u8;}`,
		`fn main(){let n=128;let alias=n;let result=alias as i8;}`,
		`fn main(){let n=300;let result=(n+1) as u8;}`,
		`fn main(){let n=300;let result=({n}) as u8;}`,
		`fn main(){let n=300;let result=(n<<2u64) as u8;}`,
	} {
		if err := Check(source); err == nil || !strings.Contains(err.Error(), "explicit source type or literal suffix") {
			t.Fatalf("defaulted cast %s: %v", source, err)
		}
	}
}

func TestCastRejectsUnsupportedSourcesAndTargets(t *testing.T) {
	cases := []struct{ source, want string }{
		{`fn main(){let n=1i32 as bool;}`, "cast target"},
		{`fn main(){let n=true as bool;}`, "cast target"},
		{`fn main(){let n=1i32 as ();}`, "cast target"},
		{`fn main(){let n=1i32 as &str;}`, "cast target"},
		{`#[derive(Copy,Clone)]struct Value{n:i32}fn main(){let n=1i32 as Value;}`, "cast target"},
		{`fn main(){let n="value" as i32;}`, "cast source"},
		{`fn main(){let n=() as u8;}`, "cast source"},
		{`#[derive(Copy,Clone)]struct Value{n:i32}fn main(){let n=Value{n:1} as u8;}`, "cast source"},
		{`fn main(){let n=({return;}) as i32;}`, "divergent cast sources"},
		{`fn main(){let n:u8=1i32 as i16;}`, "expected u8, found i16"},
	}
	for _, tt := range cases {
		err := Check(tt.source)
		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("%s: got %v,want %q", tt.source, err, tt.want)
		}
	}
}

func TestCastPreservesTypedOperandDivergence(t *testing.T) {
	c, err := analyze(`fn main(){let n=({return;0u8}) as i64;}`)
	if err != nil {
		t.Fatal(err)
	}
	cast := c.funcs["main"].Body.Statements[0].Expr
	if !c.diverges[cast] || c.types[cast] != "i64" || c.types[cast.Left] != "u8" {
		t.Fatal("cast lost typed source divergence or nominal result type")
	}
}

func TestCastSourceExpectationBarriers(t *testing.T) {
	cases := []struct{ expression, sourceType string }{
		{`(if true{256}else{0})`, "i32"},
		{`(256+0)`, "i32"},
		{`(256<<0)`, "i32"},
		{`({256+0})`, "i32"},
		{`({if true{256}else{0}})`, "i32"},
		{`(-(256+0))`, "i32"},
		{`({18446744073709551615})`, "u64"},
		{`(!18446744073709551615)`, "u64"},
	}
	for _, tt := range cases {
		target := "u8"
		if tt.sourceType == "u64" {
			target = "u64"
		}
		source := fmt.Sprintf("fn main(){let n=%s as %s;}", tt.expression, target)
		c, err := analyze(source)
		if err != nil {
			t.Fatalf("%s: %v", source, err)
		}
		cast := c.funcs["main"].Body.Statements[0].Expr
		if c.types[cast.Left] != tt.sourceType {
			t.Fatalf("%s: source type %s, want %s", source, c.types[cast.Left], tt.sourceType)
		}
	}
	for _, expression := range []string{
		`(if true{18446744073709551615}else{0}) as u64`,
		`(18446744073709551615+0) as u64`,
		`256 as u8`,
		`({256}) as u8`,
		`(!256) as u8`,
		`-({256}) as u8`,
		`(-(-256)) as u8`,
		`(!(-1)) as u8`,
	} {
		if err := Check("fn main(){let n=" + expression + ";}"); err == nil {
			t.Fatalf("invalid contextual source accepted: %s", expression)
		}
	}
}
