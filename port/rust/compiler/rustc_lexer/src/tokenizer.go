// SPDX-License-Identifier: MIT OR Apache-2.0
package rustclexer

import rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// Tokenizer is the opaque source from_fn iterator over a borrowed input.
// It advertises neither Clone, Debug nor FusedIterator. Go descriptor copies
// do not grant a source Clone operation. Count/Last/consuming callbacks move
// ownership; callers must not reuse the consumed iterator.
type Tokenizer struct{ cursor *Cursor }

func NewTokenizer(input string, frontmatter FrontmatterAllowed) *Tokenizer {
	return &Tokenizer{NewCursor(input, frontmatter)}
}
func TokenizeIterator(input string, frontmatter FrontmatterAllowed) *Tokenizer {
	return NewTokenizer(input, frontmatter)
}
func (it *Tokenizer) Next() (Token, bool) {
	token := it.cursor.AdvanceToken()
	if token.Kind.Tag == Eof {
		return Token{}, false
	}
	return token, true
}
func (it *Tokenizer) SizeHint() (uint, uint, bool) { return 0, 0, false }
func (it *Tokenizer) Nth(n uint) (Token, bool) {
	for i := uint(0); i < n; i++ {
		if _, ok := it.Next(); !ok {
			return Token{}, false
		}
	}
	return it.Next()
}
func (it *Tokenizer) Count() uint {
	var count uint
	for {
		if _, ok := it.Next(); !ok {
			return count
		}
		count++
	}
}
func (it *Tokenizer) Last() (Token, bool) {
	var last Token
	found := false
	for {
		token, ok := it.Next()
		if !ok {
			return last, found
		}
		last = token
		found = true
	}
}

type TokenConsumer[Env any] struct {
	Env     *Env
	Call    func(*Env, rustfmt.Context, Token)
	Drop    func(*Env, rustfmt.Context)
	Context rustfmt.Context
}

func (c TokenConsumer[Env]) Borrow() TokenConsumer[Env] { c.Drop = nil; return c }
func (c TokenConsumer[Env]) Destroy(context rustfmt.Context) {
	if c.Drop != nil {
		c.Drop(c.Env, context)
	}
}

type TokenFolder[Env, Acc any] struct {
	Env     *Env
	Call    func(*Env, rustfmt.Context, Acc, Token) Acc
	Drop    func(*Env, rustfmt.Context)
	Context rustfmt.Context
}

func (c TokenFolder[Env, Acc]) Borrow() TokenFolder[Env, Acc] { c.Drop = nil; return c }
func (c TokenFolder[Env, Acc]) Destroy(context rustfmt.Context) {
	if c.Drop != nil {
		c.Drop(c.Env, context)
	}
}

func tokenRequiredCleanup(context rustfmt.Context, cleanup func()) {
	defer func() {
		if recover() != nil {
			context.AbortCleanup()
		}
	}()
	cleanup()
}
func tokenProcessingGuard(context rustfmt.Context) {
	if payload := recover(); payload != nil {
		context.ResumeUnwind(payload)
	}
}
func tokenConsumerDrop(context rustfmt.Context, destroy func(rustfmt.Context)) {
	if p := recover(); p != nil {
		if context.ProcessingPanic {
			context.ResumeUnwind(p)
		}
		context.Unwinding = true
		tokenRequiredCleanup(context, func() { destroy(context) })
		context.ResumeUnwind(p)
	}
	defer tokenProcessingGuard(context)
	destroy(context)
}
func ForEachTokenOwned[Env any](it *Tokenizer, callback TokenConsumer[Env]) {
	context, restore := it.cursor.enterContext(callback.Context)
	defer restore()
	callback.Context = context
	defer tokenConsumerDrop(callback.Context, callback.Destroy)
	for {
		token, ok := it.Next()
		if !ok {
			return
		}
		callback.Call(callback.Env, callback.Context, token)
	}
}
func (it *Tokenizer) ForEach(callback func(Token)) {
	ForEachTokenOwned(it, TokenConsumer[func(Token)]{Env: &callback, Call: func(fn *func(Token), _ rustfmt.Context, token Token) { (*fn)(token) }})
}

// FoldTokensOwned moves each accumulator into Call, which owns argument
// cleanup if that call panics. dropAcc destroys the still-owned accumulator
// only if Next panics before a call. A final returned accumulator is already
// in its return place when callback destruction runs; a destructor panic must
// not invent a Drop for that value (the original source leaks that return place).
func FoldTokensOwned[Env, Acc any](it *Tokenizer, initial Acc, callback TokenFolder[Env, Acc], dropAcc func(*Acc, rustfmt.Context)) (result Acc) {
	context, restore := it.cursor.enterContext(callback.Context)
	defer restore()
	callback.Context = context
	accumulator := initial
	owned := true
	defer func() {
		defer tokenProcessingGuard(callback.Context)
		if p := recover(); p != nil {
			context := callback.Context
			if context.ProcessingPanic {
				context.ResumeUnwind(p)
			}
			context.Unwinding = true
			if owned && dropAcc != nil {
				tokenRequiredCleanup(context, func() { dropAcc(&accumulator, context) })
			}
			tokenRequiredCleanup(context, func() { callback.Destroy(context) })
			context.ResumeUnwind(p)
		}
		callback.Destroy(callback.Context)
	}()
	for {
		token, ok := it.Next()
		if !ok {
			owned = false
			return accumulator
		}
		argument := accumulator
		owned = false
		accumulator = callback.Call(callback.Env, callback.Context, argument, token)
		owned = true
	}
}
