package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestPlanoMercadoIncluiCompromissoDoLedgerEWatchlist(t *testing.T) {
	snap := fixtureSnapshot()
	snap.Market = []domain.Player{{ID: 3, Name: "Alvo", Position: domain.CM, Price: domain.Price{Coins: 20_000, UpdatedAt: time.Now()}}}
	snap.Upgrades = []analyze.Upgrade{{Slot: domain.CM, Candidate: snap.Market[0], GrossCost: 20_000, NetCost: 20_000}}
	srv, st := newTestServerWithSnapshot(t, snap)
	if err := st.AppendLedger(t.Context(), "26", domain.LedgerEntry{ID: "evo-planejada", Kind: domain.LedgerEvolucao, Status: domain.LedgerPlanejado, GrossCoins: 40_000}); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertWatchlist(t.Context(), "26", domain.WatchlistEntry{ID: "w1", EAID: 3, Name: "Alvo", Protected: true}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/planos/mercado", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body.String())
	}
	got := decodeJSON[MarketPlanResponse](t, w)
	if got.LedgerSummary.Committed != 40_000 || got.Plan.Capital.Available != 10_000 {
		t.Fatalf("capital/compromisso = %+v / %+v", got.Plan.Capital, got.LedgerSummary)
	}
	if len(got.Watchlist) != 1 || got.Watchlist[0].ID != "w1" {
		t.Fatalf("watchlist = %+v", got.Watchlist)
	}
	if len(got.Plan.Actions) == 0 || got.Plan.Actions[0].Kind != analyze.MarketWait {
		t.Fatalf("ações = %+v, esperava aguardar por capital", got.Plan.Actions)
	}
}

func TestRecursosLocaisDeWatchlistELedgerValidamCorpo(t *testing.T) {
	srv, st := newTestServer(t)
	defer st.Close()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/watchlist", bytes.NewBufferString(`{"ea_id":77,"name":"Teste","target_coins":5000}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("watchlist status %d: %s", w.Code, w.Body.String())
	}
	watch := decodeJSON[domain.WatchlistEntry](t, w)
	if watch.ID == "" {
		t.Fatal("API deveria gerar id local para a watchlist")
	}

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/ledger", bytes.NewBufferString(`{"kind":"compra","status":"confirmado","gross_coins":10000}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("ledger status %d: %s", w.Code, w.Body.String())
	}
	entry := decodeJSON[domain.LedgerEntry](t, w)
	if entry.ID == "" || entry.RecordedAt.IsZero() {
		t.Fatalf("ledger sem identidade/data: %+v", entry)
	}

	if err := st.AppendLedger(t.Context(), "26", domain.LedgerEntry{ID: "r", Kind: domain.LedgerReversao, Status: domain.LedgerConfirmado}); err == nil {
		t.Fatal("store aceitou reversão sem lançamento alvo")
	}
}
