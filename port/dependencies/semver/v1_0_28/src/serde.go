//go:build semver_serde

// SPDX-License-Identifier: MIT OR Apache-2.0
// Source: semver 1.0.28 src/serde.rs. The generic serializer/deserializer calls
// are exposed as Go protocol adapters; JSON/Text are convenience frontends.
package semver

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode/utf8"
)

const FeatureSerde = true

// Serializer is the one Serializer operation that the source implementations
// invoke. Its result and error remain caller-defined, as Rust's associated types.
type Serializer interface{ CollectString(Display) (any, error) }
type SerializerAdapter struct {
	SerializeString       func(string) (any, error)
	CollectStringOverride func(Display) (any, error)
}

func (adapter SerializerAdapter) CollectString(value Display) (any, error) {
	if adapter.CollectStringOverride != nil {
		return adapter.CollectStringOverride(value)
	}
	if adapter.SerializeString == nil {
		panic("semver: missing Serializer::serialize_str adapter")
	}
	return adapter.SerializeString(value.String())
}
func (value Version) Serialize(serializer Serializer) (any, error) {
	return serializer.CollectString(value)
}
func (value VersionReq) Serialize(serializer Serializer) (any, error) {
	return serializer.CollectString(value)
}
func (value Comparator) Serialize(serializer Serializer) (any, error) {
	return serializer.CollectString(value)
}

// SerdeExpected maps serde::de::Expected, including fallible expecting output.
type SerdeExpected interface {
	Expecting() string
	WriteExpecting(io.Writer) error
}

// Deserializer supplies deserialize_str and its associated Error::custom.
// Implement InvalidType(Unexpected,SerdeExpected) error to override Error's
// invalid_type default independently from custom, as permitted by serde.
type SerdeErrorFactory interface{ CustomError(Display) error }
type Deserializer interface {
	SerdeErrorFactory
	DeserializeStr(SerdeVisitor) (any, error)
}
type DeserializerAdapter struct {
	DeserializeString   func(SerdeVisitor) (any, error)
	Custom              func(Display) error
	InvalidTypeOverride func(Unexpected, SerdeExpected) error
}

func (adapter DeserializerAdapter) DeserializeStr(visitor SerdeVisitor) (any, error) {
	if adapter.DeserializeString == nil {
		panic("semver: missing Deserializer::deserialize_str adapter")
	}
	return adapter.DeserializeString(visitor)
}
func (adapter DeserializerAdapter) CustomError(message Display) error {
	if adapter.Custom != nil {
		return adapter.Custom(message)
	}
	return &SerdeError{Message: message.String()}
}
func (adapter DeserializerAdapter) InvalidType(value Unexpected, expected SerdeExpected) error {
	if adapter.InvalidTypeOverride != nil {
		return adapter.InvalidTypeOverride(value, expected)
	}
	return adapter.CustomError(invalidTypeMessage{unexpected: value, expected: expected})
}

// SerdeInput is a tagged adapter for every default Visitor input used by a
// deserialize_str implementation. Borrowed/owned lifetimes map to immutable Go
// values. Some/newtype/sequence/map/enum data is deliberately never traversed:
// the source visitors inherit serde's immediate invalid_type defaults.
type SerdeInputKind uint8

const (
	SerdeString SerdeInputKind = iota
	SerdeBorrowedString
	SerdeOwnedString
	SerdeChar
	SerdeBool
	SerdeI8
	SerdeI16
	SerdeI32
	SerdeI64
	SerdeI128
	SerdeU8
	SerdeU16
	SerdeU32
	SerdeU64
	SerdeU128
	SerdeF32
	SerdeF64
	SerdeBytes
	SerdeBorrowedBytes
	SerdeByteBuf
	SerdeNone
	SerdeSome
	SerdeUnit
	SerdeNewtypeStruct
	SerdeSeq
	SerdeMap
	SerdeEnum
)

