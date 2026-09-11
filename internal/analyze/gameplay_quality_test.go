package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestAvaliarQualidadeGameplayUsaSomenteAmostraReservada(t *testing.T) {
	entries := []domain.FeedbackGameplay{
		{Amostra: "avaliacao", Preferencia: "a", Funcao: "Holding"},
		{Amostra: "avaliacao", Preferencia: "b", Funcao: "Holding"},
		{Amostra: "avaliacao", Preferencia: "insuficiente", Funcao: "Holding"},
		{Amostra: "ajuste", Preferencia: "b", Funcao: "Holding"},
	}
	orders := []int{1, 1}
	index := 0
	got := AvaliarQualidadeGameplay(entries, "avaliacao", func(domain.FeedbackGameplay) (int, bool) {
		out := orders[index]
		index++
		return out, true
	})
	if got.Comparacoes != 3 || got.Cobertas != 2 || got.Concordancias != 1 || got.Divergencias != 1 || got.Insuficientes != 1 {
		t.Fatalf("métricas = %+v", got)
	}
	if got.Concordancia != 0.5 {
		t.Fatalf("concordância = %v; queria 0.5", got.Concordancia)
	}
	if len(got.PorFuncao) != 1 || got.PorFuncao[0].Cobertas != 2 {
		t.Fatalf("por função = %+v", got.PorFuncao)
	}
}
