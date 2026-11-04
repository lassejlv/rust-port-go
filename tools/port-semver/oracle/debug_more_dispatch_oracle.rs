use std::fmt::{self,Debug};
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
struct Sink{calls:Vec<String>,accepted:String,fail:usize}
impl fmt::Write for Sink{
 fn write_str(&mut self,s:&str)->fmt::Result{self.calls.push(format!("S:{s}"));if self.fail!=0&&self.calls.len()==self.fail{Err(fmt::Error)}else{self.accepted.push_str(s);Ok(())}}
 fn write_char(&mut self,c:char)->fmt::Result{self.calls.push(format!("C:{c}"));if self.fail!=0&&self.calls.len()==self.fail{Err(fmt::Error)}else{self.accepted.push(c);Ok(())}}
}
fn render(w:&mut Sink,value:&dyn Debug,mode:usize)->fmt::Result{match mode{
0=>fmt::write(w,format_args!("{value:?}")),1=>fmt::write(w,format_args!("{value:#?}")),
2=>fmt::write(w,format_args!("{value:x?}")),3=>fmt::write(w,format_args!("{value:X?}")),
4=>fmt::write(w,format_args!("{value:#x?}")),5=>fmt::write(w,format_args!("{value:#X?}")),
6=>fmt::write(w,format_args!("{value:+?}")),7=>fmt::write(w,format_args!("{value:08?}")),
8=>fmt::write(w,format_args!("{value:+08x?}")),9=>fmt::write(w,format_args!("{value:>8?}")),
10=>fmt::write(w,format_args!("{value:*^9?}")),11=>fmt::write(w,format_args!("{value:.2?}")),
12=>fmt::write(w,format_args!("{value:+#08X?}")),13=>fmt::write(w,format_args!("{value:>+#12.2X?}")),_=>unreachable!()}}
fn main(){
 let inputs=[("word_joiner","\u{2060}.2.3"),("quote","'.2.3"),("slash","\\.2.3"),("control","\u{1}.2.3"),("del","\u{7f}.2.3"),("accent","é.2.3"),("crab","🦀.2.3"),("new_unicode","\u{1e6c0}.2.3"),("nul","\0.2.3")];
 for(name,text)in inputs{let error=semver::Version::parse(text).unwrap_err();for mode in 0..14{let mut sink=Sink{calls:vec![],accepted:String::new(),fail:0};assert!(render(&mut sink,&error,mode).is_ok());let count=sink.calls.len();for fail in 0..=count+1{let mut sink=Sink{calls:vec![],accepted:String::new(),fail};let rejected=render(&mut sink,&error,mode).is_err();println!("{name}\t{mode}\t{fail}\t{rejected}\t{}\t{}",hex(&sink.accepted),sink.calls.iter().map(|s|hex(s)).collect::<Vec<_>>().join(","));}}}
}