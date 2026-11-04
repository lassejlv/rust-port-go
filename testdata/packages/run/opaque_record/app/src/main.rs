fn main(){let token=vault::make();let copy=token;println!("{} {}",vault::read(token),vault::read(copy));}
