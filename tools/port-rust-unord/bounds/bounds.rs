// Admission checks call the unchanged pinned source, not replacement traits.
#![allow(dead_code,unused_imports)]
use rustc_data_structures::unord::{UnordItems,UnordBag};
use rustc_data_structures::stable_hash::{StableHash,StableHashCtxt,StableHasher,ToStableHashKey};
use std::{cmp::Ordering,iter::{Sum,Product}};
#[derive(Clone,Copy,Eq,PartialEq,Ord,PartialOrd)]struct Value(u64);
impl Default for Value{fn default()->Self{panic!("UNEXPECTED value Default")}}
struct NoDefault;struct NoClone;#[derive(Clone)]struct OnlyClone;struct NoOrd;
struct Total;
impl Sum<Value> for Total{fn sum<I:Iterator<Item=Value>>(_:I)->Self{Total}}
impl Product<Value> for Total{fn product<I:Iterator<Item=Value>>(_:I)->Self{Total}}
struct NoTotal;
#[derive(Clone)]struct I1(std::iter::Empty<Value>);
#[derive(Clone)]struct I2(std::iter::Empty<Value>);
impl Iterator for I1{type Item=Value;fn next(&mut self)->Option<Value>{self.0.next()}}
impl Iterator for I2{type Item=Value;fn next(&mut self)->Option<Value>{self.0.next()}}
struct NoCloneInput;impl Iterator for NoCloneInput{type Item=Value;fn next(&mut self)->Option<Value>{None}}
fn opaque()->UnordItems<Value,impl Iterator<Item=Value>>{UnordItems::new(std::iter::empty()).map(|value|value)}
struct Key;
impl PartialEq for Key{fn eq(&self,_:&Self)->bool{true}}
#[cfg(not(key_eq))]impl Eq for Key{}
#[cfg(not(key_partial_ord))]impl PartialOrd for Key{fn partial_cmp(&self,_:&Self)->Option<Ordering>{Some(Ordering::Equal)}}
#[cfg(not(key_ord))]impl Ord for Key{fn cmp(&self,_:&Self)->Ordering{Ordering::Equal}}
#[cfg(not(key_stable_hash))]impl StableHash for Key{fn stable_hash<H:StableHashCtxt>(&self,_:&mut H,_:&mut StableHasher){}}
struct Item;
#[cfg(not(key_sized))]impl ToStableHashKey for Item{type KeyType=Key;fn to_stable_hash_key<H:StableHashCtxt>(&self,_:&mut H)->Key{Key}}
#[cfg(key_sized)]impl ToStableHashKey for Item{type KeyType=[u64];fn to_stable_hash_key<H:StableHashCtxt>(&self,_:&mut H)->[u64]{panic!("unsized")}}
fn main(){
 #[cfg(positive)]{
 let _:UnordBag<Value>=Default::default();
 let mut a=UnordItems::new(I1(std::iter::empty()));let b=UnordItems::new(I1(std::iter::empty()));type Alias=I1;let alias=UnordItems::new(Alias::clone(&I1(std::iter::empty())));a.clone_from(&b);drop(a.clone());drop(alias);
 drop(UnordItems::new(std::iter::empty::<Value>()).max());drop(UnordItems::new(std::iter::empty::<Value>()).min());
 let _:Total=UnordItems::new(std::iter::empty::<Value>()).sum();let _:Total=UnordItems::new(std::iter::empty::<Value>()).product();
 drop(UnordItems::new(std::iter::empty::<&Value>()).cloned());drop(UnordItems::new(std::iter::empty::<&Value>()).copied());
 }
 #[cfg(no_default)]let _=<UnordBag<NoDefault>as Default>::default();
 #[cfg(new_no_default)]let _=UnordBag::<NoDefault>::new();
 #[cfg(clone_from_self)]{let mut a=UnordItems::new(I1(std::iter::empty()));let b=UnordItems::new(I2(std::iter::empty()));a.clone_from(&b);}
 #[cfg(clone_from_opaque)]{let mut a=opaque();let b=opaque();a.clone_from(&b);}
 #[cfg(clone_item)]let _=UnordItems::<NoClone,_>::empty().clone();
 #[cfg(clone_iterator)]let _=UnordItems::new(NoCloneInput).clone();
 #[cfg(cloned_value)]let _=UnordItems::new(std::iter::empty::<&NoClone>()).cloned();
 #[cfg(copied_value)]let _=UnordItems::new(std::iter::empty::<&OnlyClone>()).copied();
 #[cfg(max_ord)]let _=UnordItems::new(std::iter::empty::<NoOrd>()).max();
 #[cfg(min_ord)]let _=UnordItems::new(std::iter::empty::<NoOrd>()).min();
 #[cfg(sum_trait)]let _:NoTotal=UnordItems::new(std::iter::empty::<Value>()).sum();
 #[cfg(product_trait)]let _:NoTotal=UnordItems::new(std::iter::empty::<Value>()).product();
}
