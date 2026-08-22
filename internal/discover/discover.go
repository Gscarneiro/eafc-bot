package discover

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// Finding é uma rota já verificada: sabemos o que ela devolve e como ler.
type Finding struct {
	Kind       string            `json:"kind"` // "players", "evolutions", ...
	URL        string            `json:"url"`
	Path       string            `json:"path"`    // caminho relativo, para o config
	Score      float64           `json:"score"`   // 0 a 1
	Wrapper    string            `json:"wrapper"` // chave que embrulha a lista ("data", "results"...)
	Samples    int               `json:"samples"`
	Fields     map[string]string `json:"fields"`      // campo lógico -> chave real
	Source     string            `json:"source"`      // "bundle" ou "página"
	SampleID   string            `json:"sample_id"`   // um id real, para sondar rotas de detalhe
	SamplePeek string            `json:"sample_peek"` // primeiro objeto, resumido
}

// Result é tudo que a descoberta apurou.
type Result struct {
	BaseURL   string             `json:"base_url"`
	Best      map[string]Finding `json:"best"` // melhor rota por tipo
	All       []Finding          `json:"all"`
	Probed    int                `json:"probed"`
	Mined     int                `json:"mined"`
	Unmatched []string           `json:"unmatched"` // rotas que responderam JSON mas não bateram
	// Blocked são rotas que o robots.txt do site pede para não bater.
	// Ficam registradas para o relatório poder explicar o que faltou em
	// vez de dizer só "não encontrado".
	Blocked []string `json:"blocked"`
	// BlockedCount é o total de verdade. Blocked guarda só uma amostra, e
	// imprimir o tamanho dela como se fosse o total já fez o relatório
	// dizer "40 rotas puladas" quando eram centenas.
	BlockedCount int      `json:"blocked_count"`
	Sitemaps     []string `json:"sitemaps"`
	// Registry é quantas candidatas vieram do registro de rotas do site.
	Registry int `json:"registry"`
	// Outcomes conta o que cada sondagem devolveu, para "nada respondeu"
	// virar um diagnóstico em vez de um mistério.
	Outcomes map[string]int `json:"outcomes"`
}

// Options controla a execução.
type Options struct {
	Mine        MineOptions
	MaxProbes   int
	Concurrency int
	// ArgValues preenche placeholders para sondar rotas parametrizadas.
	// A do clube é a que importa: "/roster/sync/{id}/" só responde com um
	// gamertag de verdade, e o clube é o dado central do bot.
	ArgValues map[string]string
	// RespectRobots decide se as rotas bloqueadas pelo robots.txt são
	// puladas. Padrão true; quem desliga assume a escolha.
	RespectRobots bool
	Verbose       func(format string, args ...any)
}

func DefaultOptions() Options {
	return Options{
		Mine:          DefaultMineOptions(),
		MaxProbes:     200,
		Concurrency:   6,
		RespectRobots: true,
	}
}

