package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"testing"

	datastructures "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_data_structures/src"
	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type spanOracleStep struct {
	Result   any      `json:"result"`
	Panic    any      `json:"panic"`
	Events   []string `json:"events"`
	Interner [][]any  `json:"interner"`
	Slots    []string `json:"slots"`
}
type spanOracleResponse struct {
	Steps []spanOracleStep `json:"steps"`
}
type spanReplay struct {
	request spanOracleRequest
	session *SessionGlobals
	config  SpanEncodingConfig
	slots   []Span
	events  []string
	tracks  int
	live    bool
}

func spanReplayNumber(value any) int64 {
	switch v := value.(type) {
	case json.Number:
		n, e := v.Int64()
		if e != nil {
			panic(e)
		}
		return n
	case int:
		return int64(v)
	case uint32:
		return int64(v)
	case uint64:
		return int64(v)
	case float64:
		return int64(v)
	case nil:
		return 0
	default:
		panic(fmt.Sprintf("bad oracle number %T", value))
	}
}
func spanReplayU32(value any) uint32                  { return uint32(spanReplayNumber(value)) }
func spanReplayBool(value any) bool                   { b, _ := value.(bool); return b }
func spanReplayString(value any) string               { s, _ := value.(string); return s }
func spanReplayIndex(op spanOracleOp, key string) int { return int(spanReplayNumber(op[key])) }
func spanReplayParent(value any) Option[LocalDefId] {
	if value == nil {
		return None[LocalDefId]()
	}
	return Some(LocalDefId{DefIndexFromU32(spanReplayU32(value))})
}
func spanReplayRaw(value Span) string { return fmt.Sprintf("%016x", value.ToRawSpan().Value) }
func (e *spanReplay) event(value string) {
	e.events = append(e.events, value)
	if e.live {
		data, _ := json.Marshal(value)
		fmt.Printf("PROBE_EVENT\t%s\n", data)
	}
}
func (e *spanReplay) rawSlots() []string {
	out := make([]string, len(e.slots))
	for i, v := range e.slots {
		out[i] = spanReplayRaw(v)
	}
	return out
}
func (e *spanReplay) interner() [][]any {
	out := make([][]any, 0)
	if e.session == nil {
		return out
	}
	e.session.spanMu.Lock()
	defer e.session.spanMu.Unlock()
	for _, v := range e.session.spanInterner.spans {
		var parent any
		if p, ok := v.parent.Get(); ok {
			parent = p.LocalDefIndex.AsU32()
		}
		out = append(out, []any{uint32(v.lo), uint32(v.hi), parent})
	}
	return out
}
func (e *spanReplay) newFields(op spanOracleOp, ordered bool) Span {
	lo, hi := BytePos(spanReplayU32(op["lo"])), BytePos(spanReplayU32(op["hi"]))
	ctxt := SyntaxContextFromU32(spanReplayU32(op["ctxt"]))
	parent := spanReplayParent(op["parent"])
	if ordered {
		return NewOrderedSpan(e.session, lo, hi, ctxt, parent, e.config)
	}
	return NewSpan(e.session, lo, hi, ctxt, parent, e.config)
}
func (e *spanReplay) track(parent LocalDefId) {
	e.event(fmt.Sprintf("track:%d", parent.LocalDefIndex.AsU32()))
	at := -1
	if e.request.TrackPanicAt != nil {
		at = *e.request.TrackPanicAt
	}
	current := e.tracks
	e.tracks++
	if current == at {
		panic("span oracle track panic")
	}
	if reentry := e.request.TrackReenter; reentry != nil {
		value := NewSpan(e.session, BytePos(reentry.Lo), BytePos(reentry.Hi), SyntaxContextFromU32(reentry.Ctxt), spanReplayParentPointer(reentry.Parent), e.config)
		e.event("track_reenter:" + spanReplayRaw(value))
	}
}
func spanReplayParentPointer(value *uint32) Option[LocalDefId] {
	if value == nil {
		return None[LocalDefId]()
	}
	return Some(LocalDefId{DefIndexFromU32(*value)})
}

