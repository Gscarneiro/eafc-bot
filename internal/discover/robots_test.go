package discover

import (
	"fmt"
	"testing"
)

// As regras abaixo são as reais do fut.gg, coladas do robots.txt deles.
// Este teste existe para o bot nunca voltar a sondar /api/* por descuido.
const futggRobots = `
User-agent: *
Disallow: /accounts/*
Disallow: /account
Disallow: /auth/*
Disallow: /user/delete-account
Disallow: /api/*
Allow: /gg-club/$
Disallow: /gg-club/
Disallow: /admin/*
Disallow: /gg-rating
Disallow: /tier-list/free/
Disallow: /evo-lab-2/

Sitemap: https://www.fut.gg/sitemap.xml
Sitemap: https://www.fut.gg/sitemap-google-news.xml
`

func futgg() *Robots {
	r := &Robots{}
	r.parse(futggRobots)
	r.Loaded = true
	return r
}

func TestRespeitaOsBloqueiosDoFutGG(t *testing.T) {
	r := futgg()

	bloqueado := []string{
		"/api/fut/players/",
		"/api/fut/evolutions/",
		"https://www.fut.gg/api/fut/gg-club/carneiro22/",
		"/accounts/login",
		"/auth/callback",
		"/admin/panel",
		"/evo-lab-2/algo",
	}
	for _, p := range bloqueado {
		if r.Allowed(p) {
			t.Errorf("%s deveria estar bloqueado pelo robots.txt", p)
		}
	}

	permitido := []string{
		"/players/192985-kevin-de-bruyne/26-117633497/",
		"/evolutions/",
		"/sbc/",
		"/objectives/",
		"/news/tots-serie-a/",
		"/sitemap.xml",
		"/",
	}
	for _, p := range permitido {
		if !r.Allowed(p) {
			t.Errorf("%s deveria ser permitido", p)
		}
	}
}

// "Allow: /gg-club/$" libera SÓ a landing; "/gg-club/qualquer-um/" continua
// bloqueado. É o caso que mais fácil se erra, e o que decide se o bot vai
// buscar o clube de alguém sem permissão.
func TestAncoraDeFimLiberaSoALanding(t *testing.T) {
	r := futgg()

	if !r.Allowed("/gg-club/") {
		t.Error("/gg-club/ (a landing) deveria ser permitido por causa do $")
	}
	for _, p := range []string{"/gg-club/carneiro22/", "/gg-club/alguem", "/gg-club/x/y/"} {
		if r.Allowed(p) {
			t.Errorf("%s deveria continuar bloqueado — o $ ancora só a landing", p)
		}
	}
}

func TestSitemapsSaoColhidos(t *testing.T) {
	r := futgg()
	if len(r.Sitemaps) != 2 {
		t.Fatalf("colheu %d sitemaps, esperava 2", len(r.Sitemaps))
	}
	if r.Sitemaps[0] != "https://www.fut.gg/sitemap.xml" {
		t.Errorf("primeiro sitemap: %q", r.Sitemaps[0])
	}
}

// Regra escrita para outro agente não é brecha para nós.
func TestSoAplicaRegrasDoAgenteCoringa(t *testing.T) {
	r := &Robots{}
	r.parse("User-agent: Googlebot\nDisallow: /segredo/\n\nUser-agent: *\nDisallow: /api/\n")
	r.Loaded = true

	if !r.Allowed("/segredo/x") {
		t.Error("regra do Googlebot não se aplica a nós")
	}
	if r.Allowed("/api/x") {
		t.Error("regra do agente coringa se aplica a nós")
	}
}

// Sem robots.txt legível não inventamos restrição — mas também não é
// desculpa para nada, porque o fut.gg publica o dele.
func TestSemRobotsNaoRestringe(t *testing.T) {
	var r *Robots
	if !r.Allowed("/api/qualquer") {
		t.Error("robots nulo não deveria bloquear")
	}
	vazio := &Robots{}
	if !vazio.Allowed("/api/qualquer") {
		t.Error("robots não carregado não deveria bloquear")
	}
}

func TestFiltraCandidatasAntesDeQualquerRequisicao(t *testing.T) {
	r := futgg()
	cands := []Candidate{
		{URL: "https://www.fut.gg/api/fut/players/"},
		{URL: "https://www.fut.gg/players/1-x/26-1/"},
		{URL: "https://www.fut.gg/api/fut/evolutions/"},
		{URL: "https://www.fut.gg/evolutions/"},
	}
	allowed, blocked, total := r.FilterCandidates(cands)
	if len(allowed) != 2 {
		t.Errorf("sobraram %d permitidas, esperava 2", len(allowed))
	}
	if len(blocked) != 2 {
		t.Errorf("bloqueou %d, esperava 2", len(blocked))
	}
	if total != 2 {
		t.Errorf("contou %d bloqueadas, esperava 2", total)
	}
}

// A amostra de bloqueadas é capada em 40, mas a contagem não pode ser: era
// isso que fazia o relatório anunciar "40 rotas puladas" quando o robots.txt
// tinha derrubado centenas — e o número é justamente o que explica a
// descoberta vazia.
func TestContagemDeBloqueadasNaoParaNoTetoDaAmostra(t *testing.T) {
	r := futgg()
	var cands []Candidate
	for i := 0; i < 150; i++ {
		cands = append(cands, Candidate{
			URL: fmt.Sprintf("https://www.fut.gg/api/fut/rota-%d/", i),
		})
	}
	allowed, blocked, total := r.FilterCandidates(cands)
	if len(allowed) != 0 {
		t.Errorf("passaram %d permitidas, esperava 0", len(allowed))
	}
	if len(blocked) != 40 {
		t.Errorf("amostra com %d, esperava o teto de 40", len(blocked))
	}
	if total != 150 {
		t.Errorf("contou %d bloqueadas, esperava 150", total)
	}
}
