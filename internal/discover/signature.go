package discover

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// A classificação não olha o nome da rota — olha o formato do JSON.
// Nome de rota muda a cada temporada; um objeto que tem um inteiro entre
// 40 e 99 chamado algo como "overall" ao lado de uma string que é uma
// posição de futebol continua sendo um jogador, chame-se a rota como se
// chamar. É isso que faz a descoberta sobreviver ao FC 27.

// Kind é o tipo esperado de um campo.
type Kind int

const (
	KAny Kind = iota
	KInt
	KString
	KBool
	KArray
	KObject
	KDate
	// KArrayOf é uma lista cujos ELEMENTOS batem com outra assinatura.
	// É a prova mais forte que existe aqui: qualquer JSON tem strings e
	// listas, mas só um clube contém uma lista que é, ela própria, um
	// conjunto de cartas de jogador. Sem isso o objeto do clube era
	// classificado como "evolutions", porque a lista de elenco casava por
	// acaso com a regra de "níveis".
	KArrayOf
)

// Rule descreve um campo que uma assinatura espera encontrar.
type Rule struct {
	Field    string   // nome lógico usado pelo bot ("rating", "position"...)
	Names    []string // pedaços de nome que sugerem esse campo
	Kind     Kind
	Min, Max int      // faixa aceita, quando Kind == KInt
	ArrayOf  string   // assinatura que os elementos devem satisfazer, quando Kind == KArrayOf
	Vocab    []string // valores aceitos, quando Kind == KString
	NotVocab []string // valores que DESQUALIFICAM (posição não é nome de jogador)
	MinLen   int      // tamanho mínimo, para strings
	Shape    Shape    // forma esperada do texto (slug x texto legível)
	Distinct bool     // valores devem ser majoritariamente únicos entre as amostras
	Weight   float64
	Required bool // sem ele, a assinatura não bate de jeito nenhum
	// NameRequired exige que o NOME da chave também aponte para o campo.
	//
	// Existe para os campos cuja forma não prova nada. "Uma lista" não é
	// evidência de "níveis de evolução": todo JSON tem listas, e sem esta
	// trava as tags de uma notícia e as categorias de uma tier list viravam
	// os níveis e os requisitos de uma evolução — a assinatura de evoluções
	// acumulava evidência em cima de qualquer objeto com dois arrays e um
	// inteiro, e ganhava de assinaturas específicas na própria rota delas.
	//
	// Vale para lista, custo e data. NÃO vale para campo com forma própria
	// (posição, overall, lista de cartas), que é onde o valor prova sozinho
	// e o nome pode ser o que o site quiser.
	NameRequired bool
}

// Shape distingue strings que o tipo sozinho não distingue.
type Shape int

const (
	ShapeAny  Shape = iota
	ShapeSlug       // minúsculo, sem espaço: "ponta-explosiva"
	ShapeText       // legível: tem espaço ou maiúscula ("Ponta Explosiva")
)

func matchesShape(s string, want Shape) bool {
	switch want {
	case ShapeSlug:
		if strings.ContainsAny(s, " \t") {
			return false
		}
		return s == strings.ToLower(s)
	case ShapeText:
		return strings.ContainsAny(s, " ") || s != strings.ToLower(s)
	}
	return true
}

// Signature é o retrato de um tipo de recurso.
type Signature struct {
	Name  string
	Rules []Rule
	// MinScore é a fração das regras atendidas.
	MinScore float64
	// MinEvidence é a quantidade ABSOLUTA de evidência forte exigida, e é
	// o que impede falso positivo: uma assinatura curta e frouxa (título +
	// data + slug) casa com quase qualquer JSON e tira nota alta na fração,
	// mas nunca acumula evidência. Ver evidenceFactor.
	MinEvidence float64
}

