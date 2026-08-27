package store

import (
	"context"
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestSnapshotRoundTrip(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	ctx := context.Background()

	generated := time.Date(2026, 8, 20, 5, 0, 0, 0, time.UTC)
	snap := Snapshot{
		GeneratedAt: generated,
		Cycle:       "26",
		Club: domain.Club{
			GamerTag: "BilingualBee", Cycle: "26", Coins: 12345,
			Players: []domain.ClubPlayer{{Player: domain.Player{ID: 1, Name: "Jogador"}}},
		},
		SquadScore: 87.5,
	}
	if err := st.SaveSnapshot(ctx, snap); err != nil {
		t.Fatalf("SaveSnapshot: %v", err)
	}

	got, ok, err := st.LatestSnapshot(ctx, "26")
	if err != nil || !ok {
		t.Fatalf("LatestSnapshot: ok=%v err=%v", ok, err)
	}
	if got.Club.Coins != 12345 || got.SquadScore != 87.5 || len(got.Club.Players) != 1 {
		t.Fatalf("snapshot lido não bate com o gravado: %+v", got)
	}

	hist, err := st.SnapshotHistory(ctx, "26", 30)
	if err != nil {
		t.Fatalf("SnapshotHistory: %v", err)
	}
	if len(hist) != 1 || hist[0].Date != "2026-08-20" || hist[0].Coins != 12345 {
		t.Fatalf("histórico inesperado: %+v", hist)
	}

	// Ciclo sem snapshot nenhum não é erro, só "não achei".
	if _, ok, err := st.LatestSnapshot(ctx, "27"); err != nil || ok {
		t.Fatalf("esperava ok=false para ciclo sem dado, veio ok=%v err=%v", ok, err)
	}
}

func TestSaveSnapshotPodaAposTrintaDias(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)

	const totalDias = 35
	for i := 0; i < totalDias; i++ {
		day := base.AddDate(0, 0, i)
		snap := Snapshot{GeneratedAt: day, Cycle: "26", SquadScore: float64(i)}
		if err := st.SaveSnapshot(ctx, snap); err != nil {
			t.Fatalf("SaveSnapshot dia %d: %v", i, err)
		}
	}

	hist, err := st.SnapshotHistory(ctx, "26", 60) // pede mais que o teto de retenção
	if err != nil {
		t.Fatalf("SnapshotHistory: %v", err)
	}
	if len(hist) != snapshotRetention {
		t.Fatalf("esperava %d dias após a poda, veio %d", snapshotRetention, len(hist))
	}
	wantOldest := base.AddDate(0, 0, totalDias-snapshotRetention).Format("2006-01-02")
	if hist[0].Date != wantOldest {
		t.Fatalf("esperava o dia mais antigo remanescente ser %s, veio %s", wantOldest, hist[0].Date)
	}

	latest, ok, err := st.LatestSnapshot(ctx, "26")
	if err != nil || !ok {
		t.Fatalf("LatestSnapshot: ok=%v err=%v", ok, err)
	}
	if latest.SquadScore != float64(totalDias-1) {
		t.Fatalf("esperava o snapshot mais recente (dia %d), veio SquadScore=%v", totalDias-1, latest.SquadScore)
	}
}

func TestSaveSnapshotRespeitaRetencaoConfigurada(t *testing.T) {
	st, err := NewJSONWithRetention(t.TempDir(), 3)
	if err != nil {
		t.Fatalf("NewJSONWithRetention: %v", err)
	}
	ctx := context.Background()
	base := time.Date(2026, 2, 1, 5, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		if err := st.SaveSnapshot(ctx, Snapshot{GeneratedAt: base.AddDate(0, 0, i), Cycle: "26"}); err != nil {
			t.Fatalf("SaveSnapshot dia %d: %v", i, err)
		}
	}
	hist, err := st.SnapshotHistory(ctx, "26", 0)
	if err != nil {
		t.Fatalf("SnapshotHistory: %v", err)
	}
	if len(hist) != 3 || hist[0].Date != "2026-02-03" {
		t.Fatalf("retenção configurada ignorada: %+v", hist)
	}
}