type spanReplaySourceMap struct{ engine *spanReplay }

func (m spanReplaySourceMap) SpanToDiagnosticString(_ *SessionGlobals, value Span) string {
	m.engine.event("source_map:" + spanReplayRaw(value))
	if m.engine.request.SourceMap.Panic {
		panic("span oracle source_map panic")
	}
	return m.engine.request.SourceMap.Text
}

func spanReplayPanic(payload any) string {
	text := fmt.Sprint(payload)
	for _, item := range [][2]string{{"source_map", "span oracle source_map"}, {"track", "span oracle track"}, {"map", "span oracle map"}, {"drop", "span oracle drop"}, {"hash", "span oracle hash"}, {"subtract", "subtract with overflow"}, {"add", "add with overflow"}, {"missing_session", "session globals are not set"}, {"index", "index out of range"}, {"identity_limit", "DefIndex:"}, {"identity_limit", "SyntaxContext:"}, {"context_limit", "context exceeds hard limit"}, {"debug_assert", "lo <= hi"}, {"debug_assert", "SpanField:"}} {
		if strings.Contains(text, item[1]) {
			return item[0]
		}
	}
	return "unclassified:" + text
}
func spanReplayCatch(call func() any) (result any, panicClass any) {
	defer func() {
		if value := recover(); value != nil {
			result = nil
			panicClass = spanReplayPanic(value)
		}
	}()
	result = call()
	return
}

type spanReplayHash struct {
	engine         *spanReplay
	count, panicAt int
}

func (h *spanReplayHash) WriteU64(value uint64) {
	h.engine.event(fmt.Sprintf("hash:u64:%016x", value))
	count := h.count
	h.count++
	if count == h.panicAt {
		panic("span oracle hash panic")
	}
}
func spanReplayData(data SpanData) []any {
	var parent any
	if p, ok := data.Parent.Get(); ok {
		parent = p.LocalDefIndex.AsU32()
	}
	return []any{uint32(data.Lo), uint32(data.Hi), data.Ctxt.AsU32(), parent}
}

func spanReplayOptions(pattern int) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	width := func(n uint) { o.Width = n; o.HasWidth = true }
	switch pattern {
	case 0:
	case 1:
		o.Alternate = true
	case 2:
		o.DebugLowerHex = true
	case 3:
		o.DebugUpperHex = true
	case 4:
		o.Alternate = true
		o.DebugLowerHex = true
	case 5:
		o.SignPlus = true
		o.ZeroPad = true
		width(8)
	case 6:
		o.DebugLowerHex = true
		o.ZeroPad = true
		width(8)
	case 7:
		o.Fill = '*'
		o.Align = rustfmt.AlignRight
		width(12)
	case 8:
		o.Fill = '*'
		o.Align = rustfmt.AlignLeft
		width(12)
	case 9:
		o.Fill = '*'
		o.Align = rustfmt.AlignCenter
		width(13)
	case 10:
		o.HasPrecision = true
		o.Precision = 2
	case 11:
		o.Alternate = true
		o.SignPlus = true
		o.ZeroPad = true
		o.DebugLowerHex = true
		width(8)
	case 12:
		o.SignMinus = true
		o.ZeroPad = true
		width(8)
	case 13:
		o.Fill = '🦀'
		o.Align = rustfmt.AlignCenter
		width(12)
	case 14:
		o.Fill = '0'
		o.Align = rustfmt.AlignRight
		width(12)
	case 15:
		o.Fill = '\n'
		o.Align = rustfmt.AlignRight
		width(8)
	case 16:
		o.Fill = 0
		o.Align = rustfmt.AlignRight
		width(8)
	case 17:
		o.Alternate = true
		o.ZeroPad = true
		width(8)
	case 18:
		o.Alternate = true
		o.HasPrecision = true
		o.Precision = 2
	default:
		panic("invalid oracle pattern")
	}
	return o
}