// Match devolve o quanto as amostras batem com a assinatura, e de quebra
// qual chave real corresponde a cada campo lógico — a classificação e a
// descoberta do mapa de campos saem do mesmo passe.
func (s Signature) Match(samples []map[string]any) (score, evidence float64, fields map[string]string) {
	if len(samples) == 0 {
		return 0, 0, nil
	}
	flat := make([]map[string]any, 0, len(samples))
	for _, s := range samples {
		flat = append(flat, flatten(s, 2))
	}

	fields = map[string]string{}
	var got, total float64
	// Duas listas separadas, e a distinção é o coração da coisa:
	//   used  — chave já reivindicada COM CERTEZA por um campo; ninguém mais toca.
	//   ambig — chave que empatou entre campos; ninguém grava, mas todos contam.
	// Reconhecer o recurso e resolver cada campo são perguntas diferentes:
	// um objeto com seis inteiros entre 15 e 99 é claramente um jogador
	// mesmo quando é impossível dizer qual deles é finalização.
	used := map[string]bool{}
	ambig := map[string]bool{}

	for _, rule := range s.Rules {
		total += rule.Weight
		key, conf, tied := bestKey(flat, rule, used)
		if key == "" {
			if rule.Required {
				return 0, 0, nil // campo obrigatório ausente derruba a assinatura
			}
			continue
		}

		// A nota e a evidência sobem de qualquer jeito: existe um campo
		// com essa cara, e é isso que a classificação pergunta.
		got += rule.Weight * conf
		evidence += rule.Weight * conf * evidenceFactor(rule)

		if len(tied) > 0 || ambig[key] {
			// Ambíguo não vira chute. Gravar um palpite faria o bot
			// comparar ritmo com desarme e recomendar a troca errada em
			// silêncio; sem gravar, o parser cai nos aliases do código,
			// que é o padrão seguro.
			ambig[key] = true
			for _, k := range tied {
				ambig[k] = true
			}
			continue
		}
		fields[rule.Field] = key
		used[key] = true
	}
	if total == 0 {
		return 0, 0, nil
	}
	return got / total, evidence, fields
}

// evidenceFactor mede o quanto ATENDER a regra prova alguma coisa.
// Casar com uma string qualquer não prova nada — todo JSON tem strings.
// Casar com um valor de um vocabulário fechado de 15 posições de futebol,
// ou com um inteiro na faixa exata de um overall, é evidência de verdade.
func evidenceFactor(r Rule) float64 {
	switch r.Kind {
	case KString:
		if len(r.Vocab) > 0 {
			return 1.0
		}
		return 0.3
	case KInt:
		if r.Max > r.Min && r.Max-r.Min <= 100 {
			return 0.9
		}
		return 0.45
	case KDate:
		return 0.8
	case KArray:
		// Uma lista qualquer é a coisa mais barata que existe num JSON:
		// quase todo objeto tem uma. Vale pouco por si só — o que a torna
		// evidência é o NOME da chave (ver Rule.NameRequired) ou o conteúdo
		// dos elementos (KArrayOf).
		return 0.25
	case KArrayOf:
		return 2.0
	case KBool:
		return 0.4
	case KObject:
		return 0.4
	}
	return 0.15 // KAny
}

// bestKey procura, entre todas as chaves das amostras, a que melhor
// atende a regra. Devolve a chave e a confiança de 0 a 1.
func bestKey(samples []map[string]any, rule Rule, used map[string]bool) (key string, conf float64, tied []string) {
	type scored struct {
		key  string
		conf float64
	}
	seen := map[string]bool{}
	var cands []scored

	for _, s := range samples {
		for key := range s {
			if seen[key] || used[key] {
				continue
			}
			seen[key] = true
			if rule.NameRequired && nameMatch(key, rule.Names) == 0 {
				continue
			}
			if conf := scoreKey(samples, key, rule); conf > 0 {
				cands = append(cands, scored{key, conf})
			}
		}
	}
	if len(cands) == 0 {
		return "", 0, nil
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].conf != cands[j].conf {
			return cands[i].conf > cands[j].conf
		}
		// Empate: prefere a chave mais rasa e mais curta.
		di, dj := strings.Count(cands[i].key, "."), strings.Count(cands[j].key, ".")
		if di != dj {
			return di < dj
		}
		return len(cands[i].key) < len(cands[j].key)
	})
	// Empate técnico sem nenhum nome ajudando: não há como saber qual é
	// qual. Devolvemos TODAS as chaves empatadas — e não só a primeira —
	// porque elas precisam ser retiradas de circulação juntas. Se só a
	// vencedora saísse, a regra seguinte herdaria a sobra e pareceria
	// confiante: era assim que "weak_foot" acabava valendo o campo de
	// movimentos de habilidade.
	if len(cands) > 1 &&
		cands[0].conf-cands[1].conf < 0.02 &&
		nameMatch(cands[0].key, rule.Names) == 0 {
		tied = append(tied, cands[0].key)
		for _, c := range cands[1:] {
			if cands[0].conf-c.conf < 0.02 {
				tied = append(tied, c.key)
			}
		}
		return cands[0].key, cands[0].conf, tied
	}
	return cands[0].key, cands[0].conf, nil
}

