use rustc_data_structures::unord::UnordItems;
struct Value;
fn main(){
 #[cfg(no_ord)]let _=UnordItems::new(std::iter::once(Value)).max();
 #[cfg(no_sum)]let _:Value=UnordItems::new(std::iter::once(1u64)).sum();
 #[cfg(no_product)]let _:Value=UnordItems::new(std::iter::once(1u64)).product();
 #[cfg(positive)]{let _=UnordItems::new(std::iter::once(1u64)).max();let _=UnordItems::new(std::iter::once(1u64)).min();let _:u64=UnordItems::new(std::iter::once(1u64)).sum();let _:u64=UnordItems::new(std::iter::once(1u64)).product();}
}
