package domain

import (
	"testing"
	"time"
)

// O "Caixa 7d" de Hoje só pode contar o que aconteceu na janela — um
// lançamento de 10 dias atrás não é resultado desta semana, mesmo que
// tenha sido REGISTRADO (RecordedAt) hoje.
func TestSummarizeLedgerSinceIgnoraLancamentoForaDaJanela(t *testing.T) {
	agora := time.Date(2026, 8, 31, 12, 0, 0, 0, time.UTC)
	sete := agora.Add(-7 * 24 * time.Hour)
	entries := []LedgerEntry{
		{ID: "dentro", Kind: LedgerVenda, Status: LedgerConfirmado, GrossCoins: 20_000, OccurredAt: agora.Add(-2 * 24 * time.Hour)},
		{ID: "fora", Kind: LedgerCompra, Status: LedgerConfirmado, GrossCoins: 50_000, OccurredAt: agora.Add(-10 * 24 * time.Hour)},
	}
	got := SummarizeLedgerSince(entries, sete)
	wantNet := 20_000 * 95 / 100
	if got.NetCash != wantNet || got.PnL != wantNet {
		t.Fatalf("resumo de 7d = %+v, esperava só a venda dentro da janela (líquido %d) — a compra de 10 dias atrás vazou pra dentro", got, wantNet)
	}
	if got.Spent != 0 {
		t.Fatalf("Spent = %d, esperava 0 — a compra de fora da janela não pode aparecer como gasto desta semana", got.Spent)
	}

	// Sem filtro nenhum, o mesmo lançamento "fora" apareceria — prova de que
	// o teste está testando a janela, não uma diferença de matemática.
	semJanela := SummarizeLedger(entries)
	if semJanela.Spent == 0 {
		t.Fatal("fixture não prova nada: sem janela a compra 'fora' deveria aparecer em Spent")
	}
}

func TestSummarizeLedgerAplicaTaxaCompromissoEReversao(t *testing.T) {
	entries := []LedgerEntry{
		{ID: "compra", Kind: LedgerCompra, Status: LedgerConfirmado, GrossCoins: 10_000},
		{ID: "venda", Kind: LedgerVenda, Status: LedgerConfirmado, GrossCoins: 20_001},
		{ID: "evo", Kind: LedgerEvolucao, Status: LedgerPlanejado, GrossCoins: 3_000},
		{ID: "cancelada", Kind: LedgerSBC, Status: LedgerPlanejado, GrossCoins: 2_000},
		{ID: "reversao", Kind: LedgerReversao, Status: LedgerConfirmado, ReversesID: "cancelada"},
	}
	got := SummarizeLedger(entries)
	if got.RaisedNet != 19_000 || got.NetCash != 9_000 || got.PnL != 9_000 {
		t.Fatalf("resumo de caixa/P&L = %+v, esperava venda líquida de 19000 menos compra", got)
	}
	if got.Committed != 3_000 {
		t.Fatalf("Committed = %d, esperava só evolução ativa de 3000", got.Committed)
	}
}

func TestLedgerReversaoExigeLancamentoAlvo(t *testing.T) {
	err := (LedgerEntry{ID: "r", Kind: LedgerReversao, Status: LedgerConfirmado}).Validate()
	if err == nil {
		t.Fatal("reversão sem reverses_id deveria falhar")
	}
}

func TestBreakEvenGrossArredondaParaCimaDepoisDaTaxa(t *testing.T) {
	if got := BreakEvenGross(10_001); got != 10_528 {
		t.Fatalf("BreakEvenGross = %d, esperava 10528", got)
	}
}
