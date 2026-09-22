package galeria

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRewardLegadoEmTextoContinuaLegivel(t *testing.T) {
	var set Set
	if err := json.Unmarshal([]byte(`{"rewards":{"D":["Badge"]}}`), &set); err != nil {
		t.Fatal(err)
	}
	if got := set.Rewards[GradeD][0].Label; got != "Badge" {
		t.Fatalf("rótulo legado = %q", got)
	}
}

func TestEvaluateIgnoraEmprestimoEAlcancaNota(t *testing.T) {
	loan := true
	set := Set{ID: "starter", Name: "Starter", RequiredCards: 2, Thresholds: map[Grade]int{GradeD: 10, GradeC: 20, GradeB: 30, GradeA: 40, GradeS: 50}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: []Card{{ID: 1, Name: "A", ItemScore: 30}, {ID: 2, Name: "B", ItemScore: 30}, {ID: 3, Name: "Loan", ItemScore: 100, Loan: &loan}}, Now: time.Now()})
	if len(got) != 1 || got[0].Evaluation.Score != 60 || got[0].Evaluation.Grade != GradeS {
		t.Fatalf("avaliação = %#v", got)
	}
}

func TestEvaluateNaoNotificaDepoisDeSRegistrado(t *testing.T) {
	now := time.Now()
	got := Evaluate(Input{Sets: []Set{{ID: "x", RequiredCards: 1, Thresholds: map[Grade]int{GradeS: 10}}}, Cards: []Card{{ID: 1, ItemScore: 20}}, Completions: map[string]Completion{"x": {SetID: "x", Grade: GradeS, CompletedAt: now}}, Now: now})
	if len(got) != 1 || got[0].Evaluation.Status != StatusFinal || got[0].Notify() {
		t.Fatalf("S deveria ser final e silencioso: %#v", got)
	}
}

func TestEvaluateAplicaBonusPorTagSobreItensCorrespondentes(t *testing.T) {
	tag := TagRule{Name: "First Owner", Attribute: "FIRST_OWNED", Operator: "COUNT", Values: []string{"1"}, Tiers: []Tier{{MinItems: 2, BonusPercent: 100}}}
	owner := true
	set := Set{ID: "tag", RequiredCards: 2, Rules: []TagRule{tag}, Thresholds: map[Grade]int{GradeD: 1, GradeC: 10, GradeB: 20, GradeA: 30, GradeS: 40}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: []Card{{ID: 1, ItemScore: 20, FirstOwner: &owner}, {ID: 2, ItemScore: 20, FirstOwner: &owner}}, Now: time.Now()})
	if got[0].Evaluation.Score != 80 || got[0].Evaluation.Grade != GradeS {
		t.Fatalf("bonus inesperado: %#v", got[0].Evaluation)
	}
}

func TestEvaluatePrimeiroDonoDesconhecidoNaoRecebeBonus(t *testing.T) {
	owner := false
	set := Set{ID: "owner", RequiredCards: 2, Rules: []TagRule{{Name: "First Owner", Attribute: "FIRST_OWNED", Operator: "COUNT", Values: []string{"1"}, Tiers: []Tier{{MinItems: 2, BonusPercent: 100}}}}, Thresholds: map[Grade]int{GradeD: 1, GradeC: 10, GradeB: 20, GradeA: 30, GradeS: 40}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: []Card{{ID: 1, ItemScore: 20}, {ID: 2, ItemScore: 20, FirstOwner: &owner}}, Now: time.Now()})[0]
	if got.Evaluation.BonusScore != 0 || got.Evaluation.Tags[0].MatchedCount != 0 || got.Evaluation.Tags[0].UnknownCount != 1 {
		t.Fatalf("primeiro dono desconhecido/falso recebeu b\u00f4nus: %#v", got.Evaluation.Tags[0])
	}
}

