use rustc_data_structures::unord::UnordBag;
#[cfg(no_clone)]struct Value;
#[cfg(no_copy)]#[derive(Clone)]struct Value;
fn main(){
 #[cfg(any(no_clone,no_copy))]let mut bag=UnordBag::new();
 #[cfg(any(no_clone,no_copy))]bag.push(Value);
 #[cfg(no_clone)]let _=bag.items().cloned();
 #[cfg(no_copy)]let _=bag.items().copied();
 #[cfg(any(opaque_cloned,opaque_copied,positive))]let mut bag=UnordBag::new();
 #[cfg(any(opaque_cloned,opaque_copied,positive))]bag.push(1u64);
 #[cfg(opaque_cloned)]let _=bag.items().cloned().clone();
 #[cfg(opaque_copied)]let _=bag.items().copied().clone();
 #[cfg(positive)]{let _=bag.items().cloned().count();let _=bag.items().copied().count();}
}
