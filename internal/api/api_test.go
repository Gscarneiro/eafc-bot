package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
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

func newTestServer(t *testing.T) (*Server, store.Store) {
	t.Helper()
	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	if err := st.SaveSnapshot(t.Context(), fixtureSnapshot()); err != nil {
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

func TestHandleTime(t *testing.T) {
	srv, _ := newTestServer(t)
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
	srv, _ := newTestServer(t)
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
