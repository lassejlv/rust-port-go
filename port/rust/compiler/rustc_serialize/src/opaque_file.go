// SPDX-License-Identifier: MIT OR Apache-2.0
// Genuine FileEncoder algorithm from pinned opaque.rs. This is a scoped
// dependency boundary, not a complete rustc_serialize source-file port.
package rustcserialize

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

const fileBufferSize = 64 * 1024

type FileEncoder struct {
	buf               [fileBufferSize]byte
	buffered, flushed uint
	file              *os.File
	result            error
	path              string
	strategy          FileFlushStrategy
	finished, closed  bool
	context           rustfmt.Context
	profile           Profile
}

func (*FileEncoder) OpaqueContiguousBytes() bool { return true }

type FileEncodeError struct {
	Path       string
	ErrorValue error
}

func (e *FileEncodeError) Error() string { return e.Path + ": " + e.ErrorValue.Error() }
func (e *FileEncodeError) Unwrap() error { return e.ErrorValue }
func NewFileEncoder(path string) (*FileEncoder, error) {
	return NewFileEncoderWithContext(path, rustfmt.Context{}, DefaultProfile())
}
func NewFileEncoderWithContext(path string, context rustfmt.Context, profile Profile) (*FileEncoder, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		return nil, err
	}
	return &FileEncoder{file: file, path: path, context: contextGuardsOnly(context), profile: profile}, nil
}
func NewFileEncoderWithFlushStrategy(path string, strategy func([]byte)) (*FileEncoder, error) {
	return NewFileEncoderWithBorrowedFlushStrategy(path, FileFlushStrategy{call: func(data []byte, _ rustfmt.Context) { strategy(data) }}, rustfmt.Context{}, DefaultProfile())
}

// FileFlushStrategy borrows one actual FnMut state cell for the encoder's
// lifetime. It owns no closure environment; its caller retains exclusive
// source borrowing and destroys that environment after the encoder is gone.
type FileFlushStrategy struct{ call func([]byte, rustfmt.Context) }

