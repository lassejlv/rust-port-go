//! Actual official SDK Unicode18 Debug and public escape_debug operations.
use std::fmt::{self,Write};
use std::io::{self,Write as IoWrite};
#[derive(Default)]struct Trace{calls:u32,data:Vec<u8>}
fn number(out:&mut Vec<u8>,n:u32){out.extend_from_slice(&n.to_le_bytes())}
fn text(out:&mut Vec<u8>,s:&str){number(out,s.len().try_into().unwrap());out.extend_from_slice(s.as_bytes())}
impl fmt::Write for Trace{
 fn write_str(&mut self,s:&str)->fmt::Result{self.calls+=1;self.data.push(b'S');text(&mut self.data,s);Ok(())}
 fn write_char(&mut self,c:char)->fmt::Result{self.calls+=1;self.data.push(b'C');number(&mut self.data,c as u32);Ok(())}
}
impl Trace{fn append(&mut self,out:&mut Vec<u8>){number(out,self.calls);out.extend_from_slice(&self.data);self.calls=0;self.data.clear()}}
fn main()->io::Result<()>{
 assert_eq!(std::char::UNICODE_VERSION,(18,0,0));
 let mut output=io::BufWriter::new(io::stdout().lock());let mut trace=Trace::default();
 for start in (0..0x110000u32).step_by(4096){
  let mut block=Vec::new();let mut scalars=0;
  for point in start..start+4096{
   let Some(c)=char::from_u32(point)else{continue};scalars+=1;number(&mut block,point);
   write!(&mut trace,"{c:?}").unwrap();trace.append(&mut block);
   let s=c.to_string();write!(&mut trace,"{s:?}").unwrap();trace.append(&mut block);
   text(&mut block,&c.escape_debug().to_string());
  }
  output.write_all(&start.to_le_bytes())?;output.write_all(&u32::to_le_bytes(scalars))?;
  output.write_all(&u32::try_from(block.len()).unwrap().to_le_bytes())?;output.write_all(&block)?;
 }
 output.flush()
}