// Run executa a descoberta inteira: minera as rotas do JavaScript, sonda
// cada uma, e classifica pelo formato do JSON. Se nada aparecer via API,
// cai para o payload embutido nas páginas.
func Run(ctx context.Context, f Fetcher, baseURL string, opt Options) (*Result, error) {
	if opt.MaxProbes <= 0 {
		opt.MaxProbes = 200
	}
	if opt.Concurrency <= 0 {
		opt.Concurrency = 6
	}
	if opt.Verbose == nil {
		opt.Verbose = func(string, ...any) {}
	}
	opt.Mine.Verbose = opt.Verbose

	res := &Result{
		BaseURL:  strings.TrimRight(baseURL, "/"),
		Best:     map[string]Finding{},
		Outcomes: map[string]int{},
	}

	// O robots.txt vem primeiro, e não é formalidade: no fut.gg ele
	// bloqueia justamente /api/* e /gg-club/. Sondar assim mesmo seria
	// ignorar um pedido explícito do site.
	robots := FetchRobots(ctx, f, baseURL)
	if !opt.RespectRobots {
		opt.Verbose("robots.txt IGNORADO por configuração — %d regras de bloqueio existem e não serão respeitadas",
			len(robots.disallow))
		res.Sitemaps = robots.Sitemaps
		robots = &Robots{} // não carregado: Allowed() libera tudo
	} else if robots.Loaded {
		opt.Verbose("robots.txt lido: %d regras de bloqueio, %d sitemaps",
			len(robots.disallow), len(robots.Sitemaps))
		res.Sitemaps = robots.Sitemaps
	} else {
		opt.Verbose("robots.txt não pôde ser lido — seguindo sem restrições declaradas")
	}

	opt.Verbose("minerando rotas no JavaScript do site...")
	cands, mineStats, err := Mine(ctx, f, baseURL, opt.Mine)
	if err != nil {
		return nil, err
	}
	res.Mined = len(cands)
	res.Registry = mineStats.Registry
	opt.Verbose("  %d rotas candidatas", len(cands))
	if mineStats.SkippedWrites > 0 {
		res.Outcomes["pulada: rota de escrita"] += mineStats.SkippedWrites
	}

	// Tira de circulação o que o site pediu para não bater, ANTES de
	// qualquer requisição.
	cands, blocked, blockedCount := robots.FilterCandidates(cands)
	res.Blocked = blocked
	res.BlockedCount = blockedCount
	if blockedCount > 0 {
		opt.Verbose("  %d rotas puladas por robots.txt", blockedCount)
	}

	// Rotas de listagem primeiro: são as que o bot realmente consome, e
	// as de detalhe precisam de um id que ainda não temos.
	var probeList []Candidate
	for _, c := range cands {
		if !c.NeedsArg {
			probeList = append(probeList, c)
		}
	}
	probeList = withinBudget(probeList, opt.MaxProbes)

	opt.Verbose("sondando %d rotas...", len(probeList))
	findings, unmatched, outcomes := probeAll(ctx, f, probeList, opt)
	res.All = findings
	res.Unmatched = unmatched
	res.Probed = len(probeList)
	for k, v := range outcomes {
		res.Outcomes[k] += v
	}

	// Segunda passada: rotas parametrizadas. Só agora dá para sondá-las,
	// porque o id de exemplo sai da listagem de jogadores descoberta acima.
	if args := argValues(opt, findings); len(args) > 0 {
		var parametrized []Candidate
		usedValue := map[string]variant{}
		for _, c := range cands {
			if !c.NeedsArg {
				continue
			}
			// O placeholder pode se chamar {gt}, {slug} ou {e} — não dá para
			// saber o que ele espera pelo nome. Sondamos uma variante por
			// valor disponível e deixamos o classificador decidir.
			for _, v := range fillVariants(c.URL, args) {
				parametrized = append(parametrized, Candidate{URL: v.url, Hits: c.Hits})
				usedValue[v.url] = v
			}
		}
		if len(parametrized) > 0 {
			if len(parametrized) > 30 {
				parametrized = parametrized[:30]
			}
			opt.Verbose("sondando %d rotas com parâmetro...", len(parametrized))
			more, unm, outc := probeAll(ctx, f, parametrized, opt)
			for k, v := range outc {
				res.Outcomes[k] += v
			}
			// A rota volta ao config com o placeholder, não com o valor de
			// exemplo — senão o bot buscaria sempre o mesmo jogador.
			for i := range more {
				v := usedValue[more[i].URL]
				more[i].Path = restoreOne(more[i].Path, v)
				more[i].URL = restoreOne(more[i].URL, v)
			}
			findings = append(findings, more...)
			res.All = findings
			res.Probed += len(parametrized)
			res.Unmatched = append(res.Unmatched, unm...)
		}
	}

	// Fallback: mesmo com alguma API achada, os tipos que faltaram podem
	// estar embutidos no HTML. Vale a busca enquanto não temos os três
	// principais (jogadores, evoluções, SBCs).
	if distinctKinds(findings) < 3 {
		if len(findings) == 0 {
			opt.Verbose("nenhuma API respondeu; procurando dados embutidos nas páginas...")
		} else {
			opt.Verbose("faltam tipos; procurando o resto embutido nas páginas...")
		}
		embedded := probeEmbedded(ctx, f, baseURL, opt)
		res.All = append(res.All, embedded...)
	}

	// Melhor rota por tipo.
	for _, fd := range res.All {
		cur, ok := res.Best[fd.Kind]
		if !ok || better(fd, cur) {
			res.Best[fd.Kind] = fd
		}
	}
	return res, nil
}

