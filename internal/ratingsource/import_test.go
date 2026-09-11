package ratingsource

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestAplicarPreservaFonteERecusaIdentidadeDivergente(t *testing.T) {
	doc := Documento{Fonte: domain.FonteFUTBIN, Metrica: "rating_per_position", EscalaMinima: 0, EscalaMaxima: 100,
		Ciclo: "27", Evidencia: "https://exemplo.test/futbin", Cartas: []Carta{{ID: 10, BasePlayerEAID: 9, Versao: "TOTS", PorPosicao: map[domain.Position]float64{domain.CM: 96}}}}
	club := domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{ID: 10, BasePlayerEaID: 9, Version: "TOTS", Cycle: "27"}}}}
	if result, err := Aplicar(doc, &club, nil); err != nil || result.Aplicadas != 1 {
		t.Fatalf("Aplicar = %+v, %v", result, err)
	}
	if note, ok := club.Players[0].ExternalRatings[domain.FonteFUTBIN].RatingAt(domain.CM, domain.ContextoAvaliacao{Ciclo: "27"}); !ok || note != 96 {
		t.Fatalf("nota importada = %.1f/%v", note, ok)
	}
	club.Players[0].BasePlayerEaID = 8
	if _, err := Aplicar(doc, &club, nil); err == nil {
		t.Fatal("identidade divergente deveria falhar")
	}
}
