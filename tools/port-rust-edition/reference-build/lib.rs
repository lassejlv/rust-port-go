pub use rustc_span::edition::*;
pub use rustc_span::{AuditEncoder, AuditDecoder, CodecOp, unused_interface_calls};
pub use rustc_serialize::{Encoder, Decoder, Encodable, Decodable};
pub use rustc_serialize::opaque::{MemDecoder, MAGIC_END_BYTES};
pub use rustc_serialize::opaque::mem_encoder::MemEncoder;
pub use rustc_data_structures::stable_hash::{StableHash, DummyStableHashCtxt, StableHasher};
pub use rustc_hashes::{Hash64, Hash128};
