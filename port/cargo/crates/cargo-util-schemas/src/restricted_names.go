// SPDX-License-Identifier: MIT OR Apache-2.0
// Complete port of crates/cargo-util-schemas/src/restricted_names.rs at
// rust-lang/cargo 24de4721844c7a4d839072bab6fb66d41c9bc1f4.
// See port/cargo/LICENSE-MIT and LICENSE-APACHE.

// Package cargoutilschemas provides Cargo schema name validation. Strings must
// be valid UTF-8 and sanitizer placeholders must be Unicode scalars, as required
// by the original Rust &str/char APIs. Violating these preconditions panics.
package cargoutilschemas

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/lassejlv/rust-port-go/port/cargo/src/util"
	rustclexer "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_lexer/src"
)

// NameValidationError is Cargo's transparent validation error. Error implements
// Display; RustDebug mirrors its derived Debug representation. Source delegates
// to the private inner error's source, which is always nil. It deliberately has
// no Unwrap method: Rust's transparent Error does not expose the inner error as
// an additional source-chain link.
type NameValidationError struct{ kind errorKind }

type errorTag uint8

const (
	emptyName errorTag = iota
	invalidCharacter
	profileReserved
	featureDepColon
)

type errorKind struct {
	tag                      errorTag
	ch                       rune
	what, name, reason, help string
}

func (e *NameValidationError) Error() string { return e.kind.Error() }
func (e *NameValidationError) Source() error { return e.kind.Source() }
func (e *NameValidationError) RustDebug() string {
	return "NameValidationError(" + e.kind.RustDebug() + ")"
}
func (e errorKind) Source() error { return nil }
func (e errorKind) Error() string {
	switch e.tag {
	case emptyName:
		return e.what + " cannot be empty"
	case invalidCharacter:
		return fmt.Sprintf("invalid character `%c` in %s: `%s`, %s", e.ch, e.what, e.name, e.reason)
	case profileReserved:
		return fmt.Sprintf("profile name `%s` is reserved\n%s\nSee https://doc.rust-lang.org/cargo/reference/profiles.html for more on configuring profiles.", e.name, e.help)
	case featureDepColon:
		return fmt.Sprintf("feature named `%s` is not allowed to start with `dep:`", e.name)
	default:
		panic("restricted_names: invalid private error kind")
	}
}
func (e errorKind) RustDebug() string {
	switch e.tag {
	case emptyName:
		return "Empty(" + nameDebugString(e.what) + ")"
	case invalidCharacter:
		return fmt.Sprintf("InvalidCharacter { ch: %s, what: %s, name: %s, reason: %s }", nameDebugChar(e.ch), nameDebugString(e.what), nameDebugString(e.name), nameDebugString(e.reason))
	case profileReserved:
		return fmt.Sprintf("ProfileNameReservedKeyword { name: %s, help: %s }", nameDebugString(e.name), nameDebugString(e.help))
	case featureDepColon:
		return "FeatureNameStartsWithDepColon(" + nameDebugString(e.name) + ")"
	default:
		panic("restricted_names: invalid private error kind")
	}
}

func nameUTF8(name string) {
	if !utf8.ValidString(name) {
		panic("restricted_names: input is not valid UTF-8")
	}
}
func nameDigit(ch rune) bool { return ch >= '0' && ch <= '9' }
func nameInvalid(ch rune, what, name, reason string) error {
	return &NameValidationError{errorKind{tag: invalidCharacter, ch: ch, what: what, name: name, reason: reason}}
}

// ValidatePackageName validates each ::-separated package-name component.
func ValidatePackageName(name string) error {
	nameUTF8(name)
	for _, part := range strings.Split(name, "::") {
		if err := ValidateName(part, "package name"); err != nil {
			return err
		}
	}
	return nil
}

func ValidateRegistryName(name string) error { return ValidateName(name, "registry name") }

// ValidateName preserves Cargo's first-invalid-character ordering. The initial
// digit-specific diagnostic applies to ASCII digits, like Rust is_digit(10).
func ValidateName(name, what string) error {
	nameUTF8(name)
	nameUTF8(what)
	if name == "" {
		return &NameValidationError{errorKind{tag: emptyName, what: what}}
	}
	first := true
	for _, ch := range name {
		if first {
			first = false
			if nameDigit(ch) {
				return nameInvalid(ch, what, name, "the name cannot start with a digit")
			}
			if !rustclexer.IsIDStart(ch) {
				return nameInvalid(ch, what, name, "the first character must be a Unicode XID start character (most letters or `_`)")
			}
		} else if !(rustclexer.IsIDContinue(ch) || ch == '-') {
			return nameInvalid(ch, what, name, "characters must be Unicode XID characters (numbers, `-`, `_`, or most letters)")
		}
	}
	return nil
}