type SerdeInput struct {
	Kind     SerdeInputKind
	Text     string
	Char     rune
	Bool     bool
	Signed   int64
	Unsigned uint64
	Float    float64
	// Wide is the two's-complement [high,low] representation for i128, or the
	// unsigned [high,low] representation for u128. No precision is discarded.
	Wide  [2]uint64
	Bytes []byte
	Data  any
	// ErrorFactory chooses the generic visit method's Error associated type.
	// Nil uses the enclosing deserializer's Error. Some/newtype/access inputs
	// may supply their own Error factory without traversing the child value.
	ErrorFactory SerdeErrorFactory
}
type UnexpectedKind uint8

const (
	UnexpectedBool UnexpectedKind = iota
	UnexpectedSigned
	UnexpectedUnsigned
	UnexpectedFloat
	UnexpectedChar
	UnexpectedString
	UnexpectedBytes
	UnexpectedUnit
	UnexpectedOption
	UnexpectedNewtypeStruct
	UnexpectedSeq
	UnexpectedMap
	UnexpectedEnum
	UnexpectedOther
)

type Unexpected struct{ input SerdeInput }

func (unexpected Unexpected) Kind() UnexpectedKind {
	switch unexpected.input.Kind {
	case SerdeBool:
		return UnexpectedBool
	case SerdeI8, SerdeI16, SerdeI32, SerdeI64:
		return UnexpectedSigned
	case SerdeU8, SerdeU16, SerdeU32, SerdeU64:
		return UnexpectedUnsigned
	case SerdeF32, SerdeF64:
		return UnexpectedFloat
	case SerdeChar:
		return UnexpectedChar
	case SerdeString, SerdeBorrowedString, SerdeOwnedString:
		return UnexpectedString
	case SerdeBytes, SerdeBorrowedBytes, SerdeByteBuf:
		return UnexpectedBytes
	case SerdeUnit:
		return UnexpectedUnit
	case SerdeNone, SerdeSome:
		return UnexpectedOption
	case SerdeNewtypeStruct:
		return UnexpectedNewtypeStruct
	case SerdeSeq:
		return UnexpectedSeq
	case SerdeMap:
		return UnexpectedMap
	case SerdeEnum:
		return UnexpectedEnum
	case SerdeI128, SerdeU128:
		return UnexpectedOther
	default:
		panic("semver: invalid unexpected kind")
	}
}
func (unexpected Unexpected) Input() SerdeInput { return unexpected.input }
func wideDecimal(value [2]uint64, signed bool) string {
	number := new(big.Int).SetUint64(value[0])
	number.Lsh(number, 64)
	number.Or(number, new(big.Int).SetUint64(value[1]))
	if signed && value[0]>>63 != 0 {
		limit := new(big.Int).Lsh(big.NewInt(1), 128)
		number.Sub(number, limit)
	}
	return number.String()
}

// Rust's shortest-decimal formatter rounds exact decimal midpoints upward;
// strconv's shortest Ryu output breaks them to even. Both round-trip, but the
// caller-visible Display text must retain Rust's choice (e.g. an f32 widened
// to f64 may print 72.24185180664063 rather than 72.24185180664062).
func rustShortestFloat(value float64) (string, int) {
	scientific := strconv.FormatFloat(value, 'e', -1, 64)
	split := strings.IndexByte(scientific, 'e')
	digits := strings.ReplaceAll(scientific[:split], ".", "")
	exponent, err := strconv.Atoi(scientific[split+1:])
	if err != nil {
		panic(err)
	}
	if digits[len(digits)-1]%2 == 0 {
		significand, _ := new(big.Int).SetString(digits, 10)
		twice := new(big.Int).Lsh(new(big.Int).Set(significand), 1)
		twice.Add(twice, big.NewInt(1))
		midpoint := new(big.Rat).SetFrac(twice, big.NewInt(2))
		scale := exponent - len(digits) + 1
		power := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(absInt(scale))), nil)
		if scale >= 0 {
			midpoint.Mul(midpoint, new(big.Rat).SetInt(power))
		} else {
			midpoint.Quo(midpoint, new(big.Rat).SetInt(power))
		}
		if midpoint.Cmp(new(big.Rat).SetFloat64(value)) == 0 {
			significand.Add(significand, big.NewInt(1))
			digits = significand.String()
		}
	}
	return digits, exponent
}
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
func serdeFloat(value float64) string {
	if math.IsNaN(value) {
		return "NaN"
	}
	if math.IsInf(value, 1) {
		return "inf"
	}
	if math.IsInf(value, -1) {
		return "-inf"
	}
	sign := ""
	if math.Signbit(value) {
		sign = "-"
		value = -value
	}
	if value == 0 {
		return sign + "0.0"
	}
	digits, exponent := rustShortestFloat(value)
	decimal := exponent + 1
	if decimal <= 0 {
		return sign + "0." + strings.Repeat("0", -decimal) + digits
	}
	if decimal < len(digits) {
		return sign + digits[:decimal] + "." + digits[decimal:]
	}
	return sign + digits + strings.Repeat("0", decimal-len(digits)) + ".0"
}

