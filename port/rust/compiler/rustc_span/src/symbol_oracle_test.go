// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
	rustvec "github.com/lassejlv/rust-port-go/port/support/rustvec"
)

type symbolOracleManifest struct {
	Records       int    `json:"records"`
	RawSHA256     string `json:"raw_sha256"`
	FixtureSHA256 string `json:"fixture_sha256"`
	Formats       []struct {
		Pattern string          `json:"pattern"`
		Debug   bool            `json:"debug"`
		Options rustfmt.Options `json:"options"`
	} `json:"formats"`
	Aborts []struct {
		Mode  int      `json:"mode"`
		Trace []string `json:"trace"`
	} `json:"owned_context_aborts"`
	ConstructorLocations         map[string]SourceLocation         `json:"constructor_direct_locations"`
	ConstructorInternalLocations map[string]rustfmt.SourceLocation `json:"constructor_internal_location_profile"`
}

type symbolOracleContext struct {
	HashSpans bool
	Spans     []uint64
}

func (c *symbolOracleContext) StableHashSpan(span datastructures.RawSpan, h *datastructures.StableHasher) {
	c.Spans = append(c.Spans, span.Value)
	if c.HashSpans {
		h.WriteU64(span.Value ^ 0x12345678)
	}
}
func (c *symbolOracleContext) DefPathHash(id datastructures.RawDefId) datastructures.Fingerprint {
	return datastructures.FingerprintFromU64(0x1234+uint64(id.Crate), 0x5678+uint64(id.Index))
}
func (c *symbolOracleContext) StableHashControls() datastructures.StableHashControls {
	return datastructures.StableHashControls{HashSpans: c.HashSpans}
}
func (c *symbolOracleContext) AssertDefaultStableHashControls(message string) {
	if c.HashSpans {
		panic(message)
	}
}

func symbolOracleU64(text string) uint64 {
	value, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		panic(err)
	}
	return value
}
func symbolOracleBytes(text string) []byte {
	value, err := hex.DecodeString(text)
	if err != nil {
		panic(err)
	}
	return value
}
func symbolOracleHex(text string) string { return hex.EncodeToString([]byte(text)) }
func symbolOracleSymbols(session *SessionGlobals, text string) []Symbol {
	if text == "-" {
		return nil
	}
	values := strings.Split(text, ",")
	result := make([]Symbol, len(values))
	for i, value := range values {
		result[i] = InternSymbol(session, string(symbolOracleBytes(value)))
	}
	return result
}
func symbolOracleOption(value Option[Symbol], session *SessionGlobals) string {
	if symbol, ok := value.Get(); ok {
		return symbolOracleHex(symbol.AsStr(session))
	}
	return "none"
}
func symbolOracleDistance(value string) Option[uint] {
	if value == "-" {
		return None[uint]()
	}
	return Some(uint(symbolOracleU64(value)))
}
func symbolOracleOrder(value int) string {
	if value < 0 {
		return "Less"
	}
	if value > 0 {
		return "Greater"
	}
	return "Equal"
}

type symbolOracleHasher []string
type symbolOracleRawHasher []string

func (h *symbolOracleRawHasher) Write(value []byte) { *h = append(*h, hex.EncodeToString(value)) }
func (h *symbolOracleRawHasher) Finish64() uint64   { return 0 }

func (h *symbolOracleHasher) Write(value []byte) { *h = append(*h, "bytes:"+hex.EncodeToString(value)) }
func (h *symbolOracleHasher) Finish64() uint64   { return 0 }

func (h *symbolOracleHasher) WriteU32(value uint32) { *h = append(*h, fmt.Sprintf("u32:%d", value)) }

type symbolOracleSink struct {
	Calls, Fail int
	Events      []string
	Rendered    strings.Builder
}

var symbolOracleWriteError = errors.New("symbol oracle writer failure")

