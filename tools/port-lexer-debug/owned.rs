// Validation clients only. Original algorithms are compiled without edits.
use std::{cell::Cell,fmt::Debug,ops::Range,panic::{catch_unwind,AssertUnwindSafe},rc::Rc};
use crate::{lexer,lit,hex};

#[derive(Clone)]
struct State {calls:Rc<Cell<usize>>,last:Rc<Cell<usize>>,fail:usize,drop_panic:bool,borrow:bool}
impl State {
 fn log(&self,s:String){println!("{s}");}
 fn call(&self,ptr:usize,value:String){let n=self.calls.get()+1;self.calls.set(n);let last=self.last.replace(ptr);self.log(format!("call:{n}:{}:{}:{value}",std::thread::panicking(),if last==0{"none"}else if last==ptr{"same"}else{"changed"}));if n==self.fail{panic!("callback");}}
}
struct Field(State);
impl Drop for Field {fn drop(&mut self){self.0.log(format!("field:{}",std::thread::panicking()));}}
struct Callback<T>{state:State,_field:Field,_type:std::marker::PhantomData<T>}
impl<T> Callback<T>{fn new(s:State)->Self{Self{state:s.clone(),_field:Field(s),_type:std::marker::PhantomData}}}
impl<T> Drop for Callback<T>{fn drop(&mut self){let same=if self.state.calls.get()==0||self.state.borrow{"none"}else if self.state.last.get()==self as *const _ as usize{"same"}else{"changed"};self.state.log(format!("drop:{}:{}:{same}",self.state.calls.get(),std::thread::panicking()));if self.state.drop_panic{panic!("environment Drop");}}}
impl<T:Debug> FnOnce<(Range<usize>,T)> for Callback<T>{type Output=();extern "rust-call" fn call_once(mut self,args:(Range<usize>,T)){self.call_mut(args)}}
impl<T:Debug> FnMut<(Range<usize>,T)> for Callback<T>{extern "rust-call" fn call_mut(&mut self,(span,value):(Range<usize>,T)){self.state.call(self as *const _ as usize,format!("{}..{}:{value:?}",span.start,span.end));}}
struct Predicate {state:State,_field:Field,stop:char}
impl Drop for Predicate{fn drop(&mut self){let same=if self.state.calls.get()==0||self.state.borrow{"none"}else if self.state.last.get()==self as *const _ as usize{"same"}else{"changed"};self.state.log(format!("drop:{}:{}:{same}",self.state.calls.get(),std::thread::panicking()));if self.state.drop_panic{panic!("predicate Drop");}}}
impl FnOnce<(char,)> for Predicate{type Output=bool;extern "rust-call" fn call_once(mut self,args:(char,))->bool{self.call_mut(args)}}
impl FnMut<(char,)> for Predicate{extern "rust-call" fn call_mut(&mut self,(c,):(char,))->bool{self.state.call(self as *const _ as usize,(c as u32).to_string());c!=self.stop}}

