package store

import (
	"context"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestSBCChallengeKeyUsaNomeOuIndiceComoFallback(t *testing.T) {
	if got, want := SBCChallengeKey("sbc-1", 0, "Portugal"), "sbc-1#Portugal"; got != want {
		t.Errorf("SBCChallengeKey com nome = %q, esperava %q", got, want)
	}
	if got, want := SBCChallengeKey("sbc-1", 2, ""), "sbc-1#2"; got != want {
		t.Errorf("SBCChallengeKey sem nome = %q, esperava %q (fallback pro índice)", got, want)
	}
}

// SaveSBCCost só grava ponto pra challenge com custo resolvido — o fut.gg
// às vezes ainda não calculou a solução mais barata, e gravar 0 inventaria
// um "ficou de graça" que não existe.
func TestSaveSBCCostGravaSoChallengeComCustoResolvido(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	ctx := context.Background()

	sbcs := []domain.SBC{
		{ID: "portugal-icons", Challenges: []domain.SBCChallenge{
			{Name: "Portugal", CheapestSolutionCoins: 46900},
			{Name: "SemCusto", CheapestSolutionCoins: 0},
		}},
	}
	if err := st.SaveSBCCost(ctx, "26", sbcs); err != nil {
		t.Fatalf("SaveSBCCost: %v", err)
	}

	var hist sbcCostHistory
	if err := st.readJSON(sbcCostFile("26"), &hist); err != nil {
		t.Fatalf("lendo arquivo gravado: %v", err)
	}
	if len(hist.Points) != 1 {
		t.Fatalf("esperava 1 chave gravada (challenge sem custo não conta), veio %d: %v", len(hist.Points), hist.Points)
	}
	wantKey := SBCChallengeKey("portugal-icons", 0, "Portugal")
	pts, ok := hist.Points[wantKey]
	if !ok || len(pts) != 1 || pts[0].Coins != 46900 {
		t.Fatalf("esperava 1 ponto de 46900 na chave %q, veio ok=%v pts=%+v", wantKey, ok, pts)
	}
}

// O número que sobe é o de demanda esquentando: o fut.gg já resolve o
// fodder mais barato, então um custo de solução em alta significa que a
// faixa exigida (liga/nação/rating) está ficando mais cara de montar.
func TestSBCCostTrendDetectaCustoDeFodderSubindo(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	ctx := context.Background()

	key := SBCChallengeKey("weekend-sbc", 0, "83-Rated Squad")
	now := time.Now()
	seed := sbcCostHistory{Points: map[string][]SBCCostPoint{
		key: {
			{Key: key, Coins: 20000, ObservedAt: now.Add(-2 * time.Hour)},
			{Key: key, Coins: 26000, ObservedAt: now},
		},
	}}
	if err := st.writeJSON(sbcCostFile("26"), seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	trends, err := st.SBCCostTrend(ctx, "26", []string{key}, 24*time.Hour)
	if err != nil {
		t.Fatalf("SBCCostTrend: %v", err)
	}
	tr, ok := trends[key]
	if !ok {
		t.Fatalf("esperava tendência pra %q, veio %v", key, trends)
	}
	if tr.First != 20000 || tr.Last != 26000 || tr.Samples != 2 {
		t.Errorf("First/Last/Samples = %d/%d/%d, esperava 20000/26000/2", tr.First, tr.Last, tr.Samples)
	}
	const wantPct = 30.0 // (26000-20000)/20000*100
	if diff := tr.ChangePct - wantPct; diff < -0.01 || diff > 0.01 {
		t.Errorf("ChangePct = %.2f, esperava %.2f", tr.ChangePct, wantPct)
	}
}

// Um só ponto não vira tendência (mesmo piso de 2 amostras que PriceTrend
// já usa) — sem isso, o primeiro dia que uma SBC aparece já "tendência
// zero, sem mudança", quando na verdade é "ainda não sabemos".
func TestSBCCostTrendExigeDuasAmostras(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	ctx := context.Background()

	key := SBCChallengeKey("weekend-sbc", 0, "83-Rated Squad")
	seed := sbcCostHistory{Points: map[string][]SBCCostPoint{
		key: {{Key: key, Coins: 20000, ObservedAt: time.Now()}},
	}}
	if err := st.writeJSON(sbcCostFile("26"), seed); err != nil {
		t.Fatalf("seed: %v", err)
	}

	trends, err := st.SBCCostTrend(ctx, "26", []string{key}, 24*time.Hour)
	if err != nil {
		t.Fatalf("SBCCostTrend: %v", err)
	}
	if _, ok := trends[key]; ok {
		t.Errorf("com 1 amostra só não deveria haver tendência, veio %+v", trends[key])
	}
}