func (s *symbolOracleSink) Write(value []byte) (int, error) { return s.WriteString(string(value)) }
func (s *symbolOracleSink) WriteString(value string) (int, error) {
	s.Calls++
	s.Events = append(s.Events, "S:"+symbolOracleHex(value))
	if s.Calls == s.Fail {
		return 0, symbolOracleWriteError
	}
	s.Rendered.WriteString(value)
	return len(value), nil
}
func (s *symbolOracleSink) WriteChar(value rune) error {
	s.Calls++
	s.Events = append(s.Events, fmt.Sprintf("C:%x", value))
	if s.Calls == s.Fail {
		return symbolOracleWriteError
	}
	s.Rendered.WriteRune(value)
	return nil
}

func symbolOracleOwnedContext(mode int, fail bool, events *[]string) string {
	type state struct{ Value uint32 }
	push := func(value string) {
		*events = append(*events, value)
		if os.Getenv("FERRUGO_SYMBOL_ABORT_TRACE") == "1" {
			fmt.Println(value)
		}
	}
	factory := func(context rustfmt.Context) {
		owned := rustvec.NewOwned(state{7}, rustvec.ValueOps[state]{Drop: func(value *state, context rustfmt.Context) {
			push(fmt.Sprintf("drop:%d:%t", value.Value, context.Panicking()))
			value.Value++
			push(fmt.Sprintf("value:%d", value.Value))
			if fail {
				context.Panic("context-drop")
			}
		}})
		compare := SymbolStableComparatorOwnedWithContext(owned, context)
		push("returned")
		for _, words := range [][2]string{{"zzzz_context", "aaaa_context"}, {"aaaa_context", "zzzz_context"}} {
			session := NewSessionGlobals(DefaultEdition, nil, None[SourceMapInputs]())
			a, b := InternSymbol(session, words[0]), InternSymbol(session, words[1])
			push("compare:" + symbolOracleOrder(compare(session, &a, &b, context)))
		}
	}
	catch := func(call func()) (panicked bool) {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		call()
		return
	}
	switch mode {
	case 0:
		if catch(func() { factory(rustfmt.Context{}) }) {
			return "panic"
		}
		return "ok"
	case 1:
		push("outer-drop")
		if catch(func() { factory(rustfmt.Context{Unwinding: true}) }) {
			push("inner:panic")
		} else {
			push("inner:ok")
		}
		return "panic"
	case 2:
		push("outer-drop")
		if catch(func() { factory(rustfmt.Context{Unwinding: true}) }) {
			os.Exit(134)
		}
		return "panic"
	case 3:
		push("processing")
		factory(rustfmt.Context{ProcessingPanic: true})
		return "panic"
	default:
		panic("invalid owned context mode")
	}
}