// JSON's associated Error formatter chooses scientific notation outside
// zmij's [-5,15] decimal-exponent window. The generic serde Error default above
// deliberately uses Rust Display's always-positional floating-point spelling.
func jsonErrorFloat(value float64) string {
	if value == 0 {
		return serdeFloat(value)
	}
	text := strconv.FormatFloat(value, 'e', -1, 64)
	split := strings.LastIndexByte(text, 'e')
	exponent, err := strconv.Atoi(text[split+1:])
	if err != nil {
		panic(err)
	}
	if exponent >= -5 && exponent <= 15 {
		return serdeFloat(value)
	}
	sign := "+"
	if exponent < 0 {
		sign = "-"
		exponent = -exponent
	}
	return text[:split] + "e" + sign + strconv.Itoa(exponent)
}
func (unexpected Unexpected) String() string {
	value := unexpected.input
	switch value.Kind {
	case SerdeBool:
		return fmt.Sprintf("boolean `%t`", value.Bool)
	case SerdeI8, SerdeI16, SerdeI32, SerdeI64:
		return fmt.Sprintf("integer `%d`", value.Signed)
	case SerdeU8, SerdeU16, SerdeU32, SerdeU64:
		return fmt.Sprintf("integer `%d`", value.Unsigned)
	case SerdeI128:
		return "integer `" + wideDecimal(value.Wide, true) + "` as i128"
	case SerdeU128:
		return "integer `" + wideDecimal(value.Wide, false) + "` as u128"
	case SerdeF32, SerdeF64:
		return "floating point `" + serdeFloat(value.Float) + "`"
	case SerdeString, SerdeBorrowedString, SerdeOwnedString:
		return "string " + strconv.Quote(value.Text)
	case SerdeChar:
		return "character `" + string(value.Char) + "`"
	case SerdeBytes, SerdeBorrowedBytes, SerdeByteBuf:
		return "byte array"
	case SerdeNone, SerdeSome:
		return "Option value"
	case SerdeUnit:
		return "unit value"
	case SerdeNewtypeStruct:
		return "newtype struct"
	case SerdeSeq:
		return "sequence"
	case SerdeMap:
		return "map"
	case SerdeEnum:
		return "enum"
	default:
		panic("semver: invalid serde input kind")
	}
}

// invalidTypeMessage retains both delegated Display values, as serde's
// Error::invalid_type passes fmt::Arguments rather than an eager String.
// The nested arguments have default format flags, independent of outer flags.
type invalidTypeMessage struct {
	unexpected Unexpected
	expected   SerdeExpected
}

