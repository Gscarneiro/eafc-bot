package discover

import (
	"context"
	"strings"
)

// O fut.gg publica um robots.txt e ele é explícito:
//
//	Disallow: /api/*
//	Disallow: /gg-club/
//	Allow:    /gg-club/$
//
// Ou seja: as rotas de API que a descoberta minerava são justamente as que
// eles pedem para não bater. A resposta certa não é trocar o User-Agent por
// um de navegador para passar despercebido — isso é contornar uma política
// declarada, e além de tudo é o que provavelmente já está devolvendo 403.
// A resposta certa é o bot ler o robots.txt e obedecer, dizendo o que pulou
// e por quê.
//
// O que sobra é bastante: as páginas de detalhe de jogador são renderizadas
// no servidor (é conteúdo de SEO), e o próprio robots.txt anuncia os
// sitemaps que as enumeram. Esse é o caminho que eles abriram de propósito.

// Robots é o conjunto de regras aplicáveis ao nosso agente.
type Robots struct {
	allow    []string
	disallow []string
	Sitemaps []string
	// Loaded diz se conseguimos ler o arquivo. Se não conseguimos, o bot
	// age de forma conservadora em vez de assumir que pode tudo.
	Loaded bool
}

// FetchRobots busca e interpreta o robots.txt do site.
func FetchRobots(ctx context.Context, f Fetcher, baseURL string) *Robots {
	r := &Robots{}
	body, err := f.GetRaw(ctx, strings.TrimRight(baseURL, "/")+"/robots.txt")
	if err != nil {
		return r
	}
	r.parse(string(body))
	r.Loaded = true
	return r
}

// parse lê o arquivo. Só consideramos os blocos que se aplicam a nós:
// "User-agent: *". Um bot pessoal não deve procurar brecha em regra escrita
// para outro agente.
func (r *Robots) parse(body string) {
	applies := false
	for _, raw := range strings.Split(body, "\n") {
		line := raw
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)

		switch key {
		case "user-agent":
			applies = value == "*"
		case "sitemap":
			// Sitemap vale para qualquer agente, fora dos blocos.
			r.Sitemaps = append(r.Sitemaps, value)
		case "allow":
			if applies && value != "" {
				r.allow = append(r.allow, value)
			}
		case "disallow":
			if applies && value != "" {
				r.disallow = append(r.disallow, value)
			}
		}
	}
}

// Allowed diz se podemos buscar o caminho. Segue a regra padrão: vale a
// regra mais específica (mais longa); empate, o Allow ganha.
func (r *Robots) Allowed(path string) bool {
	if r == nil || !r.Loaded {
		return true // sem robots.txt legível, não há restrição declarada
	}
	if i := strings.Index(path, "://"); i >= 0 {
		if j := strings.IndexByte(path[i+3:], '/'); j >= 0 {
			path = path[i+3+j:]
		} else {
			path = "/"
		}
	}
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}

	bestAllow, bestDisallow := -1, -1
	for _, p := range r.allow {
		if n := matchLen(path, p); n > bestAllow {
			bestAllow = n
		}
	}
	for _, p := range r.disallow {
		if n := matchLen(path, p); n > bestDisallow {
			bestDisallow = n
		}
	}
	if bestDisallow < 0 {
		return true
	}
	return bestAllow >= bestDisallow
}

// matchLen devolve o tamanho do padrão se ele casa com o caminho, ou -1.
// Entende o curinga "*" e a âncora de fim "$" — é o "/gg-club/$" do fut.gg
// que libera só a landing e mantém o resto bloqueado.
func matchLen(path, pattern string) int {
	anchored := strings.HasSuffix(pattern, "$")
	p := strings.TrimSuffix(pattern, "$")

	parts := strings.Split(p, "*")
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		if i == 0 {
			if !strings.HasPrefix(path[pos:], part) {
				return -1
			}
			pos += len(part)
			continue
		}
		idx := strings.Index(path[pos:], part)
		if idx < 0 {
			return -1
		}
		pos += idx + len(part)
	}
	if anchored && pos != len(path) {
		return -1
	}
	return len(p)
}

// FilterCandidates separa o que podemos sondar do que o site pediu para
// não tocarmos, devolvendo os dois para o relatório poder ser honesto
// sobre o que ficou de fora.
//
// A amostra de bloqueadas é capada, mas a CONTAGEM sai separada e inteira:
// imprimir o tamanho da amostra como se fosse o total fazia o relatório
// anunciar "40 rotas puladas" quando eram centenas — e o número é
// justamente o que diz se o robots.txt é ou não a causa da descoberta vazia.
func (r *Robots) FilterCandidates(cands []Candidate) (allowed []Candidate, blocked []string, total int) {
	for _, c := range cands {
		if r.Allowed(c.URL) {
			allowed = append(allowed, c)
			continue
		}
		total++
		if len(blocked) < 40 {
			blocked = append(blocked, c.URL)
		}
	}
	return allowed, blocked, total
}
