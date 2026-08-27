package analyze

import (
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"testing"
	"time"
)

func clubeSolver(n int, rating int) domain.Club {
	c := domain.Club{}
	for i := 0; i < n; i++ {
		c.Players = append(c.Players, domain.ClubPlayer{Player: domain.Player{ID: int64(i + 1), Name: "Carta", Rating: rating, Nation: "Portugal", Version: "Gold Rare"}, ClubItemID: "item-" + string(rune('a'+i)), Chemistry: 3})
	}
	return c
}

func TestPlanoSBCBloqueiaLinhaDesconhecida(t *testing.T) {
	got := MontarPlanoSBC(domain.SBCChallenge{RequirementsText: []string{"Squad must contain at least 1 Icon"}}, clubeSolver(11, 85), nil, PlanoSBCOpcoes{DadosDeEmprestimoConfirmados: true})
	if got.Estado != SBCRequisitoDesconhecido {
		t.Fatalf("estado=%s", got.Estado)
	}
}
func TestPlanoSBCValidaCertificadoECopiasFisicas(t *testing.T) {
	got := MontarPlanoSBC(domain.SBCChallenge{RequirementsText: []string{"Min. Team Rating: 85"}}, clubeSolver(11, 85), nil, PlanoSBCOpcoes{DadosDeEmprestimoConfirmados: true, Timeout: time.Second})
	if got.Estado != SBCOtimoComprovado || !got.Certificado.Valido || len(got.Itens) != 11 {
		t.Fatalf("plano=%+v", got)
	}
}
func TestPlanoSBCNoLimiteNaoChamaOtimo(t *testing.T) {
	got := MontarPlanoSBC(domain.SBCChallenge{RequirementsText: []string{"Min. Team Rating: 85"}}, clubeSolver(12, 85), nil, PlanoSBCOpcoes{DadosDeEmprestimoConfirmados: true, MaxNos: 1, Timeout: time.Second})
	if got.Estado == SBCOtimoComprovado || !got.LimiteAtingido {
		t.Fatalf("plano=%+v", got)
	}
}
