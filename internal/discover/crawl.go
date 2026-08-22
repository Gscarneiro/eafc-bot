// Package discover descobre sozinho como o fut.gg serve os dados.
//
// A ideia: em vez de adivinhar rotas, minerar as rotas de dentro do próprio
// JavaScript do site. Todo SPA precisa ter as URLs que ele chama escritas
// em algum lugar do bundle — se o site chama, o caminho está lá em texto.
// Depois cada candidata é sondada e classificada pelo FORMATO do JSON que
// devolve, nunca pelo nome da rota. Assim o bot funciona mesmo quando o
// fut.gg renomeia tudo, e continua funcionando na virada para o FC 27.
package discover

import (
	"context"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Fetcher é o mínimo que a descoberta precisa. O *futgg.Client satisfaz.
type Fetcher interface {
	GetRaw(ctx context.Context, url string) ([]byte, error)
}

// Candidate é uma rota encontrada no bundle, ainda não verificada.
type Candidate struct {
	URL      string   // URL absoluta, com {placeholders} normalizados
	Origins  []string // em quais arquivos JS foi encontrada
	Hits     int      // quantas vezes apareceu (sinal fraco de importância)
	NeedsArg bool     // tem placeholder e precisa de valor para ser sondada
	// Registry marca a rota que veio do REGISTRO de rotas do site, e não de
	// um literal solto no meio do código. É a diferença entre "o site
	// declara que isto é um endpoint" e "achei uma string com barra": vale
	// prioridade máxima na fila de sondagem. Ver registry.go.
	Registry bool
}

// MineStats conta o que a varredura deixou de fora, para o relatório poder
// explicar em vez de só encolher o número.
type MineStats struct {
	SkippedWrites int // rotas de escrita que não devem ser sondadas com GET
	Builders      int // montadores de URL achados no bundle
	Registry      int // rotas vindas do registro
}

// scriptSrc acha os <script src="..."> da página.
var scriptSrc = regexp.MustCompile(`<script[^>]*\ssrc=["']([^"']+\.js[^"']*)["']`)

// preloadSrc pega os módulos pré-carregados, que o Next.js declara em link.
var preloadSrc = regexp.MustCompile(`<link[^>]*\shref=["']([^"']+\.js[^"']*)["'][^>]*>`)

// pathLit acha literais de caminho dentro do JavaScript. Cobre aspas
// simples, duplas e crase (template literal), com ou sem host.
var pathLit = regexp.MustCompile(
	"[\"'`]((?:https?://[A-Za-z0-9.\\-]+)?/[A-Za-z0-9/_\\-.]" +
		"[A-Za-z0-9/_\\-.${}%:]*)[\"'`]")

// tmplVar normaliza a interpolação do JS para o placeholder do config:
// `/players/${e}/` vira `/players/{id}/`.
var tmplVar = regexp.MustCompile(`\$\{[^}]*\}`)

// domainWords são as palavras que sinalizam uma rota de dados que nos
// interessa. Uma rota sem nenhuma delas raramente vale a sondagem.
var domainWords = []string{
	"player", "jogador", "card", "item",
	"evolution", "evo",
	"sbc", "squad-building", "challenge", "set",
	"objective", "milestone", "season",
	"news", "article", "post",
	"club", "squad", "gg-club", "profile",
	"price", "market", "bin",
}

// noiseWords descartam ruído óbvio: analytics, imagens, fontes, i18n.
var noiseWords = []string{
	"google", "gtag", "analytics", "segment", "sentry", "doubleclick",
	"facebook", "hotjar", "cloudflare", "recaptcha", "adsystem",
	".png", ".jpg", ".jpeg", ".svg", ".webp", ".css", ".woff", ".ico",
	"/_next/static/", "/locales/", "/fonts/", "sourcemap", ".map",
}

// MineOptions controla o alcance da varredura.
type MineOptions struct {
	SeedPaths  []string // páginas a visitar para achar os bundles
	MaxScripts int      // teto de arquivos JS a baixar
	Verbose    func(format string, args ...any)
}

// DefaultMineOptions visita as páginas que certamente carregam cada tipo
// de dado — é nelas que o bundle correspondente é baixado pelo navegador.
func DefaultMineOptions() MineOptions {
	return MineOptions{
		SeedPaths: []string{
			"/", "/players/", "/evolutions/", "/sbc/", "/objectives/", "/news/", "/gg-club/",
		},
		MaxScripts: 60,
	}
}

// Mine baixa as páginas semente, segue os bundles e devolve as rotas de
// dados encontradas, ordenadas por relevância.
func Mine(ctx context.Context, f Fetcher, baseURL string, opt MineOptions) ([]Candidate, MineStats, error) {
	var stats MineStats
	if opt.MaxScripts <= 0 {
		opt.MaxScripts = 60
	}
	if opt.Verbose == nil {
		opt.Verbose = func(string, ...any) {}
	}
	base := strings.TrimRight(baseURL, "/")

	// 1. Páginas semente, em paralelo. As que faltarem não interrompem nada:
	// o site pode simplesmente não ter aquela seção neste ciclo.
	var mu sync.Mutex
	scripts := map[string]bool{}
	inlineHits := map[string]*Candidate{}

	var wg sync.WaitGroup
	for _, p := range opt.SeedPaths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			body, err := f.GetRaw(ctx, base+p)
			if err != nil {
				opt.Verbose("  semente %s: %v", p, err)
				return
			}
			html := string(body)

			mu.Lock()
			for _, m := range scriptSrc.FindAllStringSubmatch(html, -1) {
				scripts[absolute(base, m[1])] = true
			}
			for _, m := range preloadSrc.FindAllStringSubmatch(html, -1) {
				scripts[absolute(base, m[1])] = true
			}
			// O HTML em si já pode carregar as rotas, quando o Next.js
			// embute o payload do servidor na página.
			collect(inlineHits, html, base, "html:"+p)
			mu.Unlock()
			opt.Verbose("  semente %s: %d bytes", p, len(body))
		}(p)
	}
	wg.Wait()

	list := make([]string, 0, len(scripts))
	for s := range scripts {
		list = append(list, s)
	}
	// Ordem estável, e os chunks de página antes dos de framework:
	// os primeiros é que contêm as chamadas de dados.
	sort.Slice(list, func(i, j int) bool {
		fi, fj := frameworkish(list[i]), frameworkish(list[j])
		if fi != fj {
			return !fi
		}
		return list[i] < list[j]
	})
	if len(list) > opt.MaxScripts {
		list = list[:opt.MaxScripts]
	}
	opt.Verbose("  %d bundles JS encontrados, baixando %d", len(scripts), len(list))

	// 2. Baixa os bundles em paralelo e minera os literais.
	hits := inlineHits
	var mu2 sync.Mutex
	var wg2 sync.WaitGroup
	sem := make(chan struct{}, 8)

	for _, src := range list {
		wg2.Add(1)
		go func(src string) {
			defer wg2.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			body, err := f.GetRaw(ctx, src)
			if err != nil {
				return
			}
			text := string(body)
			// O registro de rotas vem PRIMEIRO: é a única fonte em que o
			// site declara, ele mesmo, o que é endpoint. Os literais soltos
			// entram depois, como complemento.
			regPaths, skipped := mineRegistry(text)

			mu2.Lock()
			collectRegistry(hits, regPaths, base, shortName(src))
			collect(hits, text, base, shortName(src))
			stats.SkippedWrites += skipped
			mu2.Unlock()
		}(src)
	}
	wg2.Wait()

	out := make([]Candidate, 0, len(hits))
	for _, c := range hits {
		if c.Registry {
			stats.Registry++
		}
		out = append(out, *c)
	}
	// Mais ocorrências e caminho mais curto primeiro: rotas de listagem
	// (as que queremos) são mais citadas e mais rasas que as de detalhe.
	sort.Slice(out, func(i, j int) bool {
		if pi, pj := rank(out[i]), rank(out[j]); pi != pj {
			return pi > pj
		}
		if out[i].Hits != out[j].Hits {
			return out[i].Hits > out[j].Hits
		}
		return len(out[i].URL) < len(out[j].URL)
	})
	if stats.Registry > 0 {
		opt.Verbose("  %d rotas vindas do registro do site (%d de escrita puladas)",
			stats.Registry, stats.SkippedWrites)
	}
	return out, stats, nil
}

