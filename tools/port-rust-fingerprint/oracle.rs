// Development-only comparator against unchanged pinned Fingerprint sources.
// SPDX-License-Identifier: MIT OR Apache-2.0
use fingerprint_reference::*;
use std::cell::Cell;
use std::fmt::{self, Debug, Display};
use std::hash::{Hash, Hasher};
use std::io::{self, BufRead, Write};
use std::rc::Rc;
use std::cell::RefCell;

fn hex(bytes: &[u8]) -> String { bytes.iter().map(|b| format!("{b:02x}")).collect() }
fn unhex(text: &str) -> Vec<u8> { text.as_bytes().chunks_exact(2).map(|p| u8::from_str_radix(std::str::from_utf8(p).unwrap(),16).unwrap()).collect() }
fn pair(value: Fingerprint) -> String { let (a,b)=value.split(); format!("{},{}",a.as_u64(),b.as_u64()) }
fn panic_text(payload: Box<dyn std::any::Any + Send>) -> String {
    if let Some(text)=payload.downcast_ref::<String>() { return text.clone(); }
    if let Some(text)=payload.downcast_ref::<&str>() { return text.to_string(); }
    "non-string panic".into()
}
#[derive(Default)] struct Trace(Vec<u64>);
impl Hasher for Trace {
    fn finish(&self)->u64 {0}
    fn write(&mut self,_:&[u8]) {panic!("unexpected byte hash")}
    fn write_u64(&mut self,value:u64) {self.0.push(value)}
}
struct WrappedUnhasher(fingerprint_reference::unhash::Unhasher);
impl Hasher for WrappedUnhasher {
    fn finish(&self)->u64 {self.0.finish()}
    fn write(&mut self,bytes:&[u8]) {self.0.write(bytes)}
    fn write_u64(&mut self,value:u64) {self.0.write_u64(value)}
}
struct Sink { events:Vec<String>,fail:usize,chars:bool }
impl Sink {
    fn record(&mut self,kind:char,text:&str)->fmt::Result {
        let index=self.events.len(); self.events.push(format!("{kind}:{}",hex(text.as_bytes())));
        if index==self.fail {Err(fmt::Error)} else {Ok(())}
    }
}
impl fmt::Write for Sink {
    fn write_str(&mut self,text:&str)->fmt::Result {self.record('S',text)}
    fn write_char(&mut self,ch:char)->fmt::Result { let mut b=[0;4]; let t=ch.encode_utf8(&mut b); if self.chars {self.record('C',t)} else {self.write_str(t)} }
}
static FORMAT_STATE:std::sync::Mutex<(u32,u8)>=std::sync::Mutex::new((32,0));
fn capture(f:&fmt::Formatter<'_>) {
    let alignment=match f.align(){None=>0,Some(fmt::Alignment::Left)=>1,Some(fmt::Alignment::Right)=>2,Some(fmt::Alignment::Center)=>3};
    *FORMAT_STATE.lock().unwrap()=(f.fill() as u32,alignment);
}
fn format_state()->String {let (fill,align)=*FORMAT_STATE.lock().unwrap();format!("{fill}:{align}")}
struct Observed<'a,T>(&'a T);
impl<T:Debug> Debug for Observed<'_,T> {
    fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result {let result=Debug::fmt(self.0,f);capture(f);result}
}
impl<T:Display> Display for Observed<'_,T> {
    fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result {let result=Display::fmt(self.0,f);capture(f);result}
}
fn render<T:Debug+Display>(sink:&mut Sink,value:&T,pattern:usize)->fmt::Result {
    let value=Observed(value);
    match pattern {
        // FORMAT_ARMS
        _=>panic!("unknown format pattern"),
    }
}
fn event(events:&Rc<RefCell<Vec<String>>>,text:String) {
    if std::env::var_os("FERRUGO_FINGERPRINT_ABORT").is_some() {
        println!("{text}"); io::stdout().flush().unwrap();
    }
    events.borrow_mut().push(text);
}
struct Component { id:u8,value:u64,reads:Cell<u64>,events:Rc<RefCell<Vec<String>>>,panic_read:bool,panic_drop:bool }
impl FingerprintComponent for Component {
    fn as_u64(&self)->u64 {
        self.reads.set(self.reads.get()+1);
        event(&self.events,format!("read:{}:{}:{}",self.id,self.reads.get(),std::thread::panicking()));
        if self.panic_read {panic!("read");}
        self.value
    }
}
impl Drop for Component {
    fn drop(&mut self) {
        event(&self.events,format!("drop:{}:{}:{}",self.id,self.reads.get(),std::thread::panicking()));
        if self.panic_drop {panic!("drop");}
    }
}
fn construct(read:u8,drop:u8,events:&Rc<RefCell<Vec<String>>>)->Fingerprint {
    Fingerprint::new(
        Component{id:0,value:7,reads:Cell::new(0),events:events.clone(),panic_read:read==0,panic_drop:drop&1!=0},
        Component{id:1,value:11,reads:Cell::new(0),events:events.clone(),panic_read:read==1,panic_drop:drop&2!=0})
}
fn owned(read:u8,drop:u8)->String {
    let events=Rc::new(RefCell::new(vec![]));
    let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| construct(read,drop,&events)));
    let result=match result {Ok(value)=>format!("ok:{}",pair(value)),Err(p)=>format!("panic:{}",hex(panic_text(p).as_bytes()))};
    format!("{result}\t{}",events.borrow().join("|"))
}
struct Outer {read:u8,drop:u8,catch_inner:bool,events:Rc<RefCell<Vec<String>>>}
impl Drop for Outer {
    fn drop(&mut self) {
        event(&self.events,format!("outer-drop:{}",std::thread::panicking()));
        if self.catch_inner {
            let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||construct(self.read,self.drop,&self.events)));
            let text=match result {Ok(value)=>format!("inner:ok:{}",pair(value)),Err(p)=>format!("inner:panic:{}",hex(panic_text(p).as_bytes()))};
            event(&self.events,text);
        } else {
            let value=construct(self.read,self.drop,&self.events);
            event(&self.events,format!("inner:ok:{}",pair(value)));
        }
        event(&self.events,format!("outer-return:{}",std::thread::panicking()));
    }
}
fn inherited(read:u8,drop:u8,catch_inner:bool)->String {
    let events=Rc::new(RefCell::new(vec![]));
    let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||{
        let _outer=Outer{read,drop,catch_inner,events:events.clone()};panic!("outer");
    }));
    let text=match result {Ok(())=>"unexpected normal return".to_string(),Err(p)=>format!("panic:{}",hex(panic_text(p).as_bytes()))};
    format!("{text}\t{}",events.borrow().join("|"))
}
fn processing(read:u8,drop:u8)->String {
    let saved=std::sync::Arc::new(std::sync::Mutex::new(Vec::<String>::new()));let target=saved.clone();
    std::panic::set_hook(Box::new(move |_|{
        let events=Rc::new(RefCell::new(vec![]));event(&events,format!("hook-enter:{}",std::thread::panicking()));
        let _value=construct(read,drop,&events);
        event(&events,format!("hook-return:{}",std::thread::panicking()));
        *target.lock().unwrap()=events.borrow().clone();
    }));
    let result=std::panic::catch_unwind(||panic!("outer"));
    std::panic::set_hook(Box::new(|_|{}));
    let text=match result {Ok(())=>"unexpected normal return".to_string(),Err(p)=>format!("panic:{}",hex(panic_text(p).as_bytes()))};
    format!("{text}\t{}",saved.lock().unwrap().join("|"))
}
macro_rules! encoder_number {($name:ident,$ty:ty)=>{fn $name(&mut self,v:$ty){self.inner.$name(v)}};}
struct AuditEncoder {inner:MemEncoder,operations:Vec<String>}
impl Encoder for AuditEncoder {
    encoder_number!(emit_usize,usize);encoder_number!(emit_u128,u128);encoder_number!(emit_u64,u64);encoder_number!(emit_u32,u32);encoder_number!(emit_u16,u16);encoder_number!(emit_u8,u8);
    encoder_number!(emit_isize,isize);encoder_number!(emit_i128,i128);encoder_number!(emit_i64,i64);encoder_number!(emit_i32,i32);encoder_number!(emit_i16,i16);
    fn emit_raw_bytes(&mut self,bytes:&[u8]) {self.operations.push(format!("raw:{}",hex(bytes)));self.inner.emit_raw_bytes(bytes);}
}
macro_rules! decoder_number {($name:ident,$ty:ty)=>{fn $name(&mut self)->$ty{self.inner.$name()}};}
struct AuditDecoder<'a> {inner:MemDecoder<'a>,operations:Vec<String>,override_bytes:Option<&'a [u8]>}
impl Decoder for AuditDecoder<'_> {
    decoder_number!(read_usize,usize);decoder_number!(read_u128,u128);decoder_number!(read_u64,u64);decoder_number!(read_u32,u32);decoder_number!(read_u16,u16);decoder_number!(read_u8,u8);
    decoder_number!(read_isize,isize);decoder_number!(read_i128,i128);decoder_number!(read_i64,i64);decoder_number!(read_i32,i32);decoder_number!(read_i16,i16);
    fn read_raw_bytes(&mut self,length:usize)->&[u8] {
        self.operations.push(format!("raw:{length}"));
        if let Some(bytes)=self.override_bytes {bytes} else {self.inner.read_raw_bytes(length)}
    }
    fn peek_byte(&self)->u8 {self.inner.peek_byte()}
    fn position(&self)->usize {self.inner.position()}
}
fn decode(bytes:&[u8],position:usize,returned:Option<&[u8]>,packed:bool)->String {
    let mut decoder=match MemDecoder::new(bytes,position) {Ok(inner)=>AuditDecoder{inner,operations:vec![],override_bytes:returned},Err(())=>return "error".into()};
    let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| if packed {<Fingerprint as From<PackedFingerprint>>::from(PackedFingerprint::decode(&mut decoder))} else {Fingerprint::decode(&mut decoder)}));
    let status=match result {Ok(value)=>format!("ok:{}",pair(value)),Err(p)=>format!("panic:{}",hex(panic_text(p).as_bytes()))};
    format!("{status}:{}:{}",decoder.position(),decoder.operations.join("|"))
}
fn main() {
    std::panic::set_hook(Box::new(|_|{}));
    if let Some(request)=std::env::var_os("FERRUGO_FINGERPRINT_ABORT") {
        let text=request.to_str().unwrap();let f:Vec<_>=text.split(',').collect();
        if f.len()==3 && f[2]=="processing" {let _=processing(f[0].parse().unwrap(),f[1].parse().unwrap());}
        else if f.len()==3 {let _=inherited(f[0].parse().unwrap(),f[1].parse().unwrap(),f[2].parse().unwrap());} else {let _=owned(f[0].parse().unwrap(),f[1].parse().unwrap());}
        panic!("expected abort");
    }
    let mut out=io::BufWriter::new(io::stdout().lock());
    writeln!(out,"L\t{}\t{}\t{}\t{}",std::mem::size_of::<Fingerprint>(),std::mem::align_of::<Fingerprint>(),std::mem::size_of::<PackedFingerprint>(),std::mem::align_of::<PackedFingerprint>()).unwrap();
    for line in io::stdin().lock().lines() {
        let line=line.unwrap();let f:Vec<_>=line.split('\t').collect();
        match f[0] {
            "V"=>{
                let a=Fingerprint::new(f[1].parse::<u64>().unwrap(),f[2].parse::<u64>().unwrap());
                let b=Fingerprint::new(Hash64::new(f[3].parse().unwrap()),Hash64::new(f[4].parse().unwrap()));
                let packed=PackedFingerprint::from(a);let other=PackedFingerprint::from(b);
                let mut clone=Fingerprint::ZERO;clone.clone_from(&a);assert_eq!(clone,a.clone());
                let mut packed_clone=PackedFingerprint::from(Fingerprint::ZERO);packed_clone.clone_from(&packed);assert_eq!(packed_clone,packed.clone());
                assert_eq!(<Fingerprint as From<PackedFingerprint>>::from(packed),a);assert_eq!(Fingerprint::from_le_bytes(a.to_le_bytes()),a);
                assert_eq!(<Fingerprint as FromStableHash>::from(StableHasherHash([a.split().0.as_u64(),a.split().1.as_u64()])),a);
                assert!(<Fingerprint as StableOrd>::CAN_USE_UNSTABLE_SORT);
                assert_eq!(a.stable_cmp(&b),a.cmp(&b));
                let mut trace=Trace::default();a.hash(&mut trace);packed.hash(&mut trace);
                let mut unhash=fingerprint_reference::unhash::Unhasher::default();a.hash(&mut unhash);
                let mut encoder=AuditEncoder{inner:MemEncoder::new(),operations:vec![]};a.encode(&mut encoder);packed.encode(&mut encoder);
                let bytes=encoder.inner.finish();
                writeln!(out,"{line}\t{}\t{}\t{}\t{:?}\t{:?}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}",pair(a.combine(b)),pair(a.combine_commutative(b)),a.to_smaller_hash().as_u64(),a.cmp(&b),packed.cmp(&other),a.to_hex(),a,hex(format!("{a:?}").as_bytes()),hex(format!("{packed:?}").as_bytes()),hex(&bytes),trace.0.iter().map(u64::to_string).collect::<Vec<_>>().join(","),unhash.finish(),encoder.operations.join("|")).unwrap();
            }
            "F"=>{
                let a=Fingerprint::new(f[1].parse::<u64>().unwrap(),f[2].parse::<u64>().unwrap());let pattern:usize=f[3].parse().unwrap();let packed=f[4]=="true";
                for chars in [false,true] {
                    let mut sink=Sink{events:vec![],fail:usize::MAX,chars};let status=if packed {render(&mut sink,&PackedFingerprint::from(a),pattern)} else {render(&mut sink,&a,pattern)};assert!(status.is_ok());
                    writeln!(out,"{line}\t{chars}\t-1\ttrue\t{}\t{}",sink.events.join("|"),format_state()).unwrap();
                    for fail in 0..=sink.events.len() {
                        let mut sink=Sink{events:vec![],fail,chars};let status=if packed {render(&mut sink,&PackedFingerprint::from(a),pattern)} else {render(&mut sink,&a,pattern)};
                        writeln!(out,"{line}\t{chars}\t{fail}\t{}\t{}\t{}",status.is_ok(),sink.events.join("|"),format_state()).unwrap();
                    }
                }
            }
            "O"=>writeln!(out,"{line}\t{}",owned(f[1].parse().unwrap(),f[2].parse().unwrap())).unwrap(),
            "I"=>writeln!(out,"{line}\t{}",inherited(f[1].parse().unwrap(),f[2].parse().unwrap(),f[3].parse().unwrap())).unwrap(),
            "Q"=>writeln!(out,"{line}\t{}",processing(f[1].parse().unwrap(),f[2].parse().unwrap())).unwrap(),
            "D"=>{
                let mut bytes=unhex(f[1]);if f[3]=="true" {bytes.extend(MAGIC_END_BYTES)}let position:usize=f[2].parse().unwrap();let returned=if f[4]=="-" {None} else {Some(unhex(f[4]))};
                let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||decode(&bytes,position,returned.as_deref(),f[5]=="true"))).unwrap_or_else(|p|format!("constructor-panic:{}",hex(panic_text(p).as_bytes())));
                writeln!(out,"{line}\t{result}").unwrap();
            }
            "U"=>{
                let mut hasher=fingerprint_reference::unhash::Unhasher::default();let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||for value in f[1].split(',') {hasher.write_u64(value.parse().unwrap())}));
                let status=match result {Ok(())=>"ok".to_string(),Err(p)=>format!("panic:{}",hex(panic_text(p).as_bytes()))};writeln!(out,"{line}\t{status}\t{}",hasher.finish()).unwrap();
            }
            "W"=>{
                let value=Fingerprint::new(f[1].parse::<u64>().unwrap(),f[2].parse::<u64>().unwrap());
                let mut hasher=WrappedUnhasher(fingerprint_reference::unhash::Unhasher::default());
                let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||value.hash(&mut hasher)));
                let status=match result {Ok(())=>"ok".to_string(),Err(p)=>format!("panic:{}",hex(panic_text(p).as_bytes()))};
                writeln!(out,"{line}\t{status}\t{}",hasher.finish()).unwrap();
            }
            "B"=>{
                let mut hasher=fingerprint_reference::unhash::Unhasher::default();
                let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||hasher.write(&[1,2,3])));
                let status=match result {Ok(())=>"ok".to_string(),Err(p)=>format!("panic:{}",hex(panic_text(p).as_bytes()))};
                writeln!(out,"{line}\t{status}\t{}",hasher.finish()).unwrap();
            }
            "H"=>{
                let value=Fingerprint::new(f[1].parse::<u64>().unwrap(),f[2].parse::<u64>().unwrap());let prefix=unhex(f[3]);let ops:Vec<u64>=f[4].split(',').map(|n|n.parse().unwrap()).collect();let stable_ops:Vec<u64>=f[5].split(',').map(|n|n.parse().unwrap()).collect();
                let mut a=std::collections::hash_map::DefaultHasher::new();a.write(&prefix);value.hash(&mut a);let mut b=std::collections::hash_map::DefaultHasher::new();b.write(&prefix);for op in ops {b.write_u64(op)}assert_eq!(a.finish(),b.finish());
                let mut a_stable=StableHasher::new();a_stable.write(&prefix);value.stable_hash(&mut DummyStableHashCtxt,&mut a_stable);let a_stable:Hash128=a_stable.finish();
                let mut b_stable=StableHasher::new();b_stable.write(&prefix);for op in stable_ops {b_stable.write_u64(op)}let b_stable:Hash128=b_stable.finish();assert_eq!(a_stable,b_stable);
                writeln!(out,"{line}\t{}\t{}",a.finish(),a_stable.as_u128()).unwrap();
            }
            _=>panic!("unknown request"),
        }
    }
}
