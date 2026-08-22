package futgg

import (
	"context"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// O robots.txt do fut.gg anuncia sitemaps. Isso é o site dizendo, em
// formato de máquina, "é por aqui que se enumera o conteúdo" — e as
// páginas de detalhe de jogador que eles listam são renderizadas no
// servidor, com overall, posição e os seis atributos no HTML.
//
// É a rota de coleta mais estável que existe aqui: não depende de
// adivinhar caminho de API, não muda quando o front é reescrito, e é
// exatamente o que eles abriram para crawlers.

type sitemapIndex struct {
	Sitemaps []struct {
		Loc string `xml:"loc"`
	} `xml:"sitemap"`
}

type urlSet struct {
	URLs []struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod"`
	} `xml:"url"`
}

// SitemapEntry é uma URL listada, com a data que o site declara.
type SitemapEntry struct {
	URL     string
	LastMod string
}

// Sitemaps lê o índice e devolve os sub-sitemaps cujo nome bate com o
// filtro (por exemplo "player-detail").
func (c *Client) Sitemaps(ctx context.Context, contains string) ([]string, error) {
	body, err := c.GetRaw(ctx, strings.TrimRight(c.cfg.BaseURL, "/")+"/sitemap.xml")
	if err != nil {
		return nil, fmt.Errorf("lendo sitemap.xml: %w", err)
	}
	var idx sitemapIndex
	if err := xml.Unmarshal(body, &idx); err != nil {
		return nil, fmt.Errorf("sitemap.xml não é um índice válido: %w", err)
	}
	var out []string
	for _, s := range idx.Sitemaps {
		if contains == "" || strings.Contains(s.Loc, contains) {
			out = append(out, s.Loc)
		}
	}
	sort.Strings(out)
	return out, nil
}

// SitemapURLs lê um sub-sitemap.
func (c *Client) SitemapURLs(ctx context.Context, sitemapURL string) ([]SitemapEntry, error) {
	body, err := c.GetRaw(ctx, sitemapURL)
	if err != nil {
		return nil, err
	}
	var set urlSet
	if err := xml.Unmarshal(body, &set); err != nil {
		return nil, err
	}
	out := make([]SitemapEntry, 0, len(set.URLs))
	for _, u := range set.URLs {
		out = append(out, SitemapEntry{URL: u.Loc, LastMod: u.LastMod})
	}
	return out, nil
}

// PageCollectOptions limita o esforço da coleta por páginas. Ela é
// inerentemente mais cara que uma API — uma requisição por carta — então
// o teto e a ordenação importam.
type PageCollectOptions struct {
	// MaxPlayers é quantas páginas de jogador buscar no total.
	MaxPlayers int
	// SitemapsToRead é quantos sub-sitemaps percorrer (cada um traz
	// milhares de URLs). Os mais recentes primeiro.
	SitemapsToRead int
	// Newest usa o lastmod do sitemap para priorizar o que mudou —
	// carta nova e carta com preço mexendo aparecem primeiro.
	Newest  bool
	Verbose func(format string, args ...any)
}

func DefaultPageCollectOptions() PageCollectOptions {
	return PageCollectOptions{MaxPlayers: 400, SitemapsToRead: 3, Newest: true}
}

// PlayersFromPages coleta cartas lendo as páginas de detalhe enumeradas
// pelos sitemaps, sem tocar em nenhuma rota de API.
func (c *Client) PlayersFromPages(ctx context.Context, opt PageCollectOptions) ([]domain.Player, error) {
	if opt.MaxPlayers <= 0 {
		opt.MaxPlayers = 400
	}
	if opt.SitemapsToRead <= 0 {
		opt.SitemapsToRead = 3
	}
	if opt.Verbose == nil {
		opt.Verbose = func(string, ...any) {}
	}

	maps, err := c.Sitemaps(ctx, "player-detail")
	if err != nil {
		return nil, err
	}
	if len(maps) == 0 {
		return nil, fmt.Errorf("nenhum sitemap de jogador encontrado no índice")
	}
	opt.Verbose("  %d sitemaps de jogador; lendo %d", len(maps), min2(len(maps), opt.SitemapsToRead))

	var entries []SitemapEntry
	for i, sm := range maps {
		if i >= opt.SitemapsToRead {
			break
		}
		urls, err := c.SitemapURLs(ctx, sm)
		if err != nil {
			opt.Verbose("  %s: %v", sm, err)
			continue
		}
		entries = append(entries, urls...)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("sitemaps de jogador vieram vazios")
	}

	if opt.Newest {
		sort.Slice(entries, func(i, j int) bool { return entries[i].LastMod > entries[j].LastMod })
	}
	if len(entries) > opt.MaxPlayers {
		entries = entries[:opt.MaxPlayers]
	}
	opt.Verbose("  buscando %d páginas de jogador", len(entries))

	var (
		mu      sync.Mutex
		players []domain.Player
		fails   int
	)
	sem := make(chan struct{}, c.cfg.Concurrency)
	var wg sync.WaitGroup

	for _, e := range entries {
		wg.Add(1)
		go func(e SitemapEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			p, ok := c.playerFromPage(ctx, e.URL)
			mu.Lock()
			defer mu.Unlock()
			if !ok {
				fails++
				return
			}
			players = append(players, p)
		}(e)
	}
	wg.Wait()

	if len(players) == 0 {
		return nil, fmt.Errorf("nenhuma das %d páginas rendeu uma carta legível "+
			"(o formato da página pode ter mudado)", len(entries))
	}
	opt.Verbose("  %d cartas lidas, %d páginas falharam", len(players), fails)
	return players, nil
}

