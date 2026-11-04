// This client imports the complete unchanged interval source and the actual
// original typed-storage source library. main.go verifies every Git object.
#![feature(step_trait)]

mod idx { pub use original_index::Idx; }
mod vec { pub use original_index::IndexVec; }
mod interval;

use interval::{IntervalSet, SparseIntervalMatrix};
use std::cell::RefCell;
use std::cmp::Ordering;
use std::collections::HashMap;
use std::hash::{Hash, Hasher};
use std::io::{self, BufRead, Write};
use std::iter::Step;
use std::ops::{Bound, RangeBounds};
use std::panic::{catch_unwind, AssertUnwindSafe};

#[derive(Default)]
struct State { enabled: bool, limit: usize, delta: usize, fail: String, at: usize, counts: HashMap<String,usize>, trace: Vec<String> }
thread_local! { static STATE: RefCell<State> = RefCell::new(State::default()); }
fn event(kind: &str, values: &[usize]) {
    let fail = STATE.with(|state| {
        let mut state=state.borrow_mut(); if !state.enabled {return false;}
        let mut text=kind.to_owned(); for value in values {text.push(':');text.push_str(&value.to_string());}
        state.trace.push(text);
        let count=state.counts.entry(kind.to_owned()).or_default();*count+=1;let count=*count;
        state.fail==kind && state.at==count
    });
    if fail {panic!("injected callback panic");}
}
fn key_event<const K:u8>(kind: &str, values:&[usize]) {event(&format!("{}{}",if K==0{"r"}else{"c"},kind),values)}

