package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

type avaliadorPorFuncaoTeste struct{}

func (avaliadorPorFuncaoTeste) Perfis() []domain.PerfilMeta { return nil }

func (avaliadorPorFuncaoTeste) Avaliar(card domain.Player, pos domain.Position, ctx domain.ContextoAvaliacao) domain.AvaliacaoCarta {
	notas := map[string]map[int64]float64{
		"volante": {1: 80, 2: 70, 3: 92},
		"criador": {1: 65, 2: 95, 3: 75},
	}
	nota, ok := notas[ctx.Funcao][card.ID]
	return domain.AvaliacaoCarta{Disponivel: ok, Nota: nota, Contexto: ctx}
}

func starterCP(id int64, p domain.Player) domain.ClubPlayer {
	p.ID = id
	return domain.ClubPlayer{Player: p}
}

// Reproduz o caso relatado: um titular com Score() interno alto (goleiro,
// bem cotado pelos pesos deste bot) não pode ganhar de um titular com GG
// Rating baixo (Gnabry, 92.4 contra 95+ do resto do time) na disputa por
// "elo mais fraco". Quem decide isso agora é o número do fut.gg, não a
// fórmula própria — só a fórmula própria decidia antes, e por isso o bot
// apontava o goleiro quando o fut.gg mesmo aponta outro jogador.
func TestElofMaisFracoUsaGGRatingMesmoContraScoreAlto(t *testing.T) {
	goleiro := mk(98, domain.GK, 85, 85, 85, 85, 85, 85)
	goleiro.GGRating = 95.0
	atacante := mk(98, domain.LM, 95, 92, 98, 99, 61, 86)
	atacante.GGRating = 92.4 // o "Gnabry" do caso real

	club := domain.Club{
		Players: []domain.ClubPlayer{
			starterCP(1, goleiro),
			starterCP(2, atacante),
		},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 0, Position: domain.GK, PlayerID: 1},
			{Index: 1, Position: domain.LM, PlayerID: 2},
		}},
	}

	weak := WeakestLinks(club, 1)
	if len(weak) == 0 {
		t.Fatal("WeakestLinks não devolveu nada")
	}
	if weak[0].Player.ID != 2 {
		t.Fatalf("elo mais fraco = jogador %d, esperava o 2 (menor GG Rating): "+
			"quando os dois têm GG Rating, é ele quem decide, não o Score() interno",
			weak[0].Player.ID)
	}
}

// Sem nota comparável em algum titular, a carta fica fora do ranking. Trocar
// silenciosamente para Score() misturaria duas escalas no mesmo elo fraco.
func TestElofMaisFracoNaoMisturaAvaliadorSemGGRatingCompleto(t *testing.T) {
	comGG := mk(90, domain.CB, 80, 40, 70, 70, 88, 82)
	comGG.GGRating = 95.0
	semGG := mk(90, domain.ST, 60, 60, 60, 60, 40, 60) // ruim em campo, sem GG Rating

	club := domain.Club{
		Players: []domain.ClubPlayer{starterCP(1, comGG), starterCP(2, semGG)},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 0, Position: domain.CB, PlayerID: 1},
			{Index: 1, Position: domain.ST, PlayerID: 2},
		}},
	}
	weak := WeakestLinks(club, 1)
	if len(weak) != 1 || weak[0].Player.ID != 1 {
		t.Fatalf("esperava comparar apenas o jogador com nota GG, veio %+v", weak)
	}
}

