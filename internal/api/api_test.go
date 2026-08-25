package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// fixtureSnapshot é o mesmo snapshot fixo usado por todos os testes de
// rota — pequeno o bastante para inspecionar à mão, com um titular, um
// reserva e uma oportunidade de mercado.
func fixtureSnapshot() store.Snapshot {
	titular := domain.ClubPlayer{
		Player: domain.Player{ID: 1, Name: "Titular", CommonName: "Titular",
			Rating: 88, Position: domain.CM, GGRating: 88.5},
		InSquad: true, SquadSlot: domain.CM,
	}
	reserva := domain.ClubPlayer{
		Player: domain.Player{ID: 2, Name: "Reserva", CommonName: "Reserva",
			Rating: 82, Position: domain.CB, GGRating: 81.0},
	}
	club := domain.Club{
		GamerTag: "BilingualBee", Cycle: "26", Coins: 50_000,
		Players: []domain.ClubPlayer{titular, reserva},
		Squad: domain.Squad{
			Formation: "4-2-3-1",
			Starters:  []domain.SquadSlot{{Index: 6, Position: domain.CM, PlayerID: 1}},
		},
	}

	return store.Snapshot{
		GeneratedAt: time.Date(2026, 8, 21, 5, 0, 0, 0, time.UTC),
		Cycle:       "26",
		Club:        club,
		SquadScore:  85.75,
		NewCards:    []domain.Player{{ID: 9, Name: "Carta Nova"}},
		FreshNews:   []domain.NewsItem{{ID: "n1", Title: "Notícia"}},
		Upgrades: []analyze.Upgrade{
			{Slot: domain.CM, Candidate: domain.Player{ID: 3, Name: "Alvo"}},
		},
		EvoMatches: []analyze.EvoMatch{{Player: reserva}},
		Cards: []cards.CardReport{
			{Slug: "26-1", Player: titular},
		},
	}
}

func fixtureSnapshotComEvolucaoFutGG() store.Snapshot {
	snap := fixtureSnapshot()
	evolucao := domain.Evolution{ID: "evo-reserva", Name: "Evolução Reserva"}
	snap.Evolutions = []domain.Evolution{evolucao}
	snap.EvoMatches[0].Evolution = evolucao
	snap.EvoMatches[0].Slot = domain.CB
	snap.Cards = append(snap.Cards, cards.CardReport{
		Slug: "26-2", Player: snap.EvoMatches[0].Player,
		Best: &cards.EvoPotential{
			Path:          domain.EvolutionPath{Chain: []string{evolucao.Name}},
			FinalOverall:  85,
			FinalGGRating: 84,
			GGRatingGain:  3,
		},
	})
	return snap
}

func TestHandleEvolucoesNaoDependeDaEstimativaDoAnalyze(t *testing.T) {
	snap := fixtureSnapshotComEvolucaoFutGG()
	snap.EvoMatches = nil
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes", nil))

	got := decodeJSON[EvolucoesResponse](t, w)
	if got.Total != 1 || len(got.Matches) != 1 || got.Matches[0].Evolution.ID != "evo-reserva" {
		t.Fatalf("path fut.gg desapareceu sem EvoMatches: total=%d matches=%+v", got.Total, got.Matches)
	}
}

func TestHandleConfigEditaSomenteOrigemLocal(t *testing.T) {
	cfg := config.Default()
	srv := &Server{Config: &ConfigEditor{
		Get: func() config.UISettings { return cfg.Editable() },
		Update: func(v config.UISettings) (config.UISettings, error) {
			if err := cfg.ApplyEditable(v); err != nil {
				return config.UISettings{}, err
			}
			return cfg.Editable(), nil
		},
	}}

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/config status %d", w.Code)
	}
	got := decodeJSON[ConfigResponse](t, w)
	if got.Settings.Report.MinGain != cfg.Report.MinGain {
		t.Errorf("min_gain = %v, esperava %v", got.Settings.Report.MinGain, cfg.Report.MinGain)
	}

	next := got.Settings
	next.Report.MinGain = 4.5
	body, err := json.Marshal(next)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Host = "127.0.0.1:4173"
	req.Header.Set("Origin", "http://127.0.0.1:4173")
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK || cfg.Report.MinGain != 4.5 {
		t.Fatalf("PUT /api/config status=%d min_gain=%v body=%s", w.Code, cfg.Report.MinGain, w.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body))
	req.Host = "127.0.0.1:4173"
	req.Header.Set("Origin", "http://outro-host:4173")
	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("origem externa: status=%d, esperava 403", w.Code)
	}
}