var errSpanReplayWriter = errors.New("selected span writer failure")

type spanReplayWrite struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}
type spanReplaySink struct {
	engine   *spanReplay
	writes   []spanReplayWrite
	accepted strings.Builder
	fail     int
}

func (w *spanReplaySink) write(kind, text string) error {
	index := len(w.writes)
	w.engine.event("write:" + kind + ":" + hex.EncodeToString([]byte(text)))
	w.writes = append(w.writes, spanReplayWrite{kind, text})
	if index == w.fail {
		return errSpanReplayWriter
	}
	w.accepted.WriteString(text)
	return nil
}
func (w *spanReplaySink) WriteString(text string) (int, error) {
	if err := w.write("S", text); err != nil {
		return 0, err
	}
	return len(text), nil
}
func (w *spanReplaySink) Write(text []byte) (int, error) { return w.WriteString(string(text)) }

type spanReplayCharSink struct{ *spanReplaySink }

func (w spanReplayCharSink) WriteChar(char rune) error { return w.write("C", string(char)) }
func spanReplayFormatState(options rustfmt.Options) []any {
	return []any{uint32(options.Fill), uint8(options.Align)}
}
func (e *spanReplay) debug(span Span, op spanOracleOp) any {
	fail := -1
	if value, ok := op["fail"]; ok {
		fail = int(spanReplayNumber(value))
	}
	sink := &spanReplaySink{engine: e, fail: fail, writes: make([]spanReplayWrite, 0)}
	var writer io.Writer = sink
	if spanReplayBool(op["chars"]) {
		writer = spanReplayCharSink{sink}
	}
	f := &rustfmt.Formatter{Writer: writer, Options: spanReplayOptions(spanReplayIndex(op, "pattern"))}
	before := spanReplayFormatState(f.Options)
	var first error
	switch spanReplayString(op["kind"]) {
	case "", "span":
		first = span.WriteDebugFormatter(e.session, f, e.config)
	case "ord":
		first = (OrdSpan{Span: span}).WriteDebugFormatter(e.session, f, e.config)
	case "data":
		data := span.DataUntracked(e.session, e.config)
		first = data.WriteDebugFormatter(e.session, f, e.config)
	default:
		panic("invalid oracle debug target")
	}
	after := spanReplayFormatState(f.Options)
	var second error
	if spanReplayBool(op["reuse"]) {
		second = rustfmt.FormatDebugUint(26, uint(strconv.IntSize), f)
	}
	return map[string]any{"ok": first == nil && second == nil, "first_ok": first == nil, "reuse_ok": second == nil, "writes": sink.writes, "accepted": sink.accepted.String(), "state": map[string]any{"before": before, "after": after, "final": spanReplayFormatState(f.Options)}}
}

type spanReplayOwnedEnvironment struct {
	count int
	token bool
}

