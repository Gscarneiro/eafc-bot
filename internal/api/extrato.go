package api

import (
	"net/http"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/query"
)

// extratoWindow é o "Caixa 7d" de Hoje — a mesma janela que o design pede,
// sem pretender que seja um período fiscal oficial.
const extratoWindow = 7 * 24 * time.Hour

func ledgerSchema() query.Schema[domain.LedgerEntry] {
	return query.NewSchema("capital/extrato", "recorded_at desc", 200,
		text("kind", func(v domain.LedgerEntry) string { return string(v.Kind) }, true, false),
		text("status", func(v domain.LedgerEntry) string { return string(v.Status) }, true, false),
		integer("ea_id", func(v domain.LedgerEntry) int { return int(v.EAID) }),
		integer("gross_coins", func(v domain.LedgerEntry) int { return v.GrossCoins }),
		text("note", func(v domain.LedgerEntry) string { return v.Note }, false, true),
		timeField("occurred_at", func(v domain.LedgerEntry) time.Time { return v.OccurredAt }),
		timeField("recorded_at", func(v domain.LedgerEntry) time.Time { return v.RecordedAt }),
	)
}

// ExtratoResponse é o extrato completo (Value, paginável/filtrável como
// qualquer coleção OData) mais dois resumos prontos: o total e os últimos 7
// dias — a mesma matemática de domain.SummarizeLedger(Since), nunca
// recalculada à mão na tela.
type ExtratoResponse struct {
	query.Page[domain.LedgerEntry]
	Summary   domain.LedgerSummary `json:"@eafc.summary"`
	Summary7d domain.LedgerSummary `json:"@eafc.summary_7d"`
}

func (s *Server) handleCapitalExtrato(w http.ResponseWriter, r *http.Request) {
	entries, err := s.Store.ListLedger(r.Context(), s.Cycle)
	if err != nil {
		http.Error(w, "lendo ledger local: "+err.Error(), http.StatusInternalServerError)
		return
	}
	domain.SortLedgerNewestFirst(entries)
	page, ok := serveList(w, r, ledgerSchema(), entries)
	if !ok {
		return
	}
	writeJSON(w, ExtratoResponse{
		Page:      page,
		Summary:   domain.SummarizeLedger(entries),
		Summary7d: domain.SummarizeLedgerSince(entries, time.Now().Add(-extratoWindow)),
	})
}
