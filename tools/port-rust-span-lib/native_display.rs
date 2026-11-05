// External formatter observer. Values call their complete unchanged original
// Debug/Display implementations; no original algorithm is restated here.
extern crate rustc_span;
extern crate rustc_hashes;
use rustc_span::*;
use rustc_hashes::Hash64;
use std::{fmt,fmt::Write,rc::Rc,cell::RefCell,path::Path};
fn hex(bytes:&[u8])->String{let mut s=String::new();for b in bytes{write!(s,"{b:02x}").unwrap();}s}
#[derive(Default)]struct Sink{calls:Vec<(char,String)>,accepted:String,fail:usize,chars:bool}
impl fmt::Write for Sink{
 fn write_str(&mut self,s:&str)->fmt::Result{let n=self.calls.len();self.calls.push(('S',s.to_owned()));if n==self.fail{return Err(fmt::Error)};self.accepted.push_str(s);Ok(())}
 fn write_char(&mut self,c:char)->fmt::Result{if !self.chars{return self.write_str(c.encode_utf8(&mut[0;4]))};let n=self.calls.len();self.calls.push(('C',c.to_string()));if n==self.fail{return Err(fmt::Error)};self.accepted.push(c);Ok(())}
}
type State=(u32,u8);
fn state(f:&fmt::Formatter<'_>)->State{(f.fill()as u32,match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3})}
struct Probe<'a>{value:&'a dyn fmt::Display,states:Rc<RefCell<Vec<State>>>}
impl fmt::Debug for Probe<'_>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{self.states.borrow_mut().push(state(f));let result=self.value.fmt(f);self.states.borrow_mut().push(state(f));result}}
fn format(value:&dyn fmt::Display,mode:u8,chars:bool,fail:usize)->(bool,Sink,Vec<State>){let states=Rc::new(RefCell::new(vec![]));let probe=Probe{value,states:states.clone()};let mut sink=Sink{fail,chars,..Default::default()};let result=match mode{0=>write!(&mut sink,"{probe:?}"),1=>write!(&mut sink,"{probe:#?}"),2=>write!(&mut sink,"{probe:x?}"),3=>write!(&mut sink,"{probe:#x?}"),4=>write!(&mut sink,"{probe:X?}"),5=>write!(&mut sink,"{probe:#X?}"),6=>write!(&mut sink,"{probe:+08?}"),7=>write!(&mut sink,"{probe:08x?}"),8=>write!(&mut sink,"{probe:*>18?}"),9=>write!(&mut sink,"{probe:.2?}"),10=>write!(&mut sink,"{probe:🔥^20?}"),11=>write!(&mut sink,"{probe:+#018x?}"),_=>unreachable!()};let state=states.borrow().clone();(result.is_ok(),sink,state)}
fn run(name:&str,value:&dyn fmt::Display,modes:u8){for mode in 0..modes{for chars in [false,true]{let n=format(value,mode,chars,usize::MAX).1.calls.len();for fail in 0..=n{let(ok,sink,states)=format(value,mode,chars,fail);let calls=sink.calls.iter().map(|(kind,text)|format!("{kind}:{}",hex(text.as_bytes()))).collect::<Vec<_>>().join(",");let state=states.iter().map(|(fill,align)|format!("{fill}:{align}")).collect::<Vec<_>>().join(">");println!("{name}\t{mode}\t{chars}\t{fail}\t{ok}\t{state}\t{}\t{calls}",hex(sink.accepted.as_bytes()));}}}}
fn main(){
 let real=RealFileName::from_virtual_path(Path::new("/réel/\"file.rs"));
 let mut filenames=vec![FileName::Real(real.clone()),FileName::CfgSpec(Hash64::new(31)),FileName::Anon(Hash64::ZERO),FileName::MacroExpansion(Hash64::new(26)),FileName::ProcMacroSourceCode(Hash64::new(10)),FileName::CliCrateAttr(Hash64::new(99)),FileName::Custom("é\n\0\u{323af}".to_owned()),FileName::DocTest(real.clone(),-26),FileName::InlineAsm(Hash64::new(u64::MAX)),FileName::Real(RealFileName::empty()),FileName::Custom(String::new())];
 #[cfg(unix)]{use std::os::unix::ffi::OsStrExt;filenames.push(FileName::Real(RealFileName::from_virtual_path(Path::new(std::ffi::OsStr::from_bytes(b"/raw/\xff.rs")))));}
 for(i,value)in filenames.iter().enumerate(){run(&format!("file/{i}/remapped"),&value.prefer_remapped_unconditionally(),12);run(&format!("file/{i}/local"),&value.prefer_local_unconditionally(),12);run(&format!("file/{i}/short"),&value.short(),12);for(scope_name,scope)in[("macro",RemapPathScopeComponents::MACRO),("diagnostics",RemapPathScopeComponents::DIAGNOSTICS),("coverage",RemapPathScopeComponents::COVERAGE)]{run(&format!("file/{i}/{scope_name}"),&value.display(scope),12);}}
 for(i,kind)in[SourceFileHashAlgorithm::Md5,SourceFileHashAlgorithm::Sha1,SourceFileHashAlgorithm::Sha256,SourceFileHashAlgorithm::Blake3].iter().enumerate(){run(&format!("algorithm/{i}"),kind,12);for(j,data)in[b"" as &[u8],b"source\0\xff",b"a"].iter().enumerate(){run(&format!("hash/{i}/{j}"),&SourceFileHash::new_in_memory(*kind,data),12);}}
}
