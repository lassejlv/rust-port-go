package util

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
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	semver "github.com/lassejlv/rust-port-go/port/dependencies/semver/v1_0_28/src"
)

func cargoSemverFormattingFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := "testdata/" + name
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := os.ReadFile("../../../../tools/port-cargo-semver/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Hashes map[string]string `json:"sha256"`
	}
	if err := json.Unmarshal(metadata, &m); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != m.Hashes[path] {
		t.Fatal("format fixture compressed hash mismatch")
	}
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(io.LimitReader(r, 64<<20))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 64<<20 {
		t.Fatal("format fixture too large")
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	sum = sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != m.Hashes[path+".raw"] {
		t.Fatal("format fixture raw hash mismatch")
	}
	return raw
}

var errCargoSemverWriter = errors.New("selected writer failure")

type cargoSemverFormatSink struct {
	calls    []string
	text     strings.Builder
	fail     int
	dispatch bool
}

func (w *cargoSemverFormatSink) piece(value, label string) error {
	w.calls = append(w.calls, label)
	if w.fail != 0 && len(w.calls) == w.fail {
		return errCargoSemverWriter
	}
	w.text.WriteString(value)
	return nil
}
func (w *cargoSemverFormatSink) WriteString(value string) (int, error) {
	label := value
	if w.dispatch {
		label = "S:" + value
	}
	if err := w.piece(value, label); err != nil {
		return 0, err
	}
	return len(value), nil
}
func (w *cargoSemverFormatSink) Write(value []byte) (int, error) { return w.WriteString(string(value)) }

type cargoSemverCharSink struct{ *cargoSemverFormatSink }

func (w cargoSemverCharSink) WriteChar(value rune) error {
	return w.piece(string(value), "C:"+string(value))
}

type cargoSemverDebugValue interface {
	WriteDebug(io.Writer, semver.DebugOptions) error
	WriteDebugFormatter(*semver.DebugFormatter) error
	RustDebugWith(semver.DebugOptions) string
}

func cargoSemverDebugState(options semver.DebugOptions) string {
	fill := ' '
	if options.Fill.Present {
		fill = options.Fill.Value
	}
	align := "None"
	switch options.Align {
	case semver.AlignLeft:
		align = "Some(Left)"
	case semver.AlignRight:
		align = "Some(Right)"
	case semver.AlignCenter:
		align = "Some(Center)"
	}
	return hex.EncodeToString([]byte(string(fill))) + ":" + align
}

func cargoSemverDebugOptions(mode int) semver.DebugOptions {
	o := semver.DebugOptions{}
	switch mode {
	case 0:
	case 1:
		o.Pretty = true
	case 2:
		o.Numbers = semver.DebugLowerHex
	case 3:
		o.Numbers = semver.DebugUpperHex
	case 4:
		o.Pretty = true
		o.Numbers = semver.DebugLowerHex
	case 5:
		o.Pretty = true
		o.Numbers = semver.DebugUpperHex
	case 6:
		o.SignPlus = true
	case 7:
		o.ZeroPad = true
		o.Width = semver.Some(uint(8))
	case 8:
		o.SignPlus = true
		o.ZeroPad = true
		o.Numbers = semver.DebugLowerHex
		o.Width = semver.Some(uint(8))
	case 9:
		o.Align = semver.AlignRight
		o.Width = semver.Some(uint(8))
	case 10:
		o.Align = semver.AlignCenter
		o.Fill = semver.Some('*')
		o.Width = semver.Some(uint(9))
	case 11:
		o.Precision = semver.Some(uint(2))
	case 12:
		o.SignPlus = true
		o.Pretty = true
		o.ZeroPad = true
		o.Numbers = semver.DebugUpperHex
		o.Width = semver.Some(uint(8))
	case 13:
		o.Align = semver.AlignRight
		o.SignPlus = true
		o.Pretty = true
		o.Width = semver.Some(uint(12))
		o.Precision = semver.Some(uint(2))
		o.Numbers = semver.DebugUpperHex
	default:
		panic("bad Debug probe mode")
	}
	return o
}
func cargoSemverNamedDebugValue(t *testing.T, name string) cargoSemverDebugValue {
	req := semverUpstreamReq(t, "^10.26.31-alpha, <32")
	empty := semver.Star()
	pre, _ := semver.NewPrerelease("alpha")
	raw := semver.VersionReq{Comparators: []semver.Comparator{{Op: semver.Exact, Major: 10, Patch: semver.Some(uint64(31)), Pre: pre}}}
	full := semverUpstreamVersion(t, "18446744073709551615.26.31-alpha.0+001.build")
	plain := semverUpstreamVersion(t, "10.26.31")
	switch name {
	case "any":
		return OptVersionReq{}
	case "req_empty":
		return FromVersionReq(empty)
	case "req_multi":
		return FromVersionReq(req)
	case "req_raw":
		return FromVersionReq(raw)
	case "locked":
		return LockedVersionReq(full, req)
	case "precise":
		return PreciseVersionReq(full, req)
	case "locked_empty":
		return LockedVersionReq(plain, empty)
	case "precise_raw":
		return PreciseVersionReq(plain, raw)
	case "mode_default":
		return VersionReqDefault
	case "mode_prerelease":
		return VersionReqPrerelease
	default:
		t.Fatalf("unknown native named value %q", name)
		return nil
	}
}
func TestOriginalCargoSemverDebugWriters(t *testing.T) {
	for profile := 0; profile < 3; profile++ {
		dispatch, withState := profile > 0, profile == 2
		name := "semver_debug_probes.tsv.gz"
		if dispatch {
			name = "semver_debug_dispatch.tsv.gz"
		}
		if withState {
			name = "semver_debug_state.tsv.gz"
		}
		raw := cargoSemverFormattingFixture(t, name)
		scanner := bufio.NewScanner(bytes.NewReader(raw))
		scanner.Buffer(make([]byte, 4096), 1<<20)
		count := 0
		for scanner.Scan() {
			columns := strings.Split(scanner.Text(), "\t")
			wantedColumns := 6
			if withState {
				wantedColumns = 7
			}
			if len(columns) != wantedColumns {
				t.Fatal("bad Debug record")
			}
			mode, err := strconv.Atoi(columns[1])
			if err != nil {
				t.Fatal(err)
			}
			fail, err := strconv.Atoi(columns[2])
			if err != nil {
				t.Fatal(err)
			}
			wantError, err := strconv.ParseBool(columns[3])
			if err != nil {
				t.Fatal(err)
			}
			wanted, err := hex.DecodeString(columns[4])
			if err != nil {
				t.Fatal(err)
			}
			var calls []string
			for _, part := range strings.Split(columns[5], ",") {
				decoded, err := hex.DecodeString(part)
				if err != nil {
					t.Fatal(err)
				}
				calls = append(calls, string(decoded))
			}
			sink := &cargoSemverFormatSink{fail: fail, dispatch: dispatch}
			var writer io.Writer = sink
			if dispatch {
				writer = cargoSemverCharSink{sink}
			}
			value := cargoSemverNamedDebugValue(t, columns[0])
			formatter := &semver.DebugFormatter{Writer: writer, Options: cargoSemverDebugOptions(mode)}
			before := cargoSemverDebugState(formatter.Options)
			actualError := value.WriteDebugFormatter(formatter)
			if (actualError != nil) != wantError || actualError != nil && !errors.Is(actualError, errCargoSemverWriter) || sink.text.String() != string(wanted) || !slices.Equal(sink.calls, calls) {
				t.Fatalf("dispatch=%v %s mode%d fail%d: got text%q calls%q err%v; want text%q calls%q err%v", dispatch, columns[0], mode, fail, sink.text.String(), sink.calls, actualError, wanted, calls, wantError)
			}
			if withState {
				actualState := before + ">" + cargoSemverDebugState(formatter.Options)
				if actualState != columns[6] {
					t.Fatalf("%s mode%d fail%d: outer formatter state%q want%q", columns[0], mode, fail, actualState, columns[6])
				}
			}
			if fail == 0 && value.RustDebugWith(cargoSemverDebugOptions(mode)) != string(wanted) {
				t.Fatal("text Debug and streaming Debug differ")
			}
			count++
		}
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
		if count != 8668 {
			t.Fatalf("expected8668 Debug records, got%d", count)
		}
		t.Logf("%d native Debug writer records dispatch=%v mutableState=%v", count, dispatch, withState)
	}
}