func TestClubRollupNaoSeguePodaDoSnapshotCompleto(t *testing.T) {
	st, err := NewJSONWithRetention(t.TempDir(), 2)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	base := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		day := base.AddDate(0, 0, i)
		if err := st.SaveSnapshot(ctx, Snapshot{GeneratedAt: day, Cycle: "26"}); err != nil {
			t.Fatal(err)
		}
		if err := st.SaveClubRollup(ctx, "26", domain.ClubRollup{ObservedAt: day, Entries: []domain.ClubRollupEntry{{EAID: 1, Count: 1}}}); err != nil {
			t.Fatal(err)
		}
	}
	rollups, err := st.ClubRollups(ctx, "26", 0)
	if err != nil || len(rollups) != 4 {
		t.Fatalf("rollups deveriam sobreviver à poda de snapshots: %d, %v", len(rollups), err)
	}
}

func TestDiffClubsPreservaCartasDuplicadas(t *testing.T) {
	mesmaCarta := func(item string) domain.ClubPlayer {
		return domain.ClubPlayer{Player: domain.Player{ID: 42, Name: "Carta repetida"}, ClubItemID: item}
	}
	prev := domain.Club{Players: []domain.ClubPlayer{mesmaCarta("a"), mesmaCarta("b")}}
	cur := domain.Club{Players: []domain.ClubPlayer{mesmaCarta("a"), mesmaCarta("c")}}

	d := DiffClubs(prev, cur)
	if len(d.Added) != 1 || d.Added[0].ClubItemID != "c" {
		t.Fatalf("carta adicionada foi colapsada ou errada: %+v", d.Added)
	}
	if len(d.Removed) != 1 || d.Removed[0].ClubItemID != "b" {
		t.Fatalf("carta removida foi colapsada ou errada: %+v", d.Removed)
	}
}

func TestDiffClubsUsaMulticonjuntoQuandoFonteNaoDaItemID(t *testing.T) {
	card := func(nome string) domain.ClubPlayer {
		return domain.ClubPlayer{Player: domain.Player{ID: 42, Name: nome}}
	}
	prev := domain.Club{Players: []domain.ClubPlayer{card("A"), card("B")}}
	cur := domain.Club{Players: []domain.ClubPlayer{card("A")}}

	d := DiffClubs(prev, cur)
	if len(d.Added) != 0 || len(d.Removed) != 1 {
		t.Fatalf("a contagem de duplicatas não foi preservada: %+v", d)
	}
}

func TestPriceSeriesDevolveSerieDentroDaJanela(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	ctx := context.Background()

	if err := st.SavePrices(ctx, "26", []domain.Player{{ID: 42, Price: domain.Price{Coins: 1000}}}); err != nil {
		t.Fatalf("SavePrices: %v", err)
	}

	series, err := st.PriceSeries(ctx, "26", []int64{42}, 24*time.Hour)
	if err != nil {
		t.Fatalf("PriceSeries: %v", err)
	}
	pts, ok := series[42]
	if !ok || len(pts) != 1 || pts[0].Coins != 1000 {
		t.Fatalf("série inesperada: %+v", series)
	}

	// Um ponto de 10 dias atrás, gravado direto (SavePrices sempre usa o
	// relógio real, e uma janela de tempo real curta demais é frágil no
	// Windows — a resolução do relógio pode fazer duas chamadas de
	// time.Now() caírem no mesmo tick) fica de fora de uma janela de 24h.
	old := priceHistory{Points: map[string][]PricePoint{
		"999": {{EAID: 999, Coins: 500, ObservedAt: time.Now().Add(-10 * 24 * time.Hour)}},
	}}
	if err := st.writeJSON(pricesFile("26"), old); err != nil {
		t.Fatalf("gravando fixture: %v", err)
	}

	empty, err := st.PriceSeries(ctx, "26", []int64{999}, 24*time.Hour)
	if err != nil {
		t.Fatalf("PriceSeries (janela de 24h): %v", err)
	}
	if len(empty[999]) != 0 {
		t.Fatalf("esperava série vazia fora da janela, veio %+v", empty[999])
	}
}
