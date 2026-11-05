// Authored input types; actual unchanged rustc_serialize container impls and
// actual unchanged Unord derives run through the complete native crate graph.
use rustc_data_structures::{fx::FxHasher,unord::{UnordMap,UnordSet,UnordItems,UnordBag}};
use rustc_serialize::{Decodable,Decoder};
use std::{cell::RefCell,collections::{HashMap,HashSet},hash::{Hash,Hasher,BuildHasher},io::{self,Write},panic::{catch_unwind,AssertUnwindSafe}};
#[derive(Clone,Copy)]struct Config{kind:usize,length:usize,seed:u64,read_fail:isize,hash_fail:isize,eq_fail:isize,drop_fail:isize,default_fail:bool}
struct World{config:Config,events:Vec<String>,hashes:usize,eqs:usize,drops:usize}
thread_local!{static WORLD:RefCell<World>=const{RefCell::new(World{config:Config{kind:0,length:0,seed:0,read_fail:-1,hash_fail:-1,eq_fail:-1,drop_fail:-1,default_fail:false},events:Vec::new(),hashes:0,eqs:0,drops:0})};}
fn p()->bool{std::thread::panicking()}
fn event(value:String){WORLD.with(|w|w.borrow_mut().events.push(value))}
fn config()->Config{WORLD.with(|w|w.borrow().config)}
fn random(s:&mut u64)->u64{*s^=*s<<13;*s^=*s>>7;*s^=*s<<17;*s}
struct Reader{next:usize,seed:u64}
impl Decoder for Reader{
fn read_usize(&mut self)->usize{let c=config();event(format!("LEN:{}:{}",c.length,p()));c.length}
fn read_u64(&mut self)->u64{let index=self.next;self.next+=1;let bits=random(&mut self.seed);let kind=config().kind;let value=if kind%2==1||index%2==0{bits%5}else{bits};event(format!("R:{index}:{value}:{}",p()));if index as isize==config().read_fail{std::panic::panic_any("read")};value}
fn read_u128(&mut self)->u128{self.read_u64()as u128}fn read_u32(&mut self)->u32{self.read_u64()as u32}fn read_u16(&mut self)->u16{self.read_u64()as u16}fn read_u8(&mut self)->u8{self.read_u64()as u8}
fn read_isize(&mut self)->isize{self.read_u64()as isize}fn read_i128(&mut self)->i128{self.read_u64()as i128}fn read_i64(&mut self)->i64{self.read_u64()as i64}fn read_i32(&mut self)->i32{self.read_u64()as i32}fn read_i16(&mut self)->i16{self.read_u64()as i16}
fn read_raw_bytes(&mut self,_:usize)->&[u8]{panic!("unexpected raw read")}fn peek_byte(&self)->u8{0}fn position(&self)->usize{8+self.next*8}}
#[repr(C)]struct K{key:u64,id:u64}
#[repr(C)]struct V{value:u64,id:u64}
impl Decodable<Reader> for K{fn decode(d:&mut Reader)->Self{let id=d.next as u64;let key=d.read_u64();event(format!("K:{id}:{key}:{}",p()));Self{key,id}}}
impl Decodable<Reader> for V{fn decode(d:&mut Reader)->Self{let id=d.next as u64;let value=d.read_u64();event(format!("V:{id}:{value}:{}",p()));Self{value,id}}}
impl Hash for K{fn hash<H:Hasher>(&self,h:&mut H){let index=WORLD.with(|w|{let mut w=w.borrow_mut();let n=w.hashes;w.hashes+=1;n});event(format!("HASH:{}:{}:{}",self.id,self.key,p()));h.write_u64(self.key);if index as isize==config().hash_fail{std::panic::panic_any("hash")}}}
impl PartialEq for K{fn eq(&self,b:&Self)->bool{let index=WORLD.with(|w|{let mut w=w.borrow_mut();let n=w.eqs;w.eqs+=1;n});event(format!("EQ:{}:{}:{}",self.id,b.id,p()));if index as isize==config().eq_fail{std::panic::panic_any("eq")};self.key==b.key}}
impl Eq for K{}
fn element_drop(kind:&str,id:u64){let index=WORLD.with(|w|{let mut w=w.borrow_mut();let n=w.drops;w.drops+=1;n});event(format!("D:{kind}:{id}:{}",p()));if index as isize==config().drop_fail{std::panic::panic_any("drop")}}
impl Drop for K{fn drop(&mut self){element_drop("K",self.id)}}impl Drop for V{fn drop(&mut self){element_drop("V",self.id)}}
struct H(FxHasher);
impl Hasher for H{fn write(&mut self,bytes:&[u8]){self.0.write(bytes)}fn write_u64(&mut self,value:u64){event(format!("W:{value}:{}",p()));self.0.write_u64(value)}fn finish(&self)->u64{let value=self.0.finish();event(format!("FIN:{value}:{}",p()));value}}
impl Drop for H{fn drop(&mut self){event(format!("HD:{}:{}",self.0.finish(),p()));}}
#[derive(Clone)]struct B;
impl Default for B{fn default()->Self{event(format!("DEFAULT:{}",p()));if config().default_fail{std::panic::panic_any("default")};Self}}
impl BuildHasher for B{type Hasher=H;fn build_hasher(&self)->H{event(format!("BUILD:{}",p()));H(FxHasher::default())}}
impl Drop for B{fn drop(&mut self){event(format!("BD:{}",p()));}}
fn map_snapshot<'a>(i:impl Iterator<Item=(&'a K,&'a V)>)->String{format!("{:?}",UnordItems::new(i).map(|(k,v)|(k.key,k.id,v.value,v.id)).collect::<UnordBag<_>>())}
fn set_snapshot<'a>(i:impl Iterator<Item=&'a K>)->String{format!("{:?}",UnordItems::new(i).map(|k|(k.key,k.id)).collect::<UnordBag<_>>())}
fn payload(value:Box<dyn std::any::Any+Send>)->String{value.downcast_ref::<&str>().map(|s|(*s).to_owned()).or_else(||value.downcast_ref::<String>().cloned()).unwrap_or_else(||"other".into())}
fn run(c:Config)->(String,String,String,usize){WORLD.with(|w|*w.borrow_mut()=World{config:c,events:vec![],hashes:0,eqs:0,drops:0});let mut d=Reader{next:0,seed:c.seed};let mut snapshot="-".to_owned();let failure=catch_unwind(AssertUnwindSafe(||match c.kind{
0=>{let m=HashMap::<K,V,B>::decode(&mut d);snapshot=map_snapshot(m.iter());drop(m)},1=>{let m=HashSet::<K,B>::decode(&mut d);snapshot=set_snapshot(m.iter());drop(m)},2=>{let m=UnordMap::<K,V>::decode(&mut d);snapshot=format!("{:?}",m.items().map(|(k,v)|(k.key,k.id,v.value,v.id)).collect::<UnordBag<_>>());drop(m)},3=>{let m=UnordSet::<K>::decode(&mut d);snapshot=format!("{:?}",m.items().map(|k|(k.key,k.id)).collect::<UnordBag<_>>());drop(m)},4=>{let m=Vec::<K>::decode(&mut d);snapshot=format!("Vec {{ len: {}, capacity: {}, values: {} }}",m.len(),m.capacity(),set_snapshot(m.iter()));drop(m)},5=>{let m=UnordBag::<K>::decode(&mut d);snapshot=format!("{:?}",m.items().map(|k|(k.key,k.id)).collect::<UnordBag<_>>());drop(m)},_=>unreachable!()})).err().map(payload).unwrap_or_default();let events=WORLD.with(|w|w.borrow().events.join("\n"));(failure,snapshot,events,d.next)}
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
fn emit(out:&mut impl Write,c:Config)->io::Result<()>{let(failure,snapshot,events,reads)=run(c);writeln!(out,"{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{}\t{reads}\t{}\t{}",c.kind,c.length,c.seed,c.read_fail,c.hash_fail,c.eq_fail,c.drop_fail,c.default_fail,hex(&failure),hex(&snapshot),hex(&events))}
fn main()->io::Result<()>{std::panic::set_hook(Box::new(|_|{}));let mut out=io::BufWriter::new(io::stdout().lock());for kind in 0..6{for length in [0,1,2,9,31]{for seed in [1,7]{let base=Config{kind,length,seed,read_fail:-1,hash_fail:-1,eq_fail:-1,drop_fail:-1,default_fail:false};emit(&mut out,base)?;let words=length*(if kind%2==0{2}else{1});for read_fail in 0..=words{emit(&mut out,Config{read_fail:read_fail as isize,..base})?;}for value in [0,1,5,length]{emit(&mut out,Config{hash_fail:value as isize,..base})?;emit(&mut out,Config{eq_fail:value as isize,..base})?;emit(&mut out,Config{drop_fail:value as isize,..base})?;}emit(&mut out,Config{default_fail:true,..base})?;}}}Ok(())}
