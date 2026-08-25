package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// clubComBancoEUmTitular monta um clube com um titular (CB) e o resto dos
// jogadores passados como banco — o bastante pra testar FindSellCandidates
// sem o ruído de um elenco inteiro.
func clubComBancoEUmTitular(titularID int64, banco ...domain.ClubPlayer) domain.Club {
	titular := domain.ClubPlayer{Player: domain.Player{ID: titularID, Position: domain.CB}}
	players := append([]domain.ClubPlayer{titular}, banco...)
	return domain.Club{
		Players: players,
		Squad:   domain.Squad{Starters: []domain.SquadSlot{{Position: domain.CB, PlayerID: titularID}}},
	}
}

// Untradeable nunca pode virar sugestão de venda — SellValue()/NetSellValue()
// já
// devolve 0 pra essas, e recomendar a venda do que não pode ser vendido
// é pior que não dizer nada (pesquisa de mercado).
func TestNuncaSugereVenderCartaUntradeable(t *testing.T) {
	banco := domain.ClubPlayer{
		Player:      domain.Player{ID: 2, Rating: 84, Price: domain.Price{Coins: 50000}},
		Untradeable: true,
	}
	club := clubComBancoEUmTitular(1, banco)

	got, funnel := FindSellCandidates(club, nil, nil, DefaultSellOptions())
	if len(got) != 1 || got[0].Recommendation != "nao_vendavel" {
		t.Fatalf("esperava 1 candidato \"nao_vendavel\", veio %+v", got)
	}
	if got[0].NetSellValue != 0 {
		t.Errorf("NetSellValue de carta untradeable deveria ser 0, veio %d", got[0].NetSellValue)
	}
	if funnel.NotTradeable != 1 {
		t.Errorf("funnel.NotTradeable = %d, esperava 1", funnel.NotTradeable)
	}
}

// Se o banco já bate um titular (FindSquadSwaps), a recomendação certa é
// promover, não vender — vender jogaria fora um upgrade de custo zero.
func TestPromoveQuandoBancoJaBateTitular(t *testing.T) {
	banco := domain.ClubPlayer{
		Player: domain.Player{ID: 2, Position: domain.CB, Rating: 90, Price: domain.Price{Coins: 50000}},
	}
	club := clubComBancoEUmTitular(1, banco)
	swaps := []SquadSwap{{Slot: domain.CB, Current: club.Players[0], Candidate: banco, GGRatingGap: 5}}

	got, funnel := FindSellCandidates(club, nil, swaps, DefaultSellOptions())
	if len(got) != 1 || got[0].Recommendation != "promover" {
		t.Fatalf("esperava 1 candidato \"promover\", veio %+v", got)
	}
	if funnel.Promotable != 1 {
		t.Errorf("funnel.Promotable = %d, esperava 1", funnel.Promotable)
	}
}

// cards.CardReport.Best é uma projeção REAL do fut.gg (não fórmula
// própria) — ganho suficiente justifica segurar em vez de vender.
func TestSeguraQuandoEvolucaoTemGanhoSuficiente(t *testing.T) {
	bancoPlayer := domain.Player{ID: 2, Rating: 84, Price: domain.Price{Coins: 50000}}
	club := clubComBancoEUmTitular(1, domain.ClubPlayer{Player: bancoPlayer})
	reports := []cards.CardReport{
		{Player: domain.ClubPlayer{Player: bancoPlayer}, Best: &cards.EvoPotential{GGRatingGain: 3.5, CoinsCost: 20000}},
	}

	got, funnel := FindSellCandidates(club, reports, nil, DefaultSellOptions())
	if len(got) != 1 || got[0].Recommendation != "segurar_potencial" {
		t.Fatalf("esperava 1 candidato \"segurar_potencial\", veio %+v", got)
	}
	if got[0].EvoGGGain != 3.5 || got[0].EvoCost != 20000 {
		t.Errorf("EvoGGGain/EvoCost = %.1f/%d, esperava 3.5/20000", got[0].EvoGGGain, got[0].EvoCost)
	}
	if funnel.HeldForPotential != 1 {
		t.Errorf("funnel.HeldForPotential = %d, esperava 1", funnel.HeldForPotential)
	}
}

