package compiler

import (
	"fmt"
	"strings"
	"testing"
)

func TestFormatPlanSupportedProfile(t *testing.T) {
	cases := []struct {
		format, rendered string
		arguments        int
	}{
		{"", "", 0},
		{"literal\n雪\x00", "literal\n雪\x00", 0},
		{"{{}}", "{}", 0},
		{"{{}}{{{}}}{}{{{{}}}}", "{}{[0]}[1]{{}}", 2},
		{"α{}\n{{雪}}\x00{}尾", "α[0]\n{雪}\x00[1]尾", 2},
	}
	for _, tt := range cases {
		t.Run(fmt.Sprintf("%q", tt.format), func(t *testing.T) {
			plan, err := planFormat(tt.format)
			if err != nil {
				t.Fatal(err)
			}
			var rendered strings.Builder
			next := 0
			for _, piece := range plan.pieces {
				if piece.isArgument {
					if piece.argument != next {
						t.Fatalf("argument index %d, want %d", piece.argument, next)
					}
					fmt.Fprintf(&rendered, "[%d]", piece.argument)
					next++
				} else {
					rendered.WriteString(piece.literal)
				}
			}
			if rendered.String() != tt.rendered || plan.arguments != tt.arguments || next != tt.arguments || plan.newline {
				t.Fatalf("plan rendered %q with %d arguments and newline=%v, want %q with %d arguments", rendered.String(), plan.arguments, plan.newline, tt.rendered, tt.arguments)
			}
		})
	}
}

func TestFormatPlanRejectsInvalidSyntaxAndUnsupportedProfile(t *testing.T) {
	for _, format := range []string{"{", "}", "{{}", "{!}", "{:..}", "{r#name}", "{name} }", "\xff"} {
		t.Run(fmt.Sprintf("invalid %q", format), func(t *testing.T) {
			_, err := planFormat(format)
			if err == nil || !strings.Contains(err.Error(), "invalid format string") {
				t.Fatalf("got %v, want invalid format syntax", err)
			}
		})
	}
	for _, format := range []string{"{0}", "{1}", "{name}", "{ }", "{:}", "{:?}", "{:x}", "{:02}", "{:>2}", "{:+}", "{:#}", "{:.1}", "{:.*}", "{0:1$}", "{雪}"} {
		t.Run(fmt.Sprintf("unsupported %q", format), func(t *testing.T) {
			_, err := planFormat(format)
			if err == nil || !strings.Contains(err.Error(), "unsupported format string: only {}") {
				t.Fatalf("got %v, want explicit unsupported-profile error", err)
			}
		})
	}
}

func TestFormatDiagnosticsUseLiteralSourceStart(t *testing.T) {
	for _, format := range []string{"{:?}", "{"} {
		err := Check("fn main() {\n    println!(\"" + format + "\", 1);\n}")
		if err == nil || !strings.HasPrefix(err.Error(), "2:14:") {
			t.Fatalf("format %q: got %v, want original literal source-start location", format, err)
		}
	}
}

func TestEmitterConsumesCheckedFormatPlan(t *testing.T) {
	c, err := analyze(`fn main() { print!("before{{{}}}after", 42); println!(); }`)
	if err != nil {
		t.Fatal(err)
	}
	if len(c.formats) != 2 {
		t.Fatalf("got %d plans, want print and empty println", len(c.formats))
	}
	before := emitChecked(c)
	for macro := range c.formats {
		if len(macro.Args) != 0 {
			macro.Args[0].Text = "UNPARSED_FORMAT_MARKER"
		}
	}
	output := emitChecked(c)
	if output != before {
		t.Fatal("emitter did not preserve the checked format plan")
	}
}

func TestNativeCheckedFormatPlans(t *testing.T) {
	cases := []struct{ name, source, stdout string }{
		{"adjacent escapes", `fn main() { println!("{{}}{{{}}}{}{{{{}}}}", 7, 8); }`, "{}{7}8{{}}\n"},
		{"empty formats", `fn main() { print!(""); println!(""); println!(); print!("end"); }`, "\n\nend"},
		{"unicode multiline and nul", "fn main() { println!(\"α\\0{{{}}}\n尾\", \"\\0β\"); }", "α\x00{\x00β}\n尾\n"},
		{"arguments once before literal output", `
fn mark(x: i32) -> i32 { print!("{}", x); x }
fn main() { println!("L{}M{}R", mark(1), mark(2)); }
`, "12L1M2R\n"},
		{"short circuit arguments", `
fn noisy() -> bool { print!("unexpected"); true }
fn main() { println!("{} {}", false && noisy(), true || noisy()); }
`, "false true\n"},
		{"divergent argument stops later arguments and output", `
fn later() -> i32 { print!("unexpected"); 2 }
fn emit() { print!("prefix {} {}", { print!("A"); return; 0 }, later()); }
fn main() { emit(); println!("done"); }
`, "Adone\n"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := runNative(t, tt.source)
			if got.status != 0 || got.stdout != tt.stdout || got.stderr != "" {
				t.Fatalf("got stdout=%q stderr=%q status=%d, want stdout=%q status=0", got.stdout, got.stderr, got.status, tt.stdout)
			}
		})
	}
}
