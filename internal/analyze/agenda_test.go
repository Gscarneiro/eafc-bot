package analyze

import (
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"testing"
	"time"
)

func TestMontarAgendaAgrupaSemRecalcular(t *testing.T) {
	now := time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC)
	got := MontarAgenda(AgendaInput{Agora: now, Mercado: MarketPlan{Actions: []MarketAction{{Kind: MarketBuy, Name: "Compra", NetCost: 100, Confidence: "alta"}}}, SBCs: []domain.SBC{{ID: "s", Name: "SBC", ExpiresAt: now.Add(48 * time.Hour)}}, Watchlist: []domain.WatchlistEntry{{ID: "w", Name: "Alvo"}}})
	if len(got.Agora) != 2 || len(got.Observando) != 1 {
		t.Fatalf("agenda=%+v", got)
	}
	if got.Agora[0].ID == "" || got.Agora[0].Proveniencia == "" {
		t.Fatal("acao sem rastreabilidade")
	}
}
