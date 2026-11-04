// Code generated from unchanged semver1.0.28 tests by tools/port-semver/original-tests/mirror.py.

package semver_test

import (
	"testing"
)

func TestVersionReq_test_basic(t *testing.T) {
	r := req(t, "1.0.0")
	assert_to_string(t, r, "^1.0.0")
	assert_match_all(t, r, []string{"1.0.0", "1.1.0", "1.0.1"})
	assert_match_none(t, r, []string{"0.9.9", "0.10.0", "0.1.0", "1.0.0-pre", "1.0.1-pre"})
}

func TestVersionReq_test_default(t *testing.T) {
	r := starReq()
	assert_eq(t, r, starReq())
}

func TestVersionReq_test_exact(t *testing.T) {
	r := req(t, "=1.0.0")
	assert_to_string(t, r, "=1.0.0")
	assert_match_all(t, r, []string{"1.0.0"})
	assert_match_none(t, r, []string{"1.0.1", "0.9.9", "0.10.0", "0.1.0", "1.0.0-pre"})

	r = req(t, "=0.9.0")
	assert_to_string(t, r, "=0.9.0")
	assert_match_all(t, r, []string{"0.9.0"})
	assert_match_none(t, r, []string{"0.9.1", "1.9.0", "0.0.9", "0.9.0-pre"})

	r = req(t, "=0.0.2")
	assert_to_string(t, r, "=0.0.2")
	assert_match_all(t, r, []string{"0.0.2"})
	assert_match_none(t, r, []string{"0.0.1", "0.0.3", "0.0.2-pre"})

	r = req(t, "=0.1.0-beta2.a")
	assert_to_string(t, r, "=0.1.0-beta2.a")
	assert_match_all(t, r, []string{"0.1.0-beta2.a"})
	assert_match_none(t, r, []string{"0.9.1", "0.1.0", "0.1.1-beta2.a", "0.1.0-beta2"})

	r = req(t, "=0.1.0+meta")
	assert_to_string(t, r, "=0.1.0")
	assert_match_all(t, r, []string{"0.1.0", "0.1.0+meta", "0.1.0+any"})
}

func TestVersionReq_test_greater_than(t *testing.T) {
	r := req(t, ">= 1.0.0")
	assert_to_string(t, r, ">=1.0.0")
	assert_match_all(t, r, []string{"1.0.0", "2.0.0"})
	assert_match_none(t, r, []string{"0.1.0", "0.0.1", "1.0.0-pre", "2.0.0-pre"})

	r = req(t, ">= 2.1.0-alpha2")
	assert_to_string(t, r, ">=2.1.0-alpha2")
	assert_match_all(t, r, []string{"2.1.0-alpha2", "2.1.0-alpha3", "2.1.0", "3.0.0"})
	assert_match_none(t,
		r,
		[]string{"2.0.0", "2.1.0-alpha1", "2.0.0-alpha2", "3.0.0-alpha2"},
	)
}

func TestVersionReq_test_less_than(t *testing.T) {
	r := req(t, "< 1.0.0")
	assert_to_string(t, r, "<1.0.0")
	assert_match_all(t, r, []string{"0.1.0", "0.0.1"})
	assert_match_none(t, r, []string{"1.0.0", "1.0.0-beta", "1.0.1", "0.9.9-alpha"})

	r = req(t, "<= 2.1.0-alpha2")
	assert_match_all(t, r, []string{"2.1.0-alpha2", "2.1.0-alpha1", "2.0.0", "1.0.0"})
	assert_match_none(t,
		r,
		[]string{"2.1.0", "2.2.0-alpha1", "2.0.0-alpha2", "1.0.0-alpha2"},
	)

	r = req(t, ">1.0.0-alpha, <1.0.0")
	assert_match_all(t, r, []string{"1.0.0-beta"})

	r = req(t, ">1.0.0-alpha, <1.0")
	assert_match_none(t, r, []string{"1.0.0-beta"})

	r = req(t, ">1.0.0-alpha, <1")
	assert_match_none(t, r, []string{"1.0.0-beta"})
}