// playerFromPage lê uma página de detalhe. Tenta primeiro o payload que o
// servidor embutiu — que é dado estruturado — e só depois cai para o que
// dá para tirar da URL e do texto renderizado.
func (c *Client) playerFromPage(ctx context.Context, pageURL string) (domain.Player, bool) {
	body, err := c.GetRaw(ctx, pageURL)
	if err != nil {
		return domain.Player{}, false
	}

	l := c.lensFor("players")
	best := domain.Player{}
	bestScore := 0

	for _, payload := range ExtractEmbedded(body) {
		for _, obj := range payload.Objects {
			p := mapPlayer(node(obj), c.cfg.Cycle, l)
			// Uma carta só conta se tem o mínimo que a análise precisa.
			if p.Rating < 40 || p.Rating > 99 || p.Position == "" {
				continue
			}
			if s := playerCompleteness(p); s > bestScore {
				best, bestScore = p, s
			}
		}
	}
	if bestScore == 0 {
		return domain.Player{}, false
	}

	// A URL carrega o id do recurso e o nome, e é a fonte mais confiável
	// dos dois: /players/192985-kevin-de-bruyne/26-117633497/
	if id, name, ok := parsePlayerURL(pageURL); ok {
		if best.ID == 0 {
			best.ID = id
		}
		if best.Name == "" {
			best.Name = name
		}
	}
	best.FutGGSlug = pageURL
	if best.ID == 0 {
		return domain.Player{}, false
	}
	return best, true
}

// playerCompleteness pontua o quanto o objeto encontrado parece uma carta
// completa — a mesma página traz vários objetos parecidos (o jogador, o
// "jogador similar", o histórico), e queremos o mais rico.
func playerCompleteness(p domain.Player) int {
	s := 1
	a := p.Attributes
	for _, v := range []int{a.Pace, a.Shooting, a.Passing, a.Dribbling, a.Defending, a.Physical} {
		if v > 0 {
			s++
		}
	}
	if p.Name != "" {
		s += 2
	}
	if p.ID != 0 {
		s += 2
	}
	if len(p.PlayStyles) > 0 {
		s += 2
	}
	if p.Club != "" {
		s++
	}
	if p.Price.Coins > 0 {
		s += 3
	}
	return s
}

// parsePlayerURL tira o id do recurso e o nome do caminho.
// /players/192985-kevin-de-bruyne/26-117633497/ -> 117633497, "Kevin De Bruyne"
func parsePlayerURL(u string) (int64, string, bool) {
	i := strings.Index(u, "/players/")
	if i < 0 {
		return 0, "", false
	}
	parts := strings.Split(strings.Trim(u[i+len("/players/"):], "/"), "/")
	if len(parts) < 2 {
		return 0, "", false
	}

	// Segundo segmento: "26-117633497" (ciclo-idDoRecurso).
	var id int64
	if _, after, ok := strings.Cut(parts[1], "-"); ok {
		if n, err := strconv.ParseInt(after, 10, 64); err == nil {
			id = n
		}
	}

	// Primeiro segmento: "192985-kevin-de-bruyne".
	name := parts[0]
	if _, after, ok := strings.Cut(name, "-"); ok {
		name = after
	}
	words := strings.Split(name, "-")
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return id, strings.Join(words, " "), id != 0
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}
