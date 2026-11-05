// External formatter observer. Values call their complete unchanged original
// Debug/Display implementations; no original algorithm is restated here.
extern crate rustc_span;
extern crate rustc_hashes;
use rustc_span::*;
use rustc_hashes::Hash64;
use std::{fmt,fmt::Write,rc::Rc,cell::RefCell,sync::Arc,path::Path};
fn hex(bytes:&[u8])->String{let mut s=String::new();for b in bytes{write!(s,"{b:02x}").unwrap();}s}
#[derive(Default)]struct Sink{calls:Vec<(char,String)>,accepted:String,fail:usize,chars:bool}
impl fmt::Write for Sink{
 fn write_str(&mut self,s:&str)->fmt::Result{let n=self.calls.len();self.calls.push(('S',s.to_owned()));if n==self.fail{return Err(fmt::Error)};self.accepted.push_str(s);Ok(())}
 fn write_char(&mut self,c:char)->fmt::Result{if !self.chars{return self.write_str(c.encode_utf8(&mut[0;4]))};let n=self.calls.len();self.calls.push(('C',c.to_string()));if n==self.fail{return Err(fmt::Error)};self.accepted.push(c);Ok(())}
}
type State=(u32,u8);
fn state(f:&fmt::Formatter<'_>)->State{(f.fill()as u32,match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3})}
struct Probe<'a>{value:&'a dyn fmt::Debug,states:Rc<RefCell<Vec<State>>>}
impl fmt::Debug for Probe<'_>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{self.states.borrow_mut().push(state(f));let result=self.value.fmt(f);self.states.borrow_mut().push(state(f));result}}
fn format(value:&dyn fmt::Debug,mode:u8,chars:bool,fail:usize)->(bool,Sink,Vec<State>){let states=Rc::new(RefCell::new(vec![]));let probe=Probe{value,states:states.clone()};let mut sink=Sink{fail,chars,..Default::default()};let result=match mode{0=>write!(&mut sink,"{probe:?}"),1=>write!(&mut sink,"{probe:#?}"),2=>write!(&mut sink,"{probe:x?}"),3=>write!(&mut sink,"{probe:#x?}"),4=>write!(&mut sink,"{probe:X?}"),5=>write!(&mut sink,"{probe:#X?}"),6=>write!(&mut sink,"{probe:+08?}"),7=>write!(&mut sink,"{probe:08x?}"),8=>write!(&mut sink,"{probe:*>18?}"),9=>write!(&mut sink,"{probe:.2?}"),10=>write!(&mut sink,"{probe:🔥^20?}"),11=>write!(&mut sink,"{probe:+#018x?}"),_=>unreachable!()};let state=states.borrow().clone();(result.is_ok(),sink,state)}
fn run(name:&str,value:&dyn fmt::Debug,modes:u8){for mode in 0..modes{for chars in [false,true]{let n=format(value,mode,chars,usize::MAX).1.calls.len();for fail in 0..=n{let(ok,sink,states)=format(value,mode,chars,fail);let calls=sink.calls.iter().map(|(kind,text)|format!("{kind}:{}",hex(text.as_bytes()))).collect::<Vec<_>>().join(",");let state=states.iter().map(|(fill,align)|format!("{fill}:{align}")).collect::<Vec<_>>().join(">");println!("{name}\t{mode}\t{chars}\t{fail}\t{ok}\t{state}\t{}\t{calls}",hex(sink.accepted.as_bytes()));}}}}
fn main(){create_default_session_globals_then(||{
 for bits in 0..=255u8{run(&format!("scope/{bits}"),&RemapPathScopeComponents::from_bits_retain(bits),2);}
 let real=RealFileName::from_virtual_path(Path::new("/réel/\"file.rs"));run("real/empty",&RealFileName::empty(),12);run("real/virtual",&real,12);
 let filenames=[FileName::Real(real.clone()),FileName::CfgSpec(Hash64::new(0xfedcba9876543210)),FileName::Anon(Hash64::ZERO),FileName::MacroExpansion(Hash64::new(31)),FileName::ProcMacroSourceCode(Hash64::new(26)),FileName::CliCrateAttr(Hash64::new(10)),FileName::Custom("é\n\0\u{323af}".to_owned()),FileName::DocTest(real.clone(),-26),FileName::InlineAsm(Hash64::new(u64::MAX))];for(i,value)in filenames.iter().enumerate(){run(&format!("filename/{i}"),value,12);}
 for(i,kind)in[SourceFileHashAlgorithm::Md5,SourceFileHashAlgorithm::Sha1,SourceFileHashAlgorithm::Sha256,SourceFileHashAlgorithm::Blake3].iter().enumerate(){run(&format!("algorithm/{i}"),kind,12);run(&format!("hash/{i}"),&SourceFileHash::new_in_memory(*kind,b"source\x00\xff"),12);}
 for(i,value)in[ExternalSourceKind::Present(Arc::new("é\n\0\u{323af}".to_owned())),ExternalSourceKind::AbsentOk,ExternalSourceKind::AbsentErr].iter().enumerate(){run(&format!("external-kind/{i}"),value,12);run(&format!("external/{i}"),&ExternalSource::Foreign{kind:value.clone(),metadata_index:u32::MAX-10},12);}run("external/unneeded",&ExternalSource::Unneeded,12);
 run("bytepos",&BytePos(u32::MAX-10),12);run("relativepos",&RelativeBytePos(u32::MAX-10),12);run("charpos",&CharPos(usize::MAX-10),12);run("multibyte",&MultiByteChar{pos:RelativeBytePos(u32::MAX-10),bytes:255},12);run("normalized",&NormalizedPos{pos:RelativeBytePos(u32::MAX-10),diff:u32::MAX-26},12);
 let sf=Arc::new(SourceFile::new(FileName::Custom("é\n".to_owned()),"abcdef\n".to_owned(),SourceFileHashAlgorithm::Sha256,None).unwrap());run("sourcefile",&*sf,12);run("stableid",&sf.stable_id,12);run("loc",&Loc{file:sf.clone(),line:usize::MAX-10,col:CharPos(31),col_display:26},12);run("sfline",&SourceFileAndLine{sf:sf.clone(),line:usize::MAX-10},12);run("sfpos",&SourceFileAndBytePos{sf:sf.clone(),pos:BytePos(u32::MAX-10)},12);run("lineinfo",&LineInfo{line_index:usize::MAX-10,start_col:CharPos(31),end_col:CharPos(26)},12);run("inner",&InnerSpan::new(usize::MAX-10,31),12);run("overflow",&OffsetOverflowError,12);
 #[allow(deprecated)]let guaranteed=ErrorGuaranteed::unchecked_error_guaranteed();run("guaranteed",&guaranteed,12);run("attr",&AttrId::from_u32(u32::MAX-255),12);
 let span=Span::with_root_ctxt(BytePos(10),BytePos(31));run("spanned",&respan(span,-26i64),12);
 let distinct=DistinctSources{begin:(filenames[6].clone(),BytePos(10)),end:(filenames[8].clone(),BytePos(31))};run("distinct",&distinct,12);run("lines-error",&SpanLinesError::DistinctSources(Box::new(distinct.clone())),12);run("snippet-ill",&SpanSnippetError::IllFormedSpan(span),12);run("snippet-distinct",&SpanSnippetError::DistinctSources(Box::new(distinct.clone())),12);let malformed=MalformedSourceMapPositions{name:filenames[6].clone(),source_len:usize::MAX-10,begin_pos:BytePos(10),end_pos:BytePos(31)};run("malformed",&malformed,12);run("snippet-malformed",&SpanSnippetError::MalformedForSourcemap(malformed.clone()),12);run("snippet-unavailable",&SpanSnippetError::SourceNotAvailable{filename:filenames[6].clone()},12);
});}
