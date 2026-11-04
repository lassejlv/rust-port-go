package compiler

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

type arraySpec struct {
	Element string
	Length  int
}

const maxArrayLength = 65536

func arrayInfo(typ string) (arraySpec, bool) {
	if !strings.HasPrefix(typ, "array:") {
		return arraySpec{}, false
	}
	rest := strings.TrimPrefix(typ, "array:")
	separator := strings.IndexByte(rest, ':')
	if separator < 0 {
		return arraySpec{}, false
	}
	count, err := strconv.Atoi(rest[:separator])
	if err != nil || count < 0 || count > maxArrayLength || strconv.Itoa(count) != rest[:separator] || rest[separator+1:] == "" {
		return arraySpec{}, false
	}
	element := rest[separator+1:]
	if strings.HasPrefix(element, "array:") {
		if _, ok := arrayInfo(element); !ok {
			return arraySpec{}, false
		}
	}
	return arraySpec{Element: element, Length: count}, true
}

func arrayType(element string, length int) string { return fmt.Sprintf("array:%d:%s", length, element) }

func parseArrayLength(text string, span syntax.Span) (int, error) {
	value, suffix, err := parseInteger(text)
	if err != nil {
		return 0, fail(span, "array length must be an unsuffixed or usize integer literal")
	}
	if suffix != "" && suffix != "usize" {
		return 0, fail(span, "array length must be an unsuffixed or usize integer literal")
	}
	if !value.IsInt64() || value.Int64() > maxArrayLength {
		return 0, fail(span, "array length exceeds supported maximum %d", maxArrayLength)
	}
	return int(value.Int64()), nil
}

func (c *checker) copyElement(typ string) bool {
	if integerType(typ) || typ == "bool" || c.structs[typ] != nil {
		return true
	}
	if spec, ok := arrayInfo(typ); ok && c.copyElement(spec.Element) {
		c.arrays[typ] = spec
		return true
	}
	return false
}

func (c *checker) registerArray(element string, length int, span syntax.Span) (string, error) {
	if length < 0 || length > maxArrayLength {
		return "", fail(span, "array length exceeds supported maximum %d", maxArrayLength)
	}
	if !c.copyElement(element) {
		return "", fail(span, "unsupported Copy array element type %s", element)
	}
	typ := arrayType(element, length)
	c.arrays[typ] = arraySpec{Element: element, Length: length}
	return typ, nil
}

func (c *checker) arrayExpression(e *syntax.Expr, expected string) (string, error) {
	context, typed := arrayInfo(expected)
	if e.Kind == "repeat" {
		length, err := parseArrayLength(e.Text, e.Span)
		if err != nil {
			return "", err
		}
		if typed && length != context.Length {
			return "", fail(e.Span, "expected array length %d, found %d", context.Length, length)
		}
		elementContext := ""
		if typed {
			elementContext = context.Element
		}
		element, err := c.expr(e.Left, elementContext)
		if err != nil {
			return "", err
		}
		c.diverges[e] = c.diverges[e.Left]
		return c.registerArray(element, length, e.Span)
	}
	if len(e.Args) > maxArrayLength {
		return "", fail(e.Span, "array length exceeds supported maximum %d", maxArrayLength)
	}
	if typed && len(e.Args) != context.Length {
		return "", fail(e.Span, "expected array length %d, found %d", context.Length, len(e.Args))
	}
	element := ""
	if typed {
		element = context.Element
	} else {
		for _, arg := range e.Args {
			if hint := c.hint(arg); hint != "" {
				element = hint
				break
			}
		}
	}
	if len(e.Args) == 0 && element == "" {
		return "", fail(e.Span, "empty array requires an explicit element type")
	}
	for _, arg := range e.Args {
		typ, err := c.expr(arg, element)
		if err != nil {
			return "", err
		}
		if element == "" {
			element = typ
		}
		c.diverges[e] = c.diverges[e] || c.diverges[arg]
	}
	return c.registerArray(element, len(e.Args), e.Span)
}

