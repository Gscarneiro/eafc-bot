package store

import (
	"context"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// SaveMomentum é cache do último valor, não série — cada chamada
// SUBSTITUI a anterior (ao contrário de SavePrices, que acumula).
func TestSaveMomentumSubstituiOValorAnterior(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}
	ctx := context.Background()

	if err := st.SaveMomentum(ctx, "26", []domain.Player{{ID: 1, MomentumPct: 10}}); err != nil {
		t.Fatalf("SaveMomentum (1): %v", err)
	}
	if err := st.SaveMomentum(ctx, "26", []domain.Player{{ID: 2, MomentumPct: 20}}); err != nil {
		t.Fatalf("SaveMomentum (2): %v", err)
	}

	got, err := st.LatestMomentum(ctx, "26")
	if err != nil {
		t.Fatalf("LatestMomentum: %v", err)
	}
	if len(got) != 1 || got[0].ID != 2 || got[0].MomentumPct != 20 {
		t.Fatalf("esperava só a segunda gravação (id=2, 20%%), veio %+v", got)
	}
}

// Ciclo rápido ainda não ter rodado (run/demo sem serve no ar) não é
// erro — é o estado normal antes da primeira coleta rápida.
func TestLatestMomentumSemGravacaoNaoEErro(t *testing.T) {
	st, err := NewJSON(t.TempDir())
	if err != nil {
		t.Fatalf("NewJSON: %v", err)
	}

	got, err := st.LatestMomentum(context.Background(), "26")
	if err != nil {
		t.Fatalf("LatestMomentum não deveria falhar sem gravação nenhuma: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("esperava vazio, veio %+v", got)
	}
}
