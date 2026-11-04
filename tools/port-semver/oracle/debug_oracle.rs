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
fn main(){
 let plain:semver::Version="10.26.31".parse().unwrap();let full:semver::Version="10.26.31-alpha.0+001.build".parse().unwrap();let max:semver::Version="18446744073709551615.26.31".parse().unwrap();
 let req:semver::VersionReq="^10.26.31, <32".parse().unwrap();let empty:semver::VersionReq="*".parse().unwrap();let cmp:semver::Comparator=">=10.26.31-alpha".parse().unwrap();let raw=semver::Comparator{op:semver::Op::Exact,major:10,minor:None,patch:Some(31),pre:semver::Prerelease::new("alpha").unwrap()};
 let pe=semver::Prerelease::new("").unwrap();let pre=semver::Prerelease::new("alpha.0").unwrap();let be=semver::BuildMetadata::new("").unwrap();let build=semver::BuildMetadata::new("001.build").unwrap();
 let e1=semver::Version::parse("1.2.3+🦀").unwrap_err();let e2=semver::Version::parse("1.2.3-\n").unwrap_err();let e3=semver::VersionReq::parse(">=1.2.3||<2").unwrap_err();let e4=semver::Prerelease::new("01").unwrap_err();let e5=semver::BuildMetadata::new("a..b").unwrap_err();
 let mut values:Vec<(&str,&dyn Debug)>=vec![("version_plain",&plain),("version_full",&full),("version_max",&max),("req",&req),("req_empty",&empty),("comparator",&cmp),("comparator_raw",&raw),("pre_empty",&pe),("pre",&pre),("build_empty",&be),("build",&build),("error_unicode",&e1),("error_newline",&e2),("error_req",&e3),("error_zero",&e4),("error_build",&e5)];
 let ops=[semver::Op::Exact,semver::Op::Greater,semver::Op::GreaterEq,semver::Op::Less,semver::Op::LessEq,semver::Op::Tilde,semver::Op::Caret,semver::Op::Wildcard];for op in &ops{values.push(("op",op))}
 for(name,value)in values{for mode in 0..14{let mut sink=Sink{calls:vec![],accepted:String::new(),fail:0};assert!(render(&mut sink,value,mode).is_ok());let count=sink.calls.len();let mut cuts=vec![0,count/2,count,count+1];cuts.extend(1..=12.min(count));cuts.sort_unstable();cuts.dedup();for fail in cuts{let mut sink=Sink{calls:vec![],accepted:String::new(),fail};let error=render(&mut sink,value,mode).is_err();println!("{name}\t{mode}\t{fail}\t{error}\t{}\t{}",hex(&sink.accepted),sink.calls.iter().map(|s|hex(s)).collect::<Vec<_>>().join(","));}}}
}