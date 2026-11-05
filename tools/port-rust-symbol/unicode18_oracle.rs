//! Actual matching-nightly std methods used by the unchanged Symbol file.
use std::fmt::{self, Write};
use std::io::{self, Write as IoWrite};

#[derive(Default)]
struct Trace { count:u32, data:Vec<u8> }
fn u32le(output:&mut Vec<u8>,value:u32){output.extend_from_slice(&value.to_le_bytes())}
fn string(output:&mut Vec<u8>,value:&str){u32le(output,value.len().try_into().unwrap());output.extend_from_slice(value.as_bytes())}
impl fmt::Write for Trace {
    fn write_str(&mut self,value:&str)->fmt::Result {self.count+=1;self.data.push(b'S');string(&mut self.data,value);Ok(())}
    fn write_char(&mut self,value:char)->fmt::Result {self.count+=1;self.data.push(b'C');u32le(&mut self.data,value as u32);Ok(())}
}
impl Trace {
    fn append(&mut self,output:&mut Vec<u8>){u32le(output,self.count);output.extend_from_slice(&self.data);self.count=0;self.data.clear()}
}
fn main()->io::Result<()> {
    assert_eq!(std::char::UNICODE_VERSION,(18,0,0));
    let mut stdout=io::BufWriter::new(io::stdout().lock());
    let mut trace=Trace::default();
    for start in (0..0x110000u32).step_by(4096) {
        let mut output=Vec::new();let mut scalars=0u32;
        for point in start..start+4096 {
            let Some(c)=char::from_u32(point) else{continue};scalars+=1;u32le(&mut output,point);
            let text=c.to_string();
            write!(&mut trace,"{c:?}").unwrap();trace.append(&mut output);
            write!(&mut trace,"{text:?}").unwrap();trace.append(&mut output);
            let canonical=c.escape_debug().to_string();
            for flags in 0..4 {
                // Only the two quote branches depend on these flags. Public
                // escape_debug uses both flags; the original private method's
                // two independent quote guards supply the other combinations.
                let escape=if (c=='\'' && flags&1==0)||(c=='"' && flags&2==0){&text}else{&canonical};
                string(&mut output,escape);
            }
            string(&mut output,&text.to_lowercase());
            string(&mut output,&text.to_uppercase());
            string(&mut output,&format!("A{c}Σ").to_lowercase());
            string(&mut output,&format!("AΣ{c}").to_lowercase());
        }
        stdout.write_all(&start.to_le_bytes())?;stdout.write_all(&scalars.to_le_bytes())?;
        stdout.write_all(&u32::try_from(output.len()).unwrap().to_le_bytes())?;stdout.write_all(&output)?;
    }
    stdout.flush()
}
