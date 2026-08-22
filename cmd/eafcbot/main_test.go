package main

import (
	"context"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func TestClubeVazioNaoSobrescreveSnapshotBom(t *testing.T) {
	ctx := context.Background()
	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	defer st.Close()

	cfg := config.Default()
	cfg.GamerTag = "BilingualBee"

	bom := &futgg.Snapshot{
		Club: domain.Club{
			GamerTag: cfg.GamerTag, Cycle: cfg.FutGG.Cycle, Coins: 5000,
			Players: []domain.ClubPlayer{{Player: domain.Player{ID: 1, Name: "Titular"}}},
		},
	}
	if _, err := analyzeAndBuild(ctx, cfg, st, bom, time.Now(), false, nil); err != nil {
		t.Fatalf("analyzeAndBuild (dia bom): %v", err)
	}

	before, ok, err := st.LatestSnapshot(ctx, cfg.FutGG.Cycle)
	if err != nil || !ok {
		t.Fatalf("esperava snapshot gravado no dia bom: ok=%v err=%v", ok, err)
	}

	vazio := &futgg.Snapshot{Club: domain.Club{GamerTag: cfg.GamerTag, Cycle: cfg.FutGG.Cycle}}
	data, err := analyzeAndBuild(ctx, cfg, st, vazio, time.Now(), false, nil)
	if err != nil {
		t.Fatalf("analyzeAndBuild (clube vazio): %v", err)
	}

	after, ok, err := st.LatestSnapshot(ctx, cfg.FutGG.Cycle)
	if err != nil || !ok {
		t.Fatalf("snapshot bom sumiu: ok=%v err=%v", ok, err)
	}
	if after.Club.Coins != before.Club.Coins || len(after.Club.Players) == 0 {
		t.Fatalf("clube vazio sobrescreveu o snapshot bom: %+v", after.Club)
	}

	const wantAviso = "snapshot não gravado: clube veio vazio, mantendo o último snapshot bom"
	found := false
	for _, e := range data.Errors {
		if e == wantAviso {
			found = true
		}
	}
	if !found {
		t.Fatalf("esperava o aviso %q em data.Errors, veio %v", wantAviso, data.Errors)
	}
}
