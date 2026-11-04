package compiler

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type nativeResult struct {
	stdout, stderr string
	status         int
}

func runNative(t *testing.T, source string) nativeResult {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "main.rs")
	output := filepath.Join(dir, "app")
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Build(input, output); err != nil {
		t.Fatalf("Build: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, output)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("generated executable timed out: %v", ctx.Err())
	}
	result := nativeResult{stdout: stdout.String(), stderr: stderr.String()}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("run generated executable: %v", err)
		}
		result.status = exit.ExitCode()
	}
	return result
}

func TestNativeBootstrapFeatures(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"empty main", `fn main() {}`, ""},
		{"recursion and forward call", `
fn main() { println!("{} {}", fib(10), odd(9)); }
fn fib(n: i64) -> i64 { if n < 2 { return n; } fib(n - 1) + fib(n - 2) }
fn odd(n: i32) -> bool { if n == 0 { return false; } even(n - 1) }
fn even(n: i32) -> bool { if n == 0 { return true; } odd(n - 1) }
`, "55 true\n"},
		{"integer types and bases", `
fn add(a: i64, b: i64) -> i64 { a+b }
fn main() {
 let a: i64 = 9_000_000_000;
 let b = 0xff_i32;
 let c: i64 = 0b1010;
 let d = 0o77i64;
 println!("{} {} {} {}", add(a,1), b, c, d);
}`, "9000000001 255 10 63\n"},
		{"lexical shadowing", `
fn main() {
 let x = 1;
 let x = x + 2;
 { let mut x = x + 4; x *= 2; println!("{}", x); }
 { let x = true; assert!(x); println!("{}", x); }
 println!("{}", x);
}`, "14\ntrue\n3\n"},
		{"loop break and continue", `
fn main() {
 let mut i = 0; let mut sum = 0;
 while i < 10 {
  i += 1;
  if i == 3 { continue; }
  if i == 7 { break; }
  sum += i;
 }
 println!("{} {}", i, sum);
}`, "7 18\n"},
		{"while condition reevaluation", `
fn condition(n: i32) -> bool { print!("{}", n); n < 3 }
fn main() {
 let mut n = 0;
 while condition(n) { n += 1; if n == 1 { continue; } }
 println!("={}",n);
}`, "0123=3\n"},
		{"nested loops", `
fn main() {
 let mut a = 0; let mut count = 0;
 while a < 3 {
  a += 1; let mut b = 0;
  while b < 5 { b += 1; if b == 2 { continue; } if b == 4 { break; } count += 1; }
 }
 println!("{}",count);
}`, "6\n"},
		{"branch and unit returns", `
fn noop() { print!("!"); }
fn stop() { return noop(); }
fn classify(n: i32) -> i32 {
 if n < 0 { return -1; } else if n == 0 { return 0; } else { return 1; }
}
fn main() { stop(); println!("{} {} {}",classify(-2),classify(0),classify(9)); }
`, "!-1 0 1\n"},
		{"short circuit", `
fn mark(n: i32, value: bool) -> bool { print!("{}",n); value }
fn main() {
 let a = mark(1,false) && mark(9,true);
 let b = mark(2,true) || mark(8,false);
 let c = mark(3,true) && mark(4,true);
 let d = mark(5,false) || mark(6,true);
 println!(" {} {} {} {}",a,b,c,d);
}`, "123456 false true true true\n"},
		{"left to right operand and argument evaluation", `
fn mark(n: i32) -> i32 { print!("{}",n); n }
fn pair(a: i32, b: i32) -> i32 { a * 10 + b }
fn main() {
 println!("={}",pair(mark(1),mark(2)));
 println!("={}",mark(3)+mark(4));
 println!("={}",mark(5)<mark(6));
 println!("{} {}",mark(7),mark(8));
}`, "12=12\n34=7\n56=true\n787 8\n"},
		{"signed minimum and safe boundaries", `
fn main() {
 let lo32 = -2147483648; let hi32 = 2147483647;
 let lo64 = -9223372036854775808i64; let hi64 = 9223372036854775807i64;
 println!("{} {} {} {}",lo32,hi32,lo64,hi64);
 println!("{} {} {} {}",lo32+1,hi32-1,lo64+1,hi64-1);
 println!("{} {} {} {}",lo32*1,lo64*1,lo32/1,lo64%1);
}`, "-2147483648 2147483647 -9223372036854775808 9223372036854775807\n-2147483647 2147483646 -9223372036854775807 9223372036854775806\n-2147483648 -9223372036854775808 -2147483648 0\n"},
		{"signed division remainder and compound arithmetic", `
fn main() {
 let mut a = 7; a *= 3; a -= 1; a /= 4; a %= 3; a += 8;
 println!("{} {} {} {} {}",a,-7/3,-7%3,7/-3,7%-3);
 println!("{} {} {}",-6*7,-6*-7,0*-2147483648);
}`, "10 -2 -1 -2 1\n-42 42 0\n"},
		{"UTF-8 NUL strings parameters returns equality", `
fn identity(s: &str) -> &str { s }
fn equal(a: &str,b: &str) -> bool { a == b }
fn main() {
 let text = identity("h\u{e9}🦀\0tail");
 print!("{}",text);
 println!(" {} {} {} {}",equal(text,"hé🦀\0tail"),text!="hé🦀\0other","a\0b"=="a\0c",""=="");
}`, "hé🦀\x00tail true true false true\n"},
		{"format braces and macro tail", `
fn main() { println!("{{{}}} {{}}",42); print!("{}","{}"); println!(); println!("done") }
`, "{42} {}\n{}\ndone\n"},
		{"empty strings and punctuation", `fn main() { println!("{}{}{}", "", "\\\"%", ""); }`, "\\\"%\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := runNative(t, tc.source)
			if result.status != 0 || result.stderr != "" || result.stdout != tc.want {
				t.Fatalf("status=%d stdout=%q stderr=%q; want stdout=%q", result.status, result.stdout, result.stderr, tc.want)
			}
		})
	}
}

