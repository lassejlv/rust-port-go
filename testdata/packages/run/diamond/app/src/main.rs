fn main(){let original=left::make();let updated=right::increment(original);let shared:common::Datum=updated;println!("{} {}",original.value,shared.value);}