// better prefere nota alta e, em empate, mais amostras e caminho mais curto:
// uma listagem completa é melhor que uma prévia da home.
// argValues junta o que temos para preencher placeholder: o gamertag vindo
// da configuração e um id colhido da própria listagem de jogadores.
func argValues(opt Options, findings []Finding) map[string]string {
	args := map[string]string{}
	for k, v := range opt.ArgValues {
		if v != "" {
			args[k] = v
		}
	}
	for _, f := range findings {
		if f.Kind == "players" && f.SampleID != "" {
			args["id"] = f.SampleID
			break
		}
	}
	return args
}

type variant struct{ url, value, key string }

// fillVariants gera uma URL sondável por valor conhecido. Rota com mais de
// um placeholder é descartada: adivinhar dois de uma vez só gera 404.
func fillVariants(u string, args map[string]string) []variant {
	if strings.Count(u, "{") > 1 {
		return nil
	}
	var out []variant
	seen := map[string]bool{}
	for _, key := range []string{"gamertag", "id", "slug"} {
		v, ok := args[key]
		if !ok || v == "" {
			continue
		}
		filled := genericPlaceholder.ReplaceAllString(u, v)
		if filled == u || seen[filled] {
			continue
		}
		seen[filled] = true
		out = append(out, variant{url: filled, value: v, key: key})
	}
	return out
}

var genericPlaceholder = regexp.MustCompile(`\{[^}]*\}`)

// restoreOne devolve o placeholder no lugar do valor de exemplo, para o
// config guardar a rota parametrizada e não uma busca fixa. O nome do
// placeholder é o do ARGUMENTO que serviu, não um {id} genérico: a rota do
// clube precisa sair como {gamertag} para o bot preenchê-la depois.
func restoreOne(u string, v variant) string {
	if v.value == "" {
		return u
	}
	return strings.ReplaceAll(u, v.value, "{"+v.key+"}")
}

// withinBudget corta a fila de sondagem sem sacrificar o que o site
// declarou. As rotas do registro passam INTEIRAS, mesmo que estourem o
// teto: cortar justamente a única fonte confiável para caber num limite
// pensado para literais soltos é o que fazia a descoberta voltar vazia. O
// teto continua valendo para o resto.
func withinBudget(cands []Candidate, max int) []Candidate {
	if len(cands) <= max {
		return cands
	}
	var registry, rest []Candidate
	for _, c := range cands {
		if c.Registry {
			registry = append(registry, c)
		} else {
			rest = append(rest, c)
		}
	}
	room := max - len(registry)
	if room < 0 {
		room = 0
	}
	if room > len(rest) {
		room = len(rest)
	}
	return append(registry, rest[:room]...)
}

func distinctKinds(fs []Finding) int {
	seen := map[string]bool{}
	for _, f := range fs {
		seen[f.Kind] = true
	}
	return len(seen)
}

// kindHints são palavras que uma rota daquele tipo costuma ter no caminho.
//
// NÃO servem para classificar — quem classifica é o formato do JSON, e é
// isso que faz o bot sobreviver a uma renomeação. Servem de DESEMPATE, para
// o caso em que duas rotas têm o mesmo formato e não há o que olhar no
// conteúdo: /api/fut/pools/ e /api/fut/news/ devolvem, os dois, objetos com
// título, data, slug e resumo. Estruturalmente são notícias iguais. Só o
// caminho separa, e ignorá-lo aqui é jogar fora a única informação que resta.
var kindHints = map[string][]string{
	"players":    {"player", "card", "item"},
	"evolutions": {"evolution", "evo"},
	"sbcs":       {"sbc", "squad-building", "challenge"},
	"objectives": {"objective", "milestone"},
	"news":       {"news", "article", "post"},
	"club":       {"club", "roster", "squad"},
}

