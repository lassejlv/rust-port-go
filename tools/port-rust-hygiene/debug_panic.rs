use super::*;
use rustc_span::def_id::{DEF_ID_DEBUG,DefId,CRATE_DEF_ID};
use std::fmt;
thread_local!{static DBG:RefCell<Option<(Rc<Core>,bool)>>=const{RefCell::new(None)};}
fn debug(id:DefId,f:&mut fmt::Formatter<'_>)->fmt::Result{
 let(core,error)=DBG.with(|s|s.borrow().as_ref().unwrap().clone());core.call("debug",&format!("{}/{}",id.krate.as_u32(),id.index.as_u32()));
 if core.cfg.reenter{let id=LocalExpnId::fresh_empty();println!("debug_reentered:{}",id.as_u32());}
 if error{Err(fmt::Error)}else{rustc_span::def_id::default_def_id_debug(id,f)}
}
static DEBUG:fn(DefId,&mut fmt::Formatter<'_>)->fmt::Result=debug;
struct Reset(&'static fn(DefId,&mut fmt::Formatter<'_>)->fmt::Result);
impl Drop for Reset{fn drop(&mut self){DEF_ID_DEBUG.swap(self.0);DBG.with(|s|s.borrow_mut().take());}}
pub fn build(operation:&str,core:Rc<Core>)->Box<dyn FnOnce()>{
 if operation.starts_with("already_set_debug"){
  let mut first=data();first.macro_def_id=Some(CRATE_DEF_ID.to_def_id());LocalExpnId::fresh(first.clone(),Quiet);let id=LocalExpnId::fresh(first,Quiet);let d=id.expn_data();let env=Env::new(core.clone());DBG.with(|s|*s.borrow_mut()=Some((core,operation.ends_with("_error"))));let reset=Reset(DEF_ID_DEBUG.swap(&DEBUG));
  Box::new(move||{let _reset=reset;let _=LocalExpnId::fresh(d,env);println!("returned:already_set_debug")})
 }else{
  DBG.with(|s|*s.borrow_mut()=Some((core,operation=="debug_verbose_error")));let reset=Reset(DEF_ID_DEBUG.swap(&DEBUG));
  Box::new(move||{let _reset=reset;let _=debug_hygiene_data(true);println!("returned:debug")})
 }
}
