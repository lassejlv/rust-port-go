// Reference-only inspection: source fields stay private outside this module.
pub(crate) fn snapshot(interner:&SpanInterner)->Vec<(u32,u32,Option<u32>)> {
 interner.spans.iter().map(|data|(data.lo.0,data.hi.0,data.parent.map(|p|p.local_def_index.as_u32()))).collect()
}