func (c *checker) indexExpression(e *syntax.Expr, expected string) (string, error) {
	context := ""
	if expected != "" && c.copyElement(expected) {
		if length, ok := arrayExpressionLength(e.Left); ok {
			context = arrayType(expected, length)
		}
	}
	receiver, err := c.expr(e.Left, context)
	if err != nil {
		return "", err
	}
	spec, ok := arrayInfo(receiver)
	if !ok {
		return "", fail(e.Span, "index receiver must be an array")
	}
	if _, err := c.expr(e.Right, "usize"); err != nil {
		return "", err
	}
	if c.bottom[e.Right] {
		return "", fail(e.Span, "value-less divergent indexes are unsupported")
	}

	if err := c.visibleType(spec.Element, e.Span); err != nil {
		return "", err
	}
	c.diverges[e] = c.diverges[e.Left] || c.diverges[e.Right]
	return spec.Element, nil
}

func arrayExpressionLength(e *syntax.Expr) (int, bool) {
	shape, ok := arrayExpressionShape(e)
	if !ok || len(shape) == 0 {
		return 0, false
	}
	return shape[0], true
}

func arrayExpressionShape(e *syntax.Expr) ([]int, bool) {
	if e == nil {
		return nil, false
	}
	switch e.Kind {
	case "array":
		shape := []int{len(e.Args)}
		if len(e.Args) > 0 {
			if child, ok := arrayExpressionShape(e.Args[0]); ok {
				shape = append(shape, child...)
			}
		}
		return shape, true
	case "repeat":
		length, err := parseArrayLength(e.Text, e.Span)
		if err != nil {
			return nil, false
		}
		shape := []int{length}
		if child, ok := arrayExpressionShape(e.Left); ok {
			shape = append(shape, child...)
		}
		return shape, true
	case "index":
		shape, ok := arrayExpressionShape(e.Left)
		if !ok || len(shape) < 2 {
			return nil, false
		}
		return shape[1:], true
	case "block":
		if e.Body != nil {
			return arrayExpressionShape(e.Body.Tail)
		}
	case "if":
		if e.Body != nil && e.Else != nil {
			left, lok := arrayExpressionShape(e.Body.Tail)
			right, rok := arrayExpressionShape(e.Else.Tail)
			if !lok || !rok || len(left) != len(right) {
				return nil, false
			}
			for i := range left {
				if left[i] != right[i] {
					return nil, false
				}
			}
			return left, true
		}
	}
	return nil, false
}

func (c *checker) lengthExpression(e *syntax.Expr) (string, error) {
	receiver, err := c.expr(e.Left, "")
	if err != nil {
		return "", err
	}
	if _, ok := arrayInfo(receiver); !ok {
		return "", fail(e.Span, "len receiver must be an array")
	}
	c.diverges[e] = c.diverges[e.Left]
	return "usize", nil
}

// finishLayouts runs after expression checking so inferred literal-only array
// types participate in the same by-value ordering as named records.
func (c *checker) finishLayouts() error {
	state := make(map[string]int)
	var visit func(string, syntax.Span) error
	visit = func(typ string, span syntax.Span) error {
		if state[typ] == 2 {
			return nil
		}
		structure := c.structs[typ]
		array, isArray := c.arrays[typ]
		if structure == nil && !isArray {
			return nil
		}
		if state[typ] == 1 {
			return fail(span, "recursive by-value struct/array layout involving %s", typ)
		}
		state[typ] = 1
		if structure != nil {
			for _, field := range structure.Fields {
				if err := visit(field.Type, field.Span); err != nil {
					return err
				}
			}
		} else if err := visit(array.Element, span); err != nil {
			return err
		}
		if structure != nil {
			zero := true
			for _, field := range structure.Fields {
				if !c.zeroSized[field.Type] {
					zero = false
					break
				}
			}
			c.zeroSized[typ] = zero
		} else {
			c.zeroSized[typ] = array.Length == 0 || c.zeroSized[array.Element]
		}
		state[typ] = 2
		c.typeOrder = append(c.typeOrder, typ)
		if structure != nil {
			c.structOrder = append(c.structOrder, typ)
		}
		return nil
	}
	for _, s := range c.program.Structs {
		if err := visit("struct:"+s.Name, s.Span); err != nil {
			return err
		}
	}
	keys := make([]string, 0, len(c.arrays))
	for key := range c.arrays {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if err := visit(key, syntax.Span{Line: 1, Column: 1}); err != nil {
			return err
		}
	}
	return nil
}
