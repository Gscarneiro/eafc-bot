package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// O "Caixa 7d" (@eafc.summary_7d) tem que ficar restrito à janela, enquanto
// @eafc.summary continua somando TUDO — as duas contas convivem na mesma
// resposta e não podem se confundir.
func TestExtratoSeparaResultadoDeSeteDiasDoTotal(t *testing.T) {
	srv, st := newTestServer(t)
	agora := time.Now()

	if err := st.AppendLedger(t.Context(), "26", domain.LedgerEntry{
		ID: "venda-recente", Kind: domain.LedgerVenda, Status: domain.LedgerConfirmado,
		GrossCoins: 20_000, OccurredAt: agora.Add(-2 * 24 * time.Hour), RecordedAt: agora,
	}); err != nil {
		t.Fatalf("AppendLedger (recente): %v", err)
	}
	if err := st.AppendLedger(t.Context(), "26", domain.LedgerEntry{
		ID: "compra-antiga", Kind: domain.LedgerCompra, Status: domain.LedgerConfirmado,
		GrossCoins: 50_000, OccurredAt: agora.Add(-30 * 24 * time.Hour), RecordedAt: agora,
	}); err != nil {
		t.Fatalf("AppendLedger (antiga): %v", err)
	}

	w := httptest.NewRecorder()
	srv.Handler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/capital/extrato", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d %s", w.Code, w.Body.String())
	}
	got := decodeJSON[ExtratoResponse](t, w)

	if len(got.Value) != 2 {
		t.Fatalf("Value = %d lançamentos, esperava 2 (o extrato completo lista tudo)", len(got.Value))
	}
	wantVendaLiquida := 20_000 * 95 / 100
	if got.Summary7d.NetCash != wantVendaLiquida {
		t.Errorf("Summary7d.NetCash = %d, esperava %d (só a venda recente)", got.Summary7d.NetCash, wantVendaLiquida)
	}
	if got.Summary7d.Spent != 0 {
		t.Errorf("Summary7d.Spent = %d, esperava 0 — a compra de 30 dias atrás não é desta semana", got.Summary7d.Spent)
	}
	wantTotalNet := wantVendaLiquida - 50_000
	if got.Summary.NetCash != wantTotalNet {
		t.Errorf("Summary.NetCash = %d, esperava %d (venda líquida menos a compra antiga — o total soma tudo)", got.Summary.NetCash, wantTotalNet)
	}
}
