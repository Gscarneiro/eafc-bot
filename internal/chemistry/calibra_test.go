package chemistry

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// clubeObservado lê o retrato REAL do XI de 24/08/2026 — 11 titulares, 11
// clubes / 8 ligas / 9 nações distintas, e o entrosamento que o jogo
// reportou. É a anomalia virada em teste de regressão: se um dia alguém
// "consertar" o modelo padrão para a regra clássica, estes testes quebram e
// contam por quê.
func clubeObservado(t *testing.T) domain.Club {
	t.Helper()
	b, err := os.ReadFile("testdata/xi_2026-08-24.json")
	if err != nil {
		t.Fatalf("lendo testdata: %v", err)
	}
	var club domain.Club
	if err := json.Unmarshal(b, &club); err != nil {
		t.Fatalf("decodificando testdata: %v", err)
	}
	if len(club.Squad.Starters) != 11 {
		t.Fatalf("fixture com %d titulares, esperava 11", len(club.Squad.Starters))
	}
	return club
}

func TestModeloPadraoReproduzOXIObservadoEm24Ago2026(t *testing.T) {
	club := clubeObservado(t)
	v := Verificar(ModeloPadrao(), club)

	if v.Status != StatusConfere {
		t.Fatalf("status = %q (%s), esperava confere", v.Status, v.Detalhe)
	}
	if v.Calculado != 33 || v.Observado != 33 {
		t.Fatalf("calculado=%d observado=%d, esperava 33/33", v.Calculado, v.Observado)
	}
	if v.Conferem != 11 || v.Total != 11 {
		t.Fatalf("%d de %d jogadores conferem, esperava 11 de 11", v.Conferem, v.Total)
	}
}

// A regra clássica NÃO descreve este jogo. Travar a diferença é o que
// impede a divergência de virar folclore: se um dia ela sumir, alguém
// precisa reexaminar por quê.
func TestModeloDeVinculosDivergeDoXIObservado(t *testing.T) {
	club := clubeObservado(t)
	v := Verificar(modeloFC26Vinculos, club)

	if v.Status != StatusDiverge {
		t.Fatalf("status = %q, esperava diverge — o XI tem 11 clubes distintos e mesmo assim marca 33", v.Status)
	}
	if v.Calculado >= v.Observado {
		t.Fatalf("calculado=%d observado=%d — a regra de vínculo tem que dar MENOS que o jogo reporta",
			v.Calculado, v.Observado)
	}
}

// A carta no jogo mostra a contribuição de VÍNCULO mesmo quando o efetivo é
// 3 — o Butland aparece com 1 ponto por ser inglês (England x2). Sem expor
// as duas coisas, o número do bot não bate com o que o usuário vê na carta.
func TestJogadorMostraVinculoSeparadoDoEfetivo(t *testing.T) {
	club := clubeObservado(t)
	xi, ok := DoClube(club)
	if !ok {
		t.Fatal("DoClube não montou o XI")
	}
	res := Calcular(ModeloPadrao(), xi)

	var butland *Jogador
	for i := range res.Jogadores {
		p, _ := club.PlayerByID(res.Jogadores[i].PlayerID)
		if p.Nation == "England" && p.Club == "Hull City" {
			butland = &res.Jogadores[i]
		}
	}
	if butland == nil {
		t.Skip("Butland não está no fixture — o XI mudou")
	}
	if butland.Pontos != 3 {
		t.Errorf("efetivo = %d, esperava 3", butland.Pontos)
	}
	if butland.Vinculo != 1 {
		t.Errorf("vínculo = %d, esperava 1 (England x2 no XI, o degrau +1 de nação)", butland.Vinculo)
	}
	if butland.Nacao != 1 || butland.Clube != 0 || butland.Liga != 0 {
		t.Errorf("detalhamento = clube %d / liga %d / nação %d, esperava 0/0/1",
			butland.Clube, butland.Liga, butland.Nacao)
	}
}

// A trava contra autoengano: com um único valor observado na amostra, um
// modelo que devolvesse esse valor fixo passaria com 100%. Relatorio.Confirma
// tem que recusar isso.
func TestCalibrarNaoConfirmaModeloComUmValorDistintoSo(t *testing.T) {
	club := clubeObservado(t)
	r := Calibrar(ModeloPadrao(), []domain.Club{club, club, club})

	if r.Divergem != 0 || r.Conferem != 3 {
		t.Fatalf("placar = %+v, esperava 3 conferem e 0 divergem", r)
	}
	if r.ValoresDistintos != 1 {
		t.Fatalf("valores distintos = %d, esperava 1", r.ValoresDistintos)
	}
	if r.Confirma() {
		t.Fatal("Confirma() devolveu true com um único valor observado — a amostra não tem poder discriminante")
	}
}

// Oráculo ausente é estado PRÓPRIO: não é divergência (não dá para acusar um
// modelo sem ter com o que comparar) nem confirmação.
func TestVerificarSemOraculoNaoEDivergencia(t *testing.T) {
	club := clubeObservado(t)
	club.Squad.ChemistrySynced = false

	v := Verificar(ModeloPadrao(), club)
	if v.Status != StatusSemOraculo {
		t.Fatalf("status = %q, esperava sem_oraculo", v.Status)
	}
	if v.Confiavel() {
		t.Fatal("sem oráculo não pode ser considerado confiável")
	}

	r := Calibrar(ModeloPadrao(), []domain.Club{club})
	if r.SemOraculo != 1 || r.Divergem != 0 || r.Conferem != 0 {
		t.Fatalf("placar = %+v, esperava contar só em SemOraculo", r)
	}
}

// O bug que Fase 0 consertou: química 0 com a rota falhando não pode passar
// por "modelo confere com o jogo, os dois dão zero".
func TestVerificarNaoConfundeOraculoAusenteComQuimicaZero(t *testing.T) {
	club := clubeObservado(t)
	club.Squad.Chemistry = 0
	club.Squad.ChemistrySynced = false

	if v := Verificar(modeloFC26Vinculos, club); v.Status == StatusConfere {
		t.Fatal("modelo passou por \"confere\" contra um oráculo que não existe")
	}
}

// O total é uma soma nossa dos 11 valores por carta, então um modelo errado
// pode acertá-lo por cancelamento de erros. A verificação compara POR
// JOGADOR justamente para pegar isso.
func TestVerificarComparaPorJogadorNaoSoPeloTotal(t *testing.T) {
	club := clubeObservado(t)
	// Mantém o total em 33, mas move um ponto de uma carta para outra: o
	// total continua batendo e mesmo assim isto TEM que reprovar.
	if len(club.Players) < 2 {
		t.Fatal("fixture pequeno demais")
	}
	club.Players[0].Chemistry = 2
	club.Players[1].Chemistry = 4

	v := Verificar(ModeloPadrao(), club)
	if v.Calculado != v.Observado {
		t.Fatalf("pré-condição falhou: o total deveria continuar igual (%d vs %d)", v.Calculado, v.Observado)
	}
	if v.Status == StatusConfere {
		t.Fatal("total igual mascarou dois jogadores errados — a comparação por jogador não pegou")
	}
	if v.Conferem != 9 {
		t.Errorf("%d jogadores conferem, esperava 9 (os outros 2 foram adulterados)", v.Conferem)
	}
}
