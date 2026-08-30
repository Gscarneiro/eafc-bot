package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func TestBuildEvolutionPlayerAnalysesIncluiOVR88EMantemCobertura(t *testing.T) {
	confirmada := evolutionTestPlayer(10, "confirmada", 88, domain.ST, 82)
	semPath := evolutionTestPlayer(11, "sem-path", 89, domain.CM, 83)
	inelegivel := evolutionTestPlayer(12, "inelegivel", 90, domain.CB, 84)
	falhou := evolutionTestPlayer(13, "falhou", 91, domain.RB, 85)
	naoVerificada := evolutionTestPlayer(14, "nao-verificada", 92, domain.LB, 86)
	abaixo := evolutionTestPlayer(15, "abaixo", 87, domain.ST, 99)
	snap := store.Snapshot{Cycle: "26", Club: domain.Club{Players: []domain.ClubPlayer{confirmada, semPath, inelegivel, falhou, naoVerificada, abaixo}}}
	snap.Cards = []cards.CardReport{
		{Slug: "confirmada", Player: confirmada, EvolutionStatus: cards.EvolutionConfirmed, Best: &cards.EvoPotential{Path: evolutionTestPath("Subida", domain.ST, 90, 1_000)}},
		{Slug: "sem-path", Player: semPath, EvolutionStatus: cards.EvolutionNoPath},
		{Slug: "inelegivel", Player: inelegivel, EvolutionStatus: cards.EvolutionNotEligible},
		{Slug: "falhou", Player: falhou, EvolutionStatus: cards.EvolutionFetchError},
	}

	rows := buildEvolutionPlayerAnalyses(snap)
	if len(rows) != 5 {
		t.Fatalf("linhas = %d, esperava as cinco cartas OVR 88+", len(rows))
	}
	got := map[string]cards.EvolutionStatus{}
	for _, row := range rows {
		got[row.Player.ClubItemID] = row.Status
	}
	if got["confirmada"] != cards.EvolutionConfirmed || got["sem-path"] != cards.EvolutionNoPath || got["inelegivel"] != cards.EvolutionNotEligible || got["falhou"] != cards.EvolutionFetchError || got["nao-verificada"] != cards.EvolutionNotChecked {
		t.Fatalf("cobertura = %#v", got)
	}
	if len(rows[0].Paths) != 1 {
		t.Fatalf("paths confirmados = %+v, esperava um", rows[0].Paths)
	}
}

func TestBuildEvolutionPlayerAnalysesOrdenaPathsPorGGFinalEDesempataPorCusto(t *testing.T) {
	player := evolutionTestPlayer(20, "ordem", 90, domain.ST, 70)
	melhorCaro := evolutionTestPath("GG alto caro", domain.ST, 95, 9_000)
	melhorBarato := evolutionTestPath("GG alto barato", domain.ST, 95, 1_000)
	pior := evolutionTestPath("GG menor", domain.ST, 94, 0)
	snap := store.Snapshot{Cycle: "26", Club: domain.Club{Players: []domain.ClubPlayer{player}}, Cards: []cards.CardReport{{Slug: "ordem", Player: player, EvolutionStatus: cards.EvolutionConfirmed, Graph: &domain.EvolutionGraph{}}}}
	// Um grafo vazio não tem paths lineares; o fallback de Best/Alternates é
	// usado quando Graph é nil para exercitar exatamente a ordenação pública.
	snap.Cards[0].Graph = nil
	snap.Cards[0].Best = &cards.EvoPotential{Path: melhorCaro}
	snap.Cards[0].Alternates = []cards.EvoPotential{{Path: pior}, {Path: melhorBarato}}

	rows := buildEvolutionPlayerAnalyses(snap)
	paths := rows[0].Paths
	if len(paths) != 3 {
		t.Fatalf("paths = %+v", paths)
	}
	if paths[0].Potential.Path.Chain[0] != "GG alto barato" || paths[1].Potential.Path.Chain[0] != "GG alto caro" || paths[2].Potential.Path.Chain[0] != "GG menor" {
		t.Fatalf("ordem = %+v; deve usar GG final e depois custo", paths)
	}
}