#[derive(Copy, Debug)]
struct Key<const K:u8> { value:usize, tag:u8, metadata:&'static [u8] }
impl<const K:u8> Key<K> {fn raw(value:usize,tag:u8)->Self {Self{value,tag,metadata:&[1,2]}}}
impl<const K:u8> Clone for Key<K> {fn clone(&self)->Self{key_event::<K>("c",&[self.value,self.tag as usize]);*self}}
impl<const K:u8> PartialEq for Key<K> {fn eq(&self,other:&Self)->bool{key_event::<K>("eq",&[self.value,other.value,self.tag as usize,other.tag as usize]);self.value==other.value&&self.tag==other.tag&&self.metadata==other.metadata}}
impl<const K:u8> Eq for Key<K> {}
impl<const K:u8> Hash for Key<K> {fn hash<H:Hasher>(&self,state:&mut H){self.value.hash(state);self.tag.hash(state);self.metadata.hash(state)}}
fn compare<const K:u8>(a:&Key<K>,b:&Key<K>)->Ordering {a.value.cmp(&b.value).then(a.tag.cmp(&b.tag))}
impl<const K:u8> PartialOrd for Key<K> {
    fn partial_cmp(&self,other:&Self)->Option<Ordering>{key_event::<K>("cmp",&[self.value,other.value,self.tag as usize,other.tag as usize]);Some(compare(self,other))}
    fn lt(&self,other:&Self)->bool{key_event::<K>("lt",&[self.value,other.value,self.tag as usize,other.tag as usize]);compare(self,other)==Ordering::Less}
    fn le(&self,other:&Self)->bool{key_event::<K>("le",&[self.value,other.value,self.tag as usize,other.tag as usize]);compare(self,other)!=Ordering::Greater}
    fn gt(&self,other:&Self)->bool{key_event::<K>("gt",&[self.value,other.value,self.tag as usize,other.tag as usize]);compare(self,other)==Ordering::Greater}
    fn ge(&self,other:&Self)->bool{key_event::<K>("ge",&[self.value,other.value,self.tag as usize,other.tag as usize]);compare(self,other)!=Ordering::Less}
}
impl<const K:u8> idx::Idx for Key<K> {
    fn new(value:usize)->Self{key_event::<K>("n",&[value]);assert!(value<=STATE.with(|s|s.borrow().limit));Self::raw(value,0)}
    fn index(self)->usize{key_event::<K>("i",&[self.value,self.tag as usize]);self.value}
}
impl<const K:u8> Step for Key<K> {
    fn steps_between(a:&Self,b:&Self)->(usize,Option<usize>){key_event::<K>("sb",&[a.value,b.value]);if a.value>b.value{(0,None)}else{(b.value-a.value,Some(b.value-a.value))}}
    fn forward_checked(mut a:Self,n:usize)->Option<Self>{key_event::<K>("fc",&[a.value,n]);let limit=STATE.with(|s|s.borrow().limit);if n>limit||a.value>limit-n{None}else{a.value+=n;Some(a)}}
    fn backward_checked(mut a:Self,n:usize)->Option<Self>{key_event::<K>("bc",&[a.value,n]);if n>a.value{None}else{a.value-=n;Some(a)}}
    fn forward_overflowing(mut a:Self,n:usize)->(Self,bool){key_event::<K>("fo",&[a.value,n]);let limit=STATE.with(|s|s.borrow().limit);if n>limit||a.value>limit-n{a.value=0;(a,true)}else{a.value+=n;(a,false)}}
    fn backward_overflowing(mut a:Self,n:usize)->(Self,bool){key_event::<K>("bo",&[a.value,n]);if n>a.value{a.value=0;(a,true)}else{a.value-=n;(a,false)}}
}
type Column=Key<1>;
type Row=Key<0>;

enum Bounds {
    Custom(Bound<Column>,Bound<Column>),
    HalfOpen(std::ops::Range<Column>), Closed(std::ops::RangeInclusive<Column>),
    From(std::ops::RangeFrom<Column>), To(std::ops::RangeTo<Column>),
    ToInclusive(std::ops::RangeToInclusive<Column>), Full(std::ops::RangeFull),
}
impl Clone for Bounds {
    fn clone(&self)->Self {match self {
        Self::Custom(a,b)=>{event("crclone",&[]);let mut a=*a;match &mut a {Bound::Included(key)|Bound::Excluded(key)=>key.value=key.value.wrapping_add(STATE.with(|s|s.borrow().delta)),_=>{}};Self::Custom(a,*b)},
        Self::HalfOpen(r)=>Self::HalfOpen(r.clone()),Self::Closed(r)=>Self::Closed(r.clone()),
        Self::From(r)=>Self::From(r.clone()),Self::To(r)=>Self::To(r.clone()),
        Self::ToInclusive(r)=>Self::ToInclusive(r.clone()),Self::Full(r)=>Self::Full(*r),
    }}
}
impl RangeBounds<Column> for Bounds {
    fn start_bound(&self)->Bound<&Column>{match self {
        Self::Custom(a,_)=>{event("cstart",&[]);a.as_ref()},
        Self::HalfOpen(r)=>r.start_bound(),Self::Closed(r)=>r.start_bound(),Self::From(r)=>r.start_bound(),
        Self::To(r)=>r.start_bound(),Self::ToInclusive(r)=>r.start_bound(),Self::Full(r)=>r.start_bound(),
    }}
    fn end_bound(&self)->Bound<&Column>{match self {
        Self::Custom(_,b)=>{event("cend",&[]);b.as_ref()},
        Self::HalfOpen(r)=>r.end_bound(),Self::Closed(r)=>r.end_bound(),Self::From(r)=>r.end_bound(),
        Self::To(r)=>r.end_bound(),Self::ToInclusive(r)=>r.end_bound(),Self::Full(r)=>r.end_bound(),
    }}
}
struct Op { action:String,target:usize,a:usize,b:usize,c:usize,d:usize,start:u8,end:u8,range:String,tag_a:u8,tag_b:u8 }
fn number(text:&str)->usize{text.parse().unwrap()}
impl Op {
    fn parse(text:&str)->Self{let f:Vec<_>=text.split(',').collect();assert_eq!(f.len(),11);Self{action:f[0].into(),target:number(f[1]),a:number(f[2]),b:number(f[3]),c:number(f[4]),d:number(f[5]),start:number(f[6]) as u8,end:number(f[7]) as u8,range:f[8].into(),tag_a:number(f[9]) as u8,tag_b:number(f[10]) as u8}}
    fn bounds(&self,matrix:bool)->Bounds{
        let(a,b,ta,tb)=if matrix{(self.c,self.d,0,0)}else{(self.a,self.b,self.tag_a,self.tag_b)};
        let a=Column::raw(a,ta);let b=Column::raw(b,tb);
        match &*self.range {
            "h"=>Bounds::HalfOpen(a..b),"c"=>Bounds::Closed(a..=b),"f"=>Bounds::From(a..),
            "t"=>Bounds::To(..b),"i"=>Bounds::ToInclusive(..=b),"u"=>Bounds::Full(..),
            _=>Bounds::Custom(bound(self.start,a),bound(self.end,b)),
        }
    }
}
fn bound(kind:u8,value:Column)->Bound<Column>{match kind{0=>Bound::Unbounded,1=>Bound::Included(value),2=>Bound::Excluded(value),_=>panic!()}}
fn operations(text:&str)->Vec<Op>{if text.is_empty(){vec![]}else{text.split(';').map(Op::parse).collect()}}
fn caught(f:impl FnOnce()->String)->String{catch_unwind(AssertUnwindSafe(f)).unwrap_or_else(|_|"panic".into())}
fn key_text<const K:u8>(value:Key<K>)->String{format!("{}:{}",value.value,value.tag)}
fn option_text(value:Option<Column>)->String{value.map(key_text).unwrap_or_else(||"-".into())}
fn hint_text(value:(usize,Option<usize>))->String{match value.1{Some(hi)=>format!("h({},{hi})",value.0),None=>format!("h({},-)",value.0)}}

struct Execution {sets:[IntervalSet<Column>;2],matrix:SparseIntervalMatrix<Row,Column>}
impl Execution {
    fn operation(&mut self,op:&Op)->String {
        let target=op.target;
        match &*op.action {
            "clear"=>{self.sets[target].clear();"unit".into()},
            "empty"=>self.sets[target].is_empty().to_string(),
            "clone"=>{self.sets[1-target]=self.sets[target].clone();"unit".into()},
            "debug"=>format!("{:?}",self.sets[target]),
            "all"=>{self.sets[target].insert_all();"unit".into()},
            "insert"=>self.sets[target].insert(Column::raw(op.a,op.tag_a)).to_string(),
            "append"=>{self.sets[target].append(Column::raw(op.a,op.tag_a));"unit".into()},
            "contains"=>self.sets[target].contains(Column::raw(op.a,op.tag_a)).to_string(),
            "range"=>self.sets[target].insert_range(op.bounds(false)).to_string(),
            "appendrange"=>{self.sets[target].append_range(op.bounds(false));"unit".into()},
            "intersects"=>self.sets[target].intersects_range(op.bounds(false)).to_string(),
            "first"=>option_text(self.sets[target].first_unset_in(op.bounds(false))),
            "last"=>option_text(self.sets[target].last_set_in(op.bounds(false))),
            "union"=>{let(a,b)=self.sets.split_at_mut(1);if target==0{a[0].union(&b[0])}else{b[0].union(&a[0])}.to_string()},
            "superset"=>self.sets[target].superset(&self.sets[1-target]).to_string(),
            "disjoint"=>self.sets[target].disjoint(&self.sets[1-target]).to_string(),
            "intervals"=>{
                let mut iter=self.sets[target].iter_intervals();let mut out=Vec::new();
                for _ in 0..op.a {out.push(caught(||match iter.next(){Some(r)=>format!("{}..{}",key_text(r.start),key_text(r.last)),None=>"-".into()}))}
                format!("[{}]",out.join(","))
            },
            "iter"|"hint"=>{
                let mut iter=self.sets[target].iter();let mut out=Vec::new();
                for _ in 0..op.a {if op.action=="hint"{out.push(caught(||hint_text(iter.size_hint())))};out.push(caught(||option_text(iter.next())))}
                if op.action=="hint"{out.push(caught(||hint_text(iter.size_hint())))}
                format!("[{}]",out.join(","))
            },
            "mrows"=>{let mut iter=self.matrix.rows();let mut out=Vec::new();while let Some(value)=iter.next(){out.push(key_text(value))};format!("[{}]",out.join(","))},
            "menumerated"=>{let mut iter=self.matrix.iter_enumerated();let mut out=Vec::new();while let Some((row,value))=iter.next(){out.push(format!("{}={value:?}",key_text(row)))};format!("[{}]",out.join(","))},
            "mrow"=>self.matrix.row(Row::raw(op.a,op.tag_a)).map(|r|format!("{r:?}")).unwrap_or_else(||"-".into()),
            "mclear"=>{self.matrix.clear_row(Row::raw(op.a,op.tag_a));"unit".into()},
            "munionrow"=>self.matrix.union_row(Row::raw(op.a,op.tag_a),&self.sets[target]).to_string(),
            "munionrows"=>self.matrix.union_rows(Row::raw(op.a,op.tag_a),Row::raw(op.b,op.tag_b)).to_string(),
            "mdisjoint"=>self.matrix.disjoint_rows(Row::raw(op.a,op.tag_a),Row::raw(op.b,op.tag_b)).to_string(),
            "mall"=>{self.matrix.insert_all_into_row(Row::raw(op.a,op.tag_a));"unit".into()},
            "mrange"=>{self.matrix.insert_range(Row::raw(op.a,op.tag_a),op.bounds(true));"unit".into()},
            "minsert"=>self.matrix.insert(Row::raw(op.a,op.tag_a),Column::raw(op.c,0)).to_string(),
            "mappend"=>{self.matrix.append(Row::raw(op.a,op.tag_a),Column::raw(op.c,0));"unit".into()},
            "mappendrange"=>{self.matrix.append_range(Row::raw(op.a,op.tag_a),op.bounds(true));"unit".into()},
            "mcontains"=>self.matrix.contains(Row::raw(op.a,op.tag_a),Column::raw(op.c,0)).to_string(),
            "mclone"=>{self.matrix=self.matrix.clone();"unit".into()},
            _=>panic!("unknown operation"),
        }
    }
}
fn execute(line:&str)->String{
    let f:Vec<_>=line.split('\t').collect();assert_eq!(f.len(),9);
    let domain=number(f[1]);let other=number(f[2]);let limit=number(f[3]);let delta=number(f[4]);let fail=f[5].to_owned();let at=number(f[6]);
    STATE.with(|s|*s.borrow_mut()=State{limit,delta,fail,at,..State::default()});
    let mut execution=Execution{sets:[IntervalSet::new(domain),IntervalSet::new(other)],matrix:SparseIntervalMatrix::new(domain)};
    for op in operations(f[7]){caught(||execution.operation(&op));}
    STATE.with(|s|s.borrow_mut().enabled=true);
    let results:Vec<_>=operations(f[8]).into_iter().map(|op|caught(||execution.operation(&op))).collect();
    STATE.with(|s|s.borrow_mut().enabled=false);
    let mut rows=Vec::new();for i in 0..128 {let Some(row)=execution.matrix.row(Row::raw(i,0))else{break};rows.push(format!("{row:?}"));assert!(i<127);}
    let state=format!("s0={:?}~s1={:?}~m=[{}]",execution.sets[0],execution.sets[1],rows.join(";"));
    let trace=STATE.with(|s|s.borrow().trace.join(";"));
    format!("{}\t{state}\t{trace}",results.join("|"))
}
fn main(){
    std::panic::set_hook(Box::new(|_|{}));let stdout=io::stdout();let mut out=io::BufWriter::new(stdout.lock());
    for line in io::stdin().lock().lines(){writeln!(out,"{}",execute(&line.unwrap())).unwrap()}
}
