package futgg

import (
	"encoding/json"
	"strconv"
	"strings"
)

// O fut.gg é um site de terceiros: os nomes dos campos mudam sem aviso e
// não há contrato público. Em vez de amarrar structs rígidas ao JSON dele,
// decodificamos em node e procuramos o valor por uma lista de nomes
// candidatos. Quando o site renomeia "overall" para "rating", basta somar
// o nome novo à lista — nada quebra.

type node map[string]any

// decodeNodes aceita as três formas que uma listagem costuma ter:
// um array puro, {"data": [...]} ou {"results": [...]}.
func decodeNodes(body []byte) ([]node, error) {
	var asArray []node
	if err := json.Unmarshal(body, &asArray); err == nil {
		return asArray, nil
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, err
	}
	for _, key := range []string{"data", "results", "items", "players", "evolutions", "objectives", "sets", "news"} {
		raw, ok := wrapper[key]
		if !ok {
			continue
		}
		var list []node
		if err := json.Unmarshal(raw, &list); err == nil {
			return list, nil
		}
	}
	// Objeto único: trata como lista de um.
	var single node
	if err := json.Unmarshal(body, &single); err == nil {
		return []node{single}, nil
	}
	return nil, errUnknownShape
}

// jsonUnmarshalNode decodifica um objeto único em node.
func jsonUnmarshalNode(body []byte, dst *node) error {
	return json.Unmarshal(body, dst)
}

// decodeList usa o embrulho aprendido pelo `autoconfig` quando existe, e
// só então cai para a detecção genérica. Isso resolve o caso em que a
// resposta tem várias listas e a heurística escolheria a errada.
func (c *Client) decodeList(body []byte, kind string) ([]node, error) {
	if c != nil && c.cfg.Wrappers != nil {
		if key := c.cfg.Wrappers[kind]; key != "" {
			if list, ok := listAt(body, key); ok {
				return list, nil
			}
		}
	}
	return decodeNodes(body)
}

