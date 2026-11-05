#![feature(rustc_private,panic_can_unwind,panic_internals)]
// Non-Copy From/Drop callers of the unchanged original generic default.
use rustc_span::hygiene::HygieneRemapper;
use rustc_data_structures::stable_hash::{StableHashCtxt,StableHasher};
use std::{cell::{Cell,RefCell},sync::Arc,panic::{self,catch_unwind,AssertUnwindSafe}};
thread_local!{
 static RUNTIME:Cell<u8>=const{Cell::new(1)};
 static CASE:RefCell<String>=const{RefCell::new(String::new())};
 static EXTERNAL:RefCell<Option<Arc<[u8]>>>=const{RefCell::new(None)};
 static REMAPPER:RefCell<Option<Remapper>>=const{RefCell::new(None)};
 static INVOKE_IN_HOOK:Cell<bool>=const{Cell::new(true)};
}
fn runtime()->u8{RUNTIME.with(Cell::get)}
fn refs()->usize{EXTERNAL.with(|s|Arc::strong_count(s.borrow().as_ref().unwrap()))}
fn hex(s:&str)->String{s.bytes().map(|b|format!("{b:02x}")).collect()}
fn case(s:&str)->bool{CASE.with(|c|c.borrow().as_str()==s)}
struct Local(&'static str);
impl Drop for Local{fn drop(&mut self){println!("local:{}:{}:{}:{}",self.0,std::thread::panicking(),runtime(),refs())}}
struct Field(u32);
impl Drop for Field{fn drop(&mut self){println!("field:{}:{}:{}:{}",self.0,std::thread::panicking(),runtime(),refs());if (self.0==1&&(case("old_field")||case("old_drop_field")))||(self.0==2&&(case("new_field")||case("new_drop_field"))){panic!("key_field")}}}
struct Key{value:u32,features:Arc<[u8]>,metadata:Vec<u8>,_field:Field}
impl From<u32> for Key{fn from(value:u32)->Self{let _local=Local("from");println!("from:{value}:{}:{}:{}",std::thread::panicking(),runtime(),refs());if value==2&&case("from"){panic!("from")};Self{value,features:EXTERNAL.with(|s|s.borrow().as_ref().unwrap().clone()),metadata:vec![1,2,3],_field:Field(value)}}}
impl Drop for Key{fn drop(&mut self){let _local=Local("drop");assert_eq!(self.metadata.len(),3);assert_eq!(self.features.len(),1);println!("drop:{}:{}:{}:{}",self.value,std::thread::panicking(),runtime(),refs());if (self.value==1&&(case("old_drop")||case("old_drop_field")))||(self.value==2&&(case("new_drop")||case("new_drop_field"))){panic!("key_drop")}}}
struct Remapper{slot:Option<Key>}
impl HygieneRemapper<Key> for Remapper{
 fn stable_hash(&self,_key:Key,_ctx:&mut impl StableHashCtxt,_hasher:&mut StableHasher){panic!("unexpected stable_hash")}
 fn len(&self)->usize{let _local=Local("len");println!("len:{}:{}:{}",std::thread::panicking(),runtime(),refs());if case("len"){panic!("len")};2}
 fn first_non_det_index_mut(&mut self)->&mut Option<Key>{let _local=Local("slot");println!("slot:{}:{}:{}",std::thread::panicking(),runtime(),refs());if case("slot"){panic!("slot")};&mut self.slot}
 fn first_non_det_index(&self)->u32{self.slot.as_ref().unwrap().value}
}
impl Drop for Remapper{fn drop(&mut self){println!("remapper_drop:{}:{}:{}:{}",self.slot.as_ref().map(|k|k.value).unwrap_or(0),std::thread::panicking(),runtime(),refs())}}
fn invoke(){REMAPPER.with(|r|r.borrow_mut().as_mut().unwrap().commit_end_of_determinism())}
fn report(call:impl FnOnce()){match catch_unwind(AssertUnwindSafe(call)){Ok(())=>println!("caught:false::{}:{}:{}",std::thread::panicking(),runtime(),refs()),Err(p)=>{let text=if let Some(s)=p.downcast_ref::<&str>(){s.to_string()}else if let Some(s)=p.downcast_ref::<String>(){s.clone()}else{String::new()};println!("caught:true:{}:{}:{}:{}",hex(&text),std::thread::panicking(),runtime(),refs())}}}
struct Outer{caught:bool}
impl Drop for Outer{fn drop(&mut self){println!("outer:{}:{}:{}",std::thread::panicking(),runtime(),refs());if self.caught{report(invoke)}else{invoke()}}}
fn outer_panic(){panic!("outer")}
fn main(){
 let a:Vec<_>=std::env::args().collect();let(operation,selected,mode)=(&a[1],&a[2],&a[3]);
 EXTERNAL.with(|s|*s.borrow_mut()=Some(Arc::from([7u8])));println!("construct:1:{}",refs());
 let slot=if operation.starts_with("repeat"){Some(Key::from(1))}else{None};REMAPPER.with(|r|*r.borrow_mut()=Some(Remapper{slot}));CASE.with(|s|*s.borrow_mut()=selected.clone());
 let processing=mode=="processing";RUNTIME.with(|r|r.set(if mode=="unobserved"{0}else{2}));
 panic::set_hook(Box::new(move|info|{let(kind,text)=if let Some(s)=info.payload().downcast_ref::<&str>(){("static_str",s.to_string())}else if let Some(s)=info.payload().downcast_ref::<String>(){("owned_string",s.clone())}else{("opaque",String::new())};let location=info.location().map(|l|format!("{}:{}:{}",l.file(),l.line(),l.column())).unwrap_or_default();println!("hook:{kind}:{}:{}:{}:{}:{}:{}:{}",hex(&text),std::thread::panicking(),info.can_unwind(),info.force_no_backtrace(),runtime(),refs(),location);if processing&&INVOKE_IN_HOOK.with(Cell::get){invoke();println!("hook_return:{}:{}:{}",std::thread::panicking(),runtime(),refs())}}));
 if mode=="unobserved"{panic::set_hook(Box::new(|_|{}));}
 println!("invoke:{}:{}",runtime(),refs());
 match mode.as_str(){"normal"|"unobserved"=>report(invoke),"processing"=>report(outer_panic),_=>report(||{let _outer=Outer{caught:mode=="inherited_caught"};outer_panic()})}
 INVOKE_IN_HOOK.with(|s|s.set(false));
 REMAPPER.with(|r|println!("after:{}:{}:{}",r.borrow().as_ref().unwrap().slot.as_ref().map(|k|k.value).unwrap_or(0),runtime(),refs()));
 let r=REMAPPER.with(|r|r.borrow_mut().take().unwrap());report(||drop(r));println!("complete:{}:{}",runtime(),refs());EXTERNAL.with(|s|s.borrow_mut().take());
}