func TestEvaluateCountDiffUsaMaiorCartaPorGrupo(t *testing.T) {
	tag := TagRule{Attribute: "CLUB", Operator: "COUNT_DIFF", Values: []string{"0"}, Tiers: []Tier{{MinItems: 2, BonusPercent: 100}}}
	set := Set{ID: "diff", RequiredCards: 3, Rules: []TagRule{tag}, Thresholds: map[Grade]int{GradeD: 1, GradeC: 20, GradeB: 40, GradeA: 60, GradeS: 80}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: []Card{{ID: 1, Club: "A", ItemScore: 20}, {ID: 2, Club: "A", ItemScore: 50}, {ID: 3, Club: "B", ItemScore: 20}}, Now: time.Now()})
	if got[0].Evaluation.Score != 160 {
		t.Fatalf("COUNT_DIFF = %d; esperava 159", got[0].Evaluation.Score)
	}
	ids := got[0].Evaluation.Tags[0].MatchedIDs
	if len(ids) != 2 || ids[0] != 2 || ids[1] != 3 {
		t.Fatalf("cartas do COUNT_DIFF = %v; esperava representantes [2 3]", ids)
	}
}

func TestEvaluateMaxCountEscolheGrupoPeloBonus(t *testing.T) {
	rule := TagRule{Name: "mesmo clube", Attribute: "CLUB", Operator: "MAX_COUNT_ALL_SAME", Values: []string{"0"}, Tiers: []Tier{{MinItems: 2, BonusPercent: 10}, {MinItems: 3, BonusPercent: 20}}}
	set := Set{ID: "same", RequiredCards: 3, Rules: []TagRule{rule}, Thresholds: map[Grade]int{GradeD: 1, GradeC: 1, GradeB: 1, GradeA: 1, GradeS: 1}}
	cards := []Card{{ID: 1, ClubID: 1, ItemScore: 10}, {ID: 2, ClubID: 1, ItemScore: 10}, {ID: 3, ClubID: 2, ItemScore: 100}, {ID: 4, ClubID: 2, ItemScore: 100}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: cards, Now: time.Now()})[0]
	// O grupo de duas cartas soma 200 e concede 10%; o grupo de uma não
	// atinge faixa. A escolha maximiza o bônus calculado, não a quantidade.
	if got.Evaluation.BonusScore != 20 || got.Evaluation.Tags[0].MatchedCount != 2 {
		t.Fatalf("MAX_COUNT_ALL_SAME = %#v", got.Evaluation)
	}
}

func TestEvaluateMinCountUsaEstrelasDoCatalogo(t *testing.T) {
	rule := TagRule{Name: "skilled", Attribute: "SKILL_MOVES", Operator: "MIN_COUNT", Values: []string{"4"}, Tiers: []Tier{{MinItems: 2, BonusPercent: 10}}}
	set := Set{ID: "skills", RequiredCards: 2, Rules: []TagRule{rule}, Thresholds: map[Grade]int{GradeD: 1, GradeC: 1, GradeB: 1, GradeA: 1, GradeS: 1}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: []Card{{ID: 1, ItemScore: 10, SkillMoves: 5}, {ID: 2, ItemScore: 10, SkillMoves: 5}}, Now: time.Now()})[0]
	if got.Evaluation.Tags[0].MatchedCount != 2 || got.Evaluation.BonusScore != 2 {
		t.Fatalf("MIN_COUNT = %#v", got.Evaluation)
	}
}

func TestEvaluatePoolTruncadoNaoNotificaAusencia(t *testing.T) {
	set := Set{ID: "truncated", RequiredCards: 1, PoolTruncated: true, Thresholds: map[Grade]int{GradeD: 1}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: []Card{{ID: 1, ItemScore: 100}}, Now: time.Now()})[0]
	if got.Evaluation.Status != StatusUnavailable || got.Notify() {
		t.Fatalf("pool truncado deveria permanecer pendente: %#v", got)
	}
}

