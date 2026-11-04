// Development-only dependency facade. Whole source algorithms are included
// unchanged; selected lib/identity contract fragments are exact original bytes.
extern crate self as rustc_index;
extern crate self as rustc_data_structures;
extern crate self as rustc_serialize;
pub use rustc_index_macros::newtype_index;
#[path="idx.rs"]mod idx;pub use idx::Idx;
#[path="fx.rs"]pub mod fx;
#[path="int_overflow.rs"]pub mod int_overflow;
#[path="atomic_ref.rs"]mod atomic_ref;pub use atomic_ref::AtomicRef;
pub mod sync {include!("sync_runtime.rs");}
pub mod stable_hash {
// EXACT_RAW_SPAN
}
include!("identities.rs");
mod span_encoding {
 include!("span_encoding_original.rs");
 // Reference-only inspection/call adapters, never replacements for source code.
 include!("adapters.rs");
}
pub use span_encoding::{Span,OrdSpan,DUMMY_SP};
use std::fmt;use std::sync::Arc;use def_id::{DefIndex,LocalDefId};
// The complete SourceMap/symbol/hygiene runtimes remain outside this proof.
// A recording caller supplies exactly the source-map operation Debug invokes.
pub trait DiagnosticSourceMap {fn span_to_diagnostic_string(&self,span:Span)->String;}
type SourceMap=dyn DiagnosticSourceMap;
pub struct SessionGlobals {
 span_interner:sync::Lock<span_encoding::SpanInterner>,
 source_map:Option<Arc<SourceMap>>,
}
// EXACT_TLS
// EXACT_GLOBALS_FN
// EXACT_SPAN_TRACK
include!("foundations.rs");
const _:()=assert!(std::mem::size_of::<Span>()==8&&std::mem::align_of::<Span>()==4);