// FindSquadSwaps varre o BANCO, não o mercado: um reserva na mesma posição
// com GG Rating maior é uma troca de custo zero.
func TestFindSquadSwapsAchaReservaComGGRatingMaior(t *testing.T) {
	titular := mk(85, domain.LM, 80, 70, 75, 78, 40, 75)
	titular.GGRating = 92.4
	titular.GGRatingPos = domain.LM
	titular.GGRatings = map[domain.Position]float64{domain.LM: 92.4}
	reserva := mk(84, domain.LM, 82, 72, 77, 80, 42, 76)
	reserva.GGRating = 96.1
	reserva.GGRatingPos = domain.LM
	reserva.GGRatings = map[domain.Position]float64{domain.LM: 96.1}
	outroReserva := mk(80, domain.ST, 90, 88, 60, 82, 30, 70) // posição errada
	outroReserva.GGRating = 99.0

	club := domain.Club{
		Players: []domain.ClubPlayer{
			starterCP(1, titular),
			starterCP(2, reserva),
			starterCP(3, outroReserva),
		},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 0, Position: domain.LM, PlayerID: 1},
		}},
	}

	swaps := FindSquadSwaps(club)
	if len(swaps) != 1 {
		t.Fatalf("achou %d trocas, esperava 1 (o reserva de posição errada não conta)", len(swaps))
	}
	if swaps[0].Candidate.ID != 2 {
		t.Errorf("candidato = %d, esperava 2", swaps[0].Candidate.ID)
	}
	wantGap := 96.1 - 92.4
	if swaps[0].GGRatingGap < wantGap-0.01 || swaps[0].GGRatingGap > wantGap+0.01 {
		t.Errorf("gap = %.2f, esperava %.2f", swaps[0].GGRatingGap, wantGap)
	}
}

// A nota GG geral da carta pode ser a nota da sua MELHOR posição, que não é
// necessariamente a vaga que ela disputa no XI. Promover neste caso usando o
// número geral diz ao usuário que há ganho quando a nota publicada para aquela
// vaga é menor que a do titular.
func TestFindSquadSwapsComparaGGDaVagaFisica(t *testing.T) {
	titular := mk(90, domain.CAM, 80, 85, 90, 92, 50, 70)
	titular.GGRating = 99.0
	titular.GGRatingPos = domain.CAM
	titular.GGRatings = map[domain.Position]float64{domain.CAM: 99.1}

	reserva := mk(90, domain.CAM, 90, 90, 88, 92, 40, 75)
	reserva.AltPositions = []domain.Position{domain.ST}
	reserva.GGRating = 99.5 // melhor nota da carta, em ST
	reserva.GGRatingPos = domain.ST
	reserva.GGRatings = map[domain.Position]float64{
		domain.CAM: 98.8,
		domain.ST:  99.5,
	}

	club := domain.Club{
		Players: []domain.ClubPlayer{starterCP(1, titular), starterCP(2, reserva)},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 6, Position: domain.CAM, PlayerID: 1},
		}},
	}

	if swaps := FindSquadSwaps(club); len(swaps) != 0 {
		t.Fatalf("sugeriu troca na CAM apesar de 98,8 < 99,1 nessa vaga: %+v", swaps)
	}
}

// Uma formação repete posição (dois CB) — sem o índice do slot físico, a
// UI não sabe QUAL dos dois zagueiros a troca sugerida é sobre (ver
// CLAUDE.md, "SquadSlot é lugar físico, não posição lógica"). Só um dos
// dois titulares tem reserva melhor; o SquadSwap resultante tem que carregar
// o Index do slot 3 (o titular fraco), não o do slot 4 (o outro CB, que
// nenhum reserva supera) nem um índice genérico/zero.
func TestSquadSwapGuardaOSlotFisicoComDoisZagueiros(t *testing.T) {
	cb1 := mk(85, domain.CB, 80, 40, 70, 70, 83, 78) // o zagueiro fraco
	cb1.GGRating = 83.0
	cb1.GGRatingPos = domain.CB
	cb1.GGRatings = map[domain.Position]float64{domain.CB: 83.0}
	cb2 := mk(85, domain.CB, 80, 40, 70, 70, 95, 90) // ninguém no banco supera este
	cb2.GGRating = 95.0
	cb2.GGRatingPos = domain.CB
	cb2.GGRatings = map[domain.Position]float64{domain.CB: 95.0}
	reservaCB := mk(84, domain.CB, 80, 40, 70, 70, 88, 82)
	reservaCB.GGRating = 87.0
	reservaCB.GGRatingPos = domain.CB
	reservaCB.GGRatings = map[domain.Position]float64{domain.CB: 87.0}

	club := domain.Club{
		Players: []domain.ClubPlayer{
			starterCP(1, cb1), starterCP(2, cb2), starterCP(3, reservaCB),
		},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 3, Position: domain.CB, PlayerID: 1},
			{Index: 4, Position: domain.CB, PlayerID: 2},
		}},
	}

	swaps := FindSquadSwaps(club)
	if len(swaps) != 1 {
		t.Fatalf("achou %d trocas, esperava 1 (só o zagueiro fraco tem reserva melhor): %+v", len(swaps), swaps)
	}
	got := swaps[0]
	if got.Index != 3 {
		t.Errorf("Index = %d, esperava 3 (o slot do titular 1, não do titular 2 nem zero)", got.Index)
	}
	if got.Current.ID != 1 || got.Candidate.ID != 3 {
		t.Errorf("troca = %+v, esperava titular 1 -> candidato 3", got)
	}
}

