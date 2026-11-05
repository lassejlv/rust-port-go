# Scoped Unix errno formatting

This helper implements the `format_error` contract used by the selected
SDK1.101 (`708c94ef093ecc42fb8851e17d8383d999727abc`) on Unix: XSI
`strerror_r`, a zeroed128-byte temporary, the exact negative-return check and
lossy CStr display chunks. It does not grant completion credit for a standard
library or libc source file. It covers the Unix protocol; WASI's1024-byte
variant and teeos's separate string are outside this helper.

The default cgo backend calls the actual system library, using
`__xpg_strerror_r` on glibc and `strerror_r` on musl/other Unix C libraries.
It follows the current host locale. Callers with thread-local locale state
must retain their source thread or supply an explicit Binding. No Go errno
table, capitalization rule, or Linux message table substitutes for host libc.

When cgo is disabled or this host has no binding, callers must provide a
complete `Binding`. An omitted binding raises `MissingBinding`, which is an
explicit host-configuration failure, not an invented source error message.
`RealFileLoader.UnixErrorBinding` carries this witness into SourceMap's lazy
error formatting. Its borrowed state must remain alive and obey the original
shared/Send/Sync laws until the error has finished formatting.

Source panic locations remain caller-selected semantic profile entries.
Ordinary failure uses `Context.PanicAt`; processing panic and inherited unwind
use the neutral runtime's established behavior. The native system call's
buffer must be NUL terminated, as required by the source unsafe CStr borrow.

The standalone actual-std oracle is in `tools/port-rust-errno`. Its first Linux
run matched84,123 records, including complete non-NUL one/two-byte C strings,
seeded longer invalid UTF-8, fixed-buffer/error-code behavior and fallible
writer call order. A macOS run compares macOS std with macOS libc; it does not
reuse Linux messages as expected data.
