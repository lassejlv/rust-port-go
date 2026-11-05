package util

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"unsafe"

	imrc "github.com/lassejlv/rust-port-go/port/dependencies/im-rc/v15_1_0/src"
	"github.com/lassejlv/rust-port-go/port/support/rustfmt"
	"github.com/lassejlv/rust-port-go/port/support/rustvec"
)

// These non-Copy type implementations mirror only the separate oracle client.
// All graph/container operations under test come from the production adapters.
type graphOwnedRequest struct{ scenario, size, op, mode, fail1, fail2, format, writeFail, chars, reuse uint64 }

func graphOwnedParse(s string) graphOwnedRequest {
	p := strings.Split(s, "\t")
	if len(p) != 10 {
		panic("owned graph request schema")
	}
	var n [10]uint64
	for i := range n {
		var err error
		n[i], err = strconv.ParseUint(p[i], 10, 64)
		if err != nil {
			panic(err)
		}
	}
	return graphOwnedRequest{n[0], n[1], n[2], n[3], n[4], n[5], n[6], n[7], n[8], n[9]}
}

type graphProbeInfo struct {
	serial uint32
	name   string
	live   bool
}
type graphProbeKey struct {
	id    int32
	stamp uint32
	info  *graphProbeInfo
}
type graphProbeEdge struct {
	id    int32
	stamp uint32
	info  *graphProbeInfo
}
type graphOwnedProbe struct {
	request      graphOwnedRequest
	serial       uint32
	events       uint64
	trace        []string
	registry     []*graphProbeInfo
	stored       *OwnedGraph[graphProbeKey, graphProbeEdge]
	stderr       strings.Builder
	hooks        bool
	hookBehavior string
	checked      bool
	sites        map[string]graphHookSite
	emit         bool
}