pub fn count(scope:&str)->usize {if scope=="literal"{14*9*4*2*2*2}else if scope=="iterator"{3*7*4*2*2*2}else{7*4*2*2*2}}
pub fn processing(scope:&str,id:usize){
 // A panic raised while the hook formats/processes the first panic aborts
 // before unwinding callback locals. Calling catch_unwind inside the hook
 // cannot make that second panic recoverable.
 let scope=scope.to_owned();std::panic::set_hook(Box::new(move |_|{
  println!("processing:{}",std::thread::panicking());
  let state=State{calls:Rc::new(Cell::new(0)),last:Rc::new(Cell::new(0)),fail:if id%2==0{1}else{0},drop_panic:id%2!=0,borrow:false};
  match scope.as_str(){"literal"=>literal(id/2,"\r",state),"iterator"=>iterator(id/2,"a!b",state),_=>predicate("abc",state)}
 }));panic!("first panic");
}
pub fn run(scope:&str,mut id:usize){
 let ambient=id%2!=0;id/=2;let borrow=id%2!=0;id/=2;let drop_panic=id%2!=0;id/=2;let fail=[0,1,2,99][id%4];id/=4;
 let inputs=["","abc","a\\z","🦀\r\0","\\\n \n🦀z","\\x80\\xff\\u{1f980}","\\\n \u{a0}x","\0","\\"];
 let input=if scope=="literal"{inputs[id%inputs.len()]}else{["","abc","a\0b","🦀🦀x","a!b","\0","!!!"][id%7]};
 let operation=if scope=="literal"{id/inputs.len()}else if scope=="iterator"{id/7}else{0};
 let state=State{calls:Rc::new(Cell::new(0)),last:Rc::new(Cell::new(0)),fail,drop_panic,borrow};
 let execute=||{if scope=="literal" {literal(operation,input,state.clone())}else if scope=="iterator"{iterator(operation,input,state.clone())}else{predicate(input,state.clone())}};
 if ambient {
  struct Outer<F:FnMut()>(F);impl<F:FnMut()> Drop for Outer<F>{fn drop(&mut self){println!("ambient:{}",std::thread::panicking());let result=catch_unwind(AssertUnwindSafe(&mut self.0));println!("inner_caught:{}",result.is_err());}}
  let result=catch_unwind(AssertUnwindSafe(||{let _outer=Outer(execute);panic!("outer");}));println!("caught:{}",result.is_err());
 } else {let result=catch_unwind(AssertUnwindSafe(execute));println!("caught:{}",result.is_err());}
}
fn literal(operation:usize,input:&str,state:State){
 macro_rules! call{($ty:ty,$function:path)=>{{let mut callback=Callback::<$ty>::new(state.clone());if state.borrow{$function(input,&mut callback);println!("returned");}else{$function(input,callback);println!("returned");}}};}
 match operation {
 0=>call!(Result<char,lit::EscapeError>,lit::check_raw_str),1=>call!(Result<u8,lit::EscapeError>,lit::check_raw_byte_str),2=>call!(Result<std::num::NonZero<char>,lit::EscapeError>,lit::check_raw_c_str),
 3=>call!(Result<char,lit::EscapeError>,lit::unescape_str),4=>call!(Result<u8,lit::EscapeError>,lit::unescape_byte_str),5=>call!(Result<lit::MixedUnit,lit::EscapeError>,lit::unescape_c_str),
 6..=13=>{let mode=[lit::Mode::Char,lit::Mode::Byte,lit::Mode::Str,lit::Mode::RawStr,lit::Mode::ByteStr,lit::Mode::RawByteStr,lit::Mode::CStr,lit::Mode::RawCStr][operation-6];let mut callback=Callback::<lit::EscapeError>::new(state.clone());if state.borrow{lit::check_for_errors(input,mode,&mut callback);println!("returned");}else{lit::check_for_errors(input,mode,callback);println!("returned");}},_=>panic!("invalid operation")
 }
}
fn predicate(input:&str,state:State){
 let mut cursor=lexer::Cursor::new(input,lexer::FrontmatterAllowed::No);
 let result=catch_unwind(AssertUnwindSafe(||{let mut predicate=Predicate{state:state.clone(),_field:Field(state.clone()),stop:'!'};if state.borrow{cursor.eat_while(&mut predicate);println!("returned");}else{cursor.eat_while(predicate);println!("returned");}}));
 println!("cursor:{}:{}:{}:{}:{}:{}:{}",hex(cursor.as_str().as_bytes()),cursor.prev() as u32,cursor.first() as u32,cursor.second() as u32,cursor.third() as u32,cursor.pos_within_token(),cursor.is_eof());if let Err(p)=result{std::panic::resume_unwind(p);}
}
struct Consumer {state:State,_field:Field}
impl Drop for Consumer{fn drop(&mut self){let same=if self.state.calls.get()==0||self.state.borrow{"none"}else if self.state.last.get()==self as *const _ as usize{"same"}else{"changed"};self.state.log(format!("drop:{}:{}:{same}",self.state.calls.get(),std::thread::panicking()));if self.state.drop_panic{panic!("consumer Drop");}}}
impl FnOnce<(lexer::Token,)> for Consumer{type Output=();extern "rust-call" fn call_once(mut self,args:(lexer::Token,)){self.call_mut(args)}}
impl FnMut<(lexer::Token,)> for Consumer{extern "rust-call" fn call_mut(&mut self,(token,):(lexer::Token,)){self.state.call(self as *const _ as usize,format!("{token:?}"));}}
struct Acc {id:usize,state:State,fail:usize}
impl Drop for Acc{fn drop(&mut self){self.state.log(format!("acc:{}:{}",self.id,std::thread::panicking()));if self.id==self.fail{panic!("accumulator Drop");}}}
struct Folder {state:State,_field:Field}
impl Drop for Folder{fn drop(&mut self){let same=if self.state.calls.get()==0||self.state.borrow{"none"}else if self.state.last.get()==self as *const _ as usize{"same"}else{"changed"};self.state.log(format!("drop:{}:{}:{same}",self.state.calls.get(),std::thread::panicking()));if self.state.drop_panic{panic!("folder Drop");}}}
impl FnOnce<(Acc,lexer::Token)> for Folder{type Output=Acc;extern "rust-call" fn call_once(mut self,args:(Acc,lexer::Token))->Acc{self.call_mut(args)}}
impl FnMut<(Acc,lexer::Token)> for Folder{extern "rust-call" fn call_mut(&mut self,(acc,token):(Acc,lexer::Token))->Acc{self.state.call(self as *const _ as usize,format!("{}:{token:?}",acc.id));Acc{id:acc.id+1,state:acc.state.clone(),fail:acc.fail}}}
fn iterator(operation:usize,input:&str,state:State){
 let it=lexer::tokenize(input,lexer::FrontmatterAllowed::No);
 if operation==0 {let mut consumer=Consumer{state:state.clone(),_field:Field(state.clone())};if state.borrow{it.for_each(&mut consumer);println!("returned");}else{it.for_each(consumer);println!("returned");}}
 else {let acc=Acc{id:0,state:state.clone(),fail:if operation==2{1}else{usize::MAX}};let mut folder=Folder{state:state.clone(),_field:Field(state.clone())};let result=if state.borrow{it.fold(acc,&mut folder)}else{it.fold(acc,folder)};println!("returned");drop(result);}
}
pub fn fold_next(id:usize){
 let state=State{calls:Rc::new(Cell::new(0)),last:Rc::new(Cell::new(0)),fail:0,drop_panic:id==1,borrow:false};
 let result=catch_unwind(AssertUnwindSafe(||{
  let it=std::iter::from_fn(||->Option<lexer::Token>{println!("next");panic!("Next");});
  let acc=Acc{id:0,state:state.clone(),fail:if id==2{0}else{usize::MAX}};
  let folder=Folder{state:state.clone(),_field:Field(state.clone())};let result=it.fold(acc,folder);drop(result);
 }));println!("caught:{}",result.is_err());
}
fn internal_body(mode:usize){
 struct Guard;impl Drop for Guard{fn drop(&mut self){println!("scope_drop:{}",std::thread::panicking());}}
 let _guard=Guard;println!("before");
 match mode {
  0=>{let mut c=lexer::Cursor::new("λ",lexer::FrontmatterAllowed::No);c.eat_until(0xbb);},
  1=>{let mut c=lexer::Cursor::new("λ",lexer::FrontmatterAllowed::No);c.bump_bytes(1);},
  2=>{let mut c=lexer::Cursor::new("#\"x\"#",lexer::FrontmatterAllowed::No);c.bump();c.guarded_double_quoted_string();},
  3=>{let _=lexer::validate_raw_str("",0);},
  4=>{let _=lexer::validate_raw_str("r",2);},
  5=>{let _=lexer::validate_raw_str("x",1);},
  6=>{let a=std::hint::black_box(u32::MAX);let _=a+std::hint::black_box(1);},
  7=>{let a=std::hint::black_box(0u32);let _=a-std::hint::black_box(1);},
  8|9=>{
   // Exact source frontmatter subtraction/addition, inside native from_fn
   // Next. This is a bounded arithmetic/cleanup proof, not large text lexing.
   let it=std::iter::from_fn(||{let pos=std::hint::black_box(1u32);let start=std::hint::black_box(u32::MAX);let len=pos-start+1;Some(lexer::Token{kind:lexer::TokenKind::Eof,len})});
   let state=State{calls:Rc::new(Cell::new(0)),last:Rc::new(Cell::new(0)),fail:0,drop_panic:false,borrow:false};
   if mode==8{it.for_each(Consumer{state:state.clone(),_field:Field(state)});}else{let acc=Acc{id:0,state:state.clone(),fail:usize::MAX};let result=it.fold(acc,Folder{state:state.clone(),_field:Field(state)});drop(result);}
  },
  10..=13=>{let mut it=std::iter::from_fn(||{let pos=std::hint::black_box(1u32);let start=std::hint::black_box(u32::MAX);let len=pos-start+1;Some(lexer::Token{kind:lexer::TokenKind::Eof,len})});match mode{10=>{it.next();},11=>{it.nth(0);},12=>{it.count();},_=>{it.last();}}},
  _=>panic!("invalid internal operation")
 }
 println!("returned");
}
pub fn internal(id:usize){
 let mode=id/3;match id%3 {
  0=>{let result=catch_unwind(AssertUnwindSafe(||internal_body(mode)));println!("caught:{}",result.is_err());},
  1=>{
   struct Outer(usize);impl Drop for Outer{fn drop(&mut self){println!("ambient:{}",std::thread::panicking());let result=catch_unwind(AssertUnwindSafe(||internal_body(self.0)));println!("inner_caught:{}",result.is_err());}}
   let result=catch_unwind(AssertUnwindSafe(||{let _outer=Outer(mode);panic!("outer");}));println!("caught:{}",result.is_err());
  },_=>{std::panic::set_hook(Box::new(move |_|{println!("processing:{}",std::thread::panicking());internal_body(mode);}));panic!("first panic");}
 }
}
