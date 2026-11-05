// SPDX-License-Identifier: MIT OR Apache-2.0
// Source Debug/Display/LowerHex implementations and DEF_ID_DEBUG atomic hook.
package rustcspan

import (
	"strings"
	"sync/atomic"

	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

// DefIdDebugCallback is an immutable cell for the source &'static fn pointer.
// Callbacks model static functions; owned destructible closure environments are
// not extra fields of that source pointer. Cell pointer identity survives swap.
type DefIdDebugCallback struct {
	call func(DefId, *rustfmt.Formatter) error
}

func NewDefIdDebugCallback(call func(DefId, *rustfmt.Formatter) error) *DefIdDebugCallback {
	if call == nil {
		panic("def_id: nil static Debug function")
	}
	return &DefIdDebugCallback{call: call}
}

var defIdDebug atomic.Pointer[DefIdDebugCallback]

func init()                               { defIdDebug.Store(NewDefIdDebugCallback(DefaultDefIdDebug)) }
func LoadDefIdDebug() *DefIdDebugCallback { return defIdDebug.Load() }
func SwapDefIdDebug(callback *DefIdDebugCallback) *DefIdDebugCallback {
	if callback == nil || callback.call == nil {
		panic("def_id: nil static Debug function")
	}
	return defIdDebug.Swap(callback)
}
func (callback *DefIdDebugCallback) Call(value DefId, f *rustfmt.Formatter) error {
	return defIdInvoke(f.Context, func() error { return callback.call(value, f) })
}

func DefaultDefIdDebug(value DefId, f *rustfmt.Formatter) error {
	pretty := f.Options.Alternate
	return rustfmt.WriteStruct(f.Writer, "DefId", pretty, []rustfmt.Field{
		{Name: "krate", Value: f.Value(pretty, value.Krate.WriteDebugFormatter)},
		{Name: "index", Value: f.Value(pretty, value.Index.WriteDebugFormatter)},
	})
}
func (value DefId) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return LoadDefIdDebug().Call(value, f)
}
func (value LocalDefId) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return value.ToDefId().WriteDebugFormatter(f)
}
func (value ModId) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return defIdTuple(f, "ModId", value.defId.WriteDebugFormatter)
}
func (value LocalModId) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return defIdTuple(f, "LocalModId", value.localDefId.WriteDebugFormatter)
}
func defIdTuple(f *rustfmt.Formatter, name string, field func(*rustfmt.Formatter) error) error {
	return rustfmt.WriteTuple(f.Writer, name, f.Options.Alternate, []rustfmt.Value{f.Value(f.Options.Alternate, field)})
}

// Macro Debug uses write!(f,"crate{}",number), so the nested number receives
// fresh default options. The outer caller's formatting flags remain unchanged.
func (value CrateNum) WriteDebugFormatter(f *rustfmt.Formatter) error {
	if err := f.WriteString("crate"); err != nil {
		return err
	}
	child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}
	return rustfmt.FormatDebugUint(uint64(value.AsU32()), 32, child)
}
func (value DefIndex) WriteDebugFormatter(f *rustfmt.Formatter) error {
	if err := f.WriteString("DefIndex("); err != nil {
		return err
	}
	child := &rustfmt.Formatter{Writer: f.Writer, Options: rustfmt.DefaultOptions(), Context: f.Context}
	if err := rustfmt.FormatDebugUint(uint64(value.AsU32()), 32, child); err != nil {
		return err
	}
	return f.WriteString(")")
}
func (value CrateNum) WriteDisplayFormatter(f *rustfmt.Formatter) error {
	return defIdIntegerFormat(uint64(value.AsU32()), 32, f, false)
}
func (value StableCrateId) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return defIdTuple(f, "StableCrateId", func(field *rustfmt.Formatter) error { return rustfmt.FormatDebugUint(value.AsU64(), 64, field) })
}
func (value StableCrateId) WriteLowerHexFormatter(f *rustfmt.Formatter) error {
	return defIdIntegerFormat(value.AsU64(), 64, f, true)
}
func defIdIntegerFormat(value uint64, bits uint, f *rustfmt.Formatter, hex bool) error {
	low, high := f.Options.DebugLowerHex, f.Options.DebugUpperHex
	f.Options.DebugLowerHex, f.Options.DebugUpperHex = hex, false
	defer func() { f.Options.DebugLowerHex, f.Options.DebugUpperHex = low, high }()
	return rustfmt.FormatDebugUint(value, bits, f)
}
func (value DefPathHash) WriteDebugFormatter(f *rustfmt.Formatter) error {
	return defIdTuple(f, "DefPathHash", func(field *rustfmt.Formatter) error {
		options := rustchashes.FormatOptions{Width: field.Options.Width, Alignment: rustchashes.Alignment(field.Options.Align), Fill: field.Options.Fill, FillSet: true, Plus: field.Options.SignPlus, Alternate: field.Options.Alternate, ZeroPad: field.Options.ZeroPad}
		if !field.Options.HasWidth {
			options.Width = 0
		}
		if field.Options.DebugLowerHex {
			options.DebugRadix = rustchashes.DebugLowerHex
		} else if field.Options.DebugUpperHex {
			options.DebugRadix = rustchashes.DebugUpperHex
		}
		numeric := rustchashes.NumericFormatter{Writer: field.Writer, Options: options}
		// The concrete delegated Fingerprint code uses this mutable numeric
		// formatter. Carry its error-path zero-padding mutations back to the
		// caller's actual formatter; pretty children already own copied state.
		defer func() {
			field.Options.Fill = numeric.Options.Fill
			field.Options.Align = rustfmt.Alignment(numeric.Options.Alignment)
		}()
		return value.Fingerprint.WriteDebug(&numeric)
	})
}

