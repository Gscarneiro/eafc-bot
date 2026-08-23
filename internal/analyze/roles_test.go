package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func mk(rating int, pos domain.Position, pac, sho, pas, dri, def, phy int, styles ...domain.PlayStyle) domain.Player {
	return domain.Player{
		Rating: rating, Position: pos,
		Attributes: domain.Attributes{Pace: pac, Shooting: sho, Passing: pas,
			Dribbling: dri, Defending: def, Physical: phy},
		PlayStyles: styles, WeakFoot: 3, SkillMoves: 3,
	}
}

// O overall da EA premia atributos que não aparecem em campo. O bot precisa
// preferir o zagueiro rápido ao zagueiro lento de overall mais alto, senão
// ele recomenda exatamente as cartas que perdem jogo.
func TestZagueiroLentoPerdeParaRapidoComOverallMenor(t *testing.T) {
	lento := mk(89, domain.CB, 62, 40, 70, 68, 91, 88)
	rapido := mk(84, domain.CB, 88, 38, 65, 72, 84, 83)

	sLento, sRapido := Score(lento, domain.CB), Score(rapido, domain.CB)
	if sRapido <= sLento {
		t.Fatalf("zagueiro rápido (84) deveria superar o lento (89): %.1f vs %.1f", sRapido, sLento)
	}
}

// PlayStyle+ é uma das maiores fontes de vantagem real no FC, então dois
// jogadores com atributos idênticos não podem empatar.
func TestPlayStylePlusDesempata(t *testing.T) {
	base := mk(85, domain.RW, 90, 82, 78, 88, 40, 70)
	comPS := mk(85, domain.RW, 90, 82, 78, 88, 40, 70,
		domain.PlayStyle{Name: "Rapid", Plus: true})

	if Score(comPS, domain.RW) <= Score(base, domain.RW) {
		t.Fatal("Rapid+ deveria valer pontos numa ponta")
	}
	// O mesmo PlayStyle não deve valer nada num goleiro.
	gkBase := mk(85, domain.GK, 85, 82, 70, 87, 45, 84)
	gkComPS := mk(85, domain.GK, 85, 82, 70, 87, 45, 84,
		domain.PlayStyle{Name: "Rapid", Plus: true})
	if Score(gkComPS, domain.GK) != Score(gkBase, domain.GK) {
		t.Fatal("Rapid não deveria influenciar a nota de um goleiro")
	}
}

// Score soma os pesos de atributo numa ordem fixa (attrOrder), não com
// `for range` direto no mapa de pesos — Go aleatoriza a ordem de iteração
// de mapa de propósito, e soma de float64 não é associativa. Sem a ordem
// fixa, a mesma carta podia sair com Score() diferente no último bit entre
// chamadas, e isso já vazava pra flakiness real em
// TestPlayStylePlusDesempata. Muitas repetições aumentam a chance de pegar
// a não-determinância se a regressão voltar.
func TestScoreEhDeterministicoEntreChamadas(t *testing.T) {
	p := mk(87, domain.CB, 84, 40, 65, 72, 87, 84, domain.PlayStyle{Name: "Anticipate", Plus: true})
	want := Score(p, domain.CB)
	for i := 0; i < 500; i++ {
		if got := Score(p, domain.CB); got != want {
			t.Fatalf("Score não determinístico na chamada %d: %v (esperava %v)", i, got, want)
		}
	}
}

// A mesma carta rende diferente em funções diferentes: um volante marcador
// não pode pontuar igual como CDM e como ponta.
func TestMesmaCartaRendeDiferentePorFuncao(t *testing.T) {
	volante := mk(86, domain.CDM, 70, 60, 82, 78, 87, 86)
	if Score(volante, domain.CDM) <= Score(volante, domain.RW) {
		t.Fatal("um volante marcador deveria pontuar bem mais como CDM que como RW")
	}
}

// Jogar fora de posição custa: é penalidade explícita no motor.
func TestForaDePosicaoPenaliza(t *testing.T) {
	natural := mk(84, domain.CB, 85, 40, 68, 72, 85, 84)
	natural.AltPositions = []domain.Position{domain.RB}

	semAlt := natural
	semAlt.AltPositions = nil

	if Score(natural, domain.RB) <= Score(semAlt, domain.RB) {
		t.Fatal("quem tem a posição alternativa deveria pontuar acima de quem joga deslocado")
	}
}

func TestEligibilidadeDeEvolucao(t *testing.T) {
	jogador := mk(80, domain.LW, 86, 76, 77, 85, 32, 65)

	cabe := domain.Evolution{Requirements: []domain.EvoRequirement{
		{Kind: "max_overall", IntValue: 81},
		{Kind: "position", Strings: []string{"LW", "RW"}},
	}}
	if !Eligible(jogador, cabe) {
		t.Fatal("jogador 80 LW deveria caber numa evo de max 81 para LW/RW")
	}

	naoCabe := domain.Evolution{Requirements: []domain.EvoRequirement{
		{Kind: "max_overall", IntValue: 79},
	}}
	if Eligible(jogador, naoCabe) {
		t.Fatal("jogador 80 não deveria caber numa evo de overall máximo 79")
	}

	// Requisito que o parser não entendeu tem que bloquear, nunca liberar:
	// sugerir uma evolução impossível é pior que não sugerir nada.
	desconhecido := domain.Evolution{Requirements: []domain.EvoRequirement{
		{Kind: "unknown", Raw: "algum requisito novo que o fut.gg inventou"},
	}}
	if Eligible(jogador, desconhecido) {
		t.Fatal("requisito não reconhecido deveria bloquear a sugestão")
	}
}

