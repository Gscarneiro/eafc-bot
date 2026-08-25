package analyze

import (
	"fmt"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

var squadPlanFormationPositions = []domain.Position{
	domain.GK, domain.RB, domain.CB, domain.LB, domain.CDM,
	domain.CM, domain.CAM, domain.RM, domain.LM, domain.RW, domain.ST,
}

// squadPlanFixtureClub espelha gauntletFixtureClub: `perPosition` cartas só
// daquela posição, sem sobreposição entre posições — cada candidato só
// concorre dentro da própria posição, então o resultado do matching global é
// previsível. A carta de índice i em cada posição tem GGRating 60+i.
func squadPlanFixtureClub(perPosition int) domain.Club {
	var players []domain.ClubPlayer
	var starters []domain.SquadSlot
	id := int64(1)
	for slotIdx, pos := range squadPlanFormationPositions {
		for i := 0; i < perPosition; i++ {
			rating := 60.0 + float64(i)
			cp := domain.ClubPlayer{Player: domain.Player{
				ID: id, Name: fmt.Sprintf("%s-%d", pos, i), CommonName: fmt.Sprintf("%s-%d", pos, i),
				Rating: int(rating), Position: pos, League: "Liga Teste",
				GGRating: rating, GGRatingPos: pos, BasePlayerEaID: id,
			}}
			players = append(players, cp)
			if i == 0 {
				starters = append(starters, domain.SquadSlot{Index: slotIdx, Position: pos, PlayerID: id})
			}
			id++
		}
	}
	return domain.Club{
		Players: players,
		Squad:   domain.Squad{Formation: "teste-11", Starters: starters},
	}
}

func TestBuildSquadPlanStatusOkComElencoValido(t *testing.T) {
	club := squadPlanFixtureClub(3)
	plan := BuildSquadPlan(club, DefaultSquadPlanRequest())
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	if plan.Formation != "teste-11" {
		t.Errorf("Formation = %q, esperava \"teste-11\"", plan.Formation)
	}
	if len(plan.Scenarios) == 0 {
		t.Fatal("esperava pelo menos um cenário")
	}
	for _, sc := range plan.Scenarios {
		if len(sc.Starters) != len(squadPlanFormationPositions) {
			t.Fatalf("cenário %q com %d titulares, esperava %d", sc.Label, len(sc.Starters), len(squadPlanFormationPositions))
		}
	}
}

// O cenário de peso 0 (maior nota) precisa bater EXATAMENTE com
// OptimizeSquad — squadMatch é o mesmo motor, extraído sem mudar
// comportamento nenhum.
func TestBuildSquadScenarioPesoZeroBateComOptimizeSquad(t *testing.T) {
	club := squadPlanFixtureClub(3)
	optimizado := OptimizeSquad(club)
	if optimizado.Status == "unavailable" {
		t.Fatalf("OptimizeSquad: %q", optimizado.Reason)
	}

	sc, ok := buildSquadScenario(club.Players, club.Squad.Starters, nil, club.Squad.Starters, club, chemistry.ModeloPadrao(), 0, "")
	if !ok {
		t.Fatal("buildSquadScenario com peso 0 falhou")
	}
	if len(sc.Starters) != len(optimizado.Starters) {
		t.Fatalf("titulares: %d vs %d", len(sc.Starters), len(optimizado.Starters))
	}
	for i := range sc.Starters {
		if sc.Starters[i].Player.ID != optimizado.Starters[i].Player.ID {
			t.Fatalf("slot %d: %d vs %d", i, sc.Starters[i].Player.ID, optimizado.Starters[i].Player.ID)
		}
	}
}

func squadPlanIDFor(club domain.Club, pos domain.Position, idx int) int64 {
	n := -1
	for _, p := range club.Players {
		if p.Position == pos {
			n++
			if n == idx {
				return p.ID
			}
		}
	}
	return 0
}

// Lock com ClubItemID confirmado prende a CÓPIA FÍSICA exata — não "uma
// cópia qualquer" do mesmo jogador.
func TestBuildSquadPlanLockComClubItemIDPrendeACopiaExata(t *testing.T) {
	club := squadPlanFixtureClub(3)
	copiaA := domain.ClubPlayer{Player: domain.Player{
		ID: 9600, Position: domain.ST, League: "Liga Teste",
		GGRatings: map[domain.Position]float64{domain.ST: 95},
	}, ClubItemID: "item-a"}
	copiaB := domain.ClubPlayer{Player: domain.Player{
		ID: 9600, Position: domain.ST, League: "Liga Teste",
		GGRatings: map[domain.Position]float64{domain.ST: 95},
	}, ClubItemID: "item-b"}
	club.Players = append(club.Players, copiaA, copiaB)

	req := DefaultSquadPlanRequest()
	req.Locks = []SquadPlanLock{{PlayerID: 9600, ClubItemID: "item-b", Position: domain.ST}}
	plan := BuildSquadPlan(club, req)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	var achou *SquadAssignment
	for i, a := range plan.Scenarios[0].Starters {
		if a.Player.ID == 9600 {
			achou = &plan.Scenarios[0].Starters[i]
		}
	}
	if achou == nil {
		t.Fatal("jogador 9600 não apareceu titular")
	}
	if achou.Player.ClubItemID != "item-b" {
		t.Fatalf("ClubItemID travado = %q, esperava \"item-b\"", achou.Player.ClubItemID)
	}
}

// Sem ClubItemID, o lock degrada para "quantidade preservada": garante o
// JOGADOR no XI, sem prometer qual cópia nem qual slot.
func TestBuildSquadPlanLockSemClubItemIDGarantePresenca(t *testing.T) {
	club := squadPlanFixtureClub(3)
	lockID := squadPlanIDFor(club, domain.CM, 0) // CM mais fraco — sem o lock, perderia pro CM-1/CM-2

	req := DefaultSquadPlanRequest()
	req.Locks = []SquadPlanLock{{PlayerID: lockID}}
	plan := BuildSquadPlan(club, req)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	found := false
	for _, a := range plan.Scenarios[0].Starters {
		if a.Player.ID == lockID {
			found = true
		}
	}
	if !found {
		t.Fatalf("jogador %d (lock sem ClubItemID) não apareceu titular", lockID)
	}
}

func TestBuildSquadPlanLockParaJogadorInexistenteExplicaOMotivo(t *testing.T) {
	club := squadPlanFixtureClub(3)
	req := DefaultSquadPlanRequest()
	req.Locks = []SquadPlanLock{{PlayerID: 999999}}
	plan := BuildSquadPlan(club, req)
	if plan.Status != "unavailable" {
		t.Fatalf("status = %q, esperava unavailable", plan.Status)
	}
	if plan.Reason == "" {
		t.Fatal("esperava motivo explicando o lock inválido")
	}
}

func TestBuildSquadPlanExclusaoTiraJogadorDoPlano(t *testing.T) {
	club := squadPlanFixtureClub(3)
	excludedID := squadPlanIDFor(club, domain.RW, 2) // o melhor RW (rating 62)

	req := DefaultSquadPlanRequest()
	req.Excluded = map[int64]bool{excludedID: true}
	plan := BuildSquadPlan(club, req)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	for _, sc := range plan.Scenarios {
		for _, a := range sc.Starters {
			if a.Player.ID == excludedID {
				t.Fatalf("cenário %q: jogador excluído %d apareceu titular", sc.Label, excludedID)
			}
		}
	}
}

// Formação manual não tem titular "atual" nenhum (PlayerID zero nos slots),
// então nenhum Move deveria ser gerado — todo mundo é uma sugestão nova, não
// uma troca.
func TestBuildSquadPlanFormacaoManualNaoGeraMovimentos(t *testing.T) {
	club := squadPlanFixtureClub(3)
	req := DefaultSquadPlanRequest()
	req.FormationFrom = FormationManual
	req.ManualSlots = []domain.SquadSlot{
		{Index: 0, Position: domain.GK}, {Index: 1, Position: domain.CB}, {Index: 2, Position: domain.ST},
	}
	plan := BuildSquadPlan(club, req)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	if plan.Formation != "manual" {
		t.Errorf("Formation = %q, esperava \"manual\"", plan.Formation)
	}
	for _, sc := range plan.Scenarios {
		if len(sc.Moves) != 0 {
			t.Fatalf("cenário %q: %d movimentos numa formação manual sem titular atual, esperava 0", sc.Label, len(sc.Moves))
		}
	}
}

func TestBuildSquadPlanFormacaoManualSemSlotsExplicaOMotivo(t *testing.T) {
	club := squadPlanFixtureClub(3)
	req := DefaultSquadPlanRequest()
	req.FormationFrom = FormationManual
	plan := BuildSquadPlan(club, req)
	if plan.Status != "unavailable" || plan.Reason == "" {
		t.Fatalf("status = %q reason = %q, esperava unavailable com motivo", plan.Status, plan.Reason)
	}
}

func TestBuildSquadPlanEscalacaoNaoSincronizadaExplicaOMotivo(t *testing.T) {
	club := domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{ID: 1, GGRating: 90}}}}
	plan := BuildSquadPlan(club, DefaultSquadPlanRequest())
	if plan.Status != "unavailable" || plan.Reason == "" {
		t.Fatalf("status = %q reason = %q, esperava unavailable com motivo", plan.Status, plan.Reason)
	}
}

