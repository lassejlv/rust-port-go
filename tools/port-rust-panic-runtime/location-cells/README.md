# Borrowed panic-location cells

This bounded reference uses the actual selected SDK1.101 compiler and safe
public panic hooks. It compares repeated and different compiled static
Location references after converting them to integers inside the Rust process.
Only pairwise equality and location fields are retained; numeric addresses
never cross the language boundary.

The 107 scripts contain 1,283 hooks and 20,635 pairwise comparisons. Native
results supply every expected identity bit and the exact relative wrapper
filename/line/column. The Go replay supplies caller-owned immutable primary
`PanicOperations.LocationCells`, observes those actual pointers in its hooks,
and retains exact original panic payloads. Separate units cover authoritative
nil entries, cell precedence, value-copy conveniences and honestly unlocated
defaults.

```sh
python3 tools/port-rust-panic-runtime/location-cells/generate.py --check
go test ./port/support/rustfmt -run '^TestPanicLocationCells'
```

Compiler overrides must match the exact official SDK commit; child compiles
and runs have finite timeouts. No cache invents or interns location cells.
The provider owns their lifetime and shared-borrow immutability. Existing
value-profile metadata stays a convenience with independent scalar copies.
No standard-library source-file or Rust/Go pointer ABI credit is claimed.