// scoreKey combina duas evidências independentes: o nome sugere o campo,
// e os valores se comportam como o campo. Valor pesa mais que nome —
// é o que permite achar "ovr" ou "r" quando o nome não ajuda em nada.
func scoreKey(samples []map[string]any, key string, rule Rule) float64 {
	nameScore := nameMatch(key, rule.Names)

	var valid, present int
	for _, s := range samples {
		v, ok := s[key]
		if !ok || v == nil {
			continue
		}
		present++
		if valueMatches(v, rule) {
			valid++
		}
	}
	if present == 0 {
		return 0
	}
	valueScore := float64(valid) / float64(present)

	// Valor incompatível na maioria das amostras invalida, por mais
	// sugestivo que seja o nome.
	if valueScore < 0.6 {
		return 0
	}
	// Nome irrelevante ainda passa, com confiança menor.
	conf := 0.65*valueScore + 0.35*nameScore

	// Unicidade separa campos que o nome não separa: entre "label" e
	// "tier", só um é diferente em cada registro — esse é o nome do
	// jogador; o outro é a raridade, que se repete.
	if rule.Distinct && present > 1 {
		uniq := make(map[string]bool, present)
		for _, s := range samples {
			if v, ok := s[key]; ok && v != nil {
				uniq[fmt.Sprint(v)] = true
			}
		}
		ratio := float64(len(uniq)) / float64(present)
		if ratio < 1 {
			conf *= ratio
		}
	}

	// Presença consistente é sinal de campo de verdade, não de exceção.
	coverage := float64(present) / float64(len(samples))
	return conf * (0.7 + 0.3*coverage)
}

func nameMatch(key string, names []string) float64 {
	k := strings.ToLower(key)
	// Só o último segmento importa para "position.shortLabel".
	if i := strings.LastIndexByte(k, '.'); i >= 0 {
		k = k[i+1:]
	}
	k = strings.NewReplacer("_", "", "-", "").Replace(k)

	best := 0.0
	for _, n := range names {
		n = strings.NewReplacer("_", "", "-", "").Replace(strings.ToLower(n))
		switch {
		case k == n:
			return 1.0
		case strings.HasPrefix(k, n) || strings.HasSuffix(k, n):
			if best < 0.8 {
				best = 0.8
			}
		case strings.Contains(k, n):
			if best < 0.6 {
				best = 0.6
			}
		default:
			// "heading" e "headline" não são iguais nem um contém o outro,
			// mas dividem "head". Radical comum é sinal legítimo, e sem ele
			// o título de uma notícia acaba lido como resumo.
			if cp := commonPrefix(k, n); cp >= 4 && cp*2 >= min(len(k), len(n)) {
				if best < 0.5 {
					best = 0.5
				}
			}
		}
	}
	return best
}

func commonPrefix(a, b string) int {
	i := 0
	for i < len(a) && i < len(b) && a[i] == b[i] {
		i++
	}
	return i
}

func valueMatches(v any, rule Rule) bool {
	switch rule.Kind {
	case KAny:
		return true
	case KInt:
		n, ok := asInt(v)
		if !ok {
			return false
		}
		if rule.Min != 0 || rule.Max != 0 {
			return n >= rule.Min && n <= rule.Max
		}
		return true
	case KString:
		s, ok := v.(string)
		if !ok {
			// Objeto com nome dentro conta como string ({"name":"ST"}).
			if m, ok2 := v.(map[string]any); ok2 {
				for _, k := range []string{"name", "shortLabel", "label", "value", "shortName"} {
					if inner, ok3 := m[k].(string); ok3 {
						s, ok = inner, true
						break
					}
				}
			}
			if !ok {
				return false
			}
		}
		s = strings.TrimSpace(s)
		if s == "" || len(s) < rule.MinLen {
			return false
		}
		for _, bad := range rule.NotVocab {
			if strings.EqualFold(s, bad) {
				return false
			}
		}
		// Nenhum campo de texto é um timestamp. Sem esta linha, "endsAt"
		// concorre com "label" pela vaga de nome.
		if looksLikeDate(s) {
			return false
		}
		if !matchesShape(s, rule.Shape) {
			return false
		}
		if len(rule.Vocab) > 0 {
			for _, w := range rule.Vocab {
				if strings.EqualFold(strings.TrimSpace(s), w) {
					return true
				}
			}
			return false
		}
		return true
	case KBool:
		_, ok := v.(bool)
		return ok
	case KArray:
		a, ok := v.([]any)
		return ok && len(a) > 0
	case KObject:
		_, ok := v.(map[string]any)
		return ok
	case KDate:
		return looksLikeDate(v)
	case KArrayOf:
		return arrayMatchesSignature(v, rule.ArrayOf)
	}
	return false
}

