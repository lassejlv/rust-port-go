// Development-only oracle for the complete pinned Cargo graph.rs source.
// MIT OR Apache-2.0. It is never linked into the production Go implementation.
use std::any::Any;
use std::io::{self, Read, Write};
use std::panic::{AssertUnwindSafe, catch_unwind};

#[allow(dead_code)]
mod upstream {
    include!(env!("CARGO_GRAPH_SOURCE"));

    pub fn print_generic<N: Ord + Eq + Clone>(graph: &Graph<N, i32>) {
        #[cfg(debug_assertions)]
        graph.print_for_test();
        #[cfg(not(debug_assertions))]
        let _ = graph;
    }

    pub fn print(graph: &Graph<i32, i32>) {
        #[cfg(debug_assertions)]
        graph.print_for_test();
        #[cfg(not(debug_assertions))]
        let _ = graph;
    }
}
use upstream::Graph;

fn number<'a>(words: &mut impl Iterator<Item = &'a str>) -> i32 {
    words.next().expect("missing number").parse().unwrap()
}
fn list(values: impl IntoIterator<Item = i32>) -> String {
    values.into_iter().map(|value| value.to_string()).collect::<Vec<_>>().join(",")
}
fn edges(graph: &Graph<i32, i32>) -> String {
    graph.iter().flat_map(|from| graph.edges(from).map(move |(to, edge)| format!("{from}>{to}:{edge}"))).collect::<Vec<_>>().join(",")
}
fn panic_name(error: &(dyn Any + Send)) -> &'static str {
    let message = error.downcast_ref::<String>().map(String::as_str).or_else(|| error.downcast_ref::<&str>().copied()).unwrap_or("");
    if message.contains("The last element in the path") { "path_end" }
    else if message.contains("ptr::eq") { "path_edge" }
    else if message.contains("key not found") || message.contains("no entry found") || message == "OrdMap::index: invalid key" { "missing_node" }
    else { "other" }
}
fn path(graph: &Graph<i32, i32>, node: i32, top: bool) -> String {
    match catch_unwind(AssertUnwindSafe(|| if top { graph.path_to_top(&node) } else { graph.path_to_bottom(&node) })) {
        Ok(result) => format!("ok {}", result.into_iter().map(|(node, edge)| format!("{node}:{}", edge.map(i32::to_string).unwrap_or("none".into()))).collect::<Vec<_>>().join(",")),
        Err(error) => format!("panic {}", panic_name(error.as_ref())),
    }
}
fn key_list<N>(nodes: impl IntoIterator<Item = N>, key: impl Fn(N) -> String) -> String {
    nodes.into_iter().map(key).collect::<Vec<_>>().join("|")
}
fn key_edges<N: Ord + Eq + Clone>(graph: &Graph<N, i32>, key: fn(&N) -> String) -> String {
    graph.iter().flat_map(|from| graph.edges(from).map(move |(to, edge)| format!("{}>{}:{edge}",key(from),key(to)))).collect::<Vec<_>>().join("|")
}
fn key_path<N: Ord + Eq + Clone>(graph: &Graph<N, i32>, start: &N, top: bool, key: fn(&N) -> String) -> String {
    match catch_unwind(AssertUnwindSafe(|| if top { graph.path_to_top(start) } else { graph.path_to_bottom(start) })) {
        Ok(path) => format!("ok {}",path.into_iter().map(|(node,edge)| format!("{}:{}",key(node),edge.map(i32::to_string).unwrap_or("none".into()))).collect::<Vec<_>>().join("|")),
        Err(error) => format!("panic {}",panic_name(error.as_ref())),
    }
}
fn key_sort<N: Ord + Eq + Clone>(graph: &Graph<N,i32>, key: fn(&N)->String) -> String {
    match catch_unwind(AssertUnwindSafe(||graph.sort())) { Ok(nodes)=>format!("ok {}",key_list(nodes.iter(),key)),Err(error)=>format!("panic {}",panic_name(error.as_ref())) }
}
fn key_case<N: Ord + Eq + Clone>(keys: &[N], key: fn(&N)->String) -> Graph<N,i32> {
    let mut graph: Graph<N,i32> = Graph::new();
    graph.add(keys[4].clone()); graph.add(keys[0].clone()); graph.add(keys[1].clone());
    for (from,to,edge) in [(0,1,10),(1,2,20),(1,3,21),(3,4,30)] { *graph.link(keys[from].clone(),keys[to].clone())=edge; }
    let before=graph.edge(&keys[1],&keys[2]).unwrap() as *const i32;
    let repeated=graph.link(keys[1].clone(),keys[2].clone()) as *const i32;
    println!("len {}\niter {}\nedges {}",graph.len(),key_list(graph.iter(),key),key_edges(&graph,key));
    println!("repeat_identity {}\nchild_only {}",std::ptr::eq(before,repeated),graph.contains(&keys[2]));
    println!("sort_initial {}",key_sort(&graph,key));
    let reversed=graph.reversed();
    println!("reversed_iter {}\nreversed_edges {}",key_list(reversed.iter(),key),key_edges(&reversed,key));
    graph.add(keys[2].clone()); graph.add(keys[1].clone());
    println!("sort_complete {}",key_sort(&graph,key));
    println!("bottom {}\ntop {}",key_path(&graph,&keys[0],false,key),key_path(&graph,&keys[4],true,key));
    println!("reachable {} {}",graph.is_path_from_to(&keys[0],&keys[4]),graph.is_path_from_to(&keys[4],&keys[0]));
    let mut copy=graph.clone();
    println!("clone_equal {}",graph==copy);
    *copy.link(keys[1].clone(),keys[2].clone())=99;
    println!("changed_equal {}\noriginal_edge {}\ncloned_edge {}",graph==copy,graph.edge(&keys[1],&keys[2]).unwrap(),copy.edge(&keys[1],&keys[2]).unwrap());
    upstream::print_generic(&graph);
    graph
}
fn vector_key(key: &Vec<i32>) -> String { format!("[{}]",list(key.iter().copied())) }
#[derive(Clone,PartialEq,Eq,PartialOrd,Ord)]
struct RecordKey { parts: Vec<i32>, tag: i32 }
fn record_key(key: &RecordKey) -> String { format!("{}#{}",vector_key(&key.parts),key.tag) }
impl std::fmt::Display for RecordKey {
    fn fmt(&self,formatter:&mut std::fmt::Formatter<'_>)->std::fmt::Result { formatter.write_str(&record_key(self)) }
}
fn generic_cases() {
    let vectors=vec![vec![],vec![1],vec![1,0],vec![2,3],vec![4]];
    println!("vec_keys");
    let graph=key_case(&vectors,vector_key);
    println!("borrowed {} {}",graph.contains(&[1][..]),graph.contains(&[9][..]));
    let records=vectors.into_iter().enumerate().map(|(tag,parts)|RecordKey{parts,tag:tag as i32}).collect::<Vec<_>>();
    println!("record_keys");
    let mut graph=key_case(&records,record_key);
    println!("record_debug\n{graph:?}");
    graph.add(RecordKey{parts:vec![],tag:9});
    println!("record_field_identity {}",graph.len());
    println!("operators"); operator_probe();
}

#[derive(Clone)]
struct OperatorKey(i32);
thread_local! {
    static EVENTS: std::cell::RefCell<Vec<String>> = const { std::cell::RefCell::new(Vec::new()) };
    static PANIC_OPERATOR: std::cell::Cell<u8> = const { std::cell::Cell::new(0) };
}
fn operator_event(kind: &str, id: u8, left: i32, right: i32) {
    EVENTS.with(|log| log.borrow_mut().push(format!("{kind}:{left}:{right}")));
    if PANIC_OPERATOR.with(|mode| mode.get() == id) { panic!("operator_{kind}"); }
}
impl PartialEq for OperatorKey {
    fn eq(&self, other: &Self) -> bool { operator_event("eq", 1, self.0, other.0); self.0 == other.0 }
    fn ne(&self, other: &Self) -> bool { operator_event("ne", 2, self.0, other.0); self.0 != other.0 }
}
impl Eq for OperatorKey {}
impl PartialOrd for OperatorKey { fn partial_cmp(&self, other: &Self) -> Option<std::cmp::Ordering> { Some(self.cmp(other)) } }
impl Ord for OperatorKey { fn cmp(&self, other: &Self) -> std::cmp::Ordering { self.0.cmp(&other.0) } }
#[derive(Clone,Default)]
struct OperatorEdge(i32);
impl PartialEq for OperatorEdge {
    fn eq(&self, other: &Self) -> bool { operator_event("edge_eq", 3, self.0, other.0); self.0 == other.0 }
    fn ne(&self, other: &Self) -> bool { operator_event("edge_ne", 4, self.0, other.0); self.0 != other.0 }
}
impl Eq for OperatorEdge {}
fn take_events() -> String { EVENTS.with(|events| std::mem::take(&mut *events.borrow_mut()).join(",")) }
fn operator_probe() {
    let mut graph: Graph<OperatorKey, i32> = Graph::new();
    *graph.link(OperatorKey(0), OperatorKey(1)) = 7;
    *graph.link(OperatorKey(0), OperatorKey(2)) = 8;
    graph.link(OperatorKey(1), OperatorKey(1));
    graph.link(OperatorKey(2), OperatorKey(2));
    take_events();
    println!("reach {}", graph.is_path_from_to(&OperatorKey(0), &OperatorKey(2)));
    println!("reach_trace {}", take_events());
    let result = graph.path_to_bottom(&OperatorKey(0)).iter().map(|(node,edge)| format!("{}:{}",node.0,edge.map(i32::to_string).unwrap_or("none".into()))).collect::<Vec<_>>().join(",");
    println!("bottom {result}"); println!("bottom_trace {}",take_events());
    let mut other: Graph<OperatorKey, i32> = Graph::new();
    *other.link(OperatorKey(0), OperatorKey(1)) = 7;
    *other.link(OperatorKey(0), OperatorKey(2)) = 8;
    other.link(OperatorKey(1), OperatorKey(1));
    other.link(OperatorKey(2), OperatorKey(2));
    take_events();
    println!("equal {}",graph==other); println!("equal_trace {}",take_events());
    let mut first: Graph<OperatorKey,OperatorEdge>=Graph::new();
    let mut second: Graph<OperatorKey,OperatorEdge>=Graph::new();
    *first.link(OperatorKey(0),OperatorKey(1))=OperatorEdge(7);
    *second.link(OperatorKey(0),OperatorKey(1))=OperatorEdge(7);
    take_events();
    println!("edge_equal {}",first==second); println!("edge_equal_trace {}",take_events());
    PANIC_OPERATOR.with(|mode|mode.set(1));
    let failed=catch_unwind(AssertUnwindSafe(||graph.is_path_from_to(&OperatorKey(0),&OperatorKey(2)))).is_err();
    PANIC_OPERATOR.with(|mode|mode.set(0));
    println!("eq_panic {failed} {}",take_events());
    PANIC_OPERATOR.with(|mode|mode.set(2));
    let failed=catch_unwind(AssertUnwindSafe(||{ let _ = graph.path_to_bottom(&OperatorKey(0)); })).is_err();
    PANIC_OPERATOR.with(|mode|mode.set(0));
    println!("ne_panic {failed} {}",take_events());
    PANIC_OPERATOR.with(|mode|mode.set(3));
    let no_eq_call=catch_unwind(AssertUnwindSafe(||first==second)).unwrap();
    PANIC_OPERATOR.with(|mode|mode.set(0));
    println!("edge_eq_bypassed {no_eq_call} {}",take_events());
    PANIC_OPERATOR.with(|mode|mode.set(4));
    let failed=catch_unwind(AssertUnwindSafe(||first==second)).is_err();
    PANIC_OPERATOR.with(|mode|mode.set(0));
    println!("edge_ne_panic {failed} {}",take_events());
}
fn main() {
    std::panic::set_hook(Box::new(|_| {}));
    match std::env::args().nth(1).as_deref() {
        Some("operators") => { operator_probe(); return; }
        Some("generic") => { generic_cases(); return; }
        _ => {}
    }
    let mut input = String::new(); io::stdin().read_to_string(&mut input).unwrap();
    let mut words = input.split_whitespace();
    assert_eq!(words.next(), Some("CARGO_GRAPH_V1"));
    let count = number(&mut words);
    let mut out = io::BufWriter::new(io::stdout().lock());
    for case in 0..count {
        let query_max = number(&mut words);
        let mut graph: Graph<i32, i32> = Graph::default();
        let nodes = number(&mut words);
        for _ in 0..nodes { graph.add(number(&mut words)); }
        let links = number(&mut words);
        for _ in 0..links { let from = number(&mut words); let to = number(&mut words); let edge = number(&mut words); *graph.link(from, to) = edge; }
        writeln!(out, "case {case}\nlen {}\nnodes {}\nedges {}", graph.len(), list(graph.iter().copied()), edges(&graph)).unwrap();
        let sorted = match catch_unwind(AssertUnwindSafe(|| graph.sort())) { Ok(nodes) => format!("ok {}", list(nodes)), Err(error) => format!("panic {}", panic_name(error.as_ref())) };
        writeln!(out, "sort {sorted}\ndebug\n{graph:?}").unwrap();
        upstream::print(&graph);
        for from in -1..=query_max {
            writeln!(out, "contains {from} {}", graph.contains(&from)).unwrap();
            for to in -1..=query_max {
                writeln!(out, "reachable {from}>{to} {}", graph.is_path_from_to(&from, &to)).unwrap();
                writeln!(out, "edge {from}>{to} {}", graph.edge(&from, &to).map(i32::to_string).unwrap_or("none".into())).unwrap();
            }
            writeln!(out, "bottom {from} {}\ntop {from} {}", path(&graph, from, false), path(&graph, from, true)).unwrap();
        }
        let reversed = graph.reversed();
        writeln!(out, "reversed\n{reversed:?}\nreversed_edges {}", edges(&reversed)).unwrap();
        let mut cloned = graph.clone();
        writeln!(out, "clone_equal {}", graph == cloned).unwrap();
        let first = graph.iter().flat_map(|node| graph.edges(node).map(move |(child, _)| (*node, *child))).next();
        if let Some((from, to)) = first { *cloned.link(from, to) += 7; }
        cloned.add(-1000); *cloned.link(-1000, -999) = 999;
        writeln!(out, "changed_equal {}\noriginal_edges {}\nclone_nodes {}\nclone_edges {}", graph == cloned, edges(&graph), list(cloned.iter().copied()), edges(&cloned)).unwrap();
    }
    assert!(words.next().is_none());
}
