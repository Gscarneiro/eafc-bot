package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// clubComUmTitular monta um clube com um único titular no CB — o bastante
// para testar FindUpgrades/UpgradeFunnel sem o ruído de um elenco inteiro.
func clubComUmTitular(titular domain.Player) domain.Club {
	return domain.Club{
		Players: []domain.ClubPlayer{{Player: titular, InSquad: true, SquadSlot: domain.CB}},
		Squad:   domain.Squad{Starters: []domain.SquadSlot{{Position: domain.CB, PlayerID: titular.ID}}},
	}
}

// O bug que gerou este teste: alguém aumenta market.extra_budget esperando
// a lista de sugestões mudar, e ela não muda — porque orçamento NUNCA
// filtrou. DefaultUpgradeOptions liga IncludeUnaffordable, então uma carta
// fora do bolso continua na lista, só marcada Affordable=false.
func TestOrcamentoZeroNaoFiltraSugestao(t *testing.T) {
	titular := mk(84, domain.CB, 80, 40, 66, 70, 84, 82)
	titular.ID = 1
	club := clubComUmTitular(titular)

	melhor := mk(87, domain.CB, 88, 42, 68, 74, 88, 86,
		domain.PlayStyle{Name: "Anticipate", Plus: true})
	melhor.ID = 2
	melhor.Price = domain.Price{Coins: 50_000}

	opt := DefaultUpgradeOptions(0) // sem moeda em caixa, sem extra_budget
	got, _ := FindUpgrades(club, []domain.Player{melhor}, opt)

	if len(got) != 1 {
		t.Fatalf("orçamento zero não deveria remover a sugestão da lista, veio %d", len(got))
	}
	if got[0].Affordable {
		t.Error("com orçamento zero e preço 50k, a sugestão deveria sair marcada como não afordável")
	}
}

func TestUpgradeUsaRecoupLiquidoComTaxaDeCincoPorcento(t *testing.T) {
	titular := mk(84, domain.CB, 80, 40, 66, 70, 84, 82)
	titular.ID = 1
	titular.Price = domain.Price{Coins: 100_001}
	club := clubComUmTitular(titular)

	candidata := mk(90, domain.CB, 90, 50, 75, 80, 90, 90)
	candidata.ID = 2
	candidata.Price = domain.Price{Coins: 100_001}
	got, _ := FindUpgrades(club, []domain.Player{candidata}, DefaultUpgradeOptions(1_000_000))
	if len(got) != 1 {
		t.Fatalf("esperava uma troca, vieram %d", len(got))
	}
	if got[0].Recoup != 95_000 {
		t.Fatalf("Recoup = %d, esperava 95000 líquido", got[0].Recoup)
	}
	if got[0].NetCost != 5_001 {
		t.Fatalf("NetCost = %d, esperava 5001", got[0].NetCost)
	}
}

// Cada carta do mercado deve cair em EXATAMENTE uma gaveta do funil, mesmo
// varrendo vários titulares — sem isso a mesma carta seria contada uma vez
// por slot e a soma nunca bateria Considered.
func TestFunilContaCadaCartaUmaVezSo(t *testing.T) {
	titular := mk(84, domain.CB, 80, 40, 66, 70, 84, 82)
	titular.ID = 1
	club := clubComUmTitular(titular)

	owned := titular // mesma carta que você já tem

	sbcOnly := mk(90, domain.CB, 85, 45, 70, 75, 88, 85)
	sbcOnly.ID = 3
	sbcOnly.Price = domain.Price{IsSBC: true}

	unpriced := mk(90, domain.CB, 85, 45, 70, 75, 88, 85)
	unpriced.ID = 4
	// Price.Coins fica 0 (zero-valor) de propósito, e AllowUnpriced continua
	// desligado (padrão de DefaultUpgradeOptions).

	outOfPosition := mk(99, domain.ST, 99, 99, 90, 95, 40, 90)
	outOfPosition.ID = 5
	outOfPosition.Price = domain.Price{Coins: 40_000}

	belowMinGain := mk(84, domain.CB, 80, 40, 66, 70, 84, 82) // atributos iguais ao titular: gain 0
	belowMinGain.ID = 6
	belowMinGain.Price = domain.Price{Coins: 30_000}

	suggested := mk(90, domain.CB, 88, 45, 70, 76, 90, 87,
		domain.PlayStyle{Name: "Anticipate", Plus: true})
	suggested.ID = 7
	suggested.Price = domain.Price{Coins: 60_000}

	market := []domain.Player{owned, sbcOnly, unpriced, outOfPosition, belowMinGain, suggested}

	opt := DefaultUpgradeOptions(1_000_000)
	_, funnel := FindUpgrades(club, market, opt)

	if funnel.Considered != len(market) {
		t.Fatalf("Considered deveria ser %d, veio %d", len(market), funnel.Considered)
	}
	sum := funnel.Owned + funnel.SBCOnly + funnel.Unpriced +
		funnel.OutOfPosition + funnel.BelowMinGain + funnel.Suggested
	if sum != funnel.Considered {
		t.Fatalf("gavetas do funil deveriam somar Considered (%d), somaram %d: %+v",
			funnel.Considered, sum, funnel)
	}
	if funnel.Owned != 1 || funnel.SBCOnly != 1 || funnel.Unpriced != 1 ||
		funnel.OutOfPosition != 1 || funnel.BelowMinGain != 1 || funnel.Suggested != 1 {
		t.Fatalf("cada gaveta deveria ter exatamente 1 carta, veio %+v", funnel)
	}
}