func newTestServer(t *testing.T) (*Server, store.Store) {
	return newTestServerWithSnapshot(t, fixtureSnapshot())
}

func newTestServerWithSnapshot(t *testing.T, snap store.Snapshot) (*Server, store.Store) {
	t.Helper()
	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	if err := st.SaveSnapshot(t.Context(), snap); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	return &Server{
		Store:   st,
		Cycle:   "26",
		History: 30,
		Trigger: func() {},
		Status:  func() JobStatus { return JobStatus{} },
	}, st
}

func decodeJSON[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatalf("decodificando resposta %s: %v\nbody: %s", w.Result().Status, err, w.Body.String())
	}
	return v
}

func TestHandleStatus(t *testing.T) {
	srv, _ := newTestServer(t)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/status", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[StatusResponse](t, w)
	if got.Coins != 50_000 {
		t.Errorf("Coins = %d, esperava 50000", got.Coins)
	}
	if got.SquadScore <= 0 {
		t.Errorf("SquadScore = %v, esperava > 0", got.SquadScore)
	}
	if len(got.NewCards) != 1 || got.NewCards[0].Name != "Carta Nova" {
		t.Errorf("NewCards = %+v", got.NewCards)
	}
	// A fixture tem um upgrade e uma evolução, ambos com ganho zero-valor —
	// no empate bestMove fica com o upgrade (primeiro a ser avaliado).
	if got.TopMove == nil || got.TopMove.Kind != "upgrade" || got.TopMove.Slot != domain.CM {
		t.Errorf("TopMove = %+v, esperava upgrade no CM", got.TopMove)
	}
}

func TestBestMoveDevolveNilSemUpgradeNemEvolucao(t *testing.T) {
	if m := bestMove(store.Snapshot{}); m != nil {
		t.Errorf("bestMove = %+v, esperava nil sem upgrades nem evoluções", m)
	}
}

func TestBestMoveEscolheOMaiorGanhoEntreUpgradeEEvolucao(t *testing.T) {
	snap := store.Snapshot{
		Upgrades: []analyze.Upgrade{
			{Slot: domain.CB, Gain: 2.0, Current: domain.ClubPlayer{Player: domain.Player{Name: "Atual"}}, Candidate: domain.Player{Name: "Alvo"}},
		},
		EvoMatches: []analyze.EvoMatch{
			{Slot: domain.ST, Gain: 5.0, Player: domain.ClubPlayer{Player: domain.Player{Name: "Base"}}, Evolution: domain.Evolution{Name: "Evo"}},
		},
	}
	got := bestMove(snap)
	if got == nil || got.Kind != "evolution" || got.Slot != domain.ST {
		t.Errorf("bestMove = %+v, esperava a evolução (ganho 5.0 > 2.0 do upgrade)", got)
	}

	// Invertendo os ganhos, o upgrade passa a vencer.
	snap.Upgrades[0].Gain, snap.EvoMatches[0].Gain = 8.0, 1.0
	got = bestMove(snap)
	if got == nil || got.Kind != "upgrade" || got.Slot != domain.CB {
		t.Errorf("bestMove = %+v, esperava o upgrade (ganho 8.0 > 1.0 da evolução)", got)
	}
}

