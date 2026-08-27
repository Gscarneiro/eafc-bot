package cards

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
)

func path(finalOverall int, finalGG float64, cost int, expired bool) domain.EvolutionPath {
	return domain.EvolutionPath{
		Steps: []domain.Player{
			{Rating: 89}, // inicial — GGRating fica 0 de propósito, como a API manda
			{Rating: finalOverall, GGRating: finalGG},
		},
		CoinsCost: cost,
		IsExpired: expired,
	}
}

// "Melhor" é o de maior GG Rating final — não o de maior overall, não o mais
// barato. É a mesma régua que decide "elo mais fraco" no resto do bot.
func TestBestPathsEscolheMaiorGGRatingFinal(t *testing.T) {
	current := domain.Player{GGRating: 88.4}
	paths := []domain.EvolutionPath{
		path(98, 95.0, 50000, false),
		path(98, 97.3, 10000, false),
		path(97, 96.0, 20000, false),
	}
	best, alts := bestPaths(paths, current)
	if best == nil {
		t.Fatal("best veio nil")
	}
	if best.FinalGGRating != 97.3 {
		t.Errorf("best.FinalGGRating = %v, esperava 97.3", best.FinalGGRating)
	}
	if len(alts) != 2 {
		t.Errorf("alternates = %d, esperava 2", len(alts))
	}
}

// Caminho expirado não pode ser sugerido, mesmo sendo o de maior GG Rating —
// é uma evolução que não dá mais para pegar.
func TestBestPathsIgnoraExpirado(t *testing.T) {
	current := domain.Player{GGRating: 88.4}
	paths := []domain.EvolutionPath{
		path(98, 99.0, 10000, true), // o melhor, mas expirado
		path(97, 95.0, 20000, false),
	}
	best, _ := bestPaths(paths, current)
	if best == nil || best.FinalGGRating != 95.0 {
		t.Fatalf("best = %+v, esperava o não-expirado (95.0)", best)
	}
}

// Empate em GG Rating final desempata pelo caminho mais barato.
func TestBestPathsDesempataPorCusto(t *testing.T) {
	current := domain.Player{GGRating: 88.4}
	paths := []domain.EvolutionPath{
		path(98, 97.0, 60000, false),
		path(98, 97.0, 10000, false),
	}
	best, _ := bestPaths(paths, current)
	if best == nil || best.CoinsCost != 10000 {
		t.Fatalf("best.CoinsCost = %v, esperava 10000 (o mais barato)", best)
	}
}

// Uma carta já muito boa pode "caber" no requisito de overall de uma
// evolução pensada pra carta mais fraca, mas o teto de GG Rating dessa
// evolução fica ABAIXO do que a carta já tem — caso real: Josh Acheampong,
// GG 97.3 hoje, só achava caminhos até 96.0. Isso não pode virar "Best":
// seria recomendar uma piora como se fosse potencial.
func TestBestPathsNaoRecomendaCaminhoQuePiora(t *testing.T) {
	current := domain.Player{GGRating: 97.3}
	paths := []domain.EvolutionPath{path(98, 96.0, 60000, false)} // pior que os 97.3 de hoje
	if best, alts := bestPaths(paths, current); best != nil || alts != nil {
		t.Fatalf("recomendou caminho que piora: best=%+v alts=%+v", best, alts)
	}
}

// Entre um caminho que piora e outro que melhora, só o que melhora conta.
func TestBestPathsFiltraSoOsQueMelhoram(t *testing.T) {
	current := domain.Player{GGRating: 90.0}
	paths := []domain.EvolutionPath{
		path(98, 89.0, 10000, false), // pior — descartado
		path(98, 93.0, 20000, false), // melhor — este é o Best
	}
	best, alts := bestPaths(paths, current)
	if best == nil || best.FinalGGRating != 93.0 {
		t.Fatalf("best = %+v, esperava só o de 93.0", best)
	}
	if len(alts) != 0 {
		t.Errorf("alternates = %+v, esperava vazio (o outro caminho piora)", alts)
	}
}