func (e *spanReplay) owned(span Span, op spanOracleOp) (result Span) {
	context := rustfmt.Context{}
	switch spanReplayString(op["ambient"]) {
	case "normal", "":
	case "unwinding":
		context.Unwinding = true
	case "processing_panic":
		context.ProcessingPanic = true
	default:
		panic("invalid owned context")
	}
	held := false
	mapper := NewCtxtMapper(spanReplayOwnedEnvironment{token: true}, func(env *spanReplayOwnedEnvironment, old SyntaxContext, runtime rustfmt.Context) SyntaxContext {
		e.event(fmt.Sprintf("map_call:%d:%d:%v", old.AsU32(), env.count, runtime.Panicking()))
		env.count++
		if spanReplayBool(op["move_token"]) {
			held = env.token
			env.token = false
			e.event("move_token")
		}
		if spanReplayBool(op["call_panic"]) {
			runtime.Panic("span oracle map panic")
		}
		return SyntaxContextFromU32(spanReplayU32(op["to"]))
	}, func(env *spanReplayOwnedEnvironment, runtime rustfmt.Context) {
		defer func() {
			failure := recover()
			fieldContext := runtime
			if failure != nil {
				fieldContext.Unwinding = true
			}
			if env.token {
				env.token = false
				e.event(fmt.Sprintf("token_drop:%v", fieldContext.Panicking()))
			}
			if failure != nil {
				runtime.Panic(failure)
			}
		}()
		e.event(fmt.Sprintf("map_drop:%d:%v:%v", env.count, env.token, runtime.Panicking()))
		if spanReplayBool(op["drop_panic"]) {
			runtime.Panic("span oracle drop panic")
		}
	}, context)
	var failure any
	func() {
		defer func() { failure = recover() }()
		result = MapCtxtOwned(span, e.session, mapper, e.config)
	}()
	if held {
		held = false
		e.event(fmt.Sprintf("token_drop:%v", context.Panicking()))
	}
	if failure != nil {
		context.Panic(failure)
	}
	return result
}
func (e *spanReplay) operation(op spanOracleOp) any {
	name := spanReplayString(op["op"])
	at := func() Span { return e.slots[spanReplayIndex(op, "span")] }
	var produced Span
	isSpan := true
	switch name {
	case "new":
		produced = e.newFields(op, false)
	case "new_ordered":
		produced = e.newFields(op, true)
	case "raw":
		bits, err := strconv.ParseUint(spanReplayString(op["bits"]), 16, 64)
		if err != nil {
			panic(err)
		}
		produced = SpanFromRawSpan(datastructures.RawSpan{Value: bits})
	case "dummy":
		produced = DummySpan()
	case "clone":
		produced = DummySpan()
		produced.CloneFrom(at())
	case "ord_clone":
		value := OrdSpan{Span: DummySpan()}
		value.CloneFrom(OrdSpan{Span: at()})
		produced = value.Span
	case "inline_try":
		var ok bool
		produced, ok = InlineCtxtTryNewSpan(spanReplayU32(op["lo"]), spanReplayU32(op["len"]), spanReplayU32(op["ctxt"]))
		if !ok {
			return nil
		}
	case "map_ctxt":
		produced = at().MapCtxt(e.session, func(old SyntaxContext) SyntaxContext {
			e.event(fmt.Sprintf("map:%d", old.AsU32()))
			if spanReplayBool(op["panic"]) {
				panic("span oracle map panic")
			}
			return SyntaxContextFromU32(spanReplayU32(op["to"]))
		}, e.config)
	case "map_owned":
		produced = e.owned(at(), op)
	case "with_parent":
		parent := spanReplayParent(op["parent"])
		produced = at().WithParent(e.session, parent, e.config)
	default:
		isSpan = false
	}
	if isSpan {
		e.slots = append(e.slots, produced)
		return spanReplayRaw(produced)
	}
	switch name {
	case "inline_from":
		value, ok := InlineCtxtTryFromSpan(at())
		if !ok {
			return nil
		}
		return []uint32{value.Lo, value.Len, value.Ctxt}
	case "data":
		return spanReplayData(at().Data(e.session, e.config))
	case "data_untracked":
		return spanReplayData(at().DataUntracked(e.session, e.config))
	case "ctxt":
		return at().Ctxt().AsU32()
	case "parent":
		if parent, ok := at().Parent(e.session).Get(); ok {
			return parent.LocalDefIndex.AsU32()
		}
		return nil
	case "is_dummy":
		return at().IsDummy(e.session)
	case "from_expansion":
		return at().FromExpansion()
	case "ord_eq":
		return (OrdSpan{Span: at()}).Equal(e.session, OrdSpan{Span: e.slots[spanReplayIndex(op, "other")]}, e.config)
	case "ord_ne":
		return (OrdSpan{Span: at()}).NotEqual(e.session, OrdSpan{Span: e.slots[spanReplayIndex(op, "other")]}, e.config)
	case "ord_cmp":
		return (OrdSpan{Span: at()}).Compare(e.session, OrdSpan{Span: e.slots[spanReplayIndex(op, "other")]}, e.config)
	case "ord_partial_cmp":
		value, _ := (OrdSpan{Span: at()}).PartialCompare(e.session, OrdSpan{Span: e.slots[spanReplayIndex(op, "other")]}, e.config)
		return map[string]any{"some": value}
	case "hash":
		index := -1
		if value, ok := op["panic_at"]; ok {
			index = int(spanReplayNumber(value))
		}
		at().Hash(&spanReplayHash{engine: e, panicAt: index})
		return nil
	case "debug":
		return e.debug(at(), op)
	default:
		panic("unimplemented Go oracle op:" + name)
	}
}
func spanReplayExecute(request spanOracleRequest, live bool) spanOracleResponse {
	e := &spanReplay{request: request, config: SpanEncodingConfig{request.Debug, request.Overflow}, live: live, slots: make([]Span, 0)}
	if request.Session {
		if request.SourceMap != nil {
			e.session = NewSpanEncodingSessionWithSourceMap(e.config, spanReplaySourceMap{e})
		} else {
			e.session = NewSpanEncodingSession(e.config)
		}
	}
	previous := SwapSpanTrack(e.track)
	defer SwapSpanTrack(previous)
	response := spanOracleResponse{Steps: make([]spanOracleStep, 0)}
	for index, op := range request.Ops {
		e.events = make([]string, 0)
		e.tracks = 0
		if live {
			data, _ := json.Marshal(map[string]any{"op": index, "interner": e.interner(), "slots": e.rawSlots()})
			fmt.Printf("PROBE_START\t%s\n", data)
		}
		result, panicClass := spanReplayCatch(func() any { return e.operation(op) })
		step := spanOracleStep{result, panicClass, e.events, e.interner(), e.rawSlots()}
		response.Steps = append(response.Steps, step)
		if live {
			data, _ := json.Marshal(step)
			fmt.Printf("PROBE_STEP\t%s\n", data)
		}
	}
	return response
}