func TestSquadSwapRespeitaFuncaoDeCadaVagaRepetida(t *testing.T) {
	club := domain.Club{
		Players: []domain.ClubPlayer{
			starterCP(1, mk(85, domain.CM, 80, 70, 80, 80, 75, 80)),
			starterCP(2, mk(85, domain.CM, 80, 70, 80, 80, 75, 80)),
			starterCP(3, mk(85, domain.CM, 80, 70, 80, 80, 75, 80)),
		},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 7, Position: domain.CM, PlayerID: 1},
			{Index: 8, Position: domain.CM, PlayerID: 2},
		}},
	}
	for i := range club.Players {
		club.Players[i].ID = int64(i + 1)
	}

	swaps := FindSquadSwapsWithOptions(club, SquadSwapOptions{
		Evaluator: avaliadorPorFuncaoTeste{},
		ContextosPorVaga: map[int]domain.ContextoAvaliacao{
			7: {Fonte: domain.FonteBot, Funcao: "volante"},
			8: {Fonte: domain.FonteBot, Funcao: "criador"},
		},
	})
	if len(swaps) != 1 {
		t.Fatalf("trocas = %+v, esperava somente a vaga de volante", swaps)
	}
	if swaps[0].Index != 7 || swaps[0].Current.ID != 1 || swaps[0].Candidate.ID != 3 {
		t.Fatalf("troca = %+v, esperava reserva 3 no lugar do titular 1 da vaga 7", swaps[0])
	}
	if swaps[0].CurrentEvaluation.Contexto.Funcao != "volante" || swaps[0].CandidateEvaluation.Contexto.Funcao != "volante" {
		t.Fatalf("contexto da comparação foi perdido: %+v", swaps[0])
	}
}

// Titular já sendo o melhor: nenhuma troca sugerida.
func TestFindSquadSwapsNaoSugereQuandoTitularJaEMelhor(t *testing.T) {
	titular := mk(90, domain.ST, 90, 90, 80, 85, 40, 80)
	titular.GGRating = 98.0
	titular.GGRatingPos = domain.ST
	titular.GGRatings = map[domain.Position]float64{domain.ST: 98.0}
	reserva := mk(80, domain.ST, 75, 75, 70, 75, 35, 70)
	reserva.GGRating = 85.0
	reserva.GGRatingPos = domain.ST
	reserva.GGRatings = map[domain.Position]float64{domain.ST: 85.0}

	club := domain.Club{
		Players: []domain.ClubPlayer{starterCP(1, titular), starterCP(2, reserva)},
		Squad:   domain.Squad{Starters: []domain.SquadSlot{{Index: 0, Position: domain.ST, PlayerID: 1}}},
	}
	if swaps := FindSquadSwaps(club); len(swaps) != 0 {
		t.Errorf("sugeriu %d trocas, esperava 0", len(swaps))
	}
}
