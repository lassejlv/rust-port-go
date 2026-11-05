package rustcspan

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	rustchashes "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_hashes/src"
	rustcserialize "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_serialize/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

type libPanicObservation struct {
	lines   *FreezeLock[SourceFileLines]
	reentry []string
}

type libShortRawDecoder struct{ *SpanMemDecoder }

func (value *libShortRawDecoder) ReadRawBytes(n uint) []byte {
	raw := value.SpanMemDecoder.ReadRawBytes(n)
	if n == 1 {
		return raw[:0]
	}
	return raw
}

func libPanicAction(op string, ctx rustfmt.Context, p SpanEncodingConfig, observation *libPanicObservation) {
	source := func() *SourceFile {
		v, err := NewSourceFileWithContext(AnonFileName(rustchashes.ZeroHash64()), "hi", SourceFileHashMd5, None[SourceFileHashAlgorithm](), ctx, p)
		if err != nil {
			panic(err)
		}
		return &v
	}
	switch op {
	case "decode_remap_bits", "decode_filename_tag", "decode_hashalgorithm_tag", "decode_hash_len", "decode_option_tag":
		body := []byte{255}
		switch op {
		case "decode_remap_bits":
			body = []byte{4}
		case "decode_hash_len":
			body = []byte{0, 31}
		case "decode_option_tag":
			body = []byte{2}
		}
		d, err := NewSpanMemDecoderWithContext(nil, append(body, []byte(rustcserialize.MagicEndBytes)...), 0, ctx, p)
		if err != nil {
			panic(err)
		}
		d.SetRuntimeContext(ctx)
		switch op {
		case "decode_remap_bits":
			DecodeRemapPathScopeComponents(d)
		case "decode_filename_tag":
			DecodeFileName(d)
		case "decode_hashalgorithm_tag":
			DecodeSourceFileHashAlgorithm(d)
		case "decode_hash_len":
			DecodeSourceFileHash(d)
		case "decode_option_tag":
			spanCodecOptionTag(d)
		}
	case "encode_non_utf8_path":
		e := NewSpanMemEncoderWithContext(nil, ctx, p)
		e.SetRuntimeContext(ctx)
		RealFileNameFromVirtualPath(RustPathFromUnixBytes([]byte{255})).Encode(e)
	case "original_relative_add", "normalized_cmp_add", "normalized_return_add":
		sf := source()
		defer sf.CloseWithContext(ctx)
		switch op {
		case "original_relative_add":
			sf.NormalizedPos = []NormalizedPos{{Pos: 0, Diff: ^uint32(0)}}
			sf.OriginalRelativeBytePosWithContext(1, ctx, p)
		case "normalized_cmp_add":
			sf.NormalizedPos = []NormalizedPos{{Pos: RelativeBytePos(^uint32(0)), Diff: 1}}
			sf.NormalizedBytePosWithContext(0, ctx, p)
		case "normalized_return_add":
			sf.StartPos = BytePos(^uint32(0))
			sf.NormalizedBytePosWithContext(1, ctx, p)
		}
	case "multibyte_zero", "multibyte_end_add":
		sf := source()
		defer sf.CloseWithContext(ctx)
		sf.MultibyteChars = []MultiByteChar{{Pos: 0, Bytes: 0}}
		pos := BytePos(1)
		if op == "multibyte_end_add" {
			sf.MultibyteChars = []MultiByteChar{{Pos: RelativeBytePos(^uint32(0) - 1), Bytes: 4}}
			pos = BytePos(^uint32(0))
		}
		sf.LookupFilePosWithColDisplayWithContext(pos, ctx, p)
	case "split_debug", "split_add":
		session := NewSessionGlobalsWithContext(DefaultEdition, nil, None[SourceMapInputs](), ctx, p)
		defer spanCloseOwnedCallback(ctx, session.CloseWithContext)
		span := WithRootCtxt(session, 1, 3)
		if op == "split_add" {
			span = WithRootCtxt(session, BytePos(^uint32(0)-1), BytePos(^uint32(0)))
		}
		span.SplitAtWithContext(session, 3, ctx, p)
	case "diff_short_raw":
		sf, err := NewSourceFileWithContext(AnonFileName(rustchashes.ZeroHash64()), "a\nb", SourceFileHashMd5, None[SourceFileHashAlgorithm](), ctx, p)
		if err != nil {
			panic(err)
		}
		defer sf.CloseWithContext(ctx)
		e := NewSpanMemEncoderWithContext(nil, ctx, p)
		sf.Encode(e)
		frame := append(e.Finish(), []byte(rustcserialize.MagicEndBytes)...)
		d, err := NewSpanMemDecoderWithContext(nil, frame, 0, ctx, p)
		if err != nil {
			panic(err)
		}
		d.SetRuntimeContext(ctx)
		decoded := DecodeSourceFile(&libShortRawDecoder{d})
		defer decoded.CloseWithContext(ctx)
		decoded.LinesWithContext(ctx, p)
	case "encode_diffs", "encode_first_nonzero":
		sf := source()
		defer sf.CloseWithContext(ctx)
		sf.Lines.CloseWithContext(ctx)
		if op == "encode_diffs" {
			sf.Lines = NewFreezeLock(DiffsSourceFileLines(NewSourceFileDiffs(1, 1, []byte{1})))
		} else {
			sf.Lines = FrozenFreezeLock(LinesSourceFileLines([]RelativeBytePos{1}))
		}
		observation.lines = sf.Lines
		defer func() { observation.lines = nil }()
		sf.EncodeWithContext(NewSpanMemEncoderWithContext(nil, ctx, p), ctx)
	case "mem_expn", "mem_ctxt", "mem_index", "guaranteed_encode":
		e := NewSpanMemEncoderWithContext(nil, ctx, p)
		e.SetRuntimeContext(ctx) // current borrowed operation, after construction
		switch op {
		case "mem_expn":
			e.EncodeExpnId(ExpnId{})
		case "mem_ctxt":
			e.EncodeSyntaxContext(RootSyntaxContext())
		case "mem_index":
			e.EncodeDefIndex(DefIndexFromU32(0))
		case "guaranteed_encode":
			UncheckedErrorGuaranteed().Encode(e)
		}
	case "file_expn", "file_ctxt", "file_index":
		path, err := os.CreateTemp("", "ferrugo-panic-codec-")
		if err != nil {
			panic(err)
		}
		name := path.Name()
		_ = path.Close()
		defer os.Remove(name)
		e, err := NewSpanFileEncoderWithContext(nil, NewRustPath(name), ctx, p)
		if err != nil {
			panic(err)
		}
		defer spanCloseOwnedCallback(ctx, e.CloseWithContext)
		e.SetRuntimeContext(ctx)
		switch op {
		case "file_expn":
			e.EncodeExpnId(ExpnId{})
		case "file_ctxt":
			e.EncodeSyntaxContext(RootSyntaxContext())
		case "file_index":
			e.EncodeDefIndex(DefIndexFromU32(0))
		}
	case "decode_expn", "decode_ctxt", "decode_index", "decode_attr", "guaranteed_decode":
		d, err := NewSpanMemDecoderWithContext(nil, []byte(rustcserialize.MagicEndBytes), 0, ctx, p)
		if err != nil {
			panic(err)
		}
		d.SetRuntimeContext(ctx)
		switch op {
		case "decode_expn":
			d.DecodeExpnId()
		case "decode_ctxt":
			d.DecodeSyntaxContext()
		case "decode_index":
			d.DecodeDefIndex()
		case "decode_attr":
			d.DecodeAttrId()
		case "guaranteed_decode":
			DecodeErrorGuaranteed(d)
		}
	case "path_zero", "path_multi", "embeddable_zero", "embeddable_multi":
		value := EmptyRealFileName()
		scope := EmptyRemapPathScopeComponents()
		if strings.HasSuffix(op, "multi") {
			scope = RemapPathScopeMacro | RemapPathScopeDiagnostics
		}
		if strings.HasPrefix(op, "path_") {
			value.PathWithContext(scope, ctx)
		} else {
			value.EmbeddableNameWithContext(scope, ctx)
		}
	case "def_path_default":
		DefaultSpanDefPathHasherWithContext(NewSpanMemEncoder(nil), ctx)
	case "byte_add":
		BytePos(^uint32(0)).AddWithContext(1, p, ctx)
	case "byte_sub":
		BytePos(0).SubWithContext(1, p, ctx)
	case "relative_add":
		RelativeBytePos(^uint32(0)).AddWithContext(1, p, ctx)
	case "relative_sub":
		RelativeBytePos(0).SubWithContext(1, p, ctx)
	case "char_add":
		CharPos(^uint(0)).AddWithContext(1, p, ctx)
	case "char_sub":
		CharPos(0).SubWithContext(1, p, ctx)
	case "attr_u32":
		AttrIdFromU32WithContext(^uint32(0), ctx)
	case "attr_usize":
		AttrIdFromUsizeWithContext(^uint(0), ctx)
	case "attr_add":
		AttrIdMax().AddWithContext(1, ctx, p)
	case "attr_step":
		AttrIdMax().ForwardCheckedWithContext(1, ctx)
	case "attr_forward_none":
		AttrIdFromU32(1).ForwardWithContext(^uint(0), ctx)
	case "attr_backward_none":
		AttrIdZero().BackwardWithContext(1, ctx)
	case "session_absent":
		WithSessionGlobalsOwned(NewSessionScope(), NewSpanOwned(struct{}{}, nil), func(*struct{}, *SessionGlobals, rustfmt.Context) struct{} { return struct{}{} }, ctx)
	case "session_present":
		scope := NewSessionScope()
		CreateSessionGlobalsThenOwned(scope, DefaultEdition, nil, None[SourceMapInputs](), NewSpanOwned(struct{}{}, nil), func(*struct{}, *SessionScope, rustfmt.Context) struct{} {
			return CreateSessionGlobalsThenOwned(scope, DefaultEdition, nil, None[SourceMapInputs](), NewSpanOwned(struct{}{}, nil), func(*struct{}, *SessionScope, rustfmt.Context) struct{} { return struct{}{} }, ctx, p)
		}, ctx, p)
	case "line_index":
		sf := source()
		defer sf.CloseWithContext(ctx)
		sf.LineBoundsWithContext(^uint(0), ctx, p)
	case "line_begin_missing":
		sf := source()
		defer sf.CloseWithContext(ctx)
		sf.Lines.CloseWithContext(ctx)
		sf.Lines = FrozenFreezeLock(LinesSourceFileLines(nil))
		sf.LineBeginPosWithContext(0, ctx, p)
	case "bytepos_middle", "bytepos_total_extra":
		sf := source()
		defer sf.CloseWithContext(ctx)
		sf.MultibyteChars = []MultiByteChar{{Pos: 0, Bytes: 2}}
		position := BytePos(1)
		if op == "bytepos_total_extra" {
			sf.MultibyteChars = append(sf.MultibyteChars, sf.MultibyteChars[0], sf.MultibyteChars[0])
			position = 2
		}
		sf.LookupFilePosWithColDisplayWithContext(position, ctx, p)
	case "line_local_boundary", "line_local_oob", "line_external_boundary", "line_external_oob":
		sf := source()
		defer sf.CloseWithContext(ctx)
		old, _ := sf.Src.Get()
		old.CloseWithContext(ctx)
		sf.Src = None[SharedString]()
		text := NewSharedString("é")
		if strings.HasPrefix(op, "line_local") {
			sf.Src = Some(text)
		} else {
			sf.ExternalSrc.CloseWithContext(ctx)
			sf.ExternalSrc = NewFreezeLockWithDrop(ForeignExternalSource(PresentExternalSourceKind(text), 0), true, func(e *ExternalSource, c rustfmt.Context) { e.CloseWithContext(c) })
		}
		position := uint32(1)
		if strings.HasSuffix(op, "oob") {
			position = 3
		}
		sf.Lines.CloseWithContext(ctx)
		sf.Lines = FrozenFreezeLock(LinesSourceFileLines([]RelativeBytePos{RelativeBytePos(position)}))
		sf.GetLineWithContext(0, ctx, p)
	case "absolute_overflow":
		sf := source()
		defer sf.CloseWithContext(ctx)
		sf.StartPos = BytePos(^uint32(0))
		sf.AbsolutePositionWithContext(1, ctx, p)
	case "relative_underflow":
		sf := source()
		defer sf.CloseWithContext(ctx)
		sf.StartPos = 2
		sf.RelativePositionWithContext(1, ctx, p)
	case "diff_zero", "diff_width":
		sf := source()
		defer sf.CloseWithContext(ctx)
		sf.Lines.CloseWithContext(ctx)
		width := uint(0)
		if op == "diff_width" {
			width = 3
		}
		sf.Lines = NewFreezeLock(DiffsSourceFileLines(NewSourceFileDiffs(width, 0, nil)))
		sf.LinesWithContext(ctx, p)
	case "external_state":
		sf := source()
		defer sf.CloseWithContext(ctx)
		sf.ExternalSrc.CloseWithContext(ctx)
		sf.ExternalSrc = NewFreezeLockWithDrop(UnneededExternalSource(), false, func(e *ExternalSource, ctx rustfmt.Context) { e.CloseWithContext(ctx) })
		sf.AddExternalSrcWithContext(func() Option[string] { return Some("hi") }, ctx, p)
	case "md5_overrun", "sha1_overrun", "sha256_overrun", "blake3_overrun":
		kind, ok := ParseSourceFileHashAlgorithm(strings.TrimSuffix(op, "_overrun"))
		if !ok {
			panic(op)
		}
		NewSourceFileHashFromOwnedRead(kind, NewSpanOwned(struct{}{}, nil), func(*struct{}, []byte, rustfmt.Context) (uint, error) { return 16385, nil }, ctx)
	default:
		panic(op)
	}
}
func TestLibOriginalPanicMetadata(t *testing.T) {
	selection, err := libPanicEvidencePaths(filepath.Join("..", "..", "..", "..", ".."), os.LookupEnv)
	if err != nil {
		t.Fatal(err)
	}
	expectedManifestSHA := ""
	if value, present := os.LookupEnv(libPanicManifestSHAEnv); present {
		if value == "" {
			t.Fatal("explicitly empty panic manifest hash")
		}
		expectedManifestSHA = value
	}
	manifest, err := libPanicProfileMetadata(selection, expectedManifestSHA)
	if err != nil {
		t.Fatal(err)
	}
	if selection.Explicit {
		files := make([]spanProofFile, 0, len(manifest.Files))
		for path, digest := range manifest.Files {
			files = append(files, spanProofFile{Path: path, SHA256: digest})
		}
		verifySpanProofFiles(t, files)
	}
	fixture, err := os.ReadFile(selection.Fixture)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(fixture)) != manifest.FixtureSHA {
		t.Fatal("panic producer fixture hash differs")
	}
	f, err := os.Open(selection.Fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	g, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	raw, err := io.ReadAll(io.LimitReader(g, 1<<20+1))
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 1<<20 {
		t.Fatal("panic fixture bound")
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != manifest.RawSHA {
		t.Fatal("panic producer raw hash differs")
	}
	scan := bufio.NewScanner(bytes.NewReader(raw))
	count := 0
	locations := LibPanicLocationsAtPinnedRevision("original/compiler/rustc_span/src/lib.rs", "registry/scoped-tls-1.0.1/src/lib.rs")
	for key, location := range AttrIdStepPanicLocationsAtSDK20261104("/rustc/708c94ef093ecc42fb8851e17d8383d999727abc/library/core/src/iter/range.rs") {
		locations[key] = location
	}
	for key, location := range SpanCodecPanicLocationsAtPinnedRevision("original/compiler/rustc_serialize/src/serialize.rs") {
		locations[key] = location
	}
	for scan.Scan() {
		count++
		p := strings.Split(scan.Text(), "\t")
		if len(p) != 5 || len(p[0]) != 4 || (p[0] != "d0o0" && p[0] != "d0o1" && p[0] != "d1o0" && p[0] != "d1o1") {
			t.Fatal("panic shape")
		}
		selectedLocations, expected, err := libPanicVerbatimLocations(locations, p[3])
		if err != nil {
			t.Fatal(err)
		}
		config := SpanEncodingConfig{DebugAssertions: p[0][1] == '1', OverflowChecks: p[0][3] == '1'}
		events := []string{}
		observation := &libPanicObservation{}
		ctx := rustfmt.WithPanicOperations(rustfmt.Context{}, &rustfmt.PanicOperations{Locations: selectedLocations, Hook: func(info rustfmt.SourcePanicInfo, context rustfmt.Context) {
			if !context.Panicking() || !context.ProcessingPanic {
				t.Fatal("hook ran outside processing")
			}
			if info.Location == nil {
				t.Fatal("missing explicit source profile location", p[1])
			}
			text, ok := info.Payload.(string)
			if !ok {
				t.Fatal("unexpected payload kind", p[1])
			}
			events = append(events, fmt.Sprintf("%s:%s:%s:%d:%d:%t:%t", info.Kind, hex.EncodeToString([]byte(text)), hex.EncodeToString([]byte(info.Location.File)), info.Location.Line, info.Location.Column, info.CanUnwind, info.ForceNoBacktrace))
			if observation.lines != nil {
				// Observe the actual Go lock state before invoking the blocking
				// public source operation, so a regression fails rather than hangs.
				if !observation.lines.mu.TryLock() {
					t.Error("assertion condition retained its temporary read guard")
					return
				}
				observation.lines.mu.Unlock()
				guard, ok := observation.lines.TryWriteWithContext(context)
				if ok {
					guard.Close()
				}
				observation.reentry = append(observation.reentry, strconv.FormatBool(ok))
			}
		}})
		panicked := false
		func() { defer func() { panicked = recover() != nil }(); libPanicAction(p[1], ctx, config, observation) }()
		if strconv.FormatBool(panicked) != p[2] || strings.Join(events, ",") != strings.Join(expected, ",") || strings.Join(observation.reentry, ",") != p[4] {
			t.Fatalf("%v\nGo: %t %s\nRust: %s %s", p[:2], panicked, strings.Join(events, ","), p[2], strings.Join(expected, ","))
		}
	}
	if err := scan.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 256 {
		t.Fatal(count)
	}
}
