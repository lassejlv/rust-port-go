// External observer of the original FileEncoder. The full original crate,
// std filesystem and borrowed FnMut flush strategy perform every operation.
extern crate rustc_serialize;
extern crate rustc_span;
use rustc_serialize::{Encoder,opaque::FileEncoder};
use rustc_span::{SourceFileHash,SourceFileHashAlgorithm};
use std::{sync::{Arc,Mutex},path::Path};
fn digest(b:&[u8])->String {SourceFileHash::new_in_memory(SourceFileHashAlgorithm::Sha256,b).to_string()}
fn with<const N:usize>(e:&mut FileEncoder<'_>,written:usize,fill:u8,log:&Arc<Mutex<Vec<String>>>) {
 e.write_with::<N>(|window| {log.lock().unwrap().push(format!("window:{N}:{}",digest(window)));window.fill(fill);written});
}
fn run(path:&Path,script:&str,fail:i32)->(Vec<String>,Vec<u8>){
 let log:Arc<Mutex<Vec<String>>>=Default::default();let mut calls=0i32;
 let mut strategy=|bytes:&[u8]|{let n=calls;calls+=1;log.lock().unwrap().push(format!("flush:{n}:{}:{}",bytes.len(),digest(bytes)));if n==fail{panic!("strategy")}};
 let mut e=FileEncoder::with_flush_strategy(path,&mut strategy).unwrap();
 for op in script.split(',') {let part=op.split(':').collect::<Vec<_>>();
  let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||match part[0]{
   "raw"=>e.emit_raw_bytes(&vec![part[2].parse::<u8>().unwrap();part[1].parse().unwrap()]),
   "u64"=>e.emit_u64(part[1].parse().unwrap()),
   "u16"=>e.emit_u16(part[1].parse().unwrap()),
   "with"=>{let n:usize=part[1].parse().unwrap();let written=part[2].parse().unwrap();let fill=part[3].parse().unwrap();match n{0=>with::<0>(&mut e,written,fill,&log),1=>with::<1>(&mut e,written,fill,&log),4=>with::<4>(&mut e,written,fill,&log),19=>with::<19>(&mut e,written,fill,&log),64=>with::<64>(&mut e,written,fill,&log),65536=>with::<65536>(&mut e,written,fill,&log),_=>unreachable!()}},
   "flush"=>e.flush(),
   "finish"=>{let r=e.finish();log.lock().unwrap().push(match r{Ok(p)=>format!("finish:ok:{p}"),Err(_)=>"finish:error".into()})},
   _=>unreachable!()
  }));
  log.lock().unwrap().push(format!("step:{}:{}",result.is_err(),e.position()));
 }
 let dropped=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||drop(e))).is_err();
 log.lock().unwrap().push(format!("drop:{dropped}"));
 let bytes=if path==Path::new("/dev/full"){vec![]}else{std::fs::read(path).unwrap_or_default()};let events=log.lock().unwrap().clone();(events,bytes)
}
fn main(){std::panic::set_hook(Box::new(|_|{}));let directory=std::env::args().nth(1).unwrap();let path=Path::new(&directory).join("file");
 let scripts=["finish","flush,finish,finish","raw:65535:1,u16:65535,finish","raw:65536:2,raw:1:3,finish","raw:65537:4,finish","raw:131073:5,raw:65536:6,finish","raw:65530:7,with:19:11:8,finish","with:64:3:9,with:19:7:10,flush,with:64:0:11,finish","with:4:5:12,finish","raw:65535:13,with:4:5:14,finish","raw:65536:15,with:0:0:16,finish","with:65536:65536:17,flush,finish","with:65536:65537:18,finish","u64:18446744073709551615,u16:65535,finish","finish,raw:1:19","raw:1:20"];
 for script in scripts {for fail in [-1,0,1,2,3] {let (events,bytes)=run(&path,script,fail);println!("file\t{script}\t{fail}\t{}\t{}\t{}",events.join(","),bytes.len(),digest(&bytes));}}
 #[cfg(target_os="linux")]for script in ["raw:65537:21,finish,finish","raw:65536:22,flush,raw:65537:23,finish","raw:1:24,finish,finish"] {let(events,_)=run(Path::new("/dev/full"),script,-1);println!("full\t{script}\t-1\t{}\t0\t-",events.join(","));}
}