// collectRegistry registra as rotas que o próprio site declarou como
// endpoint. Elas não passam pelo interesting(): quem decidiu que são rotas
// foi o registro do site, não uma heurística nossa — e a regra de "pelo
// menos dois segmentos" reprovaria "/api/fut/sbc/" na origem, que é
// justamente uma das que o bot precisa.
func collectRegistry(dst map[string]*Candidate, paths []string, base, origin string) {
	for _, p := range paths {
		abs := absolute(base, normalize(p))
		if abs == "" {
			continue
		}
		c, ok := dst[abs]
		if !ok {
			c = &Candidate{URL: abs, NeedsArg: strings.Contains(abs, "{")}
			dst[abs] = c
		}
		c.Registry = true
		c.Hits++
		if len(c.Origins) < 4 && !contains(c.Origins, origin) {
			c.Origins = append(c.Origins, origin)
		}
	}
}

// collect extrai os caminhos de um blob de texto (HTML ou JS).
func collect(dst map[string]*Candidate, text, base, origin string) {
	for _, m := range pathLit.FindAllStringSubmatch(text, -1) {
		raw := m[1]
		if !interesting(raw) {
			continue
		}
		norm := normalize(raw)
		abs := absolute(base, norm)
		if abs == "" {
			continue
		}

		c, ok := dst[abs]
		if !ok {
			c = &Candidate{URL: abs, NeedsArg: strings.Contains(abs, "{")}
			dst[abs] = c
		}
		c.Hits++
		if len(c.Origins) < 4 && !contains(c.Origins, origin) {
			c.Origins = append(c.Origins, origin)
		}
	}
}