func (value invalidTypeMessage) String() string {
	var out strings.Builder
	if err := value.WriteDisplay(&out, FormatOptions{}); err != nil {
		panic("a Display implementation returned an error unexpectedly: Error")
	}
	return out.String()
}
func (value invalidTypeMessage) WriteDisplay(writer io.Writer, _ FormatOptions) error {
	if err := writeDisplay(writer, "invalid type: "); err != nil {
		return err
	}
	if err := value.unexpected.WriteDisplay(writer, FormatOptions{}); err != nil {
		return err
	}
	if err := writeDisplay(writer, ", expected "); err != nil {
		return err
	}
	return value.expected.WriteExpecting(writer)
}
func (unexpected Unexpected) WriteDisplay(writer io.Writer, _ FormatOptions) error {
	value := unexpected.input
	prefix, suffix := "", ""
	var body func() error
	switch unexpected.Kind() {
	case UnexpectedBool:
		prefix, suffix = "boolean `", "`"
		body = func() error { return writeDisplay(writer, strconv.FormatBool(value.Bool)) }
	case UnexpectedSigned:
		prefix, suffix = "integer `", "`"
		body = func() error {
			if value.Signed < 0 {
				if err := writeChar(writer, '-'); err != nil {
					return err
				}
				return writeDisplay(writer, strconv.FormatUint(uint64(-(value.Signed+1))+1, 10))
			}
			return writeDisplay(writer, strconv.FormatInt(value.Signed, 10))
		}
	case UnexpectedUnsigned:
		prefix, suffix = "integer `", "`"
		body = func() error { return writeDisplay(writer, strconv.FormatUint(value.Unsigned, 10)) }
	case UnexpectedFloat:
		prefix, suffix = "floating point `", "`"
		body = func() error { return writeSerdeFloat(writer, value.Float) }
	case UnexpectedChar:
		prefix, suffix = "character `", "`"
		body = func() error { return writeChar(writer, value.Char) }
	default:
		return writeDisplay(writer, unexpected.String())
	}
	if err := writeDisplay(writer, prefix); err != nil {
		return err
	}
	if err := body(); err != nil {
		return err
	}
	return writeDisplay(writer, suffix)
}

// writeSerdeFloat reproduces the default f64 Display parts used by serde's
// WithDecimalPoint: sign, shortest significand slices, and runs of at most 64
// zeros. It preserves each delegated write and appends .0 only if needed.
func writeSerdeFloat(writer io.Writer, value float64) error {
	if math.IsNaN(value) {
		return writeDisplay(writer, "NaN")
	}
	if math.Signbit(value) {
		if err := writeDisplay(writer, "-"); err != nil {
			return err
		}
		value = -value
	}
	if math.IsInf(value, 1) {
		return writeDisplay(writer, "inf")
	}
	if value == 0 {
		if err := writeDisplay(writer, "0"); err != nil {
			return err
		}
		return writeDisplay(writer, ".0")
	}
	digits, exponent := rustShortestFloat(value)
	decimal := exponent + 1
	zeroes := func(n int) error {
		for n > 64 {
			if err := writeDisplay(writer, strings.Repeat("0", 64)); err != nil {
				return err
			}
			n -= 64
		}
		if n > 0 {
			return writeDisplay(writer, strings.Repeat("0", n))
		}
		return nil
	}
	switch {
	case decimal <= 0:
		if err := writeDisplay(writer, "0."); err != nil {
			return err
		}
		if err := zeroes(-decimal); err != nil {
			return err
		}
		return writeDisplay(writer, digits)
	case decimal < len(digits):
		if err := writeDisplay(writer, digits[:decimal]); err != nil {
			return err
		}
		if err := writeDisplay(writer, "."); err != nil {
			return err
		}
		return writeDisplay(writer, digits[decimal:])
	default:
		if err := writeDisplay(writer, digits); err != nil {
			return err
		}
		if err := zeroes(decimal - len(digits)); err != nil {
			return err
		}
		return writeDisplay(writer, ".0")
	}
}

// SerdeVisitor exposes the source's private Visitor through equivalent tagged
// inputs instead of requiring Go methods with Rust's generic error lifetimes.
type SerdeVisitor interface {
	SerdeExpected
	Visit(SerdeInput) (any, error)
}
type stringVisitor struct {
	kind    uint8
	factory Deserializer
}

