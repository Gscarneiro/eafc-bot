// Package futgg fala com o fut.gg. Só biblioteca padrão: o bot compila
// para um binário único sem nenhuma dependência externa.
package futgg

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Um User-Agent honesto: diz o que o cliente é e como falar com o dono.
// Não imita navegador de propósito — se o site quiser barrar, que barre
// sabendo o que está barrando.
const defaultUA = "eafc-bot/0.2 (+https://github.com/gscarneiro/eafc-bot; uso pessoal, 1x/dia)"

// Config guarda tudo que muda entre ciclos do jogo (FC 26 -> FC 27) ou
// quando o fut.gg mexe nas rotas. A ideia é virar a chave editando JSON,
// sem recompilar.
type Config struct {
	BaseURL        string            `json:"base_url"`
	Cycle          string            `json:"cycle"`     // "26", "27"
	Endpoints      map[string]string `json:"endpoints"` // nome lógico -> caminho com {placeholders}
	UserAgent      string            `json:"user_agent"`
	RequestsPerSec float64           `json:"requests_per_sec"`
	Concurrency    int               `json:"concurrency"`
	CacheDir       string            `json:"cache_dir"`
	CacheTTL       Duration          `json:"cache_ttl"`
	SessionCookie  string            `json:"session_cookie"` // opcional: para ler seu GG Club privado

	// RespectRobots controla se a descoberta obedece ao robots.txt do site.
	// O padrão é obedecer. Desligar é uma escolha sua, sobre a sua máquina
	// e o seu uso — o bot só deixa de fingir que a regra não existe.
	RespectRobots *bool `json:"respect_robots,omitempty"`

	// FieldMaps e Wrappers são preenchidos por `eafcbot autoconfig`.
	// Guardam o que a descoberta aprendeu sobre o formato atual do fut.gg:
	// qual chave real carrega cada campo lógico, e qual chave embrulha a
	// lista. Nome aprendido tem prioridade sobre os aliases do código, então
	// uma renomeação no site se conserta rodando o autoconfig de novo.
	FieldMaps map[string]map[string]string `json:"field_maps,omitempty"`
	Wrappers  map[string]string            `json:"wrappers,omitempty"`
}

// lens resolve um campo lógico para a chave real, colocando o nome
// aprendido na frente dos aliases conhecidos. ps é a tabela de PlayStyles
// (eaId -> nome) — carregada uma vez pelo Client e repassada aqui para
// mapPlayStyles/mapUpgrade resolverem o eaId sem cada um ter que buscá-la.
type lens struct {
	fm map[string]string
	ps map[int]string
}

func (c *Client) lensFor(kind string) lens {
	if c == nil {
		return lens{}
	}
	l := lens{ps: c.psTable}
	if c.cfg.FieldMaps != nil {
		l.fm = c.cfg.FieldMaps[kind]
	}
	return l
}

// k monta a lista de candidatos: aprendido primeiro, aliases depois.
func (l lens) k(logical string, fallbacks ...string) []string {
	if real, ok := l.fm[logical]; ok && real != "" {
		return append([]string{real}, fallbacks...)
	}
	return fallbacks
}

