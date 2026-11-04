// SPDX-License-Identifier: MIT OR Apache-2.0
// A fallible fmt::Write and ordinary Debug implementations exercise the real
// original storage formatters and the compiler's standard Formatter builders.
use super::*;
use std::fmt::Write;

pub(super)fn options(f:&fmt::Formatter<'_>)->String{
 serde_json::to_string(&json!({"alternate":f.alternate(),"width":f.width(),"precision":f.precision(),"fill":f.fill()as u32,"align":format!("{:?}",f.align()),"plus":f.sign_plus(),"minus":f.sign_minus(),"zero":f.sign_aware_zero_pad()})).unwrap()
}
struct LocalToken(&'static str,i64);
impl Drop for LocalToken{fn drop(&mut self){ev!("format_local_drop",self.0,self.1,std::thread::panicking())}}
pub(super)fn key(k:&K,f:&mut fmt::Formatter<'_>)->fmt::Result{
 let _local=LocalToken("key",k.n as i64);ev!("format_key",k.n,options(f));f.write_str("K")?;fmt::Debug::fmt(&k.n,f)
}
struct F(i64);
impl fmt::Debug for F{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{
 let _local=LocalToken("element",self.0);ev!("format_element",self.0,options(f));if self.0==17{let _=f.write_str("try");f.write_str("again")}else if self.0== -2{f.write_str("\n\nz\n")}else if self.0== -3{f.write_str("")}else if self.0<0{f.write_str("x\nλ")}else{f.write_str("E")?;fmt::Debug::fmt(&self.0,f)}
}}
struct Sink{calls:usize,fail:usize,text:String}
impl Write for Sink{fn write_str(&mut self,s:&str)->fmt::Result{self.calls+=1;ev!("write",self.calls,s);if self.calls==self.fail{Err(fmt::Error)}else{self.text.push_str(s);Ok(())}}}
struct CharSink<'a>(&'a mut Sink);
impl Write for CharSink<'_>{fn write_str(&mut self,s:&str)->fmt::Result{self.0.write_str(s)}fn write_char(&mut self,c:char)->fmt::Result{self.0.calls+=1;ev!("write_char",self.0.calls,c as u32);if self.0.calls==self.0.fail{Err(fmt::Error)}else{self.0.text.push(c);Ok(())}}}
#[derive(Default)]struct ObservedState{first:Option<String>,second:Option<String>,second_ok:Option<bool>}
struct Observed<'a,T>{value:&'a T,state:&'a RefCell<ObservedState>,repeat:bool}
impl<T:fmt::Debug> fmt::Debug for Observed<'_,T>{fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{let result=fmt::Debug::fmt(self.value,f);self.state.borrow_mut().first=Some(options(f));if self.repeat{let again=fmt::Debug::fmt(self.value,f);let mut state=self.state.borrow_mut();state.second=Some(options(f));state.second_ok=Some(again.is_ok())};result}}
fn render_to<T:fmt::Debug>(s:&mut Sink,state:&RefCell<ObservedState>,repeat:bool,chars:bool,style:&str,value:&T)->fmt::Result{let observed=Observed{value,state,repeat};if chars{render(&mut CharSink(s),style,&observed)}else{render(s,style,&observed)}}
fn render<T:fmt::Debug,W:fmt::Write>(s:&mut W,style:&str,value:&T)->fmt::Result{
 match style{
 "plain"=>write!(s,"{value:?}"),"pretty"=>write!(s,"{value:#?}"),"lower"=>write!(s,"{value:x?}"),"pretty_lower"=>write!(s,"{value:#x?}"),"upper"=>write!(s,"{value:X?}"),"pretty_upper"=>write!(s,"{value:#X?}"),
 "width"=>write!(s,"{value:10?}"),"left"=>write!(s,"{value:*<10?}"),"center"=>write!(s,"{value:λ^10?}"),"zero_plus"=>write!(s,"{value:+010?}"),"pretty_zero_plus"=>write!(s,"{value:+#010?}"),"minus"=>write!(s,"{value:-?}"),"precision"=>write!(s,"{value:.3?}"),"width_precision"=>write!(s,"{value:*>10.3?}"),"pretty_width_precision"=>write!(s,"{value:*>#10.3?}"),"nul_fill"=>write!(s,"{value:\0>10?}"),"pretty_nul_fill"=>write!(s,"{value:\0>#10?}"),_=>panic!("formatter style")
 }
}
pub(super)fn replay(r:&Value)->Value{
 HOOKS.with(|h|*h.borrow_mut()=Hooks{limit:n(r,"limit"),enabled:false,format_probe:true,..Hooks::default()});
 let mut steps=vec![];
 for a in r["actions"].as_array().unwrap(){HOOKS.with(|h|{let mut h=h.borrow_mut();h.events.clear();h.fail=text(a,"panic").to_owned();h.nth=n(a,"panic_at").max(1);h.count=0;h.enabled=true});let mut sink=Sink{calls:0,fail:n(a,"fail_at"),text:String::new()};let mut ok=false;let state=RefCell::new(ObservedState::default());
 let caught=catch_unwind(AssertUnwindSafe(||{match text(a,"op"){
 "format_list"|"format_map"=>{let v=IndexVec::<K,F>::from_raw(values(r,"values").into_iter().map(F).collect());ok=if text(a,"op")=="format_list"{render_to(&mut sink,&state,a.get("reuse").and_then(Value::as_bool).unwrap_or(false),a.get("char_writer").and_then(Value::as_bool).unwrap_or(false),text(a,"style"),&v).is_ok()}else{render_to(&mut sink,&state,a.get("reuse").and_then(Value::as_bool).unwrap_or(false),a.get("char_writer").and_then(Value::as_bool).unwrap_or(false),text(a,"style"),&v.debug_map_view()).is_ok()}},
 "format_option_map"|"format_compact"=>{let v=IndexVec::<K,Option<F>>::from_raw(values(r,"values").into_iter().map(|n|if n== -1000000{None}else{Some(F(n))}).collect());ok=if text(a,"op")=="format_compact"{render_to(&mut sink,&state,a.get("reuse").and_then(Value::as_bool).unwrap_or(false),a.get("char_writer").and_then(Value::as_bool).unwrap_or(false),text(a,"style"),&v.debug_map_view_compact()).is_ok()}else{render_to(&mut sink,&state,a.get("reuse").and_then(Value::as_bool).unwrap_or(false),a.get("char_writer").and_then(Value::as_bool).unwrap_or(false),text(a,"style"),&v.debug_map_view()).is_ok()}},_=>panic!("format operation")
 }}));let events=HOOKS.with(|h|h.borrow().events.clone());steps.push(json!({"value":{"ok":ok,"text":sink.text,"calls":sink.calls,"after_options":state.borrow().first,"second_options":state.borrow().second,"second_ok":state.borrow().second_ok},"panic":caught.is_err(),"events":events}));
 }HOOKS.with(|h|h.borrow_mut().enabled=false);json!({"steps":steps})
}