func TestHandleStatusSemSnapshotDevolve503(t *testing.T) {
	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	srv := &Server{Store: st, Cycle: "26", Trigger: func() {}, Status: func() JobStatus { return JobStatus{} }}

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/status", nil))
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, esperava %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestHandleInvestimentos(t *testing.T) {
	srv, _ := newTestServer(t)

	// Sem momentum/custo de SBC salvos ainda, a rota responde mesmo assim
	// — são sinais opcionais (ver o comentário de InvestimentosResponse).
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/investimentos", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[InvestimentosResponse](t, w)

	// A "Reserva" da fixture está fora do XI titular (só o CM id=1 é
	// titular) e não tem CardReport — deveria virar sugestão de venda.
	foundReserva := false
	for _, c := range got.SellCandidates {
		if c.Player.ID == 2 {
			foundReserva = true
			if c.Recommendation != "vender" {
				t.Errorf("Reserva deveria ser \"vender\" (sem CardReport, sem uso), veio %q", c.Recommendation)
			}
		}
	}
	if !foundReserva {
		t.Fatalf("esperava a Reserva (id=2) entre os candidatos de venda, veio %+v", got.SellCandidates)
	}
	if got.InvestmentFunnel.Considered != 0 {
		t.Errorf("sem momentum salvo, InvestmentFunnel.Considered deveria ser 0, veio %d", got.InvestmentFunnel.Considered)
	}
}

// A rota reflete o momentum mais recente salvo pelo ciclo de coleta
// rápido (scheduler.FastTicker) — não um campo congelado no snapshot
// diário, que é justamente o motivo de handleInvestimentos calcular na
// hora em vez de ler um campo pronto (ver o comentário do tipo).
func TestHandleInvestimentosUsaOMomentumMaisRecenteDoStore(t *testing.T) {
	srv, st := newTestServer(t)
	if err := st.SaveMomentum(t.Context(), "26", []domain.Player{
		{ID: 50, Name: "Descontado", Rating: 90, Price: domain.Price{Coins: 40000}, MomentumPct: 40},
	}); err != nil {
		t.Fatalf("SaveMomentum: %v", err)
	}

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/investimentos", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[InvestimentosResponse](t, w)

	if len(got.Investments) != 1 || got.Investments[0].Candidate.ID != 50 {
		t.Fatalf("esperava 1 investimento (id=50, o momentum recém-salvo), veio %+v", got.Investments)
	}
}

func TestHandleTime(t *testing.T) {
	snap := fixtureSnapshot()
	snap.Club.Players[1].Rating = 89
	snap.Club.Players[1].GGRating = 89
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/time", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[TimeResponse](t, w)
	if len(got.Starters) != 1 || got.Starters[0].Player.ID != 1 {
		t.Errorf("Starters = %+v", got.Starters)
	}
	if len(got.Bench) != 1 || got.Bench[0].Player.ID != 2 {
		t.Errorf("Bench = %+v", got.Bench)
	}
	// O titular tem CardReport na fixture (slug "26-1"); o reserva não —
	// CardSlug tem que refletir exatamente essa diferença.
	if got.Starters[0].CardSlug != "26-1" {
		t.Errorf("Starters[0].CardSlug = %q, esperava %q", got.Starters[0].CardSlug, "26-1")
	}
	if got.Bench[0].CardSlug != "" {
		t.Errorf("Bench[0].CardSlug = %q, esperava vazio (sem CardReport nesse jogador)", got.Bench[0].CardSlug)
	}
}

func TestHandleTimePaginaEFiltraReservasAcimaDoPiso(t *testing.T) {
	snap := fixtureSnapshot()
	snap.Club.Players = append(snap.Club.Players,
		domain.ClubPlayer{Player: domain.Player{ID: 3, Name: "Baixa", Rating: 87, Position: domain.CM}},
		domain.ClubPlayer{Player: domain.Player{ID: 4, Name: "Lateral negociável", Rating: 91, Position: domain.RB, GGRating: 92}},
		domain.ClubPlayer{Player: domain.Player{ID: 5, Name: "Lateral inegociável", Rating: 90, Position: domain.RB, GGRating: 93}, Untradeable: true},
	)
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/time?bench_position=RB&bench_tradeable=tradeable&bench_size=1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[TimeResponse](t, w)
	if got.BenchTotal != 1 || len(got.Bench) != 1 || got.Bench[0].Player.ID != 4 {
		t.Fatalf("reservas filtradas = total %d, cartas %+v", got.BenchTotal, got.Bench)
	}
	if got.BenchPage != 1 || got.BenchPageSize != 1 {
		t.Errorf("página = %d, tamanho = %d", got.BenchPage, got.BenchPageSize)
	}
}

func TestInferFormationReconheceSnapshotAntigo4411(t *testing.T) {
	positions := []domain.Position{domain.GK, domain.RB, domain.CB, domain.CB, domain.LB, domain.RM, domain.CM, domain.CM, domain.LM, domain.CAM, domain.ST}
	slots := make([]domain.SquadSlot, len(positions))
	for i, position := range positions {
		slots[i] = domain.SquadSlot{Index: i, Position: position}
	}
	if got := inferFormation(slots); got != "4-4-1-1" {
		t.Errorf("inferFormation = %q, esperava 4-4-1-1", got)
	}
}

func TestHandleTimeSlug(t *testing.T) {
	srv, _ := newTestServer(t)

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/time/26-1", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[CardDetailResponse](t, w)
	if got.Slug != "26-1" || got.Player.ID != 1 {
		t.Errorf("CardDetailResponse = %+v", got)
	}

	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/time/nao-existe", nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("slug inexistente: status = %d, esperava 404", w.Code)
	}
}