func TestVersionReq_test_multiple(t *testing.T) {
	r := req(t, "> 0.0.9, <= 2.5.3")
	assert_to_string(t, r, ">0.0.9, <=2.5.3")
	assert_match_all(t, r, []string{"0.0.10", "1.0.0", "2.5.3"})
	assert_match_none(t, r, []string{"0.0.8", "2.5.4"})

	r = req(t, "0.3.0, 0.4.0")
	assert_to_string(t, r, "^0.3.0, ^0.4.0")
	assert_match_none(t, r, []string{"0.0.8", "0.3.0", "0.4.0"})

	r = req(t, "<= 0.2.0, >= 0.5.0")
	assert_to_string(t, r, "<=0.2.0, >=0.5.0")
	assert_match_none(t, r, []string{"0.0.8", "0.3.0", "0.5.1"})

	r = req(t, "0.1.0, 0.1.4, 0.1.6")
	assert_to_string(t, r, "^0.1.0, ^0.1.4, ^0.1.6")
	assert_match_all(t, r, []string{"0.1.6", "0.1.9"})
	assert_match_none(t, r, []string{"0.1.0", "0.1.4", "0.2.0"})

	err := req_err(t, "> 0.1.0,")
	assert_to_string(t,
		err,
		"unexpected end of input while parsing major version number",
	)

	err = req_err(t, "> 0.3.0, ,")
	assert_to_string(t,
		err,
		"unexpected character ',' while parsing major version number",
	)

	r = req(t, ">=0.5.1-alpha3, <0.6")
	assert_to_string(t, r, ">=0.5.1-alpha3, <0.6")
	assert_match_all(t,
		r,
		[]string{
			"0.5.1-alpha3",
			"0.5.1-alpha4",
			"0.5.1-beta",
			"0.5.1",
			"0.5.5",
		},
	)
	assert_match_none(t,
		r,
		[]string{"0.5.1-alpha1", "0.5.2-alpha3", "0.5.5-pre", "0.5.0-pre"},
	)
	assert_match_none(t, r, []string{"0.6.0", "0.6.0-pre"})

	err = req_err(t, "1.2.3 - 2.3.4")
	assert_to_string(t, err, "expected comma after patch version number, found '-'")

	err = req_err(t, ">1, >2, >3, >4, >5, >6, >7, >8, >9, >10, >11, >12, >13, >14, >15, >16, >17, >18, >19, >20, >21, >22, >23, >24, >25, >26, >27, >28, >29, >30, >31, >32, >33")
	assert_to_string(t, err, "excessive number of version comparators")
}

func TestVersionReq_test_whitespace_delimited_comparator_sets(t *testing.T) {

	err := req_err(t, "> 0.0.9 <= 2.5.3")
	assert_to_string(t, err, "expected comma after patch version number, found '<'")
}

func TestVersionReq_test_tilde(t *testing.T) {
	r := req(t, "~1")
	assert_match_all(t, r, []string{"1.0.0", "1.0.1", "1.1.1"})
	assert_match_none(t, r, []string{"0.9.1", "2.9.0", "0.0.9"})

	r = req(t, "~1.2")
	assert_match_all(t, r, []string{"1.2.0", "1.2.1"})
	assert_match_none(t, r, []string{"1.1.1", "1.3.0", "0.0.9"})

	r = req(t, "~1.2.2")
	assert_match_all(t, r, []string{"1.2.2", "1.2.4"})
	assert_match_none(t, r, []string{"1.2.1", "1.9.0", "1.0.9", "2.0.1", "0.1.3"})

	r = req(t, "~1.2.3-beta.2")
	assert_match_all(t, r, []string{"1.2.3", "1.2.4", "1.2.3-beta.2", "1.2.3-beta.4"})
	assert_match_none(t, r, []string{"1.3.3", "1.1.4", "1.2.3-beta.1", "1.2.4-beta.2"})
}