// Um clube real teve duas entradas com o MESMO eaId (duas cópias/estágios
// de "Carlos Alberto", overall 95 e 92) — sem desambiguar, as duas cartas
// receberiam o MESMO Slug, e a rota /cards/:slug no front não teria como
// distinguir qual delas abrir.
func TestAssignSlugsDesambiguaEaIdRepetido(t *testing.T) {
	reports := []CardReport{
		{Player: domain.ClubPlayer{Player: domain.Player{ID: 1, Rating: 95, FutGGSlug: "26-67275999"}}},
		{Player: domain.ClubPlayer{Player: domain.Player{ID: 1, Rating: 92, FutGGSlug: "26-67275999"}}},
	}
	assignSlugs(reports)

	if reports[0].Slug == reports[1].Slug {
		t.Fatalf("os dois ficaram com o mesmo slug: %q", reports[0].Slug)
	}
	if reports[0].Slug != "26-67275999" {
		t.Errorf("primeiro slug = %q, esperava o base sem sufixo", reports[0].Slug)
	}
	if reports[1].Slug != "26-67275999-92" {
		t.Errorf("segundo slug = %q, esperava desambiguado pelo overall", reports[1].Slug)
	}
}

// Sem colisão, o slug é só o do fut.gg, sem sufixo nenhum.
func TestAssignSlugsSemColisaoNaoMexe(t *testing.T) {
	reports := []CardReport{
		{Player: domain.ClubPlayer{Player: domain.Player{ID: 1, Rating: 99, FutGGSlug: "26-168027413"}}},
		{Player: domain.ClubPlayer{Player: domain.Player{ID: 2, Rating: 98, FutGGSlug: "26-117646625"}}},
	}
	assignSlugs(reports)
	if reports[0].Slug != "26-168027413" || reports[1].Slug != "26-117646625" {
		t.Errorf("slugs = %q, %q — não deviam ganhar sufixo", reports[0].Slug, reports[1].Slug)
	}
}

// Carta sem nenhum caminho válido é "no teto" — Best fica nil, e isso
// precisa continuar sendo uma resposta válida, não um erro.
func TestBestPathsSemCaminhosDevolveNil(t *testing.T) {
	if best, alts := bestPaths(nil, domain.Player{GGRating: 99.1}); best != nil || alts != nil {
		t.Errorf("esperava nil/nil pra carta sem caminhos, veio %+v / %+v", best, alts)
	}
	// Todos expirados é equivalente a "nenhum": também fica nil.
	only := []domain.EvolutionPath{path(98, 99.0, 10000, true)}
	if best, _ := bestPaths(only, domain.Player{GGRating: 99.1}); best != nil {
		t.Errorf("todos expirados devia dar nil, veio %+v", best)
	}
}

func TestBuildReportsPreservaFalhaDePathComoEstado(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "roles") {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`falha simulada`))
	}))
	defer srv.Close()

	client := futgg.New(futgg.Config{BaseURL: srv.URL, Cycle: "26", Endpoints: map[string]string{
		"roles":           "/api/fut/roles/",
		"evolution_paths": "/api/fut/evolutions/{id}/",
	}})
	club := domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{
		ID: 7, Name: "Carta", Rating: 90, BasePlayerEaID: 700,
	}}}}
	reports, err := BuildReports(context.Background(), client, club, 80, nil)
	if err != nil {
		t.Fatalf("BuildReports: %v", err)
	}
	if len(reports) != 1 || reports[0].EvolutionStatus != EvolutionFetchError {
		t.Fatalf("status = %+v, esperava fetch_error", reports)
	}
	if reports[0].EvolutionError == "" {
		t.Fatal("falha de path não foi preservada")
	}
}

