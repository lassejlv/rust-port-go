#![feature(min_specialization)]
#![allow(internal_features)]

pub mod stable_hash;
pub mod fingerprint;
pub mod unhash;

pub use fingerprint::{Fingerprint, FingerprintComponent, PackedFingerprint};
pub use rustc_hashes::{Hash64, Hash128};
pub use rustc_serialize::{Encoder, Decoder, Encodable, Decodable};
pub use rustc_serialize::opaque::{MemDecoder, MAGIC_END_BYTES};
pub use rustc_serialize::opaque::mem_encoder::MemEncoder;
pub use stable_hash::{DummyStableHashCtxt, StableHash, StableHasher, FromStableHash, StableHasherHash, StableOrd, StableCompare};
