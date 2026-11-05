// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcspan_test

import (
	"errors"
	"fmt"
	"strings"

	sp "github.com/lassejlv/rust-port-go/port/rust/compiler/rustc_span/src"
	rf "github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func hygieneDebugPanicBuild(operation string, s *hygieneOwnedCore, construction rf.Context) func(rf.Context) {
	var action func(rf.Context)
	if strings.HasPrefix(operation, "already_set_debug") {
		first := s.data(construction)
		first.MacroDefID = sp.Some(sp.CrateDefId().ToDefId())
		_ = sp.FreshLocalExpnId(s.session, first.CloneWithContext(construction), hygieneCoreHcx{})
		id := sp.FreshLocalExpnId(s.session, first, hygieneCoreHcx{})
		data := id.ExpnDataWithContext(s.session, construction)
		env := s.newEnv(construction)
		action = func(c rf.Context) {
			_ = sp.FreshLocalExpnIdOwned(s.session, data, env, hygieneOwnedOps(), c)
			fmt.Println("returned:already_set_debug")
		}
	} else {
		action = func(c rf.Context) {
			_ = sp.DebugHygieneDataWithContext(s.session, true, c)
			fmt.Println("returned:debug")
		}
	}
	// This callback is the source &'static fn reading authored static state,
	// without an additional destructible function environment.
	callback := sp.NewDefIdDebugCallback(func(id sp.DefId, f *rf.Formatter) error {
		s.call(f.Context, "debug", fmt.Sprintf("%d/%d", id.Krate.AsU32(), id.Index.AsU32()))
		if s.reenter {
			next := sp.FreshEmptyLocalExpnIdWithContext(s.session, f.Context)
			fmt.Printf("debug_reentered:%d\n", next.AsU32())
		}
		if operation == "debug_verbose_error" || operation == "already_set_debug_error" {
			return errors.New("fmt::Error")
		}
		return sp.DefaultDefIdDebug(id, f)
	})
	previous := sp.SwapDefIdDebug(callback)
	return func(c rf.Context) { defer sp.SwapDefIdDebug(previous); action(c) }
}
