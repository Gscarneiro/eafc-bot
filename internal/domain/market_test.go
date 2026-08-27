package domain

import "testing"

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
