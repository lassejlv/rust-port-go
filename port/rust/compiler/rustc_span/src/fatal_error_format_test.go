// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

type fatalFormatSink struct {
	fail, calls int
	events      []string
	output      strings.Builder
}

func (sink *fatalFormatSink) Write(value []byte) (int, error) { return sink.WriteString(string(value)) }
func (sink *fatalFormatSink) WriteString(value string) (int, error) {
	sink.calls++
	sink.events = append(sink.events, fmt.Sprintf("S:%x", value))
	if sink.calls == sink.fail {
		return 0, errors.New("writer failure")
	}
	return sink.output.WriteString(value)
}
func (sink *fatalFormatSink) WriteChar(value rune) error {
	sink.calls++
	sink.events = append(sink.events, fmt.Sprintf("C:%x", value))
	if sink.calls == sink.fail {
		return errors.New("writer failure")
	}
	_, err := sink.output.WriteRune(value)
	return err
}
func fatalFormatOptions(style int) rustfmt.Options {
	options := rustfmt.DefaultOptions()
	switch style {
	case 0:
	case 1:
		options.Alternate = true
	case 2:
		options.HasWidth = true
		options.Width = 12
	case 3:
		options.HasWidth = true
		options.Width = 12
		options.SignPlus = true
		options.ZeroPad = true
	case 4:
		options.HasWidth = true
		options.Width = 12
		options.Align = rustfmt.AlignLeft
		options.Fill = '*'
	case 5:
		options.HasWidth = true
		options.Width = 12
		options.Align = rustfmt.AlignCenter
		options.Fill = 'λ'
		options.HasPrecision = true
		options.Precision = 3
	case 6:
		options.DebugLowerHex = true
	case 7:
		options.DebugUpperHex = true
		options.Alternate = true
	case 8:
		options.HasWidth = true
		options.Width = 12
		options.Align = rustfmt.AlignRight
		options.Fill = 0
	default:
		panic("unknown fatal format style")
	}
	return options
}
func TestFatalErrorNativeFormatterOracle(t *testing.T) {
	file, err := os.Open("testdata/fatal_error_format.tsv.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	scan := bufio.NewScanner(reader)
	records := 0
	for scan.Scan() {
		fields := strings.Split(scan.Text(), "\t")
		if len(fields) != 6 {
			t.Fatal("invalid fatal format fixture")
		}
		style, _ := strconv.Atoi(fields[1])
		fail, _ := strconv.Atoi(fields[2])
		sink := &fatalFormatSink{fail: fail}
		f := &rustfmt.Formatter{Writer: sink, Options: fatalFormatOptions(style)}
		originalOptions := f.Options
		var first error
		if fields[0] == "1" {
			first = (FatalError{}).WriteDisplayFormatter(f)
		} else {
			first = (FatalError{}).WriteDebugFormatter(f)
		}
		if f.Options != originalOptions {
			t.Fatal("fatal formatter changed caller options")
		}
		second := rustfmt.FormatDebugUint(7, 16, f)
		ok := first == nil && second == nil
		if strconv.FormatBool(ok) != fields[3] || sink.output.String() != sourceMapOracleString(fields[4]) || strings.Join(sink.events, "\n") != sourceMapOracleString(fields[5]) {
			t.Fatalf("%s: output%q calls%q", strings.Join(fields[:3], "/"), sink.output.String(), sink.events)
		}
		records++
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if (FatalError{}).Source() != nil || (FatalError{}).Error() != "fatal error" || (FatalError{}).Clone().RustDebug() != "FatalError" {
		t.Fatal("FatalError source/clone/error trait")
	}
	t.Logf("%d original native formatter cases", records)
}
