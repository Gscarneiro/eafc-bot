package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

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

// Sem GG Rating em algum titular (fonte que não é o GG Club), a escolha
// volta pro Score() — não pode travar nem escolher às cegas.
func TestElofMaisFracoCaiParaScoreSemGGRatingCompleto(t *testing.T) {
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
	if len(weak) == 0 || weak[0].Player.ID != 2 {
		t.Fatalf("esperava cair pro Score() e escolher o jogador 2, veio %+v", weak)
	}
}

// FindSquadSwaps varre o BANCO, não o mercado: um reserva na mesma posição
// com GG Rating maior é uma troca de custo zero.
func TestFindSquadSwapsAchaReservaComGGRatingMaior(t *testing.T) {
	titular := mk(85, domain.LM, 80, 70, 75, 78, 40, 75)
	titular.GGRating = 92.4
	reserva := mk(84, domain.LM, 82, 72, 77, 80, 42, 76)
	reserva.GGRating = 96.1
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

// Titular já sendo o melhor: nenhuma troca sugerida.
func TestFindSquadSwapsNaoSugereQuandoTitularJaEMelhor(t *testing.T) {
	titular := mk(90, domain.ST, 90, 90, 80, 85, 40, 80)
	titular.GGRating = 98.0
	reserva := mk(80, domain.ST, 75, 75, 70, 75, 35, 70)
	reserva.GGRating = 85.0

	club := domain.Club{
		Players: []domain.ClubPlayer{starterCP(1, titular), starterCP(2, reserva)},
		Squad:   domain.Squad{Starters: []domain.SquadSlot{{Index: 0, Position: domain.ST, PlayerID: 1}}},
	}
	if swaps := FindSquadSwaps(club); len(swaps) != 0 {
		t.Errorf("sugeriu %d trocas, esperava 0", len(swaps))
	}
}
