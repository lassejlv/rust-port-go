package compiler

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

type checked struct {
	program     *syntax.Program
	types       map[*syntax.Expr]string
	names       map[*syntax.Expr]string
	decls       map[*syntax.Stmt]string
	params      map[*syntax.Function][]string
	funcs       map[string]*syntax.Function
	calls       map[*syntax.Expr]string
	fnNames     map[*syntax.Function]string
	diverges    map[*syntax.Expr]bool
	structs     map[string]*syntax.Struct
	structOrder []string
	assignTypes map[*syntax.Stmt]string
	arrays      map[string]arraySpec
	typeOrder   []string
	zeroSized   map[string]bool
	forTypes    map[*syntax.Stmt]string
}

type binding struct {
	typ, cname string
	mutable    bool
}
type checker struct {
	*checked
	scopes        []map[string]binding
	nextVariable  int
	returnType    string
	loopDepth     int
	bottom        map[*syntax.Expr]bool
	currentModule string
}

// Check validates the complete bootstrap binary crate without producing files.
func Check(source string) error { _, err := analyze(source); return err }

func analyze(source string) (*checked, error) {
	program, err := syntax.Parse(source)
	if err != nil {
		return nil, err
	}
	return analyzeProgram(program)
}

func analyzeProgram(program *syntax.Program) (*checked, error) {
	return analyzeProgramMode(program, true)
}

func analyzeProgramMode(program *syntax.Program, requireMain bool) (*checked, error) {
	program, err := resolveModulesMode(program, requireMain)
	if err != nil {
		return nil, err
	}
	c := &checker{bottom: make(map[*syntax.Expr]bool), checked: &checked{
		program: program,
		types:   make(map[*syntax.Expr]string), names: make(map[*syntax.Expr]string),
		decls: make(map[*syntax.Stmt]string), params: make(map[*syntax.Function][]string),
		funcs: make(map[string]*syntax.Function), calls: make(map[*syntax.Expr]string),
		fnNames: make(map[*syntax.Function]string), diverges: make(map[*syntax.Expr]bool),
		structs: make(map[string]*syntax.Struct), assignTypes: make(map[*syntax.Stmt]string), arrays: make(map[string]arraySpec), zeroSized: make(map[string]bool), forTypes: make(map[*syntax.Stmt]string),
	}}
	if err := c.collectStructs(); err != nil {
		return nil, err
	}
	for i, f := range program.Functions {
		if _, exists := c.funcs[f.Name]; exists {
			return nil, fail(f.Span, "duplicate function %q", f.Name)
		}
		if !c.supportedType(f.ReturnType) {
			return nil, fail(f.Span, "unsupported return type %q", f.ReturnType)
		}
		for _, p := range f.Params {
			if !c.supportedType(p.Type) || p.Type == "()" {
				return nil, fail(p.Span, "unsupported parameter type %q", p.Type)
			}
		}
		if f.ReturnType == "&str" {
			stringParams := 0
			for _, p := range f.Params {
				if p.Type == "&str" {
					stringParams++
				}
			}
			if stringParams != 1 {
				return nil, fail(f.Span, "returning &str requires exactly one &str parameter for lifetime elision; explicit lifetimes are unsupported")
			}
		}
		c.funcs[f.Name] = f
		c.fnNames[f] = fmt.Sprintf("fr_fn%d", i)
	}
	if requireMain {
		main, exists := c.funcs["main"]
		if !exists {
			return nil, fmt.Errorf("1:1: missing main function")
		}
		if len(main.Params) != 0 || main.ReturnType != "()" {
			return nil, fail(main.Span, "main must have no parameters and return ()")
		}
	}
	for _, f := range program.Functions {
		c.scopes = []map[string]binding{{}}
		c.returnType, c.loopDepth = f.ReturnType, 0
		c.currentModule = itemModule(f.Name)
		for _, p := range f.Params {
			if _, exists := c.scopes[0][p.Name]; exists {
				return nil, fail(p.Span, "duplicate parameter %q", p.Name)
			}
			name := c.newVariable()
			c.scopes[0][p.Name] = binding{typ: p.Type, cname: name}
			c.params[f] = append(c.params[f], name)
		}
		falls, err := c.block(f.Body, f.ReturnType, false)
		if err != nil {
			return nil, err
		}
		if falls && f.ReturnType != "()" && f.Body.Tail == nil {
			return nil, fail(f.Span, "function %q can finish without returning %s", f.Name, f.ReturnType)
		}
	}
	if err := c.finishLayouts(); err != nil {
		return nil, err
	}
	return c.checked, nil
}

