// Development-only reference for the scoped std debug-builder protocol.
// SPDX-License-Identifier: MIT OR Apache-2.0
use std::cell::RefCell;
use std::fmt::{self, Debug, Write};
use std::rc::Rc;

fn hex(text: &str) -> String { text.as_bytes().iter().map(|b| format!("{b:02x}")).collect() }
type Log = Rc<RefCell<Vec<String>>>;
struct Sink { log: Log, calls: usize, fail: usize, chars: bool }
impl Sink {
    fn emit(&mut self, kind: char, text: &str) -> fmt::Result {
        self.log.borrow_mut().push(format!("{kind}:{}",hex(text)));
        self.calls += 1;
        if self.calls == self.fail { Err(fmt::Error) } else { Ok(()) }
    }
}
impl Write for Sink {
    fn write_str(&mut self, text: &str) -> fmt::Result { self.emit('S',text) }
    fn write_char(&mut self, ch: char) -> fmt::Result {
        let mut buffer=[0;4]; let text=ch.encode_utf8(&mut buffer);
        if self.chars { self.emit('C',text) } else { self.write_str(text) }
    }
}

struct Value { log: Log, id: usize, nested: bool }
fn evaluate(log: &Log, id: usize, nested: bool) -> Value {
    log.borrow_mut().push(format!("E:{id}"));
    Value {log:log.clone(),id,nested}
}
impl Debug for Value {
    fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result {
        self.log.borrow_mut().push(format!("D:{}",self.id));
        if self.id>=2000 {
            f.write_char('\n')?;
            let _=f.write_char('!');
            f.write_char('?')
        } else if self.id>=1000 {
            f.write_char('\n')?;
            let _=f.write_str("attempt");
            f.write_str("retry")
        } else if self.nested {
            let key=Value{log:self.log.clone(),id:self.id+10,nested:false};
            let value=Value{log:self.log.clone(),id:self.id+20,nested:false};
            f.debug_map().entry(&key,&value).finish()
        } else {
            f.write_str("")?;
            f.write_char(if self.id%2==0 {'é'} else {'!'})?;
            f.write_char('\n')?;
            f.write_str("x\ny")
        }
    }
}

struct Probe {log:Log,kind:usize,count:usize}
impl Debug for Probe {
    fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result {
        match self.kind {
            0|5|6|7 => {
                let mut builder=f.debug_struct("Record");
                for id in 0..self.count {
                    let value_id=if self.kind==6 {1000+id}else if self.kind==7 {2000+id}else{id};
                    builder.field(if id==0 {"first"} else {""}, &evaluate(&self.log,value_id,self.kind==5));
                }
                builder.finish()
            }
            1|2 => {
                let mut builder=f.debug_tuple(if self.kind==1 {"Tuple"} else {""});
                for id in 0..self.count {builder.field(&evaluate(&self.log,id,false));}
                builder.finish()
            }
            3 => {
                let mut builder=f.debug_list();
                for id in 0..self.count {builder.entry(&evaluate(&self.log,id,false));}
                builder.finish()
            }
            4 => {
                let mut builder=f.debug_map();
                for id in 0..self.count {builder.entry(&evaluate(&self.log,id*2,false),&evaluate(&self.log,id*2+1,false));}
                builder.finish()
            }
            _=>unreachable!(),
        }
    }
}

fn run(kind:usize,count:usize,pretty:bool,chars:bool,fail:usize)->(bool,usize,String){
    let log=Rc::new(RefCell::new(Vec::new()));
    let probe=Probe{log:log.clone(),kind,count};
    let mut sink=Sink{log:log.clone(),calls:0,fail,chars};
    let result=if pretty {write!(&mut sink,"{probe:#?}")}else{write!(&mut sink,"{probe:?}")};
    let events=log.borrow().join(",");
    (result.is_err(),sink.calls,events)
}
fn main(){
    for kind in 0..8 {for count in 0..3 {for pretty in [false,true] {for chars in [false,true] {
        let (_,calls,_)=run(kind,count,pretty,chars,0);
        for fail in 0..=calls+1 {
            let(error,_,events)=run(kind,count,pretty,chars,fail);
            println!("{kind}\t{count}\t{pretty}\t{chars}\t{fail}\t{error}\t{events}");
        }
    }}}}
}
