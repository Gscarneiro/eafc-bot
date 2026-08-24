package query

import (
	"strconv"
	"strings"
)

type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenWord
	tokenString
	tokenNumber
	tokenLeftParen
	tokenRightParen
	tokenComma
)

type token struct {
	kind tokenKind
	text string
	pos  int
}

func lex(input string) ([]token, error) {
	var out []token
	for i := 0; i < len(input); {
		if input[i] == ' ' || input[i] == '\t' || input[i] == '\r' || input[i] == '\n' {
			i++
			continue
		}
		start := i
		switch input[i] {
		case '(':
			out = append(out, token{kind: tokenLeftParen, text: "(", pos: i})
			i++
		case ')':
			out = append(out, token{kind: tokenRightParen, text: ")", pos: i})
			i++
		case ',':
			out = append(out, token{kind: tokenComma, text: ",", pos: i})
			i++
		case '\'':
			i++
			var b strings.Builder
			closed := false
			for i < len(input) {
				if input[i] != '\'' {
					b.WriteByte(input[i])
					i++
					continue
				}
				if i+1 < len(input) && input[i+1] == '\'' {
					b.WriteByte('\'')
					i += 2
					continue
				}
				i++
				closed = true
				break
			}
			if !closed {
				return nil, invalid("$filter", "texto entre aspas simples não foi fechado")
			}
			out = append(out, token{kind: tokenString, text: b.String(), pos: start})
		default:
			for i < len(input) && input[i] != ' ' && input[i] != '\t' && input[i] != '\r' && input[i] != '\n' && input[i] != '(' && input[i] != ')' && input[i] != ',' {
				i++
			}
			text := input[start:i]
			if _, err := strconv.ParseFloat(text, 64); err == nil {
				out = append(out, token{kind: tokenNumber, text: text, pos: start})
			} else {
				out = append(out, token{kind: tokenWord, text: text, pos: start})
			}
		}
	}
	out = append(out, token{kind: tokenEOF, pos: len(input)})
	return out, nil
}