func hinted(f Finding) bool {
	l := strings.ToLower(f.Path)
	for _, w := range kindHints[f.Kind] {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

func better(a, b Finding) bool {
	// O desempate pelo caminho vem antes da nota, mas só dentro de uma
	// faixa: uma rota com o nome certo e formato sofrível não pode ganhar
	// de uma com o formato claramente melhor.
	if ha, hb := hinted(a), hinted(b); ha != hb {
		if diff := a.Score - b.Score; diff > -0.3 && diff < 0.3 {
			return ha
		}
	}
	if diff := a.Score - b.Score; diff > 0.05 || diff < -0.05 {
		return a.Score > b.Score
	}
	if a.Samples != b.Samples {
		return a.Samples > b.Samples
	}
	return len(a.Path) < len(b.Path)
}

func probeAll(ctx context.Context, f Fetcher, cands []Candidate, opt Options) ([]Finding, []string, map[string]int) {
	var mu sync.Mutex
	var findings []Finding
	var unmatched []string
	outcomes := map[string]int{}

	sem := make(chan struct{}, opt.Concurrency)
	var wg sync.WaitGroup

	for _, c := range cands {
		wg.Add(1)
		go func(c Candidate) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			body, err := f.GetRaw(ctx, c.URL)
			if err != nil {
				// Sem isto, um site que devolve 403 em tudo produz
				// "nenhuma rota respondeu" e nenhuma pista do motivo.
				mu.Lock()
				outcomes[classifyErr(err)]++
				mu.Unlock()
				return
			}
			// Uma resposta admite mais de uma leitura, e escolher errado
			// custa um endpoint inteiro: o JSON do clube tem uma lista de
			// jogadores dentro, então ler só a lista o classificaria como
			// "players" e o clube nunca seria encontrado. Classificamos
			// todas as leituras plausíveis e ficamos com a melhor.
			byKind := classifyAll(body)

			mu.Lock()
			defer mu.Unlock()
			if len(byKind) == 0 {
				if len(readings(body)) == 0 {
					outcomes[shapeOf(body)]++
					return
				}
				outcomes["JSON não reconhecido"]++
				if len(unmatched) < 25 {
					unmatched = append(unmatched, c.URL)
				}
				return
			}
			outcomes["identificado"]++
			for _, kind := range sortedClassified(byKind) {
				cl := byKind[kind]
				findings = append(findings, Finding{
					Kind: cl.kind, URL: c.URL, Path: relPath(opt, c.URL),
					Score: cl.score, Wrapper: cl.wrapper, Samples: cl.samples,
					Fields: cl.fields, Source: "bundle",
					SampleID:   sampleID(peekSamples(body, cl.wrapper), cl.fields),
					SamplePeek: peek(firstSample(body, cl.wrapper)),
				})
				opt.Verbose("  ✓ %-12s %.0f%%  %s", cl.kind, cl.score*100, relPath(opt, c.URL))
			}
		}(c)
	}
	wg.Wait()

	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Kind != findings[j].Kind {
			return findings[i].Kind < findings[j].Kind
		}
		return findings[i].Score > findings[j].Score
	})
	return findings, unmatched, outcomes
}

