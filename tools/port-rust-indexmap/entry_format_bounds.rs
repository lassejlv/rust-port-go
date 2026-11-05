// Original API trait-bound witness: VacantEntry Debug does not require V Debug.
use indexmap::{IndexMap,map::Entry};
struct Undebug{_data:Vec<u8>}
fn main(){let mut map=IndexMap::<u64,Undebug>::new();
 #[cfg(entry_enum)]{let _=format!("{:?}",map.entry(7));}
 #[cfg(not(entry_enum))]{match map.entry(7){Entry::Vacant(value)=>assert_eq!(format!("{value:?}"),"VacantEntry(7)"),_=>unreachable!()}}
}
