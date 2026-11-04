// Development oracle for std character behavior used by rustc_parse_format.
// SPDX-License-Identifier: MIT OR Apache-2.0
use std::io::{self,Write};
fn main(){
 assert_eq!(std::char::UNICODE_VERSION,(17,0,0));
 let mut output=io::BufWriter::new(io::stdout().lock());
 output.write_all(b"PFCU2\0\x11\0\0").unwrap();
 for value in 0..0x110000{
  let mut flag=0;
  if let Some(c)=char::from_u32(value){
   let escaped=c.escape_debug().to_string();
   if escaped==format!("\\u{{{value:x}}}"){flag|=1;}else if escaped!=c.to_string(){
    let expected=match c{'\0'=>"\\0",'\t'=>"\\t",'\r'=>"\\r",'\n'=>"\\n",'\\'=>"\\\\",'\''=>"\\'",'"'=>"\\\"",_=>panic!("unrecognized escape_debug mapping {value:x}: {escaped}")};
    assert_eq!(escaped,expected);
   }
   let single_debug=format!("{:?}",c.to_string());
   let single_body=&single_debug[1..single_debug.len()-1];
   assert_eq!(single_body==format!("\\u{{{value:x}}}"),flag&1!=0,"leading string escape differs {value:x}");
   let following_debug=format!("{:?}",format!("A{c}"));
   let following_body=&following_debug[2..following_debug.len()-1];
   if following_body==format!("\\u{{{value:x}}}"){flag|=4;}else if following_body!=c.to_string(){
    let expected=match c{'\0'=>"\\0",'\t'=>"\\t",'\r'=>"\\r",'\n'=>"\\n",'\\'=>"\\\\",'"'=>"\\\"",_=>panic!("unrecognized string Debug mapping {value:x}: {following_body}")};
    assert_eq!(following_body,expected);
   }
   let char_debug=format!("{c:?}");let char_body=&char_debug[1..char_debug.len()-1];
   let char_expected=if c=='"'{c.to_string()}else{escaped};
   assert_eq!(char_body,char_expected,"character Debug differs {value:x}");
   if c.is_whitespace(){flag|=2;}
  }
  output.write_all(&[flag]).unwrap();
 }
}
