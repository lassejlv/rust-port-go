This directory preserves the complete tests from the official
rustc-stable-hash 0.1.2 archive, SHA-256
781442f29170c5c93b7185ad559492601acdc71d5bb0706f5868094f45cfcd08.
The implementation and independent algorithm/ownership oracle belong to the
parent tool directory and src package.

| Original source | Lines | Target test mappings |
| --- | ---: | --- |
| src/sip128/tests.rs | 320 | tests/sip128_test.go, sip128_vector_test.go, sip128_arch_{amd64,386,arm}_test.go and shared util_test.go |
| src/stable_hasher/tests.rs | 155 | tests/stable_hasher_test.go and shared util_test.go |
| Three stable_hasher.rs source doc examples | 17–45, 71–89, 100–115 | tests/doc_examples_test.go |

The source contains 17 test declarations: 11 in sip128, including three
architecture-gated declarations with the same name, and 6 in stable_hasher.
On x86_64, 15 execute. The audit retains all 38 assertion sites across all gates
(34 active sites on x86_64), signed/unsigned literal values, declaration and
assertion order, the exact 64 × 16-byte test vector, and all 12 fill-buffer type
invocations. Each fill case retains every original offset. Unit literal
coverage includes whole files and checks individual test bodies separately;
the vector data is compared in exact row/element order.

The cfg mappings are x86_64→amd64, x86→386, and arm→arm. Other architectures
retain the 14 ungated tests. Compile-only evidence does not assert runtime
behavior or execute arithmetic profiles on the target. The current amd64 host
cannot execute the 386 ELF32 binaries, and no ARM runtime/emulator was used.

The Go helper adapts only the original test input families. It does not claim
to implement Rust's entire standard Hash trait. The retained compiler witness
checks string byte writes plus 0xff sentinels, slice length prefixes followed by
one bulk native-endian i32 payload write (including empty writes), and tuple
field order. The byte wrapper deliberately writes one typed u8 per element
without a slice prefix. A supplemental Go test compares its actual dispatcher
with that native compiler trace on the trace's matching architecture.

Rust 128-bit literals remain exact arbitrary-precision Go constants before
decomposition into the port's low/high limbs. Signed 128-bit values retain
sign extension. Private pure TestHash conversion wraps FinishHash. Public
ExtendedHasher/FromStableHash descriptors are used by the doc examples.
The first and third original doc examples only declare implementations; their
Go mappings remain compile witnesses, without invented source assertions.
The second executes the typed finish. The third example intentionally places
hash[0] in the upper limb and hash[1] in the lower limb.

Go moves nested zero_byte functions immediately after their corresponding
tests to preserve source assertion order while satisfying Go declaration
rules. Go uses its testing diagnostics and explicit input adapters rather than
porting Rust's test framework. Exact original Rust tests and all three original
doc fragments are retained under fixtures and checked against the archive.
Parent dependency MIT/Apache licenses cover these original bytes.

Run from the repository root with Go 1.23 or later:

    python3 tools/port-rust-stable-hash/original-tests/audit.py --check
    python3 tools/port-rust-stable-hash/original-tests/run.py --offline
    python3 tools/port-rust-stable-hash/original-tests/go_profiles.py
    go test -race ./port/dependencies/rustc-stable-hash/v0_1_2/tests
    go test -race -tags rustc_stable_hash_nightly ./port/dependencies/rustc-stable-hash/v0_1_2/tests
    go vet ./port/dependencies/rustc-stable-hash/v0_1_2/tests ./tools/port-rust-stable-hash/original-tests

The Rust runner extracts all 17 archive files unchanged, including the official
Cargo manifest and lockfile, into an ignored workspace. It checks all original
file bytes again after compilation. There are no external Cargo dependencies.
Both default/nightly features run with each independent combination of
overflow-checks and debug-assertions. Each of the 8 profiles runs 15 original
unit tests and 3 doc examples on the current x86_64 compiler. RUSTC_BOOTSTRAP=1
only enables the source's existing nightly feature; no body or manifest edits
are made. The Go runner independently verifies 15 source unit test names and 3
doc mapping names per profile.

FERRUGO_GO, FERRUGO_CARGO, FERRUGO_RUSTC, FERRUGO_RUSTDOC and CARGO_HOME
select host tools/cache; project-local toolchains/cache are defaults. A missing
or mismatched selected archive fails explicitly. No network fetch is needed
once the exact archive and compiler are available.

Cross-compile both feature variants without executing them:

    python3 tools/port-rust-stable-hash/original-tests/go_profiles.py --goarch 386 --compile-only
    python3 tools/port-rust-stable-hash/original-tests/go_profiles.py --goarch arm --compile-only

The Rust runner also
accepts --target and --compile-only when that compiler's target standard
library is installed; the corresponding Rust target libraries were not
installed or fetched for this native validation. The optional Go integration
test runs the unchanged Rust profiles when
FERRUGO_RUN_STABLE_HASH_ORIGINAL_TESTS=1.

audit.json records exact source identities, declarations/gates, literals and
target hashes. validation.json records completed profile observations and
their limits; detailed logs/evidence are ignored under
.toolchain/oracles/stable-hash-original-tests. This directory grants no
implementation-file or whole-project inventory credit.