func TestNativeRuntimeTraps(t *testing.T) {
	cases := []struct{ name, declarations, expression, want string }{
		{"i32 add", `let a=2147483647;`, `a+1`, "addition overflow"},
		{"i32 subtract", `let a=-2147483648;`, `a-1`, "subtraction overflow"},
		{"i32 multiply", `let a=1073741824;`, `a*2`, "multiplication overflow"},
		{"i32 multiply min negative", `let a=-2147483648;`, `a*-1`, "multiplication overflow"},
		{"i32 divide zero", `let a=1; let b=0;`, `a/b`, "division by zero"},
		{"i32 divide min negative", `let a=-2147483648;`, `a/-1`, "division overflow"},
		{"i32 remainder zero", `let a=1; let b=0;`, `a%b`, "remainder by zero"},
		{"i32 remainder min negative", `let a=-2147483648;`, `a%-1`, "remainder overflow"},
		{"i32 negate", `let a=-2147483648;`, `-a`, "negation overflow"},
		{"i64 add", `let a=9223372036854775807i64;`, `a+1`, "addition overflow"},
		{"i64 subtract", `let a=-9223372036854775808i64;`, `a-1`, "subtraction overflow"},
		{"i64 multiply", `let a=4611686018427387904i64;`, `a*2`, "multiplication overflow"},
		{"i64 multiply min negative", `let a=-9223372036854775808i64;`, `a*-1`, "multiplication overflow"},
		{"i64 divide zero", `let a=1i64; let b=0i64;`, `a/b`, "division by zero"},
		{"i64 divide min negative", `let a=-9223372036854775808i64;`, `a/-1`, "division overflow"},
		{"i64 remainder zero", `let a=1i64; let b=0i64;`, `a%b`, "remainder by zero"},
		{"i64 remainder min negative", `let a=-9223372036854775808i64;`, `a%-1`, "remainder overflow"},
		{"i64 negate", `let a=-9223372036854775808i64;`, `-a`, "negation overflow"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			source := `fn main() { ` + tc.declarations + ` println!("{}",` + tc.expression + `); }`
			result := runNative(t, source)
			if result.status != 101 || !strings.Contains(result.stderr, tc.want) {
				t.Fatalf("status=%d stdout=%q stderr=%q; want exit 101 and %q", result.status, result.stdout, result.stderr, tc.want)
			}
		})
	}
	t.Run("assertion", func(t *testing.T) {
		result := runNative(t, "fn main() {\nassert!(false);\n}")
		if result.status != 101 || !strings.Contains(result.stderr, "assertion failed at line 2") {
			t.Fatalf("assert result = %#v", result)
		}
	})
}

func TestBuildPreservesExistingFilesOnFailure(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.rs")
	output := filepath.Join(dir, "app")
	valid := []byte(`fn main() { println!("original"); }`)
	if err := os.WriteFile(source, valid, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Build(source, output); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	invalid := []byte(`fn main() { absent(); }`)
	if err := os.WriteFile(source, invalid, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Build(source, output); err == nil {
		t.Fatal("invalid source built")
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("failed build changed existing executable")
	}
	got, err = os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, invalid) {
		t.Fatal("failed build changed source")
	}
	if err := Build(source, source); err == nil {
		t.Fatal("source overwrite accepted")
	}
	got, err = os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, invalid) {
		t.Fatal("source overwrite attempt changed source")
	}
}

func TestBuildPreservesExecutableOnBackendFailure(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.rs")
	output := filepath.Join(dir, "app")
	if err := os.WriteFile(source, []byte(`fn main() {}`), 0600); err != nil {
		t.Fatal(err)
	}
	original := []byte("existing executable content")
	if err := os.WriteFile(output, original, 0700); err != nil {
		t.Fatal(err)
	}
	// A missing compiler tests the failure path without interpreting a shell command.
	t.Setenv("CC", filepath.Join(dir, "missing-compiler"))
	if err := Build(source, output); err == nil {
		t.Fatal("build with missing compiler succeeded")
	}
	got, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, original) {
		t.Fatal("backend failure changed existing executable")
	}
}

func TestBuildRejectsSourceAliases(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "main.rs")
	contents := []byte(`fn main() {}`)
	if err := os.WriteFile(source, contents, 0600); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"hardlink", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			alias := filepath.Join(dir, kind)
			var err error
			if kind == "hardlink" {
				err = os.Link(source, alias)
			} else {
				err = os.Symlink(source, alias)
			}
			if err != nil {
				t.Skipf("link unsupported: %v", err)
			}
			if err := Build(source, alias); err == nil {
				t.Fatal("source alias overwrite accepted")
			}
			got, err := os.ReadFile(source)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, contents) {
				t.Fatal("source alias overwrite changed source")
			}
		})
	}
}

func FuzzEmitC(f *testing.F) {
	for _, seed := range []string{
		`fn main() {}`,
		`fn main() { println!("{}",1); }`,
		`fn main() { let mut n=0; while n<2 {n+=1;} }`,
		`fn f(n:i64)->i64{n+1} fn main(){println!("{}",f(3));}`,
		`fn main(){println!("{{{}}}","hello");}`,
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, source string) { _, _ = EmitC(source) })
}