// A fronteira nota×química precisa produzir mais de um cenário genuinamente
// distinto quando existe um trade-off real — mesma armadilha do Gauntlet:
// ModeloPadrao() satura o teto só com Base e não deixaria nada aparecer (ver
// o comentário de modeloFC26Vinculos).
func TestBuildSquadPlanFronteiraProduzCenariosDistintos(t *testing.T) {
	club := squadPlanFixtureClub(3)
	for i := range club.Players {
		club.Players[i].League = "" // isola o efeito de clube, sem saturar via liga
	}
	// limiaresClube exige pelo menos 2 titulares do mesmo clube pra valer
	// QUALQUER ponto — uma busca local de UMA passada por slot nunca troca o
	// PRIMEIRO membro sozinho (contagem=1 não cruza limiar nenhum, então o
	// placar nunca compensa a perda de rating). Por isso a âncora (GK,
	// rating 65) vence de cara no rating puro e já entra no XI sem precisar
	// de química — é o que dá ao segundo membro, ao ser avaliado, algo pra
	// se juntar e cruzar o limiar {2,1} numa troca só.
	vinculado := func(id int64, pos domain.Position, rating float64) domain.ClubPlayer {
		return domain.ClubPlayer{Player: domain.Player{
			ID: id, Position: pos, Club: "Vinculados",
			GGRatings: map[domain.Position]float64{pos: rating},
		}}
	}
	club.Players = append(club.Players,
		vinculado(40000, domain.GK, 65), // âncora: vence por rating puro
		vinculado(40001, domain.CB, 60.5), vinculado(40002, domain.CM, 60.5), vinculado(40003, domain.RM, 60.5))

	m, err := chemistry.Escolher("fc26_vinculos")
	if err != nil {
		t.Fatal(err)
	}
	req := DefaultSquadPlanRequest()
	req.ChemistryModel = m
	plan := BuildSquadPlan(club, req)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	if len(plan.Scenarios) < 2 {
		t.Fatalf("esperava pelo menos 2 cenários na fronteira, veio %d: %+v", len(plan.Scenarios), plan.Scenarios)
	}

	// A fronteira precisa mostrar o trade-off de verdade: o de maior nota
	// não pode ter química maior ou igual ao de maior química (senão um
	// dominaria o outro e não formaria fronteira nenhuma).
	var maiorNota, maiorQuimica SquadPlanScenario
	for _, sc := range plan.Scenarios {
		if sc.AverageRating > maiorNota.AverageRating {
			maiorNota = sc
		}
		if chemTotal(sc) > chemTotal(maiorQuimica) {
			maiorQuimica = sc
		}
	}
	if maiorNota.AverageRating <= maiorQuimica.AverageRating {
		t.Fatalf("cenário de maior nota (%.2f) não é maior que o de maior química (%.2f)",
			maiorNota.AverageRating, maiorQuimica.AverageRating)
	}
	if chemTotal(maiorQuimica) <= chemTotal(maiorNota) {
		t.Fatalf("cenário de maior química (%d) não é maior que o de maior nota (%d)",
			chemTotal(maiorQuimica), chemTotal(maiorNota))
	}
}