// SanitizePackageName applies SanitizeName to every ::-separated component.
// Like the source, arbitrary scalar placeholders may produce an invalid name.
func SanitizePackageName(name string, placeholder rune) string {
	nameUTF8(name)
	namePlaceholder(placeholder)
	var slug strings.Builder
	for _, part := range strings.Split(name, "::") {
		if slug.Len() != 0 {
			slug.WriteString("::")
		}
		slug.WriteString(SanitizeName(part, placeholder))
	}
	return slug.String()
}

func namePlaceholder(placeholder rune) {
	if !utf8.ValidRune(placeholder) {
		panic("restricted_names: placeholder is not a Unicode scalar")
	}
}

// SanitizeName skips the invalid prefix, then replaces invalid continuation
// characters. A name with no valid initial character becomes "package".
func SanitizeName(name string, placeholder rune) string {
	nameUTF8(name)
	namePlaceholder(placeholder)
	var slug strings.Builder
	rest := ""
	for index, ch := range name {
		if rustclexer.IsIDStart(ch) && !nameDigit(ch) {
			slug.WriteRune(ch)
			rest = name[index+utf8.RuneLen(ch):]
			break
		}
	}
	for _, ch := range rest {
		if rustclexer.IsIDContinue(ch) || ch == '-' {
			slug.WriteRune(ch)
		} else {
			slug.WriteRune(placeholder)
		}
	}
	if slug.Len() == 0 {
		slug.WriteString("package")
	}
	return slug.String()
}

// ValidateProfileName permits the empty name, as the original source does.
func ValidateProfileName(name string) error {
	nameUTF8(name)
	for _, ch := range name {
		if !nameInRanges(ch, nameAlphanumericRanges) && ch != '_' && ch != '-' {
			return nameInvalid(ch, "profile name", name, "allowed characters are letters, numbers, underscore, and hyphen")
		}
	}
	lowerName := util.Lowercase17(name)
	if lowerName == "build-override" {
		return &NameValidationError{errorKind{tag: profileReserved, name: name, help: "To configure build dependency settings, use [profile.dev.build-override] and [profile.release.build-override]"}}
	}
	reserved := false
	switch lowerName {
	case "build", "check", "clean", "config", "fetch", "fix", "install", "metadata", "package", "publish", "report", "root", "run", "rust", "rustc", "rustdoc", "target", "tmp", "uninstall":
		reserved = true
	}
	if reserved || strings.HasPrefix(lowerName, "cargo") {
		return &NameValidationError{errorKind{tag: profileReserved, name: name, help: "Please choose a different name."}}
	}
	return nil
}

func ValidateFeatureName(name string) error {
	nameUTF8(name)
	const what = "feature name"
	if name == "" {
		return &NameValidationError{errorKind{tag: emptyName, what: what}}
	}
	if strings.HasPrefix(name, "dep:") {
		return &NameValidationError{errorKind{tag: featureDepColon, name: name}}
	}
	if strings.Contains(name, "/") {
		return nameInvalid('/', what, name, "feature name is not allowed to contain slashes")
	}
	first := true
	for _, ch := range name {
		if first {
			first = false
			if !(rustclexer.IsIDStart(ch) || nameDigit(ch)) {
				return nameInvalid(ch, what, name, "the first character must be a Unicode XID start character or digit (most letters or `_` or `0` to `9`)")
			}
		} else if !(rustclexer.IsIDContinue(ch) || ch == '-' || ch == '+' || ch == '.') {
			return nameInvalid(ch, what, name, "characters must be Unicode XID characters, '-', `+`, or `.` (numbers, `+`, `-`, `_`, `.`, or most letters)")
		}
	}
	return nil
}

func ValidatePathBaseName(name string) error { return ValidateName(name, "path base name") }

func nameInRanges(ch rune, ranges [][2]rune) bool {
	lo, hi := 0, len(ranges)
	for lo < hi {
		mid := lo + (hi-lo)/2
		if ranges[mid][1] < ch {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo < len(ranges) && ranges[lo][0] <= ch
}

func nameEscape(ch rune, ranges [][2]rune) string {
	switch ch {
	case '\x00':
		return `\0`
	case '\t':
		return `\t`
	case '\n':
		return `\n`
	case '\r':
		return `\r`
	case '\\':
		return `\\`
	}
	if nameInRanges(ch, ranges) {
		return fmt.Sprintf(`\u{%x}`, ch)
	}
	return string(ch)
}
func nameDebugChar(ch rune) string {
	if ch == '\'' {
		return `'\''`
	}
	return "'" + nameEscape(ch, nameEscapedRanges) + "'"
}
func nameDebugString(value string) string {
	var result strings.Builder
	result.WriteByte('"')
	first := true
	for _, ch := range value {
		if ch == '"' {
			result.WriteString(`\"`)
		} else if first {
			result.WriteString(nameEscape(ch, nameEscapedRanges))
		} else {
			result.WriteString(nameEscape(ch, nameNonInitialEscapedRanges))
		}
		first = false
	}
	result.WriteByte('"')
	return result.String()
}
