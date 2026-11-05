// Positive UI witnesses: discarded component has no Clone/Eq/Hash traits.
use indexmap::IndexMap;
struct NoClone;
fn main(){let keys=IndexMap::<u64,NoClone>::new().into_keys();let _=keys.clone();let values=IndexMap::<NoClone,u64>::new().into_values();let _=values.clone();}