func TestEvolutionPathImpactRespeitaSlotsRepetidosEIdentidadeFisica(t *testing.T) {
	titularForte := evolutionTestPlayer(30, "forte", 90, domain.CB, 94)
	titularFraco := evolutionTestPlayer(31, "fraco", 90, domain.CB, 90)
	reserva := evolutionTestPlayer(32, "reserva", 90, domain.CB, 86)
	club := domain.Club{Players: []domain.ClubPlayer{titularForte, titularFraco, reserva}, Squad: domain.Squad{Starters: []domain.SquadSlot{{Index: 2, Position: domain.CB, PlayerID: titularForte.ID}, {Index: 3, Position: domain.CB, PlayerID: titularFraco.ID}}}}
	final := domain.Player{GGRatingPos: domain.CB, GGRating: 92}
	impact := evolutionPathImpact(club, reserva, final)
	if impact.Kind != "entra_no_xi" || impact.SlotIndex != 3 || impact.Gain != 2 {
		t.Fatalf("impacto = %+v, esperava entrada na vaga CB mais fraca", impact)
	}

	jaTitular := evolutionPathImpact(club, titularFraco, domain.Player{GGRatingPos: domain.CB, GGRating: 93})
	if jaTitular.Kind != "melhora_titular" || jaTitular.SlotIndex != 3 || jaTitular.Gain != 3 {
		t.Fatalf("titular = %+v, esperava melhora da própria vaga", jaTitular)
	}

	semGG := evolutionPathImpact(club, reserva, domain.Player{GGRatingPos: domain.CB})
	if semGG.Kind != "sem_comparacao" {
		t.Fatalf("sem GG = %+v, esperava comparação indisponível", semGG)
	}
}

func TestEvolutionPathIDPreservaCopiaEDetectaAlteracao(t *testing.T) {
	path := evolutionTestPath("Rota estável", domain.ST, 92, 2_000)
	a := evolutionPathID("26", "item:uma", path)
	b := evolutionPathID("26", "item:outra", path)
	if a == b || a != evolutionPathID("26", "item:uma", path) {
		t.Fatalf("ids físicos/estáveis = %q, %q", a, b)
	}
	versao := evolutionPathVersion(path)
	path.CoinsCost++
	if evolutionPathVersion(path) == versao {
		t.Fatal("mudança de custo não alterou version_hash")
	}
}

func TestHandleSavedEvolutionPathsFazRoundtripEProtegeOrigem(t *testing.T) {
	player := evolutionTestPlayer(40, "salvavel", 90, domain.ST, 80)
	path := evolutionTestPath("Salvar", domain.ST, 93, 500)
	snap := store.Snapshot{Cycle: "26", Club: domain.Club{Players: []domain.ClubPlayer{player}}, Cards: []cards.CardReport{{Slug: "salvavel", Player: player, EvolutionStatus: cards.EvolutionConfirmed, Best: &cards.EvoPotential{Path: path}}}}
	srv, _ := newTestServerWithSnapshot(t, snap)
	candidate := buildEvolutionPlayerAnalyses(snap)[0].Paths[0]

	blocked := httptest.NewRequest(http.MethodPost, "/api/evolucoes/caminhos/salvos", strings.NewReader(`{"path_id":"`+candidate.ID+`"}`))
	blocked.Host = "127.0.0.1:4173"
	blocked.Header.Set("Origin", "http://externo:4173")
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, blocked)
	if w.Code != http.StatusForbidden {
		t.Fatalf("origem externa = %d, esperava 403", w.Code)
	}

	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/evolucoes/caminhos/salvos", strings.NewReader(`{"path_id":"`+candidate.ID+`"}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("salvar = %d: %s", w.Code, w.Body.String())
	}

	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes/caminhos/salvos", nil))
	got := decodeJSON[savedEvolutionPathsResponse](t, w)
	if got.Count != 1 || got.Value[0].Status != "disponivel" || got.Value[0].Saved.PathID != candidate.ID {
		t.Fatalf("salvos = %+v", got)
	}

	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/evolucoes/caminhos/salvos/"+candidate.ID, nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("remover = %d: %s", w.Code, w.Body.String())
	}
}

func evolutionTestPlayer(id int64, item string, overall int, position domain.Position, gg float64) domain.ClubPlayer {
	return domain.ClubPlayer{Player: domain.Player{ID: id, Name: item, CommonName: item, Rating: overall, Position: position, GGRating: gg, GGRatingPos: position, GGRatings: map[domain.Position]float64{position: gg}}, ClubItemID: item}
}

func evolutionTestPath(name string, position domain.Position, gg float64, cost int) domain.EvolutionPath {
	return domain.EvolutionPath{Chain: []string{name}, CoinsCost: cost, Steps: []domain.Player{{}, {ID: 999, Rating: 99, Position: position, GGRatingPos: position, GGRating: gg}}}
}
