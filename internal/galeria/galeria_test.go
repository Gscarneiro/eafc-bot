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
	if got[0].Evaluation.Score != 79 || got[0].Evaluation.Grade != GradeS {
		t.Fatalf("bonus inesperado: %#v", got[0].Evaluation)
	}
}

func TestEvaluatePrimeiroDonoDesconhecidoNaoRecebeBonus(t *testing.T) {
	owner := false
	set := Set{ID: "owner", RequiredCards: 2, Rules: []TagRule{{Name: "First Owner", Attribute: "FIRST_OWNED", Operator: "COUNT", Values: []string{"1"}, Tiers: []Tier{{MinItems: 2, BonusPercent: 100}}}}, Thresholds: map[Grade]int{GradeD: 1, GradeC: 10, GradeB: 20, GradeA: 30, GradeS: 40}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: []Card{{ID: 1, ItemScore: 20}, {ID: 2, ItemScore: 20, FirstOwner: &owner}}, Now: time.Now()})[0]
	if got.Evaluation.BonusScore != 0 || got.Evaluation.Tags[0].MatchedCount != 0 {
		t.Fatalf("primeiro dono desconhecido/falso recebeu b\u00f4nus: %#v", got.Evaluation.Tags[0])
	}
}

func TestEvaluateCountDiffUsaMaiorCartaPorGrupo(t *testing.T) {
	tag := TagRule{Attribute: "CLUB", Operator: "COUNT_DIFF", Values: []string{"0"}, Tiers: []Tier{{MinItems: 2, BonusPercent: 100}}}
	set := Set{ID: "diff", RequiredCards: 3, Rules: []TagRule{tag}, Thresholds: map[Grade]int{GradeD: 1, GradeC: 20, GradeB: 40, GradeA: 60, GradeS: 80}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: []Card{{ID: 1, Club: "A", ItemScore: 20}, {ID: 2, Club: "A", ItemScore: 50}, {ID: 3, Club: "B", ItemScore: 20}}, Now: time.Now()})
	if got[0].Evaluation.Score != 159 {
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
	if got.Evaluation.BonusScore != 19 || got.Evaluation.Tags[0].MatchedCount != 2 {
		t.Fatalf("MAX_COUNT_ALL_SAME = %#v", got.Evaluation)
	}
}

func TestEvaluateMinCountUsaEstrelasDoCatalogo(t *testing.T) {
	rule := TagRule{Name: "skilled", Attribute: "SKILL_MOVES", Operator: "MIN_COUNT", Values: []string{"4"}, Tiers: []Tier{{MinItems: 2, BonusPercent: 10}}}
	set := Set{ID: "skills", RequiredCards: 2, Rules: []TagRule{rule}, Thresholds: map[Grade]int{GradeD: 1, GradeC: 1, GradeB: 1, GradeA: 1, GradeS: 1}}
	got := Evaluate(Input{Sets: []Set{set}, Cards: []Card{{ID: 1, ItemScore: 10, SkillMoves: 5}, {ID: 2, ItemScore: 10, SkillMoves: 5}}, Now: time.Now()})[0]
	if got.Evaluation.Tags[0].MatchedCount != 2 || got.Evaluation.BonusScore != 1 {
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
