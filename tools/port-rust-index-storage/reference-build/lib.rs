#![allow(internal_features)]
#![feature(step_trait, pattern_types, pattern_type_macro, rustc_attrs, structural_match)]
#![cfg_attr(feature = "nightly", feature(extend_one))]

mod idx;
mod slice;
mod vec;

pub use idx::{Idx, IntoSliceIdx};
pub use slice::IndexSlice;
pub use vec::IndexVec;
pub use rustc_index_macros::newtype_index;

#[cfg(feature = "nightly")]
pub use rustc_serialize::{Decodable, Decoder, Encodable, Encoder};
#[cfg(feature = "nightly")]
pub use rustc_serialize::opaque;
