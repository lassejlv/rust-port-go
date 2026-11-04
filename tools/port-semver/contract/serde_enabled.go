//go:build semver_serde

package contract

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
	"io"
	"strconv"
	"strings"
)

func observeSerde(q Request) any {
	if q.Input == "json" || q.Input == "json_bytes" {
		var value interface {
			UnmarshalJSON([]byte) error
			MarshalJSON() ([]byte, error)
			String() string
		}
		switch q.Kind {
		case "version":
			value = &semver.Version{}
		case "req":
			value = &semver.VersionReq{}
		case "comparator":
			value = &semver.Comparator{}
		}
		data := []byte(q.A)
		if q.Input == "json_bytes" {
			var err error
			data, err = hex.DecodeString(q.A)
			if err != nil {
				panic(err)
			}
		}
		err := value.UnmarshalJSON(data)
		if err != nil {
			message := err.Error()
			if err.(*semver.SerdeError).Syntax {
				message = "invalid JSON"
			}
			return map[string]any{"ok": false, "error": message, "syntax": err.(*semver.SerdeError).Syntax, "trace": []string{}}
		}
		encoded, err := value.MarshalJSON()
		if err != nil {
			panic(err)
		}
		return map[string]any{"ok": true, "value": value.String(), "encoded": string(encoded), "trace": []string{}}
	}
	trace := []string{}
	var value any
	var err error
	if q.Input == "serialize" {
		v := must(q.Kind, q.A)
		s := semver.SerializerAdapter{SerializeString: func(s string) (any, error) {
			trace = append(trace, "serialize_str:"+s)
			if q.Reject {
				return nil, fmt.Errorf("serialize rejection")
			}
			return s, nil
		}}
		if q.Collect {
			s.CollectStringOverride = func(v semver.Display) (any, error) {
				trace = append(trace, "collect_str:"+v.String())
				if q.Reject {
					return nil, fmt.Errorf("collect rejection")
				}
				return "collected:" + v.String(), nil
			}
		}
		switch v := v.(type) {
		case semver.Version:
			value, err = v.Serialize(s)
		case semver.VersionReq:
			value, err = v.Serialize(s)
		case semver.Comparator:
			value, err = v.Serialize(s)
		}
	} else {
		kinds := map[string]semver.SerdeInputKind{"str": semver.SerdeString, "borrowed_str": semver.SerdeBorrowedString, "string": semver.SerdeOwnedString, "char": semver.SerdeChar, "bool": semver.SerdeBool, "i8": semver.SerdeI8, "i16": semver.SerdeI16, "i32": semver.SerdeI32, "i64": semver.SerdeI64, "i128": semver.SerdeI128, "u8": semver.SerdeU8, "u16": semver.SerdeU16, "u32": semver.SerdeU32, "u64": semver.SerdeU64, "u128": semver.SerdeU128, "f32": semver.SerdeF32, "f64": semver.SerdeF64, "bytes": semver.SerdeBytes, "borrowed_bytes": semver.SerdeBorrowedBytes, "byte_buf": semver.SerdeByteBuf, "none": semver.SerdeNone, "some": semver.SerdeSome, "unit": semver.SerdeUnit, "newtype": semver.SerdeNewtypeStruct, "seq": semver.SerdeSeq, "map": semver.SerdeMap, "enum": semver.SerdeEnum}
		f, _ := strconv.ParseFloat(q.Float, 64)
		input := semver.SerdeInput{Kind: kinds[q.Input], Text: q.A, Signed: q.Signed, Unsigned: q.Unsigned, Wide: q.Wide, Float: f, Bool: q.Collect, Bytes: []byte(q.A), Data: func() { panic("child traversed") }}
		if q.Input == "char" {
			input.Char = []rune(q.A)[0]
		}
		d := semver.DeserializerAdapter{Custom: func(s semver.Display) error {
			if q.Reject {
				sink := &serdeSink{chunks: []string{}, fail: q.Fail}
				err := s.WriteDisplay(sink, semver.FormatOptions{Width: semver.Some(uint(30)), Fill: semver.Some('*'), Align: semver.AlignCenter})
				var rawBuffer bytes.Buffer
				encoder := json.NewEncoder(&rawBuffer)
				encoder.SetEscapeHTML(false)
				_ = encoder.Encode(map[string]any{"chunks": sink.chunks, "accepted": sink.accepted.String(), "error": err != nil})
				raw := bytes.TrimSuffix(rawBuffer.Bytes(), []byte{'\n'})
				trace = append(trace, "custom_writer:"+string(raw))
				return fmt.Errorf("custom writer")
			}
			trace = append(trace, "custom:"+s.String())
			return fmt.Errorf("%s", s.String())
		}}
		if q.Override {
			d.InvalidTypeOverride = func(u semver.Unexpected, e semver.SerdeExpected) error {
				classes := []string{"Bool", "Signed", "Unsigned", "Float", "Char", "Str", "Bytes", "Unit", "Option", "NewtypeStruct", "Seq", "Map", "Enum", "Other"}
				s := "invalid_type:" + classes[u.Kind()] + ":" + u.String() + ":" + e.Expecting()
				trace = append(trace, s)
				return fmt.Errorf("%s", s)
			}
		}
		d.DeserializeString = func(v semver.SerdeVisitor) (any, error) {
			trace = append(trace, "deserialize_str", "expecting:"+v.Expecting())
			return v.Visit(input)
		}
		switch q.Kind {
		case "version":
			value, err = semver.DeserializeVersion(d)
		case "req":
			value, err = semver.DeserializeVersionReq(d)
		case "comparator":
			value, err = semver.DeserializeComparator(d)
		}
		if err == nil {
			value = value.(fmt.Stringer).String()
		}
	}
	if err != nil {
		return map[string]any{"ok": false, "error": err.Error(), "trace": trace}
	}
	return map[string]any{"ok": true, "value": value, "trace": trace}
}

type serdeSink struct {
	chunks   []string
	accepted strings.Builder
	fail     uint64
}

func (w *serdeSink) Write(p []byte) (int, error) { return w.WriteString(string(p)) }
func (w *serdeSink) WriteString(s string) (int, error) {
	w.chunks = append(w.chunks, "S:"+s)
	if w.fail != 0 && uint64(len(w.chunks)) == w.fail {
		return 0, io.ErrClosedPipe
	}
	return w.accepted.WriteString(s)
}
func (w *serdeSink) WriteChar(c rune) error {
	w.chunks = append(w.chunks, "C:"+string(c))
	if w.fail != 0 && uint64(len(w.chunks)) == w.fail {
		return io.ErrClosedPipe
	}
	_, err := w.accepted.WriteRune(c)
	return err
}