// Duration é time.Duration que serializa como "15m", "24h" no JSON.
type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		var n int64
		if err2 := json.Unmarshal(b, &n); err2 == nil {
			*d = Duration(time.Duration(n))
			return nil
		}
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// DefaultConfig traz os caminhos conhecidos do fut.gg no ciclo FC 26.
// Os endpoints marcados como TODO precisam ser confirmados com o
// comando `eafcbot discover`, que grava a resposta crua para inspeção.
func DefaultConfig() Config {
	return Config{
		BaseURL: "https://www.fut.gg",
		Cycle:   "26",
		Endpoints: map[string]string{
			"players":    "/api/fut/players/",
			"player":     "/api/fut/players/{id}/",
			"prices":     "/api/fut/player-prices/",
			"evolutions": "/api/fut/evolutions/",
			"evolution":  "/api/fut/evolutions/{slug}/",
			"objectives": "/api/fut/objectives/",
			"sbcs":       "/api/fut/sbc/sets/",
			"news":       "/api/fut/news/",
			// O clube vem pela rota PÚBLICA do perfil, a mesma que
			// fut.gg/gg-club/<voce>/ usa para os outros verem o seu elenco.
			// Ela responde 200 sem login nenhum, então não há cookie para
			// colar nem token para renovar: basta o gamer_tag estar certo.
			// A rota de clube logado (/api/gg-club/overview/) devolve
			// "You must be logged in" e exigiria sessão.
			"club": "/api/gg-club/{gamertag}/players/",
			// A tabela eaId -> nome. O elenco do GG Club manda os PlayStyles
			// como número (playstyles:[0,34,5]); sem essa tabela o motor de
			// pontuação, que compara por nome ("Rapid"), nunca acha nada.
			"playstyles": "/api/fut/playstyles/",
			// O XI titular. É rota separada da listagem de elenco — o
			// /players/ não traz escalação nenhuma — e também pública, sem
			// exigir sessão: ver ActiveSquad em collect.go.
			"club_squad": "/api/gg-club/{gamertag}/active-squad/",
			// O catálogo de funções táticas (Wide Playmaker, Box-To-Box...).
			// A carta só declara IDs (rolesPlus/rolesPlusPlus); é esta tabela
			// que traduz o ID em nome e diz a posição. Ver roles.go.
			"roles": "/api/fut/roles/",
			// Os caminhos de evolução de um JOGADOR (não de uma carta:
			// {id} é o basePlayerEaId, compartilhado por todas as versões
			// dele). Cada caminho é a carta passo a passo, do estado atual
			// ao final — inclusive o GG Rating final, que a carta sozinha
			// não tem enquanto não evoluiu de verdade. Ver evopaths.go.
			"evolution_paths": "/api/fut/evolutions/v2/26/paths/v2/{id}/",
		},
		UserAgent:      defaultUA,
		RequestsPerSec: 3,
		Concurrency:    6,
		CacheDir:       filepath.Join(os.TempDir(), "eafc-bot-cache"),
		CacheTTL:       Duration(30 * time.Minute),
	}
}

// Client é seguro para uso concorrente. Ele limita a taxa de requisições
// e cacheia em disco para não martelar o fut.gg — o bot roda uma vez por
// dia, não há motivo para ser agressivo.
type Client struct {
	cfg     Config
	http    *http.Client
	limiter *rateLimiter
	sem     chan struct{}

	mu    sync.Mutex
	stats Stats

	// psOnce/psTable cacheiam a tabela de PlayStyles em memória: é uma
	// requisição por execução (o cache em disco cobre o resto), buscada na
	// hora em que o primeiro mapPlayer/mapClubPlayer/mapEvolution precisar
	// dela. sync.Once garante que chamadas concorrentes (Club, Players e
	// Evolutions rodam em paralelo em Collect) esperem a mesma busca em vez
	// de disparar três.
	psOnce  sync.Once
	psTable map[int]string

	// rolesOnce/rolesTable seguem o mesmo padrão, para o catálogo de
	// funções táticas (ver roles.go).
	rolesOnce  sync.Once
	rolesTable *RolesTable
}

// ensurePlayStyles carrega a tabela de PlayStyles (eaId -> nome), se ainda
// não carregada. Falha em silêncio: sem a tabela, mapPlayStyles mantém o
// eaId numérico em vez de travar a coleta inteira por um endpoint acessório.
func (c *Client) ensurePlayStyles(ctx context.Context) {
	c.psOnce.Do(func() {
		u, err := c.URL("playstyles", nil)
		if err != nil {
			return
		}
		body, err := c.GetRaw(ctx, u)
		if err != nil {
			return
		}
		nodes, err := c.decodeList(body, "playstyles")
		if err != nil {
			return
		}
		table := make(map[int]string, len(nodes))
		for _, n := range nodes {
			// "eaId" > 0 pareceria a checagem óbvia, mas o próprio fut.gg
			// usa 0 como eaId legítimo (é o "Finesse Shot") — exigir > 0
			// dropava exatamente esse PlayStyle da tabela, e ele sobrava
			// sem nome em qualquer carta que o tivesse. A presença da
			// chave "eaId" é o que importa, não o valor.
			if _, ok := n.lookup("eaId"); !ok {
				continue
			}
			id := n.int("eaId", "ea_id")
			name := n.str("name")
			if name != "" {
				table[id] = name
			}
		}
		if len(table) > 0 {
			c.psTable = table
		}
	})
}

