// External test client. All algorithms under comparison come from the complete
// unchanged rustc_span/rustc_serialize libraries prepared by the shared tool.
extern crate rustc_span;
extern crate rustc_serialize;
use rustc_span::{FileName,SourceFile,SourceFileHashAlgorithm};
use rustc_serialize::{Encodable,Decodable};
use std::fmt::Write;

fn hex(bytes:&[u8])->String { let mut s=String::new();for b in bytes {write!(s,"{b:02x}").unwrap();}s }
fn main(){
 assert_eq!((std::mem::size_of::<rustc_span::Span>(),std::mem::align_of::<rustc_span::Span>()),(8,4));
 assert_eq!((std::mem::size_of::<(rustc_span::Span,bool)>(),std::mem::align_of::<(rustc_span::Span,bool)>()),(12,4));
 assert_eq!((std::mem::size_of::<(rustc_span::Span,(rustc_span::Span,bool))>(),std::mem::align_of::<(rustc_span::Span,(rustc_span::Span,bool))>()),(20,4));
 assert_eq!((std::mem::size_of::<(rustc_span::Span,rustc_span::Span)>(),std::mem::align_of::<(rustc_span::Span,rustc_span::Span)>()),(16,4));
 let mut inputs:Vec<String>=["","\n","\r","\r\n","\r\r\n","\u{feff}","\u{feff}hello\r\nferries\r\n","hello\nworld","é😀界\t\0\n\u{202e}","\r\n\u{feff}\r\n\r\n"].iter().map(|s|s.to_string()).collect();
 let alphabet=['a','b','\r','\n','\u{feff}','é','😀','界','\0','\u{1b}','\u{10ffff}','\u{301}','\t'];let mut random=0xb5396a7785d40321u64;
 for _ in 0..2048{random=random.wrapping_mul(6364136223846793005).wrapping_add(1442695040888963407);let n=(random%48)as usize;let mut s=String::new();for _ in 0..n{random=random.wrapping_mul(6364136223846793005).wrapping_add(1442695040888963407);s.push(alphabet[(random%alphabet.len()as u64)as usize]);}inputs.push(s);}
 for (i,src) in inputs.iter().enumerate(){for kind in [SourceFileHashAlgorithm::Md5,SourceFileHashAlgorithm::Sha1,SourceFileHashAlgorithm::Sha256,SourceFileHashAlgorithm::Blake3]{
  let sf=SourceFile::new(FileName::anon_source_code(src),src.clone(),kind,Some(kind)).unwrap();
  let mut text=format!("{};{};{};{};{:?};{:?};{:?};{:?};{};{}",sf.unnormalized_source_len,sf.normalized_source_len.0,hex(sf.src.as_ref().unwrap().as_bytes()),sf.src_hash,sf.stable_id,sf.lines(),sf.multibyte_chars,sf.normalized_pos,sf.multibyte_chars.capacity(),sf.normalized_pos.capacity());
  for index in 0..sf.count_lines()+2{match sf.get_line(index){Some(line)=>{let owned=matches!(line,std::borrow::Cow::Owned(_));write!(text,";L{index}:{owned}:{}",hex(line.as_bytes())).unwrap();},None=>{write!(text,";L{index}:-").unwrap();}}}
  let normalized=sf.src.as_ref().unwrap();for pos in 0..=normalized.len(){if normalized.is_char_boundary(pos){let (line,col,display)=sf.lookup_file_pos_with_col_display(rustc_span::BytePos(pos as u32));write!(text,";P{pos}:{line}:{}:{display}",col.0).unwrap();}}
  let clone=sf.clone();write!(text,";C{}:{}:{}",clone.multibyte_chars.capacity(),clone.normalized_pos.capacity(),std::sync::Arc::strong_count(sf.src.as_ref().unwrap())).unwrap();
  let mut encoder=rustc_serialize::opaque::mem_encoder::MemEncoder::new();sf.encode(&mut encoder);let raw=encoder.finish();write!(text,";E{}",hex(&raw)).unwrap();
  let mut frame=raw;frame.extend_from_slice(rustc_serialize::opaque::MAGIC_END_BYTES);let mut decoder=rustc_serialize::opaque::MemDecoder::new(&frame,0).unwrap();let decoded=SourceFile::decode(&mut decoder);
  write!(text,";D{}:{}:{}:{}:{}:{:?}",decoded.src.is_none(),decoded.lines.is_frozen(),decoded.multibyte_chars.capacity(),decoded.normalized_pos.capacity(),decoded.external_src.is_frozen(),decoded.lines()).unwrap();
  println!("{i}\t{kind}\t{}\t{}",hex(src.as_bytes()),hex(text.as_bytes()));
 }}
}