func TestVersionReq_test_caret(t *testing.T) {
	r := req(t, "^1")
	assert_match_all(t, r, []string{"1.1.2", "1.1.0", "1.2.1", "1.0.1"})
	assert_match_none(t, r, []string{"0.9.1", "2.9.0", "0.1.4"})
	assert_match_none(t, r, []string{"1.0.0-beta1", "0.1.0-alpha", "1.0.1-pre"})

	r = req(t, "^1.1")
	assert_match_all(t, r, []string{"1.1.2", "1.1.0", "1.2.1"})
	assert_match_none(t, r, []string{"0.9.1", "2.9.0", "1.0.1", "0.1.4"})

	r = req(t, "^1.1.2")
	assert_match_all(t, r, []string{"1.1.2", "1.1.4", "1.2.1"})
	assert_match_none(t, r, []string{"0.9.1", "2.9.0", "1.1.1", "0.0.1"})
	assert_match_none(t, r, []string{"1.1.2-alpha1", "1.1.3-alpha1", "2.9.0-alpha1"})

	r = req(t, "^0.1.2")
	assert_match_all(t, r, []string{"0.1.2", "0.1.4"})
	assert_match_none(t, r, []string{"0.9.1", "2.9.0", "1.1.1", "0.0.1"})
	assert_match_none(t, r, []string{"0.1.2-beta", "0.1.3-alpha", "0.2.0-pre"})

	r = req(t, "^0.5.1-alpha3")
	assert_match_all(t,
		r,
		[]string{
			"0.5.1-alpha3",
			"0.5.1-alpha4",
			"0.5.1-beta",
			"0.5.1",
			"0.5.5",
		},
	)
	assert_match_none(t,
		r,
		[]string{
			"0.5.1-alpha1",
			"0.5.2-alpha3",
			"0.5.5-pre",
			"0.5.0-pre",
			"0.6.0",
		},
	)

	r = req(t, "^0.0.2")
	assert_match_all(t, r, []string{"0.0.2"})
	assert_match_none(t, r, []string{"0.9.1", "2.9.0", "1.1.1", "0.0.1", "0.1.4"})

	r = req(t, "^0.0")
	assert_match_all(t, r, []string{"0.0.2", "0.0.0"})
	assert_match_none(t, r, []string{"0.9.1", "2.9.0", "1.1.1", "0.1.4"})

	r = req(t, "^0")
	assert_match_all(t, r, []string{"0.9.1", "0.0.2", "0.0.0"})
	assert_match_none(t, r, []string{"2.9.0", "1.1.1"})

	r = req(t, "^1.4.2-beta.5")
	assert_match_all(t,
		r,
		[]string{"1.4.2", "1.4.3", "1.4.2-beta.5", "1.4.2-beta.6", "1.4.2-c"},
	)
	assert_match_none(t,
		r,
		[]string{
			"0.9.9",
			"2.0.0",
			"1.4.2-alpha",
			"1.4.2-beta.4",
			"1.4.3-beta.5",
		},
	)
}

func TestVersionReq_test_wildcard(t *testing.T) {
	err := req_err(t, "")
	assert_to_string(t,
		err,
		"unexpected end of input while parsing major version number",
	)

	r := req(t, "*")
	assert_match_all(t, r, []string{"0.9.1", "2.9.0", "0.0.9", "1.0.1", "1.1.1"})
	assert_match_none(t, r, []string{"1.0.0-pre"})

	for _, s := range []string{"x", "X"} {
		assert_eq(t, r, req(t, s))
	}

	r = req(t, "1.*")
	assert_match_all(t, r, []string{"1.2.0", "1.2.1", "1.1.1", "1.3.0"})
	assert_match_none(t, r, []string{"0.0.9", "1.2.0-pre"})

	for _, s := range []string{"1.x", "1.X", "1.*.*"} {
		assert_eq(t, r, req(t, s))
	}

	r = req(t, "1.2.*")
	assert_match_all(t, r, []string{"1.2.0", "1.2.2", "1.2.4"})
	assert_match_none(t, r, []string{"1.9.0", "1.0.9", "2.0.1", "0.1.3", "1.2.2-pre"})

	for _, s := range []string{"1.2.x", "1.2.X"} {
		assert_eq(t, r, req(t, s))
	}
}

func TestVersionReq_test_logical_or(t *testing.T) {

	err := req_err(t, "=1.2.3 || =2.3.4")
	assert_to_string(t, err, "expected comma after patch version number, found '|'")

	err = req_err(t, "1.1 || =1.2.3")
	assert_to_string(t, err, "expected comma after minor version number, found '|'")

	err = req_err(t, "6.* || 8.* || >= 10.*")
	assert_to_string(t, err, "expected comma after minor version number, found '|'")
}

func TestVersionReq_test_any(t *testing.T) {
	r := starReq()
	assert_match_all(t, r, []string{"0.0.1", "0.1.0", "1.0.0"})
}

func TestVersionReq_test_pre(t *testing.T) {
	r := req(t, "=2.1.1-really.0")
	assert_match_all(t, r, []string{"2.1.1-really.0"})
}

