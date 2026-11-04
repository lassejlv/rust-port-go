// Reference-only facade around the unchanged edition and exact original codec
// traits. Opaque interfaces below are never used by a unit-variant edition.
extern crate self as rustc_span;

use rustc_serialize::{Encoder, Decoder, Encodable};
use std::sync::atomic::{AtomicUsize, Ordering};

static UNUSED_INTERFACES: AtomicUsize = AtomicUsize::new(0);

pub fn unused_interface_calls() -> usize { UNUSED_INTERFACES.load(Ordering::SeqCst) }
pub(crate) fn unused_interface<T>(name: &str) -> T {
    UNUSED_INTERFACES.fetch_add(1, Ordering::SeqCst);
    panic!("unused edition reference interface reached: {name}")
}

pub enum Span {}
pub enum Symbol {}
pub enum ByteSymbol {}
pub enum ExpnId {}
pub enum SyntaxContext {}
pub enum CrateNum {}
pub enum DefIndex {}
pub enum DefId {}
pub enum DefPathHash {}
pub enum LocalDefId {}

impl LocalDefId {
    pub fn to_def_id(self) -> DefId { unused_interface("LocalDefId::to_def_id") }
}
impl<E: Encoder> Encodable<E> for DefIndex {
    fn encode(&self, _encoder: &mut E) { unused_interface("DefIndex::encode") }
}

include!("codec_traits.rs");

pub mod edition;
mod audit;
pub use audit::{AuditEncoder, AuditDecoder, CodecOp};
