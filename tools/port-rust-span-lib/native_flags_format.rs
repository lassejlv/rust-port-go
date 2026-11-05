// Original bitflags macro implementations, observed through the same native
// mutable formatter/sink witness as the complete lib.rs formatter corpus.
#[allow(dead_code,unused_imports)]
mod observer {
 include!("native_format.rs");
 struct Radix {value:RemapPathScopeComponents,kind:u8}
 impl fmt::Debug for Radix {fn fmt(&self,f:&mut fmt::Formatter<'_>)->fmt::Result{match self.kind{0=>fmt::Binary::fmt(&self.value,f),1=>fmt::Octal::fmt(&self.value,f),2=>fmt::LowerHex::fmt(&self.value,f),3=>fmt::UpperHex::fmt(&self.value,f),_=>unreachable!()}}}
 pub fn flags(){for bits in 0..=255u8{let modes=if[0,1,4,59,255].contains(&bits){12}else{1};for kind in 0..4{run(&format!("radix/{bits}/{kind}"),&Radix{value:RemapPathScopeComponents::from_bits_retain(bits),kind},modes)}}}
}
fn main(){observer::flags()}
