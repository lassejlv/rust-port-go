package contract

import (
	"fmt"
	hash "github.com/lassejlv/rust-port-go/port/dependencies/rustc-stable-hash/v0_1_2/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"io"
	"strconv"
	"strings"
)

type FormatRow struct {
	Name     string   `json:"name"`
	Mode     int      `json:"mode"`
	Dispatch bool     `json:"dispatch"`
	Fail     int      `json:"fail"`
	Error    bool     `json:"error"`
	Accepted string   `json:"accepted"`
	Calls    []string `json:"calls"`
	State    string   `json:"state"`
}
type debugSink struct {
	calls    []string
	accepted strings.Builder
	fail     int
	dispatch bool
}

func (w *debugSink) Write(p []byte) (int, error) { return w.WriteString(string(p)) }
func (w *debugSink) WriteString(s string) (int, error) {
	call := s
	if w.dispatch {
		call = "S:" + s
	}
	w.calls = append(w.calls, call)
	if w.fail != 0 && len(w.calls) == w.fail {
		return 0, io.ErrClosedPipe
	}
	return w.accepted.WriteString(s)
}
func (w *debugSink) WriteChar(c rune) error {
	if !w.dispatch {
		_, e := w.WriteString(string(c))
		return e
	}
	w.calls = append(w.calls, "C:"+string(c))
	if w.fail != 0 && len(w.calls) == w.fail {
		return io.ErrClosedPipe
	}
	_, e := w.accepted.WriteString(string(c))
	return e
}
func FormatOptions(mode int) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	switch mode {
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
		o.Alternate = true
		o.DebugUpperHex = true
	case 6:
		o.SignPlus = true
	case 7:
		o.HasWidth = true
		o.Width = 8
		o.ZeroPad = true
	case 8:
		o.HasWidth = true
		o.Width = 8
		o.ZeroPad = true
		o.SignPlus = true
		o.DebugLowerHex = true
	case 9:
		o.HasWidth = true
		o.Width = 8
		o.Align = rustfmt.AlignRight
	case 10:
		o.HasWidth = true
		o.Width = 9
		o.Align = rustfmt.AlignCenter
		o.Fill = '*'
	case 11:
		o.HasPrecision = true
		o.Precision = 2
	case 12:
		o.Alternate = true
		o.HasWidth = true
		o.Width = 8
		o.ZeroPad = true
		o.SignPlus = true
		o.DebugUpperHex = true
	case 13:
		o.Alternate = true
		o.HasWidth = true
		o.Width = 12
		o.HasPrecision = true
		o.Precision = 2
		o.Align = rustfmt.AlignRight
		o.SignPlus = true
		o.DebugUpperHex = true
	case 14:
		o.Alternate = true
		o.HasWidth = true
		o.Width = 12
		o.Align = rustfmt.AlignCenter
		o.Fill = '\n'
	case 15:
		o.HasWidth = true
		o.Width = 12
		o.Align = rustfmt.AlignLeft
		o.Fill = '\n'
	default:
		panic("bad mode")
	}
	return o
}
func formatState(o rustfmt.Options) string {
	return string(o.Fill) + ":" + []string{"None", "Some(Left)", "Some(Right)", "Some(Center)"}[o.Align]
}
func ObserveFormat(row FormatRow) FormatRow {
	var value interface {
		WriteDebugFormatter(*rustfmt.Formatter) error
	}
	if row.Name == "hash_small" {
		value = hash.SipHasher128Hash{10, 26}
	} else {
		parts := strings.Split(row.Name, "_")
		n, e := strconv.Atoi(parts[1])
		if e != nil {
			panic(e)
		}
		h := hash.NewSipHasher128WithKeys(1, 2)
		bytes := make([]byte, n)
		for i := range bytes {
			bytes[i] = byte(i*17 + 3)
		}
		h.Write(bytes)
		switch parts[0] {
		case "raw":
			value = h
		case "stable":
			value = hash.WithHasher(h, sipReferenceOps(hash.DefaultProfile()))
		case "hash":
			value = h.Finish128()
		default:
			panic("bad value")
		}
	}
	sink := &debugSink{calls: []string{}, fail: row.Fail, dispatch: row.Dispatch}
	options := FormatOptions(row.Mode)
	f := rustfmt.Formatter{Writer: sink, Options: options}
	e := value.WriteDebugFormatter(&f)
	if e != nil && e != io.ErrClosedPipe {
		panic(fmt.Sprintf("wrong writer failure%v", e))
	}
	row.Error = e != nil
	row.Accepted = sink.accepted.String()
	row.Calls = sink.calls
	row.State = formatState(options) + ">" + formatState(f.Options)
	return row
}
