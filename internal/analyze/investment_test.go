package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestNuncaSugereCartaJaPossuidaComoInvestimento(t *testing.T) {
	club := domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{ID: 1}}}}
	candidates := []domain.Player{
		{ID: 1, Rating: 90, Price: domain.Price{Coins: 10000}, MomentumPct: 30},
	}

	got, funnel := FindInvestments(club, candidates, nil, DefaultInvestmentOptions())
	if len(got) != 0 {
		t.Fatalf("carta que você já tem não deveria virar sugestão de investimento, veio %d", len(got))
	}
	if funnel.Owned != 1 {
		t.Errorf("funnel.Owned = %d, esperava 1", funnel.Owned)
	}
}

func TestFiltraCartaNaoTradeavelMesmoComDescontoAlto(t *testing.T) {
	club := domain.Club{}
	candidates := []domain.Player{
		{ID: 1, Rating: 90, Price: domain.Price{IsSBC: true}, MomentumPct: 50},
		{ID: 2, Rating: 90, Price: domain.Price{Coins: 10000, Extinct: true}, MomentumPct: 50},
	}

	got, funnel := FindInvestments(club, candidates, nil, DefaultInvestmentOptions())
	if len(got) != 0 {
		t.Fatalf("SBC-only e extinta não podem virar sugestão de compra, veio %d", len(got))
	}
	if funnel.NotTradeable != 2 {
		t.Errorf("funnel.NotTradeable = %d, esperava 2", funnel.NotTradeable)
	}
}

// A pesquisa de mercado é enfática: quando existe uma versão do mesmo
// jogador com rating maior, a inferior nunca deve entrar como sugestão de
// compra — ela tende a desabar quando a melhor sai (ou já saiu).
func TestNuncaSugereVersaoInferiorQuandoIrmaMelhorEstaEntreOsCandidatos(t *testing.T) {
	club := domain.Club{}
	candidates := []domain.Player{
		{ID: 1, BasePlayerEaID: 500, Rating: 85, Price: domain.Price{Coins: 10000}, MomentumPct: 40},
		{ID: 2, BasePlayerEaID: 500, Rating: 92, Price: domain.Price{Coins: 80000}, MomentumPct: 40},
	}

	got, funnel := FindInvestments(club, candidates, nil, DefaultInvestmentOptions())
	if len(got) != 1 || got[0].Candidate.ID != 2 {
		t.Fatalf("esperava só a versão de rating maior (id=2) sugerida, veio %+v", got)
	}
	if funnel.SupersededBySibling != 1 {
		t.Errorf("funnel.SupersededBySibling = %d, esperava 1", funnel.SupersededBySibling)
	}
}

func TestAbaixoDoPisoDeMomentumViraRejeitadoERegistraOMelhor(t *testing.T) {
	club := domain.Club{}
	candidates := []domain.Player{
		{ID: 1, Name: "Pouco", Rating: 85, Price: domain.Price{Coins: 10000}, MomentumPct: 3},
		{ID: 2, Name: "QuaseLa", Rating: 85, Price: domain.Price{Coins: 10000}, MomentumPct: 12},
	}
	opt := DefaultInvestmentOptions() // MinMomentumPct: 15

	got, funnel := FindInvestments(club, candidates, nil, opt)
	if len(got) != 0 {
		t.Fatalf("nenhum candidato deveria passar do piso de %.1f%%, veio %d", opt.MinMomentumPct, len(got))
	}
	if funnel.BelowMinMomentum != 2 {
		t.Errorf("funnel.BelowMinMomentum = %d, esperava 2", funnel.BelowMinMomentum)
	}
	if !funnel.HasBestRejected || funnel.BestRejectedPct != 12 || funnel.BestRejectedName != "QuaseLa" {
		t.Errorf("esperava o melhor rejeitado ser QuaseLa a 12%%, veio %+v", funnel)
	}
}

// Sinal mais robusto da pesquisa: quando o jogador ganha uma carta
// especial nova (aparece em newCards), a carta atual sai do pool de
// pacotes — sinalizado independente do desconto de momentum já bater o
// piso.
func TestSinalOutOfPacksQuandoJogadorGanhaCartaEspecialNova(t *testing.T) {
	club := domain.Club{}
	candidates := []domain.Player{
		{ID: 1, BasePlayerEaID: 700, Rating: 84, Price: domain.Price{Coins: 5000}, MomentumPct: 20},
	}
	newCards := []domain.Player{
		{ID: 999, BasePlayerEaID: 700, Rating: 91}, // a carta especial nova do mesmo jogador
	}

	got, _ := FindInvestments(club, candidates, newCards, DefaultInvestmentOptions())
	if len(got) != 1 {
		t.Fatalf("esperava 1 sugestão, veio %d", len(got))
	}
	if got[0].Signal != "out-of-packs" {
		t.Errorf("Signal = %q, esperava \"out-of-packs\"", got[0].Signal)
	}
}

func TestOrdenaOutOfPacksAntesDeDesconto(t *testing.T) {
	club := domain.Club{}
	candidates := []domain.Player{
		{ID: 1, Name: "SoDesconto", Rating: 84, Price: domain.Price{Coins: 5000}, MomentumPct: 50},
		{ID: 2, Name: "OutOfPacks", BasePlayerEaID: 700, Rating: 84, Price: domain.Price{Coins: 5000}, MomentumPct: 16},
	}
	newCards := []domain.Player{{ID: 999, BasePlayerEaID: 700, Rating: 91}}

	got, _ := FindInvestments(club, candidates, newCards, DefaultInvestmentOptions())
	if len(got) != 2 {
		t.Fatalf("esperava 2 sugestões, veio %d", len(got))
	}
	if got[0].Candidate.ID != 2 {
		t.Errorf("out-of-packs deveria vir primeiro mesmo com desconto menor, veio %+v", got[0])
	}
}

// ImpliedAverage é a média recente reconstruída a partir do desconto —
// currentPrice / (1 - pct/100). Com 20%% de desconto e preço 8000, a
// média deveria ser 10000 (8000 é 80%% de 10000).
func TestImpliedAverageReconstroiAMediaDoDesconto(t *testing.T) {
	club := domain.Club{}
	candidates := []domain.Player{
		{ID: 1, Rating: 84, Price: domain.Price{Coins: 8000}, MomentumPct: 20},
	}

	got, _ := FindInvestments(club, candidates, nil, DefaultInvestmentOptions())
	if len(got) != 1 {
		t.Fatalf("esperava 1 sugestão, veio %d", len(got))
	}
	if got[0].ImpliedAverage != 10000 {
		t.Errorf("ImpliedAverage = %d, esperava 10000", got[0].ImpliedAverage)
	}
}
