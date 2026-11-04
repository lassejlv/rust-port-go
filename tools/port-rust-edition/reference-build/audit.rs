use crate::*;
use rustc_serialize::opaque::mem_encoder::MemEncoder;
use rustc_serialize::opaque::MemDecoder;

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum CodecOp {
    Usize(usize), U128(u128), U64(u64), U32(u32), U16(u16), U8(u8),
    Isize(isize), I128(i128), I64(i64), I32(i32), I16(i16), RawBytes(Vec<u8>),
    ReadUsize, ReadU128, ReadU64, ReadU32, ReadU16, ReadU8,
    ReadIsize, ReadI128, ReadI64, ReadI32, ReadI16, ReadRawBytes(usize),
}

pub struct AuditEncoder {
    pub inner: MemEncoder,
    pub operations: Vec<CodecOp>,
}
impl AuditEncoder {
    pub fn new() -> Self { Self { inner: MemEncoder::new(), operations: vec![] } }
    pub fn finish(self) -> Vec<u8> { self.inner.finish() }
    pub fn into_parts(self) -> (Vec<u8>, Vec<CodecOp>) { (self.inner.finish(), self.operations) }
}

macro_rules! emit {
    ($name:ident, $ty:ty, $op:ident) => {
        fn $name(&mut self, value: $ty) {
            self.operations.push(CodecOp::$op(value));
            self.inner.$name(value);
        }
    }
}
impl Encoder for AuditEncoder {
    emit!(emit_usize, usize, Usize);
    emit!(emit_u128, u128, U128);
    emit!(emit_u64, u64, U64);
    emit!(emit_u32, u32, U32);
    emit!(emit_u16, u16, U16);
    emit!(emit_u8, u8, U8);
    emit!(emit_isize, isize, Isize);
    emit!(emit_i128, i128, I128);
    emit!(emit_i64, i64, I64);
    emit!(emit_i32, i32, I32);
    emit!(emit_i16, i16, I16);
    fn emit_raw_bytes(&mut self, bytes: &[u8]) {
        self.operations.push(CodecOp::RawBytes(bytes.to_vec()));
        self.inner.emit_raw_bytes(bytes);
    }
}
impl SpanEncoder for AuditEncoder {
    fn encode_span(&mut self, _: Span) { unused_interface("encode_span") }
    fn encode_symbol(&mut self, _: Symbol) { unused_interface("encode_symbol") }
    fn encode_byte_symbol(&mut self, _: ByteSymbol) { unused_interface("encode_byte_symbol") }
    fn encode_expn_id(&mut self, _: ExpnId) { unused_interface("encode_expn_id") }
    fn encode_syntax_context(&mut self, _: SyntaxContext) { unused_interface("encode_syntax_context") }
    fn encode_crate_num(&mut self, _: CrateNum) { unused_interface("encode_crate_num") }
    fn encode_def_index(&mut self, _: DefIndex) { unused_interface("encode_def_index") }
    fn encode_def_id(&mut self, _: DefId) { unused_interface("encode_def_id") }
}

pub struct AuditDecoder<'a> {
    pub inner: MemDecoder<'a>,
    pub operations: Vec<CodecOp>,
}
impl<'a> AuditDecoder<'a> {
    pub fn new(bytes: &'a [u8], position: usize) -> Result<Self, ()> {
        Ok(Self { inner: MemDecoder::new(bytes, position)?, operations: vec![] })
    }
}
macro_rules! read {
    ($name:ident, $ty:ty, $op:ident) => {
        fn $name(&mut self) -> $ty {
            self.operations.push(CodecOp::$op);
            self.inner.$name()
        }
    }
}
impl Decoder for AuditDecoder<'_> {
    read!(read_usize, usize, ReadUsize);
    read!(read_u128, u128, ReadU128);
    read!(read_u64, u64, ReadU64);
    read!(read_u32, u32, ReadU32);
    read!(read_u16, u16, ReadU16);
    read!(read_u8, u8, ReadU8);
    read!(read_isize, isize, ReadIsize);
    read!(read_i128, i128, ReadI128);
    read!(read_i64, i64, ReadI64);
    read!(read_i32, i32, ReadI32);
    read!(read_i16, i16, ReadI16);
    fn read_raw_bytes(&mut self, len: usize) -> &[u8] {
        self.operations.push(CodecOp::ReadRawBytes(len));
        self.inner.read_raw_bytes(len)
    }
    fn peek_byte(&self) -> u8 { self.inner.peek_byte() }
    fn position(&self) -> usize { self.inner.position() }
}
impl BlobDecoder for AuditDecoder<'_> {
    fn decode_symbol(&mut self) -> Symbol { unused_interface("decode_symbol") }
    fn decode_byte_symbol(&mut self) -> ByteSymbol { unused_interface("decode_byte_symbol") }
    fn decode_def_index(&mut self) -> DefIndex { unused_interface("decode_def_index") }
    fn decode_local_def_id(&mut self) -> LocalDefId { unused_interface("decode_local_def_id") }
}
