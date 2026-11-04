// Reference-only witness for the standard Hash input conversions in tests.rs.
// It does not supply a production hashing algorithm.
use std::hash::{Hash, Hasher};

fn hex(bytes:&[u8])->String {bytes.iter().map(|b|format!("{b:02x}")).collect()}
#[derive(Default)]
struct Trace(Vec<String>);
macro_rules! integer {
    ($name:ident,$ty:ty)=>{fn $name(&mut self,value:$ty){self.0.push(format!("{}:{value}",stringify!($ty)));}}
}
impl Hasher for Trace {
    fn finish(&self)->u64 {0}
    fn write(&mut self,bytes:&[u8]){self.0.push(format!("bytes:{}",hex(bytes)));}
    integer!(write_u8,u8);integer!(write_u16,u16);integer!(write_u32,u32);integer!(write_u64,u64);integer!(write_u128,u128);integer!(write_usize,usize);
    integer!(write_i8,i8);integer!(write_i16,i16);integer!(write_i32,i32);integer!(write_i64,i64);integer!(write_i128,i128);integer!(write_isize,isize);
}
fn trace<T:Hash>(name:&str,value:&T) {let mut hasher=Trace::default();value.hash(&mut hasher);println!("{name}\t{}",hasher.0.join("|"));}
fn main() {
    trace("strings",&("aa","bb"));
    let u=[1_i32,0,0,0];
    trace("slices",&(&u[..1],&u[1..3],&u[3..]));
    trace("wide_slice",&(&u[..],&u[4..4],&u[4..4]));
    trace("isizes",&(-1_isize,1_isize));
}
