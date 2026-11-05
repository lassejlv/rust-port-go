// SPDX-License-Identifier: MIT OR Apache-2.0
// Real std Windows host operations used by RealFileLoader, under Wine.
use std::{io::{self,BufRead,Read},path::{Path,PathBuf},ffi::OsString};
use std::os::windows::ffi::OsStringExt;
fn hex(s:impl AsRef<[u8]>)->String{s.as_ref().iter().map(|b|format!("{b:02x}")).collect()}
fn path(s:&str)->PathBuf{let bytes:Vec<u8>=(0..s.len()).step_by(2).map(|i|u8::from_str_radix(&s[i..i+2],16).unwrap()).collect();PathBuf::from(OsString::from_wide(&bytes.chunks_exact(2).map(|b|u16::from_le_bytes([b[0],b[1]])).collect::<Vec<_>>()))}
fn error(e:io::Error)->String{format!("err:{}",hex(format!("{e:?}")))}
fn setup(){
 for(name,data)in [("empty",b"".as_slice()),("text",b"hello\r\nworld".as_slice()),("invalid",b"\xff\xfe\0\x80".as_slice()),("bom",b"\xef\xbb\xbfhi\r\n".as_slice())]{std::fs::write(name,data).unwrap();}
 for n in [1,2,15,16,31,32,33,63,64,65,511,512,513,4096]{std::fs::write(format!("bytes{n}"),(0..n).map(|i|((i*73+19)%256)as u8).collect::<Vec<_>>()).unwrap();}
 std::fs::create_dir_all("directory").unwrap();
 for(name,target,directory)in [("link","text",false),("broken","missing",false),("dirlink","directory",true)]{let p=Path::new(name);if p.symlink_metadata().is_err(){let result=if directory{std::os::windows::fs::symlink_dir(target,p)}else{std::os::windows::fs::symlink_file(target,p)};if let Err(e)=result{eprintln!("symlink setup {name}: {e}");}}}
 let long=std::iter::repeat_n("abcdefghijklmnopqrstuvwx",28).collect::<Vec<_>>().join("\\");std::fs::create_dir_all(&long).unwrap();std::fs::write(Path::new(&long).join("deep.txt"),b"deep contents").unwrap();
 let wide=[b'w' as u16,0xd800,b'x' as u16];if let Err(e)=std::fs::write(PathBuf::from(OsString::from_wide(&wide)),b"surrogate contents"){eprintln!("surrogate fixture creation unsupported by selected Wine host: {e:?}");}
}
fn run(p:&[&str])->String{
 let name=path(p[2]);match p[1]{
 "exists"=>name.exists().to_string(),
 "open_meta"=>match std::fs::File::open(&name){Err(e)=>error(e),Ok(f)=>match f.metadata(){Err(e)=>error(e),Ok(m)=>format!("ok:{}",m.len())}},
 "open_read"=>match std::fs::File::open(&name){Err(e)=>error(e),Ok(mut f)=>{let mut b=vec![];match f.read_to_end(&mut b){Err(e)=>error(e),Ok(_)=>format!("ok:{}",hex(b))}}},
 "text"=>match std::fs::read_to_string(&name){Err(e)=>error(e),Ok(s)=>format!("ok:{}",hex(s))},
 "cwd"=>{let old=std::env::current_dir().unwrap();let expected=old.join(&name);match std::env::set_current_dir(&name){Err(e)=>error(e),Ok(())=>{let actual=std::env::current_dir().unwrap();let out=format!("{}:{}:{}",actual==expected,actual.has_root(),actual.is_absolute());std::env::set_current_dir(old).unwrap();out}}},
 _=>panic!("unknown file operation")}}
fn main(){if std::env::args().nth(1).as_deref()==Some("--setup"){setup();return}for line in io::stdin().lock().lines(){let line=line.unwrap();let p:Vec<_>=line.split('\t').collect();println!("{}\t{}",p[0],hex(run(&p)));}}
