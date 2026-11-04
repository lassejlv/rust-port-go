package compiler

import (
	"strings"
	"testing"
)

func TestNativeArrayStorageAndBorrowedProjectionRegressions(t *testing.T) {
	cases := []struct{ name, source, want string }{
		{"large zero sized nest", `fn main(){let a=[[[0u8;0];4096];4096];println!("{}",a.len());}`, "4096\n"},
		{"repeated borrowed length", `fn main(){let a=[0u64;65536];` + strings.Repeat(`println!("{}",a.len());`, 20) + `}`, strings.Repeat("65536\n", 20)},
		{"repeated borrowed scalar field", `#[derive(Copy,Clone)]struct S{big:[u64;65536],n:i32}fn main(){let s=S{big:[0;65536],n:7};` + strings.Repeat(`println!("{}",s.n);`, 20) + `}`, strings.Repeat("7\n", 20)},
		{"omitted zero sized fields retain effects", `
#[derive(Copy,Clone)]struct Z{}
#[derive(Copy,Clone)]struct Container{zs:[Z;5],empty:[[u8;0];4096],n:i32}
fn mark()->Z{print!("Z");Z{}}
fn read(z:Z)->i32{3}
fn main(){
 let mut c=Container{zs:[mark();5],empty:[[0u8;0];4096],n:7};
 c.zs[{print!("I");0usize}]=mark();
 println!("{} {} {}",c.zs.len(),c.empty.len(),c.n);
 println!("{}",read(c.zs[{print!("J");1usize}]));
}`, "ZZI5 4096 7\nJ3\n"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := runNative(t, tt.source)
			if got.status != 0 || got.stdout != tt.want {
				t.Fatalf("got%+v want%q", got, tt.want)
			}
		})
	}
}

func TestZeroLengthArrayOfLargeElementsPanicsBeforeStorage(t *testing.T) {
	got := runNative(t, `fn index()->usize{print!("I");0}fn main(){let a:[[u64;65536];0]=[];println!("{}",a[index()].len());}`)
	if got.status != 101 || got.stdout != "I" {
		t.Fatalf("got%+v wantstatus101 withindexsideeffect", got)
	}
}