func (visitor stringVisitor) Expecting() string {
	if visitor.kind == 2 {
		return "semver comparator"
	}
	return "semver version"
}
func (visitor stringVisitor) WriteExpecting(writer io.Writer) error {
	return writeDisplay(writer, visitor.Expecting())
}
func (visitor stringVisitor) Visit(value SerdeInput) (any, error) {
	factory := value.ErrorFactory
	if factory == nil {
		factory = visitor.factory
	}
	switch value.Kind {
	case SerdeString, SerdeBorrowedString, SerdeOwnedString:
		requireUTF8(value.Text)
		var parsed any
		var err error
		switch visitor.kind {
		case 0:
			parsed, err = ParseVersion(value.Text)
		case 1:
			parsed, err = ParseVersionReq(value.Text)
		case 2:
			parsed, err = ParseComparator(value.Text)
		default:
			panic("semver: invalid serde visitor")
		}
		if err != nil {
			return nil, factory.CustomError(err.(*Error))
		}
		return parsed, nil
	case SerdeChar:
		if !utf8.ValidRune(value.Char) {
			panic("semver: serde char must be a Unicode scalar")
		}
		return visitor.Visit(SerdeInput{Kind: SerdeString, Text: string(value.Char), ErrorFactory: factory})
	case SerdeI8:
		if value.Signed < math.MinInt8 || value.Signed > math.MaxInt8 {
			panic("semver: serde i8 out of range")
		}
	case SerdeI16:
		if value.Signed < math.MinInt16 || value.Signed > math.MaxInt16 {
			panic("semver: serde i16 out of range")
		}
	case SerdeI32:
		if value.Signed < math.MinInt32 || value.Signed > math.MaxInt32 {
			panic("semver: serde i32 out of range")
		}
	case SerdeU8:
		if value.Unsigned > math.MaxUint8 {
			panic("semver: serde u8 out of range")
		}
	case SerdeU16:
		if value.Unsigned > math.MaxUint16 {
			panic("semver: serde u16 out of range")
		}
	case SerdeU32:
		if value.Unsigned > math.MaxUint32 {
			panic("semver: serde u32 out of range")
		}
	case SerdeF32:
		value.Float = float64(float32(value.Float))
	case SerdeBool, SerdeI64, SerdeI128, SerdeU64, SerdeU128, SerdeF64, SerdeBytes, SerdeBorrowedBytes, SerdeByteBuf, SerdeNone, SerdeSome, SerdeUnit, SerdeNewtypeStruct, SerdeSeq, SerdeMap, SerdeEnum:
	default:
		panic("semver: invalid serde input kind")
	}
	unexpected := Unexpected{value}
	if factory, ok := factory.(interface {
		InvalidType(Unexpected, SerdeExpected) error
	}); ok {
		return nil, factory.InvalidType(unexpected, visitor)
	}
	return nil, factory.CustomError(invalidTypeMessage{unexpected: unexpected, expected: visitor})
}
func DeserializeVersion(deserializer Deserializer) (Version, error) {
	value, err := deserializer.DeserializeStr(stringVisitor{0, deserializer})
	if err != nil {
		return Version{}, err
	}
	result, ok := value.(Version)
	if !ok {
		panic("semver: deserializer returned wrong Visitor::Value")
	}
	return result, nil
}
func DeserializeVersionReq(deserializer Deserializer) (VersionReq, error) {
	value, err := deserializer.DeserializeStr(stringVisitor{1, deserializer})
	if err != nil {
		return VersionReq{}, err
	}
	result, ok := value.(VersionReq)
	if !ok {
		panic("semver: deserializer returned wrong Visitor::Value")
	}
	return result, nil
}
func DeserializeComparator(deserializer Deserializer) (Comparator, error) {
	value, err := deserializer.DeserializeStr(stringVisitor{2, deserializer})
	if err != nil {
		return Comparator{}, err
	}
	result, ok := value.(Comparator)
	if !ok {
		panic("semver: deserializer returned wrong Visitor::Value")
	}
	return result, nil
}

// SerdeError is the default adapter's associated error; custom deserializers
// retain their own error identity instead. It adds no source-chain wrapper.
type SerdeError struct {
	Message string
	Syntax  bool
}

