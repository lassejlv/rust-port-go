// SPDX-License-Identifier: MIT OR Apache-2.0
// Validation-only adapter for parse_format's sole rustc_index dependency.
// This enforces the original compile-time type-size assertion; it is not a
// source port of rustc_index and receives no source-completion credit.
#[macro_export]
macro_rules! static_assert_size {
    ($ty:ty, $size:expr) => {
        const _: [(); $size] = [(); ::std::mem::size_of::<$ty>()];
    };
}
