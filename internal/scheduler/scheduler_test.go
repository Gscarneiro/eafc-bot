package scheduler

import (
	"testing"
	"time"
)

func TestNextHojeQuandoOHorarioAindaNaoPassou(t *testing.T) {
	now := time.Date(2026, 8, 21, 3, 0, 0, 0, time.UTC)
	got, err := Next(now, "05:00")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	want := time.Date(2026, 8, 21, 5, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Next(%v) = %v, esperava %v", now, got, want)
	}
}

func TestNextAmanhaQuandoOHorarioJaPassou(t *testing.T) {
	now := time.Date(2026, 8, 21, 6, 0, 0, 0, time.UTC)
	got, err := Next(now, "05:00")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	want := time.Date(2026, 8, 22, 5, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Next(%v) = %v, esperava %v", now, got, want)
	}
}

// Exatamente em cima do horário conta como "já passou" — vira o dia em vez
// de disparar de novo no mesmo instante em que acabou de rodar.
func TestNextViraODiaEmCimaDoHorarioExato(t *testing.T) {
	now := time.Date(2026, 8, 21, 5, 0, 0, 0, time.UTC)
	got, err := Next(now, "05:00")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	want := time.Date(2026, 8, 22, 5, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Next(%v) = %v, esperava %v", now, got, want)
	}
}

func TestNextPreservaOFuso(t *testing.T) {
	loc := time.FixedZone("BRT", -3*60*60)
	now := time.Date(2026, 8, 21, 3, 0, 0, 0, loc)
	got, err := Next(now, "05:00")
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if got.Location() != loc {
		t.Fatalf("Next perdeu o fuso: %v", got.Location())
	}
}

func TestNextErroParaHorarioMalformado(t *testing.T) {
	for _, bad := range []string{"25:00", "05:60", "5h", "", "abc"} {
		if _, err := Next(time.Now(), bad); err == nil {
			t.Errorf("Next com daily_at=%q deveria falhar", bad)
		}
	}
}

func TestShouldRunNowSemSnapshotAnterior(t *testing.T) {
	if !ShouldRunNow(time.Now(), time.Time{}, 20*time.Hour) {
		t.Fatal("sem snapshot nenhum, deveria rodar já")
	}
}

func TestShouldRunNowComSnapshotVelho(t *testing.T) {
	now := time.Date(2026, 8, 21, 5, 0, 0, 0, time.UTC)
	last := now.Add(-21 * time.Hour)
	if !ShouldRunNow(now, last, 20*time.Hour) {
		t.Fatal("snapshot de 21h atrás deveria ser considerado velho (teto de 20h)")
	}
}

func TestShouldRunNowComSnapshotFresco(t *testing.T) {
	now := time.Date(2026, 8, 21, 5, 0, 0, 0, time.UTC)
	last := now.Add(-3 * time.Hour)
	if ShouldRunNow(now, last, 20*time.Hour) {
		t.Fatal("snapshot de 3h atrás não deveria disparar coleta na subida")
	}
}
