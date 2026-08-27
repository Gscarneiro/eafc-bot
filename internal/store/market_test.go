package store

import (
	"context"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestWatchlistELedgerSaoLocaisPorCicloEAppendOnly(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	entry := domain.WatchlistEntry{ID: "alvo", EAID: 10, Name: "Alvo", TargetCoins: 4_000}
	if err := st.UpsertWatchlist(ctx, "26", entry); err != nil {
		t.Fatal(err)
	}
	if got, err := st.ListWatchlist(ctx, "27"); err != nil || len(got) != 0 {
		t.Fatalf("watchlist de outro ciclo = %+v, %v", got, err)
	}
	if err := st.AppendLedger(ctx, "26", domain.LedgerEntry{ID: "compra", Kind: domain.LedgerCompra, Status: domain.LedgerPlanejado, GrossCoins: 4_000}); err != nil {
		t.Fatal(err)
	}
	if err := st.AppendLedger(ctx, "26", domain.LedgerEntry{ID: "compra", Kind: domain.LedgerCompra, Status: domain.LedgerPlanejado, GrossCoins: 4_000}); err == nil {
		t.Fatal("duplicata do ledger deveria falhar")
	}
	if got, err := st.ListLedger(ctx, "26"); err != nil || len(got) != 1 {
		t.Fatalf("ledger = %+v, %v", got, err)
	}
}