func symbolOracleReplay(parts []string, manifest *symbolOracleManifest, cache map[Edition]*SessionGlobals) (out string) {
	defer func() {
		if value := recover(); value != nil {
			out = "panic"
			if parts[0] == "Z" {
				if message, ok := value.(string); ok {
					out += "\t" + symbolOracleHex(message)
				}
			}
		}
	}()
	if parts[0] == "O" {
		events := []string{}
		status := symbolOracleOwnedContext(int(symbolOracleU64(parts[2])), parts[3] == "1", &events)
		return "ok\t" + status + "\t" + strings.Join(events, ",")
	}
	if parts[0] == "Z" {
		extra := []string{}
		if parts[2] != "-" {
			for _, value := range strings.Split(parts[2], ",") {
				extra = append(extra, string(symbolOracleBytes(value)))
			}
		}
		session := NewSessionGlobals(Edition(symbolOracleU64(parts[1])), extra, None[SourceMapInputs]())
		values := []string{}
		for _, entry := range strings.Split(parts[3], ",") {
			symbol := InternSymbol(session, string(symbolOracleBytes(entry[1:])))
			values = append(values, fmt.Sprintf("%d:%s", symbol.AsU32(), symbolOracleHex(symbol.AsStr(session))))
		}
		return "ok\t" + strings.Join(values, ",")
	}
	edition := Edition(symbolOracleU64(parts[1]))
	var session *SessionGlobals
	// Reuse only the stateless-observable cases: fixed keyword IDs, string-based
	// matching and ordinary root-context formatting. Every ID-producing case and
	// dollar-crate mutation starts the exact fresh session used by the reference.
	reusable := parts[0] == "K" || parts[0] == "U" || parts[0] == "M" || parts[0] == "N" || parts[0] == "R" || parts[0] == "E" || parts[0] == "F" && parts[2] != "A"
	if reusable {
		session = cache[edition]
	}
	if session == nil {
		session = NewSessionGlobals(edition, nil, None[SourceMapInputs]())
		if reusable {
			cache[edition] = session
		}
	}
	result := ""
	switch parts[0] {
	case "K":
		value := NewSymbol(uint32(symbolOracleU64(parts[2])))
		ident := Ident{Name: value, Span: DummySpan()}
		calls := 0
		reserved := value.IsReserved(func() Edition { calls++; return edition })
		flags := []bool{value.IsEmpty(), reserved, value.IsWeak(), value.IsPathSegmentKeyword(), value.IsBoolLit(), value.CanBeRaw(), IsPredefinedSymbol(value.AsU32()), ident.IsSpecial(), ident.IsUsedKeyword(session), ident.IsUnusedKeyword(session), ident.IsReserved(session), ident.IsPathSegmentKeyword(), ident.IsRawGuess(session), ident.IsReservedLifetime(session), ident.IsRawLifetimeGuess(session), ident.IsNumeric(session)}
		var bits strings.Builder
		for _, flag := range flags {
			if flag {
				bits.WriteByte('1')
			} else {
				bits.WriteByte('0')
			}
		}
		var h, ih symbolOracleHasher
		value.Hash(&h)
		ident.Hash(&ih)
		result = fmt.Sprintf("%s\t%d\t%d\t%s\t%s\t%s", bits.String(), calls, ident.GuessPrintMode(session), symbolOracleHex(value.AsStr(session)), strings.Join(h, ","), strings.Join(ih, ","))
	case "U":
		sequence := strings.Split(parts[2], ",")
		calls := 0
		values := UsedKeywords(func() Edition { e := Edition(symbolOracleU64(sequence[calls%len(sequence)])); calls++; return e })
		ids := make([]string, len(values))
		for i, value := range values {
			ids[i] = strconv.FormatUint(uint64(value.AsU32()), 10)
		}
		result = fmt.Sprintf("%s\t%d", strings.Join(ids, ","), calls)
	case "I":
		values := []string{}
		for _, entry := range strings.Split(parts[2], ",") {
			data := symbolOracleBytes(entry[1:])
			if entry[0] == 's' {
				value := InternSymbol(session, string(data))
				values = append(values, fmt.Sprintf("%d:%s", value.AsU32(), symbolOracleHex(value.AsStr(session))))
			} else {
				value := InternByteSymbol(session, data)
				if entry[0] == 'm' && len(data) != 0 {
					data[0] ^= 255
				}
				values = append(values, fmt.Sprintf("%d:%s", value.AsU32(), symbolOracleHex(value.AsByteStr(session))))
			}
		}
		result = strings.Join(values, ",")
	case "M":
		candidates := symbolOracleSymbols(session, parts[2])
		lookup := InternSymbol(session, string(symbolOracleBytes(parts[3])))
		dist := symbolOracleDistance(parts[4])
		var value Option[Symbol]
		if parts[5] == "1" {
			value = FindBestMatchForNameWithSubstrings(session, candidates, lookup, dist)
		} else {
			value = FindBestMatchForName(session, candidates, lookup, dist)
		}
		result = symbolOracleOption(value, session)
	case "N":
		result = symbolOracleOption(FindBestMatchForNames(session, symbolOracleSymbols(session, parts[2]), symbolOracleSymbols(session, parts[3]), symbolOracleDistance(parts[4])), session)
	case "R":
		candidates := symbolOracleSymbols(session, parts[2])
		lookup := InternSymbol(session, string(symbolOracleBytes(parts[3])))
		if value, ok := lookup.FindSimilar(session, candidates).Get(); ok {
			result = fmt.Sprintf("%s:%t", symbolOracleHex(value.Symbol.AsStr(session)), value.IncorrectCase)
		} else {
			result = "none"
		}
	case "V":
		a := InternSymbol(session, string(symbolOracleBytes(parts[2])))
		b := InternSymbol(session, string(symbolOracleBytes(parts[3])))
		var h symbolOracleHasher
		a.Hash(&h)
		stable := datastructures.NewStableHasher()
		stable.Write(symbolOracleBytes(parts[4]))
		a.StableHash(session, nil, stable)
		hash := stable.FinishHash()
		result = fmt.Sprintf("%d\t%d\t%s\t%s\t%s\t%d\t%d", a.AsU32(), b.AsU32(), symbolOracleOrder(a.Compare(b)), symbolOracleOrder(a.StableCompare(session, b)), strings.Join(h, ","), hash[0], hash[1])
	case "B":
		value := InternByteSymbol(session, symbolOracleBytes(parts[2]))
		var h symbolOracleHasher
		value.Hash(&h)
		stable := datastructures.NewStableHasher()
		stable.Write(symbolOracleBytes(parts[3]))
		value.StableHash(session, nil, stable)
		hash := stable.FinishHash()
		result = fmt.Sprintf("%d\t%s\t%d\t%d", value.AsU32(), strings.Join(h, ","), hash[0], hash[1])
	case "F":
		sink := symbolOracleSink{Fail: int(symbolOracleU64(parts[6]))}
		format := manifest.Formats[symbolOracleU64(parts[4])]
		f := rustfmt.Formatter{Writer: &sink, Options: format.Options}
		status := "ok"
		func() {
			defer func() {
				if recover() != nil {
					status = "panic"
				}
			}()
			data := symbolOracleBytes(parts[3])
			var err error
			switch parts[2] {
			case "B":
				err = InternByteSymbol(session, data).WriteDebugFormatter(session, &f)
			case "S":
				value := InternSymbol(session, string(data))
				if format.Debug {
					err = value.WriteDebugFormatter(session, &f)
				} else {
					err = value.WriteDisplayFormatter(session, &f)
				}
			case "I":
				value := Ident{Name: InternSymbol(session, string(data)), Span: DummySpan()}
				if format.Debug {
					err = value.WriteDebugFormatter(session, &f)
				} else {
					err = value.WriteDisplayFormatter(session, &f)
				}
			case "N":
				value := NewMacroRulesNormalizedIdent(session, Ident{Name: InternSymbol(session, string(data)), Span: DummySpan()})
				if format.Debug {
					err = value.WriteDebugFormatter(session, &f)
				} else {
					err = value.WriteDisplayFormatter(session, &f)
				}
			case "P":
				err = NewIdentPrinter(InternSymbol(session, string(data)), IdentPrintMode(symbolOracleU64(parts[7])), None[Span]()).WriteDisplayFormatter(session, &f)
			case "A":
				dollar := InternSymbol(session, string(symbolOracleBytes(parts[8])))
				UpdateDollarCrateNames(session, func(SyntaxContext) Symbol { return dollar })
				err = IdentPrinterForASTIdent(Ident{Name: InternSymbol(session, string(data)), Span: DummySpan()}, IdentPrintMode(symbolOracleU64(parts[7]))).WriteDisplayFormatter(session, &f)
			default:
				panic("unknown format kind")
			}
			if err != nil {
				status = "error"
			}
		}()
		result = status + "\t" + symbolOracleHex(sink.Rendered.String()) + "\t" + strings.Join(sink.Events, ",")
	case "C":
		data := symbolOracleBytes(parts[3])
		encoder := NewSpanMemEncoder(session)
		switch parts[2] {
		case "S":
			InternSymbol(session, string(data)).Encode(encoder)
		case "B":
			InternByteSymbol(session, data).Encode(encoder)
		case "I":
			Ident{Name: InternSymbol(session, string(data)), Span: NewSpan(session, BytePos(symbolOracleU64(parts[4])), BytePos(symbolOracleU64(parts[5])), RootSyntaxContext(), None[LocalDefId]())}.Encode(encoder)
		default:
			panic("invalid codec kind")
		}
		raw := encoder.Finish()
		framed := append(append([]byte{}, raw...), []byte("rust-end-file")...)
		decoder, err := NewSpanMemDecoder(session, framed, 0)
		if err != nil {
			panic(err)
		}
		decoded := ""
		switch parts[2] {
		case "S":
			decoded = symbolOracleHex(DecodeSymbol(decoder).AsStr(session))
		case "B":
			decoded = symbolOracleHex(DecodeByteSymbol(decoder).AsByteStr(session))
		case "I":
			value := DecodeIdent(decoder)
			decoded = fmt.Sprintf("%s:%d:%d:%d", symbolOracleHex(value.AsStr(session)), value.Span.Lo(session), value.Span.Hi(session), value.Span.Ctxt().AsU32())
		}
		result = fmt.Sprintf("%s\t%s\t%d", hex.EncodeToString(raw), decoded, decoder.Position())
	case "J":
		func() {
			defer func() {
				if recover() != nil {
					result = "panic"
				}
			}()
			result = strconv.FormatUint(uint64(NewSymbol(uint32(symbolOracleU64(parts[2]))).AsU32()), 10)
		}()
	case "E":
		value := Ident{Name: InternSymbol(session, string(symbolOracleBytes(parts[2]))), Span: DummySpan()}
		result = symbolOracleHex(value.WithoutFirstQuote(session).AsStr(session)) + "\t" + symbolOracleHex(value.Name.ToIdentString(session)) + "\t" + symbolOracleHex(DummyIdent(session).AsStr(session))
	case "L":
		c := rune(symbolOracleU64(parts[2]))
		value := CharacterSymbol(session, c)
		ascii := "none"
		if a, ok := ASCIILetterDigitSymbol(c); ok {
			ascii = strconv.FormatUint(uint64(a.AsU32()), 10)
		}
		result = fmt.Sprintf("%d\t%s\t%s", value.AsU32(), symbolOracleHex(value.AsStr(session)), ascii)
	case "G":
		var value Symbol
		if parts[2] == "i128" || parts[2] == "u128" {
			n, ok := new(big.Int).SetString(parts[3], 10)
			if !ok {
				panic("invalid integer")
			}
			value = Integer128Symbol(session, n, parts[2] == "i128")
		} else if parts[2] == "isize" {
			n, err := strconv.ParseInt(parts[3], 10, 64)
			if err != nil {
				panic(err)
			}
			value = IntegerSymbol(session, int(n))
		} else {
			value = IntegerSymbol(session, uint(symbolOracleU64(parts[3])))
		}
		result = fmt.Sprintf("%d\t%s", value.AsU32(), symbolOracleHex(value.AsStr(session)))
	case "H":
		ctxt := RootSyntaxContext()
		for i, mark := range parts[2] {
			kind := ExpnKind{Tag: ExpnMacro, MacroKind: BangMacro, Name: InternSymbol(session, fmt.Sprintf("symbol_macro_%d", i))}
			data := DefaultExpnData(kind, DummySpan(), Edition((int(edition)+i+1)%5), None[DefId](), None[ModId]())
			id := FreshLocalExpnId(session, data, &symbolOracleContext{})
			ctxt = ctxt.ApplyMark(session, id.ToExpnId(), Transparency(mark-'0'))
		}
		ident := NewIdent(session, InternSymbol(session, string(symbolOracleBytes(parts[3]))), NewSpan(session, 1, 2, ctxt, None[LocalDefId]()))
		pos := ident.WithSpanPos(session, NewSpan(session, 90, 99, RootSyntaxContext(), None[LocalDefId]()))
		normalized := ident.NormalizeToMacros20(session)
		rules := ident.NormalizeToMacroRules(session)
		wrapped := NewMacroRulesNormalizedIdent(session, ident)
		var hasher symbolOracleHasher
		ident.Hash(&hasher)
		result = fmt.Sprintf("%d\t%d\t%d\t%t\t%t\t%t\t%d\t%d\t%d\t%s\t%s\t%s\t%s", ctxt.AsU32(), normalized.Span.Ctxt().AsU32(), rules.Span.Ctxt().AsU32(), wrapped.Ident().Equal(rules), ident.Equal(pos), ident.Equal(NewIdentWithDummySpan(session, ident.Name)), pos.Span.Lo(session), pos.Span.Hi(session), pos.Span.Ctxt().AsU32(), symbolOracleHex(ident.DisplayString(session)), symbolOracleHex(ident.RustDebug(session)), symbolOracleHex(normalized.DisplayString(session)), strings.Join(hasher, ","))
	case "W":
		value := Ident{Name: InternSymbol(session, string(symbolOracleBytes(parts[2]))), Span: NewSpan(session, BytePos(symbolOracleU64(parts[3])), BytePos(symbolOracleU64(parts[4])), RootSyntaxContext(), None[LocalDefId]())}
		context := &symbolOracleContext{HashSpans: true}
		hasher := datastructures.NewStableHasher()
		hasher.Write(symbolOracleBytes(parts[5]))
		value.StableHash(session, context, hasher)
		hash := hasher.FinishHash()
		spans := make([]string, len(context.Spans))
		for i, span := range context.Spans {
			spans[i] = strconv.FormatUint(span, 10)
		}
		result = fmt.Sprintf("%s\t%d\t%d", strings.Join(spans, ","), hash[0], hash[1])
	case "X":
		events := []string{}
		// This named reference profile normalizes only the native build prefix.
		// Production callers can install exact unnormalized selected-build sites.
		context := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: manifest.ConstructorInternalLocations})
		previous := SwapEmitBugDiagnostic(func(_ *SessionGlobals, span Option[Span], args SourceFormatArguments, location SourceLocation, _ rustfmt.Context) bool {
			value, ok := span.Get()
			if !ok {
				panic("missing source span")
			}
			message, literal := args.AsStr().Get()
			if !literal {
				panic("missing source literal")
			}
			if location.Host {
				panic("unexpected host caller location")
			}
			events = append(events, fmt.Sprintf("%d\t%s\t%t\t%s\t%d\t%d", value.ToRawSpan().Value, symbolOracleHex(message), literal, symbolOracleHex(location.File), location.Line, location.Column))
			return parts[3] == "1"
		})
		defer SwapEmitBugDiagnostic(previous)
		span := NewSpan(session, 1, 2, RootSyntaxContext(), None[LocalDefId]())
		empty := Ident{Name: Sym_empty(), Span: span}
		status := "ok"
		func() {
			defer func() {
				if value := recover(); value != nil {
					switch value.(type) {
					case ExplicitBug:
						status = "panic:explicit"
					case string:
						status = "panic:string"
					default:
						status = "panic:other"
					}
				}
			}()
			switch parts[2] {
			case "new0", "new1":
				NewIdentAtWithContext(session, Sym_empty(), span, manifest.ConstructorLocations[parts[2]], context)
			case "dummy":
				NewIdentWithDummySpanWithContext(session, Sym_empty(), context)
			case "from":
				IdentFromStrWithContext(session, "", context)
			case "from_span":
				IdentFromStrAndSpanWithContext(session, "", span, context)
			case "spanpos":
				empty.WithSpanPosWithContext(session, NewSpan(session, 90, 99, RootSyntaxContext(), None[LocalDefId]()), context)
			case "quote":
				Ident{Name: InternSymbol(session, "'"), Span: span}.WithoutFirstQuoteWithContext(session, context)
			case "macro20":
				empty.NormalizeToMacros20WithContext(session, context)
			case "rules":
				empty.NormalizeToMacroRulesWithContext(session, context)
			case "normalizer":
				NewMacroRulesNormalizedIdentWithContext(session, empty, context)
			default:
				panic("invalid constructor case")
			}
		}()
		result = status + "\t" + strings.Join(events, ";")
	case "P":
		a, b, c := NewSymbol(uint32(symbolOracleU64(parts[2]))), NewSymbol(uint32(symbolOracleU64(parts[3]))), NewSymbol(uint32(symbolOracleU64(parts[4])))
		clamp := func() (out string) {
			defer func() {
				if recover() != nil {
					out = "panic"
				}
			}()
			return strconv.FormatUint(uint64(a.Clamp(b, c).AsU32()), 10)
		}()
		var raw, bytes symbolOracleRawHasher
		HashSymbolSlice([]Symbol{a, b, c}, &raw)
		HashByteSymbolSlice([]ByteSymbol{NewByteSymbol(a.AsU32()), NewByteSymbol(b.AsU32()), NewByteSymbol(c.AsU32())}, &bytes)
		order, present := a.PartialCompare(b).Get()
		if !present {
			panic("unexpected unordered Symbol")
		}
		result = fmt.Sprintf("Some(%s)\t%d\t%d\t%s\t%s\t%s", symbolOracleOrder(order), a.Min(b).AsU32(), a.Max(b).AsU32(), clamp, strings.Join(raw, ","), strings.Join(bytes, ","))
	default:
		panic("unknown symbol request")
	}
	return "ok\t" + result
}

