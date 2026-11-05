// External test-data helper only. The complete original SourceFile encoder and
// decoder perform construction; this merely mutates their public byte input.
// No private SourceFileLines/Diffs value or source algorithm is fabricated.
use rustc_span::{FileName,RelativeBytePos,SourceFile,SourceFileHash,SourceFileHashAlgorithm};
use rustc_serialize::{Decodable,Decoder,Encodable,Encoder};
use rustc_serialize::opaque::{MemDecoder,MAGIC_END_BYTES,mem_encoder::MemEncoder};

pub fn source_with_encoded_line_diffs(num_lines:u32,bytes_per_diff:u8,raw_diffs:&[u8])->SourceFile{
    let source=SourceFile::new(FileName::Custom("lazy-lines".to_owned()),"x".to_owned(),SourceFileHashAlgorithm::Sha256,None).unwrap();
    let mut encoder=MemEncoder::new();source.encode(&mut encoder);let body=encoder.finish();
    let mut original_frame=body.clone();original_frame.extend_from_slice(MAGIC_END_BYTES);
    let mut locate=MemDecoder::new(&original_frame,0).unwrap();
    let _:FileName=Decodable::decode(&mut locate);
    let _:SourceFileHash=Decodable::decode(&mut locate);
    let _:Option<SourceFileHash>=Decodable::decode(&mut locate);
    let _:RelativeBytePos=Decodable::decode(&mut locate);
    let _:u32=Decodable::decode(&mut locate);
    let start=locate.position();let original_lines=locate.read_u32();
    if original_lines>0{let width=locate.read_u8()as usize;locate.read_raw_bytes(width*(original_lines as usize-1));}
    let end=locate.position();
    let mut replacement=MemEncoder::new();replacement.emit_u32(num_lines);
    if num_lines>0{replacement.emit_u8(bytes_per_diff);replacement.emit_raw_bytes(raw_diffs);}
    let mut frame=body[..start].to_vec();frame.extend_from_slice(&replacement.finish());frame.extend_from_slice(&body[end..]);frame.extend_from_slice(MAGIC_END_BYTES);
    SourceFile::decode(&mut MemDecoder::new(&frame,0).unwrap())
}