// listAt segue o embrulho aprendido pelo autoconfig. Ele tem duas formas:
//
//	"data"              — a lista está na chave "data"
//	"data[].evolution"  — a lista está em "data", mas o recurso é o objeto
//	                      "evolution" DENTRO de cada item
//
// A segunda existe porque o fut.gg serve as evoluções embrulhadas junto com
// a carta do jogador: {"data":[{"evolution":{…}, …campos da carta…}]}. Sem
// descer esse nível, o parser leria cartas achando que são evoluções.
func listAt(body []byte, wrapper string) ([]node, bool) {
	listKey, itemKey, nested := strings.Cut(wrapper, "[].")

	var list []node
	if listKey == "" {
		if err := json.Unmarshal(body, &list); err != nil {
			return nil, false
		}
	} else {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(body, &obj); err != nil {
			return nil, false
		}
		raw, ok := obj[listKey]
		if !ok {
			return nil, false
		}
		if err := json.Unmarshal(raw, &list); err != nil {
			return nil, false
		}
	}
	if len(list) == 0 {
		return nil, false
	}
	if !nested {
		return list, true
	}

	out := make([]node, 0, len(list))
	for _, item := range list {
		if sub, ok := item[itemKey].(map[string]any); ok && len(sub) > 0 {
			out = append(out, node(sub))
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

var errUnknownShape = &shapeError{}

type shapeError struct{}

func (e *shapeError) Error() string {
	return "formato de resposta não reconhecido (nem array, nem {data|results|items:[...]})"
}

// get procura a primeira chave existente, aceitando caminho aninhado com
// ponto: get(n, "price.current", "currentPrice", "price").
func (n node) get(keys ...string) any {
	for _, key := range keys {
		if v, ok := n.lookup(key); ok && v != nil {
			return v
		}
	}
	return nil
}

func (n node) lookup(path string) (any, bool) {
	parts := strings.Split(path, ".")
	var cur any = map[string]any(n)
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[p]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

func (n node) str(keys ...string) string { return toStr(n.get(keys...)) }
func (n node) i64(keys ...string) int64  { return int64(n.int(keys...)) }

// float64 é o irmão de int para campos que não são inteiros — o GG Rating
// do fut.gg vem como "92.4", e n.int() truncaria para 92 sem avisar.
func (n node) float64(keys ...string) float64 {
	for _, key := range keys {
		v, ok := n.lookup(key)
		if !ok || v == nil {
			continue
		}
		if f, ok := v.(float64); ok {
			return f
		}
	}
	return 0
}

// int devolve o primeiro candidato que dá um número USÁVEL, e não o primeiro
// que simplesmente existe.
//
// A distinção decide se o seu elenco entra ou não no relatório: no GG Club a
// chave "id" é a string "2567219-919095915063", que não é número nenhum.
// Parando nela, todo jogador chegava com id 0 e era descartado — com o
// "eaId" numérico ali do lado, na lista de alternativas.
//
// Zero legítimo continua valendo: o que faz pular é o valor não converter,
// não o valor ser zero.
func (n node) int(keys ...string) int {
	for _, key := range keys {
		v, ok := n.lookup(key)
		if !ok || v == nil {
			continue
		}
		if iv, ok := toIntOK(v); ok {
			return iv
		}
	}
	return 0
}

func (n node) bool_(keys ...string) bool {
	switch v := n.get(keys...).(type) {
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	}
	return false
}

// strs devolve uma lista de strings, aceitando tanto ["a","b"] quanto
// [{"name":"a"},{"name":"b"}] quanto "a, b".
// ints devolve uma lista de inteiros — usado pelos ids de função
// (rolesPlus/rolesPlusPlus), que o fut.gg manda como array de números.
func (n node) ints(keys ...string) []int {
	v, ok := n.get(keys...).([]any)
	if !ok {
		return nil
	}
	out := make([]int, 0, len(v))
	for _, item := range v {
		if iv, ok := toIntOK(item); ok {
			out = append(out, iv)
		}
	}
	return out
}

func (n node) strs(keys ...string) []string {
	v := n.get(keys...)
	switch t := v.(type) {
	case []any:
		out := make([]string, 0, len(t))
		for _, item := range t {
			switch e := item.(type) {
			case string:
				out = append(out, e)
			case map[string]any:
				if s := toStr(firstOf(e, "name", "shortName", "label", "value", "title")); s != "" {
					out = append(out, s)
				}
			default:
				if s := toStr(item); s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	case string:
		parts := strings.Split(t, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	return nil
}

// nodes devolve uma lista de sub-objetos.
func (n node) nodes(keys ...string) []node {
	v, ok := n.get(keys...).([]any)
	if !ok {
		return nil
	}
	out := make([]node, 0, len(v))
	for _, item := range v {
		if m, ok := item.(map[string]any); ok {
			out = append(out, node(m))
		}
	}
	return out
}

// sub devolve um sub-objeto único.
func (n node) sub(keys ...string) node {
	if m, ok := n.get(keys...).(map[string]any); ok {
		return node(m)
	}
	return node{}
}

func firstOf(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			return v
		}
	}
	return nil
}

func toStr(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case map[string]any:
		return toStr(firstOf(t, "name", "shortName", "label", "value", "title"))
	case nil:
		return ""
	}
	return ""
}

func toInt(v any) int {
	n, _ := toIntOK(v)
	return n
}

// toIntOK converte e diz se deu. O "deu" é o que separa um zero de verdade
// de um valor que não é número — ver node.int.
func toIntOK(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		return int(t), true
	case int:
		return t, true
	case string:
		// Aceita "1.2M", "850K" e "12,500" — formatos comuns de preço.
		return parseCoinStringOK(t)
	case map[string]any:
		return toIntOK(firstOf(t, "value", "amount", "current", "price"))
	}
	return 0, false
}

// parseCoinStringOK entende as abreviações de preço que o fut.gg exibe, e
// diz se o texto era mesmo um número.
func parseCoinStringOK(s string) (int, bool) {
	s = strings.TrimSpace(strings.ReplaceAll(s, ",", ""))
	if s == "" {
		return 0, false
	}
	mult := 1.0
	switch {
	case strings.HasSuffix(s, "M"), strings.HasSuffix(s, "m"):
		mult, s = 1_000_000, s[:len(s)-1]
	case strings.HasSuffix(s, "K"), strings.HasSuffix(s, "k"):
		mult, s = 1_000, s[:len(s)-1]
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return int(f * mult), true
}

// parseCoinString é a forma curta, para quem só quer o número.
func parseCoinString(s string) int {
	n, _ := parseCoinStringOK(s)
	return n
}