func TestSymbolOriginalFullCrateReference(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("retained native whole-span profile is64-bit; no32-bit runtime equivalence is claimed")
	}
	encoded, err := os.ReadFile("symbol_oracle_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest symbolOracleManifest
	if err = json.Unmarshal(encoded, &manifest); err != nil {
		t.Fatal(err)
	}
	compressed, err := os.ReadFile("testdata/symbol_oracle.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(compressed)
	if hex.EncodeToString(sum[:]) != manifest.FixtureSHA256 {
		t.Fatal("Symbol compressed fixture checksum mismatch")
	}
	z, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer z.Close()
	raw, err := io.ReadAll(io.LimitReader(z, 64<<20))
	if err != nil {
		t.Fatal(err)
	}
	sum = sha256.Sum256(raw)
	if len(raw) >= 64<<20 || hex.EncodeToString(sum[:]) != manifest.RawSHA256 {
		t.Fatal("Symbol raw fixture checksum/bound mismatch")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 8192), 4<<20)
	records := 0
	cache := map[Edition]*SessionGlobals{}
	for scanner.Scan() {
		records++
		pair := strings.SplitN(scanner.Text(), "\t|\t", 2)
		if len(pair) != 2 {
			t.Fatalf("record%d malformed", records)
		}
		request := strings.Split(pair[0], "\t")
		actual := symbolOracleReplay(request, &manifest, cache)
		if actual != pair[1] {
			summary := pair[0]
			if len(summary) > 500 {
				summary = summary[:500] + "..."
			}
			t.Fatalf("record%d %s\ngot: %s\nwant:%s", records, summary, actual, pair[1])
		}
	}
	if err = scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if records != manifest.Records {
		t.Fatalf("records%d, want%d", records, manifest.Records)
	}
}

