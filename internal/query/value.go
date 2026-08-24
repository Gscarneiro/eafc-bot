package query

import (
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Kind é o conjunto pequeno de tipos que o subconjunto OData suporta.
type Kind uint8

const (
	Invalid Kind = iota
	Null
	String
	Number
	Boolean
	Time
)

// Value é uma união explícita; o motor não usa interface{} para comparar
// campos, o que deixa conversões inesperadas visíveis no parser.
type Value struct {
	Kind Kind
	S    string
	N    float64
	B    bool
	T    time.Time
}

func StringValue(value string) Value  { return Value{Kind: String, S: value} }
func NumberValue(value float64) Value { return Value{Kind: Number, N: value} }
func IntValue(value int) Value        { return NumberValue(float64(value)) }
func BoolValue(value bool) Value      { return Value{Kind: Boolean, B: value} }
func TimeValue(value time.Time) Value { return Value{Kind: Time, T: value} }
func NullValue() Value                { return Value{Kind: Null} }

func (v Value) String() string {
	switch v.Kind {
	case String:
		return v.S
	case Number:
		return strconv.FormatFloat(v.N, 'f', -1, 64)
	case Boolean:
		return strconv.FormatBool(v.B)
	case Time:
		return v.T.Format(time.RFC3339)
	case Null:
		return "null"
	default:
		return ""
	}
}

// Fold torna buscas e comparações textuais estáveis para o português e para
// nomes de cartas. A normalização completa Unicode exigiria uma dependência;
// esta tabela cobre os diacríticos usados pelo jogo e pelo relatório.
func Fold(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		switch r {
		case 'á', 'à', 'â', 'ã', 'ä':
			r = 'a'
		case 'é', 'è', 'ê', 'ë':
			r = 'e'
		case 'í', 'ì', 'î', 'ï':
			r = 'i'
		case 'ó', 'ò', 'ô', 'õ', 'ö':
			r = 'o'
		case 'ú', 'ù', 'û', 'ü':
			r = 'u'
		case 'ç':
			r = 'c'
		case 'ñ':
			r = 'n'
		default:
			if unicode.IsSpace(r) {
				r = ' '
			}
		}
		b.WriteRune(r)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

func compareValues(left, right Value) int {
	if left.Kind == Null || right.Kind == Null {
		switch {
		case left.Kind == Null && right.Kind == Null:
			return 0
		case left.Kind == Null:
			return -1
		default:
			return 1
		}
	}
	if left.Kind == String && right.Kind == String {
		return strings.Compare(Fold(left.S), Fold(right.S))
	}
	if left.Kind == Time && right.Kind == Time {
		if left.T.Before(right.T) {
			return -1
		}
		if left.T.After(right.T) {
			return 1
		}
		return 0
	}
	if left.Kind == Number && right.Kind == Number {
		if left.N < right.N {
			return -1
		}
		if left.N > right.N {
			return 1
		}
		return 0
	}
	if left.Kind == Boolean && right.Kind == Boolean {
		if left.B == right.B {
			return 0
		}
		if !left.B {
			return -1
		}
		return 1
	}
	// Tipos incompatíveis não casam. A ordenação ainda precisa ser total e
	// previsível para que uma consulta mal tipada nunca cause um panic.
	return strings.Compare(left.String(), right.String())
}

func valueMatches(left, right Value, op string) bool {
	cmp := compareValues(left, right)
	switch op {
	case "eq":
		return left.Kind == right.Kind && cmp == 0
	case "ne":
		return left.Kind != right.Kind || cmp != 0
	case "gt":
		return cmp > 0
	case "ge":
		return cmp >= 0
	case "lt":
		return cmp < 0
	case "le":
		return cmp <= 0
	default:
		return false
	}
}