func graphProbeContext(scope string) rustfmt.Context {
	return rustfmt.Context{Runtime: &rustfmt.RuntimeMetadata{Value: scope}}
}
func (s *graphOwnedProbe) event(ctx rustfmt.Context, message string) uint64 {
	if ctx.Runtime == nil {
		panic("missing current operation scope")
	}
	scope := ctx.Runtime.Value.(string)
	line := fmt.Sprintf("%s:%t:%s", message, ctx.Panicking(), scope)
	s.trace = append(s.trace, line)
	if s.emit && scope != "setup" {
		fmt.Fprintln(os.Stdout, line)
	}
	n := s.events
	s.events++
	return n
}
func (s *graphOwnedProbe) trip(ctx rustfmt.Context, n uint64) {
	if ctx.Runtime.Value == "op" && (n == s.request.fail1 || n == s.request.fail2) {
		ctx.PanicAt(rustfmt.SourcePanicInfo{Payload: "selected external trait failure", Kind: rustfmt.PanicStaticStr, Location: ctx.SourcePanicLocation("graph.client.trip"), CanUnwind: true})
	}
}
func (s *graphOwnedProbe) guard(ctx rustfmt.Context, kind string, body func()) {
	imrc.Scope(ctx, body, func(ctx rustfmt.Context) { s.event(ctx, "guard:"+kind) })
}
func (s *graphOwnedProbe) info(kind byte, id int32) *graphProbeInfo {
	p := &graphProbeInfo{serial: s.serial, name: fmt.Sprintf("%c%d@%d", kind, id, s.serial), live: true}
	s.serial++
	s.registry = append(s.registry, p)
	return p
}
func (s *graphOwnedProbe) key(id int32) graphProbeKey {
	return graphProbeKey{id: id, stamp: 100, info: s.info('K', id)}
}
func (s *graphOwnedProbe) edge(id int32) graphProbeEdge {
	return graphProbeEdge{id: id, stamp: 300, info: s.info('E', id)}
}
func (v *graphProbeKey) label() string  { return fmt.Sprintf("%d:%d:%d", v.info.serial, v.id, v.stamp) }
func (v *graphProbeEdge) label() string { return fmt.Sprintf("%d:%d:%d", v.info.serial, v.id, v.stamp) }
func (s *graphOwnedProbe) dropInfo(info *graphProbeInfo, ctx rustfmt.Context) {
	defer func() { info.live = false }()
	s.guard(ctx, "info", func() { s.trip(ctx, s.event(ctx, fmt.Sprintf("info-drop:%d", info.serial))) })
}
func (s *graphOwnedProbe) dropKey(value *graphProbeKey, ctx rustfmt.Context) {
	imrc.Scope(ctx, func() {
		s.guard(ctx, "key-drop", func() { n := s.event(ctx, "key-drop:"+value.label()); value.stamp += 10000; s.trip(ctx, n) })
	}, func(ctx rustfmt.Context) { s.dropInfo(value.info, ctx) })
}
func (s *graphOwnedProbe) dropEdge(value *graphProbeEdge, ctx rustfmt.Context) {
	imrc.Scope(ctx, func() {
		s.guard(ctx, "edge-drop", func() { n := s.event(ctx, "edge-drop:"+value.label()); value.stamp += 10000; s.trip(ctx, n) })
	}, func(ctx rustfmt.Context) { s.dropInfo(value.info, ctx) })
}
func (s *graphOwnedProbe) ownKey(value graphProbeKey) *rustvec.Owned[graphProbeKey] {
	return rustvec.NewOwned(value, rustvec.ValueOps[graphProbeKey]{Drop: s.dropKey})
}
func (s *graphOwnedProbe) ownEdge(value graphProbeEdge) *rustvec.Owned[graphProbeEdge] {
	return rustvec.NewOwned(value, rustvec.ValueOps[graphProbeEdge]{Drop: s.dropEdge})
}
func (s *graphOwnedProbe) keyBinary(kind string, delta uint32, a, b rustvec.Ref[graphProbeKey]) (result int) {
	s.guard(a.Context, "key-"+kind, func() {
		n := s.event(a.Context, "key-"+kind+":"+a.Value.label()+":"+b.Value.label())
		a.Value.stamp += delta
		b.Value.stamp += delta
		s.trip(a.Context, n)
		if a.Value.id < b.Value.id {
			result = -1
		} else if a.Value.id > b.Value.id {
			result = 1
		}
	})
	return
}
func (s *graphOwnedProbe) edgeBinary(kind string, delta uint32, a, b rustvec.Ref[graphProbeEdge]) (result bool) {
	s.guard(a.Context, "edge-"+kind, func() {
		n := s.event(a.Context, "edge-"+kind+":"+a.Value.label()+":"+b.Value.label())
		a.Value.stamp += delta
		b.Value.stamp += delta
		s.trip(a.Context, n)
		result = a.Value.id == b.Value.id
	})
	return
}
func (s *graphOwnedProbe) traits() GraphTraits[graphProbeKey, graphProbeEdge] {
	out := GraphTraits[graphProbeKey, graphProbeEdge]{
		Node: imrc.Ops[graphProbeKey]{Drop: s.dropKey, Clone: func(ref rustvec.Ref[graphProbeKey]) (out *rustvec.Owned[graphProbeKey]) {
			s.guard(ref.Context, "key-clone", func() {
				n := s.event(ref.Context, "key-clone:"+ref.Value.label())
				old := ref.Value.stamp
				ref.Value.stamp += 10
				s.trip(ref.Context, n)
				value := s.key(ref.Value.id)
				value.stamp = old + 1
				out = s.ownKey(value)
			})
			return
		}},
		Edge: imrc.Ops[graphProbeEdge]{Drop: s.dropEdge, Clone: func(ref rustvec.Ref[graphProbeEdge]) (out *rustvec.Owned[graphProbeEdge]) {
			s.guard(ref.Context, "edge-clone", func() {
				n := s.event(ref.Context, "edge-clone:"+ref.Value.label())
				old := ref.Value.stamp
				ref.Value.stamp += 10
				s.trip(ref.Context, n)
				value := s.edge(ref.Value.id)
				value.stamp = old + 1
				out = s.ownEdge(value)
			})
			return
		}},
		CompareNode: func(a, b rustvec.Ref[graphProbeKey]) int { return s.keyBinary("cmp", 1, a, b) },
		LessNode: func(a, b rustvec.Ref[graphProbeKey]) bool {
			if s.request.scenario == 4 {
				return s.keyBinary("lt", 300, a, b) < 0
			}
			return s.keyBinary("cmp", 1, a, b) < 0
		},
		EqualNode: func(a, b rustvec.Ref[graphProbeKey]) bool { return s.keyBinary("eq", 100, a, b) == 0 },
		NotEqualNode: func(a, b rustvec.Ref[graphProbeKey]) bool {
			if s.request.scenario == 5 {
				return s.keyBinary("eq", 100, a, b) != 0
			}
			return s.keyBinary("ne", 200, a, b) != 0
		},
		EqualEdge: func(a, b rustvec.Ref[graphProbeEdge]) bool { return s.edgeBinary("eq", 100, a, b) },
		NotEqualEdge: func(a, b rustvec.Ref[graphProbeEdge]) bool {
			if s.request.scenario == 5 {
				return !s.edgeBinary("eq", 100, a, b)
			}
			return !s.edgeBinary("ne", 200, a, b)
		},
		DefaultEdge: func(ctx rustfmt.Context) (out *rustvec.Owned[graphProbeEdge]) {
			s.guard(ctx, "edge-default", func() { s.trip(ctx, s.event(ctx, "edge-default")); out = s.ownEdge(s.edge(-1)) })
			return
		},
		DisplayNode: func(ref rustvec.Ref[graphProbeKey], f *rustfmt.Formatter) (err error) {
			s.guard(ref.Context, "display", func() {
				n := s.event(ref.Context, "display:"+ref.Value.label()+":options:"+graphProbeFormatState(f.Options))
				ref.Value.stamp += 500
				s.trip(ref.Context, n)
				if s.request.format == 7 {
					err = fmt.Errorf("selected Display error")
					return
				}
				if err = f.WriteString(ref.Value.info.name); err == nil {
					if s.request.format == 6 {
						err = fmt.Errorf("selected Display error")
						return
					}
					err = f.WriteChar('!')
				}
			})
			return
		},
	}
	if s.request.scenario == 5 {
		out.NotEqualNode = nil
		out.NotEqualEdge = nil
	}
	return out
}
func graphProbeFormatState(o rustfmt.Options) string {
	w, p := "-", "-"
	if o.HasWidth {
		w = strconv.FormatUint(uint64(o.Width), 10)
	}
	if o.HasPrecision {
		p = strconv.FormatUint(uint64(o.Precision), 10)
	}
	flags := uint32(0)
	for i, v := range []bool{o.SignPlus, o.SignMinus, o.Alternate, o.ZeroPad, o.DebugLowerHex, o.DebugUpperHex} {
		if v {
			flags |= 1 << i
		}
	}
	return fmt.Sprintf("%d:%d:%s:%s:%d", o.Fill, o.Align, w, p, flags)
}
func graphProbeFormatOptions(pattern uint64) rustfmt.Options {
	o := rustfmt.DefaultOptions()
	switch pattern {
	case 1:
		o.Alternate = true
	case 2:
		o.ZeroPad, o.HasWidth, o.Width, o.DebugLowerHex = true, true, 8, true
	case 3:
		o.Fill, o.Align, o.HasWidth, o.Width, o.HasPrecision, o.Precision = '*', rustfmt.AlignCenter, true, 13, true, 2
	case 4:
		o.Fill, o.Align, o.HasWidth, o.Width = '🦀', rustfmt.AlignRight, true, 12
	case 5:
		o.SignPlus, o.Alternate, o.ZeroPad, o.HasWidth, o.Width, o.DebugUpperHex = true, true, true, true, 8, true
	}
	return o
}