func TestOriginalCargoSemverDisplayAndDebugOptions(t *testing.T) {
	r := cargoSemverReader{bytes.NewReader(cargoSemverFormattingFixture(t, "semver_format_cases.bin.gz")), t}
	r.magic("CGSF")
	count := r.u32()
	for i := uint32(0); i < count; i++ {
		constructor, reqText, versionText, op, fail := r.byte(), r.text(), r.text(), r.byte(), r.u32()
		wantText, wantError, calls := r.text(), r.byte() != 0, r.texts()
		var value cargoSemverDebugValue
		var optional OptVersionReq
		if constructor >= 7 {
			if constructor == 7 {
				value = VersionReqDefault
			} else {
				value = VersionReqPrerelease
			}
		} else {
			var constructorError string
			optional, _, _, constructorError = cargoSemverConstruct(cargoSemverRequest{constructor: constructor, requirement: reqText, version: versionText})
			if constructorError != "" {
				t.Fatal(constructorError)
			}
			value = optional
		}
		if op >= 9 {
			mode := map[byte]int{9: 0, 10: 2, 11: 3, 12: 1, 13: 4, 14: 5}[op]
			actual := value.RustDebugWith(cargoSemverDebugOptions(mode))
			if actual != wantText || wantError || len(calls) != 0 {
				t.Fatalf("format case%d constructor%d Debugop%d: got%q want%q", i, constructor, op, actual, wantText)
			}
			continue
		}
		options := semver.FormatOptions{}
		switch op {
		case 1, 6, 7, 8:
			options.Width = semver.Some(uint(30))
		case 2:
			options.Width = semver.Some(uint(30))
			options.Align = semver.AlignLeft
		case 3:
			options.Width = semver.Some(uint(30))
			options.Align = semver.AlignRight
		case 5:
			options.Width = semver.Some(uint(30))
			options.Align = semver.AlignCenter
			options.Fill = semver.Some('*')
		}
		sink := &cargoSemverFormatSink{fail: int(fail)}
		var display semver.Display = optional
		actualError := display.WriteDisplay(sink, options)
		if (actualError != nil) != wantError || sink.text.String() != wantText || !slices.Equal(sink.calls, calls) {
			t.Fatalf("format case%d constructor%d op%d fail%d: got%q calls%q err%v; want%q calls%q err%v", i, constructor, op, fail, sink.text.String(), sink.calls, actualError, wantText, calls, wantError)
		}
		if fail == 0 && (fmt.Sprintf("%30v", optional) != wantText || fmt.Sprintf("%.2v", optional) != wantText) {
			t.Fatal("Go formatter added padding or precision absent from source Display")
		}
	}
	if r.Len() != 0 {
		t.Fatal("trailing format fixture bytes")
	}
	t.Logf("%d original Display/Debug formatting cases", count)
}