func TestEvaluateLimiteAusenteNaoCriaNotaS(t *testing.T) {
	set := Set{ID: "unknown", RequiredCards: 1, Thresholds: map[Grade]int{GradeD: 1, GradeC: 2, GradeB: 3, GradeA: 4}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: []Card{{ID: 1, ItemScore: 100}}, Now: time.Now()})[0]
	if got.Evaluation.Status != StatusUnavailable || got.Evaluation.Grade != GradeNone {
		t.Fatalf("limiar ausente = %#v", got.Evaluation)
	}
}

// Caso sanitizado da tela LALIGA EA SPORTS enviada em 20/09/2026. Mantém os
// Item Scores e o único não-primeiro-dono (Gordon), sem dados do clube.
// A seleção fixa troca Jesus (160) por Pape Gueye (180).
func TestCasoRealLaligaReproduz81454(t *testing.T) {
	owner, bought := true, false
	scores := []int{2100, 830, 830, 830, 830, 410, 410, 410, 410, 410, 340, 340, 340, 340, 340, 340, 340, 340, 280, 280, 280, 280, 280, 280, 280, 280, 280, 180, 180, 180}
	groups := map[string][]int{
		"same-nation":      {0, 1, 2, 3, 4, 5, 18, 27},
		"different-nation": {0, 1, 2, 3, 4, 5, 6, 7, 8, 10, 11, 27, 28},
		"same-club":        {1, 10, 11, 12, 13, 27},
		"different-club":   {0, 1, 2, 3, 4, 5, 6, 10, 11, 18, 27},
		"defensive":        {0, 1, 2, 3, 5, 6, 10, 11},
		"midfield":         {0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 18, 19, 20, 21},
		"attack":           {0, 1, 2, 5, 6, 10, 11},
	}
	cards := make([]Card, len(scores))
	for i, score := range scores {
		cards[i] = Card{ID: int64(i + 1), Name: "Carta", ItemScore: score, FirstOwner: &owner, Positions: []string{"all"}}
	}
	cards[13].FirstOwner = &bought
	for label, indexes := range groups {
		for _, index := range indexes {
			cards[index].Positions = append(cards[index].Positions, label)
		}
	}
	rule := func(name, label string, percent int) TagRule {
		return TagRule{Name: name, Attribute: "POSSIBLE_POSITIONS", Operator: "COUNT_ANY", Values: []string{label}, Tiers: []Tier{{MinItems: 1, BonusPercent: percent}}}
	}
	rules := []TagRule{
		{Name: "First Owner", Attribute: "FIRST_OWNED", Operator: "COUNT", Values: []string{"1"}, Tiers: []Tier{{MinItems: 20, BonusPercent: 500}}},
		rule("Same Nation", "same-nation", 2), rule("Different Nation", "different-nation", 2),
		rule("Same Club", "same-club", 1), rule("Different Club", "different-club", 2),
		rule("Same League", "all", 8), rule("Golden", "all", 4),
		rule("Defensive Wall", "defensive", 6), rule("Midfield Control", "midfield", 10), rule("All out Attack", "attack", 6),
	}
	set := Set{ID: "laliga-real", RequiredCards: 30, Rules: rules, Thresholds: map[Grade]int{GradeD: 10, GradeC: 275000, GradeB: 725000, GradeA: 1900000, GradeS: 4000000}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: cards, Now: time.Now()})[0].Evaluation
	if got.BaseScore != 13250 || got.BonusScore != 68204 || got.Score != 81454 || got.Grade != GradeD {
		t.Fatalf("caso real = base %d, bônus %d, total %d, nota %s", got.BaseScore, got.BonusScore, got.Score, got.Grade)
	}
	if got.Tags[0].MatchedScore != 12910 || got.Tags[0].BonusPoints != 64550 {
		t.Fatalf("First Owner = %#v; esperava subtotal 12.910 e bônus 64.550", got.Tags[0])
	}
}

func TestFloorBonusMantemProdutoExatamenteDivisivelPorCem(t *testing.T) {
	if got := floorBonus(500, 12910); got != 64550 {
		t.Fatalf("500%% de 12.910 = %d; esperava 64.550", got)
	}
}
