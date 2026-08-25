package analyze

import (
	"fmt"
	"testing"

	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

var gauntletFormationPositions = []domain.Position{
	domain.GK, domain.RB, domain.CB, domain.LB, domain.CDM,
	domain.CM, domain.CAM, domain.RM, domain.LM, domain.RW, domain.ST,
}

// gauntletFixtureClub monta um elenco com `perPosition` cartas só daquela
// posição para cada uma das 11 posições da formação de teste — sem
// sobreposição de posição entre cartas, para o resultado do matching global
// ficar previsível: em cada posição, os candidatos só concorrem entre si. A
// carta de índice i em cada posição tem GGRating 60+i (a de maior índice é
// a melhor daquela posição).
func gauntletFixtureClub(perPosition int) domain.Club {
	var players []domain.ClubPlayer
	var starters []domain.SquadSlot
	id := int64(1)
	for slotIdx, pos := range gauntletFormationPositions {
		for i := 0; i < perPosition; i++ {
			rating := 60.0 + float64(i)
			cp := domain.ClubPlayer{Player: domain.Player{
				ID: id, Name: fmt.Sprintf("%s-%d", pos, i), CommonName: fmt.Sprintf("%s-%d", pos, i),
				Rating: int(rating), Position: pos, League: "Liga Teste",
				GGRating: rating, GGRatingPos: pos,
				// Um jogador-base por carta: no fixture base ninguém é outra
				// versão de ninguém (quem testa isso é gauntletComDuasVersoes).
				BasePlayerEaID: id,
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

func allAssignments(rounds []GauntletSquad) []GauntletAssignment {
	var out []GauntletAssignment
	for _, r := range rounds {
		out = append(out, r.Starters...)
	}
	return out
}

// As quatro rodadas juntas usam 72 vagas (44 titulares + 28 reservas) sem
// repetir carta nenhuma — a regra oficial do Gauntlet exige elenco
// inteiramente diferente, banco incluso, a cada partida.
func TestBuildGauntletPlanMontaQuatroElencosSemRepetirCarta(t *testing.T) {
	club := gauntletFixtureClub(8) // 88 cartas: 44 titulares, ao menos 28 sobram pro banco
	plan := BuildGauntletPlan(club)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q — esperava ok", plan.Status, plan.Reason)
	}
	if len(plan.Rounds) != GauntletRounds {
		t.Fatalf("rodadas = %d, esperava %d", len(plan.Rounds), GauntletRounds)
	}

	seen := map[int64]bool{}
	total := 0
	for _, round := range plan.Rounds {
		if len(round.Starters) != gauntletStartersCount {
			t.Fatalf("rodada %d: %d titulares, esperava %d", round.Round, len(round.Starters), gauntletStartersCount)
		}
		if len(round.Bench) != gauntletBenchPerRound {
			t.Fatalf("rodada %d: %d reservas, esperava %d", round.Round, len(round.Bench), gauntletBenchPerRound)
		}
		for _, a := range round.Starters {
			if seen[a.Player.ID] {
				t.Fatalf("carta %d repetida entre as 72 vagas (titular)", a.Player.ID)
			}
			seen[a.Player.ID] = true
			total++
		}
		for _, b := range round.Bench {
			if seen[b.ID] {
				t.Fatalf("carta %d repetida entre as 72 vagas (reserva)", b.ID)
			}
			seen[b.ID] = true
			total++
		}
	}
	if total != gauntletTotalCards {
		t.Fatalf("total de cartas usadas = %d, esperava %d", total, gauntletTotalCards)
	}
}

// Todo titular precisa jogar de verdade na posição do SLOT que ocupou — não
// só na posição natural da carta.
func TestBuildGauntletPlanTitularRespeitaPosicaoDoSlot(t *testing.T) {
	club := gauntletFixtureClub(8)
	plan := BuildGauntletPlan(club)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	for _, a := range allAssignments(plan.Rounds) {
		if !a.Player.PlaysAt(a.Position) {
			t.Fatalf("titular %d escalado em %s, mas não joga lá (posição natural %s, alt %v)",
				a.Player.ID, a.Position, a.Player.Position, a.Player.AltPositions)
		}
		if got, ok := a.Player.GGRatingAt(a.Position); !ok || got != a.Rating {
			t.Fatalf("rating registrado (%v) não bate com GGRatingAt(%s) = %v/%v", a.Rating, a.Position, got, ok)
		}
	}
}

// Pegadinha documentada no CLAUDE.md (SquadSlot é lugar físico, não posição
// lógica): uma carta elegível em duas posições precisa entrar com a nota DA
// POSIÇÃO DO SLOT que ocupou, não da posição natural dela.
func TestBuildGauntletPlanTitularUsaNotaDoSlotFisicoNaoDaPosicaoNatural(t *testing.T) {
	club := gauntletFixtureClub(8)
	hybrid := domain.ClubPlayer{Player: domain.Player{
		ID: 9000, Name: "Hibrido", CommonName: "Hibrido",
		Position: domain.CB, AltPositions: []domain.Position{domain.CDM},
		GGRatings: map[domain.Position]float64{domain.CB: 50.0, domain.CDM: 99.0},
		League:    "Liga Teste",
	}}
	club.Players = append(club.Players, hybrid)

	plan := BuildGauntletPlan(club)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	var found *GauntletAssignment
	for _, a := range allAssignments(plan.Rounds) {
		if a.Player.ID == 9000 {
			a := a
			found = &a
		}
	}
	if found == nil {
		t.Fatal("híbrido não foi escalado em rodada nenhuma — esperava CDM titular, 99.0 supera os 67.0 do pool base")
	}
	if found.Position != domain.CDM {
		t.Fatalf("híbrido escalado em %s, esperava CDM (nota muito maior lá)", found.Position)
	}
	if found.Rating != 99.0 {
		t.Fatalf("rating = %v, esperava 99.0 (a nota do SLOT, não a nota de CB)", found.Rating)
	}
}

// A rodada 1 nunca pode ser mais forte que a 4 — os melhores titulares
// ficam guardados para a última partida.
func TestBuildGauntletPlanRodada1NaoEMaisForteQueRodada4(t *testing.T) {
	club := gauntletFixtureClub(8)
	plan := BuildGauntletPlan(club)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	for i := 0; i < len(plan.Rounds); i++ {
		for j := i + 1; j < len(plan.Rounds); j++ {
			if plan.Rounds[i].TotalRating > plan.Rounds[j].TotalRating {
				t.Fatalf("rodada %d (%.1f) mais forte que a rodada %d (%.1f)",
					plan.Rounds[i].Round, plan.Rounds[i].TotalRating, plan.Rounds[j].Round, plan.Rounds[j].TotalRating)
			}
		}
	}
	if plan.Rounds[0].TotalRating == plan.Rounds[3].TotalRating {
		t.Fatal("rodada 1 e rodada 4 saíram com a mesma força — fixture devia produzir crescimento real")
	}
}

// Reserva nunca pode superar, numa posição em que joga, o titular mais
// fraco daquela mesma posição — senão devia ter sido titular ela mesma.
func TestBuildGauntletPlanReservaNaoDeslocaTitularSuperior(t *testing.T) {
	club := gauntletFixtureClub(8)
	plan := BuildGauntletPlan(club)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}

	weakestStarterAt := map[domain.Position]float64{}
	for _, a := range allAssignments(plan.Rounds) {
		if cur, ok := weakestStarterAt[a.Position]; !ok || a.Rating < cur {
			weakestStarterAt[a.Position] = a.Rating
		}
	}

	for _, round := range plan.Rounds {
		for _, b := range round.Bench {
			for pos, weakest := range weakestStarterAt {
				if !b.PlaysAt(pos) {
					continue
				}
				if r, ok := b.GGRatingAt(pos); ok && r > weakest {
					t.Fatalf("reserva %d (%.1f em %s) supera o titular mais fraco daquela posição (%.1f) — devia ter sido titular",
						b.ID, r, pos, weakest)
				}
			}
		}
	}
}

// BuildGauntletPlan preenche a química de cada rodada — Icon/Hero e o resto
// da regra em si já são cobertos a fundo em internal/chemistry; aqui o que
// importa é a FIAÇÃO: cada GauntletSquad.Quimica bate com o que
// chemistry.Calcular daria para os mesmos 11 titulares.
func TestBuildGauntletPlanPreencheQuimicaPorRodada(t *testing.T) {
	club := gauntletFixtureClub(8)
	plan := BuildGauntletPlan(club)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	for _, round := range plan.Rounds {
		if round.Quimica == nil {
			t.Fatalf("rodada %d sem Quimica", round.Round)
		}
		xi := make([]chemistry.Titular, len(round.Starters))
		for i, a := range round.Starters {
			xi[i] = chemistry.Titular{Index: a.Index, Position: a.Position, Player: a.Player.Player}
		}
		esperado := chemistry.Calcular(chemistry.ModeloPadrao(), xi)
		if round.Quimica.Total != esperado.Total {
			t.Fatalf("rodada %d: Quimica.Total = %d, esperava %d", round.Round, round.Quimica.Total, esperado.Total)
		}
	}
}

func TestBuildGauntletPlanSinalizaFormacaoNaoSincronizada(t *testing.T) {
	club := domain.Club{Players: []domain.ClubPlayer{{Player: domain.Player{ID: 1, GGRating: 90}}}}
	plan := BuildGauntletPlan(club)
	if plan.Status != "unavailable" || plan.Reason == "" {
		t.Fatalf("esperava unavailable com motivo, veio %+v", plan)
	}
}

// Clube sem as 72 cartas elegíveis precisa avisar claramente, em vez de
// montar rodadas incompletas ou repetir carta.
func TestBuildGauntletPlanSinalizaClubeComPoucasCartas(t *testing.T) {
	club := gauntletFixtureClub(2) // 22 cartas, bem abaixo de 72
	plan := BuildGauntletPlan(club)
	if plan.Status != "unavailable" {
		t.Fatalf("status = %q, esperava unavailable", plan.Status)
	}
	if plan.Reason == "" {
		t.Fatal("esperava um motivo explicando a falta de cartas elegíveis")
	}
}

func TestStarterIDsListaSemRepetirDeTodasAsRodadas(t *testing.T) {
	club := gauntletFixtureClub(8)
	plan := BuildGauntletPlan(club)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	ids := plan.StarterIDs()
	if len(ids) != GauntletRounds*gauntletStartersCount {
		t.Fatalf("StarterIDs devolveu %d ids, esperava %d", len(ids), GauntletRounds*gauntletStartersCount)
	}
	seen := map[int64]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("id %d repetido em StarterIDs", id)
		}
		seen[id] = true
	}
}

// gauntletComDuasVersoes reproduz o caso relatado: duas cartas do MESMO
// jogador (mesmo BasePlayerEaID, ids de carta diferentes), cada uma sendo a
// melhor da sua posição — foi assim que o Mbappé apareceu de LM e de RM na
// mesma rodada. O que separa as duas é só a versão da carta, e o jogo não
// aceita as duas no mesmo elenco.
func gauntletComDuasVersoes() domain.Club {
	club := gauntletFixtureClub(8)
	versao := func(id int64, nome string, pos domain.Position, gg float64) domain.ClubPlayer {
		return domain.ClubPlayer{Player: domain.Player{
			ID: id, Name: nome, CommonName: nome, Rating: 99, Position: pos,
			League: "Liga Teste", GGRating: gg, GGRatingPos: pos,
			BasePlayerEaID: 777,
		}}
	}
	club.Players = append(club.Players,
		versao(9001, "Mbappé TOTS", domain.RM, 99.0),
		versao(9002, "Mbappé Ouro", domain.LM, 98.0))
	return club
}

func versoesPorRodada(round GauntletSquad) map[int64][]int64 {
	porBase := map[int64][]int64{}
	for _, a := range round.Starters {
		porBase[a.Player.BasePlayerEaID] = append(porBase[a.Player.BasePlayerEaID], a.Player.ID)
	}
	for _, b := range round.Bench {
		porBase[b.BasePlayerEaID] = append(porBase[b.BasePlayerEaID], b.ID)
	}
	return porBase
}

// O bug relatado: duas versões do mesmo jogador escaladas na mesma rodada
// (Mbappé de LM e de RM). O jogo não aceita duas cartas do mesmo atleta num
// elenco — e a regra vale para o banco também.
func TestGauntletNaoEscalaDuasVersoesDoMesmoJogadorNaMesmaRodada(t *testing.T) {
	plan := BuildGauntletPlan(gauntletComDuasVersoes())
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	for _, round := range plan.Rounds {
		for base, ids := range versoesPorRodada(round) {
			if len(ids) > 1 {
				t.Fatalf("rodada %d: jogador-base %d entrou %d vezes (cartas %v) — o jogo não aceita duas versões do mesmo jogador no mesmo elenco",
					round.Round, base, len(ids), ids)
			}
		}
	}
}

// A trava é POR RODADA, não global: cada rodada é um elenco diferente, então
// o Mbappé TOTS numa e o Mbappé ouro noutra são duas escalações legais — e
// desperdiçar a segunda carta seria pior que o bug.
func TestGauntletUsaVersoesDoMesmoJogadorEmRodadasDiferentes(t *testing.T) {
	plan := BuildGauntletPlan(gauntletComDuasVersoes())
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	rodadaDe := map[int64]int{}
	for _, round := range plan.Rounds {
		for _, a := range round.Starters {
			rodadaDe[a.Player.ID] = round.Round
		}
	}
	r1, ok1 := rodadaDe[9001]
	r2, ok2 := rodadaDe[9002]
	if !ok1 || !ok2 {
		t.Fatalf("as duas versões deviam ser titulares em alguma rodada (99.0 e 98.0 contra 67.0 do pool base), escaladas = %v", rodadaDe)
	}
	if r1 == r2 {
		t.Fatalf("as duas versões caíram na mesma rodada (%d) — era exatamente o bug", r1)
	}
}

// Ter duas CÓPIAS da mesma carta é normal no FUT, e o elenco real tem
// várias. Elas têm o mesmo Player.ID, então o pool as trata como um jogador
// só — e uma rodada não pode escalar as duas.
func TestGauntletNaoRepeteCopiaDaMesmaCartaNaMesmaRodada(t *testing.T) {
	club := gauntletFixtureClub(8)
	copia := domain.ClubPlayer{Player: domain.Player{
		ID: 9500, Name: "Cópia", CommonName: "Cópia", Rating: 99,
		Position: domain.RM, AltPositions: []domain.Position{domain.LM},
		League: "Liga Teste",
		// Sem BasePlayerEaID de propósito: a carta vira chave de si mesma, que
		// é o que faz as duas cópias colidirem (ver domain.Player.PlayerKey).
		GGRatings: map[domain.Position]float64{domain.RM: 99.0, domain.LM: 98.0},
	}}
	club.Players = append(club.Players, copia, copia)

	plan := BuildGauntletPlan(club)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	for _, round := range plan.Rounds {
		vezes := 0
		for _, a := range round.Starters {
			if a.Player.ID == 9500 {
				vezes++
			}
		}
		for _, b := range round.Bench {
			if b.ID == 9500 {
				vezes++
			}
		}
		if vezes > 1 {
			t.Fatalf("rodada %d escalou a mesma carta %d vezes", round.Round, vezes)
		}
	}
}

// Sem basePlayerEaId o bot não tem como saber se duas cartas são o mesmo
// atleta. Ele não chuta pelo nome (CLAUDE.md: na dúvida, não afirma) — mas
// também não pode deixar o buraco invisível.
func TestGauntletAvisaCartaSemChaveDeJogador(t *testing.T) {
	club := gauntletFixtureClub(8)
	for i := range club.Players {
		club.Players[i].BasePlayerEaID = 0
	}
	plan := BuildGauntletPlan(club)
	if plan.Status != "ok" {
		t.Fatalf("status = %q, motivo = %q", plan.Status, plan.Reason)
	}
	if len(plan.Warnings) == 0 {
		t.Fatal("esperava aviso de que a trava de um-jogador-por-elenco não alcança cartas sem id de jogador-base")
	}

	if len(BuildGauntletPlan(gauntletFixtureClub(8)).Warnings) != 0 {
		t.Fatal("elenco com todos os ids de jogador-base preenchidos não devia gerar aviso nenhum")
	}
}
