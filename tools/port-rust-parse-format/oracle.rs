// SPDX-License-Identifier: MIT OR Apache-2.0
// This development-only wrapper is compiled against unchanged pinned upstream
// source materialized by the Go generator; it never services production parsing.
use rustc_parse_format as pf;
use serde_json::{json, Value};
use std::{io::{self, BufRead, Write}, ops::Range, panic::{catch_unwind, AssertUnwindSafe}};

fn range(r: &Range<usize>) -> Value { json!([r.start, r.end]) }
fn optional_range(r: &Option<Range<usize>>) -> Value {
    r.as_ref().map_or(Value::Null, range)
}
fn position(p: &pf::Position<'_>) -> Value {
    match p {
        pf::Position::ArgumentImplicitlyIs(i) => json!({"kind":"ArgumentImplicitlyIs","index":i,"index_value":p.index()}),
        pf::Position::ArgumentIs(i) => json!({"kind":"ArgumentIs","index":i,"index_value":p.index()}),
        pf::Position::ArgumentNamed(name) => json!({"kind":"ArgumentNamed","name":name,"index_value":p.index()}),
    }
}
fn count(c: &pf::Count<'_>) -> Value {
    match c {
        pf::Count::CountIs(n) => json!({"kind":"CountIs","value":n}),
        pf::Count::CountIsName(name, span) => json!({"kind":"CountIsName","name":name,"span":range(span)}),
        pf::Count::CountIsParam(i) => json!({"kind":"CountIsParam","index":i}),
        pf::Count::CountIsStar(i) => json!({"kind":"CountIsStar","index":i}),
        pf::Count::CountImplied => json!({"kind":"CountImplied"}),
    }
}
fn format_spec(f: &pf::FormatSpec<'_>) -> Value {
    json!({
        "fill":f.fill.map(u32::from),"fill_span":optional_range(&f.fill_span),
        "align":match f.align {pf::Alignment::AlignLeft=>"AlignLeft",pf::Alignment::AlignRight=>"AlignRight",pf::Alignment::AlignCenter=>"AlignCenter",pf::Alignment::AlignUnknown=>"AlignUnknown"},
        "sign":f.sign.map(|s|match s {pf::Sign::Plus=>"Plus",pf::Sign::Minus=>"Minus"}),
        "alternate":f.alternate,"zero_pad":f.zero_pad,
        "debug_hex":f.debug_hex.map(|h|match h {pf::DebugHex::Lower=>"Lower",pf::DebugHex::Upper=>"Upper"}),
        "precision":count(&f.precision),"precision_span":optional_range(&f.precision_span),
        "width":count(&f.width),"width_span":optional_range(&f.width_span),
        "ty":f.ty,"ty_span":optional_range(&f.ty_span)
    })
}
fn piece(p: pf::Piece<'_>) -> Value {
    let debug = format!("{p:?}");
    match p {
        pf::Piece::Lit(text) => json!({"kind":"Lit","text":text,"debug":debug}),
        pf::Piece::NextArgument(a) => json!({"kind":"NextArgument","debug":debug,"argument":{
            "position":position(&a.position),"position_span":range(&a.position_span),
            "format":format_spec(&a.format),"is_identifier":a.is_identifier()
        }}),
    }
}
fn suggestion(s: &pf::Suggestion) -> Value {
    match s {
        pf::Suggestion::None => json!({"kind":"None"}),
        pf::Suggestion::UsePositional => json!({"kind":"UsePositional"}),
        pf::Suggestion::RemoveRawIdent(span) => json!({"kind":"RemoveRawIdent","span":range(span)}),
        pf::Suggestion::ReorderFormatParameter(span, replacement) => json!({"kind":"ReorderFormatParameter","span":range(span),"replacement":replacement}),
        pf::Suggestion::AddMissingColon(span) => json!({"kind":"AddMissingColon","span":range(span)}),
        pf::Suggestion::UseRustDebugPrintingMacro => json!({"kind":"UseRustDebugPrintingMacro"}),
    }
}
fn error(e: &pf::ParseError) -> Value {
    json!({"description":e.description,"note":e.note,"label":e.label,"span":range(&e.span),
        "secondary_label":e.secondary_label.as_ref().map(|(label,span)|json!({"label":label,"span":range(span)})),
        "suggestion":suggestion(&e.suggestion)})
}
fn peek(value: Option<(Range<usize>, usize, char)>) -> Value {
    value.map_or(Value::Null, |(span,index,c)| json!({"span":range(&span),"index":index,"char":c as u32}))
}
fn state(parser: &pf::Parser<'_>) -> Value {
    json!({"errors":parser.errors.iter().map(error).collect::<Vec<_>>(),"curarg":parser.curarg,
        "arg_places":parser.arg_places.iter().map(range).collect::<Vec<_>>(),
        "is_source_literal":parser.is_source_literal,"line_spans":parser.line_spans.iter().map(range).collect::<Vec<_>>(),
        "peek":peek(parser.peek()),"peek_ahead":peek(parser.peek_ahead())})
}