// interesting descarta apenas ruído evidente. Nome de rota NÃO é critério
// de inclusão: quem decide o que a rota é, é o classificador, olhando o
// JSON. Filtrar por palavra-chave aqui derrubaria uma rota de evoluções
// chamada "/progression/upgrade-paths/" — que é exatamente o caso que o
// bot precisa aguentar.
func interesting(p string) bool {
	l := strings.ToLower(p)
	for _, n := range noiseWords {
		if strings.Contains(l, n) {
			return false
		}
	}
	if strings.HasSuffix(l, ".js") || strings.HasSuffix(l, ".json.br") {
		return false
	}
	// Precisa de pelo menos dois segmentos para ser rota de dados.
	trimmed := strings.Trim(strings.TrimPrefix(strings.TrimPrefix(l, "https://"), "http://"), "/")
	return strings.Count(trimmed, "/") >= 1
}

// rank ordena a fila de sondagem. Rota do registro vem antes de tudo: é a
// única categoria em que o site AFIRMA que aquilo é um endpoint, e o
// orçamento de sondagens é finito.
func rank(c Candidate) int {
	if c.Registry {
		return priority(c.URL) + 10
	}
	return priority(c.URL)
}

// priority ordena a fila de sondagem: rota com palavra de domínio ou sob
// /api/ vai na frente, porque o orçamento de sondagens é finito. Rota sem
// nenhum sinal ainda é sondada — só depois.
func priority(u string) int {
	l := strings.ToLower(u)
	score := 0
	if strings.Contains(l, "/api/") {
		score += 2
	}
	for _, w := range domainWords {
		if strings.Contains(l, w) {
			score += 3
			break
		}
	}
	return score
}

// normalize troca interpolação de template por {id} e limpa a query.
func normalize(p string) string {
	p = tmplVar.ReplaceAllString(p, "{id}")
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p = p[:i]
	}
	// Colapsa barras duplicadas que sobram da concatenação no JS.
	for strings.Contains(p, "//") && !strings.Contains(p, "://") {
		p = strings.ReplaceAll(p, "//", "/")
	}
	return p
}

// absolute resolve o caminho contra a base, preservando host próprio
// quando a rota aponta para outro domínio (uma api.* separada, por exemplo).
func absolute(base, p string) string {
	// A barra final NÃO é acrescentada: um bundle é ".../chunk.js" e nunca
	// ".../chunk.js/", e nem toda API usa barra no fim. Preservamos o
	// caminho exatamente como o JavaScript o escreveu, que é o que o
	// próprio site chama.
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return p
	}
	if !strings.HasPrefix(p, "/") {
		return ""
	}
	u, err := url.Parse(base + p)
	if err != nil {
		return ""
	}
	// url.Parse escapa as chaves; devolvemos {id} legível para o config e
	// para a detecção de placeholder funcionar.
	return strings.NewReplacer("%7B", "{", "%7D", "}").Replace(u.String())
}

// frameworkish marca bundles de framework, que raramente têm rotas de dados.
func frameworkish(src string) bool {
	l := strings.ToLower(src)
	for _, w := range []string{"framework", "webpack", "polyfill", "main-app", "react", "vendor"} {
		if strings.Contains(l, w) {
			return true
		}
	}
	return false
}

func shortName(src string) string {
	if i := strings.LastIndexByte(src, '/'); i >= 0 && i < len(src)-1 {
		return src[i+1:]
	}
	return src
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
