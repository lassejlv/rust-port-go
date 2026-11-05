use rustc_data_structures::unord::{ExtendUnord,UnordItems,UnordMap,UnordSet,UnordBag};
use std::{marker::PhantomData,ops::{Deref,DerefMut}};
fn requires<T,C:ExtendUnord<T>>(){}
struct Input<T>(PhantomData<T>);impl<T>Iterator for Input<T>{type Item=T;fn next(&mut self)->Option<T>{println!("next");None}}
impl<T>Drop for Input<T>{fn drop(&mut self){println!("drop")}}
struct Wrapper{inner:UnordBag<u64>,ordered:Vec<u64>}
impl Extend<u64> for Wrapper{fn extend<I:IntoIterator<Item=u64>>(&mut self,input:I){self.ordered.extend(input)}}
struct DerefWrapper(Wrapper);
impl Deref for DerefWrapper{type Target=UnordBag<u64>;fn deref(&self)->&Self::Target{&self.0.inner}}
impl DerefMut for DerefWrapper{fn deref_mut(&mut self)->&mut Self::Target{&mut self.0.inner}}
impl Extend<u64> for DerefWrapper{fn extend<I:IntoIterator<Item=u64>>(&mut self,input:I){self.0.ordered.extend(input)}}
fn main(){
 #[cfg(positive)]{requires::<u64,UnordBag<u64>>();requires::<u64,UnordSet<u64>>();requires::<(u64,u64),UnordMap<u64,u64>>();let mut b=UnordBag::new();b.extend_unord(UnordItems::new(Input::<u64>(PhantomData)));let mut s=UnordSet::new();s.extend_unord(UnordItems::new(Input::<u64>(PhantomData)));let mut m=UnordMap::default();m.extend_unord(UnordItems::new(Input::<(u64,u64)>(PhantomData)));}
 #[cfg(aliases)]{type Bag=UnordBag<u64>;type Set=UnordSet<u64>;type Map=UnordMap<u64,u64>;requires::<u64,Bag>();requires::<u64,Set>();requires::<(u64,u64),Map>();}
 #[cfg(ordered)]requires::<u64,Vec<u64>>();
 #[cfg(wrapper)]requires::<u64,Wrapper>();
 #[cfg(deref_wrapper)]requires::<u64,DerefWrapper>();
}
