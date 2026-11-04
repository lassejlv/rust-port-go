use storage_reference::{IndexVec, Idx};
use storage_reference as rustc_index;

rustc_index::newtype_index! {
    #[gate_rustc_only]
    struct ProofIdx {}
}

fn main() {
    let values = IndexVec::<ProofIdx, u32>::from_raw(vec![0, 127, 128, u32::MAX]);
    assert_eq!(values.len(), 4);
    assert_eq!(values[ProofIdx::new(2)], 128);
    #[cfg(feature = "nightly")]
    {
        use storage_reference::{Decodable, Encodable};
        use storage_reference::opaque::{MemDecoder, MAGIC_END_BYTES};
        use storage_reference::opaque::mem_encoder::MemEncoder;
        let mut encoder = MemEncoder::new();
        values.encode(&mut encoder);
        let mut bytes = encoder.finish();
        bytes.extend_from_slice(MAGIC_END_BYTES);
        let mut decoder = MemDecoder::new(&bytes, 0).unwrap();
        let decoded = IndexVec::<ProofIdx, u32>::decode(&mut decoder);
        assert_eq!(decoded.raw, values.raw);
        println!("actual rustc_index vector and rustc_serialize codec round-trip passed");
    }
    #[cfg(not(feature = "nightly"))]
    println!("actual rustc_index vector storage passed");
}