func (err *SerdeError) Error() string { return err.Message }

func textDeserializer(text string) DeserializerAdapter {
	return DeserializerAdapter{DeserializeString: func(visitor SerdeVisitor) (any, error) {
		return visitor.Visit(SerdeInput{Kind: SerdeString, Text: text})
	}}
}
func (value Version) MarshalText() ([]byte, error)    { return []byte(value.String()), nil }
func (value VersionReq) MarshalText() ([]byte, error) { return []byte(value.String()), nil }
func (value Comparator) MarshalText() ([]byte, error) { return []byte(value.String()), nil }
func (value *Version) UnmarshalText(text []byte) error {
	if !utf8.Valid(text) {
		return &SerdeError{Message: "invalid UTF-8 in semver text"}
	}
	parsed, err := DeserializeVersion(textDeserializer(string(text)))
	if err == nil {
		*value = parsed
	}
	return err
}
func (value *VersionReq) UnmarshalText(text []byte) error {
	if !utf8.Valid(text) {
		return &SerdeError{Message: "invalid UTF-8 in semver text"}
	}
	parsed, err := DeserializeVersionReq(textDeserializer(string(text)))
	if err == nil {
		*value = parsed
	}
	return err
}
func (value *Comparator) UnmarshalText(text []byte) error {
	if !utf8.Valid(text) {
		return &SerdeError{Message: "invalid UTF-8 in semver text"}
	}
	parsed, err := DeserializeComparator(textDeserializer(string(text)))
	if err == nil {
		*value = parsed
	}
	return err
}

// MarshalJSON returns serde_json's unescaped ASCII semver string. Go's outer
// json.Marshal may apply its own HTML escaping; use Encoder.SetEscapeHTML(false)
// or these methods directly when canonical serde_json bytes are required.
func (value Version) MarshalJSON() ([]byte, error) { return []byte("\"" + value.String() + "\""), nil }
func (value VersionReq) MarshalJSON() ([]byte, error) {
	return []byte("\"" + value.String() + "\""), nil
}
func (value Comparator) MarshalJSON() ([]byte, error) {
	return []byte("\"" + value.String() + "\""), nil
}
func jsonDeserializer(data []byte) (DeserializerAdapter, error) {
	var parsed any
	trimmed := strings.TrimLeft(string(data), " \t\r\n")
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	container := SerdeInputKind(255)
	if strings.HasPrefix(trimmed, "[") {
		container = SerdeSeq
	} else if strings.HasPrefix(trimmed, "{") {
		container = SerdeMap
	}
	if container == 255 {
		if err := decoder.Decode(&parsed); err != nil {
			return DeserializerAdapter{}, &SerdeError{Message: err.Error(), Syntax: true}
		}
		// Deserializers reject Seq/Map without traversing children, and errors
		// from the first scalar visitor precede invalid trailing input.
		// Validate only the scalar bytes actually consumed by this decoder.
		consumed := data[:decoder.InputOffset()]
		if !utf8.Valid(consumed) {
			return DeserializerAdapter{}, &SerdeError{Message: "invalid UTF-8 in semver JSON", Syntax: true}
		}
		if !validJSONSurrogates(consumed) {
			return DeserializerAdapter{}, &SerdeError{Message: "unpaired UTF-16 surrogate in semver JSON", Syntax: true}
		}
	}
	input := SerdeInput{}
	if container != 255 {
		input.Kind = container
	} else {
		switch value := parsed.(type) {
		case string:
			input.Kind = SerdeString
			input.Text = value
		case nil:
			input.Kind = SerdeUnit
		case bool:
			input.Kind = SerdeBool
			input.Bool = value
		case json.Number:
			text := string(value)
			if strings.ContainsAny(text, ".eE") {
				number, err := strconv.ParseFloat(text, 64)
				if err != nil {
					return DeserializerAdapter{}, &SerdeError{Message: err.Error(), Syntax: true}
				}
				input.Kind = SerdeF64
				input.Float = number
			} else if text == "-0" {
				input.Kind = SerdeF64
				input.Float = math.Copysign(0, -1)
			} else if strings.HasPrefix(text, "-") {
				number, err := strconv.ParseInt(text, 10, 64)
				if err == nil {
					input.Kind = SerdeI64
					input.Signed = number
				} else {
					input.Kind = SerdeF64
					var parseErr error
					input.Float, parseErr = strconv.ParseFloat(text, 64)
					if parseErr != nil {
						return DeserializerAdapter{}, &SerdeError{Message: "number out of range in semver JSON", Syntax: true}
					}
				}
			} else {
				number, err := strconv.ParseUint(text, 10, 64)
				if err == nil {
					input.Kind = SerdeU64
					input.Unsigned = number
				} else {
					input.Kind = SerdeF64
					var parseErr error
					input.Float, parseErr = strconv.ParseFloat(text, 64)
					if parseErr != nil {
						return DeserializerAdapter{}, &SerdeError{Message: "number out of range in semver JSON", Syntax: true}
					}
				}
			}
		case []any:
			input.Kind = SerdeSeq
		case map[string]any:
			input.Kind = SerdeMap
		default:
			panic("semver: unexpected encoding/json value")
		}
	}
	adapter := DeserializerAdapter{DeserializeString: func(visitor SerdeVisitor) (any, error) {
		value, err := visitor.Visit(input)
		if err == nil {
			if tailErr := decoder.Decode(new(any)); tailErr != io.EOF {
				return nil, &SerdeError{Message: "trailing data after semver JSON value", Syntax: true}
			}
		}
		return value, err
	}, InvalidTypeOverride: func(value Unexpected, expected SerdeExpected) error {
		description := value.String()
		if value.Kind() == UnexpectedFloat {
			description = "floating point `" + jsonErrorFloat(value.Input().Float) + "`"
		}
		if value.Kind() == UnexpectedUnit {
			description = "null"
		}
		return &SerdeError{Message: "invalid type: " + description + ", expected " + expected.Expecting()}
	}}
	return adapter, nil
}

