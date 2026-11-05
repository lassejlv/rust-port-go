// SPDX-License-Identifier: MIT OR Apache-2.0
package rustclexer

import rustfmt "github.com/lassejlv/rust-port-go/port/support/rustfmt"

// Predicate witnesses the owned FnMut(char)->bool in Cursor::eat_while.
// Env is the actual borrowed call/destruction cell. Drop represents complete
// environment destruction. Borrow transfers a mutable reference without owning
// its environment; an owned call transfers ownership and must not be reused.
type Predicate[Env any] struct {
	Env     *Env
	Call    func(*Env, rustfmt.Context, rune) bool
	Drop    func(*Env, rustfmt.Context)
	Context rustfmt.Context
}

func (p Predicate[Env]) Borrow() Predicate[Env] { p.Drop = nil; return p }
func (p Predicate[Env]) Destroy(context rustfmt.Context) {
	if p.Drop != nil {
		p.Drop(p.Env, context)
	}
}

// EatWhileOwned retains the source predicate-before-EOF evaluation order,
// including one predicate call with EOFChar on an initially empty cursor.
func EatWhileOwned[Env any](cursor *Cursor, predicate Predicate[Env]) {
	context, restore := cursor.enterContext(predicate.Context)
	defer restore()
	predicate.Context = context
	defer tokenConsumerDrop(predicate.Context, predicate.Destroy)
	cursor.EatWhile(func(c rune) bool { return predicate.Call(predicate.Env, predicate.Context, c) })
}
