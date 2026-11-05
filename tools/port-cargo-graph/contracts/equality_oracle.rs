// SPDX-License-Identifier: MIT OR Apache-2.0
// Appended after the external type/client declarations in owned_oracle.rs,
// before its operation/main functions. The included Cargo Graph is unchanged.
fn equality_graph(case:u32,size:u32)->Graph<Key,Edge>{
 let mut h=Graph::new();
 let mut values=(0..size).collect::<Vec<_>>();
 if case==0||case==1{for n in &mut values{*n+=1}}
 if case==2{values.reverse()}
 if case==6||case==7{if size>0{values.rotate_left(size as usize/2)}}
 for n in values{h.add(key(n as i32));}
 h
}
fn main(){
 std::panic::set_hook(Box::new(|_|{}));
 for line in std::io::stdin().lock().lines(){
  let line=line.unwrap();let p=line.split('\t').collect::<Vec<_>>();assert_eq!(p.len(),10);
  REQUEST.with(|v|v.set(Request{scenario:p[0].parse().unwrap(),size:p[1].parse().unwrap(),op:p[2].parse().unwrap(),mode:p[3].parse().unwrap(),fail1:p[4].parse().unwrap(),fail2:p[5].parse().unwrap(),format:0,write_fail:usize::MAX,chars:false,reuse:false}));
  let cfg=request();SCOPE.with(|v|v.set("setup"));SERIAL.with(|v|v.set(0));REGISTRY.with(|v|v.borrow_mut().clear());TRACE.with(|v|v.borrow_mut().clear());
  let g=build();
  // Scenario5 selects the source Eq defaults; prepare the same star as6.
  let mut g=g;if cfg.scenario==5{for i in 1..cfg.size{*g.link(key(0),key(i as i32))=edge(i as i32+10)}}
  let mut h=if (4..=5).contains(&cfg.op)||cfg.op>=8{g.clone()}else{equality_graph(cfg.op,cfg.size)};
  TRACE.with(|v|v.borrow_mut().clear());EVENTS.with(|v|v.set(0));SCOPE.with(|v|v.set("op"));
  let result=std::panic::catch_unwind(std::panic::AssertUnwindSafe(||{
   if cfg.op==4||cfg.op==5{let n=if cfg.op==4{cfg.size-1}else{cfg.size/2};*h.link(key(n as i32),key(n as i32))=edge(99)}
   if cfg.op>=8{let n=cfg.size as i32-1;*h.link(key(0),key(n))=edge(if cfg.op==8{99}else{n+10})}
   let equal=if cfg.op==1||cfg.op==3||cfg.op==7{h.eq(&g)}else{g.eq(&h)};
   format!("equal:{equal}")
  })).unwrap_or_else(|_|"panic".into());
  let len=g.len();SCOPE.with(|v|v.set("after"));
  let after=if std::panic::catch_unwind(std::panic::AssertUnwindSafe(||drop(g))).is_ok(){"ok"}else{"panic"};
  let stored=if std::panic::catch_unwind(std::panic::AssertUnwindSafe(||drop(h))).is_ok(){"ok"}else{"panic"};
  let alive=REGISTRY.with(|v|v.borrow().iter().filter_map(|p|p.upgrade()).map(|p|p.serial.to_string()).collect::<Vec<_>>().join(","));
  let trace=TRACE.with(|v|v.borrow().join("|"));println!("{}\t{len}\t{after}\t{stored}\t{alive}\t{}",hex(&result),hex(&trace));
 }
}