func fail(pos syntax.Span, format string, args ...any) error {
	prefix := ""
	if pos.File != "" {
		prefix = pos.File + ":"
	}
	return fmt.Errorf("%s%d:%d: %s", prefix, pos.Line, pos.Column, fmt.Sprintf(format, args...))
}
func supportedType(t string) bool {
	return integerType(t) || t == "bool" || t == "&str" || t == "()"
}
func integerType(t string) bool { _, ok := integerInfo(t); return ok }
func (c *checker) newVariable() string {
	c.nextVariable++
	return fmt.Sprintf("fr_v%d", c.nextVariable)
}
func (c *checker) lookup(name string) (binding, bool) {
	for i := len(c.scopes) - 1; i >= 0; i-- {
		if b, ok := c.scopes[i][name]; ok {
			return b, true
		}
	}
	return binding{}, false
}

// block returns whether control can reach the end. expected applies to its tail.
func (c *checker) block(b *syntax.Block, expected string, nested bool) (bool, error) {
	_, falls, err := c.blockValue(b, expected, nested)
	return falls, err
}

// blockValue also infers a result type when expected is empty. A block that
// always transfers control has no value; () is its concrete fallback when no
// context supplies a type. The enclosing expression records that divergence.
func (c *checker) blockValue(b *syntax.Block, expected string, nested bool) (string, bool, error) {
	if b == nil {
		return "", false, fmt.Errorf("internal error: missing block")
	}
	if nested {
		c.scopes = append(c.scopes, map[string]binding{})
		defer func() { c.scopes = c.scopes[:len(c.scopes)-1] }()
	}
	falls := true
	for i := range b.Statements {
		stmtFalls, err := c.statement(&b.Statements[i])
		if err != nil {
			return "", false, err
		}
		falls = falls && stmtFalls
	}
	if b.Tail != nil {
		typ, err := c.expr(b.Tail, expected)
		if err != nil {
			return "", false, err
		}
		return typ, falls && !c.diverges[b.Tail], nil
	}
	if falls && expected != "" && expected != "()" {
		return "", false, fail(b.Span, "expected a %s block result", expected)
	}
	if !falls && expected != "" {
		return expected, false, nil
	}
	return "()", falls, nil
}

func (c *checker) statement(s *syntax.Stmt) (bool, error) {
	switch s.Kind {
	case "let":
		if s.Expr == nil {
			return false, fail(s.Span, "let bindings require an initializer in this bootstrap")
		}
		if s.Type != "" && !c.supportedType(s.Type) {
			return false, fail(s.Span, "unsupported type %q", s.Type)
		}
		t, err := c.expr(s.Expr, s.Type)
		if err != nil {
			return false, err
		}
		if t == "()" {
			return false, fail(s.Span, "unit-valued bindings are not supported")
		}
		name := c.newVariable()
		c.decls[s] = name
		c.scopes[len(c.scopes)-1][s.Name] = binding{typ: t, cname: name, mutable: s.Mutable}
		return !c.diverges[s.Expr], nil
	case "assign":
		if s.Target != nil {
			return c.fieldAssignment(s)
		}
		b, ok := c.lookup(s.Name)
		if !ok {
			return false, fail(s.Span, "unknown variable %q", s.Name)
		}
		if !b.mutable {
			return false, fail(s.Span, "cannot assign to immutable variable %q", s.Name)
		}
		if err := c.assignmentRHS(s, b.typ); err != nil {
			return false, err
		}
		c.decls[s] = b.cname
		return !c.diverges[s.Expr], nil
	case "expr":
		if _, err := c.expr(s.Expr, s.Type); err != nil {
			return false, err
		}
		return !c.diverges[s.Expr], nil
	case "return":
		if s.Expr == nil {
			if c.returnType != "()" {
				return false, fail(s.Span, "return requires a %s value", c.returnType)
			}
		} else if _, err := c.expr(s.Expr, c.returnType); err != nil {
			return false, err
		}
		return false, nil
	case "if":
		if _, err := c.expr(s.Expr, "bool"); err != nil {
			return false, err
		}
		left, err := c.block(s.Body, "()", true)
		if err != nil {
			return false, err
		}
		right := true
		if s.Else != nil {
			right, err = c.block(s.Else, "()", true)
			if err != nil {
				return false, err
			}
		}
		return left || right, nil
	case "for":
		return c.forStatement(s)
	case "while":
		if _, err := c.expr(s.Expr, "bool"); err != nil {
			return false, err
		}
		c.loopDepth++
		_, err := c.block(s.Body, "()", true)
		c.loopDepth--
		if err != nil {
			return false, err
		}
		return !c.diverges[s.Expr], nil
	case "break", "continue":
		if c.loopDepth == 0 {
			return false, fail(s.Span, "%s outside a loop", s.Kind)
		}
		return false, nil
	case "block":
		return c.block(s.Body, "()", true)
	default:
		return false, fail(s.Span, "unsupported statement %q", s.Kind)
	}
}

