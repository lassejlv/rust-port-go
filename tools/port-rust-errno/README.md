# Native Unix errno used-contract proof

This standalone tool needs only the exact official SDK1.101 compiler, its
verified rust-src archive, Go1.23 or newer and the host C compiler. It imports
the scoped `rusterrno` helper and neutral formatter; it does not build the WIP
span package or compiler graph.

```sh
GOCACHE=/tmp/ferrugo-go123-cache python3 tools/port-rust-errno/generate.py --check --go /path/to/go --rustc /path/to/rustc
```

`--check` executes the actual public std `io::Error::Display` and
`CStr::display` operations and compares every result and C/S writer call with
the real Go helper. The comparison runs against the current host libc, so the
same command is suitable for a macOS native CI runner. It does not compare
macOS results with retained Linux message bytes. `--output PATH` writes that
host's bounded proof metadata; without either option it refreshes the retained
producer `VALIDATION.json`.

The SDK archive/channel identities are verified by the existing ruststr
preparation helper, then the exact Unix error, CStr and ByteStr source members
are hashed into the result. Compiler overrides reject a different commit;
executables preserve their invoked name. Child compile/run calls have finite
timeouts. There are84,123 requests:4,355 errno values, errno writer failures,
all non-NUL one/two-byte strings and seeded longer UTF-8 boundary/invalid
strings with fallible sinks. Only native outputs supply expected results.

The actual native strerror_r implementation does not return a negative value
for this code domain on the tested Linux host. Synthetic binding unit tests
separately cover the source negative-result branch, exact hook/profile
propagation, zero buffer, positive-return handling and explicit no-cgo
configuration failure; those are not mislabeled as native failure evidence.
No whole std/libc/dependency source-file credit is claimed.