// Quando nenhuma candidata passa do corte, o funil ainda deve apontar a mais
// próxima — inclusive quando o "mais próximo" é negativo (elenco melhor que
// tudo no mercado), porque é essa distinção que diz se vale a pena baixar
// report.min_gain ou não.
func TestFunilRegistraOMelhorReprovadoPorGanho(t *testing.T) {
	titular := mk(90, domain.CB, 85, 45, 70, 75, 88, 85)
	titular.ID = 1
	club := clubComUmTitular(titular)

	pior := mk(80, domain.CB, 70, 35, 60, 65, 75, 78) // bem pior que o titular
	pior.ID = 2
	pior.Price = domain.Price{Coins: 10_000}

	quaseLa := mk(90, domain.CB, 86, 46, 71, 76, 89, 86) // levemente melhor, mas não o bastante
	quaseLa.ID = 3
	quaseLa.Price = domain.Price{Coins: 20_000}

	opt := DefaultUpgradeOptions(1_000_000)
	opt.MinGain = 5.0 // corte alto o bastante para reprovar as duas

	got, funnel := FindUpgrades(club, []domain.Player{pior, quaseLa}, opt)
	if len(got) != 0 {
		t.Fatalf("nenhuma candidata deveria passar do corte de %.1f, veio %d", opt.MinGain, len(got))
	}
	if !funnel.HasBest {
		t.Fatal("com candidatas reprovadas, HasBest deveria ser true")
	}

	quaseLaGain := Score(quaseLa, domain.CB) - Score(titular, domain.CB)
	if funnel.BestGain != quaseLaGain {
		t.Errorf("BestGain deveria ser o da candidata mais próxima (%.2f), veio %.2f", quaseLaGain, funnel.BestGain)
	}
	if funnel.BestName != quaseLa.Display() {
		t.Errorf("BestName deveria apontar para %q, veio %q", quaseLa.Display(), funnel.BestName)
	}
}

// Uma carta que não joga em nenhum slot titular não pode contaminar
// BestGain — comparar Score() fora de posição não significa nada, porque a
// penalidade de posição já distorce o número (ver TestForaDePosicaoPenaliza).
func TestFunilNaoMedeGanhoForaDePosicao(t *testing.T) {
	titular := mk(84, domain.CB, 80, 40, 66, 70, 84, 82)
	titular.ID = 1
	club := clubComUmTitular(titular)

	// Overall altíssimo, mas atacante puro: nem posição natural nem
	// alternativa em CB. Se o funil medisse Score() fora de posição, esta
	// carta pareceria de longe a melhor reprovada.
	foraDePosicao := mk(99, domain.ST, 99, 99, 90, 95, 40, 90)
	foraDePosicao.ID = 2
	foraDePosicao.Price = domain.Price{Coins: 100_000}

	opt := DefaultUpgradeOptions(1_000_000)
	_, funnel := FindUpgrades(club, []domain.Player{foraDePosicao}, opt)

	if funnel.OutOfPosition != 1 {
		t.Fatalf("carta que não joga em nenhum slot titular deveria cair em OutOfPosition, veio %+v", funnel)
	}
	if funnel.HasBest {
		t.Errorf("sem candidata jogando na posição, BestGain não deveria ser preenchido — "+
			"veio HasBest=true, BestGain=%.2f, BestName=%q", funnel.BestGain, funnel.BestName)
	}
}