func BorrowFileFlushStrategy[S any](state *S, call func(*S, []byte, rustfmt.Context)) FileFlushStrategy {
	if state == nil || call == nil {
		panic("rustcserialize: missing borrowed flush strategy")
	}
	return FileFlushStrategy{call: func(data []byte, ctx rustfmt.Context) { call(state, data, ctx) }}
}
func NewFileEncoderWithBorrowedFlushStrategy(path string, strategy FileFlushStrategy, context rustfmt.Context, profile Profile) (*FileEncoder, error) {
	encoder, err := NewFileEncoderWithContext(path, context, profile)
	if err != nil {
		return nil, err
	}
	encoder.strategy = strategy
	return encoder, nil
}
func (e *FileEncoder) callFlushStrategy(data []byte) {
	if e.strategy.call == nil {
		return
	}
	defer func() {
		if failure := recover(); failure != nil {
			e.context.ResumeUnwind(failure)
		}
	}()
	e.strategy.call(data, e.context)
}
func (e *FileEncoder) RuntimeContext() rustfmt.Context           { return e.context }
func (e *FileEncoder) BuildProfile() Profile                     { return e.profile }
func (e *FileEncoder) SetRuntimeContext(context rustfmt.Context) { e.context = context }
func (e *FileEncoder) check() {
	if e.closed {
		e.context.Panic("rustcserialize: use of closed FileEncoder")
	}
}
func (e *FileEncoder) add(a, b uint, strict bool) uint {
	if e.profile.OverflowChecks && (!strict || e.profile.DebugAssertions) && b > ^uint(0)-a {
		e.context.Panic("attempt to add with overflow")
	}
	return a + b
}
func (e *FileEncoder) Position() uint { e.check(); return e.add(e.flushed, e.buffered, true) }
func (e *FileEncoder) File() *os.File { e.check(); return e.file }
func (e *FileEncoder) Path() string   { e.check(); return e.path }
func fileWriteAll(file *os.File, buf []byte) error {
	for len(buf) > 0 {
		n, err := file.Write(buf)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		buf = buf[n:]
	}
	return nil
}
func (e *FileEncoder) Flush() {
	e.check()
	if e.profile.DebugAssertions {
		e.finished = false
	}
	if e.result == nil {
		e.result = fileWriteAll(e.file, e.buf[:e.buffered])
	}
	e.flushed = e.add(e.flushed, e.buffered, false)
	e.callFlushStrategy(e.buf[:e.buffered])
	e.buffered = 0
}
func (e *FileEncoder) writeAll(data []byte) {
	e.check()
	if e.profile.DebugAssertions {
		e.finished = false
	}
	if uint(len(data)) <= fileBufferSize-e.buffered {
		copy(e.buf[e.buffered:], data)
		e.buffered = e.add(e.buffered, uint(len(data)), true)
		return
	}
	e.Flush()
	if len(data) <= fileBufferSize {
		copy(e.buf[:], data)
		e.buffered = e.add(e.buffered, uint(len(data)), false)
	} else {
		if e.result == nil {
			e.result = fileWriteAll(e.file, data)
			e.callFlushStrategy(data)
		}
		e.flushed = e.add(e.flushed, uint(len(data)), false)
	}
}
func (e *FileEncoder) WriteWith(n uint, visitor func([]byte) uint) {
	e.check()
	if n > fileBufferSize {
		e.context.Panic("FileEncoder::write_with const N exceeds BUF_SIZE")
	}
	if e.profile.DebugAssertions {
		e.finished = false
	}
	if e.buffered > fileBufferSize-n {
		e.Flush()
	}
	window := e.buf[e.buffered : e.buffered+n]
	written := visitor(window)
	if written > n {
		e.context.Panic(fmt.Sprintf("FileEncoder::write_with::<%d> cannot be used to write %d bytes", n, written))
	}
	e.buffered = e.add(e.buffered, written, true)
}
func (e *FileEncoder) WriteArray(data []byte) {
	e.WriteWith(uint(len(data)), func(dest []byte) uint { copy(dest, data); return uint(len(data)) })
}
func (e *FileEncoder) Finish() (uint, error) {
	e.writeAll([]byte(MagicEndBytes))
	e.Flush()
	if e.profile.DebugAssertions {
		e.finished = true
	}
	result := e.result
	e.result = nil
	if result != nil {
		return 0, &FileEncodeError{Path: e.path, ErrorValue: result}
	}
	return e.Position(), nil
}
func (e *FileEncoder) Close() { e.CloseWithContext(e.context) }
func (e *FileEncoder) CloseWithContext(context rustfmt.Context) {
	if e == nil || e.closed {
		return
	}
	e.closed = true
	defer func() {
		if payload := recover(); payload != nil {
			if context.ProcessingPanic {
				context.ResumeUnwind(payload)
			}
			_ = e.file.Close()
			context.ResumeUnwind(payload)
		}
		_ = e.file.Close()
	}()
	if e.profile.DebugAssertions && !context.Panicking() && !e.finished {
		context.Panic("assertion failed: self.finished")
	}
}
func (e *FileEncoder) EmitU8(v uint8) { e.WriteArray([]byte{v}) }
func (e *FileEncoder) EmitU16(v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	e.WriteArray(b[:])
}
func (e *FileEncoder) EmitU32(v uint32) { e.writeUnsigned(Uint128{Lo: uint64(v)}, 5) }
func (e *FileEncoder) EmitU64(v uint64) { e.writeUnsigned(Uint128{Lo: v}, 10) }
func (e *FileEncoder) EmitUsize(v uint) {
	e.writeUnsigned(Uint128{Lo: uint64(v)}, uint((strconv.IntSize+6)/7))
}
func (e *FileEncoder) EmitU128(v Uint128) { e.writeUnsigned(v, 19) }
func (e *FileEncoder) EmitI16(v int16)    { e.EmitU16(uint16(v)) }
func (e *FileEncoder) EmitI32(v int32) {
	e.writeSigned(Int128{Lo: uint64(int64(v)), Hi: signHi(int64(v))}, 5)
}
func (e *FileEncoder) EmitI64(v int64) { e.writeSigned(Int128{Lo: uint64(v), Hi: signHi(v)}, 10) }
func (e *FileEncoder) EmitIsize(v int) {
	e.writeSigned(Int128{Lo: uint64(int64(v)), Hi: signHi(int64(v))}, uint((strconv.IntSize+6)/7))
}
func (e *FileEncoder) EmitI128(v Int128) { e.writeSigned(v, 19) }
func (e *FileEncoder) writeUnsigned(v Uint128, n uint) {
	e.WriteWith(n, func(out []byte) uint { return writeUnsignedLEB(out, v) })
}
func (e *FileEncoder) writeSigned(v Int128, n uint) {
	e.WriteWith(n, func(out []byte) uint { return writeSignedLEB(out, v) })
}
func (e *FileEncoder) EmitRawBytes(v []byte) { e.writeAll(v) }
func (e *FileEncoder) EmitI8(v int8)         { DefaultEmitI8(e, v) }
func (e *FileEncoder) EmitBool(v bool)       { DefaultEmitBool(e, v) }
func (e *FileEncoder) EmitChar(v rune)       { DefaultEmitChar(e, v) }
func (e *FileEncoder) EmitStr(v string)      { DefaultEmitStr(e, v) }
func (e *FileEncoder) EmitByteStr(v []byte)  { DefaultEmitByteStr(e, v) }
