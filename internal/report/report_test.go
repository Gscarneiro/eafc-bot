package report

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func starter(id int64, pos domain.Position, ggRating float64) domain.ClubPlayer {
	return domain.ClubPlayer{Player: domain.Player{
		ID: id, Position: pos, Rating: 85,
		CommonName: "Jogador " + string(pos),
		Attributes: domain.Attributes{Pace: 80, Shooting: 70, Passing: 75, Dribbling: 78, Defending: 40, Physical: 75},
		GGRating:   ggRating,
	}}
}

// A "Nota do time" usa o GG Rating do fut.gg — a nota que já é familiar de
// quem usa o site, presa em ~0-99 — em vez do Score() deste bot, que soma
// bônus sem teto e pode passar de 99. É a mudança que resolve "deu 103 mas
// o máximo é 100".
func TestSquadSummaryUsaGGRatingQuandoDisponivel(t *testing.T) {
	club := domain.Club{
		Players: []domain.ClubPlayer{
			starter(1, domain.GK, 90.0),
			starter(2, domain.ST, 92.4), // Gnabry: overall alto, GGRating menor
		},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 0, Position: domain.GK, PlayerID: 1},
			{Index: 1, Position: domain.ST, PlayerID: 2},
		}},
	}

	avg, _, _, _ := SquadSummary(club)
	want := (90.0 + 92.4) / 2
	if avg < want-0.01 || avg > want+0.01 {
		t.Errorf("nota do time = %.2f, esperava a média de GG Rating (%.2f)", avg, want)
	}
	if avg > 99 {
		t.Errorf("nota do time = %.2f — não devia passar de ~99, GG Rating nunca estoura essa escala", avg)
	}
}

// Fontes que não são o GG Club (csv, chrome) não mandam GG Rating nenhum.
// Sem ele, a nota do time cai pro Score() interno em vez de mostrar zero —
// mas o próprio Score() pode passar de 99, e isso é esperado nesse caso:
// só existe se a fonte não trouxe o número oficial pra comparar.
func TestSquadSummaryCaiParaScoreSemGGRating(t *testing.T) {
	club := domain.Club{
		Players: []domain.ClubPlayer{starter(1, domain.GK, 0)},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 0, Position: domain.GK, PlayerID: 1},
		}},
	}
	avg, _, _, _ := SquadSummary(club)
	if avg <= 0 {
		t.Errorf("nota do time = %.2f, esperava cair pro Score() e não zerar", avg)
	}
}

// O elo mais fraco continua sendo decidido pelo Score() (pesa a função do
// jogador, não só o cartão isolado) — o GG Rating dele só acompanha como
// referência, não decide quem é o mais fraco.
func TestSquadSummaryEloMaisFracoLevaGGRatingComoReferencia(t *testing.T) {
	club := domain.Club{
		Players: []domain.ClubPlayer{
			starter(1, domain.GK, 95.0),
			starter(2, domain.CB, 60.0), // pior em campo E menor GGRating
		},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 0, Position: domain.GK, PlayerID: 1},
			{Index: 1, Position: domain.CB, PlayerID: 2},
		}},
	}
	_, weakSlot, _, weakGG := SquadSummary(club)
	if weakSlot != domain.CB {
		t.Errorf("elo mais fraco = %q, esperava CB", weakSlot)
	}
	if weakGG != 60.0 {
		t.Errorf("GG Rating do elo mais fraco = %.1f, esperava 60.0", weakGG)
	}
}

// O elenco principal sai na ordem que o fut.gg usa (positionIdx), não na
// ordem em que os slots foram lidos.
func TestMainSquadOrdenaPorIndex(t *testing.T) {
	club := domain.Club{
		Players: []domain.ClubPlayer{
			starter(10, domain.ST, 90),
			starter(1, domain.GK, 88),
			starter(4, domain.CB, 85),
		},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 10, Position: domain.ST, PlayerID: 10},
			{Index: 0, Position: domain.GK, PlayerID: 1},
			{Index: 4, Position: domain.CB, PlayerID: 4},
		}},
	}
	got := MainSquad(club)
	if len(got) != 3 {
		t.Fatalf("montou %d cards, esperava 3", len(got))
	}
	wantOrder := []int64{1, 4, 10}
	for i, id := range wantOrder {
		if got[i].Player.ID != id {
			t.Errorf("posição %d: id %d, esperava %d", i, got[i].Player.ID, id)
		}
	}
}