// The host encoding/json decoder accepts lone UTF-16 surrogates by replacing
// them. serde_json rejects them before the semver visitor, so validate pairing.
func validJSONSurrogates(data []byte) bool {
	quoted := false
	for i := 0; i < len(data); {
		if data[i] == '"' {
			quoted = !quoted
			i++
			continue
		}
		if !quoted || data[i] != '\\' {
			i++
			continue
		}
		if i+1 >= len(data) {
			return false
		}
		if data[i+1] != 'u' {
			i += 2
			continue
		}
		if i+6 > len(data) {
			return false
		}
		first, err := strconv.ParseUint(string(data[i+2:i+6]), 16, 16)
		if err != nil {
			return false
		}
		if first >= 0xdc00 && first <= 0xdfff {
			return false
		}
		if first >= 0xd800 && first <= 0xdbff {
			if i+12 > len(data) || string(data[i+6:i+8]) != "\\u" {
				return false
			}
			second, err := strconv.ParseUint(string(data[i+8:i+12]), 16, 16)
			if err != nil || second < 0xdc00 || second > 0xdfff {
				return false
			}
			i += 12
		} else {
			i += 6
		}
	}
	return true
}
func (value *Version) UnmarshalJSON(data []byte) error {
	deserializer, err := jsonDeserializer(data)
	if err != nil {
		return err
	}
	parsed, err := DeserializeVersion(deserializer)
	if err == nil {
		*value = parsed
	}
	return err
}
func (value *VersionReq) UnmarshalJSON(data []byte) error {
	deserializer, err := jsonDeserializer(data)
	if err != nil {
		return err
	}
	parsed, err := DeserializeVersionReq(deserializer)
	if err == nil {
		*value = parsed
	}
	return err
}
func (value *Comparator) UnmarshalJSON(data []byte) error {
	deserializer, err := jsonDeserializer(data)
	if err != nil {
		return err
	}
	parsed, err := DeserializeComparator(deserializer)
	if err == nil {
		*value = parsed
	}
	return err
}
