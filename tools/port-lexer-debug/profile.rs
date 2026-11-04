// Small inputs exercise complete original cursor helpers in independent cfgs.
// The arithmetic section executes exact native u32 source expressions, not a
// claimed allocation of a >4GiB Rust string.
use crate::{lexer,hex};
use std::panic::{catch_unwind,AssertUnwindSafe};
fn state(c:&lexer::Cursor<'_>)->String{format!("{}:{}:{}:{}:{}:{}:{}",hex(c.as_str().as_bytes()),c.prev() as u32,c.first() as u32,c.second() as u32,c.third() as u32,c.pos_within_token(),c.is_eof())}
const INPUTS:[&str;13]=["","a","a\0🦀\n","r#\"a\"#tail","##\"abc\"#suffix","---\nx: 1\n---\nfn main() {}","\n---\n---"," /* hi */ 1u8 + 'a'","\"unterminated","\0","λ🦀\r\n", "r###\"abc\"#x\"##", " abc!tail"];
pub fn cursor(){
 for (id,input)in INPUTS.iter().enumerate(){for front in [false,true]{
  let mut c=lexer::Cursor::new(input,if front{lexer::FrontmatterAllowed::Yes}else{lexer::FrontmatterAllowed::No});let mut events=vec![state(&c)];
  for action in 0..9 {let result=catch_unwind(AssertUnwindSafe(||match action{0=>format!("{:?}",c.bump()),1=>{c.bump_bytes(0);"unit".into()},2=>{c.eat_until(b'!');"unit".into()},3=>{c.reset_pos_within_token();"unit".into()},4=>format!("{:?}",c.bump()),5=>{c.eat_while(|x|x!='\n');"unit".into()},6=>{c.bump_bytes(1);"unit".into()},7=>format!("{:?}",c.advance_token()),_=>format!("{:?}",c.advance_token())}));events.push(match result{Ok(v)=>v,Err(_)=>"panic".into()});events.push(state(&c));}
  println!("cursor\t{id}\t{front}\t{}",hex(events.join("|").as_bytes()));
 }}
 for (id,input)in INPUTS.iter().enumerate(){for bumps in 0..=2{let mut c=lexer::Cursor::new(input,lexer::FrontmatterAllowed::No);for _ in 0..bumps{c.bump();}let result=catch_unwind(AssertUnwindSafe(||c.guarded_double_quoted_string()));let value=match result{Ok(v)=>format!("{v:?}"),Err(_)=>"panic".into()};println!("guard\t{id}\t{bumps}\t{}",hex(format!("{value}|{}",state(&c)).as_bytes()));}}
 for (id,input)in INPUTS.iter().enumerate(){for prefix in [0,1,2,3,8]{let value=catch_unwind(||lexer::validate_raw_str(input,prefix));println!("raw\t{id}\t{prefix}\t{}",hex(match value{Ok(v)=>format!("{v:?}"),Err(_)=>"panic".into()}.as_bytes()));}}
 for a in [0u32,1,26,u32::MAX-1,u32::MAX]{for b in [0u32,1,26,u32::MAX-1,u32::MAX]{for op in 0..4{let result=catch_unwind(||{let a=std::hint::black_box(a);let b=std::hint::black_box(b);match op{0=>a+b,1=>a-b,2=>a-b-b+1,_=>a+1}});println!("arith\t{a}\t{b}\t{op}\t{}",match result{Ok(v)=>v.to_string(),Err(_)=>"panic".into()});}}}
 println!("cast\t{}",usize::MAX as u32);
 for (id,input)in INPUTS.iter().enumerate(){for byte in 0..=255u8{let mut c=lexer::Cursor::new(input,lexer::FrontmatterAllowed::No);let result=catch_unwind(AssertUnwindSafe(||c.eat_until(byte)));println!("until\t{id}\t{byte}\t{}",hex(format!("{}|{}",if result.is_ok(){"unit"}else{"panic"},state(&c)).as_bytes()));}
 for bytes in 0..=input.len()+1{let mut c=lexer::Cursor::new(input,lexer::FrontmatterAllowed::No);let result=catch_unwind(AssertUnwindSafe(||c.bump_bytes(bytes)));println!("bytes\t{id}\t{bytes}\t{}",hex(format!("{}|{}",if result.is_ok(){"unit"}else{"panic"},state(&c)).as_bytes()));}}
}
pub fn iterator(){
 for (id,input)in INPUTS.iter().enumerate(){for front in [false,true]{for script in 0..6{
  let mut it=lexer::tokenize(input,if front{lexer::FrontmatterAllowed::Yes}else{lexer::FrontmatterAllowed::No});let mut events=vec![format!("{:?}",it.size_hint())];
  match script{
   0=>{for _ in 0..20{events.push(format!("{:?}",it.next()));events.push(format!("{:?}",it.size_hint()));}},
   1=>{events.push(format!("{:?}",it.nth(0)));events.push(format!("{:?}",it.next()));events.push(format!("{:?}",it.nth(1)));events.push(format!("{:?}",it.next()));events.push(format!("{:?}",it.nth(100)));for _ in 0..3{events.push(format!("{:?}",it.next()));}},
   2=>{events.push(format!("{:?}",it.nth(usize::MAX)));events.push(format!("{:?}",it.next()));},
   3=>{events.push(format!("{:?}",it.next()));events.push(format!("{}",it.count()));},
   4=>{events.push(format!("{:?}",it.next()));events.push(format!("{:?}",it.last()));},_=>{}
  }
  println!("iterator\t{id}\t{front}\t{script}\t{}",hex(events.join("|").as_bytes()));
 }}}
}
