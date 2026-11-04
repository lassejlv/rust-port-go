mod sealed;
fn main() { let record = sealed::Record { value: 1 }; println!("{}", record.value); }