// classifyErr resume o erro de rede numa categoria legível no relatório.
func classifyErr(err error) string {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "404"):
		return "404 não existe"
	case strings.Contains(msg, "403") && needsSession(msg):
		return "403 exige sessão (preencha session_cookie)"
	case strings.Contains(msg, "403"):
		return "403 bloqueado pelo servidor"
	case strings.Contains(msg, "401"):
		return "401 exige autenticação"
	case strings.Contains(msg, "429"):
		return "429 excesso de requisições"
	case strings.Contains(msg, "50"):
		return "erro 5xx no servidor"
	case strings.Contains(msg, "timeout"), strings.Contains(msg, "deadline"):
		return "tempo esgotado"
	}
	return "erro de rede"
}

// needsSession separa o 403 que é falta de login do 403 que é bloqueio. O
// /api/gg-club/ do fut.gg responde {"error":"You must be logged in to access
// your club overview."} — esse tem conserto (preencher session_cookie), e
// misturá-lo com o bloqueio do Cloudflare esconde o conserto.
func needsSession(msg string) bool {
	l := strings.ToLower(msg)
	for _, s := range []string{
		"logged in", "log in", "login", "signed in", "sign in",
		"unauthorized", "not authenticated", "authentication",
		"sessão", "sessao",
	} {
		if strings.Contains(l, s) {
			return true
		}
	}
	return false
}

// shapeOf diz por que a resposta não virou amostra.
func shapeOf(body []byte) string {
	t := strings.TrimSpace(string(body))
	switch {
	case t == "":
		return "resposta vazia"
	case strings.HasPrefix(t, "<"):
		return "devolveu HTML, não JSON"
	case strings.HasPrefix(t, "{"), strings.HasPrefix(t, "["):
		return "JSON sem lista de objetos"
	}
	return "conteúdo não-JSON"
}

func relPath(opt Options, u string) string {
	// A base real está no Result, mas para log basta cortar o esquema+host.
	if i := strings.Index(u, "://"); i >= 0 {
		if j := strings.IndexByte(u[i+3:], '/'); j >= 0 {
			return u[i+3+j:]
		}
	}
	return u
}

// classified é uma leitura da resposta que bateu com alguma assinatura.
type classified struct {
	kind    string
	score   float64
	fields  map[string]string
	wrapper string
	samples int
}

// classifyAll devolve a melhor leitura POR TIPO.
//
// Uma mesma resposta pode servir a dois tipos de verdade, e ficar só com a
// melhor leitura geral fazia perder um deles. O /evolutions/…/v3/all/ do
// fut.gg é o caso: por fora é uma lista de cartas (classificava como
// "players", com 90% de confiança) e um nível abaixo, em "evolution", é a
// lista de evoluções. Guardando as duas, o endpoint de jogadores continua
// ganhando na rota dele — que pontua mais — e o de evoluções deixa de ser
// invisível.
func classifyAll(body []byte) map[string]classified {
	out := map[string]classified{}
	for _, r := range readings(body) {
		k, sc, f := Classify(r.samples)
		if k == "" {
			continue
		}
		cur, ok := out[k]
		// Empate vai para a leitura com mais amostras: uma listagem de 25
		// cartas é mais confiável que um objeto solto que por acaso casou.
		if !ok || sc > cur.score || (sc == cur.score && len(r.samples) > cur.samples) {
			out[k] = classified{k, sc, f, r.wrapper, len(r.samples)}
		}
	}
	return out
}

// classifyBody devolve a leitura de maior nota, sem olhar o tipo.
func classifyBody(body []byte) (kind string, score float64, fields map[string]string, wrapper string, samples int) {
	all := classifyAll(body)
	for _, k := range sortedClassified(all) {
		c := all[k]
		if c.score > score || (c.score == score && c.samples > samples) {
			kind, score, fields, wrapper, samples = c.kind, c.score, c.fields, c.wrapper, c.samples
		}
	}
	return kind, score, fields, wrapper, samples
}

// sortedClassified dá ordem estável ao mapa, para empate não virar sorteio.
func sortedClassified(m map[string]classified) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type reading struct {
	samples []map[string]any
	wrapper string
}

