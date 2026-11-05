The unchanged official SDK 1.101 operations match 197,024 deterministic native
requests, including 85,911 panics. Retained fixture and tool/source identities
are in `port/support/ruststr/testdata/manifest.json`. Inputs cover all six
failure priorities, usize maximum, empty and long strings, inclusive and
consumed singleton ranges, NUL, invalid boundaries, representative Unicode
controls and 512 seeded Unicode scalars, plus 2,000 random short sequences.
Unicode character Debug itself has a separate exhaustive Unicode 18 proof.

All 27 original ambient records match, including eight actual native aborts.
The records exercise normal, inherited-unwind and panic-hook execution,
preserving hook/caller-drop order and the absence of cleanup during abort.

Actual Go 1.23.12 ordinary tests, race tests (3.636s), and vet passed. Successful
nonempty slices borrow the original backing bytes. Dedicated tests reject
invalid Go UTF-8 at `Checked`, retain honestly absent default locations, and
compare `Get` and all boundary methods with independent boundary scans.
An actual Go 1.23.12 fuzz run completed 238,325 executions in 11.020s without a
failure. Go 1.27.3 ordinary replay also passed.

Twelve tool integrity tests cover official archive/channel/source/owner tamper,
unowned caches, symlinks, compiler selection and bare PATH resolution,
bounded gzip/CRC/trailing-byte handling, a copied-checkout shared-toolchain symlink live check, and rejected CLI calls preserving all
evidence. Both live original-SDK `--check` commands also run with a bare `rustc`
on an explicit minimal PATH. They preserve byte-identical retained fixtures.

This evidence validates only the declared used-contract adapter. It does not
port entire core source modules, the general Range iterator family, global
panic hooks, const evaluation, or invalid-UTF-8 unsafe inputs.