// Stats conta o que aconteceu na coleta, para o relatório dizer se os
// dados vieram frescos ou do cache.
type Stats struct {
	Requests  int   `json:"requests"`
	CacheHits int   `json:"cache_hits"`
	Retries   int   `json:"retries"`
	Errors    int   `json:"errors"`
	Bytes     int64 `json:"bytes"`
	// RobotsBypassed conta quantas rotas configuradas (endpoints já
	// aprendidos, não a sondagem do autoconfig) o robots.txt do site pede
	// para não tocar — preenchido por checkRobots (robots.go), para essa
	// leitura nunca ficar silenciosa mesmo quando não bloqueia a coleta.
	RobotsBypassed int `json:"robots_bypassed"`
}

func New(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg = DefaultConfig()
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 6
	}
	if cfg.RequestsPerSec <= 0 {
		cfg.RequestsPerSec = 3
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaultUA
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConnsPerHost: cfg.Concurrency,
				IdleConnTimeout:     90 * time.Second,
				// Fixa HTTP/1.1. O HTTP/2 do Go contra o Cloudflare do
				// fut.gg cospe "protocol error: received DATA after
				// END_STREAM" no meio da descoberta — erro que aborta a
				// sondagem daquela rota e ainda suja a saída, porque o
				// net/http2 o imprime pelo log padrão.
				//
				// Um mapa VAZIO e não-nulo é o que desliga: como este
				// Transport não define TLSClientConfig nem DialContext, o
				// Go liga HTTP/2 sozinho, e nesse caminho o
				// ForceAttemptHTTP2:false não tem efeito nenhum.
				TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
			},
		},
		limiter: newRateLimiter(cfg.RequestsPerSec),
		sem:     make(chan struct{}, cfg.Concurrency),
	}
}

func (c *Client) Stats() Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stats
}

func (c *Client) Config() Config { return c.cfg }

// ShouldRespectRobots devolve o padrão (obedecer) quando não configurado.
func (cfg Config) ShouldRespectRobots() bool {
	return cfg.RespectRobots == nil || *cfg.RespectRobots
}

// URL monta a URL de um endpoint lógico, trocando os {placeholders}.
func (c *Client) URL(endpoint string, args map[string]string) (string, error) {
	path, ok := c.cfg.Endpoints[endpoint]
	if !ok {
		return "", fmt.Errorf("endpoint %q não configurado", endpoint)
	}
	for k, v := range args {
		path = strings.ReplaceAll(path, "{"+k+"}", v)
	}
	if strings.Contains(path, "{") {
		return "", fmt.Errorf("endpoint %q ficou com placeholder não preenchido: %s", endpoint, path)
	}
	return strings.TrimRight(c.cfg.BaseURL, "/") + path, nil
}

// ErrNotFound sinaliza 404, que em coleta em lote é normal e não fatal.
var ErrNotFound = errors.New("não encontrado (404)")

// GetRaw busca uma URL respeitando rate limit, cache e retry com backoff.
// Devolve o corpo cru — é o que o comando `discover` usa para inspecionar.
func (c *Client) GetRaw(ctx context.Context, rawURL string) ([]byte, error) {
	if body, ok := c.readCache(rawURL); ok {
		c.mu.Lock()
		c.stats.CacheHits++
		c.mu.Unlock()
		return body, nil
	}

	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			c.mu.Lock()
			c.stats.Retries++
			c.mu.Unlock()
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		if err := c.limiter.wait(ctx); err != nil {
			return nil, err
		}

		body, status, err := c.do(ctx, rawURL)
		if err != nil {
			lastErr = err
			continue
		}
		switch {
		case status == http.StatusNotFound:
			return nil, ErrNotFound
		case status == http.StatusTooManyRequests || status >= 500:
			lastErr = fmt.Errorf("%s devolveu HTTP %d", rawURL, status)
			continue
		case status >= 400:
			return nil, statusError(rawURL, status, body)
		}

		c.writeCache(rawURL, body)
		return body, nil
	}

	c.mu.Lock()
	c.stats.Errors++
	c.mu.Unlock()
	return nil, fmt.Errorf("falhou após 4 tentativas: %w", lastErr)
}