func spanOracleJSON(data []byte) any {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		panic(err)
	}
	return value
}
func spanOracleCanonical(value any) any {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return spanOracleJSON(data)
}

func TestSpanOracleAbortChild(t *testing.T) {
	encoded, present := os.LookupEnv("FERRUGO_SPAN_ABORT_REQUEST")
	if !present {
		t.Skip("abort child not requested")
	}
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	var request spanOracleRequest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&request); err != nil {
		t.Fatal(err)
	}
	spanReplayExecute(request, true)
}
func spanOracleAbortResponse(t *testing.T, request spanOracleRequest) any {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestSpanOracleAbortChild$")
	cmd.Env = append(os.Environ(), "FERRUGO_SPAN_ABORT_REQUEST="+base64.StdEncoding.EncodeToString(data))
	output, err := cmd.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 134 {
		t.Fatalf("%s abort status%v output%s", request.ID, err, output)
	}
	steps := make([]any, 0)
	events := make([]string, 0)
	var observed map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 4096), 16<<20)
	for scanner.Scan() {
		prefix, text, ok := strings.Cut(scanner.Text(), "\t")
		if !ok {
			t.Fatalf("unexpected abort output%q", scanner.Text())
		}
		switch prefix {
		case "PROBE_START":
			observed = spanOracleJSON([]byte(text)).(map[string]any)
			events = make([]string, 0)
		case "PROBE_EVENT":
			events = append(events, spanOracleJSON([]byte(text)).(string))
		case "PROBE_STEP":
			steps = append(steps, spanOracleJSON([]byte(text)))
		default:
			t.Fatalf("unexpected abort prefix%q", prefix)
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if observed == nil {
		t.Fatal("abort omitted pre-operation observation")
	}
	return map[string]any{"steps": steps, "abort": map[string]any{"op": observed["op"], "class": "abort", "events": events, "last_observed": map[string]any{"interner": observed["interner"], "slots": observed["slots"]}}}
}

func TestOriginalSpanEncoding(t *testing.T) {
	manifestBytes, err := os.ReadFile("../../../../../tools/port-rust-span-encoding/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Revision      string `json:"revision"`
		Requests      int    `json:"requests"`
		Steps         int    `json:"steps"`
		Aborts        int    `json:"aborts"`
		FixtureSHA256 string `json:"fixture_sha256"`
		RawSHA256     string `json:"raw_sha256"`
		BuilderSHA256 string `json:"request_builder_sha256"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || manifest.Requests != 15264 || manifest.Steps != 142168 || manifest.Aborts != 64 {
		t.Fatal("unexpected original-source corpus identity or coverage")
	}
	builder, err := os.ReadFile("span_requests_test.go")
	if err != nil {
		t.Fatal(err)
	}
	if actual := fmt.Sprintf("%x", sha256.Sum256(builder)); actual != manifest.BuilderSHA256 {
		t.Fatal("request builder changed without original-source corpus regeneration")
	}
	file, err := os.Open("span_testdata/cases.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	compressed, err := io.ReadAll(io.LimitReader(file, (4<<20)+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(compressed) > 4<<20 {
		t.Fatal("original-source fixture exceeds compressed size bound")
	}
	if actual := fmt.Sprintf("%x", sha256.Sum256(compressed)); actual != manifest.FixtureSHA256 {
		t.Fatal("original-source compressed fixture checksum differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	rawHash := sha256.New()
	// Reading through EOF also checks gzip's CRC/trailer. Bound decompression so
	// accidental replacement by an oversized fixture fails before replay grows.
	raw := io.LimitReader(io.TeeReader(reader, rawHash), (64<<20)+1)
	scanner := bufio.NewScanner(raw)
	scanner.Buffer(make([]byte, 64<<10), 32<<20)
	count, steps, aborts, rawBytes := 0, 0, 0, 0
	for scanner.Scan() {
		rawBytes += len(scanner.Bytes()) + 1
		if rawBytes > 64<<20 {
			t.Fatal("original-source fixture exceeds decompressed size bound")
		}
		var record struct {
			Request  spanOracleRequest `json:"request"`
			Response json.RawMessage   `json:"response"`
		}
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.UseNumber()
		if err := decoder.Decode(&record); err != nil {
			t.Fatal(err)
		}
		wanted := spanOracleJSON(record.Response)
		steps += len(wanted.(map[string]any)["steps"].([]any))
		var actual any
		if record.Request.Abort {
			actual = spanOracleCanonical(spanOracleAbortResponse(t, record.Request))
			aborts++
		} else {
			response := spanReplayExecute(record.Request, false)
			actual = spanOracleCanonical(response)
		}
		if !reflect.DeepEqual(actual, wanted) {
			actualData, _ := json.Marshal(actual)
			t.Fatalf("%s profile debug=%v overflow=%v session=%v\nGo:%s\nRust:%s", record.Request.ID, record.Request.Debug, record.Request.Overflow, record.Request.Session, actualData, record.Response)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if actual := fmt.Sprintf("%x", rawHash.Sum(nil)); actual != manifest.RawSHA256 {
		t.Fatal("original-source decompressed fixture checksum differs")
	}
	if count != manifest.Requests || steps != manifest.Steps || aborts != manifest.Aborts {
		t.Fatalf("corpus coverage differs: requests=%d steps=%d aborts=%d", count, steps, aborts)
	}
	t.Logf("%d original requests,%d completed steps,%d native abort records", count, steps, aborts)
}
