use rustc_data_structures::unord::UnordItems;
fn main(){
 #[cfg(positive)]{let _=UnordItems::new(std::iter::empty::<u64>()).filter(|_|true).count();let _=UnordItems::new(std::iter::empty::<u64>()).filter_map(Some).count();let _=UnordItems::new(std::iter::empty::<u64>()).flat_map(|x|UnordItems::new(std::iter::once(x))).count();let _=UnordItems::new(std::iter::empty::<u64>()).chain(UnordItems::new(std::iter::empty::<u64>())).count();}
 #[cfg(filter_not_fn)]{let mut n=0;let _=UnordItems::new(std::iter::empty::<u64>()).filter(|_|{n+=1;true});}
 #[cfg(filter_map_not_fn)]{let mut n=0;let _=UnordItems::new(std::iter::empty::<u64>()).filter_map(|x|{n+=1;Some(x)});}
 #[cfg(flat_map_not_fn)]{let mut n=0;let _=UnordItems::new(std::iter::empty::<u64>()).flat_map(|x|{n+=1;UnordItems::new(std::iter::once(x))});}
 #[cfg(chain_mismatch)]let _=UnordItems::new(std::iter::empty::<u64>()).chain(UnordItems::new(std::iter::empty::<u8>()));
 #[cfg(opaque_filter)]let _=UnordItems::new(std::iter::empty::<u64>()).filter(|_|true).clone();
 #[cfg(opaque_filter_map)]let _=UnordItems::new(std::iter::empty::<u64>()).filter_map(Some).clone();
 #[cfg(opaque_flat_map)]let _=UnordItems::new(std::iter::empty::<u64>()).flat_map(|x|UnordItems::new(std::iter::once(x))).clone();
 #[cfg(opaque_chain)]let _=UnordItems::new(std::iter::empty::<u64>()).chain(UnordItems::new(std::iter::empty::<u64>())).clone();
}
