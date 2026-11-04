// Development-only caller witnesses around the unchanged pinned Hash impls.
use def_id_reference::{fingerprint::{Fingerprint, PackedFingerprint}, unhash::Unhasher};
use rustc_hashes::{Hash64, Hash128};
use std::{cell::RefCell, hash::{Hash, Hasher}, io::{self, BufRead}, panic::{self, AssertUnwindSafe}, rc::Rc};
use serde_json::{json, Value};

fn event(events:&Rc<RefCell<Vec<String>>>, text:String) {
    println!("E:{text}"); events.borrow_mut().push(text);
}
fn hex(bytes:&[u8])->String { bytes.iter().map(|b|format!("{b:02x}")).collect() }
fn number(q:&Value,name:&str)->u64 {q[name].as_u64().unwrap_or(0)}
fn text<'a>(q:&'a Value,name:&str)->&'a str {q[name].as_str().unwrap_or("")}
fn payload(value:Box<dyn std::any::Any+Send>)->String {
    if let Some(v)=value.downcast_ref::<String>() {v.clone()}
    else if let Some(v)=value.downcast_ref::<&str>() {(*v).to_owned()} else {"opaque".into()}
}
fn hash<H:Hasher>(q:&Value, h:&mut H) {
    let a=number(q,"first");let b=number(q,"second");
    match text(q,"kind") {
        "hash64"=>Hash64::new(a).hash(h),
        "hash128"=>Hash128::new((b as u128)<<64|a as u128).hash(h),
        "fingerprint"=>Fingerprint::new(a,b).hash(h),
        "packed"=>PackedFingerprint::from(Fingerprint::new(a,b)).hash(h),
        "hash64_slice"=>Hash::hash_slice(&[Hash64::new(a),Hash64::new(b)],h),
        "hash128_slice"=>Hash::hash_slice(&[Hash128::new((b as u128)<<64|a as u128),Hash128::new((a as u128)<<64|b as u128)],h),
        "fingerprint_slice"=>Hash::hash_slice(&[Fingerprint::new(a,b),Fingerprint::new(b,a)],h),
        "packed_slice"=>Hash::hash_slice(&[PackedFingerprint::from(Fingerprint::new(a,b)),PackedFingerprint::from(Fingerprint::new(b,a))],h),
        "hash64_empty"=><Hash64 as Hash>::hash_slice(&[],h),
        "hash128_empty"=><Hash128 as Hash>::hash_slice(&[],h),
        "fingerprint_empty"=><Fingerprint as Hash>::hash_slice(&[],h),
        "packed_empty"=><PackedFingerprint as Hash>::hash_slice(&[],h),
        _=>panic!("unknown value"),
    }
}
struct Guard{events:Rc<RefCell<Vec<String>>>, call:usize}
impl Drop for Guard {fn drop(&mut self){event(&self.events,format!("local-drop:{}:{}",self.call,std::thread::panicking()))}}
struct State{events:Rc<RefCell<Vec<String>>>, bytes:Vec<u8>, value:u64, calls:usize, fail:i64, expected:*const State}
impl State {
    fn write(&mut self, bytes:&[u8]) {
        event(&self.events,format!("bytes:{}:{}:{}",hex(bytes),std::thread::panicking(),std::ptr::eq(self,self.expected)));
        let call=self.calls;self.calls+=1;let _guard=Guard{events:self.events.clone(),call};
        if call as i64==self.fail {panic!("write")}
        for byte in bytes {self.value=self.value.rotate_left(5)^u64::from(*byte)}
        self.bytes.extend_from_slice(bytes);
    }
    fn typed(&mut self,value:u64) {
        event(&self.events,format!("u64:{value}:{}:{}",std::thread::panicking(),std::ptr::eq(self,self.expected)));
        let call=self.calls;self.calls+=1;let _guard=Guard{events:self.events.clone(),call};
        if call as i64==self.fail {panic!("write")}
        self.value=self.value.rotate_left(9)^value^0xa55aa55aa55aa55a;
    }
    fn finish(&self)->u64 {event(&self.events,format!("finish:{}",std::thread::panicking()));self.value}
}
impl Drop for State {fn drop(&mut self){event(&self.events,format!("state-drop:{}:{}:{}",self.calls,self.value,std::thread::panicking()))}}
struct Raw(State);impl Hasher for Raw {fn write(&mut self,b:&[u8]){self.0.write(b)}fn finish(&self)->u64{self.0.finish()}}
struct Typed(State);impl Hasher for Typed {fn write(&mut self,b:&[u8]){self.0.write(b)}fn finish(&self)->u64{self.0.finish()}fn write_u64(&mut self,v:u64){self.0.typed(v)}}
struct Wrapper(Unhasher);impl Hasher for Wrapper {fn write(&mut self,b:&[u8]){self.0.write(b)}fn finish(&self)->u64{self.0.finish()}fn write_u64(&mut self,v:u64){self.0.write_u64(v)}}
fn custom(q:&Value,events:&Rc<RefCell<Vec<String>>>)->Value {
    let state=State{events:events.clone(),bytes:vec![0xfe,0xed],value:number(q,"seed"),calls:0,fail:q["fail"].as_i64().unwrap(),expected:std::ptr::null()};
    macro_rules! run{($h:expr)=>{{let mut h=$h;h.0.expected=&h.0 as *const State;
        let error=panic::catch_unwind(AssertUnwindSafe(||hash(q,&mut h))).err().map(payload);
        event(events,format!("after-hash:{}",std::thread::panicking()));
        let result=json!({"panic":error,"value":h.finish(),"calls":h.0.calls,"bytes":hex(&h.0.bytes)});
        drop(h);result
    }}}
    if text(q,"hasher")=="raw" {run!(Raw(state))} else {run!(Typed(state))}
}
fn execute(q:&Value) {
    let events=Rc::new(RefCell::new(Vec::new()));
    event(&events,format!("enter:{}",std::thread::panicking()));
    let result=if ["raw","typed"].contains(&text(q,"hasher")) {custom(q,&events)} else {
        let mut h=Unhasher::default();h.write_u64(number(q,"seed"));
        let error=match text(q,"hasher") {
            "unhash"=>panic::catch_unwind(AssertUnwindSafe(||hash(q,&mut h))),
            "ref_unhash"=>panic::catch_unwind(AssertUnwindSafe(||hash(q,&mut &mut h))),
            "wrapper_unhash"=>{let mut wrapper=Wrapper(h);let result=panic::catch_unwind(AssertUnwindSafe(||hash(q,&mut wrapper)));h=wrapper.0;result},
            _=>panic!("unknown hasher"),
        }.err().map(payload);
        event(&events,format!("after-hash:{}",std::thread::panicking()));
        json!({"panic":error,"value":h.finish()})
    };
    println!("R:{}",json!({"result":result,"events":events.borrow().clone()}));
}
struct Ambient<F:FnOnce()>(Option<F>);impl<F:FnOnce()>Drop for Ambient<F>{fn drop(&mut self){self.0.take().unwrap()()}}
fn dispatch(q:Value) {
    match text(&q,"context") {
        "normal"=>execute(&q),
        "inherited"=>{let _=panic::catch_unwind(AssertUnwindSafe(||{let _guard=Ambient(Some(||execute(&q)));panic!("ambient")}));},
        "processing"=>{panic::set_hook(Box::new(move |_|execute(&q)));let _=panic::catch_unwind(||panic!("ambient"));panic::set_hook(Box::new(|_|{}));},
        _=>panic!("unknown context"),
    }
}
fn main(){panic::set_hook(Box::new(|_|{}));for line in io::stdin().lock().lines(){dispatch(serde_json::from_str(&line.unwrap()).unwrap())}}
