package analyze

import (
	"strings"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// FodderMatch é "esta carta específica cobre este requisito de SBC" — mais
// forte que FodderSignal, que fala do mercado inteiro, nunca de uma carta
// sua. Só existe para Kind=="min_from" (nação/liga/clube), pela MESMA
// técnica que poolSize já usa pro mercado (comparação direta contra
// Nation/League/Club). Os outros dois tipos de ParsedSBCRequirement NÃO
// entram: min_team_rating é média de SQUAD — nenhuma carta isolada
// "cobre" uma média, por melhor que seja — e min_rarity_count esbarra no
// mesmo risco de falso positivo por abreviação de raridade que o
// comentário de poolSize já documenta (ex.: "TOTW" vs o nome completo).
// Afirmar cobertura nesses dois seria o palpite silencioso que CLAUDE.md
// pede pra evitar — ficam de fora, não "-1", porque aqui não há contagem
// nenhuma pra mostrar, só a ausência da afirmação.
type FodderMatch struct {
	SBCID       string `json:"sbc_id"`
	SBCName     string `json:"sbc_name"`
	Challenge   string `json:"challenge"`
	Requirement string `json:"requirement"`
}

// FodderMatches cruza os sinais já resolvidos por FindFodderDemand contra
// as cartas do BANCO, devolvendo por carta (chave: Player.ID) os SBCs cujo
// requisito min_from ela bate de verdade. Uma carta pode aparecer em mais
// de um SBC; a chamada não toca rede — signals já vem pronto do snapshot.
func FodderMatches(bench []domain.ClubPlayer, signals []FodderSignal) map[int64][]FodderMatch {
	out := make(map[int64][]FodderMatch)
	for _, sig := range signals {
		if sig.Parsed == nil || sig.Parsed.Kind != "min_from" {
			continue
		}
		for _, card := range bench {
			if !matchesMinFrom(*sig.Parsed, card.Player) {
				continue
			}
			out[card.ID] = append(out[card.ID], FodderMatch{
				SBCID: sig.SBCID, SBCName: sig.SBCName, Challenge: sig.Challenge, Requirement: sig.Requirement,
			})
		}
	}
	return out
}

func matchesMinFrom(req ParsedSBCRequirement, p domain.Player) bool {
	return strings.EqualFold(p.Nation, req.Value) ||
		strings.EqualFold(p.League, req.Value) ||
		strings.EqualFold(p.Club, req.Value)
}