func (c *checker) expr(e *syntax.Expr, expected string) (string, error) {
	if e == nil {
		return "", fmt.Errorf("internal error: missing expression")
	}
	var typ string
	switch e.Kind {
	case "array", "repeat":
		var err error
		typ, err = c.arrayExpression(e, expected)
		if err != nil {
			return "", err
		}
	case "index":
		var err error
		typ, err = c.indexExpression(e, expected)
		if err != nil {
			return "", err
		}
	case "len":
		var err error
		typ, err = c.lengthExpression(e)
		if err != nil {
			return "", err
		}

	case "int":
		var err error
		typ, err = c.integer(e, expected, false)
		if err != nil {
			return "", err
		}
	case "struct":
		var err error
		typ, err = c.structLiteral(e)
		if err != nil {
			return "", err
		}
	case "field":
		var err error
		typ, err = c.fieldExpression(e)
		if err != nil {
			return "", err
		}
	case "unit":
		typ = "()"
	case "block":
		var falls bool
		var err error
		typ, falls, err = c.blockValue(e.Body, expected, true)
		if err != nil {
			return "", err
		}
		c.diverges[e] = !falls
		c.bottom[e] = c.blockBottom(e.Body, falls)
	case "if":
		var err error
		typ, err = c.ifExpression(e, expected)
		if err != nil {
			return "", err
		}
	case "bool":
		typ = "bool"
	case "string":
		typ = "&str"
	case "name":
		b, ok := c.lookup(e.Text)
		if !ok {
			return "", fail(e.Span, "unknown variable %q", e.Text)
		}
		typ = b.typ
		c.names[e] = b.cname
	case "cast":
		var err error
		typ, err = c.castExpression(e)
		if err != nil {
			return "", err
		}
	case "unary":
		switch e.Op {
		case "-":
			var err error
			if e.Right.Kind == "int" {
				typ, err = c.integer(e.Right, expected, true)
			} else {
				typ, err = c.expr(e.Right, expected)
			}
			if err != nil {
				return "", err
			}
			if c.bottom[e.Right] {
				return "", fail(e.Span, "value-less divergent operands are unsupported for unary -")
			}
			info, ok := integerInfo(typ)
			if !ok {
				return "", fail(e.Span, "unary - requires an integer")
			}
			if !info.Signed {
				return "", fail(e.Span, "cannot apply unary - to unsigned integer %s", typ)
			}
		case "!":
			context := expected
			if context != "bool" && !integerType(context) {
				context = c.hint(e.Right)
			}
			if context == "" {
				context = "i32"
			}
			var err error
			typ, err = c.expr(e.Right, context)
			if err != nil {
				return "", err
			}
			if typ != "bool" && !integerType(typ) {
				return "", fail(e.Span, "unary ! requires a boolean or integer")
			}

		default:
			return "", fail(e.Span, "unsupported unary operator %q", e.Op)
		}
		c.diverges[e] = c.diverges[e.Right]
	case "binary":
		var err error
		typ, err = c.binary(e, expected)
		if err != nil {
			return "", err
		}
	case "call":
		if !strings.Contains(e.Text, "::") {
			if _, ok := c.lookup(e.Text); ok {
				return "", fail(e.Span, "%q is a variable and cannot be called", e.Text)
			}
		}
		f, ok := c.funcs[callTarget(e)]
		if !ok {
			return "", fail(e.Span, "unknown function %q", e.Text)
		}
		if len(e.Args) != len(f.Params) {
			return "", fail(e.Span, "function %q expects %d arguments, got %d", e.Text, len(f.Params), len(e.Args))
		}
		if err := c.visibleType(f.ReturnType, e.Span); err != nil {
			return "", err
		}
		for i, arg := range e.Args {
			if err := c.visibleType(f.Params[i].Type, e.Span); err != nil {
				return "", err
			}
			if _, err := c.expr(arg, f.Params[i].Type); err != nil {
				return "", err
			}
		}
		for _, arg := range e.Args {
			c.diverges[e] = c.diverges[e] || c.diverges[arg]
		}
		c.calls[e] = c.fnNames[f]
		typ = f.ReturnType
	case "macro":
		if err := c.macro(e); err != nil {
			return "", err
		}
		typ = "()"
		for _, arg := range e.Args {
			c.diverges[e] = c.diverges[e] || c.diverges[arg]
		}
	default:
		return "", fail(e.Span, "unsupported expression %q", e.Kind)
	}
	if c.bottom[e] && expected != "" {
		typ = expected
	}
	if expected != "" && typ != expected {
		return "", fail(e.Span, "expected %s, found %s", expected, typ)
	}
	c.types[e] = typ
	return typ, nil
}