// Ganho pequeno demais (abaixo do piso) não trava moeda parada — vende.
func TestVendeQuandoGanhoDeEvolucaoEhPequenoDemais(t *testing.T) {
	bancoPlayer := domain.Player{ID: 2, Rating: 84, Price: domain.Price{Coins: 50000}}
	club := clubComBancoEUmTitular(1, domain.ClubPlayer{Player: bancoPlayer})
	reports := []cards.CardReport{
		{Player: domain.ClubPlayer{Player: bancoPlayer}, Best: &cards.EvoPotential{GGRatingGain: 0.5}},
	}
	opt := DefaultSellOptions() // MinEvoGGGain: 2.0

	got, funnel := FindSellCandidates(club, reports, nil, opt)
	if len(got) != 1 || got[0].Recommendation != "vender" {
		t.Fatalf("ganho de 0.5 (piso 2.0) deveria vender, veio %+v", got)
	}
	if funnel.Suggested != 1 {
		t.Errorf("funnel.Suggested = %d, esperava 1", funnel.Suggested)
	}
}

// Carta abaixo do piso de overall analisado (sem CardReport) não tem
// evidência de potencial — vende, mas a razão diz que faltou análise em
// vez de fingir que a ausência de dado é uma resposta.
func TestVendeSemAnaliseQuandoNaoHaCardReport(t *testing.T) {
	banco := domain.ClubPlayer{Player: domain.Player{ID: 2, Rating: 75, Price: domain.Price{Coins: 5000}}}
	club := clubComBancoEUmTitular(1, banco)

	got, _ := FindSellCandidates(club, nil, nil, DefaultSellOptions())
	if len(got) != 1 || got[0].Recommendation != "vender" {
		t.Fatalf("esperava 1 candidato \"vender\", veio %+v", got)
	}
	found := false
	for _, r := range got[0].Rationale {
		if r == "sem análise de evolução disponível (abaixo do piso de overall analisado — ver serve.cards_min_rating)" {
			found = true
		}
	}
	if !found {
		t.Errorf("rationale deveria avisar que não há CardReport, veio %v", got[0].Rationale)
	}
}

func TestNaoVendeQuandoVerificacaoDeEvolucaoFalha(t *testing.T) {
	banco := domain.ClubPlayer{Player: domain.Player{ID: 2, Rating: 90, Price: domain.Price{Coins: 50000}}}
	club := clubComBancoEUmTitular(1, banco)
	reports := []cards.CardReport{{
		Player:          banco,
		EvolutionStatus: cards.EvolutionFetchError,
		EvolutionError:  "falha simulada",
	}}

	got, funnel := FindSellCandidates(club, reports, nil, DefaultSellOptions())
	if len(got) != 1 || got[0].Recommendation != "aguardar_verificacao" {
		t.Fatalf("falha de verificação deveria bloquear venda: %+v", got)
	}
	if funnel.WaitingVerification != 1 || got[0].NetSellValue != 0 {
		t.Fatalf("funnel/capital incorretos: funnel=%+v candidato=%+v", funnel, got[0])
	}
}

// O titular nunca aparece na lista — FindSellCandidates é só sobre o
// banco.
func TestApenasBancoEntraNaAnalise(t *testing.T) {
	club := clubComBancoEUmTitular(1) // sem banco nenhum
	got, funnel := FindSellCandidates(club, nil, nil, DefaultSellOptions())
	if len(got) != 0 || funnel.Considered != 0 {
		t.Fatalf("sem banco, não deveria haver candidato nenhum, veio %+v / considered=%d", got, funnel.Considered)
	}
}

// NetSellValue desconta a taxa de venda de 5% da EA — a mesma conta usada
// pelo orçamento e por Upgrade.Recoup.
func TestNetSellValueDescontaTaxaDeVendaDe5Porcento(t *testing.T) {
	banco := domain.ClubPlayer{Player: domain.Player{ID: 2, Rating: 84, Price: domain.Price{Coins: 100000}}}
	club := clubComBancoEUmTitular(1, banco)

	got, _ := FindSellCandidates(club, nil, nil, DefaultSellOptions())
	if len(got) != 1 {
		t.Fatalf("esperava 1 candidato, veio %d", len(got))
	}
	if got[0].NetSellValue != 95000 {
		t.Errorf("NetSellValue = %d, esperava 95000 (100000 * 0.95)", got[0].NetSellValue)
	}
}
