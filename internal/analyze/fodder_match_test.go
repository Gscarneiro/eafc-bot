package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func benchCard(id int64, nation, league, club, version string, rating int) domain.ClubPlayer {
	return domain.ClubPlayer{Player: domain.Player{
		ID: id, Nation: nation, League: league, Club: club, Version: version, Rating: rating,
	}}
}

// min_from é o único requisito que FodderMatches afirma pra uma carta
// específica — mesma técnica de poolSize (Nation/League/Club por igual).
func TestFodderMatchesAchaCartaPorNacaoLigaOuClube(t *testing.T) {
	carta := benchCard(1, "Portugal", "", "", "Ouro Raro", 85)
	signal := FodderSignal{
		SBCID: "sbc-1", SBCName: "Seleção", Challenge: "Portugal",
		Requirement: "Min. 1 Players from: Portugal",
		Parsed:      &ParsedSBCRequirement{Kind: "min_from", Value: "Portugal", Min: 1},
	}

	matches := FodderMatches([]domain.ClubPlayer{carta}, []FodderSignal{signal})
	if len(matches[1]) != 1 || matches[1][0].SBCID != "sbc-1" {
		t.Fatalf("matches[1] = %+v, esperava 1 match no sbc-1", matches[1])
	}
}

// min_team_rating (média de SQUAD) e min_rarity_count (risco de abreviação
// de raridade) NÃO podem virar "esta carta cobre" — mesmo quando o valor
// da carta bateria ingenuamente o requisito (rating 90 >= "85"; versão
// "TOTW" == valor "TOTW"), FodderMatches tem que ficar em silêncio, não
// afirmar uma cobertura que poolSize propositalmente marca como -1 (não
// computado) pro mercado.
func TestRequisitoMinFromNaoAfirmaCobertura(t *testing.T) {
	cartaAlta := benchCard(1, "", "", "", "Ouro Raro", 90)
	cartaTOTW := benchCard(2, "", "", "", "TOTW", 80)

	ratingSignal := FodderSignal{
		SBCID: "sbc-rating", Requirement: "Min. Team Rating: 85",
		Parsed: &ParsedSBCRequirement{Kind: "min_team_rating", Value: "85"},
	}
	raritySignal := FodderSignal{
		SBCID: "sbc-rarity", Requirement: "Min. 1 Players: Any TOTW",
		Parsed: &ParsedSBCRequirement{Kind: "min_rarity_count", Value: "TOTW", Min: 1},
	}

	matches := FodderMatches([]domain.ClubPlayer{cartaAlta, cartaTOTW}, []FodderSignal{ratingSignal, raritySignal})
	if len(matches) != 0 {
		t.Fatalf("matches = %+v, esperava nenhum — min_team_rating e min_rarity_count nunca afirmam cobertura de uma carta isolada", matches)
	}
}

// Sem Parsed (o fut.gg mandou um texto que o parser não reconheceu),
// FodderMatches também fica em silêncio — nada a afirmar sobre um requisito
// que nem foi entendido.
func TestFodderMatchesIgnoraSinalSemParsed(t *testing.T) {
	carta := benchCard(1, "Brazil", "", "", "Ouro Raro", 85)
	signal := FodderSignal{SBCID: "sbc-1", Requirement: "algo que o parser não reconheceu", Parsed: nil}

	matches := FodderMatches([]domain.ClubPlayer{carta}, []FodderSignal{signal})
	if len(matches) != 0 {
		t.Fatalf("matches = %+v, esperava nenhum (sem Parsed não há o que comparar)", matches)
	}
}
