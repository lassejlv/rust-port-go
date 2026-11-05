// UI caller: actual source conditional Clone bound must reject this call.
use indexmap::IndexMap;
#[derive(Clone)]struct NoHash(Vec<u8>);
struct NoClone;
fn main(){let map=IndexMap::<NoHash,NoHash,NoClone>::with_hasher(NoClone);let _=map.clone();}
