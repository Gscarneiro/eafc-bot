package main

import (
	"context"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func TestRestoreLastSuccessLêSnapshotMaisRecente(t *testing.T) {
	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	want := time.Date(2026, 8, 25, 5, 0, 0, 0, time.UTC)
	if err := st.SaveSnapshot(context.Background(), store.Snapshot{
		GeneratedAt: want,
		Cycle:       "26",
		Club:        domain.Club{Cycle: "26", Players: []domain.ClubPlayer{{Player: domain.Player{ID: 1}}}},
	}); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}
	got := restoreLastSuccess(context.Background(), st, "26")
	if got == nil || !got.Equal(want) {
		t.Fatalf("LastSuccess = %v, esperava %v", got, want)
	}
}

func TestRestoreLastSuccessSemSnapshotDevolveNil(t *testing.T) {
	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	if got := restoreLastSuccess(context.Background(), st, "26"); got != nil {
		t.Fatalf("sem snapshot, LastSuccess = %v, esperava nil", got)
	}
}

func TestListenHostFicaEmLoopbackPorPadrao(t *testing.T) {
	t.Setenv("EAFC_LISTEN_HOST", "")
	if got := listenHost(); got != "127.0.0.1" {
		t.Fatalf("host padrão = %q, esperava loopback", got)
	}
	t.Setenv("EAFC_LISTEN_HOST", "0.0.0.0")
	if got := listenHost(); got != "0.0.0.0" {
		t.Fatalf("host explícito = %q, esperava 0.0.0.0", got)
	}
}
