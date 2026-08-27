package main

import (
	"context"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func TestMergeCurrentPriceTrendsIncluiCotacaoDaRodada(t *testing.T) {
	trends := map[int64]store.PriceTrend{
		42: {EAID: 42, First: 100000, Last: 110000, Min: 100000, Max: 110000, Samples: 2},
	}
	club := domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{ID: 42, Price: domain.Price{Coins: 120000}}}}}

	mergeCurrentPriceTrends(trends, club, nil)
	got := trends[42]
	if got.Last != 120000 || got.Min != 100000 || got.Max != 120000 || got.Samples != 3 {
		t.Fatalf("cotação atual não entrou na série: %+v", got)
	}
	if got.ChangePct != 20 {
		t.Fatalf("variação deveria ser +20%% desde a primeira amostra, veio %.2f", got.ChangePct)
	}
}

func TestMergeCurrentPriceTrendsIgnoraPrecoDesconhecido(t *testing.T) {
	trends := map[int64]store.PriceTrend{}
	mergeCurrentPriceTrends(trends, domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{ID: 42}}}}, nil)
	if len(trends) != 0 {
		t.Fatalf("preço zero virou tendência inventada: %+v", trends)
	}
}

// A reserva existia no config mas não entrava em nenhuma decisão: Budget
// somava cash+raisable+extra_budget direto, sem descontar market.reserve —
// então configurar uma reserva não mudava NADA na lista de upgrades. Este
// teste prova que o orçamento passado para FindUpgrades agora é
// Capital.Available, que já desconta a reserva.
func TestAnalyzeAndBuildOrcamentoDescontaReserva(t *testing.T) {
	ctx := context.Background()
	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	defer st.Close()

	titular := domain.Player{ID: 1, Rating: 70, Position: domain.CB,
		Attributes: domain.Attributes{Pace: 50, Shooting: 50, Passing: 50, Dribbling: 50, Defending: 50, Physical: 50}}
	candidata := domain.Player{ID: 2, Rating: 90, Position: domain.CB,
		Attributes: domain.Attributes{Pace: 90, Shooting: 90, Passing: 90, Dribbling: 90, Defending: 90, Physical: 90},
		Price:      domain.Price{Coins: 5000}}

	club := domain.Club{
		GamerTag: "BilingualBee", Cycle: "26", Coins: 5000,
		Players: []domain.ClubPlayer{{Player: titular, InSquad: true, SquadSlot: domain.CB}},
		Squad:   domain.Squad{Starters: []domain.SquadSlot{{Position: domain.CB, PlayerID: titular.ID}}},
	}

	build := func(reserve int) analyze.Upgrade {
		cfg := config.Default()
		cfg.GamerTag = club.GamerTag
		cfg.Market.Reserve = reserve
		snap := &futgg.Snapshot{Club: club, Market: []domain.Player{candidata}}
		data, err := analyzeAndBuild(ctx, cfg, st, snap, time.Now(), true, nil, analyze.GauntletPlan{})
		if err != nil {
			t.Fatalf("analyzeAndBuild (reserve=%d): %v", reserve, err)
		}
		if len(data.Upgrades) != 1 {
			t.Fatalf("esperava 1 upgrade sugerido (reserve=%d), veio %d", reserve, len(data.Upgrades))
		}
		return data.Upgrades[0]
	}

	if u := build(0); !u.Affordable {
		t.Fatalf("sem reserva, 5000 em caixa cobre a carta de 5000: esperava Affordable, veio %+v", u)
	}
	if u := build(1000); u.Affordable {
		t.Fatalf("com reserva de 1000, só sobram 4000 pra uma carta de 5000: esperava não Affordable, veio %+v", u)
	}
}

// Um compromisso planejado no ledger é tão real para o orçamento quanto a
// reserva: a coleta seguinte não pode voltar a recomendar gastar essas moedas
// só porque o snapshot anterior foi criado antes do lançamento local.
func TestAnalyzeAndBuildOrcamentoDescontaCompromissoDoLedger(t *testing.T) {
	ctx := context.Background()
	st, err := store.NewJSON(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.AppendLedger(ctx, "26", domain.LedgerEntry{ID: "evo", Kind: domain.LedgerEvolucao, Status: domain.LedgerPlanejado, GrossCoins: 1000}); err != nil {
		t.Fatal(err)
	}
	titular := domain.Player{ID: 1, Rating: 70, Position: domain.CB, Attributes: domain.Attributes{Pace: 50, Shooting: 50, Passing: 50, Dribbling: 50, Defending: 50, Physical: 50}}
	candidata := domain.Player{ID: 2, Rating: 90, Position: domain.CB, Attributes: domain.Attributes{Pace: 90, Shooting: 90, Passing: 90, Dribbling: 90, Defending: 90, Physical: 90}, Price: domain.Price{Coins: 5000}}
	club := domain.Club{GamerTag: "BilingualBee", Cycle: "26", Coins: 5000, Players: []domain.ClubPlayer{{Player: titular, InSquad: true, SquadSlot: domain.CB}}, Squad: domain.Squad{Starters: []domain.SquadSlot{{Position: domain.CB, PlayerID: 1}}}}
	cfg := config.Default()
	cfg.GamerTag = club.GamerTag
	data, err := analyzeAndBuild(ctx, cfg, st, &futgg.Snapshot{Club: club, Market: []domain.Player{candidata}}, time.Now(), true, nil, analyze.GauntletPlan{})
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Upgrades) != 1 || data.Upgrades[0].Affordable {
		t.Fatalf("compromisso deveria deixar a carta fora do orçamento: %+v", data.Upgrades)
	}
}

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
	if _, err := analyzeAndBuild(ctx, cfg, st, bom, time.Now(), false, nil, analyze.GauntletPlan{}); err != nil {
		t.Fatalf("analyzeAndBuild (dia bom): %v", err)
	}

	before, ok, err := st.LatestSnapshot(ctx, cfg.FutGG.Cycle)
	if err != nil || !ok {
		t.Fatalf("esperava snapshot gravado no dia bom: ok=%v err=%v", ok, err)
	}

	vazio := &futgg.Snapshot{Club: domain.Club{GamerTag: cfg.GamerTag, Cycle: cfg.FutGG.Cycle}}
	data, err := analyzeAndBuild(ctx, cfg, st, vazio, time.Now(), false, nil, analyze.GauntletPlan{})
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
