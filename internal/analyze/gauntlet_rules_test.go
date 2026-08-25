package analyze

import (
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

func TestGauntletRulesValidateAceitaTresAQuinco(t *testing.T) {
	for _, rodadas := range []int{3, 4, 5} {
		r := DefaultGauntletRules()
		r.Rodadas = rodadas
		if err := r.Validate(); err != nil {
			t.Errorf("rodadas=%d deveria ser válido: %v", rodadas, err)
		}
	}
}

func TestGauntletRulesValidateRecusaForaDoIntervalo(t *testing.T) {
	for _, rodadas := range []int{0, 1, 2, 6, 10} {
		r := DefaultGauntletRules()
		r.Rodadas = rodadas
		if err := r.Validate(); err == nil {
			t.Errorf("rodadas=%d deveria ser recusado", rodadas)
		}
	}
}

// BuildGauntletPlanWithOptions (a API antiga) precisa continuar batendo
// exatamente com BuildGauntletPlanFromRequest(DefaultGauntletRequest()) —
// é o que garante que generalizar o motor não mudou o comportamento de quem
// já chamava a função antiga.
func TestBuildGauntletPlanWithOptionsBateComRequestPadrao(t *testing.T) {
	club := gauntletFixtureClub(8)
	viaOpcoes := BuildGauntletPlanWithOptions(club, DefaultGauntletOptions())
	viaRequest := BuildGauntletPlanFromRequest(club, DefaultGauntletRequest())

	if viaOpcoes.Status != "ok" || viaRequest.Status != "ok" {
		t.Fatalf("status: opções=%q request=%q", viaOpcoes.Status, viaRequest.Status)
	}
	if len(viaOpcoes.Rounds) != len(viaRequest.Rounds) {
		t.Fatalf("rodadas: opções=%d request=%d", len(viaOpcoes.Rounds), len(viaRequest.Rounds))
	}
	for i := range viaOpcoes.Rounds {
		if viaOpcoes.Rounds[i].TotalRating != viaRequest.Rounds[i].TotalRating {
			t.Fatalf("rodada %d: total rating diverge (%.2f vs %.2f)",
				i+1, viaOpcoes.Rounds[i].TotalRating, viaRequest.Rounds[i].TotalRating)
		}
		for j := range viaOpcoes.Rounds[i].Starters {
			if viaOpcoes.Rounds[i].Starters[j].Player.ID != viaRequest.Rounds[i].Starters[j].Player.ID {
				t.Fatalf("rodada %d slot %d: titular diverge (%d vs %d)",
					i+1, j, viaOpcoes.Rounds[i].Starters[j].Player.ID, viaRequest.Rounds[i].Starters[j].Player.ID)
			}
		}
	}
}

func gauntletRequestFor(rodadas int) GauntletRequest {
	req := DefaultGauntletRequest()
	req.Rules.Rodadas = rodadas
	return req
}

func TestBuildGauntletPlanFromRequestMontaTresRodadas(t *testing.T) {
	club := gauntletFixtureClub(6) // 66 cartas: 3 rodadas x 18 (11+7) = 54, sobra folga
	plan := BuildGauntletPlanFromRequest(club, gauntletRequestFor(3))
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	if len(plan.Rounds) != 3 {
		t.Fatalf("rodadas = %d, esperava 3", len(plan.Rounds))
	}
}

func TestBuildGauntletPlanFromRequestMontaCincoRodadas(t *testing.T) {
	club := gauntletFixtureClub(9) // 99 cartas: 5 rodadas x 18 = 90, sobra folga
	plan := BuildGauntletPlanFromRequest(club, gauntletRequestFor(5))
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	if len(plan.Rounds) != 5 {
		t.Fatalf("rodadas = %d, esperava 5", len(plan.Rounds))
	}
	seen := map[int64]bool{}
	for _, round := range plan.Rounds {
		for _, a := range round.Starters {
			if seen[a.Player.ID] {
				t.Fatalf("carta %d repetida entre as 5 rodadas", a.Player.ID)
			}
			seen[a.Player.ID] = true
		}
		for _, b := range round.Bench {
			if seen[b.ID] {
				t.Fatalf("carta %d repetida entre as 5 rodadas (banco)", b.ID)
			}
			seen[b.ID] = true
		}
	}
}

// mais_forte_primeiro é o espelho de crescente: a primeira rodada escolhe
// primeiro no pool cheio, então ela — não a última — deve ser a mais forte.
func TestGauntletMaisForteFirstInverteAOrdemDeCrescente(t *testing.T) {
	club := gauntletFixtureClub(8)

	crescente := BuildGauntletPlanFromRequest(club, gauntletRequestFor(4))
	if crescente.Status != "ok" {
		t.Fatalf("crescente: status = %q, motivo = %q", crescente.Status, crescente.Reason)
	}

	req := gauntletRequestFor(4)
	req.Strategy = GauntletMaisForteFirst
	maisForte := BuildGauntletPlanFromRequest(club, req)
	if maisForte.Status != "ok" {
		t.Fatalf("mais_forte_primeiro: status = %q, motivo = %q", maisForte.Status, maisForte.Reason)
	}

	if maisForte.Rounds[0].TotalRating <= crescente.Rounds[0].TotalRating {
		t.Fatalf("mais_forte_primeiro devia dar a rodada 1 mais forte que crescente: %.1f <= %.1f",
			maisForte.Rounds[0].TotalRating, crescente.Rounds[0].TotalRating)
	}
	for i := 0; i < len(maisForte.Rounds)-1; i++ {
		if maisForte.Rounds[i].TotalRating < maisForte.Rounds[i+1].TotalRating {
			t.Fatalf("mais_forte_primeiro: rodada %d mais fraca que a %d — esperava força decrescente", i+1, i+2)
		}
	}
}

// gauntletLockedClub prende um jogador MEDÍOCRE (bem abaixo do pool) na
// rodada 1 via lock — isso torna a rodada 1 estruturalmente pior que as
// outras (que seguem simétricas entre si), o suficiente para diferenciar
// equilibrada de valor_total por DADO, não por índice.
func gauntletLockedIDFor(club domain.Club, pos domain.Position) int64 {
	for _, p := range club.Players {
		if p.Position == pos {
			return p.ID
		}
	}
	return 0
}

// equilibrada prioriza a rodada que sairia mais fraca (dá a ela a escolha
// livre primeiro, pra compensar o quanto der); valor_total prioriza a que
// ganharia mais agora (deixa a rodada já prejudicada por último, com o que
// sobrar). Um lock fraco na rodada 1 torna essa divergência mensurável: sob
// equilibrada a rodada 1 deve compensar melhor (processada mais cedo) do
// que sob valor_total (processada por último).
func TestGauntletEquilibradaEValorTotalDivergemComRodadaPrejudicada(t *testing.T) {
	club := gauntletFixtureClub(8)
	lockID := gauntletLockedIDFor(club, domain.GK) // GK-0, a pior carta de GK do pool (rating 60)

	base := gauntletRequestFor(4)
	base.Locks = []GauntletLock{{Round: 1, PlayerID: lockID}}

	eq := base
	eq.Strategy = GauntletEquilibrada
	planEq := BuildGauntletPlanFromRequest(club, eq)
	if planEq.Status != "ok" {
		t.Fatalf("equilibrada: status = %q, motivo = %q", planEq.Status, planEq.Reason)
	}

	vt := base
	vt.Strategy = GauntletValorTotal
	planVt := BuildGauntletPlanFromRequest(club, vt)
	if planVt.Status != "ok" {
		t.Fatalf("valor_total: status = %q, motivo = %q", planVt.Status, planVt.Reason)
	}

	if planEq.Rounds[0].TotalRating <= planVt.Rounds[0].TotalRating {
		t.Fatalf("com a rodada 1 prejudicada pelo lock, equilibrada devia compensar melhor que valor_total: %.1f <= %.1f",
			planEq.Rounds[0].TotalRating, planVt.Rounds[0].TotalRating)
	}
}

// Um lock com ClubItemID confirmado prende a CÓPIA FÍSICA exata pedida —
// não "uma cópia qualquer" do mesmo jogador.
func TestGauntletLockComClubItemIDPrendeACopiaExata(t *testing.T) {
	club := gauntletFixtureClub(8)
	// Duas cópias do mesmo jogador, IDs de carta e ClubItemID diferentes —
	// sem BasePlayerEaID/slug elas viram jogadores diferentes por
	// PlayerKey; para testar o lock por ClubItemID isso não importa (o lock
	// resolve por ClubItemID, não por PlayerKey).
	copiaA := domain.ClubPlayer{Player: domain.Player{
		ID: 9600, Position: domain.ST, League: "Liga Teste",
		GGRatings: map[domain.Position]float64{domain.ST: 90},
	}, ClubItemID: "item-a"}
	copiaB := domain.ClubPlayer{Player: domain.Player{
		ID: 9600, Position: domain.ST, League: "Liga Teste",
		GGRatings: map[domain.Position]float64{domain.ST: 90},
	}, ClubItemID: "item-b"}
	club.Players = append(club.Players, copiaA, copiaB)

	req := gauntletRequestFor(4)
	req.Locks = []GauntletLock{{Round: 1, PlayerID: 9600, ClubItemID: "item-b", Position: domain.ST}}
	plan := BuildGauntletPlanFromRequest(club, req)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	var achou *GauntletAssignment
	for i, a := range plan.Rounds[0].Starters {
		if a.Player.ID == 9600 {
			achou = &plan.Rounds[0].Starters[i]
		}
	}
	if achou == nil {
		t.Fatal("jogador 9600 não apareceu titular na rodada 1")
	}
	if achou.Player.ClubItemID != "item-b" {
		t.Fatalf("ClubItemID travado = %q, esperava \"item-b\"", achou.Player.ClubItemID)
	}
	if achou.Position != domain.ST {
		t.Fatalf("posição = %q, esperava ST (a pedida no lock)", achou.Position)
	}
}

// Sem ClubItemID, o lock degrada para "quantidade preservada": garante que
// o JOGADOR entra como titular na rodada pedida, sem prometer slot exato.
func TestGauntletLockSemClubItemIDGarantePresencaSemPrometerSlot(t *testing.T) {
	club := gauntletFixtureClub(8)
	lockID := gauntletLockedIDFor(club, domain.CM)

	req := gauntletRequestFor(4)
	req.Locks = []GauntletLock{{Round: 2, PlayerID: lockID}}
	plan := BuildGauntletPlanFromRequest(club, req)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	found := false
	for _, a := range plan.Rounds[1].Starters {
		if a.Player.ID == lockID {
			found = true
		}
	}
	if !found {
		t.Fatalf("jogador %d (lock sem ClubItemID) não apareceu titular na rodada 2", lockID)
	}
}

// Um lock para um jogador que não existe no elenco precisa falhar com um
// motivo que aponte exatamente o requisito não atendido — não uma rodada
// incompleta silenciosa.
func TestGauntletLockParaJogadorInexistenteExplicaOMotivo(t *testing.T) {
	club := gauntletFixtureClub(8)
	req := gauntletRequestFor(4)
	req.Locks = []GauntletLock{{Round: 1, PlayerID: 999999}}
	plan := BuildGauntletPlanFromRequest(club, req)
	if plan.Status != "unavailable" {
		t.Fatalf("status = %q, esperava unavailable", plan.Status)
	}
	if plan.Reason == "" {
		t.Fatal("esperava um motivo explicando o lock inválido")
	}
}

// Excluir um jogador precisa tirá-lo do plano inteiro — titular e banco, em
// toda rodada.
func TestGauntletExclusaoTiraJogadorDeTodoOPlano(t *testing.T) {
	club := gauntletFixtureClub(8)
	excludedID := gauntletLockedIDFor(club, domain.RW)

	req := gauntletRequestFor(4)
	req.Excluded = map[int64]bool{excludedID: true}
	plan := BuildGauntletPlanFromRequest(club, req)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	for _, round := range plan.Rounds {
		for _, a := range round.Starters {
			if a.Player.ID == excludedID {
				t.Fatalf("jogador excluído %d apareceu titular na rodada %d", excludedID, round.Round)
			}
		}
		for _, b := range round.Bench {
			if b.ID == excludedID {
				t.Fatalf("jogador excluído %d apareceu no banco da rodada %d", excludedID, round.Round)
			}
		}
	}
}

// Duas versões do mesmo jogador (mesmo BasePlayerEaID) continuam proibidas
// na MESMA rodada pelo motor geral — a trava não pode ter se perdido na
// generalização.
func TestGauntletFromRequestNaoEscalaDuasVersoesDoMesmoJogadorNaMesmaRodada(t *testing.T) {
	plan := BuildGauntletPlanFromRequest(gauntletComDuasVersoes(), DefaultGauntletRequest())
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	for _, round := range plan.Rounds {
		por := versoesPorRodada(round)
		for base, ids := range por {
			if len(ids) > 1 {
				t.Fatalf("rodada %d: jogador-base %d entrou %d vezes (%v)", round.Round, base, len(ids), ids)
			}
		}
	}
}

// Elenco pequeno demais para a regra pedida precisa dizer QUANTAS cartas
// faltam e para quê — não só "não deu".
func TestGauntletFromRequestExplicaFaltaDeCartasPorRequisito(t *testing.T) {
	club := gauntletFixtureClub(2) // 22 cartas, bem abaixo do que 4 rodadas exigem
	plan := BuildGauntletPlanFromRequest(club, DefaultGauntletRequest())
	if plan.Status != "unavailable" {
		t.Fatalf("status = %q, esperava unavailable", plan.Status)
	}
	if plan.Reason == "" {
		t.Fatal("esperava motivo explicando quantas cartas faltam")
	}
}

// Formação manual com menos slots do que Rules.Titulares pede precisa
// falhar dizendo QUAL é o descompasso, não montar rodada incompleta.
func TestGauntletFormacaoManualComContagemErradaExplicaOMotivo(t *testing.T) {
	club := gauntletFixtureClub(8)
	req := gauntletRequestFor(4)
	req.FormationFrom = FormationManual
	req.ManualSlots = []domain.SquadSlot{{Index: 0, Position: domain.GK, PlayerID: 1}}
	plan := BuildGauntletPlanFromRequest(club, req)
	if plan.Status != "unavailable" {
		t.Fatalf("status = %q, esperava unavailable", plan.Status)
	}
	if plan.Reason == "" {
		t.Fatal("esperava motivo explicando o descompasso de slots x titulares")
	}
}

// chemistrySwapRound é a peça central da química ponderada: com peso 0 não
// troca nada (compatibilidade histórica); com peso suficiente, troca quando
// o ganho de vínculo compensa a perda de rating. Usa o modelo fc26_vinculos
// (Base 0) de propósito: ModeloPadrao() satura o teto só com Base
// (Base==MaxPorJogador), então o vínculo nunca mudaria o placar e a
// diferença de peso ficaria invisível neste teste — ver o comentário de
// modeloFC26Vinculos.
func TestChemistrySwapRoundTrocaSoQuandoPesoCompensaAPerdaDeRating(t *testing.T) {
	m, err := chemistry.Escolher("fc26_vinculos")
	if err != nil {
		t.Fatal(err)
	}
	pos := domain.ST
	membro := func(id int64) domain.ClubPlayer {
		return domain.ClubPlayer{Player: domain.Player{
			ID: id, Position: pos, Club: "ClubeX",
			GGRatings: map[domain.Position]float64{pos: 80},
		}}
	}
	fora := domain.ClubPlayer{Player: domain.Player{
		ID: 4, Position: pos, Club: "Outro",
		GGRatings: map[domain.Position]float64{pos: 80},
	}}
	candidato := domain.ClubPlayer{Player: domain.Player{
		ID: 5, Position: pos, Club: "ClubeX",
		GGRatings: map[domain.Position]float64{pos: 78}, // 2 pontos mais fraco
	}}

	newRound := func() (*GauntletSquad, []gauntletCard) {
		round := &GauntletSquad{Starters: []GauntletAssignment{
			{Index: 0, Position: pos, Player: membro(1), Rating: 80},
			{Index: 1, Position: pos, Player: membro(2), Rating: 80},
			{Index: 2, Position: pos, Player: membro(3), Rating: 80},
			{Index: 3, Position: pos, Player: fora, Rating: 80},
		}}
		for _, a := range round.Starters {
			round.TotalRating += a.Rating
		}
		cards := make([]gauntletCard, len(round.Starters))
		for i, a := range round.Starters {
			cards[i] = gauntletCard{idx: i, p: a.Player, key: a.Player.PlayerKey()}
		}
		return round, cards
	}

	semPeso, cardsSemPeso := newRound()
	chemistrySwapRound(semPeso, cardsSemPeso, []gauntletCard{{idx: 100, p: candidato, key: candidato.PlayerKey()}}, m, 0)
	if semPeso.Starters[3].Player.ID != 4 {
		t.Fatalf("peso 0: esperava NENHUMA troca (compatibilidade histórica), trocou pro id %d", semPeso.Starters[3].Player.ID)
	}

	comPeso, cardsComPeso := newRound()
	sobra := chemistrySwapRound(comPeso, cardsComPeso, []gauntletCard{{idx: 100, p: candidato, key: candidato.PlayerKey()}}, m, 1.0)
	if comPeso.Starters[3].Player.ID != 5 {
		t.Fatalf("peso 1.0: esperava trocar o slot 3 pelo candidato 5 (ganha vínculo de clube), veio id %d", comPeso.Starters[3].Player.ID)
	}
	if comPeso.Starters[3].Rating != 78 {
		t.Fatalf("rating do slot trocado = %v, esperava 78", comPeso.Starters[3].Rating)
	}
	devolvida := false
	for _, c := range sobra {
		if c.p.ID == 4 {
			devolvida = true
		}
	}
	if !devolvida {
		t.Fatal("a carta deslocada (id 4) não voltou pra sobra")
	}
}

// Peso de química fim a fim: a fiação de ChemistryWeight chega até o motor
// geral (BuildGauntletPlanFromRequest), não só até chemistrySwapRound
// isolada — sem quebrar os invariantes de sempre (XI completo, sem
// duplicata). O comportamento PRECISO de uma troca já está provado, sem
// depender de acertar limiares de química através do pipeline inteiro, em
// TestChemistrySwapRoundTrocaSoQuandoPesoCompensaAPerdaDeRating.
func TestBuildGauntletPlanFromRequestAplicaPesoDeQuimicaSemQuebrarInvariantes(t *testing.T) {
	club := gauntletFixtureClub(8)
	m, err := chemistry.Escolher("fc26_vinculos")
	if err != nil {
		t.Fatal(err)
	}

	req := gauntletRequestFor(4)
	req.ChemistryModel = m
	req.ChemistryWeight = 5.0
	plan := BuildGauntletPlanFromRequest(club, req)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	for _, round := range plan.Rounds {
		if len(round.Starters) != gauntletStartersCount {
			t.Fatalf("rodada %d: %d titulares, esperava %d — peso de química não pode mudar o tamanho do XI",
				round.Round, len(round.Starters), gauntletStartersCount)
		}
		if round.Quimica == nil {
			t.Fatalf("rodada %d sem Quimica", round.Round)
		}
		seen := map[int64]bool{}
		for _, a := range round.Starters {
			if seen[a.Player.ID] {
				t.Fatalf("rodada %d: carta %d repetida depois da troca por química", round.Round, a.Player.ID)
			}
			seen[a.Player.ID] = true
		}
	}
}