// arrayMatchesSignature roda outra assinatura sobre os elementos da lista.
// A recursão termina porque nenhuma assinatura referenciada usa KArrayOf.
func arrayMatchesSignature(v any, sigName string) bool {
	raw, ok := v.([]any)
	if !ok || len(raw) == 0 {
		return false
	}
	objs := make([]map[string]any, 0, len(raw))
	for i, item := range raw {
		if i >= 10 {
			break
		}
		if m, ok := item.(map[string]any); ok {
			objs = append(objs, m)
		}
	}
	if len(objs) == 0 {
		return false
	}
	for _, sig := range Signatures {
		if sig.Name != sigName {
			continue
		}
		sc, ev, _ := sig.Match(objs)
		return sc >= sig.MinScore && ev >= sig.MinEvidence
	}
	return false
}

// looksLikeDate aceita ISO-8601 e epoch em segundos ou milissegundos,
// que são as três formas em que datas aparecem nessas APIs.
func looksLikeDate(v any) bool {
	switch t := v.(type) {
	case string:
		s := strings.TrimSpace(t)
		if len(s) < 8 || len(s) > 40 {
			return false
		}
		for _, layout := range []string{
			"2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05.999Z07:00",
			"2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02",
			"02/01/2006", "01/02/2006",
		} {
			if _, err := time.Parse(layout, s); err == nil {
				return true
			}
		}
		return false
	case float64:
		n := int64(t)
		// ~2001 a ~2035, em segundos ou em milissegundos.
		return (n > 1_000_000_000 && n < 2_100_000_000) ||
			(n > 1_000_000_000_000 && n < 2_100_000_000_000)
	}
	return false
}

func asInt(v any) (int, bool) {
	switch t := v.(type) {
	case float64:
		if t != float64(int(t)) {
			return 0, false
		}
		return int(t), true
	case int:
		return t, true
	}
	return 0, false
}

// flatten achata o objeto até a profundidade dada, com chaves em notação
// de ponto: {"position":{"shortLabel":"ST"}} vira {"position.shortLabel":"ST"}.
// O nível de cima é mantido também, porque a regra pode querer o objeto.
func flatten(m map[string]any, depth int) map[string]any {
	out := map[string]any{}
	var walk func(prefix string, m map[string]any, d int)
	walk = func(prefix string, m map[string]any, d int) {
		for k, v := range m {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			out[key] = v
			if d > 1 {
				if sub, ok := v.(map[string]any); ok {
					walk(key, sub, d-1)
				}
			}
		}
	}
	walk("", m, depth)
	return out
}

var positionVocab = []string{
	"GK", "RB", "LB", "CB", "RWB", "LWB", "CDM", "CM", "CAM",
	"RM", "LM", "RW", "LW", "CF", "ST",
}

