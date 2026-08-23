package analyze

import (
	"testing"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestParseRequisitoDeNacao(t *testing.T) {
	p, ok := parseSBCRequirementLine("Min. 1 Players from: Portugal")
	if !ok {
		t.Fatal("esperava reconhecer o padrão \"Min. N Players from: X\"")
	}
	if p.Kind != "min_from" || p.Min != 1 || p.Value != "Portugal" {
		t.Errorf("parse = %+v, esperava {min_from 1 Portugal}", p)
	}
}

func TestParseRequisitoDeRatingDeSquad(t *testing.T) {
	p, ok := parseSBCRequirementLine("Min. Team Rating: 89")
	if !ok {
		t.Fatal("esperava reconhecer o padrão \"Min. Team Rating: N\"")
	}
	if p.Kind != "min_team_rating" || p.Value != "89" {
		t.Errorf("parse = %+v, esperava {min_team_rating 89}", p)
	}
}

func TestParseRequisitoDeContagemDeRaridade(t *testing.T) {
	p, ok := parseSBCRequirementLine("Min. 1 Players: Any TOTW/TOTS/FOF")
	if !ok {
		t.Fatal("esperava reconhecer o padrão \"Min. N Players: Any X\"")
	}
	if p.Kind != "min_rarity_count" || p.Min != 1 || p.Value != "TOTW/TOTS/FOF" {
		t.Errorf("parse = %+v, esperava {min_rarity_count 1 TOTW/TOTS/FOF}", p)
	}
}

// Requisito que o parser não reconhece cai como texto cru, sem virar
// ParsedSBCRequirement nenhum — mesmo espírito de "na dúvida, não afirma"
// que internal/futgg/map.go já segue pra evolução.
func TestRequisitoNaoReconhecidoNaoViraParsed(t *testing.T) {
	_, ok := parseSBCRequirementLine("Squad must contain at least 1 Icon")
	if ok {
		t.Fatal("frase fora dos 3 padrões conhecidos não deveria ser reconhecida")
	}
}

// PoolSize só é computado pra min_from (comparação direta de texto) — os
// outros dois tipos reconhecidos não têm uma comparação confiável
// carta-a-carta, então ficam em -1 (não computado), não em 0.
func TestPoolSizeSoComputaParaMinFrom(t *testing.T) {
	market := []domain.Player{{Nation: "Portugal"}, {Nation: "Brazil"}, {Nation: "Portugal"}}

	got := poolSize(ParsedSBCRequirement{Kind: "min_from", Value: "Portugal"}, market)
	if got != 2 {
		t.Errorf("poolSize(min_from Portugal) = %d, esperava 2", got)
	}

	if got := poolSize(ParsedSBCRequirement{Kind: "min_team_rating", Value: "89"}, market); got != -1 {
		t.Errorf("poolSize(min_team_rating) deveria ser -1 (não computado), veio %d", got)
	}
	if got := poolSize(ParsedSBCRequirement{Kind: "min_rarity_count", Value: "TOTW"}, market); got != -1 {
		t.Errorf("poolSize(min_rarity_count) deveria ser -1 (não computado), veio %d", got)
	}
}

func sbcComChallenge(id string, expiresAt time.Time, repeatable bool, coins int) domain.SBC {
	return domain.SBC{
		ID: id, Name: id, ExpiresAt: expiresAt, Repeatable: repeatable,
		Challenges: []domain.SBCChallenge{{Name: "Desafio", CheapestSolutionCoins: coins}},
	}
}

// Achado central da pesquisa de mercado: a demanda pica no LANÇAMENTO do
// SBC, não perto da expiração — perto de expirar é hora de ESVAZIAR
// fodder, não comprar mais.
func TestFaseEsvaziarPertoDaExpiracao(t *testing.T) {
	sbcs := []domain.SBC{sbcComChallenge("sbc-1", time.Now().Add(24*time.Hour), false, 20000)}

	got := FindFodderDemand(sbcs, nil, nil, DefaultFodderDemandOptions())
	if len(got) != 1 || got[0].Phase != PhaseDump {
		t.Fatalf("SBC expirando em 24h deveria estar em fase %q, veio %+v", PhaseDump, got)
	}
}

// Custo subindo forte é o PICO de demanda (logo após lançamento) — não é
// sinal de compra, é o oposto: momento mais caro pra comprar fodder.
func TestFasePicoQuandoCustoSobeForteNaoEUmSinalDeCompra(t *testing.T) {
	sbcs := []domain.SBC{sbcComChallenge("sbc-1", time.Now().Add(30*24*time.Hour), false, 26000)}
	trends := map[string]CostTrend{
		challengeKey("sbc-1", 0, "Desafio"): {ChangePct: 30, Samples: 3},
	}

	got := FindFodderDemand(sbcs, nil, trends, DefaultFodderDemandOptions())
	if len(got) != 1 || got[0].Phase != PhasePeak {
		t.Fatalf("custo subindo 30%% deveria estar em fase %q, veio %+v", PhasePeak, got)
	}
	for _, r := range got[0].Rationale {
		if r == "custo da solução mais barata subindo forte — provável pico de demanda logo após o lançamento, não é hora de comprar fodder" {
			return
		}
	}
	t.Errorf("rationale deveria deixar claro que pico não é sinal de compra, veio %v", got[0].Rationale)
}

func TestFaseEsfriandoQuandoCustoCai(t *testing.T) {
	sbcs := []domain.SBC{sbcComChallenge("sbc-1", time.Now().Add(30*24*time.Hour), false, 15000)}
	trends := map[string]CostTrend{
		challengeKey("sbc-1", 0, "Desafio"): {ChangePct: -25, Samples: 3},
	}

	got := FindFodderDemand(sbcs, nil, trends, DefaultFodderDemandOptions())
	if len(got) != 1 || got[0].Phase != PhaseCooling {
		t.Fatalf("custo caindo 25%% deveria estar em fase %q, veio %+v", PhaseCooling, got)
	}
}

// Menos de 2 amostras não é tendência — é cedo demais pra dizer se está
// subindo ou descendo (mesmo piso que store.PriceTrend usa).
func TestFaseRecenteSemTendenciaSuficiente(t *testing.T) {
	sbcs := []domain.SBC{sbcComChallenge("sbc-1", time.Now().Add(30*24*time.Hour), false, 15000)}
	trends := map[string]CostTrend{
		challengeKey("sbc-1", 0, "Desafio"): {ChangePct: 40, Samples: 1}, // só 1 amostra
	}

	got := FindFodderDemand(sbcs, nil, trends, DefaultFodderDemandOptions())
	if len(got) != 1 || got[0].Phase != PhaseRecent {
		t.Fatalf("com 1 amostra só, deveria estar em fase %q, veio %+v", PhaseRecent, got)
	}
}

// Challenge sem custo resolvido pelo fut.gg ainda não vira sinal — 0 não
// pode ser confundido com "de graça".
func TestChallengeSemCustoResolvidoNaoViraSinal(t *testing.T) {
	sbcs := []domain.SBC{sbcComChallenge("sbc-1", time.Now().Add(30*24*time.Hour), false, 0)}

	got := FindFodderDemand(sbcs, nil, nil, DefaultFodderDemandOptions())
	if len(got) != 0 {
		t.Fatalf("challenge sem custo resolvido não deveria virar sinal, veio %d", len(got))
	}
}

// A ordenação prioriza o que pede ação: esvaziar (venda antes do crash) e
// pico (não compre) antes de estável.
func TestOrdenaEsvaziarEPicoAntesDeEstavel(t *testing.T) {
	sbcs := []domain.SBC{
		sbcComChallenge("estavel", time.Now().Add(30*24*time.Hour), false, 15000),
		sbcComChallenge("esvaziar", time.Now().Add(24*time.Hour), false, 15000),
	}
	trends := map[string]CostTrend{
		challengeKey("estavel", 0, "Desafio"): {ChangePct: 1, Samples: 3},
	}

	got := FindFodderDemand(sbcs, nil, trends, DefaultFodderDemandOptions())
	if len(got) != 2 {
		t.Fatalf("esperava 2 sinais, veio %d", len(got))
	}
	if got[0].SBCID != "esvaziar" {
		t.Errorf("esvaziar deveria vir primeiro, veio %+v", got[0])
	}
}