func TestVersionReq_test_parse(t *testing.T) {
	err := req_err(t, "\u0000")
	assert_to_string(t,
		err,
		"unexpected character '\\0' while parsing major version number",
	)

	err = req_err(t, ">= >= 0.0.2")
	assert_to_string(t,
		err,
		"unexpected character '>' while parsing major version number",
	)

	err = req_err(t, ">== 0.0.2")
	assert_to_string(t,
		err,
		"unexpected character '=' while parsing major version number",
	)

	err = req_err(t, "a.0.0")
	assert_to_string(t,
		err,
		"unexpected character 'a' while parsing major version number",
	)

	err = req_err(t, "1.0.0-")
	assert_to_string(t, err, "empty identifier segment in pre-release identifier")

	err = req_err(t, ">=")
	assert_to_string(t,
		err,
		"unexpected end of input while parsing major version number",
	)
}

func TestVersionReq_test_comparator_parse(t *testing.T) {
	parsed := comparator(t, "1.2.3-alpha")
	assert_to_string(t, parsed, "^1.2.3-alpha")

	parsed = comparator(t, "2.X")
	assert_to_string(t, parsed, "2.*")

	parsed = comparator(t, "2")
	assert_to_string(t, parsed, "^2")

	parsed = comparator(t, "2.x.x")
	assert_to_string(t, parsed, "2.*")

	err := comparator_err(t, "1.2.3-01")
	assert_to_string(t, err, "invalid leading zero in pre-release identifier")

	err = comparator_err(t, "1.2.3+4.")
	assert_to_string(t, err, "empty identifier segment in build metadata")

	err = comparator_err(t, ">")
	assert_to_string(t,
		err,
		"unexpected end of input while parsing major version number",
	)

	err = comparator_err(t, "1.")
	assert_to_string(t,
		err,
		"unexpected end of input while parsing minor version number",
	)

	err = comparator_err(t, "1.*.")
	assert_to_string(t, err, "unexpected character after wildcard in version req")

	err = comparator_err(t, "1.2.3+4ÿ")
	assert_to_string(t, err, "unexpected character 'ÿ' after build metadata")
}

func TestVersionReq_test_cargo3202(t *testing.T) {
	r := req(t, "0.*.*")
	assert_to_string(t, r, "0.*")
	assert_match_all(t, r, []string{"0.5.0"})

	r = req(t, "0.0.*")
	assert_to_string(t, r, "0.0.*")
}

func TestVersionReq_test_digit_after_wildcard(t *testing.T) {
	err := req_err(t, "*.1")
	assert_to_string(t, err, "unexpected character after wildcard in version req")

	err = req_err(t, "1.*.1")
	assert_to_string(t, err, "unexpected character after wildcard in version req")

	err = req_err(t, ">=1.*.1")
	assert_to_string(t, err, "unexpected character after wildcard in version req")
}

func TestVersionReq_test_eq_hash(t *testing.T) {

	assert_true(t, rust_eq(req(t, "^1"), req(t, "^1")))
	assert_true(t, rust_eq(calculate_hash(req(t, "^1")), calculate_hash(req(t, "^1"))))
	assert_true(t, !rust_eq(req(t, "^1"), req(t, "^2")))
}

func TestVersionReq_test_leading_digit_in_pre_and_build(t *testing.T) {
	for _, op := range []string{"=", ">", ">=", "<", "<=", "~", "^"} {

		req(t, rust_format("{} 1.2.3-1a", op))
		req(t, rust_format("{} 1.2.3+1a", op))

		req(t, rust_format("{} 1.2.3-01a", op))
		req(t, rust_format("{} 1.2.3+01", op))

		req(t, rust_format("{} 1.2.3-1+1", op))
		req(t, rust_format("{} 1.2.3-1-1+1-1-1", op))
		req(t, rust_format("{} 1.2.3-1a+1a", op))
		req(t, rust_format("{} 1.2.3-1a-1a+1a-1a-1a", op))
	}
}

func TestVersionReq_test_wildcard_and_another(t *testing.T) {
	err := req_err(t, "*, 0.20.0-any")
	assert_to_string(t,
		err,
		"wildcard req (*) must be the only comparator in the version req",
	)

	err = req_err(t, "0.20.0-any, *")
	assert_to_string(t,
		err,
		"wildcard req (*) must be the only comparator in the version req",
	)

	err = req_err(t, "0.20.0-any, *, 1.0")
	assert_to_string(t,
		err,
		"wildcard req (*) must be the only comparator in the version req",
	)
}