func callTarget(e *syntax.Expr) string {
	if e.Target != "" {
		return e.Target
	}
	return e.Text
}

func (c *checker) binary(e *syntax.Expr, expected string) (string, error) {
	if e.Op == "&&" || e.Op == "||" {
		if _, err := c.expr(e.Left, "bool"); err != nil {
			return "", err
		}
		if _, err := c.expr(e.Right, "bool"); err != nil {
			return "", err
		}
		c.diverges[e] = c.diverges[e.Left]
		return "bool", nil
	}
	shift := e.Op == "<<" || e.Op == ">>"
	bitwise := e.Op == "&" || e.Op == "|" || e.Op == "^"
	compare := e.Op == "==" || e.Op == "!=" || e.Op == "<" || e.Op == "<=" || e.Op == ">" || e.Op == ">="
	if !shift && !bitwise && !compare && e.Op != "+" && e.Op != "-" && e.Op != "*" && e.Op != "/" && e.Op != "%" {
		return "", fail(e.Span, "unsupported binary operator %q", e.Op)
	}
	context := ""
	if !compare && (integerType(expected) || (bitwise && expected == "bool")) {
		context = expected
	}
	if context == "" {
		context = c.hint(e.Left)
		if context == "" && !shift {
			context = c.hint(e.Right)
		}
	}
	if context == "" {
		context = "i32"
	}
	left, err := c.expr(e.Left, context)
	if err != nil {
		return "", err
	}
	rightContext := left
	if shift {
		rightContext = ""
	}
	right, err := c.expr(e.Right, rightContext)
	if err != nil {
		return "", err
	}
	// Never-type trait inference is outside the bootstrap. A typed but unreachable
	// operand remains supported, while a value-less block cannot acquire an
	// arbitrary operator type through a coercion here.
	if c.bottom[e.Left] || c.bottom[e.Right] {
		return "", fail(e.Span, "value-less divergent operands are unsupported for %s", e.Op)
	}
	c.diverges[e] = c.diverges[e.Left] || c.diverges[e.Right]
	if shift {
		if !integerType(left) || !integerType(right) {
			return "", fail(e.Span, "shift operands must be integers")
		}
		return left, nil
	}
	if left != right {
		return "", fail(e.Span, "operands have different types")
	}
	if compare {
		if !integerType(left) && !(left == "bool" && (e.Op == "==" || e.Op == "!=")) && !(left == "&str" && (e.Op == "==" || e.Op == "!=")) {
			return "", fail(e.Span, "operator %s does not support %s", e.Op, left)
		}
		return "bool", nil
	}
	if bitwise {
		if !integerType(left) && left != "bool" {
			return "", fail(e.Span, "bitwise operators require matching integers or booleans")
		}
		return left, nil
	}
	if !integerType(left) {
		return "", fail(e.Span, "arithmetic requires integers")
	}
	return left, nil
}

func (c *checker) assignmentRHS(s *syntax.Stmt, left string) error {
	c.assignTypes[s] = left
	switch s.Op {
	case "=":
		_, err := c.expr(s.Expr, left)
		return err
	case "<<=", ">>=":
		if !integerType(left) {
			return fail(s.Span, "shift assignment requires an integer")
		}
		right, err := c.expr(s.Expr, "")
		if err != nil {
			return err
		}
		if !integerType(right) {
			return fail(s.Span, "shift count must be an integer")
		}
		if c.bottom[s.Expr] {
			return fail(s.Span, "value-less divergent shift counts are unsupported")
		}
		return nil
	case "&=", "|=", "^=":
		if !integerType(left) && left != "bool" {
			return fail(s.Span, "bitwise assignment requires an integer or boolean")
		}
	case "+=", "-=", "*=", "/=", "%=":
		if !integerType(left) {
			return fail(s.Span, "compound assignment requires an integer")
		}
	default:
		return fail(s.Span, "unsupported assignment operator %q", s.Op)
	}
	_, err := c.expr(s.Expr, left)
	return err
}

