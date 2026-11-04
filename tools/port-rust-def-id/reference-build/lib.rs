#![allow(internal_features)]
#![feature(const_trait_impl,const_default,negative_impls,map_try_insert,min_specialization,rustc_attrs,step_trait,pattern_types,pattern_type_macro,fn_traits,unboxed_closures)]
extern crate self as rustc_data_structures;
extern crate self as rustc_span;
extern crate self as rustc_index;
extern crate rustc_serialize;
extern crate rustc_hashes;
extern crate rustc_stable_hash;
extern crate rustc_hash;
extern crate rustc_macros;
extern crate rustc_index_macros;
extern crate indexmap;
pub mod stable_hash;
pub mod fingerprint;
pub mod unhash;
pub mod unord;
pub mod atomic_ref;
pub use atomic_ref::AtomicRef;
pub mod fx;
pub mod idx;
pub use idx::Idx;
pub use rustc_index_macros::newtype_index;
pub mod def_id;
pub use def_id::*;
use rustc_serialize::{Encodable,Decodable};
// Functional provider of the sole Symbol operation observed by def_id.rs.
// This wrapper makes no claim to compile or replace the full Symbol interner.
#[derive(Copy,Clone)]
pub struct Symbol(pub &'static str);
impl Symbol {pub fn as_str(&self)->&str {self.0}}
include!("span_contracts.rs");
