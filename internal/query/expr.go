package query

import "strings"

type expressionKind uint8

const (
	expressionCompare expressionKind = iota
	expressionFunction
	expressionAnd
	expressionOr
	expressionNot
)

// Expr é a árvore imutável produzida por Parse. Campos são resolvidos pelo
// Schema durante Apply, mantendo o parser desacoplado dos tipos da aplicação.
type Expr struct {
	kind     expressionKind
	left     *Expr
	right    *Expr
	field    string
	op       string
	value    Value
	values   []Value
	function string
}

func (e *Expr) Eval(resolve func(string) (Value, bool)) bool {
	if e == nil {
		return true
	}
	switch e.kind {
	case expressionAnd:
		return e.left.Eval(resolve) && e.right.Eval(resolve)
	case expressionOr:
		return e.left.Eval(resolve) || e.right.Eval(resolve)
	case expressionNot:
		return !e.left.Eval(resolve)
	case expressionCompare:
		left, ok := resolve(e.field)
		if !ok {
			return false
		}
		if e.op == "in" {
			for _, value := range e.values {
				if valueMatches(left, value, "eq") {
					return true
				}
			}
			return false
		}
		return valueMatches(left, e.value, e.op)
	case expressionFunction:
		left, ok := resolve(e.field)
		if !ok || left.Kind != String || e.value.Kind != String {
			return false
		}
		actual, expected := Fold(left.S), Fold(e.value.S)
		switch e.function {
		case "contains":
			return strings.Contains(actual, expected)
		case "startswith":
			return strings.HasPrefix(actual, expected)
		case "endswith":
			return strings.HasSuffix(actual, expected)
		default:
			return false
		}
	default:
		return false
	}
}

func (e *Expr) fields(out map[string]bool) {
	if e == nil {
		return
	}
	if e.field != "" {
		out[e.field] = true
	}
	e.left.fields(out)
	e.right.fields(out)
}
