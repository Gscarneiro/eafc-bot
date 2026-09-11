package api

import (
	"net/http"
	"sort"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// PosicaoAberta é uma carta do clube com custo de aquisição conhecido
// (ClubStats.PurchasedFor — campo real que o fut.gg às vezes publica, não
// inventado) comparada contra o valor líquido de hoje. NUNCA é P&L
// realizado: o bot nunca toca a conta EA e não lê histórico de transação,
// só o snapshot atual — isto é "quanto custou" contra "quanto vale agora",
// se o custo é conhecido; cartas sem PurchasedFor simplesmente não entram
// na lista, em vez de aparecer com um custo inventado.
type PosicaoAberta struct {
	Player        domain.ClubPlayer `json:"player"`
	CardSlug      string            `json:"card_slug,omitempty"`
	PurchasedFor  int               `json:"purchased_for"`
	CurrentValue  int               `json:"current_value"`
	UnrealizedPnL int               `json:"unrealized_pnl"`
}

// PosicoesResponse junta as posições com custo conhecido com o que já está
// planejado no ledger (LedgerSummary.Committed) — as duas únicas fontes
// honestas de "quanto está comprometido" que o bot tem (ver restrição de
// honestidade sobre "posições abertas" no plano do redesenho).
type PosicoesResponse struct {
	Posicoes  []PosicaoAberta      `json:"posicoes"`
	Planejado []domain.LedgerEntry `json:"planejado"`
	Committed int                  `json:"committed"`
}

func (s *Server) handleCapitalPosicoes(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	lookup := newCardSlugLookup(snap.Cards)
	posicoes := make([]PosicaoAberta, 0)
	for _, p := range snap.Club.Players {
		if p.Stats == nil || p.Stats.PurchasedFor == nil || *p.Stats.PurchasedFor <= 0 {
			continue
		}
		current := p.NetSellValue()
		posicoes = append(posicoes, PosicaoAberta{
			Player: p, CardSlug: lookup.slug(p),
			PurchasedFor: *p.Stats.PurchasedFor, CurrentValue: current,
			UnrealizedPnL: current - *p.Stats.PurchasedFor,
		})
	}
	sort.Slice(posicoes, func(i, j int) bool { return posicoes[i].UnrealizedPnL < posicoes[j].UnrealizedPnL })

	ledger, err := s.Store.ListLedger(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo ledger local: "+err.Error(), http.StatusInternalServerError)
		return
	}
	planejado := make([]domain.LedgerEntry, 0)
	for _, e := range ledger {
		if e.Status == domain.LedgerPlanejado {
			planejado = append(planejado, e)
		}
	}
	writeJSON(w, PosicoesResponse{Posicoes: posicoes, Planejado: planejado, Committed: domain.SummarizeLedger(ledger).Committed})
}