// MaxScenarios limita quantos cenários voltam, mesmo quando a fronteira tem
// mais pontos não dominados que isso.
func TestBuildSquadPlanRespeitaMaxScenarios(t *testing.T) {
	club := squadPlanFixtureClub(3)
	for i := range club.Players {
		club.Players[i].League = ""
	}
	vinculado := func(id int64, pos domain.Position, rating float64) domain.ClubPlayer {
		return domain.ClubPlayer{Player: domain.Player{
			ID: id, Position: pos, Club: "Vinculados",
			GGRatings: map[domain.Position]float64{pos: rating},
		}}
	}
	club.Players = append(club.Players,
		vinculado(40000, domain.GK, 65), // âncora — ver o comentário em TestBuildSquadPlanFronteiraProduzCenariosDistintos
		vinculado(40001, domain.CB, 60.8), vinculado(40002, domain.CM, 60.6),
		vinculado(40003, domain.RM, 60.4), vinculado(40004, domain.LM, 60.2))

	m, err := chemistry.Escolher("fc26_vinculos")
	if err != nil {
		t.Fatal(err)
	}
	req := DefaultSquadPlanRequest()
	req.ChemistryModel = m
	req.MaxScenarios = 2
	plan := BuildSquadPlan(club, req)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	if len(plan.Scenarios) > 2 {
		t.Fatalf("cenários = %d, esperava no máximo 2", len(plan.Scenarios))
	}
}

// Uma posição sem NENHUMA alternativa no elenco (só a titular escalada joga
// ali) precisa virar necessidade — o Squad Planner aponta, não escolhe
// compra.
func TestSquadPlanNeedsApontaPosicaoSemAlternativa(t *testing.T) {
	club := domain.Club{
		Players: []domain.ClubPlayer{
			{Player: domain.Player{ID: 1, Position: domain.GK, GGRating: 85, GGRatingPos: domain.GK}, InSquad: true},
			{Player: domain.Player{ID: 2, Position: domain.CB, GGRating: 80, GGRatingPos: domain.CB}},
			{Player: domain.Player{ID: 3, Position: domain.CB, GGRating: 79, GGRatingPos: domain.CB}},
		},
		Squad: domain.Squad{Starters: []domain.SquadSlot{
			{Index: 0, Position: domain.GK, PlayerID: 1},
			{Index: 1, Position: domain.CB, PlayerID: 2},
		}},
	}
	plan := BuildSquadPlan(club, DefaultSquadPlanRequest())
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	var gkNeed *SquadPlanNeed
	for i, n := range plan.Needs {
		if n.Position == domain.GK {
			gkNeed = &plan.Needs[i]
		}
	}
	if gkNeed == nil {
		t.Fatalf("esperava necessidade no GK (sem alternativa nenhuma no elenco), needs = %+v", plan.Needs)
	}
}