fn uint(value: &Value) -> usize { usize::try_from(value.as_u64().expect("expected unsigned integer")).unwrap() }
fn read_range(value: &Value) -> Range<usize> {
    let parts = value.as_array().expect("expected range array");
    assert_eq!(parts.len(), 2);
    uint(&parts[0])..uint(&parts[1])
}
fn read_ranges(value: &Value) -> Vec<Range<usize>> {
    value.as_array().expect("expected range array list").iter().map(read_range).collect()
}
fn read_suggestion(value: &Value) -> pf::Suggestion {
    match value["kind"].as_str().unwrap() {
        "None" => pf::Suggestion::None,
        "UsePositional" => pf::Suggestion::UsePositional,
        "RemoveRawIdent" => pf::Suggestion::RemoveRawIdent(read_range(&value["span"])),
        "ReorderFormatParameter" => pf::Suggestion::ReorderFormatParameter(read_range(&value["span"]),value["replacement"].as_str().unwrap().to_owned()),
        "AddMissingColon" => pf::Suggestion::AddMissingColon(read_range(&value["span"])),
        "UseRustDebugPrintingMacro" => pf::Suggestion::UseRustDebugPrintingMacro,
        _ => panic!("unknown suggestion kind"),
    }
}
fn read_error(value: &Value) -> pf::ParseError {
    pf::ParseError {
        description:value["description"].as_str().unwrap().to_owned(),
        note:value["note"].as_str().map(str::to_owned),
        label:value["label"].as_str().unwrap().to_owned(),span:read_range(&value["span"]),
        secondary_label:if value["secondary_label"].is_null(){None}else{let s=&value["secondary_label"];Some((s["label"].as_str().unwrap().to_owned(),read_range(&s["span"])))},
        suggestion:read_suggestion(&value["suggestion"]),
    }
}
fn read_errors(value: &Value) -> Vec<pf::ParseError> { value.as_array().unwrap().iter().map(read_error).collect() }
fn seed_error() -> pf::ParseError {
    pf::ParseError {description:"seed-description".into(),note:Some("seed-note".into()),label:"seed-label".into(),
        span:3..5,secondary_label:Some(("seed-secondary".into(),6..7)),suggestion:pf::Suggestion::RemoveRawIdent(8..9)}
}
fn configure(parser: &mut pf::Parser<'_>, initial: &Value) {
    if let Some(v)=initial.get("curarg") { parser.curarg=uint(v); }
    if let Some(v)=initial.get("is_source_literal") { parser.is_source_literal=v.as_bool().unwrap(); }
    if let Some(v)=initial.get("errors") { parser.errors=read_errors(v); }
    if let Some(v)=initial.get("arg_places") { parser.arg_places=read_ranges(v); }
    if let Some(v)=initial.get("line_spans") { parser.line_spans=read_ranges(v); }
}
fn panic_kind(payload: Box<dyn std::any::Any + Send>) -> &'static str {
    let text = payload.downcast_ref::<&str>().copied().or_else(||payload.downcast_ref::<String>().map(String::as_str)).unwrap_or("");
    if text.contains("attempt to add with overflow") { "add_overflow" }
    else if text.contains("attempt to subtract with overflow") { "sub_overflow" }
    else if text.contains("char boundary") { "char_boundary" }
    else if ["out of bounds","range end index","range start index","begin <= end","begin > end","start <= end","slice index starts at","byte range starts at","string slice"].iter().any(|s|text.contains(s)) { "bounds" }
    else { "other" }
}
fn execute(request: &Value) -> Value {
    let mode = match request["mode"].as_str().unwrap() {"Format"=>pf::ParseMode::Format,"InlineAsm"=>pf::ParseMode::InlineAsm,"Diagnostic"=>pf::ParseMode::Diagnostic,_=>panic!("unknown parse mode")};
    let input=request["input"].as_str().unwrap();
    let style=if request["style"].is_null(){None}else{Some(uint(&request["style"]))};
    let snippet=request["snippet"].as_str().map(str::to_owned);
    let constructed=catch_unwind(AssertUnwindSafe(||pf::Parser::new(input,style,snippet,request["appended_newline"].as_bool().unwrap(),mode)));
    let mut parser=match constructed {Ok(p)=>p,Err(e)=>return json!({"constructor_panic":panic_kind(e),"initial":null,"configured":null,"steps":[]})};
    let initial=state(&parser);
    if let Some(patch)=request.get("initial") { if !patch.is_null() {configure(&mut parser,patch);} }
    let configured=state(&parser);
    let mut steps=Vec::new();
    for action in request["actions"].as_array().unwrap() {
        let op=action["op"].as_str().unwrap();
        let mut returned_piece=Value::Null;
        let mut value=Value::Null;
        let outcome=catch_unwind(AssertUnwindSafe(||match op {
            "next"=>returned_piece=parser.next().map_or(Value::Null,piece),
            "peek"=>value=peek(parser.peek()),
            "peek_ahead"=>value=peek(parser.peek_ahead()),
            "set_curarg"=>parser.curarg=uint(&action["value"]),
            "set_source"=>parser.is_source_literal=action["value"].as_bool().unwrap(),
            "set_errors"=>parser.errors=read_errors(&action["value"]),
            "set_arg_places"=>parser.arg_places=read_ranges(&action["value"]),
            "set_line_spans"=>parser.line_spans=read_ranges(&action["value"]),
            "clear_errors"=>parser.errors.clear(),
            "seed_errors"=>parser.errors.push(seed_error()),
            _=>panic!("unknown action"),
        }));
        let panic=outcome.err().map(panic_kind);
        steps.push(json!({"op":op,"panic":panic,"piece":returned_piece,"value":value,"state":state(&parser)}));
    }
    json!({"constructor_panic":null,"initial":initial,"configured":configured,"steps":steps})
}
fn main() -> io::Result<()> {
    std::panic::set_hook(Box::new(|_|{}));
    let mut output=io::BufWriter::new(io::stdout().lock());
    for line in io::stdin().lock().lines() {
        let request:Value=serde_json::from_str(&line?).expect("invalid request JSON");
        serde_json::to_writer(&mut output,&execute(&request))?;
        writeln!(output)?;
    }
    Ok(())
}
