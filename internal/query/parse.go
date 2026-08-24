package query

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Options contém o subconjunto de parâmetros OData aceito pelo servidor.
type Options struct {
	Filter *Expr
	Orders []Order
	Top    int
	Skip   int
	Count  bool
	Search string
}

type Order struct {
	Field string
	Desc  bool
}

func Parse(values url.Values) (Options, error) {
	var options Options
	if raw := strings.TrimSpace(values.Get("$filter")); raw != "" {
		expr, err := parseFilter(raw)
		if err != nil {
			return Options{}, err
		}
		options.Filter = expr
	}
	if raw := strings.TrimSpace(values.Get("$orderby")); raw != "" {
		orders, err := parseOrder(raw)
		if err != nil {
			return Options{}, err
		}
		options.Orders = orders
	}
	var err error
	options.Top, err = parseNonNegative(values.Get("$top"), "$top")
	if err != nil {
		return Options{}, err
	}
	options.Skip, err = parseNonNegative(values.Get("$skip"), "$skip")
	if err != nil {
		return Options{}, err
	}
	if raw := strings.TrimSpace(values.Get("$count")); raw != "" {
		options.Count, err = strconv.ParseBool(raw)
		if err != nil {
			return Options{}, invalid("$count", "$count deve ser true ou false")
		}
	}
	options.Search = strings.TrimSpace(values.Get("$search"))
	for key := range values {
		if strings.HasPrefix(key, "$") && key != "$filter" && key != "$orderby" && key != "$top" && key != "$skip" && key != "$count" && key != "$search" {
			return Options{}, invalid(key, "parâmetro OData não suportado")
		}
	}
	return options, nil
}

func parseNonNegative(raw, parameter string) (int, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, invalid(parameter, parameter+" deve ser um inteiro maior ou igual a zero")
	}
	return n, nil
}

type parser struct {
	tokens []token
	index  int
}

func parseFilter(raw string) (*Expr, error) {
	tokens, err := lex(raw)
	if err != nil {
		return nil, err
	}
	p := parser{tokens: tokens}
	expr, err := p.parseOr()
	if err != nil {
		return nil, err
	}
	if p.peek().kind != tokenEOF {
		return nil, p.errorf("token inesperado %q", p.peek().text)
	}
	return expr, nil
}

func (p *parser) parseOr() (*Expr, error) {
	left, err := p.parseAnd()
	if err != nil {
		return nil, err
	}
	for strings.EqualFold(p.peek().text, "or") {
		p.next()
		right, err := p.parseAnd()
		if err != nil {
			return nil, err
		}
		left = &Expr{kind: expressionOr, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseAnd() (*Expr, error) {
	left, err := p.parseUnary()
	if err != nil {
		return nil, err
	}
	for strings.EqualFold(p.peek().text, "and") {
		p.next()
		right, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		left = &Expr{kind: expressionAnd, left: left, right: right}
	}
	return left, nil
}

func (p *parser) parseUnary() (*Expr, error) {
	if strings.EqualFold(p.peek().text, "not") {
		p.next()
		inner, err := p.parseUnary()
		if err != nil {
			return nil, err
		}
		return &Expr{kind: expressionNot, left: inner}, nil
	}
	if p.peek().kind == tokenLeftParen {
		p.next()
		inner, err := p.parseOr()
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenRightParen, ")"); err != nil {
			return nil, err
		}
		return inner, nil
	}
	return p.parsePrimary()
}

func (p *parser) parsePrimary() (*Expr, error) {
	field, err := p.expect(tokenWord, "nome do campo")
	if err != nil {
		return nil, err
	}
	if p.peek().kind == tokenLeftParen {
		function := strings.ToLower(field.text)
		if function != "contains" && function != "startswith" && function != "endswith" {
			return nil, p.errorf("função %q não é suportada; use contains, startswith ou endswith", field.text)
		}
		p.next()
		name, err := p.expect(tokenWord, "nome do campo")
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenComma, ","); err != nil {
			return nil, err
		}
		value, err := p.literal()
		if err != nil {
			return nil, err
		}
		if value.Kind != String {
			return nil, p.errorf("o segundo argumento de %s deve ser um texto literal", function)
		}
		if _, err := p.expect(tokenRightParen, ")"); err != nil {
			return nil, err
		}
		return &Expr{kind: expressionFunction, field: name.text, function: function, value: value}, nil
	}
	op, err := p.expect(tokenWord, "operador")
	if err != nil {
		return nil, err
	}
	operator := strings.ToLower(op.text)
	if operator == "in" {
		if _, err := p.expect(tokenLeftParen, "("); err != nil {
			return nil, err
		}
		var values []Value
		for {
			value, err := p.literal()
			if err != nil {
				return nil, err
			}
			values = append(values, value)
			if p.peek().kind != tokenComma {
				break
			}
			p.next()
		}
		if len(values) == 0 {
			return nil, p.errorf("in exige pelo menos um valor")
		}
		if _, err := p.expect(tokenRightParen, ")"); err != nil {
			return nil, err
		}
		return &Expr{kind: expressionCompare, field: field.text, op: operator, values: values}, nil
	}
	switch operator {
	case "eq", "ne", "gt", "ge", "lt", "le":
	default:
		return nil, p.errorf("operador %q não é suportado", op.text)
	}
	value, err := p.literal()
	if err != nil {
		return nil, err
	}
	return &Expr{kind: expressionCompare, field: field.text, op: operator, value: value}, nil
}

func (p *parser) literal() (Value, error) {
	t := p.next()
	switch t.kind {
	case tokenString:
		return StringValue(t.text), nil
	case tokenNumber:
		n, _ := strconv.ParseFloat(t.text, 64)
		return NumberValue(n), nil
	case tokenWord:
		switch strings.ToLower(t.text) {
		case "true":
			return BoolValue(true), nil
		case "false":
			return BoolValue(false), nil
		case "null":
			return NullValue(), nil
		default:
			parsed, err := time.Parse(time.RFC3339, t.text)
			if err == nil {
				return TimeValue(parsed), nil
			}
			return Value{}, p.errorf("literal %q não é texto entre aspas, número, booleano, null ou RFC3339", t.text)
		}
	default:
		return Value{}, p.errorf("esperava um literal")
	}
}

func parseOrder(raw string) ([]Order, error) {
	var out []Order
	seen := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		parts := strings.Fields(item)
		if len(parts) == 0 || len(parts) > 2 {
			return nil, invalid("$orderby", "cada item de $orderby deve ser campo [asc|desc]")
		}
		field := parts[0]
		if seen[field] {
			continue
		}
		order := Order{Field: field}
		if len(parts) == 2 {
			switch strings.ToLower(parts[1]) {
			case "asc":
			case "desc":
				order.Desc = true
			default:
				return nil, invalid("$orderby", "a direção deve ser asc ou desc")
			}
		}
		seen[field] = true
		out = append(out, order)
	}
	return out, nil
}

func (p *parser) peek() token { return p.tokens[p.index] }

func (p *parser) next() token {
	t := p.peek()
	if p.index < len(p.tokens)-1 {
		p.index++
	}
	return t
}

func (p *parser) expect(kind tokenKind, expected string) (token, error) {
	t := p.next()
	if t.kind != kind {
		return token{}, p.errorf("esperava %s", expected)
	}
	return t, nil
}

func (p *parser) errorf(format string, args ...any) error {
	return invalid("$filter", "posição "+strconv.Itoa(p.peek().pos)+": "+sprintf(format, args...))
}

func sprintf(format string, args ...any) string {
	return fmt.Sprintf(format, args...)
}