type graphProbeSink struct {
	probe    *graphOwnedProbe
	fail     uint64
	chars    bool
	calls    []string
	accepted strings.Builder
}

func (s *graphProbeSink) WriteString(text string) (int, error) {
	var ids []string
	for _, info := range s.probe.registry {
		if info.live && unsafe.StringData(info.name) == unsafe.StringData(text) && len(info.name) == len(text) {
			ids = append(ids, strconv.FormatUint(uint64(info.serial), 10))
		}
	}
	s.calls = append(s.calls, fmt.Sprintf("S:%x:%s", text, strings.Join(ids, ",")))
	if uint64(len(s.calls)-1) == s.fail {
		return 0, fmt.Errorf("selected writer failure")
	}
	return s.accepted.WriteString(text)
}
func (s *graphProbeSink) Write(p []byte) (int, error) { return s.WriteString(string(p)) }
func (s *graphProbeSink) WriteChar(c rune) error {
	if !s.chars {
		_, err := s.WriteString(string(c))
		return err
	}
	s.calls = append(s.calls, fmt.Sprintf("C:%d", c))
	if uint64(len(s.calls)-1) == s.fail {
		return fmt.Errorf("selected writer failure")
	}
	s.accepted.WriteRune(c)
	return nil
}
func (s *graphOwnedProbe) format(g *OwnedGraph[graphProbeKey, graphProbeEdge], ctx rustfmt.Context) string {
	sink := &graphProbeSink{probe: s, fail: s.request.writeFail, chars: s.request.chars != 0}
	f := rustfmt.Formatter{Writer: sink, Options: graphProbeFormatOptions(s.request.format), Context: ctx}
	call := func() bool {
		s.event(ctx, "format-before:"+graphProbeFormatState(f.Options))
		err := g.WriteDebugFormatter(&f)
		s.event(ctx, "format-after:"+graphProbeFormatState(f.Options))
		return err == nil
	}
	a, b := call(), true
	if s.request.reuse != 0 {
		b = call()
	}
	return fmt.Sprintf("%t:%t:%x:%x", a, b, sink.accepted.String(), strings.Join(sink.calls, "|"))
}
func (s *graphOwnedProbe) build(ctx rustfmt.Context, debug bool) (result *OwnedGraph[graphProbeKey, graphProbeEdge]) {
	var printer io.Writer = &s.stderr
	if s.emit {
		printer = io.MultiWriter(&s.stderr, os.Stderr)
	}
	if s.request.op == 17 && s.request.writeFail <= 1 {
		errno := syscall.EBADF
		if s.request.writeFail == 1 {
			errno = syscall.EPIPE
		}
		printer = graphProbeBadFD{errno: errno}
	}
	g := NewOwnedGraphWithConfig(s.traits(), OwnedGraphConfig{DebugAssertions: debug, OverflowChecks: s.checked, Stderr: printer})
	remaining := g
	imrc.Scope(ctx, func() {
		for i := uint64(0); i < s.request.size; i++ {
			id := i
			if s.request.scenario == 3 {
				id = s.request.size - i - 1
			}
			g.AddWithContext(s.ownKey(s.key(int32(id))), ctx)
		}
		if s.request.scenario == 1 || s.request.scenario == 2 {
			for i := uint64(0); i+1 < s.request.size; i++ {
				value := s.ownEdge(s.edge(int32(i) + 10))
				rustvec.ConsumeValue(value, ctx, func(value *rustvec.Owned[graphProbeEdge]) struct{} {
					g.LinkWithContext(s.ownKey(s.key(int32(i))), s.ownKey(s.key(int32(i)+1)), ctx).AssignWithContext(value, ctx)
					return struct{}{}
				})
			}
		}
		if s.request.scenario == 2 && s.request.size > 0 {
			value := s.ownEdge(s.edge(88))
			rustvec.ConsumeValue(value, ctx, func(value *rustvec.Owned[graphProbeEdge]) struct{} {
				g.LinkWithContext(s.ownKey(s.key(0)), s.ownKey(s.key(0)), ctx).AssignWithContext(value, ctx)
				return struct{}{}
			})
		}
		assign := func(a, b, id int32) {
			value := s.ownEdge(s.edge(id))
			rustvec.ConsumeValue(value, ctx, func(value *rustvec.Owned[graphProbeEdge]) struct{} {
				g.LinkWithContext(s.ownKey(s.key(a)), s.ownKey(s.key(b)), ctx).AssignWithContext(value, ctx)
				return struct{}{}
			})
		}
		if s.request.scenario == 6 {
			for i := uint64(1); i < s.request.size; i++ {
				assign(0, int32(i), int32(i)+10)
			}
		}
		if s.request.scenario == 7 && s.request.size > 0 {
			assign(0, int32(s.request.size), 77)
		}
		if s.request.scenario == 8 && s.request.size >= 3 {
			assign(0, 1, 10)
			assign(0, int32(s.request.size), 77)
			assign(1, 2, 11)
		}
		if s.request.scenario >= 100 && s.request.scenario < 612 {
			bits := s.request.scenario - 100
			for a := int32(0); a < 3; a++ {
				for b := int32(0); b < 3; b++ {
					if bits&(uint64(1)<<uint(a*3+b)) != 0 {
						assign(a, b, a*3+b+10)
					}
				}
			}
		}
		result = g
		remaining = nil
	}, func(ctx rustfmt.Context) { remaining.CloseWithContext(ctx) })
	return
}

