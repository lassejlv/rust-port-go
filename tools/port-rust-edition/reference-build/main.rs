use edition_reference::*;
use std::hash::{Hash, Hasher};

#[derive(Default)]
struct TraceHasher(Vec<String>);
impl Hasher for TraceHasher {
    fn finish(&self) -> u64 { panic!("Hash::hash unexpectedly finalized caller-owned hasher") }
    fn write(&mut self, bytes: &[u8]) { self.0.push(format!("bytes:{bytes:?}")); }
    fn write_isize(&mut self, value: isize) { self.0.push(format!("isize:{value}")); }
    fn write_usize(&mut self, value: usize) { self.0.push(format!("usize:{value}")); }
    fn write_u8(&mut self, value: u8) { self.0.push(format!("u8:{value}")); }
}

fn main() {
    for (tag, edition) in ALL_EDITIONS.iter().enumerate() {
        let mut encoder = AuditEncoder::new();
        edition.encode(&mut encoder);
        assert_eq!(encoder.operations, [CodecOp::U8(tag as u8)]);
        let mut bytes = encoder.finish();
        assert_eq!(bytes, [tag as u8]);
        bytes.extend_from_slice(MAGIC_END_BYTES);
        let mut decoder = AuditDecoder::new(&bytes, 0).unwrap();
        assert_eq!(Edition::decode(&mut decoder), *edition);
        assert_eq!(decoder.operations, [CodecOp::ReadU8]);
        assert_eq!(decoder.position(), 1);
        let mut std_hash = TraceHasher::default();
        edition.hash(&mut std_hash);
        assert_eq!(std_hash.0, [format!("isize:{tag}")]);
        let mut digests = Vec::new();
        for prefix in [b"".as_slice(), b"prefix".as_slice(), &[0,1,2,255]] {
            let mut hasher = StableHasher::new();
            hasher.write(prefix);
            edition.stable_hash(&mut DummyStableHashCtxt, &mut hasher);
            let value: Hash128 = hasher.finish();
            digests.push(value.as_u128());
        }
        assert_ne!(digests[0], digests[1]);
        println!("{tag} {edition:?} std_hash={:?} stable={digests:?}", std_hash.0);
    }
    std::panic::set_hook(Box::new(|_| {}));
    for tag in 5..=255u8 {
        let mut bytes = vec![tag]; bytes.extend_from_slice(MAGIC_END_BYTES);
        let mut decoder = AuditDecoder::new(&bytes, 0).unwrap();
        let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| Edition::decode(&mut decoder)));
        let panic = result.expect_err("invalid enum tag accepted");
        let message = panic.downcast_ref::<String>().map(String::as_str).or_else(|| panic.downcast_ref::<&str>().copied()).unwrap();
        assert!(message.starts_with("invalid enum variant tag while decoding `Edition`, expected 0..5, actual "));
        assert_eq!(decoder.operations, [CodecOp::ReadU8]);
        assert_eq!(decoder.position(), 1);
    }
    assert!(AuditDecoder::new(&[0], 0).is_err());
    assert_eq!(unused_interface_calls(), 0);
    println!("all 5 editions and 251 invalid tags passed; unused interface calls=0");
}
