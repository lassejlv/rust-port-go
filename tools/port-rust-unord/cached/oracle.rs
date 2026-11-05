// Development-only owned FnMut/key observer. Calls the actual unchanged SDK's
// slice::sort_by_cached_key, including its real collection and tuple operators.
#![feature(fn_traits,unboxed_closures,partial_ord_chaining_methods)]
use std::{cell::{Cell,RefCell},cmp::Ordering,fmt::Write as _,io::{self,Write},marker::PhantomData,ops::ControlFlow,panic::{catch_unwind,AssertUnwindSafe}};
#[derive(Clone,Copy)]pub(crate) struct Config{pub(crate) kind:usize,pub(crate) chain:bool,pub(crate) length:usize,pub(crate) seed:u64,pub(crate) key_fail:isize,pub(crate) cmp_fail:isize,pub(crate) drop_fail:isize,pub(crate) fn_fail:bool,pub(crate) token_fail:bool,pub(crate) moved:bool,pub(crate) stream:bool}
struct State{config:Config,events:Vec<String>,compares:usize,drops:usize,moved:Option<Token>}
thread_local!{static RESUME:Cell<bool>=const{Cell::new(false)};static STATE:RefCell<State>=const{RefCell::new(State{config:Config{kind:0,chain:false,length:0,seed:0,key_fail:-1,cmp_fail:-1,drop_fail:-1,fn_fail:false,token_fail:false,moved:false,stream:false},events:Vec::new(),compares:0,drops:0,moved:None})};}
pub(crate) fn set_resume(value:bool){RESUME.with(|r|r.set(value));}
#[track_caller]fn raise(message:&'static str)->!{if RESUME.with(Cell::get){std::panic::resume_unwind(Box::new(message))}else{std::panic::panic_any(message)}}
fn config()->Config{STATE.with(|s|s.borrow().config)}
fn event(value:String){let stream=STATE.with(|s|{let mut s=s.borrow_mut();s.events.push(value.clone());s.config.stream});if stream{println!("{value}");io::stdout().flush().unwrap();}}
fn p()->bool{std::thread::panicking()}
#[derive(Clone,Copy)]struct Item{key:i64,id:u64}
trait Body:Sized{fn new(v:&Item)->Self;fn key(&self)->i64;fn id(&self)->u64;fn touch(&self){}fn touches(&self)->u64{0}}
fn compare<T:Body>(label:&str,a:&T,b:&T)->Ordering{a.touch();b.touch();let index=STATE.with(|s|{let mut s=s.borrow_mut();let i=s.compares;s.compares+=1;i});event(format!("{label}:{}:{}:{}:{}:{}",a.id(),b.id(),a.touches(),b.touches(),p()));if index as isize==config().cmp_fail{raise("key_compare")};a.key().cmp(&b.key())}
fn dropped<T:Body>(v:&T){let index=STATE.with(|s|{let mut s=s.borrow_mut();let i=s.drops;s.drops+=1;i});event(format!("KD:{}:{}:{}",v.id(),v.touches(),p()));if index as isize==config().drop_fail{raise("key_drop")}}
#[repr(transparent)]#[derive(Copy,Clone)]struct Tiny(u8);
impl Body for Tiny{fn new(v:&Item)->Self{Self(v.key as u8)}fn key(&self)->i64{self.0 as i64}fn id(&self)->u64{self.0 as u64}}
#[repr(C)]struct Owned<const P:usize>{key:i64,id:u64,padding:[u8;P]}
impl<const P:usize>Body for Owned<P>{fn new(v:&Item)->Self{Self{key:v.key,id:v.id,padding:[0;P]}}fn key(&self)->i64{self.key}fn id(&self)->u64{self.id}}
impl<const P:usize>Drop for Owned<P>{fn drop(&mut self){dropped(self)}}
#[repr(C)]struct Interior{key:i64,id:u64,touches:Cell<u64>}
impl Body for Interior{fn new(v:&Item)->Self{Self{key:v.key,id:v.id,touches:Cell::new(0)}}fn key(&self)->i64{self.key}fn id(&self)->u64{self.id}fn touch(&self){self.touches.set(self.touches.get()+1)}fn touches(&self)->u64{self.touches.get()}}
impl Drop for Interior{fn drop(&mut self){dropped(self)}}
struct Zero;
impl Body for Zero{fn new(_:&Item)->Self{Self}fn key(&self)->i64{0}fn id(&self)->u64{0}}
impl Drop for Zero{fn drop(&mut self){dropped(self)}}
#[repr(C)]#[derive(Copy,Clone)]struct Plain{key:i64,id:u64}
impl Body for Plain{fn new(v:&Item)->Self{Self{key:v.key,id:v.id}}fn key(&self)->i64{self.key}fn id(&self)->u64{self.id}}
#[repr(transparent)]#[derive(Copy,Clone)]struct Key<B:Body>(B);
impl<B:Body>Body for Key<B>{fn new(v:&Item)->Self{Self(B::new(v))}fn key(&self)->i64{self.0.key()}fn id(&self)->u64{self.0.id()}fn touch(&self){self.0.touch()}fn touches(&self)->u64{self.0.touches()}}
impl<B:Body>PartialEq for Key<B>{fn eq(&self,b:&Self)->bool{event(format!("EQ:{}:{}",self.id(),b.id()));self.key()==b.key()}}
impl<B:Body>Eq for Key<B>{}
impl<B:Body>PartialOrd for Key<B>{fn partial_cmp(&self,b:&Self)->Option<Ordering>{Some(compare("P",self,b))}fn lt(&self,b:&Self)->bool{compare("LT",self,b).is_lt()}}
impl<B:Body>Ord for Key<B>{fn cmp(&self,b:&Self)->Ordering{compare("ORD",self,b)}}
#[repr(transparent)]#[derive(Copy,Clone)]struct Chained<B:Body>(B);
impl<B:Body>Body for Chained<B>{fn new(v:&Item)->Self{Self(B::new(v))}fn key(&self)->i64{self.0.key()}fn id(&self)->u64{self.0.id()}fn touch(&self){self.0.touch()}fn touches(&self)->u64{self.0.touches()}}
impl<B:Body>PartialEq for Chained<B>{fn eq(&self,b:&Self)->bool{event(format!("EQ:{}:{}",self.id(),b.id()));self.key()==b.key()}}
impl<B:Body>Eq for Chained<B>{}
impl<B:Body>PartialOrd for Chained<B>{fn partial_cmp(&self,b:&Self)->Option<Ordering>{Some(compare("P",self,b))}fn lt(&self,b:&Self)->bool{compare("LT",self,b).is_lt()}fn __chaining_less(&self,b:&Self)->ControlFlow<bool>{match compare("C",self,b){Ordering::Equal=>ControlFlow::Continue(()),value=>ControlFlow::Break(value.is_lt())}}}
impl<B:Body>Ord for Chained<B>{fn cmp(&self,b:&Self)->Ordering{compare("ORD",self,b)}}
struct Token;
impl Drop for Token{fn drop(&mut self){event(format!("TD:{}",p()));if config().token_fail{raise("token_drop")}}}
struct Function<K>{calls:usize,first_address:usize,token:Option<Token>,key:PhantomData<K>}
impl<K>Drop for Function<K>{fn drop(&mut self){event(format!("FD:{}:{}:{}:{}",self.calls,self.token.is_some(),self.calls==0||self.first_address==self as*const Self as usize,p()));if config().fn_fail{raise("function_drop")}}}
impl<K:Body>FnOnce<(&Item,)>for Function<K>{type Output=K;extern "rust-call" fn call_once(self,_:(&Item,))->K{raise("unexpected_call_once")}}
impl<K:Body>FnMut<(&Item,)>for Function<K>{extern "rust-call" fn call_mut(&mut self,(item,):(&Item,))->K{let address=self as*const Self as usize;if self.calls==0{self.first_address=address};let same=address==self.first_address;let index=self.calls;self.calls+=1;event(format!("F:{}:{}:{same}:{}",item.id,self.calls,p()));if config().moved&&index==0{let token=self.token.take();STATE.with(|s|s.borrow_mut().moved=token);event(format!("move:{}",p()));}if index as isize==config().key_fail{raise("key_function")};K::new(item)}}
fn random(s:&mut u64)->u64{*s^=*s<<13;*s^=*s>>7;*s^=*s<<17;*s}
fn execute<K:Body+Ord>(values:&mut[Item]){event(format!("TYPE:{}:{}:{}:{}",size_of::<K>(),align_of::<K>(),size_of::<(K,u32)>(),size_of::<(K,usize)>()));values.sort_by_cached_key(Function::<K>{calls:0,first_address:0,token:Some(Token),key:PhantomData});}
fn payload(value:Box<dyn std::any::Any+Send>)->String{if let Some(s)=value.downcast_ref::<&str>(){(*s).into()}else if let Some(s)=value.downcast_ref::<String>(){s.clone()}else{"other_panic".into()}}
pub(crate) fn run(c:Config)->(String,String,String,usize,usize){STATE.with(|s|{let mut s=s.borrow_mut();assert!(s.moved.is_none());*s=State{config:c,events:vec![],compares:0,drops:0,moved:None};});let mut seed=c.seed;let mut values:Vec<_>=(0..c.length).map(|id|Item{key:(random(&mut seed)%13)as i64,id:id as u64}).collect();macro_rules! call{($b:ty)=>{if c.chain{execute::<Chained<$b>>(&mut values)}else{execute::<Key<$b>>(&mut values)}}}let failure=catch_unwind(AssertUnwindSafe(||match c.kind{0=>call!(Tiny),1=>call!(Owned<0>),2=>call!(Interior),3=>call!(Zero),4=>call!(Plain),5=>call!(Owned<240>),_=>unreachable!()})).err().map(payload).unwrap_or_default();event(format!("end:{}:{}",!failure.is_empty(),p()));let moved=STATE.with(|s|s.borrow_mut().moved.take());drop(moved);let mut state=String::new();for value in values{write!(state,"{}:{},",value.id,value.key).unwrap();}let(events,compares,drops)=STATE.with(|s|{let s=s.borrow();(s.events.join("\n"),s.compares,s.drops)});(failure,state,events,compares,drops)}
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
fn emit(out:&mut impl Write,c:Config)->io::Result<usize>{let(failure,state,events,compares,drops)=run(c);writeln!(out,"{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}",c.kind,c.chain,c.length,c.seed,c.key_fail,c.cmp_fail,c.drop_fail,c.fn_fail,c.token_fail,c.moved,hex(&failure),compares,drops,hex(&events),hex(&state))?;Ok(compares)}
fn direct(value:String){println!("{value}");io::stdout().flush().unwrap();}
struct Scope;
impl Drop for Scope{fn drop(&mut self){direct(format!("scope_drop:{}",p()));}}
fn child_body(c:Config){direct(format!("scope:{}",p()));let _scope=Scope;let(failure,state,_,compares,drops)=run(c);direct(format!("RESULT:{}:{compares}:{drops}:{}",hex(&failure),hex(&state)));}
fn child(context:&str,c:Config){match context{"normal"=>child_body(c),"unwind"=>{struct Outer(Config);impl Drop for Outer{fn drop(&mut self){child_body(self.0);}}let _=catch_unwind(AssertUnwindSafe(||{let _outer=Outer(c);std::panic::panic_any("outer")}));direct(format!("outer_end:{}",p()));},"processing"=>{std::panic::set_hook(Box::new(move|_|{child_body(c);}));let _=catch_unwind(||std::panic::panic_any("outer"));direct(format!("outer_end:{}",p()));},_=>panic!("invalid child context")}}
fn main()->io::Result<()>{std::panic::set_hook(Box::new(|_|{}));let args:Vec<_>=std::env::args().collect();if args.len()>1{assert_eq!(args.len(),4);assert_eq!(args[1],"--child");let v:Vec<i64>=args[3].split(',').map(|v|v.parse().unwrap()).collect();assert_eq!(v.len(),10);child(&args[2],Config{kind:v[0]as usize,chain:v[1]!=0,length:v[2]as usize,seed:v[3]as u64,key_fail:v[4]as isize,cmp_fail:v[5]as isize,drop_fail:v[6]as isize,fn_fail:v[7]!=0,token_fail:v[8]!=0,moved:v[9]!=0,stream:true});return Ok(())}let mut out=io::BufWriter::new(io::stdout().lock());for kind in 0..6{for chain in [false,true]{for length in [0,1,2,3,9,21,33,65]{for seed in [1,7]{let base=Config{kind,chain,length,seed,key_fail:-1,cmp_fail:-1,drop_fail:-1,fn_fail:false,token_fail:false,moved:false,stream:false};let compares=emit(&mut out,base)?;for key_fail in 0..=length{emit(&mut out,Config{key_fail:key_fail as isize,..base})?;}let mut cuts=vec![0,1,2,5,compares/2,compares.saturating_sub(1),compares,compares+1];cuts.sort_unstable();cuts.dedup();for cmp_fail in cuts{emit(&mut out,Config{cmp_fail:cmp_fail as isize,..base})?;}if kind!=0&&kind!=4{let mut cuts=vec![0,length/2,length.saturating_sub(1),length];cuts.sort_unstable();cuts.dedup();for drop_fail in cuts{emit(&mut out,Config{drop_fail:drop_fail as isize,..base})?;}}emit(&mut out,Config{fn_fail:true,..base})?;emit(&mut out,Config{token_fail:true,..base})?;emit(&mut out,Config{moved:true,..base})?;}}}}Ok(())}
