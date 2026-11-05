// SPDX-License-Identifier: MIT OR Apache-2.0
// Actual public std owning operations, not a replacement of their algorithms.
use std::borrow::Cow;
use std::ffi::{OsStr, OsString};
use std::io::{self, BufRead};
use std::panic::{self, AssertUnwindSafe};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
#[cfg(unix)] use std::os::unix::ffi::{OsStrExt, OsStringExt};
#[cfg(windows)] use std::os::windows::ffi::OsStringExt;

fn unhex(s: &str) -> Vec<u8> {
    (0..s.len()).step_by(2).map(|i| u8::from_str_radix(&s[i..i+2], 16).unwrap()).collect()
}
fn hex(bytes: &[u8]) -> String { bytes.iter().map(|b| format!("{b:02x}")).collect() }
fn os(s: &str) -> OsString {
    let data = unhex(s);
    #[cfg(unix)] { OsString::from_vec(data) }
    #[cfg(windows)] { OsString::from_wide(&data.chunks_exact(2).map(|c| u16::from_le_bytes([c[0],c[1]])).collect::<Vec<_>>()) }
}
fn snapshot(p: &PathBuf) -> String { format!("{}:{}:{}",p.as_os_str().len(),p.capacity(),hex(p.as_os_str().as_encoded_bytes())) }
fn ssnapshot(p: &String) -> String { format!("{}:{}:{}",p.len(),p.capacity(),hex(p.as_bytes())) }
fn main() {
    let hook = Arc::new(Mutex::new(String::new()));
    let output = hook.clone();
    panic::set_hook(Box::new(move |info| {
        let (kind, message) = if let Some(s) = info.payload().downcast_ref::<String>() { ("owned",s.as_str()) }
            else if let Some(s) = info.payload().downcast_ref::<&str>() { ("static",*s) } else { ("opaque","") };
        *output.lock().unwrap() = format!("P:{kind}:{}",hex(message.as_bytes()));
    }));
    for line in io::stdin().lock().lines() {
        let line=line.unwrap(); let f:Vec<_>=line.split('\t').collect();
        let mut path=match f[1] {
            "from"=>PathBuf::from(os(f[3]).as_os_str()),
            "wide"=>PathBuf::from(os(f[3])),
            "string"=>{ let mut s=String::with_capacity(f[2].parse().unwrap()); s.push_str(std::str::from_utf8(&unhex(f[3])).unwrap()); let ptr=s.as_ptr(); let p=PathBuf::from(s); assert_eq!(p.as_os_str().as_encoded_bytes().as_ptr(),ptr); p },
            "cap"=>{ let mut p=PathBuf::with_capacity(f[2].parse().unwrap());p.as_mut_os_string().push(os(f[3])); p },
            _=>unreachable!(),
        };
        let mut rows=vec![snapshot(&path)];
        for op in f[4].split(';').filter(|s|!s.is_empty()) {
            let parts:Vec<_>=op.split(',').collect(); let arg=parts.get(1).copied().unwrap_or("");
            *hook.lock().unwrap()=String::new();
            let mut event=String::from("-");
            let result=panic::catch_unwind(AssertUnwindSafe(|| {
                match parts[0] {
                    "push"=>path.push(os(arg)),
                    "ospush"=>path.as_mut_os_string().push(os(arg)),
                    "pop"=>event=format!("{}",path.pop()),
                    "name"=>path.set_file_name(os(arg)),
                    "ext"=>event=format!("{}",path.set_extension(os(arg))),
                    "add"=>event=format!("{}",path.add_extension(os(arg))),
                    "clear"=>path.clear(),
                    "reserve"=>path.reserve(arg.parse().unwrap()),
                    "exact"=>path.reserve_exact(arg.parse().unwrap()),
                    "shrink"=>path.shrink_to(arg.parse().unwrap()),
                    "fit"=>path.shrink_to_fit(),
                    "lower"=>path.as_mut_os_str().make_ascii_lowercase(),
                    "upper"=>path.as_mut_os_str().make_ascii_uppercase(),
                    "clone"=>{let copy=path.clone();event=format!("C:{}:{}",snapshot(&copy),copy.as_os_str().as_encoded_bytes().as_ptr()==path.as_os_str().as_encoded_bytes().as_ptr());},
                    "clonefrom"=>{let mut copy=PathBuf::with_capacity(arg.parse().unwrap());copy.as_mut_os_string().push("123");copy.clone_from(&path);event=format!("F:{}",snapshot(&copy));},
                    "move"=>{let ptr=path.as_os_str().as_encoded_bytes().as_ptr();let p=std::mem::take(&mut path);path=p;event=format!("M:{}",ptr==path.as_os_str().as_encoded_bytes().as_ptr());},
                    "round"=>{let ptr=path.as_os_str().as_encoded_bytes().as_ptr();let p=std::mem::take(&mut path).into_os_string();path=PathBuf::from(p);event=format!("R:{}",ptr==path.as_os_str().as_encoded_bytes().as_ptr());},
                    "intostring"=>{let ptr=path.as_os_str().as_encoded_bytes().as_ptr();let value=std::mem::take(&mut path).into_os_string().into_string();path=match value {Ok(s)=>{event=format!("S:{}",ptr==s.as_ptr());PathBuf::from(s)},Err(s)=>{event=format!("E:{}",ptr==s.as_encoded_bytes().as_ptr());PathBuf::from(s)}};},
                    "join"=>{path=path.join(os(arg));},
                    "cowborrow"=>{let mut c:Cow<'_,Path>=Cow::Borrowed(path.as_path());let before=c.as_os_str().as_encoded_bytes().as_ptr();let clone=c.clone();assert!(matches!(clone,Cow::Borrowed(_)));assert_eq!(before,clone.as_os_str().as_encoded_bytes().as_ptr());let own=c.to_mut();event=format!("B:{}:{}",snapshot(own),own.as_os_str().as_encoded_bytes().as_ptr()==before);path=c.into_owned();},
                    "cowowned"=>{let before=path.as_os_str().as_encoded_bytes().as_ptr();let c:Cow<'_,Path>=Cow::Owned(std::mem::take(&mut path));let clone=c.clone();event=format!("O:{}",snapshot(&clone.into_owned()));path=c.into_owned();event.push_str(&format!(":{}",before==path.as_os_str().as_encoded_bytes().as_ptr()));},
                    "cowclonefrom"=>{let mut dest:Cow<'_,Path>=Cow::Owned(PathBuf::with_capacity(arg.parse().unwrap()));dest.to_mut().as_mut_os_string().push("123");let source:Cow<'_,Path>=Cow::Owned(path.clone());dest.clone_from(&source);event=format!("D:{}",snapshot(&dest.into_owned()));},
                    "cowtrans"=>{let mut result=Vec::new();for dest_owned in [false,true] {for source_owned in [false,true] {
                        let mut dest:Cow<'_,Path>=if dest_owned {let mut p=PathBuf::with_capacity(64);p.as_mut_os_string().push("old");Cow::Owned(p)} else {Cow::Borrowed(Path::new("old"))};
                        let source:Cow<'_,Path>=if source_owned {Cow::Owned(path.clone())} else {Cow::Borrowed(path.as_path())};
                        dest.clone_from(&source);let owned=matches!(dest,Cow::Owned(_));result.push(format!("{}:{}",owned,snapshot(&dest.into_owned())));
                    }}event=format!("V:{}",result.join("~"));},
                    "lossy"=>{let c=path.to_string_lossy();let owned=matches!(c,Cow::Owned(_));let before=path.as_os_str().as_encoded_bytes().as_ptr();let alias=before==c.as_ptr();let s=c.into_owned();event=format!("L:{}:{}:{}",owned,alias,ssnapshot(&s));},
                    "lossymut"=>{let mut c=path.to_string_lossy();c.to_mut().push_str(std::str::from_utf8(&unhex(arg)).unwrap());event=format!("T:{}",ssnapshot(&c.into_owned()));},
                    "cowstr"=>{let mut s=path.to_string_lossy().into_owned();s.reserve(arg.parse().unwrap());let ptr=s.as_ptr();let c:Cow<'_,str>=Cow::Owned(s);let clone=c.clone().into_owned();event=format!("U:{}",ssnapshot(&clone));let s=c.into_owned();event.push_str(&format!(":{}:{}",ssnapshot(&s),ptr==s.as_ptr()));},
                    "cowstrtrans"=>{let text=path.to_string_lossy();let mut result=Vec::new();for dest_owned in [false,true] {for source_owned in [false,true] {
                        let mut dest:Cow<'_,str>=if dest_owned {let mut s=String::with_capacity(64);s.push_str("old");Cow::Owned(s)} else {Cow::Borrowed("old")};
                        let source:Cow<'_,str>=if source_owned {Cow::Owned(text.to_string())} else {Cow::Borrowed(&text)};
                        dest.clone_from(&source);let owned=matches!(dest,Cow::Owned(_));result.push(format!("{}:{}",owned,ssnapshot(&dest.into_owned())));
                    }}event=format!("W:{}",result.join("~"));},
                    "stable"=>{let ptr=path.as_os_str().as_encoded_bytes().as_ptr();path.as_mut_os_str().make_ascii_lowercase();event=format!("I:{}",ptr==path.as_os_str().as_encoded_bytes().as_ptr());},
                    _=>panic!("unknown operation"),
                }
            }));
            if result.is_err() { event=hook.lock().unwrap().clone(); }
            rows.push(format!("{event}/{}",snapshot(&path)));
        }
        println!("{}\t{}",f[0],rows.join("|"));
    }
}