// Signatures são os retratos que a descoberta sabe reconhecer.
// A ordem não importa: a rota fica com a assinatura de maior nota.
var Signatures = []Signature{
	{
		Name:        "players",
		MinScore:    0.42,
		MinEvidence: 4.0,
		Rules: []Rule{
			{Field: "id", Names: []string{"eaid", "id", "resourceid", "definitionid"},
				Kind: KInt, Min: 1000, Max: 2_000_000_000, Distinct: true, Weight: 1.5, Required: true},
			{Field: "rating", Names: []string{"overall", "rating", "ovr"},
				Kind: KInt, Min: 40, Max: 99, Weight: 3, Required: true},
			{Field: "position", Names: []string{"position", "pos", "mainposition"},
				Kind: KString, Vocab: positionVocab, Weight: 3, Required: true},
			{Field: "name", Names: []string{"name", "commonname", "shortname", "playername"},
				Kind: KString, MinLen: 3, NotVocab: positionVocab, Distinct: true, Weight: 2, Required: true},
			// Estes dois vêm ANTES dos atributos de propósito: valem 1 a 5 e
			// seriam confundidos com atributos se os atributos os pegassem
			// primeiro. Uma chave só pode servir a um campo.
			{Field: "skill_moves", Names: []string{"skillmoves", "skillmovelevel", "sm"}, Kind: KInt, Min: 1, Max: 5, Weight: 0.5},
			{Field: "weak_foot", Names: []string{"weakfoot", "wf"}, Kind: KInt, Min: 1, Max: 5, Weight: 0.5},
			// Faixa 15..99: atributo de carta nunca é de um dígito.
			{Field: "pace", Names: []string{"pace", "pac", "attributepace"}, Kind: KInt, Min: 15, Max: 99, Weight: 1.5},
			{Field: "shooting", Names: []string{"shooting", "sho"}, Kind: KInt, Min: 15, Max: 99, Weight: 1},
			{Field: "passing", Names: []string{"passing", "pas"}, Kind: KInt, Min: 15, Max: 99, Weight: 1},
			{Field: "dribbling", Names: []string{"dribbling", "dri"}, Kind: KInt, Min: 15, Max: 99, Weight: 1},
			{Field: "defending", Names: []string{"defending", "def"}, Kind: KInt, Min: 15, Max: 99, Weight: 1},
			{Field: "physical", Names: []string{"physicality", "physical", "phy"}, Kind: KInt, Min: 15, Max: 99, Weight: 1},
			{Field: "price", Names: []string{"price", "currentprice", "lowestbin", "bin", "coins"},
				Kind: KInt, Min: 0, Max: 20_000_000, Weight: 2},
			{Field: "version", Names: []string{"rarity", "version", "cardtype"}, Kind: KString, Weight: 1},
			{Field: "club", Names: []string{"club", "team"}, Kind: KString, Weight: 0.5},
			{Field: "league", Names: []string{"league"}, Kind: KString, Weight: 0.5},
			{Field: "nation", Names: []string{"nation", "country"}, Kind: KString, Weight: 0.5},
		},
	},
	{
		Name:        "evolutions",
		MinScore:    0.45,
		MinEvidence: 2.5,
		Rules: []Rule{
			{Field: "name", Names: []string{"name", "title", "label"}, Kind: KString, Shape: ShapeText, Distinct: true, Weight: 2, Required: true},
			{Field: "levels", Names: []string{"levels", "tiers", "stages", "steps"},
				Kind: KArray, NameRequired: true, Weight: 3},
			{Field: "requirements", Names: []string{"requirements", "entryrequirements", "requirementgroups"},
				Kind: KArray, NameRequired: true, Weight: 3},
			{Field: "slug", Names: []string{"slug", "path", "handle"}, Kind: KString, Shape: ShapeSlug, Weight: 1},
			{Field: "coin_cost", Names: []string{"coincost", "coinscost", "pricecoins", "costcoins", "coins", "cost"},
				Kind: KInt, Min: 0, Max: 5_000_000, NameRequired: true, Weight: 2.5},
			{Field: "point_cost", Names: []string{"pointcost", "pointscost", "pricepoints", "costpoints"},
				Kind: KInt, Min: 0, Max: 100_000, NameRequired: true, Weight: 0.5},
			{Field: "expires_at", Names: []string{"expiresat", "endsat", "enddate", "endtime"},
				Kind: KDate, NameRequired: true, Weight: 1},
		},
	},
	{
		Name:        "sbcs",
		MinScore:    0.45,
		MinEvidence: 2.0,
		Rules: []Rule{
			{Field: "name", Names: []string{"name", "title", "label"}, Kind: KString, Shape: ShapeText, Distinct: true, Weight: 2, Required: true},
			{Field: "challenges", Names: []string{"challenges", "segments", "sets", "sbcs"},
				Kind: KArray, NameRequired: true, Weight: 3},
			{Field: "solution_cost", Names: []string{"cheapestsolution", "solutioncost", "estimatedcost", "cost", "price"},
				Kind: KInt, Min: 0, Max: 20_000_000, NameRequired: true, Weight: 2.5},
			{Field: "repeatable", Names: []string{"repeatable", "isrepeatable"}, Kind: KBool, NameRequired: true, Weight: 1},
			{Field: "rewards", Names: []string{"rewards", "reward", "awards", "award"},
				Kind: KArray, NameRequired: true, Weight: 1.5},
			{Field: "expires_at", Names: []string{"expiresat", "endsat", "enddate", "endtime"},
				Kind: KDate, NameRequired: true, Weight: 1},
			{Field: "slug", Names: []string{"slug", "path", "handle"}, Kind: KString, Shape: ShapeSlug, Weight: 0.5},
		},
	},
	{
		Name:        "objectives",
		MinScore:    0.45,
		MinEvidence: 2.0,
		Rules: []Rule{
			{Field: "name", Names: []string{"name", "title", "label"}, Kind: KString, Shape: ShapeText, Distinct: true, Weight: 2, Required: true},
			{Field: "tasks", Names: []string{"tasks", "objectives", "challenges", "milestones", "missions"},
				Kind: KArray, NameRequired: true, Weight: 3},
			{Field: "rewards", Names: []string{"rewards", "reward", "awards", "award"},
				Kind: KArray, NameRequired: true, Weight: 2},
			{Field: "group", Names: []string{"group", "category", "campaign", "season"},
				Kind: KString, NameRequired: true, Weight: 1.8},
			{Field: "expires_at", Names: []string{"expiresat", "endsat", "enddate", "endtime"},
				Kind: KDate, NameRequired: true, Weight: 1.5},
		},
	},
	{
		Name:        "news",
		MinScore:    0.5,
		MinEvidence: 2.0,
		Rules: []Rule{
			{Field: "published_at", Names: []string{"publishedat", "createdat", "date", "publishdate", "postedon"},
				Kind: KDate, Weight: 2.5, Required: true},
			{Field: "title", Names: []string{"title", "headline", "name"},
				Kind: KString, MinLen: 6, Shape: ShapeText, Distinct: true, Weight: 2.5, Required: true},
			{Field: "slug", Names: []string{"slug", "path", "url", "handle"}, Kind: KString, Shape: ShapeSlug, Weight: 2},
			{Field: "summary", Names: []string{"summary", "excerpt", "description", "subtitle", "blurb"},
				Kind: KString, MinLen: 12, Shape: ShapeText, Weight: 1.5},
		},
	},
	{
		Name:        "club",
		MinScore:    0.45,
		MinEvidence: 2.5,
		Rules: []Rule{
			// "Contém cartas de jogador" é a forma; só ela não basta. Metade
			// da API do fut.gg devolve listas de cartas embrulhadas em algum
			// objeto — /hotness/, /live-hub/, as evoluções em alta — e todas
			// casavam com esta assinatura. O que faz um CLUBE é o resto:
			// moedas, escalação, plataforma, gamertag. Nenhum desses tem
			// forma própria (é um inteiro, um objeto, uma string), então o
			// nome da chave é a única evidência real e passa a ser exigido —
			// senão um objeto chamado "data" virava a escalação.
			{Field: "players", Names: []string{"players", "club", "items", "cards", "roster", "squad"},
				Kind: KArrayOf, ArrayOf: "players", Weight: 3, Required: true},
			{Field: "coins", Names: []string{"coins", "balance"},
				Kind: KInt, Min: 0, Max: 100_000_000, NameRequired: true, Weight: 1.5},
			{Field: "squad", Names: []string{"squad", "activesquad", "squads", "formation"},
				Kind: KObject, NameRequired: true, Weight: 2},
			{Field: "platform", Names: []string{"platform", "console"},
				Kind: KString, NameRequired: true, Weight: 1},
			{Field: "gamer_tag", Names: []string{"gamertag", "eaid", "username", "name"},
				Kind: KString, NameRequired: true, Weight: 1},
		},
	},
}

// Classify roda todas as assinaturas e devolve a melhor, com o mapa de
// campos aprendido. Devolve nome vazio quando nada atinge o mínimo.
func Classify(samples []map[string]any) (name string, score float64, fields map[string]string) {
	var bestEvidence float64
	for _, sig := range Signatures {
		s, ev, f := sig.Match(samples)
		if s < sig.MinScore || ev < sig.MinEvidence {
			continue
		}
		// Desempate por evidência, não pela fração: a fração premia
		// assinatura curta, a evidência premia acerto específico.
		if ev > bestEvidence {
			name, score, fields, bestEvidence = sig.Name, s, f, ev
		}
	}
	return name, score, fields
}