// readings enumera as formas de ler a resposta: o objeto de topo inteiro,
// cada chave que contenha uma lista de objetos, e — para cada uma dessas
// listas — o SUB-OBJETO comum aos itens dela.
//
// A última leitura é a que faltava. O fut.gg serve as evoluções como
// {"data":[{"evolution":{…}, …campos da carta…}]}: lida de fora, a lista é
// de jogadores; a evolução só existe descendo um nível. Sem descer, o
// endpoint de evoluções era impossível de achar — ele se classificava como
// jogadores e sumia.
func readings(body []byte) []reading {
	base := baseReadings(body)
	out := make([]reading, 0, len(base)*3)
	out = append(out, base...)
	for _, r := range base {
		out = append(out, subReadings(r)...)
	}
	return out
}

// subReadings desce um nível: para cada chave cujo valor é um objeto na
// maioria dos itens, devolve a lista desses objetos como leitura própria.
// "Na maioria" importa — um objeto que aparece em dois itens de trinta é
// exceção, não a estrutura do recurso.
func subReadings(r reading) []reading {
	if len(r.samples) == 0 {
		return nil
	}
	keys := make([]string, 0, len(r.samples[0]))
	for k, v := range r.samples[0] {
		if m, ok := v.(map[string]any); ok && len(m) > 0 {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	if len(keys) > 8 {
		keys = keys[:8]
	}

	var out []reading
	for _, k := range keys {
		subs := make([]map[string]any, 0, len(r.samples))
		for _, s := range r.samples {
			if m, ok := s[k].(map[string]any); ok && len(m) > 0 {
				subs = append(subs, m)
			}
		}
		if len(subs)*2 < len(r.samples) {
			continue
		}
		out = append(out, reading{samples: subs, wrapper: r.wrapper + "[]." + k})
	}
	return out
}

func baseReadings(body []byte) []reading {
	var out []reading

	var asArray []map[string]any
	if err := json.Unmarshal(body, &asArray); err == nil && len(asArray) > 0 {
		return []reading{{samples: cap25(asArray)}}
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil
	}

	// O objeto de topo como amostra única — é assim que um clube se parece.
	var single map[string]any
	if err := json.Unmarshal(body, &single); err == nil && len(single) > 0 {
		out = append(out, reading{samples: []map[string]any{single}})
	}

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var list []map[string]any
		if err := json.Unmarshal(obj[k], &list); err == nil && len(list) > 0 {
			out = append(out, reading{samples: cap25(list), wrapper: k})
		}
	}
	return out
}

// peekSamples e firstSample recuperam as amostras da leitura escolhida,
// para o relatório mostrar em que a classificação se baseou.
func peekSamples(body []byte, wrapper string) []map[string]any {
	for _, r := range readings(body) {
		if r.wrapper == wrapper {
			return r.samples
		}
	}
	return nil
}

func firstSample(body []byte, wrapper string) map[string]any {
	if s := peekSamples(body, wrapper); len(s) > 0 {
		return s[0]
	}
	return map[string]any{}
}

// extractSamples tira da resposta uma lista de objetos para classificar,
// e diz qual chave embrulhava a lista — o parser precisa saber disso.
func extractSamples(body []byte) (samples []map[string]any, wrapper string, ok bool) {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return nil, "", false // não é JSON, provavelmente HTML
	}

	var asArray []map[string]any
	if err := json.Unmarshal(body, &asArray); err == nil && len(asArray) > 0 {
		return cap25(asArray), "", true
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, "", false
	}
	// Procura a chave que guarda a lista. Ordem: as convenções conhecidas
	// primeiro, depois qualquer chave cujo valor seja lista de objetos.
	known := []string{"data", "results", "items", "players", "evolutions",
		"objectives", "sbcs", "sets", "news", "records", "content"}
	for _, k := range known {
		if raw, has := obj[k]; has {
			var list []map[string]any
			if err := json.Unmarshal(raw, &list); err == nil && len(list) > 0 {
				return cap25(list), k, true
			}
		}
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		var list []map[string]any
		if err := json.Unmarshal(obj[k], &list); err == nil && len(list) > 0 {
			return cap25(list), k, true
		}
	}

	// Objeto único (o clube, por exemplo) é amostra de um.
	var single map[string]any
	if err := json.Unmarshal(body, &single); err == nil && len(single) > 0 {
		return []map[string]any{single}, "", true
	}
	return nil, "", false
}