func TestHandleMercado(t *testing.T) {
	srv, _ := newTestServer(t)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/mercado", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[MercadoResponse](t, w)
	if len(got.Upgrades) != 1 || got.Upgrades[0].Candidate.Name != "Alvo" {
		t.Errorf("Upgrades = %+v", got.Upgrades)
	}
}

func TestHandleEvolucoes(t *testing.T) {
	srv, _ := newTestServerWithSnapshot(t, fixtureSnapshotComEvolucaoFutGG())
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[EvolucoesResponse](t, w)
	if len(got.Matches) != 1 || got.Matches[0].Player.ID != 2 {
		t.Errorf("Matches = %+v", got.Matches)
	}
}

func TestHandleEvolucoesPaginaNoServidor(t *testing.T) {
	srv, _ := newTestServerWithSnapshot(t, fixtureSnapshotComEvolucaoFutGG())
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes?page=2&page_size=1", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[EvolucoesResponse](t, w)
	if got.Page != 1 || got.PageSize != 1 || got.Total != 1 || got.Pages != 1 {
		t.Fatalf("paginação = %+v, esperava página corrigida e total 1", got)
	}
	if got.Summary.Matches != 1 || got.Summary.Players != 1 {
		t.Fatalf("resumo = %+v", got.Summary)
	}
}

func TestHandleEvolucoesBuscaSemResultadoPreservaFacetas(t *testing.T) {
	srv, _ := newTestServerWithSnapshot(t, fixtureSnapshotComEvolucaoFutGG())
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes?q=mbappe", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[EvolucoesResponse](t, w)
	if got.Total != 0 || len(got.Matches) != 0 {
		t.Fatalf("busca sem resultado = total %d matches %d", got.Total, len(got.Matches))
	}
	if len(got.Filters.Positions) == 0 || len(got.Filters.Categories) == 0 {
		t.Fatalf("facetas desapareceram com a busca vazia: %+v", got.Filters)
	}
}

