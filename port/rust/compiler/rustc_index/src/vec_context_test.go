// SPDX-License-Identifier: MIT OR Apache-2.0
package rustcindex

import (
	"reflect"
	"testing"

	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
)

func TestIndexStorageUsesCurrentRuntimeAndDoesNotRetainIt(t *testing.T) {
	a, b := &rustfmt.RuntimeMetadata{Value: "A"}, &rustfmt.RuntimeMetadata{Value: "B"}
	var seen []*rustfmt.RuntimeMetadata
	ops := ValueOps[uint8]{RustSize: 1, CloneRefWithContext: func(v *uint8, c rustfmt.Context) uint8 { seen = append(seen, c.Runtime); return *v }, DropWithRuntimeContext: func(_ *uint8, c rustfmt.Context) { seen = append(seen, c.Runtime) }}
	v := NewIndexVec(UsizeIdx(), ops)
	v.PushWithContext(7, rustfmt.Context{Runtime: a})
	clone := v.CloneWithContext(rustfmt.Context{Runtime: b})
	clone.Close() // An escaping owned value must not keep its clone call's B.
	iterator := v.IntoIter()
	copy := iterator.CloneWithContext(rustfmt.Context{Runtime: b})
	copy.CloseWithRuntimeContext(rustfmt.Context{Runtime: b})
	iterator.Close() // Neither the original A nor the intervening B is retained.
	if want := []*rustfmt.RuntimeMetadata{b, nil, b, b, nil}; !reflect.DeepEqual(seen, want) {
		t.Fatalf("runtime identities: got %v, want %v", seen, want)
	}
}