// A evolução tem que projetar a carta final corretamente, incluindo o
// teto de 99 nos atributos.
func TestAplicacaoDeEvolucao(t *testing.T) {
	jogador := mk(80, domain.LW, 96, 76, 77, 85, 32, 65)
	evo := domain.Evolution{Levels: []domain.EvoLevel{
		{Upgrades: []domain.EvoUpgrade{
			{Kind: "overall", Amount: 4},
			{Kind: "attribute", Attr: "pac", Amount: 8}, // 96 + 8 deve travar em 99
			{Kind: "playstyle", PlayStyle: domain.PlayStyle{Name: "Rapid", Plus: true}},
			{Kind: "position", Position: domain.ST},
		}},
	}}

	out := evo.Apply(jogador)
	if out.Rating != 84 {
		t.Fatalf("overall deveria virar 84, veio %d", out.Rating)
	}
	if out.Attributes.Pace != 99 {
		t.Fatalf("ritmo deveria travar em 99, veio %d", out.Attributes.Pace)
	}
	if !out.HasPlayStyle("Rapid", true) {
		t.Fatal("deveria ter ganhado Rapid+")
	}
	if !out.PlaysAt(domain.ST) {
		t.Fatal("deveria ter destravado ST")
	}
	// O original não pode ter sido alterado.
	if jogador.Attributes.Pace != 96 || jogador.Rating != 80 {
		t.Fatal("Apply não pode modificar a carta original")
	}
}

// Uma evolução que dá PlayStyle+ sobre um PlayStyle normal já existente
// deve promover, não duplicar.
func TestPlayStylePlusPromoveEmVezDeDuplicar(t *testing.T) {
	jogador := mk(80, domain.LW, 86, 76, 77, 85, 32, 65,
		domain.PlayStyle{Name: "Rapid", Plus: false})
	evo := domain.Evolution{Levels: []domain.EvoLevel{
		{Upgrades: []domain.EvoUpgrade{
			{Kind: "playstyle", PlayStyle: domain.PlayStyle{Name: "Rapid", Plus: true}},
		}},
	}}

	out := evo.Apply(jogador)
	if len(out.PlayStyles) != 1 {
		t.Fatalf("deveria continuar com 1 PlayStyle, veio %d", len(out.PlayStyles))
	}
	if !out.HasPlayStyle("Rapid", true) {
		t.Fatal("Rapid deveria ter sido promovido para Rapid+")
	}
}

// Quando a fonte de dados não entrega cotação (é o caso de coletar pelas
// páginas do fut.gg, onde o preço carrega por JS), "preço zero" reprovaria
// todo candidato e a seção de trocas sairia vazia. Com AllowUnpriced o bot
// ainda sugere, ranqueando por ganho em campo e marcando o custo como
// desconhecido — em vez de inventar um número.
func TestSugereMesmoSemPrecoQuandoPermitido(t *testing.T) {
	titular := mk(84, domain.CB, 80, 40, 66, 70, 84, 82)
	melhor := mk(87, domain.CB, 88, 42, 68, 74, 88, 86,
		domain.PlayStyle{Name: "Anticipate", Plus: true})

	club := domain.Club{
		Coins:   100_000,
		Players: []domain.ClubPlayer{{Player: titular, InSquad: true, SquadSlot: domain.CB}},
	}
	titular.ID, melhor.ID = 1, 2
	club.Players[0].Player.ID = 1
	club.Squad.Starters = []domain.SquadSlot{{Position: domain.CB, PlayerID: 1}}

	semPreco := []domain.Player{melhor} // Price.Coins == 0

	opt := DefaultUpgradeOptions(100_000)
	if got, _ := FindUpgrades(club, semPreco, opt); len(got) != 0 {
		t.Fatalf("sem AllowUnpriced não deveria sugerir nada, veio %d", len(got))
	}

	opt.AllowUnpriced = true
	got, _ := FindUpgrades(club, semPreco, opt)
	if len(got) != 1 {
		t.Fatalf("com AllowUnpriced deveria sugerir 1, veio %d", len(got))
	}
	if !got[0].Unpriced {
		t.Error("a sugestão deveria estar marcada como sem preço")
	}
	if got[0].NetCost != 0 || got[0].Efficiency != 0 {
		t.Errorf("custo e eficiência devem ficar zerados e o relatório mostra \"?\", "+
			"veio custo=%d eficiência=%.2f", got[0].NetCost, got[0].Efficiency)
	}
	if got[0].Gain <= 0 {
		t.Error("o ganho em campo continua sendo calculado normalmente")
	}
}

// Carta com preço deve vir antes da sem preço: dá para raciocinar sobre
// custo numa e não na outra.
func TestPrecificadaVemAntesDaSemPreco(t *testing.T) {
	a := Upgrade{Affordable: true, Unpriced: false, Efficiency: 1.0, Gain: 3}
	b := Upgrade{Affordable: true, Unpriced: true, Gain: 99}
	if !lessUpgrade(a, b) {
		t.Error("a precificada deveria vir primeiro mesmo com ganho menor")
	}
	// Entre duas sem preço, o ganho decide.
	c := Upgrade{Affordable: true, Unpriced: true, Gain: 5}
	d := Upgrade{Affordable: true, Unpriced: true, Gain: 9}
	if !lessUpgrade(d, c) {
		t.Error("entre sem-preço, o maior ganho vem primeiro")
	}
}
