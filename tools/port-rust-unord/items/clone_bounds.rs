// Compile-only tests of actual source bounds, not a replacement trait body.
use rustc_data_structures::unord::UnordItems;
struct NoClone;
#[cfg(no_item_clone)]
fn main() {
    let items = UnordItems::<NoClone, std::iter::Empty<NoClone>>::empty();
    let _ = items.clone();
}
#[cfg(opaque_map_clone)]
fn main() {
    let items = UnordItems::new([1u64].into_iter()).map(|value| value);
    let _ = items.clone();
}
#[cfg(opaque_filter_clone)]
fn main() {
    let items = UnordItems::new([1u64].into_iter()).filter(|_| true);
    let _ = items.clone();
}
#[cfg(not(any(no_item_clone, opaque_map_clone, opaque_filter_clone)))]
fn main() {
    assert_eq!(UnordItems::<NoClone, std::iter::Empty<NoClone>>::empty().count(), 0);
    assert_eq!(UnordItems::new([1u64].into_iter()).clone().count(), 1);
}