func TestSymbolOwnedContextAbortHelper(t *testing.T) {
	value := os.Getenv("FERRUGO_SYMBOL_ABORT_MODE")
	if value == "" {
		t.Skip("isolated helper")
	}
	mode, err := strconv.Atoi(value)
	if err != nil {
		t.Fatal(err)
	}
	events := []string{}
	symbolOracleOwnedContext(mode, true, &events)
	t.Fatal("abort helper returned")
}
func TestSymbolOwnedContextNativeAborts(t *testing.T) {
	encoded, err := os.ReadFile("symbol_oracle_manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest symbolOracleManifest
	if err = json.Unmarshal(encoded, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, test := range manifest.Aborts {
		t.Run(strconv.Itoa(test.Mode), func(t *testing.T) {
			command := exec.Command(os.Args[0], "-test.run=^TestSymbolOwnedContextAbortHelper$")
			command.Env = append(os.Environ(), "FERRUGO_SYMBOL_ABORT_TRACE=1", fmt.Sprintf("FERRUGO_SYMBOL_ABORT_MODE=%d", test.Mode))
			output, err := command.CombinedOutput()
			var exit *exec.ExitError
			if !errors.As(err, &exit) || exit.ExitCode() != 134 {
				t.Fatalf("abort exit=%v, output=%s", err, output)
			}
			if actual := strings.Split(strings.TrimSpace(string(output)), "\n"); strings.Join(actual, "\n") != strings.Join(test.Trace, "\n") {
				t.Fatalf("trace=%q, want%q", actual, test.Trace)
			}
		})
	}
}