func TestHandleEvolucoesUsaSomentePathDoFutGGQueContemAEvolucao(t *testing.T) {
	snap := fixtureSnapshot()
	carta := domain.ClubPlayer{Player: domain.Player{
		ID: 22, Name: "Carta Teste", CommonName: "Carta Teste",
		Rating: 90, Position: domain.CM, GGRating: 89.5,
	}}
	evolucao := domain.Evolution{ID: "evo-certa", Name: "Evolução certa"}
	snap.Evolutions = []domain.Evolution{evolucao}
	snap.Club.Players = append(snap.Club.Players, carta)
	snap.EvoMatches = []analyze.EvoMatch{{
		Evolution: evolucao, Player: carta, Slot: domain.CM,
		Before: 80, After: 89, Gain: 9,
		Result: domain.Player{Rating: 92},
	}}
	snap.Cards = append(snap.Cards, cards.CardReport{
		Slug: "carta-teste", Player: carta,
		Best: &cards.EvoPotential{
			Path:          domain.EvolutionPath{Chain: []string{"Outra evolução"}},
			FinalGGRating: 99, GGRatingGain: 9.5,
		},
		Alternates: []cards.EvoPotential{{
			Path:          domain.EvolutionPath{Chain: []string{"Evolução certa"}},
			FinalGGRating: 93, GGRatingGain: 3.5,
		}},
	})
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[EvolucoesResponse](t, w)
	if len(got.Matches) != 1 {
		t.Fatalf("Matches = %+v, esperava uma evolução", got.Matches)
	}
	match := got.Matches[0]
	if match.Impact != 3.5 {
		t.Fatalf("impacto = %.1f, esperava +3.5 do fut.gg", match.Impact)
	}
	if match.BestPath == nil || len(match.BestPath.Path.Chain) != 1 || match.BestPath.Path.Chain[0] != "Evolução certa" {
		t.Fatalf("BestPath = %+v, esperava somente o path da evolução certa", match.BestPath)
	}
	if len(match.Alternates) != 0 {
		t.Fatalf("Alternates = %+v, não deveria incluir path de outra evolução", match.Alternates)
	}
}

func TestHandleEvolucoesDescartaEstimativaQuandoNaoHaPathCorrespondente(t *testing.T) {
	snap := fixtureSnapshot()
	snap.Evolutions = []domain.Evolution{{ID: "sem-path", Name: "Sem path"}}
	snap.EvoMatches[0].Evolution = snap.Evolutions[0]
	snap.EvoMatches[0].Gain = 4.25
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes", nil))

	got := decodeJSON[EvolucoesResponse](t, w)
	if got.Total != 0 || len(got.Matches) != 0 {
		t.Fatalf("estimativa sem path fut.gg entrou no ranking: total=%d matches=%+v", got.Total, got.Matches)
	}
	if len(got.Filters.Positions) != 0 || len(got.Filters.Categories) != 0 {
		t.Fatalf("estimativa sem path fut.gg criou facetas: %+v", got.Filters)
	}
}

func TestHandleEvolucoesDescartaPathSemGanhoDeGGRating(t *testing.T) {
	snap := fixtureSnapshotComEvolucaoFutGG()
	snap.Cards[len(snap.Cards)-1].Best.GGRatingGain = 0
	snap.Cards[len(snap.Cards)-1].Best.FinalGGRating = snap.EvoMatches[0].Player.GGRating
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes", nil))

	got := decodeJSON[EvolucoesResponse](t, w)
	if got.Total != 0 || len(got.Matches) != 0 {
		t.Fatalf("path sem ganho de GG Rating entrou no ranking: %+v", got.Matches)
	}
}

func TestHandleEvolucoesOrdenaPorMultiplosCriteriosNaPrioridadeInformada(t *testing.T) {
	snap := fixtureSnapshot()
	snap.EvoMatches = []analyze.EvoMatch{
		{Player: domain.ClubPlayer{Player: domain.Player{ID: 10, Name: "A", Rating: 90, GGRating: 80}}, Evolution: domain.Evolution{Name: "A"}, Cost: 100},
		{Player: domain.ClubPlayer{Player: domain.Player{ID: 11, Name: "B", Rating: 90, GGRating: 80}}, Evolution: domain.Evolution{Name: "B"}, Cost: 50},
		{Player: domain.ClubPlayer{Player: domain.Player{ID: 12, Name: "C", Rating: 90, GGRating: 80}}, Evolution: domain.Evolution{Name: "C"}, Cost: 0},
	}
	snap.Evolutions = []domain.Evolution{{Name: "A", CoinCost: 100}, {Name: "B", CoinCost: 50}, {Name: "C"}}
	snap.Cards = []cards.CardReport{
		{Player: snap.EvoMatches[0].Player, Best: &cards.EvoPotential{Path: domain.EvolutionPath{Chain: []string{"A"}}, FinalGGRating: 85, GGRatingGain: 5}},
		{Player: snap.EvoMatches[1].Player, Best: &cards.EvoPotential{Path: domain.EvolutionPath{Chain: []string{"B"}}, FinalGGRating: 85, GGRatingGain: 5}},
		{Player: snap.EvoMatches[2].Player, Best: &cards.EvoPotential{Path: domain.EvolutionPath{Chain: []string{"C"}}, FinalGGRating: 83, GGRatingGain: 3}},
	}
	srv, _ := newTestServerWithSnapshot(t, snap)
	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/evolucoes?sort=impact:desc,cost:asc", nil))

	got := decodeJSON[EvolucoesResponse](t, w)
	if len(got.Matches) != 3 {
		t.Fatalf("Matches = %+v, esperava três evoluções", got.Matches)
	}
	ids := []int64{got.Matches[0].Player.ID, got.Matches[1].Player.ID, got.Matches[2].Player.ID}
	want := []int64{11, 10, 12}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ordem = %v, esperava %v", ids, want)
		}
	}
}