type graphProbeBadFD struct{ errno syscall.Errno }

func (w graphProbeBadFD) Write([]byte) (int, error) { return 0, w.errno }
func graphProbeIDs(it *GraphNodeIter[graphProbeKey, graphProbeEdge], ctx rustfmt.Context) string {
	var values []string
	for {
		node, ok := it.NextWithContext(ctx)
		if !ok {
			break
		}
		values = append(values, strconv.Itoa(int(node.id)))
	}
	return strings.Join(values, ",")
}
func graphProbeHint(h rustvec.Hint) string {
	if h.HasUpper {
		return fmt.Sprintf("(%d, Some(%d))", h.Lower, h.Upper)
	}
	return fmt.Sprintf("(%d, None)", h.Lower)
}
func graphProbePath(path []GraphBorrowedPathEntry[graphProbeKey, graphProbeEdge], q [4]*graphProbeKey) string {
	var text []string
	for _, entry := range path {
		alias := "-"
		for i, p := range q {
			if p == entry.Node {
				alias = strconv.Itoa(i)
				break
			}
		}
		edge := "-"
		if entry.Edge != nil {
			edge = strconv.FormatUint(uint64(entry.Edge.info.serial), 10)
		}
		text = append(text, fmt.Sprintf("%d:%d:%s:%s", entry.Node.id, entry.Node.info.serial, alias, edge))
	}
	return strings.Join(text, ",")
}
func (s *graphOwnedProbe) operation(gp **OwnedGraph[graphProbeKey, graphProbeEdge], q [4]*graphProbeKey, ctx rustfmt.Context) (result string) {
	g := *gp
	switch s.request.op {
	case 0:
		g.AddWithContext(s.ownKey(s.key(1)), ctx)
		return fmt.Sprintf("len:%d", g.Len())
	case 1:
		e := g.LinkWithContext(s.ownKey(s.key(0)), s.ownKey(s.key(1)), ctx)
		return fmt.Sprintf("edge:%d", e.ValueWithContext(ctx).id)
	case 2:
		value := s.ownEdge(s.edge(99))
		rustvec.ConsumeValue(value, ctx, func(value *rustvec.Owned[graphProbeEdge]) struct{} {
			g.LinkWithContext(s.ownKey(s.key(0)), s.ownKey(s.key(1)), ctx).AssignWithContext(value, ctx)
			return struct{}{}
		})
		return "assigned"
	case 3, 4, 5:
		var h *OwnedGraph[graphProbeKey, graphProbeEdge]
		if s.request.op == 5 {
			h = g.ReversedWithContext(ctx)
		} else {
			h = g.CloneWithContext(ctx)
			s.event(ctx, "cloned")
		}
		imrc.Scope(ctx, func() {
			if s.request.op == 4 {
				value := s.ownEdge(s.edge(99))
				rustvec.ConsumeValue(value, ctx, func(value *rustvec.Owned[graphProbeEdge]) struct{} {
					h.LinkWithContext(s.ownKey(s.key(0)), s.ownKey(s.key(1)), ctx).AssignWithContext(value, ctx)
					return struct{}{}
				})
			}
			if s.request.op == 5 {
				result = graphProbeIDs(h.IterWithContext(ctx), ctx)
				s.event(ctx, "drop-reversed")
			} else {
				equal := g.EqualWithContext(h, ctx)
				s.event(ctx, "drop-clone")
				result = fmt.Sprintf("equal:%t", equal)
			}
			h.CloseWithContext(ctx)
		}, func(ctx rustfmt.Context) { h.CloseWithContext(ctx) })
		return
	case 6:
		return strconv.FormatBool(g.EqualWithContext(g, ctx))
	case 7:
		return graphProbeIDs(g.IterWithContext(ctx), ctx)
	case 8:
		it := g.IterWithContext(ctx)
		a := graphProbeHint(it.SizeHint())
		n, ok := it.NextWithContext(ctx)
		first := int32(-1)
		if ok {
			first = n.id
		}
		b := graphProbeHint(it.SizeHint())
		it.NextWithContext(ctx)
		n, ok = it.NextWithContext(ctx)
		nth := int32(-1)
		if ok {
			nth = n.id
		}
		count := 0
		for {
			_, ok := it.NextWithContext(ctx)
			if !ok {
				break
			}
			count++
		}
		return fmt.Sprintf("%s:%d:%s:%d:%d", a, first, b, nth, count)
	case 9:
		it := g.EdgesWithContext(q[0], ctx)
		s.event(ctx, "created-edges")
		a := graphProbeHint(it.SizeHint())
		edge, ok := it.NextWithContext(ctx)
		first := "none"
		if ok {
			first = fmt.Sprintf("%d:%d", edge.Node.id, edge.Edge.id)
		}
		b := graphProbeHint(it.SizeHint())
		count := 0
		for {
			_, ok := it.NextWithContext(ctx)
			if !ok {
				break
			}
			count++
		}
		return fmt.Sprintf("%s:%s:%s:%d", a, first, b, count)
	case 10:
		a := g.ContainsWithContext(q[0], ctx)
		b := g.ContainsWithContext(q[1], ctx)
		id := int32(999)
		c := OwnedGraphContainsBorrowed(g, &id, func(ref rustvec.Ref[graphProbeKey]) (out *int32) {
			s.guard(ref.Context, "borrow", func() {
				n := s.event(ref.Context, "borrow:"+ref.Value.label())
				ref.Value.stamp += 1000
				s.trip(ref.Context, n)
				out = &ref.Value.id
			})
			return
		}, func(a, b *int32, _ rustfmt.Context) int {
			if *a < *b {
				return -1
			}
			if *a > *b {
				return 1
			}
			return 0
		}, ctx)
		return fmt.Sprintf("%t:%t:%t", a, b, c)
	case 11:
		e := g.EdgeWithContext(q[0], q[1], ctx)
		if e == nil {
			return "none"
		}
		return strconv.Itoa(int(e.id))
	case 12:
		return s.format(g, ctx)
	case 13:
		s.stored = g.CloneWithContext(ctx)
		return "stored"
	case 14:
		h := s.build(ctx, g.config.DebugAssertions)
		imrc.Scope(ctx, func() {
			equal := g.EqualWithContext(h, ctx)
			h.CloseWithContext(ctx)
			result = fmt.Sprintf("independent:%t", equal)
		}, func(ctx rustfmt.Context) { h.CloseWithContext(ctx) })
		return
	case 15:
		*gp = NewOwnedGraphWithConfig(s.traits(), g.config)
		g.CloseWithContext(ctx)
		return "closed"
	case 16:
		h := NewOwnedGraphWithConfig(s.traits(), g.config)
		imrc.Scope(ctx, func() {
			h.AddWithContext(s.ownKey(s.key(-10)), ctx)
			h.CloneFromWithContext(g, ctx)
			s.event(ctx, "clone-from")
			equal := g.EqualWithContext(h, ctx)
			h.CloseWithContext(ctx)
			result = fmt.Sprintf("equal:%t", equal)
		}, func(ctx rustfmt.Context) { h.CloseWithContext(ctx) })
		return
	case 17:
		if g.config.DebugAssertions {
			g.printOwnedForTest(ctx)
			return "printed"
		}
		return "not-compiled"
	case 18:
		values := g.SortWithContext(ctx)
		imrc.Scope(ctx, func() {
			var ids []string
			for i := 0; i < values.Len(); i++ {
				ids = append(ids, strconv.Itoa(int(values.Ref(i).id)))
			}
			result = strings.Join(ids, ",")
			s.event(ctx, "drop-sorted")
			values.CloseWithContext(ctx)
		}, func(ctx rustfmt.Context) { values.CloseWithContext(ctx) })
		return
	case 19:
		return graphProbePath(g.PathToBottomWithContext(q[0], ctx), q)
	case 20:
		return graphProbePath(g.PathToTopWithContext(q[3], ctx), q)
	case 21:
		return graphProbePath(g.PathToBottomWithContext(q[2], ctx), q)
	case 22:
		return fmt.Sprintf("%t:%t:%t:%t", g.IsPathFromToWithContext(q[0], q[1], ctx), g.IsPathFromToWithContext(q[1], q[0], ctx), g.IsPathFromToWithContext(q[0], q[3], ctx), g.IsPathFromToWithContext(q[0], q[0], ctx))
	case 23:
		h := g.CloneWithContext(ctx)
		imrc.Scope(ctx, func() {
			a, b := g.EdgeWithContext(q[0], q[1], ctx), h.EdgeWithContext(q[0], q[1], ctx)
			before := a != nil && a == b
			value := s.ownEdge(s.edge(99))
			rustvec.ConsumeValue(value, ctx, func(value *rustvec.Owned[graphProbeEdge]) struct{} {
				h.LinkWithContext(s.ownKey(s.key(0)), s.ownKey(s.key(1)), ctx).AssignWithContext(value, ctx)
				return struct{}{}
			})
			a, b = g.EdgeWithContext(q[0], q[1], ctx), h.EdgeWithContext(q[0], q[1], ctx)
			after := a != nil && a == b
			a, b = g.EdgeWithContext(q[0], q[3], ctx), h.EdgeWithContext(q[0], q[3], ctx)
			other := a != nil && a == b
			h.CloseWithContext(ctx)
			result = fmt.Sprintf("%t:%t:%t", before, after, other)
		}, func(ctx rustfmt.Context) { h.CloseWithContext(ctx) })
		return
	case 24:
		path := g.PathToBottomWithContext(q[0], ctx)
		if len(path) == 0 || path[len(path)-1].Edge == nil {
			return "none"
		}
		last := path[len(path)-1]
		last.Edge.stamp += 7000
		var found *graphProbeEdge
		if len(path) > 1 {
			p := path[len(path)-2].Node
			a, b := g.EdgeWithContext(p, last.Node, ctx), g.EdgeWithContext(last.Node, p, ctx)
			found = a
			if found == nil {
				found = b
			}
		}
		return fmt.Sprintf("%t:%d", found != nil && found == last.Edge, last.Edge.stamp)
	case 26:
		g.AddWithContext(s.ownKey(s.key(int32(s.request.size)+1)), ctx)
		return fmt.Sprintf("len:%d", g.Len())
	case 27:
		e := g.LinkWithContext(s.ownKey(s.key(0)), s.ownKey(s.key(int32(s.request.size)+1)), ctx)
		return fmt.Sprintf("edge:%d", e.ValueWithContext(ctx).id)
	case 28:
		h := g.CloneWithContext(ctx)
		imrc.Scope(ctx, func() {
			h.AddWithContext(s.ownKey(s.key(int32(s.request.size)+1)), ctx)
			count := 0
			it := h.IterWithContext(ctx)
			for {
				_, ok := it.NextWithContext(ctx)
				if !ok {
					break
				}
				count++
			}
			h.CloseWithContext(ctx)
			result = fmt.Sprintf("count:%d", count)
		}, func(ctx rustfmt.Context) { h.CloseWithContext(ctx) })
		return
	case 29:
		first := "ok"
		if graphProbeCatch(func() { g.AddWithContext(s.ownKey(s.key(int32(s.request.size)+1)), ctx) }) != nil {
			first = "panic"
		}
		count := 0
		it := g.IterWithContext(ctx)
		for {
			_, ok := it.NextWithContext(ctx)
			if !ok {
				break
			}
			count++
		}
		return fmt.Sprintf("%s:%d:%d", first, count, g.Len())
	default:
		panic("unsupported graph operation")
	}
}
func graphProbeCatch(body func()) (p any) { defer func() { p = recover() }(); body(); return nil }
func graphOwnedReplay(line string, debug, emit bool) string {
	return graphOwnedReplayDetailed(line, debug, emit).observation
}

