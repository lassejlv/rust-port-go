// Development-only oracle for the complete pinned Cargo graph.rs source.
// MIT OR Apache-2.0. It is never linked into the production Go implementation.
use std::any::Any;
use std::io::{self, Read, Write};
use std::panic::{AssertUnwindSafe, catch_unwind};

#[allow(dead_code)]
mod upstream {
    include!(env!("CARGO_GRAPH_SOURCE"));

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
fn main() {
    std::panic::set_hook(Box::new(|_| {}));
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