func TestHandleJobStatusETrigger(t *testing.T) {
	var triggered bool
	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	srv := &Server{
		Store: st, Cycle: "26",
		Trigger: func() { triggered = true },
		Status:  func() JobStatus { return JobStatus{Running: true} },
	}

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/job", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/job status %d", w.Code)
	}
	if got := decodeJSON[JobStatus](t, w); !got.Running {
		t.Errorf("JobStatus.Running = false, esperava true")
	}

	w = httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/job", nil))
	if w.Code != http.StatusAccepted {
		t.Fatalf("POST /api/job status %d, esperava %d", w.Code, http.StatusAccepted)
	}
	if !triggered {
		t.Error("POST /api/job não chamou Trigger")
	}
}

// fixtureSnapshot só tem 2 cartas e 1 slot titular — bem menos que os 72
// que o Gauntlet exige, então o recompute cai no ramo "escalação não
// sincronizada" (1 slot, não 11). O que este teste prova é que a API
// RECOMPUTA de verdade contra snap.Club (Formation ecoa "4-2-3-1", o valor
// real da fixture) em vez de devolver uma resposta vazia/zero-valor.
func TestHandleGauntletRecomputaPlanoDeSnapshotAntigo(t *testing.T) {
	snap := fixtureSnapshot() // GauntletPlan não setado: Status == "" (sentinela)
	srv, _ := newTestServerWithSnapshot(t, snap)

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/gauntlet", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[GauntletResponse](t, w)
	if got.Formation != "4-2-3-1" {
		t.Fatalf("Formation = %q, esperava 4-2-3-1 (ecoado do club.Squad recomputado)", got.Formation)
	}
	if got.Status != "unavailable" || got.Reason == "" {
		t.Fatalf("status=%q reason=%q — esperava unavailable com motivo (fixture não tem 72 cartas elegíveis)", got.Status, got.Reason)
	}
	if got.Rules == "" {
		t.Error("Rules vazio — a tela precisa da explicação da regra oficial mesmo sem plano completo")
	}
}

