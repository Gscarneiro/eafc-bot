package futgg

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf16"
)

// Um app Next.js moderno (App Router) não usa mais __NEXT_DATA__: ele
// despeja o resultado da renderização do servidor em pedaços de
// `self.__next_f.push([1,"..."])`. Os dados que a página mostra estão ali,
// dentro do HTML que o servidor já mandou — não é preciso chamar /api/*
// para tê-los, porque a página já os recebeu.
//
// Este arquivo extrai esse payload e devolve os objetos JSON que houver
// dentro, para o classificador identificar o que é jogador, evolução, SBC.

var (
	flightChunk = regexp.MustCompile(`self\.__next_f\.push\(\[1,\s*"((?:[^"\\]|\\.)*)"\s*\]\)`)
	nextDataTag = regexp.MustCompile(`(?s)<script[^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>`)
	ldJSONTag   = regexp.MustCompile(`(?s)<script[^>]*type="application/ld\+json"[^>]*>(.*?)</script>`)
)

// EmbeddedPayload é um blob JSON achado dentro do HTML.
type EmbeddedPayload struct {
	Source  string // "__NEXT_DATA__", "flight" ou "ld+json"
	Objects []map[string]any
}

// ExtractEmbedded varre o HTML atrás de todo dado estruturado que o
// servidor já entregou.
func ExtractEmbedded(html []byte) []EmbeddedPayload {
	var out []EmbeddedPayload

	if m := nextDataTag.FindSubmatch(html); m != nil {
		if objs := objectsIn(m[1]); len(objs) > 0 {
			out = append(out, EmbeddedPayload{Source: "__NEXT_DATA__", Objects: objs})
		}
	}

	for _, m := range ldJSONTag.FindAllSubmatch(html, -1) {
		if objs := objectsIn(m[1]); len(objs) > 0 {
			out = append(out, EmbeddedPayload{Source: "ld+json", Objects: objs})
		}
	}

	// Os pedaços do flight são concatenados: um objeto pode começar num
	// push e terminar no seguinte, então juntar antes de varrer é o que
	// evita perder justamente os objetos grandes (os de jogador).
	var flight strings.Builder
	for _, m := range flightChunk.FindAllSubmatch(html, -1) {
		flight.WriteString(unescapeJS(string(m[1])))
	}
	if flight.Len() > 0 {
		if objs := scanJSONObjects(flight.String()); len(objs) > 0 {
			out = append(out, EmbeddedPayload{Source: "flight", Objects: objs})
		}
	}
	return out
}

func objectsIn(raw []byte) []map[string]any {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return scanJSONObjects(string(raw))
	}
	return collectObjects(v, 0)
}

// collectObjects achata a árvore em objetos, que é o que o classificador
// consome.
func collectObjects(v any, depth int) []map[string]any {
	if depth > 12 {
		return nil
	}
	switch t := v.(type) {
	case map[string]any:
		out := []map[string]any{t}
		for _, sub := range t {
			out = append(out, collectObjects(sub, depth+1)...)
		}
		return out
	case []any:
		var out []map[string]any
		for _, item := range t {
			out = append(out, collectObjects(item, depth+1)...)
		}
		return out
	}
	return nil
}

// scanJSONObjects acha regiões {...} balanceadas dentro de um texto que
// não é JSON puro — o payload do flight é justamente isso: JSON solto
// misturado com marcadores do framework.
func scanJSONObjects(s string) []map[string]any {
	var out []map[string]any
	for i := 0; i < len(s); i++ {
		if s[i] != '{' {
			continue
		}
		end := matchBrace(s, i)
		if end < 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(s[i:end+1]), &m); err == nil && len(m) >= 3 {
			out = append(out, m)
			// Não pula o bloco inteiro: objetos aninhados também
			// interessam, e um jogador costuma estar dentro de outro.
			out = append(out, collectObjects(m, 1)...)
			i = end
		}
		if len(out) > 4000 {
			break // teto de segurança para páginas gigantes
		}
	}
	return dedupeObjects(out)
}

// matchBrace acha a chave que fecha, respeitando strings e escapes.
func matchBrace(s string, start int) int {
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
		// Objeto absurdamente grande quase certamente não é um registro.
		if i-start > 2_000_000 {
			return -1
		}
	}
	return -1
}

func dedupeObjects(objs []map[string]any) []map[string]any {
	seen := make(map[string]bool, len(objs))
	out := objs[:0]
	for _, o := range objs {
		if len(o) < 3 {
			continue
		}
		b, err := json.Marshal(o)
		if err != nil || len(b) > 100_000 {
			continue
		}
		key := string(b)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, o)
	}
	return out
}

// unescapeJS desfaz os escapes de uma string literal de JavaScript,
// incluindo os pares substitutos de \uXXXX — sem isso, todo acento vira
// lixo e nome de jogador não bate com nada.
func unescapeJS(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i+1 >= len(s) {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case '"':
			b.WriteByte('"')
		case '\'':
			b.WriteByte('\'')
		case '\\':
			b.WriteByte('\\')
		case '/':
			b.WriteByte('/')
		case 'u':
			if i+4 < len(s) {
				r := hex4(s[i+1 : i+5])
				i += 4
				if utf16.IsSurrogate(rune(r)) && i+6 < len(s) && s[i+1] == '\\' && s[i+2] == 'u' {
					lo := hex4(s[i+3 : i+7])
					if dec := utf16.DecodeRune(rune(r), rune(lo)); dec != 0xFFFD {
						b.WriteRune(dec)
						i += 6
						continue
					}
				}
				b.WriteRune(rune(r))
			}
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func hex4(s string) int {
	v := 0
	for i := 0; i < len(s) && i < 4; i++ {
		v <<= 4
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			v |= int(c - '0')
		case c >= 'a' && c <= 'f':
			v |= int(c-'a') + 10
		case c >= 'A' && c <= 'F':
			v |= int(c-'A') + 10
		default:
			return 0
		}
	}
	return v
}