// hint finds a fixed operand type without committing unsuffixed literals to i32.
func (c *checker) hint(e *syntax.Expr) string {
	if e == nil {
		return ""
	}
	switch e.Kind {
	case "array":
		if typ := c.types[e]; typ != "" {
			return typ
		}
		for _, arg := range e.Args {
			if hint := c.hint(arg); hint != "" {
				return arrayType(hint, len(e.Args))
			}
		}
	case "repeat":
		if element := c.hint(e.Left); element != "" {
			if length, err := parseArrayLength(e.Text, e.Span); err == nil {
				return arrayType(element, length)
			}
		}
	case "index":
		if array, ok := arrayInfo(c.hint(e.Left)); ok {
			return array.Element
		}
	case "len":
		return "usize"

	case "cast":
		return e.Target
	case "struct":
		return e.Target
	case "field":
		if s := c.structs[c.hint(e.Left)]; s != nil {
			for _, field := range s.Fields {
				if field.Name == e.Text {
					return field.Type
				}
			}
		}
	case "name":
		if b, ok := c.lookup(e.Text); ok {
			return b.typ
		}
	case "int":
		_, suffix, err := parseInteger(e.Text)
		if err == nil {
			return suffix
		}
	case "unit":
		return "()"
	case "block":
		return c.blockHint(e.Body)
	case "if":
		if e.Else == nil {
			return "()"
		}
		if h := c.blockHint(e.Body); h != "" {
			return h
		}
		return c.blockHint(e.Else)
	case "string":
		return "&str"
	case "bool":
		return "bool"
	case "call":
		if f, ok := c.funcs[callTarget(e)]; ok {
			return f.ReturnType
		}
	case "unary":
		return c.hint(e.Right)
	case "binary":
		if e.Op == "<<" || e.Op == ">>" {
			return c.hint(e.Left)
		}
		if e.Op == "==" || e.Op == "!=" || e.Op == "<" || e.Op == "<=" || e.Op == ">" || e.Op == ">=" || e.Op == "&&" || e.Op == "||" {
			return "bool"
		}
		if h := c.hint(e.Left); h != "" {
			return h
		}
		return c.hint(e.Right)
	}
	return ""
}

// blockBottom distinguishes a block with no value from a merely unreachable
// tail. An unreachable typed tail must still agree with its expected type.
func (c *checker) blockBottom(b *syntax.Block, falls bool) bool {
	if b.Tail != nil {
		return c.bottom[b.Tail]
	}
	return !falls
}

func (c *checker) ifExpression(e *syntax.Expr, expected string) (string, error) {
	if _, err := c.expr(e.Left, "bool"); err != nil {
		return "", err
	}
	if e.Else == nil {
		if _, _, err := c.blockValue(e.Body, "()", true); err != nil {
			return "", err
		}
		c.diverges[e] = c.diverges[e.Left]
		return "()", nil
	}
	context := expected
	if context == "" {
		context = c.blockHint(e.Body)
		if context == "" {
			context = c.blockHint(e.Else)
		}
	}
	left, leftFalls, err := c.blockValue(e.Body, context, true)
	if err != nil {
		return "", err
	}
	leftBottom := c.blockBottom(e.Body, leftFalls)
	rightContext := context
	if rightContext == "" && !leftBottom {
		rightContext = left
	}
	right, rightFalls, err := c.blockValue(e.Else, rightContext, true)
	if err != nil {
		return "", err
	}
	rightBottom := c.blockBottom(e.Else, rightFalls)
	c.diverges[e] = c.diverges[e.Left] || (!leftFalls && !rightFalls)
	c.bottom[e] = leftBottom && rightBottom
	if leftBottom {
		if !rightBottom {
			return right, nil
		}
		if expected != "" {
			return expected, nil
		}
		return "()", nil
	}
	if !rightBottom && left != right {
		return "", fail(e.Span, "if branches have different types: %s and %s", left, right)
	}
	return left, nil
}