// statusError leva uma pista do corpo junto do status. Um 403 que responde
// {"error":"You must be logged in…"} é falta de sessão, não bloqueio de bot
// — e sem o corpo os dois viram a mesma linha no diagnóstico da descoberta,
// que é a diferença entre "preencha session_cookie" e "desista".
func statusError(rawURL string, status int, body []byte) error {
	msg := fmt.Sprintf("%s devolveu HTTP %d", rawURL, status)
	if hint := jsonHint(body); hint != "" {
		msg += ": " + hint
	}
	return errors.New(msg)
}

// jsonHint devolve o começo do corpo quando ele é JSON. Página de erro em
// HTML não vira pista: são quilobytes de markup que não dizem nada.
func jsonHint(body []byte) string {
	t := strings.TrimSpace(string(body))
	if !strings.HasPrefix(t, "{") {
		return ""
	}
	if len(t) > 160 {
		t = t[:160] + "…"
	}
	return t
}

func (c *Client) do(ctx context.Context, rawURL string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept-Language", "pt-BR,pt;q=0.9,en;q=0.8")

	// O Accept muda conforme o que se espera: pedir JSON numa página HTML
	// faz alguns servidores responderem 406, e pedir só HTML numa rota de
	// dados faz outros devolverem a casca em vez do payload.
	switch {
	case strings.HasSuffix(rawURL, ".xml"):
		req.Header.Set("Accept", "application/xml,text/xml;q=0.9,*/*;q=0.8")
	case strings.HasSuffix(rawURL, ".js"):
		req.Header.Set("Accept", "*/*")
	case strings.Contains(rawURL, "/api/"):
		req.Header.Set("Accept", "application/json, text/plain, */*")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
	default:
		req.Header.Set("Accept",
			"text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	}

	// Referer do próprio site: é o que um cliente honesto manda ao seguir
	// um link interno, e alguns backends recusam sem ele.
	if base := strings.TrimRight(c.cfg.BaseURL, "/"); base != "" && strings.HasPrefix(rawURL, base) {
		req.Header.Set("Referer", base+"/")
	}

	// NÃO definimos Accept-Encoding: quando ele é setado à mão, o net/http
	// deixa de descomprimir a resposta sozinho e o corpo chega como gzip cru.

	if c.cfg.SessionCookie != "" {
		req.Header.Set("Cookie", c.cfg.SessionCookie)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}

	c.mu.Lock()
	c.stats.Requests++
	c.stats.Bytes += int64(len(body))
	c.mu.Unlock()

	return body, resp.StatusCode, nil
}

// GetJSON busca e decodifica direto no destino.
func (c *Client) GetJSON(ctx context.Context, rawURL string, dst any) error {
	body, err := c.GetRaw(ctx, rawURL)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, dst); err != nil {
		preview := string(body)
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}
		return fmt.Errorf("resposta de %s não é o JSON esperado: %w (começo: %s)", rawURL, err, preview)
	}
	return nil
}

// --- cache em disco ---

func (c *Client) cachePath(rawURL string) string {
	if c.cfg.CacheDir == "" {
		return ""
	}
	return filepath.Join(c.cfg.CacheDir, hashKey(rawURL)+".cache")
}

func (c *Client) readCache(rawURL string) ([]byte, bool) {
	p := c.cachePath(rawURL)
	if p == "" || c.cfg.CacheTTL == 0 {
		return nil, false
	}
	fi, err := os.Stat(p)
	if err != nil || time.Since(fi.ModTime()) > time.Duration(c.cfg.CacheTTL) {
		return nil, false
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	return b, true
}

func (c *Client) writeCache(rawURL string, body []byte) {
	p := c.cachePath(rawURL)
	if p == "" || c.cfg.CacheTTL == 0 {
		return
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, p)
}

// hashKey gera um nome de arquivo estável a partir da URL (FNV-1a).
func hashKey(s string) string {
	var h uint64 = 14695981039346656037
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return fmt.Sprintf("%016x", h)
}

// --- rate limiter (token bucket, sem dependência externa) ---

type rateLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newRateLimiter(perSec float64) *rateLimiter {
	return &rateLimiter{interval: time.Duration(float64(time.Second) / perSec)}
}

func (r *rateLimiter) wait(ctx context.Context) error {
	r.mu.Lock()
	now := time.Now()
	if r.next.Before(now) {
		r.next = now
	}
	wait := r.next.Sub(now)
	r.next = r.next.Add(r.interval)
	r.mu.Unlock()

	if wait <= 0 {
		return nil
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