type graphOwnedObservation struct{ observation, stderr string }

type graphHookSite struct {
	Kind             rustfmt.PanicPayloadKind `json:"kind"`
	Payload          string                   `json:"payload"`
	File             string                   `json:"file"`
	Line             uint32                   `json:"line"`
	Column           uint32                   `json:"column"`
	CanUnwind        bool                     `json:"can_unwind"`
	ForceNoBacktrace bool                     `json:"force_no_backtrace"`
}

func (site graphHookSite) info() rustfmt.SourcePanicInfo {
	var location *rustfmt.SourceLocation
	if site.File != "" {
		location = &rustfmt.SourceLocation{File: site.File, Line: site.Line, Column: site.Column}
	}
	return rustfmt.SourcePanicInfo{Payload: site.Payload, Kind: site.Kind, Location: location, CanUnwind: site.CanUnwind, ForceNoBacktrace: site.ForceNoBacktrace}
}
func (s *graphOwnedProbe) hook(info rustfmt.SourcePanicInfo, ctx rustfmt.Context) {
	if !s.hooks {
		return
	}
	file := ""
	var line, column uint32
	if info.Location != nil {
		file, line, column = info.Location.File, info.Location.Line, info.Location.Column
	}
	s.event(ctx, fmt.Sprintf("hook:%s:%x:%t:%t:%x:%d:%d", info.Kind, fmt.Sprint(info.Payload), info.CanUnwind, info.ForceNoBacktrace, file, line, column))
}
func graphOwnedReplayDetailed(line string, debug, emit bool) graphOwnedObservation {
	return graphOwnedReplaySettings(line, debug, emit, false, nil)
}
func graphOwnedReplaySettings(line string, debug, emit, hooks bool, sites map[string]graphHookSite, behaviors ...string) graphOwnedObservation {
	return graphOwnedReplayProfile(line, debug, true, emit, hooks, sites, behaviors...)
}
func graphOwnedReplayProfile(line string, debug, checked, emit, hooks bool, sites map[string]graphHookSite, behaviors ...string) graphOwnedObservation {
	behavior := "observe"
	if len(behaviors) != 0 {
		behavior = behaviors[0]
	}
	s := graphOwnedProbe{request: graphOwnedParse(line), emit: emit, hooks: hooks, sites: sites, hookBehavior: behavior, checked: checked}
	g := s.build(graphProbeContext("setup"), debug)
	q := [4]*graphProbeKey{}
	for i, id := range []int32{0, 1, 999, int32(s.request.size) - 1} {
		value := s.key(id)
		q[i] = &value
	}
	s.trace = nil
	s.events = 0
	ctx := graphProbeContext("op")
	ops := rustfmt.PanicOperations{Locations: map[string]rustfmt.SourceLocation{}}
	for name, site := range sites {
		ops.Locations[name] = rustfmt.SourceLocation{File: site.File, Line: site.Line, Column: site.Column}
	}
	if hooks {
		cleanup := sites["graph.runtime.cleanup"].info()
		ops.CleanupAbort = &cleanup
	}
	var result string
	ops.Hook = func(info rustfmt.SourcePanicInfo, hookCtx rustfmt.Context) {
		s.hook(info, hookCtx)
		imrc.Scope(hookCtx, func() {
			if behavior == "panic" {
				hookCtx.Panic("hook panic")
			}
			if behavior == "resume" {
				hookCtx.ResumeUnwind("hook resume")
			}
			if s.request.mode == 3 {
				s.event(hookCtx, "processing")
				result = s.operation(&g, q, hookCtx)
			}
		}, func(ctx rustfmt.Context) {
			if hooks {
				s.event(ctx, "guard:hook")
				if behavior == "drop_panic" {
					ctx.Panic("hook destructor panic")
				}
			}
		})
	}
	if hooks || s.request.mode == 3 {
		ctx = rustfmt.WithPanicOperations(ctx, &ops)
	}
	outerInfo := func(site string) rustfmt.SourcePanicInfo {
		info := sites[site].info()
		info.Payload = "outer"
		info.Kind = rustfmt.PanicStaticStr
		info.CanUnwind = true
		return info
	}
	switch s.request.mode {
	case 0:
		if graphProbeCatch(func() { result = s.operation(&g, q, ctx) }) != nil {
			result = "panic"
		}
	case 1, 2:
		graphProbeCatch(func() {
			imrc.Scope(ctx, func() { ctx.PanicAt(outerInfo("graph.client.outer1")) }, func(unwind rustfmt.Context) {
				s.event(unwind, "outer-drop")
				if s.request.mode == 1 {
					if graphProbeCatch(func() { result = s.operation(&g, q, unwind) }) != nil {
						result = "panic"
					}
				} else {
					result = s.operation(&g, q, unwind)
				}
			})
		})
	case 3:
		graphProbeCatch(func() { ctx.PanicAt(outerInfo("graph.client.outer3")) })
	}
	len := g.Len()
	after, stored := "ok", "ok"
	afterCtx := graphProbeContext("after")
	if hooks {
		afterCtx = rustfmt.WithPanicOperations(afterCtx, &ops)
	}
	if graphProbeCatch(func() { g.CloseWithContext(afterCtx) }) != nil {
		after = "panic"
	}
	if graphProbeCatch(func() { s.stored.CloseWithContext(afterCtx) }) != nil {
		stored = "panic"
	}
	var alive []string
	for _, info := range s.registry {
		if info.live {
			alive = append(alive, strconv.FormatUint(uint64(info.serial), 10))
		}
	}
	return graphOwnedObservation{observation: fmt.Sprintf("%x\t%d\t%s\t%s\t%s\t%x", result, len, after, stored, strings.Join(alive, ","), strings.Join(s.trace, "|")), stderr: s.stderr.String()}
}