func cap25(list []map[string]any) []map[string]any {
	if len(list) > 25 {
		return list[:25]
	}
	return list
}

// nextData acha o payload que o Next.js embute na página.
var nextData = regexp.MustCompile(
	`(?s)<script[^>]*id="__NEXT_DATA__"[^>]*>(.*?)</script>`)

// probeEmbedded é o plano B: quando o site não expõe API, os dados ainda
// estão no HTML renderizado pelo servidor. Achamos o payload e varremos
// atrás de qualquer lista de objetos que case com uma assinatura.
func probeEmbedded(ctx context.Context, f Fetcher, baseURL string, opt Options) []Finding {
	base := strings.TrimRight(baseURL, "/")
	var out []Finding

	for _, p := range opt.Mine.SeedPaths {
		body, err := f.GetRaw(ctx, base+p)
		if err != nil {
			continue
		}
		m := nextData.FindSubmatch(body)
		if m == nil {
			continue
		}
		var payload any
		if err := json.Unmarshal(m[1], &payload); err != nil {
			continue
		}

		for path, list := range findObjectArrays(payload, "", 0) {
			kind, score, fields := Classify(cap25(list))
			if kind == "" {
				continue
			}
			out = append(out, Finding{
				Kind: kind, URL: base + p, Path: p,
				Score: score, Wrapper: "__NEXT_DATA__:" + path,
				Samples: len(list), Fields: fields,
				Source: "página", SamplePeek: peek(list[0]),
			})
			opt.Verbose("  ✓ %-12s %.0f%%  %s (embutido em %s)", kind, score*100, p, path)
		}
	}
	return out
}

// findObjectArrays desce no payload atrás de listas de objetos, guardando
// o caminho em notação de ponto para o parser saber onde procurar depois.
func findObjectArrays(v any, path string, depth int) map[string][]map[string]any {
	out := map[string][]map[string]any{}
	if depth > 8 {
		return out
	}
	switch t := v.(type) {
	case map[string]any:
		for k, sub := range t {
			p := k
			if path != "" {
				p = path + "." + k
			}
			for kk, vv := range findObjectArrays(sub, p, depth+1) {
				out[kk] = vv
			}
		}
	case []any:
		var objs []map[string]any
		for _, item := range t {
			if m, ok := item.(map[string]any); ok {
				objs = append(objs, m)
			}
		}
		// Lista de objetos com pelo menos 2 itens é candidata a coleção.
		if len(objs) >= 2 && path != "" {
			out[path] = objs
		}
		// Continua descendo: pode haver coleção mais adentro.
		for i, item := range t {
			if i > 3 {
				break // basta olhar os primeiros
			}
			for kk, vv := range findObjectArrays(item, path, depth+1) {
				if _, exists := out[kk]; !exists {
					out[kk] = vv
				}
			}
		}
	}
	return out
}

// sampleID pega um id de verdade da listagem, para poder sondar as rotas
// de detalhe na segunda passada.
func sampleID(samples []map[string]any, fields map[string]string) string {
	key := fields["id"]
	if key == "" {
		return ""
	}
	for _, s := range samples {
		if v, ok := flatten(s, 2)[key]; ok && v != nil {
			if n, ok := v.(float64); ok && n > 0 {
				return strconv.FormatInt(int64(n), 10)
			}
			if str, ok := v.(string); ok && str != "" {
				return str
			}
		}
	}
	return ""
}

// peek resume o primeiro objeto para o relatório mostrar no que a
// classificação se baseou.
func peek(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) > 12 {
		keys = keys[:12]
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := fmt.Sprint(m[k])
		if len(v) > 24 {
			v = v[:24] + "…"
		}
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, " ")
}