// blockHint uses explicit types and fixed initializer types in a temporary
// lexical scope. It does not commit unsuffixed integers to a default too early.
func (c *checker) blockHint(b *syntax.Block) string {
	if b == nil {
		return ""
	}
	c.scopes = append(c.scopes, map[string]binding{})
	defer func() { c.scopes = c.scopes[:len(c.scopes)-1] }()
	for _, s := range b.Statements {
		if s.Kind == "let" {
			typ := s.Type
			if typ == "" {
				typ = c.hint(s.Expr)
			}
			c.scopes[len(c.scopes)-1][s.Name] = binding{typ: typ}
		}
	}
	return c.hint(b.Tail)
}

func (c *checker) integer(e *syntax.Expr, expected string, negative bool) (string, error) {
	value, suffix, err := parseInteger(e.Text)
	if err != nil {
		return "", fail(e.Span, "%s", err)
	}
	typ := suffix
	if typ == "" && integerType(expected) {
		typ = expected
	}
	if typ == "" {
		typ = "i32"
	}
	info, _ := integerInfo(typ)
	if negative {
		if !info.Signed {
			return "", fail(e.Span, "cannot apply unary - to unsigned integer %s", typ)
		}
		value.Neg(value)
	}
	width := uint(info.Bits)
	min := big.NewInt(0)
	limit := new(big.Int).Lsh(big.NewInt(1), width)
	if info.Signed {
		limit.Rsh(limit, 1)
		min.Neg(new(big.Int).Set(limit))
	}
	max := new(big.Int).Sub(limit, big.NewInt(1))
	if value.Cmp(min) < 0 || value.Cmp(max) > 0 {
		return "", fail(e.Span, "integer literal is out of range for %s", typ)
	}
	c.types[e] = typ
	return typ, nil
}

// parseInteger decodes bootstrap Rust integer syntax without applying a sign.
func parseInteger(text string) (*big.Int, string, error) {
	suffix := ""
	for _, s := range integerSuffixes {
		if strings.HasSuffix(text, s) {
			suffix = s
			text = strings.TrimSuffix(text, s)
			break
		}
	}
	text = strings.ReplaceAll(text, "_", "")
	base := 10
	if len(text) > 2 {
		switch text[:2] {
		case "0x":
			base = 16
			text = text[2:]
		case "0o":
			base = 8
			text = text[2:]
		case "0b":
			base = 2
			text = text[2:]
		}
	}
	v, ok := new(big.Int).SetString(text, base)
	if !ok || v.Sign() < 0 {
		return nil, "", fmt.Errorf("invalid or unsupported integer literal")
	}
	return v, suffix, nil
}

func (c *checker) macro(e *syntax.Expr) error {
	name := strings.TrimSuffix(e.Text, "!")
	if name == "assert" {
		if len(e.Args) != 1 {
			return fail(e.Span, "assert! supports one boolean argument")
		}
		_, err := c.expr(e.Args[0], "bool")
		return err
	}
	if name != "print" && name != "println" {
		return fail(e.Span, "unsupported macro %q", name)
	}
	if len(e.Args) == 0 {
		if name == "println" {
			return nil
		}
		return fail(e.Span, "print! requires a literal format string")
	}
	if e.Args[0].Kind != "string" {
		return fail(e.Args[0].Span, "format must be a string literal")
	}
	if _, err := c.expr(e.Args[0], "&str"); err != nil {
		return err
	}
	format := e.Args[0].Text
	count := 0
	for i := 0; i < len(format); i++ {
		switch format[i] {
		case '{':
			if i+1 < len(format) && format[i+1] == '{' {
				i++
				continue
			}
			if i+1 < len(format) && format[i+1] == '}' {
				count++
				i++
				continue
			}
			return fail(e.Args[0].Span, "only {} format placeholders and escaped braces are supported")
		case '}':
			if i+1 < len(format) && format[i+1] == '}' {
				i++
				continue
			}
			return fail(e.Args[0].Span, "unmatched } in format string")
		}
	}
	if count != len(e.Args)-1 {
		return fail(e.Span, "format expects %d arguments, got %d", count, len(e.Args)-1)
	}
	for _, arg := range e.Args[1:] {
		typ, err := c.expr(arg, "")
		if err != nil {
			return err
		}
		if typ == "()" {
			return fail(arg.Span, "unit values cannot be formatted")
		}
		if structType(typ) {
			return fail(arg.Span, "struct values cannot be formatted without a Display implementation")
		}
		if _, ok := arrayInfo(typ); ok {
			return fail(arg.Span, "array values cannot be formatted without a Display implementation")
		}

	}
	return nil
}