func graphOwnedAssertObservation(t *testing.T, request, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	a, b := strings.Split(got, "\t"), strings.Split(want, "\t")
	if len(a) != 6 || len(b) != 6 {
		t.Fatalf("schema\n%s\n%s", got, want)
	}
	aaBytes, _ := hex.DecodeString(a[5])
	bbBytes, _ := hex.DecodeString(b[5])
	aa, bb := strings.Split(string(aaBytes), "|"), strings.Split(string(bbBytes), "|")
	first := 0
	for first < len(aa) && first < len(bb) && aa[first] == bb[first] {
		first++
	}
	lo := max(0, first-3)
	t.Fatalf("%s result\ngot %v\nwant%v\ntrace at%d\ngot %v\nwant%v", request, a[:5], b[:5], first, aa[lo:min(len(aa), first+7)], bb[lo:min(len(bb), first+7)])
}
func TestGraphOwnedAbortHelper(t *testing.T) {
	line, ok := os.LookupEnv("FERRUGO_GRAPH_OWNED_ABORT")
	if !ok {
		t.Skip("isolated original-source abort replay")
	}
	var sites map[string]graphHookSite
	hooks := os.Getenv("FERRUGO_GRAPH_HOOKS") == "1"
	if hooks {
		if err := json.Unmarshal([]byte(os.Getenv("FERRUGO_GRAPH_HOOK_SITES")), &sites); err != nil {
			t.Fatal(err)
		}
	}
	graphOwnedReplayProfile(line, os.Getenv("FERRUGO_GRAPH_DEBUG") != "0", os.Getenv("FERRUGO_GRAPH_CHECKED") != "0", true, hooks, sites, os.Getenv("FERRUGO_GRAPH_HOOK_BEHAVIOR"))
	t.Fatal("expected source abort returned")
}