// CardReport.Graph precisa vir preenchido quando o path é confirmado — é o
// que /api/evolucoes/{slug}/plano vai ler, sem precisar de rede.
func TestBuildReportsPreenchaGraphQuandoPathConfirmado(t *testing.T) {
	const evoPathsFixture = `{"data":[
	 {"path":[
	   {"id":900001,"eaId":7,"overall":90,"commonName":"Carta"},
	   {"id":900002,"eaId":7,"overall":95,"ggRating":92.0,"commonName":"Carta"}
	 ],"coinsCost":15000,"pointsCost":0,"isExpired":false,
	  "readableTrainingTime":"3 days","evolutions":[{"name":"Salto"}]}
	]}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "roles") {
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		_, _ = w.Write([]byte(evoPathsFixture))
	}))
	defer srv.Close()

	client := futgg.New(futgg.Config{BaseURL: srv.URL, Cycle: "26", Endpoints: map[string]string{
		"roles":           "/api/fut/roles/",
		"evolution_paths": "/api/fut/evolutions/{id}/",
	}})
	club := domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{
		ID: 7, Name: "Carta", Rating: 90, GGRating: 88.4, Cycle: "26", BasePlayerEaID: 700,
	}}}}
	reports, err := BuildReports(context.Background(), client, club, 80, nil)
	if err != nil {
		t.Fatalf("BuildReports: %v", err)
	}
	if len(reports) != 1 {
		t.Fatalf("esperava 1 relatório, achou %d", len(reports))
	}
	r := reports[0]
	if r.Graph == nil {
		t.Fatal("Graph veio nil com path confirmado")
	}
	if err := r.Graph.Validate(); err != nil {
		t.Errorf("Graph inválido: %v", err)
	}
	linear := r.Graph.LinearPaths()
	if len(linear) != 1 || linear[0].CoinsCost != 15000 {
		t.Errorf("LinearPaths() = %+v, esperava 1 caminho de 15000 moedas", linear)
	}
	if r.EvolutionStatus != EvolutionConfirmed || r.Best == nil {
		t.Errorf("status/Best = %v/%+v, esperava confirmed com Best preenchido", r.EvolutionStatus, r.Best)
	}
}

// ggGain usa a carta REAL de hoje (com GGRating de verdade), não path[0] —
// que a API sempre manda sem nota (ver domain.EvolutionPath.Initial).
func TestGgGainUsaCartaAtualNaoPathInicial(t *testing.T) {
	current := domain.Player{GGRating: 88.4} // veio do elenco, de verdade
	final := domain.Player{GGRating: 97.3}
	if got := ggGain(current, final); got < 8.8 || got > 8.9 {
		t.Errorf("ggGain = %v, esperava ~8.9 (97.3-88.4)", got)
	}
	// Sem GGRating em algum dos lados, não inventa um ganho.
	if got := ggGain(domain.Player{}, final); got != 0 {
		t.Errorf("sem GGRating atual, ggGain deveria ser 0, veio %v", got)
	}
}

// positionRoles cruza os ids da carta com o catálogo e agrupa por posição,
// natural primeiro. Os valores (128->Wide Midfielder/LM) são os que
// confirmei ao vivo contra a API real.
func TestPositionRolesAgrupaPorPosicaoNaturalPrimeiro(t *testing.T) {
	p := domain.Player{
		Position:      domain.CAM,
		AltPositions:  []domain.Position{domain.LM},
		RolesPlusPlus: []int{128, 114}, // 128 = LM, 114 = CDM (não é a posição nem alt!)
		RolesPlus:     []int{28},
	}
	roles := futgg.RolesTable{
		PlusPlus: map[int]futgg.Role{
			128: {Name: "Wide Midfielder", Position: domain.LM},
			114: {Name: "Holding", Position: domain.CDM},
		},
		Plus: map[int]futgg.Role{
			28: {Name: "Wide Midfielder (fraco)", Position: domain.LM},
		},
	}

	out := positionRoles(p, roles)
	if len(out) != 2 {
		t.Fatalf("agrupou em %d posições, esperava 2 (LM e CDM)", len(out))
	}
	// LM é alt position (rank 1) e vem antes de CDM (rank "outros", maior).
	if out[0].Position != domain.LM {
		t.Errorf("primeira posição = %q, esperava LM (é alt position da carta)", out[0].Position)
	}
	if len(out[0].PlusPlus) != 1 || out[0].PlusPlus[0] != "Wide Midfielder" {
		t.Errorf("LM PlusPlus = %v", out[0].PlusPlus)
	}
	if len(out[0].Plus) != 1 {
		t.Errorf("LM Plus = %v, esperava 1 função", out[0].Plus)
	}
}

// Id sem correspondência no catálogo não trava nem inventa nome — a
// posição simplesmente fica sem aquele detalhe.
func TestPositionRolesIgnoraIdSemCorrespondencia(t *testing.T) {
	p := domain.Player{Position: domain.ST, RolesPlusPlus: []int{999999}}
	out := positionRoles(p, futgg.RolesTable{Plus: map[int]futgg.Role{}, PlusPlus: map[int]futgg.Role{}})
	if len(out) != 0 {
		t.Errorf("esperava nenhuma posição (id não bateu no catálogo), veio %+v", out)
	}
}
