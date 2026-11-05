// Safe boundary and inherited Step callers of actual newtype_index expansions.
use super::*;
use std::iter::Step;
use rustc_index::Idx;
struct IndexState<T:Copy>(T,fn(T)->u32);
impl<T:Copy>Drop for IndexState<T>{fn drop(&mut self){println!("index_state:{}:{}:{}:{}",(self.1)(self.0),std::thread::panicking(),runtime(),refs())}}
pub fn build(operation:&str)->Box<dyn FnOnce()>{
 let op=operation.to_owned();
 Box::new(move||{
  macro_rules! invoke{($ty:ty,$action:expr)=>{match $action{
   "u32"=>{let _=<$ty>::from_u32(u32::MAX);},
   "usize"=>{let _=<$ty>::from_usize(usize::MAX);},
   "add_bound"=>{let _=<$ty>::MAX+1;},
   "add_overflow"=>{let v=<$ty>::from_u32(1)+usize::MAX;println!("returned:index:{}",v.as_u32());},
   "assign_bound"=>{let mut v=IndexState(<$ty>::MAX,<$ty>::as_u32);v.0+=1;println!("returned:assign:{}",v.0.as_u32());},
   "assign_overflow"=>{let mut v=IndexState(<$ty>::from_u32(1),<$ty>::as_u32);v.0+=usize::MAX;println!("returned:assign:{}",v.0.as_u32());},
   "plus_bound"=>{let _=<$ty>::MAX.plus(1);},
   "plus_overflow"=>{let v=<$ty>::from_u32(1).plus(usize::MAX);println!("returned:index:{}",v.as_u32());},
   "increment_bound"=>{let mut v=IndexState(<$ty>::MAX,<$ty>::as_u32);v.0.increment_by(1);println!("returned:assign:{}",v.0.as_u32());},
   "increment_overflow"=>{let mut v=IndexState(<$ty>::from_u32(1),<$ty>::as_u32);v.0.increment_by(usize::MAX);println!("returned:assign:{}",v.0.as_u32());},
   _=>panic!("index action")}}}
  let rest=op.strip_prefix("index_").expect("index operation");
  if let Some(action)=rest.strip_prefix("syntax_"){invoke!(SyntaxContext,action)}
  else if let Some(action)=rest.strip_prefix("expn_"){invoke!(ExpnIndex,action)}
  else if let Some(action)=rest.strip_prefix("local_"){invoke!(LocalExpnId,action)}
  else if let Some(action)=rest.strip_prefix("decoded_"){invoke!(DecodedSyntaxContext,action)}
  else{match rest{
   "step_forward_bound"=>{let _=ExpnIndex::forward_checked(ExpnIndex::MAX,1);},
   "step_forward_none"=>println!("returned:step:{:?}",ExpnIndex::forward_checked(ExpnIndex::from_u32(1),usize::MAX).map(ExpnIndex::as_u32)),
   "step_backward_none"=>println!("returned:step:{:?}",ExpnIndex::backward_checked(ExpnIndex::ZERO,1).map(ExpnIndex::as_u32)),
   "step_forward_overflow"=>{let(v,o)=ExpnIndex::forward_overflowing(ExpnIndex::from_u32(1),usize::MAX);println!("returned:step_overflow:{}:{o}",v.as_u32());},
   "step_backward_overflow"=>{let _=ExpnIndex::backward_overflowing(ExpnIndex::ZERO,1);},
   "step_forward"=>{let _=ExpnIndex::forward(ExpnIndex::from_u32(1),usize::MAX);},
   "step_backward"=>{let _=ExpnIndex::backward(ExpnIndex::ZERO,1);},
   _=>panic!("step action")
  }}
 })
}
