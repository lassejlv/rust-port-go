package util

// Lowercase17 implements Rust 1.99.1 str::to_lowercase with its Unicode 17.0.0
// tables, expansions, and contextual final sigma. Input must be valid UTF-8;
// invalid input panics, matching the Rust &str precondition. This compatibility
// helper shares the validated data used by the edit_distance source port.
func Lowercase17(value string) string { return string(editDistanceLower(value)) }
