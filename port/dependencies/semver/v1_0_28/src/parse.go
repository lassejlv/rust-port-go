// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: semver 1.0.28 src/parse.rs. This is the upstream byte parser;
// requirements trim ASCII spaces only, and numeric components are exact u64.
package semver

import "strings"

type Error struct{ kind errorKind }

func parseError(kind errorTag, pos parsePosition, ch rune) *Error {
	return &Error{errorKind{tag: kind, pos: pos, ch: ch}}
}
func firstRune(text string) rune {
	for _, ch := range text {
		return ch
	}
	panic("semver: expected nonempty parser remainder")
}

func ParseVersion(text string) (Version, error) {
	requireUTF8(text)
	if text == "" {
		return Version{}, parseError(errorEmpty, positionMajor, 0)
	}
	pos := positionMajor
	major, text, err := numericIdentifier(text, pos)
	if err != nil {
		return Version{}, err
	}
	text, err = dot(text, pos)
	if err != nil {
		return Version{}, err
	}
	pos = positionMinor
	minor, text, err := numericIdentifier(text, pos)
	if err != nil {
		return Version{}, err
	}
	text, err = dot(text, pos)
	if err != nil {
		return Version{}, err
	}
	pos = positionPatch
	patch, text, err := numericIdentifier(text, pos)
	if err != nil {
		return Version{}, err
	}
	if text == "" {
		return NewVersion(major, minor, patch), nil
	}
	var pre Prerelease
	if strings.HasPrefix(text, "-") {
		pos = positionPre
		pre, text, err = prereleaseIdentifier(text[1:])
		if err != nil {
			return Version{}, err
		}
		if pre.IsEmpty() {
			return Version{}, parseError(errorEmptySegment, pos, 0)
		}
	}
	var build BuildMetadata
	if strings.HasPrefix(text, "+") {
		pos = positionBuild
		build, text, err = buildIdentifier(text[1:])
		if err != nil {
			return Version{}, err
		}
		if build.IsEmpty() {
			return Version{}, parseError(errorEmptySegment, pos, 0)
		}
	}
	if text != "" {
		return Version{}, parseError(errorUnexpectedAfter, pos, firstRune(text))
	}
	return Version{Major: major, Minor: minor, Patch: patch, Pre: pre, Build: build}, nil
}
func ParseVersionReq(text string) (VersionReq, error) {
	requireUTF8(text)
	text = strings.TrimLeft(text, " ")
	if wildcard, rest, ok := parseWildcard(text); ok {
		rest = strings.TrimLeft(rest, " ")
		if rest == "" {
			return Star(), nil
		}
		if strings.HasPrefix(rest, ",") {
			return VersionReq{}, parseError(errorWildcardNotOnly, positionMajor, wildcard)
		}
		return VersionReq{}, parseError(errorUnexpectedAfterWildcard, positionMajor, 0)
	}
	comparators, err := versionReq(text, 0)
	if err != nil {
		return VersionReq{}, err
	}
	return VersionReq{Comparators: comparators}, nil
}
func ParseComparator(text string) (Comparator, error) {
	requireUTF8(text)
	text = strings.TrimLeft(text, " ")
	comparator, pos, rest, err := parseComparator(text)
	if err != nil {
		return Comparator{}, err
	}
	if rest != "" {
		return Comparator{}, parseError(errorUnexpectedAfter, pos, firstRune(rest))
	}
	return comparator, nil
}
func NewPrerelease(text string) (Prerelease, error) {
	requireUTF8(text)
	pre, rest, err := prereleaseIdentifier(text)
	if err != nil {
		return Prerelease{}, err
	}
	if rest != "" {
		return Prerelease{}, parseError(errorIllegalCharacter, positionPre, 0)
	}
	return pre, nil
}
func NewBuildMetadata(text string) (BuildMetadata, error) {
	requireUTF8(text)
	build, rest, err := buildIdentifier(text)
	if err != nil {
		return BuildMetadata{}, err
	}
	if rest != "" {
		return BuildMetadata{}, parseError(errorIllegalCharacter, positionBuild, 0)
	}
	return build, nil
}
func numericIdentifier(input string, pos parsePosition) (uint64, string, error) {
	length := 0
	var value uint64
	for length < len(input) {
		digit := input[length]
		if digit < '0' || digit > '9' {
			break
		}
		if value == 0 && length > 0 {
			return 0, "", parseError(errorLeadingZero, pos, 0)
		}
		add := uint64(digit - '0')
		if value > (^uint64(0)-add)/10 {
			return 0, "", parseError(errorOverflow, pos, 0)
		}
		value = value*10 + add
		length++
	}
	if length > 0 {
		return value, input[length:], nil
	}
	if input != "" {
		return 0, "", parseError(errorUnexpected, pos, firstRune(input))
	}
	return 0, "", parseError(errorUnexpectedEnd, pos, 0)
}
func parseWildcard(input string) (rune, string, bool) {
	if input != "" && (input[0] == '*' || input[0] == 'x' || input[0] == 'X') {
		return rune(input[0]), input[1:], true
	}
	return 0, "", false
}
func dot(input string, pos parsePosition) (string, error) {
	if strings.HasPrefix(input, ".") {
		return input[1:], nil
	}
	if input != "" {
		return "", parseError(errorUnexpectedAfter, pos, firstRune(input))
	}
	return "", parseError(errorUnexpectedEnd, pos, 0)
}
func prereleaseIdentifier(input string) (Prerelease, string, error) {
	text, rest, err := parseIdentifier(input, positionPre)
	if err != nil {
		return Prerelease{}, "", err
	}
	return Prerelease{identifier: identifierUnchecked(text)}, rest, nil
}
func buildIdentifier(input string) (BuildMetadata, string, error) {
	text, rest, err := parseIdentifier(input, positionBuild)
	if err != nil {
		return BuildMetadata{}, "", err
	}
	return BuildMetadata{identifier: identifierUnchecked(text)}, rest, nil
}
func parseIdentifier(input string, pos parsePosition) (string, string, error) {
	accumulated, segment := 0, 0
	nondigit := false
	for {
		index := accumulated + segment
		if index < len(input) {
			ch := input[index]
			if ch >= 'A' && ch <= 'Z' || ch >= 'a' && ch <= 'z' || ch == '-' {
				segment++
				nondigit = true
				continue
			}
			if ch >= '0' && ch <= '9' {
				segment++
				continue
			}
		}
		boundaryDot := index < len(input) && input[index] == '.'
		if segment == 0 {
			if accumulated == 0 && !boundaryDot {
				return "", input, nil
			}
			return "", "", parseError(errorEmptySegment, pos, 0)
		}
		if pos == positionPre && segment > 1 && !nondigit && input[accumulated] == '0' {
			return "", "", parseError(errorLeadingZero, pos, 0)
		}
		accumulated += segment
		if boundaryDot {
			accumulated++
			segment = 0
			nondigit = false
		} else {
			return input[:accumulated], input[accumulated:], nil
		}
	}
}
func parseOp(input string) (Op, string) {
	if strings.HasPrefix(input, "=") {
		return Exact, input[1:]
	}
	if strings.HasPrefix(input, ">") {
		if strings.HasPrefix(input, ">=") {
			return GreaterEq, input[2:]
		}
		return Greater, input[1:]
	}
	if strings.HasPrefix(input, "<") {
		if strings.HasPrefix(input, "<=") {
			return LessEq, input[2:]
		}
		return Less, input[1:]
	}
	if strings.HasPrefix(input, "~") {
		return Tilde, input[1:]
	}
	if strings.HasPrefix(input, "^") {
		return Caret, input[1:]
	}
	return Caret, input
}
func parseComparator(input string) (Comparator, parsePosition, string, error) {
	op, text := parseOp(input)
	defaultOp := len(input) == len(text)
	text = strings.TrimLeft(text, " ")
	pos := positionMajor
	major, text, err := numericIdentifier(text, pos)
	if err != nil {
		return Comparator{}, pos, "", err
	}
	hasWildcard := false
	var minor, patch Option[uint64]
	if strings.HasPrefix(text, ".") {
		pos = positionMinor
		text = text[1:]
		if _, rest, ok := parseWildcard(text); ok {
			hasWildcard = true
			if defaultOp {
				op = Wildcard
			}
			text = rest
		} else {
			var number uint64
			number, text, err = numericIdentifier(text, pos)
			if err != nil {
				return Comparator{}, pos, "", err
			}
			minor = Some(number)
		}
	}
	if strings.HasPrefix(text, ".") {
		pos = positionPatch
		text = text[1:]
		if _, rest, ok := parseWildcard(text); ok {
			if defaultOp {
				op = Wildcard
			}
			text = rest
		} else if hasWildcard {
			return Comparator{}, pos, "", parseError(errorUnexpectedAfterWildcard, pos, 0)
		} else {
			var number uint64
			number, text, err = numericIdentifier(text, pos)
			if err != nil {
				return Comparator{}, pos, "", err
			}
			patch = Some(number)
		}
	}
	var pre Prerelease
	if patch.Present && strings.HasPrefix(text, "-") {
		pos = positionPre
		pre, text, err = prereleaseIdentifier(text[1:])
		if err != nil {
			return Comparator{}, pos, "", err
		}
		if pre.IsEmpty() {
			return Comparator{}, pos, "", parseError(errorEmptySegment, pos, 0)
		}
	}
	if patch.Present && strings.HasPrefix(text, "+") {
		pos = positionBuild
		var build BuildMetadata
		build, text, err = buildIdentifier(text[1:])
		if err != nil {
			return Comparator{}, pos, "", err
		}
		if build.IsEmpty() {
			return Comparator{}, pos, "", parseError(errorEmptySegment, pos, 0)
		}
	}
	text = strings.TrimLeft(text, " ")
	return Comparator{Op: op, Major: major, Minor: minor, Patch: patch, Pre: pre}, pos, text, nil
}
func versionReq(input string, depth int) ([]Comparator, error) {
	comparator, pos, rest, err := parseComparator(input)
	if err != nil {
		if wildcard, tail, ok := parseWildcard(input); ok {
			tail = strings.TrimLeft(tail, " ")
			if tail == "" || strings.HasPrefix(tail, ",") {
				err = parseError(errorWildcardNotOnly, positionMajor, wildcard)
			}
		}
		return nil, err
	}
	if rest == "" {
		out := make([]Comparator, depth+1)
		out[depth] = comparator
		return out, nil
	}
	if !strings.HasPrefix(rest, ",") {
		return nil, parseError(errorExpectedComma, pos, firstRune(rest))
	}
	rest = strings.TrimLeft(rest[1:], " ")
	if depth+1 == 32 {
		return nil, parseError(errorExcessiveComparators, positionMajor, 0)
	}
	out, err := versionReq(rest, depth+1)
	if err != nil {
		return nil, err
	}
	out[depth] = comparator
	return out, nil
}
