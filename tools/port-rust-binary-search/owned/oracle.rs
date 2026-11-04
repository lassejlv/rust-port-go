// SPDX-License-Identifier: MIT OR Apache-2.0
// Development-only oracle. The search body is the verified untouched source.
mod original { include!(env!("BINARY_SEARCH_SOURCE")); }

use std::cell::{Cell,RefCell};
use std::cmp::Ordering;
use std::io::{self,BufRead,Write};
use std::panic::{catch_unwind,AssertUnwindSafe};
use std::rc::Rc;

#[derive(Clone)]
struct Request {
    id:String,zst:bool,embedded:bool,length:usize,query:i64,zst_key:i64,context:u8,
    fail:[usize;6],values:Vec<i64>,
}
impl Request {
    fn parse(line:&str)->Self {
        let fields:Vec<_>=line.split('\t').collect();assert_eq!(fields.len(),13);
        let values=if fields[12].is_empty(){vec![]}else{fields[12].split(',').map(|v|v.parse().unwrap()).collect()};
        let request=Self{id:fields[0].into(),zst:fields[1]!="0",embedded:fields[1]=="2",length:fields[2].parse().unwrap(),query:fields[3].parse().unwrap(),zst_key:fields[4].parse().unwrap(),context:fields[5].parse().unwrap(),fail:std::array::from_fn(|i|fields[6+i].parse().unwrap()),values};
        assert!(request.values.len()<=128);assert!(request.context<=3);
        request
    }
}
#[derive(Default)]
struct Meta {extracts:Cell<usize>,operators:Cell<usize>,returned:Cell<usize>}
struct Element {value:i64,index:usize,meta:Rc<Meta>}
struct Shared {
    request:Request,events:RefCell<Vec<String>>,extracts:Cell<usize>,keys:Cell<usize>,
    key_drops:Cell<usize>,fn_drops:Cell<usize>,query_drops:Cell<usize>,
    operators:[Cell<usize>;3],query_ptr:Cell<*const Key>,env_ptr:Cell<*const Environment>,
    data_ptrs:Vec<*const Element>,unit_ptr:*const (),metadata:Vec<Rc<Meta>>,
}
impl Shared {
    fn event(&self,text:String) {
        if std::env::var_os("FERRUGO_BINARY_OWNED_STREAM").is_some() {
            println!("{text}");io::stdout().flush().unwrap();
        }
        let mut events=self.events.borrow_mut();
        assert!(events.len()<2048,"owned oracle event bound exceeded");
        events.push(text);
    }
}
struct Key {
    value:i64,id:usize,query:bool,ticks:Cell<usize>,borrowed:Cell<*const Key>,
    data:Option<Rc<Meta>>,shared:Rc<Shared>,
}
impl Key {
    fn operator(&self,other:&Self,operator:usize)->bool {
        let shared=&self.shared;
        self.ticks.set(self.ticks.get()+1);other.ticks.set(other.ticks.get()+1);
        if let Some(meta)=&self.data {meta.operators.set(meta.operators.get()+1);}
        let count=shared.operators[operator].get()+1;shared.operators[operator].set(count);
        let query_same=std::ptr::eq(other,shared.query_ptr.get());
        let temp_same=self.borrowed.get().is_null()||std::ptr::eq(self,self.borrowed.get());
        self.borrowed.set(self as *const Self);
        shared.event(format!("op:{}:{}:{}:{}:{}:{}:{}:{}",["lt","ne","le"][operator],count,self.id,self.ticks.get(),other.ticks.get(),query_same,temp_same,std::thread::panicking()));
        if shared.request.fail[operator+1]==count {panic!("{}",["lt","ne","le"][operator]);}
        match operator {0=>self.value<other.value,1=>self.value!=other.value,_=>self.value<=other.value}
    }
}
impl PartialEq for Key {
    fn eq(&self,other:&Self)->bool {self.value==other.value}
    fn ne(&self,other:&Self)->bool {self.operator(other,1)}
}
impl Eq for Key {}
impl PartialOrd for Key {
    fn partial_cmp(&self,other:&Self)->Option<Ordering>{Some(self.cmp(other))}
    fn lt(&self,other:&Self)->bool {self.operator(other,0)}
    fn le(&self,other:&Self)->bool {self.operator(other,2)}
}
impl Ord for Key {fn cmp(&self,other:&Self)->Ordering {self.value.cmp(&other.value)}}
impl Drop for Key {
    fn drop(&mut self) {
        if self.query {self.shared.query_drops.set(self.shared.query_drops.get()+1);return;}
        let shared=&self.shared;let count=shared.key_drops.get()+1;shared.key_drops.set(count);
        let same=std::ptr::eq(self,self.borrowed.get());
        shared.event(format!("key-drop:{}:{}:{}:{}:{}",self.id,count,self.ticks.get(),same,std::thread::panicking()));
        if shared.request.fail[4]==count {panic!("key-drop");}
    }
}
struct Environment {shared:Rc<Shared>,calls:Cell<usize>}
impl Environment {
    fn extract(&self,value:i64,label:&str,data:Option<Rc<Meta>>,data_same:bool)->Key {
        let shared=&self.shared;let count=shared.extracts.get()+1;shared.extracts.set(count);
        self.calls.set(self.calls.get()+1);
        let env_same=shared.env_ptr.get().is_null()||std::ptr::eq(self,shared.env_ptr.get());
        shared.env_ptr.set(self as *const Self);
        if let Some(meta)=&data {meta.extracts.set(meta.extracts.get()+1);}
        shared.event(format!("extract:{}:{}:{}:{}:{}:{}",count,label,self.calls.get(),env_same,data_same,std::thread::panicking()));
        if shared.request.fail[0]==count {panic!("extract");}
        let id=shared.keys.get()+1;shared.keys.set(id);
        Key{value,id,query:false,ticks:Cell::new(0),borrowed:Cell::new(std::ptr::null()),data,shared:shared.clone()}
    }
    fn element(&self,element:&Element)->Key {
        let same=std::ptr::eq(element,self.shared.data_ptrs[element.index]);
        self.extract(element.value,&element.index.to_string(),Some(element.meta.clone()),same)
    }
    fn unit(&self,element:&())->Key {
        self.extract(self.shared.request.zst_key,"z",None,std::ptr::eq(element,self.shared.unit_ptr))
    }
}
impl Drop for Environment {
    fn drop(&mut self) {
        let shared=&self.shared;shared.fn_drops.set(shared.fn_drops.get()+1);
        let same=shared.env_ptr.get().is_null()||std::ptr::eq(self,shared.env_ptr.get());
        shared.event(format!("fn-drop:{}:{}:{}:{}",shared.extracts.get(),self.calls.get(),same,std::thread::panicking()));
        if shared.request.fail[5]!=0 {panic!("fn-drop");}
    }
}
#[derive(Clone)]
struct View {start:usize,length:usize,alias:bool}
struct Outcome {status:&'static str,message:String,view:Option<View>}
fn panic_text(payload:Box<dyn std::any::Any+Send>)->String {
    if let Some(text)=payload.downcast_ref::<String>() {return text.clone();}
    if let Some(text)=payload.downcast_ref::<&str>() {return text.to_string();}
    "non-string panic".into()
}
fn invoke(shared:&Rc<Shared>,data:&[Element],units:&[()],query:&Key)->View {
    let env=Environment{shared:shared.clone(),calls:Cell::new(0)};
    if shared.request.zst {
        let result=original::binary_search_slice(units,move|e|env.unit(e),query);
        let alias=!result.is_empty()&&std::ptr::eq(&result[0],&units[0]);
        View{start:0,length:result.len(),alias}
    } else {
        let result=original::binary_search_slice(data,move|e|env.element(e),query);
        let start=result.first().map(|e|e.index).unwrap_or(0);
        let alias=!result.is_empty()&&std::ptr::eq(&result[0],&data[start]);
        if !result.is_empty() {result[0].meta.returned.set(result[0].meta.returned.get()+1);}
        View{start,length:result.len(),alias}
    }
}
fn caught(shared:&Rc<Shared>,data:&[Element],units:&[()],query:&Key)->Outcome {
    match catch_unwind(AssertUnwindSafe(||invoke(shared,data,units,query))) {
        Ok(view)=>Outcome{status:"ok",message:String::new(),view:Some(view)},
        Err(payload)=>Outcome{status:"panic",message:panic_text(payload),view:None},
    }
}
struct Outer<'a> {
    shared:Rc<Shared>,data:&'a[Element],units:&'a[()],query:&'a Key,
    result:&'a RefCell<Option<Outcome>>,catch_inner:bool,
}
impl Drop for Outer<'_> {
    fn drop(&mut self) {
        self.shared.event(format!("outer-enter:{}",std::thread::panicking()));
        let outcome=if self.catch_inner {caught(&self.shared,self.data,self.units,self.query)} else {
            Outcome{status:"ok",message:String::new(),view:Some(invoke(&self.shared,self.data,self.units,self.query))}
        };
        self.shared.event(format!("outer-inner:{}:{}",outcome.status,outcome.message));
        *self.result.borrow_mut()=Some(outcome);
        self.shared.event(format!("outer-return:{}",std::thread::panicking()));
    }
}
fn string(text:&str)->String {
    let mut out=String::from("\"");
    for ch in text.chars() {match ch {'"'=>out.push_str("\\\""),'\\'=>out.push_str("\\\\"),'\n'=>out.push_str("\\n"),'\r'=>out.push_str("\\r"),'\t'=>out.push_str("\\t"),_=>out.push(ch)}}
    out.push('"');out
}
fn run_case(request:Request)->String {
    let metadata=request.values.iter().map(|_|Rc::new(Meta::default())).collect::<Vec<_>>();
    let data=request.values.iter().enumerate().map(|(index,value)|Element{value:*value,index,meta:metadata[index].clone()}).collect::<Vec<_>>();
    // This is a safe standard Vec construction. Unit elements require no
    // allocation, even when the logical length is usize::MAX.
    #[repr(C)]
    struct Holder {values:[();4],anchor:u8}
    let holder=Holder{values:[();4],anchor:0x7b};
    let allocated_units=if request.zst&&!request.embedded {vec![();request.length]}else{vec![]};
    let units=if request.embedded {assert!(request.length<=4);&holder.values[..request.length]}else{&allocated_units};
    let shared=Rc::new(Shared{request:request.clone(),events:RefCell::new(vec![]),extracts:Cell::new(0),keys:Cell::new(0),key_drops:Cell::new(0),fn_drops:Cell::new(0),query_drops:Cell::new(0),operators:std::array::from_fn(|_|Cell::new(0)),query_ptr:Cell::new(std::ptr::null()),env_ptr:Cell::new(std::ptr::null()),data_ptrs:data.iter().map(|e|e as *const Element).collect(),unit_ptr:units.as_ptr(),metadata});
    let query=Key{value:request.query,id:0,query:true,ticks:Cell::new(0),borrowed:Cell::new(std::ptr::null()),data:None,shared:shared.clone()};
    shared.query_ptr.set(&query as *const Key);
    let outcome=if request.context==1||request.context==2 {
        let result=RefCell::new(None);
        let outer=catch_unwind(AssertUnwindSafe(||{
            let _guard=Outer{shared:shared.clone(),data:&data,units:&units,query:&query,result:&result,catch_inner:request.context==1};
            panic!("outer");
        }));
        match outer {Err(payload)=>shared.event(format!("outer-caught:{}",panic_text(payload))),Ok(())=>panic!("outer panic disappeared")}
        result.into_inner().expect("outer destructor did not save result")
    } else {caught(&shared,&data,&units,&query)};
    let trace=shared.events.borrow().iter().map(|e|string(e)).collect::<Vec<_>>().join(",");
    let view=match outcome.view {Some(v)=>format!("{{\"start\":\"{}\",\"length\":\"{}\",\"alias\":{}}}",v.start,v.length,v.alias),None=>"null".into()};
    let data_state=shared.metadata.iter().map(|m|format!("[{},{},{}]",m.extracts.get(),m.operators.get(),m.returned.get())).collect::<Vec<_>>().join(",");
    let response=format!("{{\"id\":{},\"status\":{},\"message\":{},\"view\":{},\"trace\":[{}],\"snapshot\":{{\"extracts\":{},\"keys\":{},\"keyDrops\":{},\"fnDrops\":{},\"queryDrops\":{},\"queryTicks\":{},\"operators\":[{},{},{}],\"data\":[{}]}}}}",string(&request.id),string(outcome.status),string(&outcome.message),view,trace,shared.extracts.get(),shared.keys.get(),shared.key_drops.get(),shared.fn_drops.get(),shared.query_drops.get(),query.ticks.get(),shared.operators[0].get(),shared.operators[1].get(),shared.operators[2].get(),data_state);
    // Query is caller-owned. Its destructor runs only after the response's
    // post-search liveness snapshot has observed queryDrops == 0.
    response
}
fn execute(request:Request)->String {
    if request.context!=3 {return run_case(request);}
    let saved=std::sync::Arc::new(std::sync::Mutex::new(None::<String>));let output=saved.clone();
    std::panic::set_hook(Box::new(move |_|{
        let response=run_case(request.clone());
        *output.lock().unwrap()=Some(response);
    }));
    let _=catch_unwind(||panic!("processing"));
    std::panic::set_hook(Box::new(|_|{}));
    let response=saved.lock().unwrap().take().expect("processing hook did not return");
    response
}
fn main() {
    std::panic::set_hook(Box::new(|_|{}));
    for line in io::stdin().lock().lines() {
        let line=line.unwrap();if line.is_empty(){continue;}
        let response=execute(Request::parse(&line));
        if std::env::var_os("FERRUGO_BINARY_OWNED_STREAM").is_none() {println!("{response}");} else {println!("RESULT\t{response}");}
    }
}
