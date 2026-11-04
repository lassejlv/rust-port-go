// This development oracle executes the pinned Cargo source. It is never linked
// into the production Go package. MIT OR Apache-2.0, as the upstream source.
use std::io::{self, Write};
mod upstream { include!(env!("CARGO_EDIT_DISTANCE_SOURCE")); }

fn u32le(out:&mut impl Write,value:u32){out.write_all(&value.to_le_bytes()).unwrap();}
fn string(out:&mut impl Write,value:&str){u32le(out,value.len() as u32);out.write_all(value.as_bytes()).unwrap();}
fn scalar(value:u32)->Option<char>{char::from_u32(value)}

fn unicode(out:&mut impl Write){
 assert_eq!(std::char::UNICODE_VERSION,(17,0,0));
 let mut flags=vec![0u8;0x110000];
 let mut mappings=Vec::new();
 for cp in 0..0x110000 {
  let Some(c)=scalar(cp) else {continue};
  let lower=c.to_lowercase().collect::<Vec<_>>();
  if lower!=[c]{mappings.push((cp,lower));}
  // Probe the exact predicates that Rust's final-sigma context consumes.
  // AΣc has a medial target sigma iff c is cased and not case-ignorable.
  let after=format!("AΣ{c}").to_lowercase().chars().nth(1)==Some('σ');
  // AcΣ has a medial target sigma iff c is uncased and not case-ignorable.
  let before=format!("A{c}Σ").to_lowercase().chars().last()==Some('σ');
  assert!(!(after&&before));
  if after{flags[cp as usize]|=1;}
  if !after&&!before{flags[cp as usize]|=2;}
 }
 out.write_all(b"CEDU1\0").unwrap();
 out.write_all(&[17,0,0]).unwrap();
 out.write_all(&flags).unwrap();
 u32le(out,mappings.len() as u32);
 for (cp,lower) in mappings{
  u32le(out,cp);out.write_all(&[lower.len() as u8]).unwrap();
  for c in lower{u32le(out,c as u32);}
 }
}

fn strings(alphabet:&[&str],length:usize)->Vec<String>{
 let mut result=vec![String::new()];let mut layer=vec![String::new()];
 for _ in 0..length{
  let mut next=Vec::new();
  for prefix in &layer{for c in alphabet{next.push(format!("{prefix}{c}"));}}
  result.extend(next.iter().cloned());layer=next;
 }
 result
}
fn next(seed:&mut u64)->u64{*seed^=*seed<<13;*seed^=*seed>>7;*seed^=*seed<<17;*seed}
fn random_string(seed:&mut u64)->String{
 let alphabet=['a','b','C','Σ','σ','ς','İ','i','\u{307}','\u{301}','\u{345}','\u{2b0}','\u{1c89}','\u{a7cb}','\0','🦀','é','\n','\u{200d}','\u{10400}','\u{10d50}'];
 let count=(next(seed)%13) as usize;
 (0..count).map(|_|alphabet[(next(seed)%alphabet.len() as u64) as usize]).collect()
}
fn cases(out:&mut impl Write){
 let mut distances=Vec::new();
 for values in [strings(&["a","b","C"],3),strings(&["Σ","İ","a"],2)]{
  for a in &values{for b in &values{for limit in 0..=4{distances.push((a.clone(),b.clone(),limit));}}}
 }
 let mut seed=0xa73f_492b_16c8_5de1;
 for _ in 0..10000{
  let a=random_string(&mut seed);let b=random_string(&mut seed);let limit=(next(&mut seed)%14) as usize;
  distances.push((a,b,limit));
 }
 for (a,b) in [("CA","ABC"),("ab","ba"),("İ","i\u{307}"),("ΟΣ","ος"),("ΟΣΑ","οσα"),("A\u{2b0}Σ","a\u{2b0}ς"),("Σ","σ"),("\u{1c89}","\u{1c8a}")]{distances.push((a.into(),b.into(),usize::MAX));}
 out.write_all(b"CEDC1\0").unwrap();
 u32le(out,distances.len() as u32);
 for (a,b,limit) in distances{
  string(out,&a);string(out,&b);out.write_all(&(limit as u64).to_le_bytes()).unwrap();
  match upstream::edit_distance(&a,&b,limit){Some(distance)=>{out.write_all(&[1]).unwrap();u32le(out,distance as u32)},None=>out.write_all(&[0]).unwrap()}
 }
 let mut suggestions=vec![
  ("x".into(),vec!["a".into(),"b".into()],"command".into()),
  ("CHECK".into(),vec!["build".into(),"check".into(),"CHECK".into()],"subcommand".into()),
  ("absent".into(),vec![],"target".into()),
  ("unrelated".into(),vec!["a".into(),"b".into()],"package".into()),
  ("".into(),vec!["a".into(),"".into()],"empty".into()),
 ];
 for _ in 0..1000{
  let choice=random_string(&mut seed);let kind=random_string(&mut seed);
  let count=(next(&mut seed)%9) as usize;
  let candidates=(0..count).map(|_|random_string(&mut seed)).collect::<Vec<_>>();
  suggestions.push((choice,candidates,kind));
 }
 u32le(out,suggestions.len() as u32);
 for (choice,candidates,kind) in suggestions{
  string(out,&choice);string(out,&kind);u32le(out,candidates.len() as u32);
  for candidate in &candidates{string(out,candidate);}
  let indexed=candidates.iter().enumerate().collect::<Vec<_>>();
  let closest=upstream::closest(&choice,indexed.iter().copied(),|pair|pair.1.as_str());
  let index=closest.map(|pair|pair.0 as i32).unwrap_or(-1);
  out.write_all(&index.to_le_bytes()).unwrap();
  string(out,&upstream::closest_msg(&choice,indexed.iter().copied(),|pair|pair.1.as_str(),&kind));
 }
}
fn main(){
 let mut out=io::BufWriter::new(io::stdout().lock());
 match std::env::args().nth(1).as_deref(){Some("unicode")=>unicode(&mut out),Some("cases")=>cases(&mut out),_=>panic!("expected unicode or cases")}
}
