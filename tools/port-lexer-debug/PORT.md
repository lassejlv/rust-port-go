# Source/API contract map

This closes the whole pinned lexer implementation and the separate
literal-escaper implementation contracts, preserving their existing algorithms.
Source completion is recorded by the repository coordinator, not by this tool.

| Original source surface | Go representation |
| --- | --- |
| Lexer Token, TokenKind, DocStyle, LiteralKind, GuardedStr, RawStrError, Base Debug | Each type's primary `(*T).FormatDebug(*rustfmt.Formatter) error`; `RustDebug` is the ordinary-default collection convenience |
| LiteralKind, GuardedStr, RawStrError Ord/PartialOrd | `Compare` and `PartialCompare`, with source discriminants, active field order and Option None before Some |
| Base Ord/PartialOrd | Ordered numeric discriminants 2, 8, 10, 16 |
| Source closed enum/record Clone, Copy and equality derives | Go value assignment and `==` on lawful tagged values; inactive representation fields remain zero |
| tokenize's lazy from_fn iterator | `NewTokenizer` / `TokenizeIterator`, `Next`, default `SizeHint`, `Nth`, consuming `Count` and `Last` |
| Iterator consuming FnMut operations used by the adapter | `ForEachTokenOwned`, `FoldTokensOwned`, `TokenConsumer` and `TokenFolder`; plain `ForEach` has a trivial-drop environment |
| Cursor::eat_while owned FnMut | `Predicate[Env]` and `EatWhileOwned`; `Borrow` retains the same mutable environment cell with no callee-owned destructor |
| All lexer cursor functions and classification/token/raw-string helpers | Existing complete `lib.go` APIs, unchanged algorithm corpus, plus independent configuration constructors |
| Source debug_assertions and arithmetic overflow build settings | `CursorConfig{OverflowChecks, DebugAssertions}`; checked/debug default and all four explicit combinations |
| Source ambient panic state at internal failure sites | Private cursor context; constructor/validator/shebang `WithContext` / `WithConfigAndContext` forms and per-operation context forms for existing cursors/iterators |
| EscapeError, MixedUnit, Mode derived Debug | Primary `FormatDebug`, shared formatter/options and fallible sink; ordinary `RustDebug` convenience |
| MixedUnit's NonZero<char>/NonZero<u8> Debug | `NonZeroChar`/`NonZeroByte` primitive witnesses using exact native char/integer dispatch |
| Literal's seven owned FnMut entry points | `CheckRawStrOwned`, `CheckRawByteStrOwned`, `CheckRawCStrOwned`, `UnescapeStrOwned`, `UnescapeByteStrOwned`, `UnescapeCStrOwned`, `CheckForErrorsOwned` with `UnitCallback`/`ErrorCallback` |
| Literal's private CheckRaw/Unescape traits | All three fixed source specializations remain implemented; these private traits have no external implementation surface |

The final derive audit found no source Default or Hash implementation in these
files. Token has Debug only; Cursor and FrontmatterAllowed have no source Debug
or Clone derive. No such source traits are advertised. DocStyle's pre-existing
`String` describes its Option-shaped representation; standalone source Debug
prints Outer/Inner. Go's NoDocStyle, NoError, inactive union fields and invalid
discriminants are representation helpers, not invented Rust variants.
Go assignment may exist for additional host types without claiming a Rust Copy
derive. Arbitrary invalid Go union states are outside the source input domain.

`rustfmt.Formatter` is the neutral primary descriptor. It retains the mutable
formatter object, options and fallible caller writer. Independent character
dispatch uses a writer's `WriteChar(rune) error`; otherwise the source writer's
default converts that character to a string write. Compact fields share the
formatter object. Pretty fields use copied options and an indentation writer.
Numeric failure can leave fill/alignment changed; caller reuse sees that state.
Strings/chars use independently verified Unicode 17 Debug escaping. No compiler
package imports semver or another compiler formatter for this support.

Owned descriptors carry the actual mutable environment cell, Call, complete
environment Drop witness and explicit ambient `rustfmt.Context`. Moving a
descriptor into an owned operation transfers ownership; callers must not use or
destroy it again. Borrow transfers only a trivial-drop mutable reference.
The Drop witness includes the environment's field destruction and its own
internal cleanup. Each library operation performs the source-ordered outer
cleanup. A callback owns its moved accumulator argument and its cleanup when a
fold Call panics. A final accumulator already placed for return leaks if the
callback environment destructor panics, matching the native default fold.

Context reports ambient unwinding or panic processing. Ambient unwinding alone
does not force abort: a caller inside an outer destructor can catch a new panic.
An escaping second panic during locally required cleanup is unrecoverable.
Go represents that Rust abort class by process exit 134; the native oracle uses
SIGABRT. Signal identity, panic payload type/text, Rust ABI and allocation
failure behavior are not claimed identical. `Context.Panic` aborts immediately
during panic processing, before Go cleanup can run. No global Go panic policy
or interceptor is installed.

Internal cursor failures use this context at their actual source sites:
debug assertions, u32 additions/subtractions, prefix unwrap and byte slicing.
Owned predicate/iterator operations inherit the callback and cursor context for
the whole exclusive borrow, including environment destruction, then restore
the stored context. A supplied callback context therefore also controls Next's
internal failures. Existing cursor/iterator state can enter a different ambient
state through per-operation `WithContext` forms; those use the explicitly
supplied current state for that operation and restore the stored default.
Constructors/standalone validators offer corresponding context forms. Default
APIs retain normal context and checked/debug settings.

The lazy iterator does not advertise Clone, Debug or FusedIterator. Count,
Last, ForEach and Fold consume source ownership even though Go cannot enforce
that move statically. The surrounding standard-library iterator adaptor
ecosystem is not counted as ported source. Source-owned closure environments
are supported where consumed; internal closed closures and the lexer's
function-pointer constructors do not acquire invented user destruction.

Position usize-to-u32 conversions deliberately truncate. Source u32 arithmetic
uses the selected checked or wrapping profile; debug assertions are independent.
Small-input original cursor/iterator tests cover each configuration. Boundary
tests execute exact native u32 expressions and the corresponding Go helpers;
they do not claim to allocate or tokenize a string larger than 4GiB. A native
from_fn Next-panic probe checks default fold cleanup; the Go test injects that
failure through a private nil cursor only as a fault injector, not as a lawful
Rust cursor state or an end-to-end large-input test.
The internal-context probe separately executes real native UTF-8 boundary,
assertion and unwrap failures. Exact native u32 expressions run inside native
from_fn operations; Go uses a private position bias to reach the actual checked
source arithmetic site inside Next with bounded text. This tests inherited
callback context and abort timing before cleanup without claiming an allocated
large-text comparison.

The existing eager `Tokenize`, default Debug strings, Unicode tables, literal
algorithms, source tests and complete scalar fixtures remain compatible.
These supporting contracts grant no additional std source-file completion
credit, general macro translation credit or Rust ABI claim.
