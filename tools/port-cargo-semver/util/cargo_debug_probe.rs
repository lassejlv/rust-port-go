use std::fmt::{self,Debug};
fn hex(s:&str)->String{s.as_bytes().iter().map(|b|format!("{b:02x}")).collect()}
struct Sink{calls:Vec<String>,accepted:String,fail:usize}
impl fmt::Write for Sink{fn write_str(&mut self,s:&str)->fmt::Result{self.calls.push(s.into());if self.fail!=0&&self.calls.len()==self.fail{Err(fmt::Error)}else{self.accepted.push_str(s);Ok(())}}}
fn render(w:&mut Sink,value:&dyn Debug,mode:usize)->fmt::Result{match mode{
0=>fmt::write(w,format_args!("{value:?}")),1=>fmt::write(w,format_args!("{value:#?}")),
2=>fmt::write(w,format_args!("{value:x?}")),3=>fmt::write(w,format_args!("{value:X?}")),
4=>fmt::write(w,format_args!("{value:#x?}")),5=>fmt::write(w,format_args!("{value:#X?}")),
6=>fmt::write(w,format_args!("{value:+?}")),7=>fmt::write(w,format_args!("{value:08?}")),
8=>fmt::write(w,format_args!("{value:+08x?}")),9=>fmt::write(w,format_args!("{value:>8?}")),
10=>fmt::write(w,format_args!("{value:*^9?}")),11=>fmt::write(w,format_args!("{value:.2?}")),
12=>fmt::write(w,format_args!("{value:+#08X?}")),13=>fmt::write(w,format_args!("{value:>+#12.2X?}")),_=>unreachable!()}}
#[path="../util/mod.rs"] mod util;
fn main(){
 use util::semver_ext::{OptVersionReq,VersionReqMatchMode};
 let full:semver::Version="18446744073709551615.26.31-alpha.0+001.build".parse().unwrap();
 let plain:semver::Version="10.26.31".parse().unwrap();let req:semver::VersionReq="^10.26.31-alpha, <32".parse().unwrap();let empty:semver::VersionReq="*".parse().unwrap();
 let raw=semver::VersionReq{comparators:vec![semver::Comparator{op:semver::Op::Exact,major:10,minor:None,patch:Some(31),pre:semver::Prerelease::new("alpha").unwrap()}]};
 let any=OptVersionReq::Any;let reqempty=OptVersionReq::Req(empty.clone());let reqmulti=OptVersionReq::Req(req.clone());let reqraw=OptVersionReq::Req(raw.clone());
 let locked=OptVersionReq::Locked(full.clone(),req.clone());let precise=OptVersionReq::Precise(full.clone(),req.clone());let lockedempty=OptVersionReq::Locked(plain.clone(),empty.clone());let preciseraw=OptVersionReq::Precise(plain.clone(),raw.clone());
 let values:Vec<(&str,&dyn Debug)>=vec![("any",&any),("req_empty",&reqempty),("req_multi",&reqmulti),("req_raw",&reqraw),("locked",&locked),("precise",&precise),("locked_empty",&lockedempty),("precise_raw",&preciseraw),("mode_default",&VersionReqMatchMode::Default),("mode_prerelease",&VersionReqMatchMode::Prerelease)];
 for(name,value)in values{for mode in 0..14{let mut sink=Sink{calls:vec![],accepted:String::new(),fail:0};assert!(render(&mut sink,value,mode).is_ok());let count=sink.calls.len();for fail in 0..=count+1{let mut sink=Sink{calls:vec![],accepted:String::new(),fail};let error=render(&mut sink,value,mode).is_err();println!("{name}\t{mode}\t{fail}\t{error}\t{}\t{}",hex(&sink.accepted),sink.calls.iter().map(|s|hex(s)).collect::<Vec<_>>().join(","));}}}
}