package api

import (
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"testing"
)

func TestRecommendationFallbackEIdentificadaComoBot(t *testing.T) {
	cp := domain.ClubPlayer{Player: domain.Player{Position: domain.ST, PlayStyles: []domain.PlayStyle{{Name: "Finesse Shot", Plus: true}}}}
	r := recommendationFor(cp, domain.ST, nil, nil)
	if r.Source != "bot" {
		t.Fatalf("fonte = %q, esperava bot", r.Source)
	}
	if len(r.Styles) == 0 || r.Styles[0].Name != "Finesse Shot" || !r.Styles[0].Plus {
		t.Fatalf("fallback não preservou PlayStyle+ esperado: %+v", r.Styles)
	}
}