func defIdDebugString(context rustfmt.Context, format func(*rustfmt.Formatter) error) string {
	var output strings.Builder
	f := rustfmt.Formatter{Writer: &output, Options: rustfmt.DefaultOptions(), Context: context}
	if err := format(&f); err != nil {
		defIdRaise(context, DefIdFormatErrorPanicSite, "a formatting trait implementation returned an error when the underlying stream did not: Error", rustfmt.PanicOwnedString)
	}
	return output.String()
}
func (value DefId) RustDebug() string {
	return defIdDebugString(rustfmt.Context{}, value.WriteDebugFormatter)
}
func (value LocalDefId) RustDebug() string {
	return defIdDebugString(rustfmt.Context{}, value.WriteDebugFormatter)
}
func (value ModId) RustDebug() string {
	return defIdDebugString(rustfmt.Context{}, value.WriteDebugFormatter)
}
func (value LocalModId) RustDebug() string {
	return defIdDebugString(rustfmt.Context{}, value.WriteDebugFormatter)
}
func (value CrateNum) RustDebug() string {
	return defIdDebugString(rustfmt.Context{}, value.WriteDebugFormatter)
}
func (value CrateNum) String() string {
	return defIdDebugString(rustfmt.Context{}, value.WriteDisplayFormatter)
}
func (value DefIndex) RustDebug() string {
	return defIdDebugString(rustfmt.Context{}, value.WriteDebugFormatter)
}
func (value DefPathHash) RustDebug() string {
	return defIdDebugString(rustfmt.Context{}, value.WriteDebugFormatter)
}
func (value StableCrateId) RustDebug() string {
	return defIdDebugString(rustfmt.Context{}, value.WriteDebugFormatter)
}
func (value DefId) ExpectLocal() LocalDefId { return value.ExpectLocalWithContext(rustfmt.Context{}) }
func (value DefId) ExpectLocalWithContext(context rustfmt.Context) LocalDefId {
	return value.expectLocalWithSite(context, DefIdExpectLocalPanicSite)
}
func (value DefId) expectLocalWithSite(context rustfmt.Context, site string) LocalDefId {
	if local, ok := value.AsLocal().Get(); ok {
		return local
	}
	// The source panic's fmt::Arguments are formatted after panic processing
	// starts, before its hook runs. A new panic during that formatting aborts;
	// fmt::Error instead preserves the best-effort partial panic message.
	context.PanicFormattedAt(rustfmt.SourcePanicInfo{Kind: rustfmt.PanicOwnedString, Location: context.SourcePanicLocation(site), CanUnwind: true}, func(processing rustfmt.Context) any {
		var output strings.Builder
		output.WriteString("DefId::expect_local: `")
		f := rustfmt.Formatter{Writer: &output, Options: rustfmt.DefaultOptions(), Context: processing}
		err := defIdInvoke(processing, func() error { return value.WriteDebugFormatter(&f) })
		if err == nil {
			output.WriteString("` isn't local")
		}
		return output.String()
	})
	return LocalDefId{}
}
func (value ModId) ExpectLocal() LocalModId { return value.ExpectLocalWithContext(rustfmt.Context{}) }
func (value ModId) ExpectLocalWithContext(context rustfmt.Context) LocalModId {
	return NewLocalModIdUnchecked(value.defId.expectLocalWithSite(context, ModIdExpectLocalPanicSite))
}
