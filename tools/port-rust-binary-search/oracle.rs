// Development oracle for the original pinned Rust binary_search_util module.
// SPDX-License-Identifier: MIT OR Apache-2.0
use std::{cell::RefCell, cmp::Ordering, io::{self,Write}, panic::{catch_unwind,AssertUnwindSafe}, rc::Rc};
mod upstream {include!(env!("BINARY_SEARCH_SOURCE"));}

#[derive(Clone)]struct Key {value:i64,context:Rc<RefCell<Context>>}
#[derive(Default)]struct Context {
 mode:u8,key_panic:u32,compare_panic:u32,operator_panic:u8,operator_panic_at:u32,
 keys:Vec<u32>,comparisons:Vec<(i64,i64)>,operators:Vec<u8>,operator_counts:[u32;7],
}
impl Key {
 fn operation(&self,other:&Self,operator:u8)->Ordering {
  let mut context=self.context.borrow_mut();
  context.comparisons.push((self.value,other.value));
  context.operators.push(operator);context.operator_counts[operator as usize]+=1;
  let count=context.comparisons.len() as u32;
  if context.compare_panic!=0&&context.compare_panic==count{panic!("compare panic");}
  if context.operator_panic==operator&&context.operator_panic_at!=0&&context.operator_panic_at==context.operator_counts[operator as usize] {
   match operator{1=>panic!("lt panic"),2=>panic!("ne panic"),3=>panic!("le panic"),_=>panic!("unexpected operator panic")}
  }
  raw_compare(context.mode,self.value,other.value,count)
 }
}
impl PartialEq for Key {
 fn eq(&self,other:&Self)->bool{self.operation(other,4)==Ordering::Equal}
 fn ne(&self,other:&Self)->bool{self.operation(other,2)!=Ordering::Equal}
}
impl Eq for Key {}
impl PartialOrd for Key {
 fn partial_cmp(&self,other:&Self)->Option<Ordering>{Some(self.operation(other,5))}
 fn lt(&self,other:&Self)->bool{self.operation(other,1)==Ordering::Less}
 fn le(&self,other:&Self)->bool{self.operation(other,3)!=Ordering::Greater}
}
impl Ord for Key {
 fn cmp(&self,_other:&Self)->Ordering{panic!("cmp must not be called");}
}
#[derive(Clone)]struct Element {value:i64,index:u32}
fn raw_compare(mode:u8,a:i64,b:i64,count:u32)->Ordering{
 match mode{
  0=>a.cmp(&b),1=>b.cmp(&a),
  2=>a.unsigned_abs().cmp(&b.unsigned_abs()).then_with(||a.cmp(&b)),
  3=>a.rem_euclid(5).cmp(&b.rem_euclid(5)).then_with(||a.cmp(&b)),
  4=>Ordering::Equal,
  5=>{let x=a.rem_euclid(3);let y=b.rem_euclid(3);if x==y{Ordering::Equal}else if (x+1)%3==y{Ordering::Less}else{Ordering::Greater}},
  6=>{let order=a.cmp(&b);if count%2==0{order.reverse()}else{order}},
  _=>panic!("unknown mode")
 }
}
struct Case {mode:u8,target:i64,values:Vec<i64>,key_panic:u32,compare_panic:u32,operator_panic:u8,operator_panic_at:u32}
fn case(mode:u8,target:i64,values:Vec<i64>)->Case{Case{mode,target,values,key_panic:0,compare_panic:0,operator_panic:0,operator_panic_at:0}}
fn u32le(out:&mut impl Write,value:u32){out.write_all(&value.to_le_bytes()).unwrap();}
fn i64le(out:&mut impl Write,value:i64){out.write_all(&value.to_le_bytes()).unwrap();}
fn string(out:&mut impl Write,value:&str){u32le(out,value.len() as u32);out.write_all(value.as_bytes()).unwrap();}
fn execute(out:&mut impl Write,test:Case){
 out.write_all(&[test.mode]).unwrap();i64le(out,test.target);u32le(out,test.values.len() as u32);
 for value in &test.values{i64le(out,*value)}
 u32le(out,test.key_panic);u32le(out,test.compare_panic);
 out.write_all(&[test.operator_panic]).unwrap();u32le(out,test.operator_panic_at);
 let data=test.values.iter().enumerate().map(|(i,value)|Element{value:*value,index:i as u32}).collect::<Vec<_>>();
 let context=Rc::new(RefCell::new(Context{mode:test.mode,key_panic:test.key_panic,compare_panic:test.compare_panic,operator_panic:test.operator_panic,operator_panic_at:test.operator_panic_at,..Default::default()}));
 let key=Key{value:test.target,context:context.clone()};
 let result=catch_unwind(AssertUnwindSafe(||{
  upstream::binary_search_slice(&data,|element|{
   let mut ctx=context.borrow_mut();ctx.keys.push(element.index);
   if ctx.key_panic!=0&&ctx.key_panic==ctx.keys.len() as u32{panic!("key panic");}
   Key{value:element.value,context:context.clone()}
  },&key)
 }));
 match result{
  Ok(slice)=>{out.write_all(&[0]).unwrap();u32le(out,slice.first().map(|e|e.index).unwrap_or(0));u32le(out,slice.len() as u32);},
  Err(error)=>{out.write_all(&[1]).unwrap();let message=error.downcast_ref::<&str>().copied().unwrap_or("unknown panic");string(out,message);}
 }
 let context=context.borrow();u32le(out,context.keys.len() as u32);for index in &context.keys{u32le(out,*index);}
 u32le(out,context.comparisons.len() as u32);for (a,b) in &context.comparisons{i64le(out,*a);i64le(out,*b);}
 assert!(context.operators.iter().all(|operator|[1,2,3].contains(operator)),"source unexpectedly called eq/partial_cmp");
 u32le(out,context.operators.len() as u32);out.write_all(&context.operators).unwrap();
}
fn sorted_lists(values:&[i64],length:usize,minimum:usize,current:&mut Vec<i64>,output:&mut Vec<Vec<i64>>){
 if current.len()==length{output.push(current.clone());return;}
 for i in minimum..values.len(){current.push(values[i]);sorted_lists(values,length,i,current,output);current.pop();}
}
fn all_lists(values:&[i64],length:usize,current:&mut Vec<i64>,output:&mut Vec<Vec<i64>>){
 if current.len()==length{output.push(current.clone());return;}
 for value in values{current.push(*value);all_lists(values,length,current,output);current.pop();}
}
fn next(seed:&mut u64)->u64{*seed^=*seed<<13;*seed^=*seed>>7;*seed^=*seed<<17;*seed}
fn main(){
 std::panic::set_hook(Box::new(|_|{}));
 let mut cases=Vec::new();let mut sorted=Vec::new();
 for length in 0..=7{sorted_lists(&[-2,-1,0,1,2],length,0,&mut Vec::new(),&mut sorted);}
 for mode in 0..4{for values in &sorted{let mut values=values.clone();values.sort_by(|a,b|raw_compare(mode,*a,*b,0));for target in -3..=3{cases.push(case(mode,target,values.clone()));}}}
 let mut unsorted=Vec::new();for length in 0..=5{all_lists(&[-1,0,1],length,&mut Vec::new(),&mut unsorted);}
 for mode in [0,5,6]{for values in &unsorted{for target in -2..=2{cases.push(case(mode,target,values.clone()));}}}
 for length in 0..=18{for target in [-2,0,2,10]{for mode in 0..4{
  let mut values=(0..length).map(|n|n as i64/3-2).collect::<Vec<_>>();values.sort_by(|a,b|raw_compare(mode,*a,*b,0));
  for at in 1..=8{let mut key=case(mode,target,values.clone());key.key_panic=at;cases.push(key);let mut comparator=case(mode,target,values.clone());comparator.compare_panic=at;cases.push(comparator);}
  for operator in 1..=3{for at in 1..=8{let mut test=case(mode,target,values.clone());test.operator_panic=operator;test.operator_panic_at=at;cases.push(test);}}
 }}}
 let mut seed=0xe957_43a1_c6bd_f028;
 for _ in 0..2000{let length=(next(&mut seed)%129) as usize;let mode=(next(&mut seed)%4) as u8;let mut values=(0..length).map(|_|(next(&mut seed)%401) as i64-200).collect::<Vec<_>>();values.sort_by(|a,b|raw_compare(mode,*a,*b,0));let target=(next(&mut seed)%601) as i64-300;cases.push(case(mode,target,values));}
 for mode in 0..7{for target in [i64::MIN,-1,0,1,i64::MAX]{let mut values=vec![i64::MIN,i64::MIN,-1,0,0,1,i64::MAX,i64::MAX];if mode<5{values.sort_by(|a,b|raw_compare(mode,*a,*b,0));}cases.push(case(mode,target,values));}}
 for operator in 0..=3{let mut test=case(0,2,vec![1,2,2,3]);if operator!=0{test.operator_panic=operator;test.operator_panic_at=1;}cases.push(test);}
 let mut out=io::BufWriter::new(io::stdout().lock());out.write_all(b"RBSU2\0").unwrap();u32le(&mut out,cases.len() as u32);
 for test in cases{execute(&mut out,test);}
}
