package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type spanAmbientRequest struct {
	ID       string `json:"id"`
	Debug    bool   `json:"debug_assertions"`
	Overflow bool   `json:"overflow_checks"`
	Session  bool   `json:"session"`
	Ambient  string `json:"ambient"`
	Kind     string `json:"kind"`
	Value    string `json:"value"`
	Pretty   bool   `json:"pretty"`
	Chars    bool   `json:"chars"`
	Fail     int    `json:"fail"`
}

func spanAmbientReplay(request spanAmbientRequest, live bool) any {
	e := &spanReplay{config: SpanEncodingConfig{request.Debug, request.Overflow}, events: make([]string, 0), live: live}
	if request.Session {
		e.session = NewSpanEncodingSession(e.config)
	}
	context := rustfmt.Context{}
	switch request.Ambient {
	case "normal":
	case "unwinding":
		context.Unwinding = true
	case "processing_panic":
		context.ProcessingPanic = true
	default:
		panic("unknown ambient context")
	}
	previous := SwapSpanTrack(func(parent LocalDefId) {
		e.event(fmt.Sprintf("track:%d:%v", parent.LocalDefIndex.AsU32(), context.Panicking()))
	})
	defer SwapSpanTrack(previous)
	parent := None[LocalDefId]()
	if request.Value == "parent" {
		parent = spanTestParent(7)
	}
	var span Span
	data := SpanData{Lo: 1, Hi: 6, Ctxt: RootSyntaxContext(), Parent: parent}
	if request.Kind == "data" {
		if request.Value == "needs_interner" {
			data.Lo, data.Hi = 0, 40000
		}
	} else {
		switch request.Value {
		case "inline", "parent":
			span = NewSpan(e.session, 1, 6, RootSyntaxContext(), parent, e.config)
		case "bad_index":
			span = spanFromBits(0xe000000000000000)
		case "overflow":
			span = spanFromBits(0x00010000ffffffff)
		default:
			panic("unknown debug target")
		}
	}
	sink := &spanReplaySink{engine: e, fail: request.Fail, writes: make([]spanReplayWrite, 0)}
	var writer io.Writer = sink
	if request.Chars {
		writer = spanReplayCharSink{sink}
	}
	options := rustfmt.DefaultOptions()
	options.Alternate = request.Pretty
	f := &rustfmt.Formatter{Writer: writer, Options: options, Context: context}
	var before, after any
	var ok any
	_, failure := spanReplayCatch(func() any {
		defer func() {
			failure := recover()
			dropContext := context
			if failure != nil {
				dropContext.Unwinding = true
			}
			e.event(fmt.Sprintf("scope_drop:%v", dropContext.Panicking()))
			if failure != nil {
				context.Panic(failure)
			}
		}()
		before = spanReplayFormatState(f.Options)
		var err error
		switch request.Kind {
		case "span":
			err = span.WriteDebugFormatter(e.session, f, e.config)
		case "ord":
			err = (OrdSpan{Span: span}).WriteDebugFormatter(e.session, f, e.config)
		case "data":
			err = data.WriteDebugFormatter(e.session, f, e.config)
		default:
			panic("unknown debug kind")
		}
		ok = err == nil
		after = spanReplayFormatState(f.Options)
		return nil
	})
	return map[string]any{"ok": ok, "panic": failure, "events": e.events, "writes": sink.writes, "accepted": sink.accepted.String(), "state": map[string]any{"before": before, "after": after}, "interner": e.interner()}
}

func TestSpanAmbientDebugAbortChild(t *testing.T) {
	value, present := os.LookupEnv("FERRUGO_SPAN_AMBIENT_ABORT")
	if !present {
		t.Skip("abort child not requested")
	}
	data, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	var request spanAmbientRequest
	if err := json.Unmarshal(data, &request); err != nil {
		t.Fatal(err)
	}
	spanAmbientReplay(request, true)
}
func spanAmbientAbort(t *testing.T, request spanAmbientRequest) any {
	t.Helper()
	data, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestSpanAmbientDebugAbortChild$")
	cmd.Env = append(os.Environ(), "FERRUGO_SPAN_AMBIENT_ABORT="+base64.StdEncoding.EncodeToString(data))
	output, err := cmd.CombinedOutput()
	exit, yes := err.(*exec.ExitError)
	if !yes || exit.ExitCode() != 134 {
		t.Fatalf("%s expected abort134: %v\n%s", request.ID, err, output)
	}
	events := make([]string, 0)
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		prefix, text, found := strings.Cut(scanner.Text(), "\t")
		if !found || prefix != "PROBE_EVENT" {
			t.Fatalf("unexpected child output%q", scanner.Text())
		}
		events = append(events, spanOracleJSON([]byte(text)).(string))
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return map[string]any{"abort": "abort", "events": events}
}

func TestOriginalSpanAmbientDebug(t *testing.T) {
	var manifest struct {
		Revision   string `json:"revision"`
		Records    int    `json:"records"`
		Aborts     int    `json:"aborts"`
		Held       int    `json:"lock_held_at_abort"`
		RawSHA     string `json:"raw_sha256"`
		FixtureSHA string `json:"fixture_sha256"`
	}
	metadata, err := os.ReadFile("../../../../../tools/port-rust-span-encoding/ambient-debug/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(metadata, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.Revision != "2d20b42b883dcfc014f3c2c56081eeb6091c2179" || manifest.Records != 1344 || manifest.Aborts != 140 || manifest.Held != 48 {
		t.Fatal("unexpected ambient source/coverage identity")
	}
	encoded, err := os.ReadFile("span_testdata/ambient-debug/cases.jsonl.gz")
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded) > 4<<20 || fmt.Sprintf("%x", sha256.Sum256(encoded)) != manifest.FixtureSHA {
		t.Fatal("ambient fixture checksum/size differs")
	}
	reader, err := gzip.NewReader(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(reader, (16<<20)+1))
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	if len(raw) > 16<<20 || fmt.Sprintf("%x", sha256.Sum256(raw)) != manifest.RawSHA {
		t.Fatal("ambient raw checksum/size differs")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	count, aborts, referenceHeld := 0, 0, 0
	for scanner.Scan() {
		var record struct {
			Request       spanAmbientRequest `json:"request"`
			Response      json.RawMessage    `json:"response"`
			ReferenceLock *string            `json:"reference_abort_lock"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &record); err != nil {
			t.Fatal(err)
		}
		wanted := spanOracleJSON(record.Response)
		var actual any
		if wanted.(map[string]any)["abort"] != nil {
			actual = spanAmbientAbort(t, record.Request)
			aborts++
			if record.ReferenceLock == nil {
				t.Fatal("native abort omitted its actual lock observation")
			}
			switch *record.ReferenceLock {
			case "held":
				referenceHeld++
			case "free":
			default:
				t.Fatal("unknown native lock observation")
			}
		} else {
			if record.ReferenceLock != nil {
				t.Fatal("nonabort has a signal-time lock observation")
			}
			actual = spanAmbientReplay(record.Request, false)
		}
		if actual = spanOracleCanonical(actual); !reflect.DeepEqual(actual, wanted) {
			data, _ := json.Marshal(actual)
			t.Fatalf("%s mismatch\nGo:%s\nRust:%s", record.Request.ID, data, record.Response)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != manifest.Records || aborts != manifest.Aborts || referenceHeld != manifest.Held {
		t.Fatal("ambient observed coverage differs")
	}
	t.Logf("%d ambient Debug cases,%d native aborts,%d native-only lock-held observations", count, aborts, referenceHeld)
}
