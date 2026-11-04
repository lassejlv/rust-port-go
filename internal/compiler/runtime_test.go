package compiler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Set FERRUGO_RUNTIME_SANITIZE=1 to additionally exercise these same checks with
// the host C compiler's undefined-behavior sanitizer. Normal tests require C11.
func compileRuntimeTest(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	source := filepath.Join(dir, "runtime.c")
	output := filepath.Join(dir, "runtime")
	if err := os.WriteFile(source, []byte(runtimeC+body), 0600); err != nil {
		t.Fatal(err)
	}
	cc := os.Getenv("CC")
	if cc == "" {
		cc = "cc"
	}
	args := []string{"-std=c11", "-O2", source, "-o", output}
	if os.Getenv("FERRUGO_RUNTIME_SANITIZE") == "1" {
		args = append([]string{"-fsanitize=undefined", "-fno-sanitize-recover=all"}, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if diagnostics, err := exec.CommandContext(ctx, cc, args...).CombinedOutput(); err != nil {
		t.Fatalf("compile C runtime: %v\n%s", err, diagnostics)
	}
	return output
}

func runRuntimeTest(t *testing.T, path string, args ...string) ([]byte, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, args...).CombinedOutput()
	if ctx.Err() != nil {
		t.Fatalf("C runtime execution timed out: %v", ctx.Err())
	}
	if err == nil {
		return output, 0
	}
	var exited *exec.ExitError
	if !errors.As(err, &exited) {
		t.Fatalf("run C runtime: %v", err)
	}
	return output, exited.ExitCode()
}

func TestRuntimeIntegerArithmeticAndShiftEdges(t *testing.T) {
	program := compileRuntimeTest(t, `
#define VERIFY(condition) do { if (!(condition)) { fprintf(stderr,"runtime condition failed at line %d: %s\n",__LINE__,#condition); return 1; } } while (0)
#define CHECK_SIGNED_EDGES(T, NAME, MIN, MAX) do { \
 VERIFY(fr_add_##NAME(MIN,MAX)==-1); \
 VERIFY(fr_add_##NAME(MIN+1,-1)==MIN); \
 VERIFY(fr_sub_##NAME(MIN,MIN)==0); \
 VERIFY(fr_mul_##NAME(MIN,1)==MIN); \
 VERIFY(fr_mul_##NAME(MAX,0)==0); \
 VERIFY(fr_div_##NAME(MIN,1)==MIN); \
 VERIFY(fr_rem_##NAME(MIN,1)==0); \
 VERIFY(fr_neg_##NAME(MAX)==-MAX); \
 VERIFY(fr_neg_##NAME(MIN+1)==MAX); \
} while (0)
#define CHECK_UNSIGNED_EDGES(T, NAME, MAX) do { \
 VERIFY(fr_add_##NAME(MAX,0)==MAX); \
 VERIFY(fr_add_##NAME(MAX-1,1)==MAX); \
 VERIFY(fr_sub_##NAME(MAX,MAX)==0); \
 VERIFY(fr_sub_##NAME(MAX,MAX-1)==1); \
 VERIFY(fr_mul_##NAME(MAX,1)==MAX); \
 VERIFY(fr_mul_##NAME(0,MAX)==0); \
 VERIFY(fr_div_##NAME(MAX,1)==MAX); \
 VERIFY(fr_rem_##NAME(MAX,1)==0); \
} while (0)
#define CHECK_SIGNED_SHIFTS(T, U, NAME, MIN, MAX) do { \
 const T values[]={MIN,MIN+1,-7,-1,0,1,7,MAX-1,MAX}; \
 const uint64_t width=sizeof(T)*CHAR_BIT; \
 const uint64_t mask=UINT64_MAX>>(64-width); \
 for (size_t index=0;index<sizeof(values)/sizeof(values[0]);index++) { \
  T value=values[index]; \
  for (uint64_t count=0;count<width;count++) { \
   uint64_t divisor=UINT64_C(1)<<count; \
   uint64_t left=((uint64_t)(U)value<<count)&mask; \
   uint64_t right; \
   if (value>=0) right=(uint64_t)value/divisor; \
   else { uint64_t magnitude=UINT64_C(0)-(uint64_t)value; uint64_t rounded=magnitude/divisor+(magnitude%divisor!=0); right=(UINT64_C(0)-rounded)&mask; } \
   VERIFY((uint64_t)(U)fr_shl_##NAME(value,count)==left); \
   VERIFY((uint64_t)(U)fr_shr_##NAME(value,count)==right); \
  } \
 } \
} while (0)
#define CHECK_UNSIGNED_SHIFTS(T, NAME, MAX) do { \
 const T values[]={0,1,7,MAX/2,MAX-1,MAX}; \
 const uint64_t width=sizeof(T)*CHAR_BIT; \
 const uint64_t mask=UINT64_MAX>>(64-width); \
 for (size_t index=0;index<sizeof(values)/sizeof(values[0]);index++) { \
  T value=values[index]; \
  for (uint64_t count=0;count<width;count++) { \
   VERIFY((uint64_t)fr_shl_##NAME(value,count)==(((uint64_t)value<<count)&mask)); \
   VERIFY((uint64_t)fr_shr_##NAME(value,count)==(uint64_t)value/(UINT64_C(1)<<count)); \
  } \
 } \
} while (0)
int main(void) {
 CHECK_SIGNED_EDGES(int8_t,i8,INT8_MIN,INT8_MAX);
 CHECK_SIGNED_EDGES(int16_t,i16,INT16_MIN,INT16_MAX);
 CHECK_SIGNED_EDGES(int32_t,i32,INT32_MIN,INT32_MAX);
 CHECK_SIGNED_EDGES(int64_t,i64,INT64_MIN,INT64_MAX);
 CHECK_SIGNED_EDGES(intptr_t,isize,INTPTR_MIN,INTPTR_MAX);
 CHECK_UNSIGNED_EDGES(uint8_t,u8,UINT8_MAX);
 CHECK_UNSIGNED_EDGES(uint16_t,u16,UINT16_MAX);
 CHECK_UNSIGNED_EDGES(uint32_t,u32,UINT32_MAX);
 CHECK_UNSIGNED_EDGES(uint64_t,u64,UINT64_MAX);
 CHECK_UNSIGNED_EDGES(uintptr_t,usize,UINTPTR_MAX);
 CHECK_SIGNED_SHIFTS(int8_t,uint8_t,i8,INT8_MIN,INT8_MAX);
 CHECK_SIGNED_SHIFTS(int16_t,uint16_t,i16,INT16_MIN,INT16_MAX);
 CHECK_SIGNED_SHIFTS(int32_t,uint32_t,i32,INT32_MIN,INT32_MAX);
 CHECK_SIGNED_SHIFTS(int64_t,uint64_t,i64,INT64_MIN,INT64_MAX);
 CHECK_SIGNED_SHIFTS(intptr_t,uintptr_t,isize,INTPTR_MIN,INTPTR_MAX);
 CHECK_UNSIGNED_SHIFTS(uint8_t,u8,UINT8_MAX);
 CHECK_UNSIGNED_SHIFTS(uint16_t,u16,UINT16_MAX);
 CHECK_UNSIGNED_SHIFTS(uint32_t,u32,UINT32_MAX);
 CHECK_UNSIGNED_SHIFTS(uint64_t,u64,UINT64_MAX);
 CHECK_UNSIGNED_SHIFTS(uintptr_t,usize,UINTPTR_MAX);
 /* Exhaust every i8/u8 input pair, comparing valid arithmetic with wider
    mathematical results. Invalid-result cases run separately to check traps. */
 for (int a=-128;a<=127;a++) for (int b=-128;b<=127;b++) {
  int sum=a+b, difference=a-b, product=a*b;
  if (sum>=-128 && sum<=127) VERIFY(fr_add_i8(a,b)==sum);
  if (difference>=-128 && difference<=127) VERIFY(fr_sub_i8(a,b)==difference);
  if (product>=-128 && product<=127) VERIFY(fr_mul_i8(a,b)==product);
  if (b!=0 && !(a==-128 && b==-1)) { VERIFY(fr_div_i8(a,b)==a/b); VERIFY(fr_rem_i8(a,b)==a%b); }
 }
 for (unsigned a=0;a<=255;a++) for (unsigned b=0;b<=255;b++) {
  unsigned sum=a+b, product=a*b;
  if (sum<=255) VERIFY(fr_add_u8(a,b)==sum);
  if (a>=b) VERIFY(fr_sub_u8(a,b)==a-b);
  if (product<=255) VERIFY(fr_mul_u8(a,b)==product);
  if (b!=0) { VERIFY(fr_div_u8(a,b)==a/b); VERIFY(fr_rem_u8(a,b)==a%b); }
 }
 VERIFY(fr_mul_u64(UINT64_MAX/3,3)==UINT64_MAX);
 VERIFY(fr_mul_i64(INT64_MIN/2,2)==INT64_MIN);
 VERIFY(fr_mul_i64(INT64_MAX/2,2)==INT64_MAX-1);
 VERIFY(fr_shl_i64(INT64_MAX,1)==-2);
 VERIFY(fr_shl_i64(INT64_MIN,1)==0);
 VERIFY(fr_shr_i64(INT64_MIN,63)==-1);
 VERIFY(fr_shl_u64(UINT64_MAX,63)==(UINT64_C(1)<<63));
 puts("runtime integer checks passed");
 return 0;
}
`)
	output, status := runRuntimeTest(t, program)
	if status != 0 || string(output) != "runtime integer checks passed\n" {
		t.Fatalf("runtime checks status=%d output=%q", status, output)
	}
}

func TestRuntimeIntegerFailuresExit101(t *testing.T) {
	type integerCase struct{ name, typ, min, max string }
	signed := []integerCase{
		{"i8", "int8_t", "INT8_MIN", "INT8_MAX"},
		{"i16", "int16_t", "INT16_MIN", "INT16_MAX"},
		{"i32", "int32_t", "INT32_MIN", "INT32_MAX"},
		{"i64", "int64_t", "INT64_MIN", "INT64_MAX"},
		{"isize", "intptr_t", "INTPTR_MIN", "INTPTR_MAX"},
	}
	unsigned := []integerCase{
		{"u8", "uint8_t", "0", "UINT8_MAX"},
		{"u16", "uint16_t", "0", "UINT16_MAX"},
		{"u32", "uint32_t", "0", "UINT32_MAX"},
		{"u64", "uint64_t", "0", "UINT64_MAX"},
		{"usize", "uintptr_t", "0", "UINTPTR_MAX"},
	}
	type trap struct{ name, expression string }
	var cases []trap
	for _, integer := range signed {
		for _, operation := range []struct{ name, args string }{
			{"add", integer.max + ",1"}, {"sub", integer.min + ",1"}, {"mul", integer.min + ",-1"},
			{"div", integer.min + ",-1"}, {"rem", integer.min + ",-1"}, {"neg", integer.min},
			{"div", "1,0"}, {"rem", "1,0"},
			{"shl", "1,sizeof(" + integer.typ + ")*CHAR_BIT"}, {"shr", "1,sizeof(" + integer.typ + ")*CHAR_BIT"},
			{"shl", "1,(uint64_t)-1"}, {"shr", "1,(uint64_t)-1"},
		} {
			cases = append(cases, trap{integer.name + "/" + operation.name + "/" + operation.args, "fr_" + operation.name + "_" + integer.name + "(" + operation.args + ")"})
		}
	}
	for _, integer := range unsigned {
		for _, operation := range []struct{ name, args string }{
			{"add", integer.max + ",1"}, {"sub", "0,1"}, {"mul", integer.max + ",2"},
			{"div", "1,0"}, {"rem", "1,0"},
			{"shl", "1,sizeof(" + integer.typ + ")*CHAR_BIT"}, {"shr", "1,sizeof(" + integer.typ + ")*CHAR_BIT"},
			{"shl", "1,(uint64_t)-1"}, {"shr", "1,(uint64_t)-1"},
		} {
			cases = append(cases, trap{integer.name + "/" + operation.name + "/" + operation.args, "fr_" + operation.name + "_" + integer.name + "(" + operation.args + ")"})
		}
	}
	var body strings.Builder
	body.WriteString("\nint main(int argc,char **argv) { if(argc!=2) return 2; switch(atoi(argv[1])) {\n")
	for i, test := range cases {
		fmt.Fprintf(&body, "case %d: (void)%s; break;\n", i, test.expression)
	}
	body.WriteString("default: return 3; } return 0; }\n")
	program := compileRuntimeTest(t, body.String())
	for i, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			output, status := runRuntimeTest(t, program, fmt.Sprint(i))
			if status != 101 || !bytes.Contains(output, []byte("Ferrugo panic:")) {
				t.Fatalf("status=%d output=%q", status, output)
			}
		})
	}
}