// Quando o snapshot já traz um GauntletPlan calculado no momento da coleta,
// a rota deve devolver exatamente ELE — não recalcular por cima. Formation
// usa um valor sentinela que NUNCA sairia de um recompute contra
// snap.Club (que tem Formation "4-2-3-1").
func TestHandleGauntletUsaPlanoJaPersistidoSemRecomputar(t *testing.T) {
	snap := fixtureSnapshot()
	titular := snap.Club.Players[0]
	// Total 17 é um sentinela: nenhum recompute real (11 titulares, teto 33)
	// chegaria nesse número para uma única carta — ele só aparece na
	// resposta se vier do valor PERSISTIDO, sem recalcular.
	quimicaSentinela := chemistry.Resultado{Total: 17, Maximo: 33, Modelo: "sentinela"}
	snap.GauntletPlan = analyze.GauntletPlan{
		Status:    "ok",
		Formation: "sentinela-9-9-9",
		Rounds: []analyze.GauntletSquad{{
			Round: 1,
			Starters: []analyze.GauntletAssignment{
				{Round: 1, Index: 0, Position: domain.CM, Player: titular, Rating: 88.5},
			},
			TotalRating: 88.5, AverageRating: 88.5, Quimica: &quimicaSentinela,
		}},
	}
	srv, _ := newTestServerWithSnapshot(t, snap)

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/gauntlet", nil))
	got := decodeJSON[GauntletResponse](t, w)
	if got.Formation != "sentinela-9-9-9" {
		t.Fatalf("Formation = %q, esperava o valor persistido (a rota recomputou por cima)", got.Formation)
	}
	if len(got.Rounds) != 1 || len(got.Rounds[0].Starters) != 1 {
		t.Fatalf("rounds = %+v, esperava a rodada sentinela intacta", got.Rounds)
	}
	if got.Rounds[0].Quimica == nil || got.Rounds[0].Quimica.Total != 17 {
		t.Errorf("chemistry = %+v, esperava a persistida (total 17)", got.Rounds[0].Quimica)
	}
	if got.Rounds[0].Starters[0].CardSlug != "26-1" {
		t.Errorf("card_slug = %q, esperava cruzar com snap.Cards (26-1)", got.Rounds[0].Starters[0].CardSlug)
	}
}

func evoPotential(pos domain.Position, finalGG float64, cost int, styles ...domain.PlayStyle) cards.EvoPotential {
	return cards.EvoPotential{
		Path:             domain.EvolutionPath{Steps: []domain.Player{{}, {GGRatingPos: pos, GGRating: finalGG}}},
		FinalGGRating:    finalGG,
		CoinsCost:        cost,
		GainedPlayStyles: styles,
	}
}

// Só caminhos cujo Final().GGRatingPos bate com a posição do SLOT titular
// sobrevivem, e duas variações do mesmo PlayStyle ganho colapsam na de
// melhor nota final.
func TestGauntletPotentialsFiltraPorPosicaoEAgrupaPorPlayStyle(t *testing.T) {
	anticipatePlus := domain.PlayStyle{Name: "Anticipate", Plus: true}
	best := evoPotential(domain.CDM, 90.0, 20000, anticipatePlus)
	report := cards.CardReport{
		Best: &best,
		Alternates: []cards.EvoPotential{
			evoPotential(domain.CB, 92.0, 5000, domain.PlayStyle{Name: "Block"}),       // posição errada: descartado
			evoPotential(domain.CDM, 88.0, 10000, anticipatePlus),                      // mesmo playstyle do Best, nota pior: descartado
			evoPotential(domain.CDM, 95.0, 30000, domain.PlayStyle{Name: "Intercept"}), // playstyle diferente: mantido
		},
	}

	out := gauntletPotentials(report, domain.CDM)
	if len(out) != 2 {
		t.Fatalf("gauntletPotentials devolveu %d, esperava 2 (posição errada fora, duplicata de playstyle reduzida): %+v", len(out), out)
	}
	if out[0].FinalGGRating != 95.0 || out[1].FinalGGRating != 90.0 {
		t.Fatalf("ordem/valores = %+v, esperava [95.0, 90.0]", out)
	}
}

// Sem nenhum caminho confirmado para a posição do slot, o potencial fica
// vazio — omitido no JSON (omitempty), nunca uma estimativa inventada.
func TestGauntletPotentialsSemCaminhoConfirmadoDevolveVazio(t *testing.T) {
	if out := gauntletPotentials(cards.CardReport{}, domain.ST); len(out) != 0 {
		t.Fatalf("esperava vazio sem Best/Alternates, veio %+v", out)
	}
	report := cards.CardReport{Alternates: []cards.EvoPotential{evoPotential(domain.CB, 92.0, 5000)}}
	if out := gauntletPotentials(report, domain.ST); len(out) != 0 {
		t.Fatalf("esperava vazio quando nenhum caminho bate a posição do slot, veio %+v", out)
	}
}
