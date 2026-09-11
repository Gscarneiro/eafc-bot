package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/config"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
	"github.com/gscarneiro/eafc-bot/internal/report"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

// cmdDemo roda o pipeline inteiro com dados fictícios e sem rede. Serve
// para conferir o motor de análise e o layout do relatório antes de
// qualquer endpoint estar calibrado.
func cmdDemo(args []string) error {
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	outPath := fs.String("out", "briefing-demo.html", "onde gravar o relatório")
	seed := fs.Int64("seed", 26, "semente do gerador, para reproduzir o mesmo elenco")
	if err := fs.Parse(args); err != nil {
		return err
	}

	started := time.Now()
	rng := rand.New(rand.NewSource(*seed))

	snap := demoSnapshot(rng)
	cfg := config.Default()
	cfg.GamerTag = snap.Club.GamerTag

	// O elenco fictício do demo (15 cartas) fica bem abaixo das 72 que o
	// Gauntlet exige — BuildGauntletPlan sinaliza "unavailable" sozinho,
	// exercitando o mesmo caminho que um clube pequeno de verdade veria.
	gauntletPlan := analyze.BuildGauntletPlanWithOptions(snap.Club, analyze.GauntletOptions{ChemistryModel: cfg.ChemistryModel()})
	data, err := analyzeAndBuild(context.Background(), cfg, nil, snap, started, true, nil, gauntletPlan)
	if err != nil {
		return err
	}
	// Preenche as seções que dependem de histórico, para o demo mostrar
	// o relatório completo.
	data.NewCards = snap.Market[:3]
	data.Market = demoMarketRows(snap, rng)
	data.TrendWindow = "72 horas"

	// Mesmo motivo das linhas acima: analyzeAndBuild com dryRun=true e
	// st=nil não tem de onde ler momentum nem tendência de custo de SBC
	// (vêm do Store, não do snapshot) — monta na mão pro demo mostrar a
	// seção de investimentos completa, inclusive o sinal de out-of-packs
	// (Doué ganha uma versão nova em demoNewCards).
	investments, invFunnel := analyze.FindInvestments(
		snap.Club, demoMomentum(), demoNewCards(), analyze.DefaultInvestmentOptions())
	data.Investments, data.InvestmentFunnel = investments, invFunnel
	sellCandidates, sellFunnel := analyze.FindSellCandidates(
		snap.Club, demoCardReports(snap.Club), data.SquadSwaps, analyze.DefaultSellOptions())
	data.SellCandidates, data.SellFunnel = sellCandidates, sellFunnel
	data.FodderDemand = analyze.FindFodderDemand(
		snap.SBCs, snap.Market, demoSBCCostTrends(), analyze.DefaultFodderDemandOptions())

	if err := writeReport(*outPath, data); err != nil {
		return err
	}

	fmt.Printf("relatório de exemplo: %s\n", *outPath)
	fmt.Printf("  %d upgrades · %d evoluções · %d SBCs · %d investimentos · %d sinais de fodder\n",
		len(data.Upgrades), len(data.Evolutions), len(data.SBCs), len(data.Investments), len(data.FodderDemand))
	return nil
}

func demoSnapshot(rng *rand.Rand) *futgg.Snapshot {
	club := demoClub()
	market := demoMarket()
	evolutions := demoEvolutions()
	sbcs := demoSBCs()
	objectives := demoObjectives()
	news := demoNews()
	return &futgg.Snapshot{
		Club:       club,
		Market:     market,
		Evolutions: evolutions,
		SBCs:       sbcs,
		Objectives: objectives,
		News:       news,
		Stats:      futgg.Stats{Requests: 14, CacheHits: 3, Retries: 1, Bytes: 2_400_000},
		// Capabilities fictícia, coerente com o resto do dado sintético: o
		// modo demo não passa por Collect() (não há rede), então nada aqui
		// vem de fato de "futgg" — mas /api/saude precisa de ALGUM contrato
		// pra exercitar, senão a tela cairia sempre no ramo de snapshot
		// legado (ver serve -demo no CLAUDE.md).
		Capabilities: map[string]futgg.Observation{
			"clube":     {Source: "demo", ObservedAt: time.Now(), Coverage: len(club.Players), Status: futgg.StatusConfirmado},
			"mercado":   {Source: "demo", ObservedAt: time.Now(), Coverage: len(market), Status: futgg.StatusConfirmado},
			"evoluções": {Source: "demo", ObservedAt: time.Now(), Coverage: len(evolutions), Status: futgg.StatusConfirmado},
			"objetivos": {Source: "demo", ObservedAt: time.Now(), Coverage: len(objectives), Status: futgg.StatusConfirmado},
			"SBCs":      {Source: "demo", ObservedAt: time.Now(), Coverage: len(sbcs), Status: futgg.StatusConfirmado},
			"notícias":  {Source: "demo", ObservedAt: time.Now(), Coverage: len(news), Status: futgg.StatusConfirmado},
		},
	}
}

func p(id int64, name string, rating int, pos domain.Position, version string,
	pac, sho, pas, dri, def, phy int, price int, styles ...domain.PlayStyle) domain.Player {
	return domain.Player{
		ID: id, Name: name, CommonName: name, Rating: rating, Position: pos, Version: version,
		Attributes: domain.Attributes{Pace: pac, Shooting: sho, Passing: pas,
			Dribbling: dri, Defending: def, Physical: phy},
		PlayStyles: styles,
		WeakFoot:   4, SkillMoves: 4,
		Price: domain.Price{Coins: price, UpdatedAt: time.Now()},
		Cycle: "26",
		// O demo precisa de GG posicional confirmado para a comparação do XI;
		// overall continua sendo só o dado de entrada, como na coleta real.
		GGRating: float64(rating) - 1.2, GGRatingPos: pos,
		GGRatings: map[domain.Position]float64{pos: float64(rating) - 1.2},
	}
}

func ps(name string, plus bool) domain.PlayStyle { return domain.PlayStyle{Name: name, Plus: plus} }

// nat preenche clube/liga/nação depois de p() — os dois vêm separados porque
// p() já tem 12 parâmetros posicionais e é reusado por demoMarket() também,
// onde clube/liga/nação não fazem falta (só o XI ativo entra na conta de
// entrosamento). O elenco abaixo tem vínculo de propósito (3 do AC Milan,
// 4 da Premier League, 4 da Serie A, 3 franceses) para o modo demo exercitar
// a regra de vínculo de verdade, não só o modelo padrão (que preenche a
// barra pela posição sozinha — ver internal/chemistry).
func nat(pl domain.Player, club, league, nation string) domain.Player {
	pl.Club, pl.League, pl.Nation = club, league, nation
	return pl
}

func demoClub() domain.Club {
	players := []domain.ClubPlayer{
		{Player: nat(p(1, "Maignan", 87, domain.GK, "Ouro Raro", 86, 84, 70, 88, 45, 82, 38_000, ps("Far Reach", true)), "AC Milan", "Serie A Enilive", "France")},
		{Player: nat(p(2, "Frimpong", 84, domain.RB, "Ouro Raro", 94, 70, 76, 84, 78, 74, 22_000, ps("Quick Step", false)), "Bayer Leverkusen", "Bundesliga", "Netherlands")},
		{Player: nat(p(3, "Saliba", 86, domain.CB, "Ouro Raro", 84, 40, 65, 72, 87, 84, 46_000, ps("Anticipate", true)), "Arsenal", "Premier League", "France")},
		{Player: nat(p(4, "Gvardiol", 85, domain.CB, "Ouro Raro", 82, 55, 72, 75, 85, 83, 34_000, ps("Block", false)), "Manchester City", "Premier League", "Croatia")},
		{Player: nat(p(5, "T. Hernández", 85, domain.LB, "Ouro Raro", 93, 76, 79, 84, 79, 82, 41_000, ps("Rapid", false)), "AC Milan", "Serie A Enilive", "France")},
		{Player: nat(p(6, "Rodri", 89, domain.CDM, "Ouro Raro", 68, 78, 86, 82, 87, 85, 128_000, ps("Anticipate", false), ps("Press Proven", true)), "Manchester City", "Premier League", "Spain")},
		{Player: nat(p(7, "Valverde", 88, domain.CM, "Ouro Raro", 85, 84, 85, 84, 79, 86, 96_000, ps("Relentless", true)), "Real Madrid", "LALIGA EA SPORTS", "Uruguay")},
		{Player: nat(p(8, "Wirtz", 87, domain.CAM, "Ouro Raro", 82, 82, 87, 89, 45, 68, 74_000, ps("Technical", true)), "Bayer Leverkusen", "Bundesliga", "Germany")},
		{Player: nat(p(9, "Saka", 87, domain.RW, "Ouro Raro", 88, 84, 82, 88, 52, 72, 88_000, ps("Trickster", false)), "Arsenal", "Premier League", "England")},
		{Player: nat(p(10, "Leão", 86, domain.LW, "Ouro Raro", 95, 82, 76, 87, 38, 78, 62_000, ps("Rapid", true)), "AC Milan", "Serie A Enilive", "Portugal")},
		{Player: nat(p(11, "Osimhen", 88, domain.ST, "Ouro Raro", 90, 87, 70, 84, 42, 85, 118_000, ps("Power Shot", false), ps("Aerial", true)), "Napoli", "Serie A Enilive", "Nigeria")},
		// Reservas: dão para vender e financiar as trocas.
		{Player: p(12, "Reijnders", 84, domain.CM, "Ouro Raro", 78, 76, 83, 84, 72, 76, 18_000)},
		{Player: p(13, "Kolo Muani", 83, domain.ST, "Ouro Raro", 89, 81, 72, 80, 40, 82, 14_000)},
		// Reserva 88+ para a Análise demonstrar uma evolução que toma a vaga
		// do XI sem depender de uma coleta externa.
		{Player: p(16, "J. David", 88, domain.ST, "Ouro Raro", 88, 86, 75, 86, 43, 78, 54_000, ps("Finesse Shot", false))},
		// Base barata e sem PlayStyle+ — o alvo natural das evoluções.
		{Player: p(14, "Yildiz", 79, domain.LW, "Ouro Raro", 86, 76, 77, 85, 32, 65, 3_200)},
		{Player: p(15, "Zaïre-Emery", 80, domain.CM, "Ouro Raro", 80, 71, 79, 82, 76, 78, 4_100)},
	}

	var starters []domain.SquadSlot
	for i, cp := range players {
		if i < 11 {
			players[i].InSquad = true
			players[i].SquadSlot = cp.Position
			players[i].Chemistry = 3
			starters = append(starters, domain.SquadSlot{Index: i, Position: cp.Position, PlayerID: cp.ID})
		}
	}
	for i := range players {
		players[i].DetailedAttributes = demoDetailedAttributes(players[i].Player)
	}
	// Untradeable típico de recompensa: entra no time mas não vira moeda.
	players[6].Untradeable = true

	return domain.Club{
		GamerTag: "carneiro22", Platform: "ps5", Coins: 145_000,
		Players: players,
		// Chemistry 33 = 11 titulares x 3 (o mesmo padrão observado no
		// elenco real, ver internal/chemistry/modelos.go). ChemistrySynced
		// true porque, no demo, "a coleta" é sempre completa por definição.
		Squad: domain.Squad{Name: "Titular", Formation: "4-2-3-1", Chemistry: 33, ChemistrySynced: true, Starters: starters, SyncedAt: time.Now()},
		Cycle: "26", SyncedAt: time.Now(), Source: "demo",
	}
}

func demoDetailedAttributes(player domain.Player) *domain.DetailedAttributes {
	acc, sprint := player.Attributes.Pace-2, player.Attributes.Pace+1
	agility, balance := player.Attributes.Dribbling-3, player.Attributes.Dribbling-5
	stamina, strength := player.Attributes.Physical-2, player.Attributes.Physical
	vision, shortPass := player.Attributes.Passing-2, player.Attributes.Passing+2
	finishing, shotPower := player.Attributes.Shooting-1, player.Attributes.Shooting+3
	defensive, stand := player.Attributes.Defending-2, player.Attributes.Defending+1
	return &domain.DetailedAttributes{
		Acceleration: &acc, SprintSpeed: &sprint, Agility: &agility, Balance: &balance,
		Stamina: &stamina, Strength: &strength, Vision: &vision, ShortPassing: &shortPass,
		Finishing: &finishing, ShotPower: &shotPower, DefensiveAwareness: &defensive, StandingTackle: &stand,
	}
}

// demoCardReports monta à mão um punhado de cards.CardReport para o modo
// demo. cards.BuildReports precisa de um *futgg.Client de verdade (Roles e
// EvolutionPaths batem na rede), e o demo é sem rede por definição — sem
// isto snap.Cards fica vazio e /api/time/{slug} sempre devolve 404 aqui
// (era o aviso que serveDemo imprimia). Cobre os estados reais da tela de
// carta: caminho disponível com cadeia de 3 evoluções pra alimentar o
// pipeline de progresso (Osimhen, alternate de 3 passos; J. David, Best de
// 3 passos), já no teto (Rodri, Best nil — resposta válida, não ausência de
// dado), falha de coleta (Reijnders, EvolutionFetchError) e um grafo
// confirmado com ramificação E reencontro de verdade (Yildiz) — o único
// visual do Workbench que os outros quatro não sustentam sozinhos.
func demoCardReports(club domain.Club) []cards.CardReport {
	byID := make(map[int64]domain.ClubPlayer, len(club.Players))
	for _, cp := range club.Players {
		byID[cp.ID] = cp
	}
	osimhen, rodri, david, reijnders, yildiz := byID[11], byID[6], byID[16], byID[12], byID[14]

	evoluido := osimhen.Player
	evoluido.Rating = 90
	evoluido.GGRating, evoluido.GGRatingPos = 90.5, domain.ST
	evoluido.GGRatings = map[domain.Position]float64{domain.ST: 90.5}
	alternativo := evoluido
	alternativo.GGRating = 91.0
	alternativo.GGRatings = map[domain.Position]float64{domain.ST: 91.0}
	// alternativo3 é o único caminho de 3 evoluções encadeadas do demo além
	// do de David — dá pra /api/evolucoes/progresso mostrar "1 de 3" ao lado
	// do "3 de 3" de David, em vez de dois caminhos de um passo só.
	alternativo3 := evoluido
	alternativo3.Rating, alternativo3.GGRating = 91, 91.6
	alternativo3.GGRatings = map[domain.Position]float64{domain.ST: 91.6}
	davidFinal := david.Player
	davidFinal.Rating, davidFinal.GGRating, davidFinal.GGRatingPos = 92, 93.2, domain.ST
	davidFinal.GGRatings = map[domain.Position]float64{domain.ST: 93.2}

	return []cards.CardReport{
		{
			Slug:            "osimhen-88",
			Player:          osimhen,
			EvolutionStatus: cards.EvolutionConfirmed,
			ByPosition:      []cards.PositionRoles{{Position: domain.ST, PlusPlus: []string{"Finalizador"}, Plus: []string{"Alvo Avançado"}}},
			Best: &cards.EvoPotential{
				Path: domain.EvolutionPath{
					Steps:        []domain.Player{osimhen.Player, evoluido},
					Chain:        []string{"Ponta Explosiva"},
					CoinsCost:    25_000,
					TrainingTime: "2 dias",
				},
				FinalOverall:     90,
				FinalGGRating:    90.5,
				GGRatingGain:     3.5,
				GainedPlayStyles: []domain.PlayStyle{{Name: "Power Shot", Plus: true}},
				CoinsCost:        25_000,
				TrainingTime:     "2 dias",
			},
			Alternates: []cards.EvoPotential{
				{
					Path:         domain.EvolutionPath{Steps: []domain.Player{osimhen.Player, alternativo}, Chain: []string{"Ponta Explosiva", "Finalizador Livre"}, CoinsCost: 45_000, TrainingTime: "4 dias"},
					FinalOverall: 91, FinalGGRating: 91.0, GGRatingGain: 4.2, CoinsCost: 45_000, TrainingTime: "4 dias",
				},
				{
					Path:         domain.EvolutionPath{Steps: []domain.Player{osimhen.Player, alternativo3}, Chain: []string{"Ponta Explosiva", "Finalizador Livre", "Artilheiro Nato"}, CoinsCost: 60_000, TrainingTime: "6 dias"},
					FinalOverall: 91, FinalGGRating: 91.6, GGRatingGain: 4.8, CoinsCost: 60_000, TrainingTime: "6 dias",
				},
			},
		},
		{
			Slug:            "rodri-89",
			Player:          rodri,
			EvolutionStatus: cards.EvolutionNoPath,
			ByPosition:      []cards.PositionRoles{{Position: domain.CDM, PlusPlus: []string{"Âncora"}, Plus: []string{"Armador Recuado"}}},
			Best:            nil,
		},
		{
			Slug: "j-david-88", Player: david, EvolutionStatus: cards.EvolutionConfirmed,
			Best: &cards.EvoPotential{
				Path:         domain.EvolutionPath{Steps: []domain.Player{david.Player, davidFinal}, Chain: []string{"Caçador de Área", "Artilheiro Implacável", "Matador de Área"}, CoinsCost: 18_000, TrainingTime: "3 dias"},
				FinalOverall: 92, FinalGGRating: 93.2, GGRatingGain: 6.4, CoinsCost: 18_000, TrainingTime: "3 dias",
			},
		},
		{
			Slug:            "reijnders-84",
			Player:          reijnders,
			EvolutionStatus: cards.EvolutionFetchError,
			EvolutionError:  "timeout consultando o catálogo de evoluções do fut.gg",
		},
		demoBranchingCardReport(yildiz),
	}
}

// demoBranchingCardReport monta o único grafo confirmado do demo com
// ramificação E reencontro reais — a fixture de hoje (antes desta função)
// só tinha Best/Alternates lineares, então o Workbench (branch/rejoin, ver
// EvolutionGraph.IsBranch/IsRejoin em internal/domain) nunca tinha dado pra
// mostrar o próprio motivo de existir. Yildiz (79 LW, sem PlayStyle+) é "o
// alvo natural das evoluções" no resto da fixture — cabe bem aqui também.
func demoBranchingCardReport(yildiz domain.ClubPlayer) cards.CardReport {
	root := yildiz.Player
	branchA := root
	branchA.Rating, branchA.GGRating = 81, 80.6
	branchA.GGRatings = map[domain.Position]float64{domain.LW: 80.6}
	branchB := root
	branchB.Rating, branchB.GGRating = 81, 80.2
	branchB.GGRatings = map[domain.Position]float64{domain.LW: 80.2}
	final := root
	final.Rating, final.GGRating = 84, 84.6
	final.GGRatings = map[domain.Position]float64{domain.LW: 84.6}

	graph := &domain.EvolutionGraph{
		Cycle: "26", RootID: "root",
		Nodes: map[string]domain.EvolutionNode{
			"root":  {ID: "root", Card: root},
			"a":     {ID: "a", Card: branchA},
			"b":     {ID: "b", Card: branchB},
			"final": {ID: "final", Card: final},
		},
		Transitions: []domain.EvolutionTransition{
			// root se bifurca em "a" e "b" (IsBranch(root) == true).
			{From: "root", To: "a", Evolution: "Ala Veloz", CoinsCost: 0, TrainingTime: "2 dias"},
			{From: "root", To: "b", Evolution: "Drible Refinado", CoinsCost: 8_000, TrainingTime: "3 dias"},
			// "a" e "b" convergem em "final" (IsRejoin(final) == true).
			{From: "a", To: "final", Evolution: "Finalizador Ágil", CoinsCost: 22_000, TrainingTime: "4 dias"},
			{From: "b", To: "final", Evolution: "Camisa 10", CoinsCost: 15_000, TrainingTime: "3 dias", Repeatable: true},
		},
	}

	return cards.CardReport{
		Slug: "yildiz-79", Player: yildiz, EvolutionStatus: cards.EvolutionConfirmed,
		ByPosition: []cards.PositionRoles{{Position: domain.LW, Plus: []string{"Ala Invertida"}}},
		Best: &cards.EvoPotential{
			Path:         domain.EvolutionPath{Steps: []domain.Player{root, branchB, final}, Chain: []string{"Drible Refinado", "Camisa 10"}, CoinsCost: 23_000, TrainingTime: "6 dias"},
			FinalOverall: 84, FinalGGRating: 84.6, GGRatingGain: 84.6 - root.GGRating, CoinsCost: 23_000, TrainingTime: "6 dias",
		},
		Alternates: []cards.EvoPotential{{
			Path:         domain.EvolutionPath{Steps: []domain.Player{root, branchA, final}, Chain: []string{"Ala Veloz", "Finalizador Ágil"}, CoinsCost: 22_000, TrainingTime: "6 dias"},
			FinalOverall: 84, FinalGGRating: 84.6, GGRatingGain: 84.6 - root.GGRating, CoinsCost: 22_000, TrainingTime: "6 dias",
		}},
		Graph: graph,
	}
}

func demoMarket() []domain.Player {
	return []domain.Player{
		p(101, "Doué", 88, domain.RW, "TOTS", 93, 85, 84, 92, 48, 72, 210_000, ps("Rapid", true), ps("Trickster", true)),
		p(102, "Nico Williams", 87, domain.LW, "TOTS", 96, 83, 78, 89, 42, 74, 165_000, ps("Rapid", true), ps("Quick Step", true)),
		p(103, "Guéhi", 87, domain.CB, "TOTS", 86, 42, 68, 74, 88, 86, 71_000, ps("Anticipate", true), ps("Block", true)),
		p(104, "Dorgu", 84, domain.LB, "TOTS", 97, 72, 76, 84, 80, 76, 33_000, ps("Rapid", true), ps("Quick Step", false)),
		p(105, "Ekitiké", 88, domain.ST, "TOTS", 92, 88, 76, 87, 40, 80, 245_000, ps("Finesse Shot", true), ps("Rapid", false)),
		p(106, "Kvaratskhelia", 89, domain.LW, "Herói", 91, 85, 83, 93, 45, 74, 480_000, ps("Trickster", true), ps("Technical", true)),
		p(107, "Nunes", 85, domain.CM, "TOTS", 88, 80, 82, 85, 78, 88, 52_000, ps("Press Proven", true), ps("Relentless", true)),
		p(108, "Bastoni", 88, domain.CB, "TOTS", 83, 48, 78, 77, 89, 85, 96_000, ps("Anticipate", true)),
		p(109, "Hato", 82, domain.CB, "Ouro Raro", 85, 38, 70, 74, 82, 78, 8_500, ps("Anticipate", false)),
		p(110, "Wahi", 82, domain.ST, "Ouro Raro", 91, 80, 68, 82, 38, 79, 6_800, ps("Rapid", false)),
		p(111, "Anton", 84, domain.CB, "TOTW", 82, 40, 66, 71, 86, 87, 24_000, ps("Bruiser", true)),
		p(112, "Olise", 88, domain.RW, "TOTS", 89, 86, 87, 91, 46, 70, 315_000, ps("Finesse Shot", true), ps("Technical", true)),
	}
}

func demoEvolutions() []domain.Evolution {
	return []domain.Evolution{
		{
			ID: "e1", Slug: "ponta-explosiva", Name: "Ponta Explosiva",
			Description: "Ritmo e drible para uma ponta abaixo de 82.",
			CoinCost:    25_000,
			ExpiresAt:   time.Now().Add(60 * time.Hour),
			Requirements: []domain.EvoRequirement{
				{Kind: "max_overall", IntValue: 81, Raw: "Overall máximo: 81"},
				{Kind: "position", Strings: []string{"LW", "RW", "LM", "RM"}, Raw: "Posição: LW, RW, LM, RM"},
			},
			Levels: []domain.EvoLevel{
				{Number: 1, Upgrades: []domain.EvoUpgrade{
					{Kind: "overall", Amount: 3}, {Kind: "attribute", Attr: "pac", Amount: 7},
					{Kind: "attribute", Attr: "dri", Amount: 6},
				}, Objectives: []string{"Marque 4 gols em Rivals"}},
				{Number: 2, Upgrades: []domain.EvoUpgrade{
					{Kind: "overall", Amount: 3}, {Kind: "attribute", Attr: "sho", Amount: 8},
					{Kind: "playstyle", PlayStyle: ps("Rapid", true)},
					{Kind: "playstyle", PlayStyle: ps("Trickster", false)},
				}, Objectives: []string{"Dê 3 assistências"}},
			},
			URL: "https://www.fut.gg/evolutions/ponta-explosiva/",
		},
		{
			ID: "e2", Slug: "motor-do-meio", Name: "Motor do Meio",
			Description: "Transforma um volante mediano em box-to-box.",
			CoinCost:    0, PointCost: 0,
			ExpiresAt: time.Now().Add(20 * time.Hour),
			Requirements: []domain.EvoRequirement{
				{Kind: "max_overall", IntValue: 82, Raw: "Overall máximo: 82"},
				{Kind: "position", Strings: []string{"CM", "CDM"}, Raw: "Posição: CM, CDM"},
				{Kind: "max_playstyles_plus", IntValue: 1, Raw: "Máximo de PlayStyles+: 1"},
			},
			Levels: []domain.EvoLevel{
				{Number: 1, Upgrades: []domain.EvoUpgrade{
					{Kind: "overall", Amount: 4}, {Kind: "attribute", Attr: "pac", Amount: 6},
					{Kind: "attribute", Attr: "phy", Amount: 6}, {Kind: "attribute", Attr: "def", Amount: 5},
					{Kind: "playstyle", PlayStyle: ps("Press Proven", true)},
					{Kind: "position", Position: domain.CDM},
				}, Objectives: []string{"Jogue 5 partidas"}},
			},
			URL: "https://www.fut.gg/evolutions/motor-do-meio/",
		},
		{
			ID: "e3", Slug: "muralha", Name: "Muralha",
			Description: "Zagueiro rápido e agressivo.",
			CoinCost:    50_000,
			ExpiresAt:   time.Now().Add(120 * time.Hour),
			Requirements: []domain.EvoRequirement{
				{Kind: "max_overall", IntValue: 84, Raw: "Overall máximo: 84"},
				{Kind: "position", Strings: []string{"CB"}, Raw: "Posição: CB"},
			},
			Levels: []domain.EvoLevel{
				{Number: 1, Upgrades: []domain.EvoUpgrade{
					{Kind: "overall", Amount: 4}, {Kind: "attribute", Attr: "def", Amount: 6},
					{Kind: "attribute", Attr: "pac", Amount: 5}, {Kind: "attribute", Attr: "phy", Amount: 5},
					{Kind: "playstyle", PlayStyle: ps("Anticipate", true)},
				}, Objectives: []string{"Vença 3 partidas"}},
			},
			URL: "https://www.fut.gg/evolutions/muralha/",
		},
		{
			ID: "e4", Slug: "recompensa-futties", Name: "FUTTIES 5th Rapid+",
			Description: "Recompensa de objetivo com PlayStyle+.", IsRewardEvolution: true,
			ObjectiveGroupName: "FUTTIES 5th", ExpiresAt: time.Now().Add(96 * time.Hour),
			Levels: []domain.EvoLevel{{Number: 1, Upgrades: []domain.EvoUpgrade{{Kind: "playstyle", PlayStyle: ps("Rapid", true)}, {Kind: "attribute", Attr: "pac", Amount: 2}}}},
			URL:    "https://www.fut.gg/evolutions/recompensa-futties/",
		},
		{
			ID: "e5", Slug: "tiki-taka-lab", Name: "Tiki Taka",
			Description: "PlayStyles Lab: adicione Tiki Taka.", CategoryName: "PlayStyles Lab", CategorySlug: "playstyle-lab",
			ExpiresAt: time.Now().Add(72 * time.Hour),
			Levels:    []domain.EvoLevel{{Number: 1, Upgrades: []domain.EvoUpgrade{{Kind: "playstyle", PlayStyle: ps("Tiki Taka", false)}}}},
			URL:       "https://www.fut.gg/evolutions/tiki-taka-lab/",
		},
		{
			ID: "e6", Slug: "tiki-taka-plus-lab", Name: "Tiki Taka+",
			Description: "PlayStyles Lab: eleve Tiki Taka a PlayStyle+.", CategoryName: "PlayStyles Lab", CategorySlug: "playstyle-lab",
			ExpiresAt: time.Now().Add(72 * time.Hour),
			Levels:    []domain.EvoLevel{{Number: 1, Upgrades: []domain.EvoUpgrade{{Kind: "playstyle", PlayStyle: ps("Tiki Taka", true)}}}},
			URL:       "https://www.fut.gg/evolutions/tiki-taka-plus-lab/",
		},
		{
			ID: "e7", Slug: "training-camp-finishing", Name: "Training Camp: Finalização",
			Description: "Training Camp Evolutions: sessão curta de treino.", TotalTrainingTime: 86400,
			ExpiresAt: time.Now().Add(44 * time.Hour),
			Levels:    []domain.EvoLevel{{Number: 1, Upgrades: []domain.EvoUpgrade{{Kind: "attribute", Attr: "sho", Amount: 2}, {Kind: "ignored", Attr: "finishing", Amount: 4}}}},
			URL:       "https://www.fut.gg/evolutions/training-camp-finishing/",
		},
		{
			ID: "e8", Slug: "cosmetic-kit", Name: "Kit de Verão",
			Description: "Cosmético desbloqueável para a carta.", CategoryName: "Cosmetics", CategorySlug: "cosmetics", DoesNotUpgradePlayer: true,
			ExpiresAt: time.Now().Add(240 * time.Hour),
			URL:       "https://www.fut.gg/evolutions/cosmetic-kit/",
		},
	}
}

func demoSBCs() []domain.SBC {
	return []domain.SBC{
		{ID: "s1", Name: "Upgrade 85+ Garantido", Group: "Upgrades", Repeatable: true,
			SolutionCost: 62_000, ExpiresAt: time.Now().Add(30 * time.Hour),
			Rewards: []domain.Reward{{Kind: "pack", Description: "1x jogador 85+ não negociável", PackValue: 78_000}},
			Challenges: []domain.SBCChallenge{
				{Name: "85+", RequirementsText: []string{"Min. Team Rating: 85"}, CheapestSolutionCoins: 62_000},
			}},
		{ID: "s2", Name: "Desafio da Liga: Serie A", Group: "Ligas",
			SolutionCost: 18_000, ExpiresAt: time.Now().Add(200 * time.Hour),
			Rewards: []domain.Reward{{Kind: "pack", Description: "Pacote Ouro Premium Jogadores", PackValue: 25_000}},
			Challenges: []domain.SBCChallenge{
				{Name: "Serie A", RequirementsText: []string{"Min. 1 Players from: Serie A", "Min. Team Rating: 83"}, CheapestSolutionCoins: 18_000},
			}},
		{ID: "s3", Name: "Marcelo Icon", Group: "Ícones",
			SolutionCost: 890_000, ExpiresAt: time.Now().Add(400 * time.Hour),
			Rewards: []domain.Reward{{Kind: "player", Description: "Marcelo 89 LB (não negociável)", PackValue: 700_000}},
			Challenges: []domain.SBCChallenge{
				{Name: "Ícones", RequirementsText: []string{"Min. 1 Players: Any Icons"}, CheapestSolutionCoins: 890_000},
			}},
	}
}

// demoSBCCostTrends dá uma fase diferente pra cada SBC do demo — pico
// (s1, custo subindo forte), esfriando (s2, custo caindo) e recente (s3,
// sem tendência ainda) — pra tela de Investimentos mostrar os três casos.
func demoSBCCostTrends() map[string]analyze.CostTrend {
	return map[string]analyze.CostTrend{
		store.SBCChallengeKey("s1", 0, "85+"):     {ChangePct: 22.5, Samples: 4},
		store.SBCChallengeKey("s2", 0, "Serie A"): {ChangePct: -14.0, Samples: 3},
	}
}

// demoMomentum simula a rota de momentum do fut.gg: cartas do mercado
// caindo da própria média recente. Doué (id=101) também aparece como
// out-of-packs: demoNewCards inclui uma carta nova que compartilha o
// mesmo BasePlayerEaID, simulando "o jogador ganhou uma versão nova".
func demoMomentum() []domain.Player {
	discount := func(base domain.Player, pct float64, baseEaID int64) domain.Player {
		base.MomentumPct = pct
		base.BasePlayerEaID = baseEaID
		return base
	}
	market := demoMarket()
	byID := make(map[int64]domain.Player, len(market))
	for _, p := range market {
		byID[p.ID] = p
	}
	return []domain.Player{
		discount(byID[101], 28.4, 9001), // Doué — vira out-of-packs (ver demoNewCards)
		discount(byID[103], 19.2, 9002), // Guéhi
		discount(byID[109], 8.0, 9003),  // Hato — abaixo do piso de 15%, cai no funil
	}
}

// demoNewCards simula "cartas vistas pela primeira vez hoje" — uma versão
// especial nova do MESMO jogador do Doué (BasePlayerEaID 9001), pra
// exercitar o sinal de out-of-packs em FindInvestments.
func demoNewCards() []domain.Player {
	novaVersao := p(9101, "Doué TOTS+", 91, domain.RW, "TOTS+", 95, 87, 86, 94, 50, 74, 480_000)
	novaVersao.BasePlayerEaID = 9001
	return []domain.Player{novaVersao}
}

func demoObjectives() []domain.Objective {
	return []domain.Objective{
		{ID: "o1", Name: "Vitórias em Rivals", Group: "Semanais",
			Tasks: []string{"Vença 4 partidas de Rivals"}, ExpiresAt: time.Now().Add(40 * time.Hour),
			Rewards: []domain.Reward{{Kind: "pack", Description: "Pacote Ouro Raro Jogadores", PackValue: 12_500}}},
		{ID: "o2", Name: "Estreia do TOTS", Group: "Live",
			Tasks: []string{"Marque com 3 jogadores TOTS diferentes"}, ExpiresAt: time.Now().Add(90 * time.Hour),
			Rewards: []domain.Reward{{Kind: "player", Description: "Escolha de jogador TOTS 84+", PackValue: 45_000}}},
	}
}

func demoNews() []domain.NewsItem {
	return []domain.NewsItem{
		{ID: "n1", Title: "Time da Temporada: Serie A anunciado", PublishedAt: time.Now().Add(-3 * time.Hour),
			Summary: "Onze titular e reservas do TOTS da Serie A entram nos pacotes hoje às 19h.",
			URL:     "https://www.fut.gg/news/tots-serie-a/"},
		{ID: "n2", Title: "Nova evolução gratuita destrava CDM", PublishedAt: time.Now().Add(-9 * time.Hour),
			Summary: "Motor do Meio dá +4 de overall e Press Proven+ sem custo de moedas.",
			URL:     "https://www.fut.gg/news/evo-motor-do-meio/"},
	}
}

func demoMarketRows(snap *futgg.Snapshot, rng *rand.Rand) []report.MarketRow {
	rows := []report.MarketRow{}
	add := func(name, role string, last int, pct float64) {
		first := int(float64(last) / (1 + pct/100))
		lo, hi := first, last
		if lo > hi {
			lo, hi = hi, lo
		}
		rows = append(rows, report.MarketRow{
			Name: name, Role: role,
			Trend: store.PriceTrend{Last: last, First: first, Min: lo, Max: hi, ChangePct: pct, Samples: 6},
		})
	}
	add("Osimhen", "titular", 118_000, 14.2)
	add("Doué", "alvo RW", 210_000, -11.8)
	add("Rodri", "titular (untradeable)", 128_000, 9.4)
	add("Nico Williams", "alvo LW", 165_000, -7.3)
	add("Guéhi", "alvo CB", 71_000, -5.1)
	add("Leão", "titular", 62_000, 6.2)
	_ = rng
	return rows
}

// demoWatchlist é a Vigiadas do modo demo: um alvo ainda longe do preço, um
// protegido (marca manual, não filtro — nada no motor a aplica sozinho, ver
// domain.WatchlistEntry.Protected), um já dentro do próprio alvo (Guéhi,
// 71.000 de preço contra meta de 75.000) e um cuja compra já está no ledger
// abaixo, mostrando que a watchlist sobrevive à decisão já tomada.
func demoWatchlist() []domain.WatchlistEntry {
	now := time.Now()
	return []domain.WatchlistEntry{
		{ID: "demo-watch-doue", EAID: 101, Name: "Doué", TargetCoins: 180_000,
			Note: "porta-estandarte se cair de novo", CreatedAt: now.AddDate(0, 0, -12), UpdatedAt: now.AddDate(0, 0, -2)},
		{ID: "demo-watch-nico", EAID: 102, Name: "Nico Williams", TargetCoins: 150_000, Protected: true,
			Note: "alternativa barata de ala esquerda — não descartar mesmo se subir", CreatedAt: now.AddDate(0, 0, -9), UpdatedAt: now.AddDate(0, 0, -9)},
		{ID: "demo-watch-guehi", EAID: 103, Name: "Guéhi", TargetCoins: 75_000,
			Note: "zaga: já dentro do alvo, falta fechar a compra", CreatedAt: now.AddDate(0, 0, -6), UpdatedAt: now.AddDate(0, 0, -1)},
		{ID: "demo-watch-ekitike", EAID: 105, Name: "Ekitiké", TargetCoins: 200_000,
			Note: "alvo de ataque, ainda caro pelo bolso atual", CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: now.AddDate(0, 0, -5)},
	}
}

// demoLedger dá ~10 dias de extrato cruzando a janela de 7d de Hoje: dois
// lançamentos ficam FORA da janela (9 e 8 dias atrás), o resto dentro —
// prova que domain.SummarizeLedgerSince filtra por OccurredAt de verdade.
// Cobre compra confirmada, venda confirmada (mostra a taxa de 5%), SBC,
// ajuste negativo revertido em seguida (auditoria: reversão referencia o id
// anterior em vez de apagar) e uma compra ainda planejada (alimenta
// LedgerSummary.Committed, e casa com a watchlist do Guéhi acima).
func demoLedger(agora time.Time) []domain.LedgerEntry {
	at := func(daysAgo int) time.Time { return agora.AddDate(0, 0, -daysAgo) }
	const ajusteID = "demo-ledger-ajuste"
	return []domain.LedgerEntry{
		{ID: "demo-ledger-compra-ekitike", Kind: domain.LedgerCompra, Status: domain.LedgerConfirmado, EAID: 105, GrossCoins: 245_000,
			Note: "reforço de ataque — Ekitiké", OccurredAt: at(9), RecordedAt: at(9)},
		{ID: "demo-ledger-venda-kolo-muani", Kind: domain.LedgerVenda, Status: domain.LedgerConfirmado, EAID: 13, GrossCoins: 14_000,
			Note: "saída de reserva pra abrir espaço no elenco", OccurredAt: at(8), RecordedAt: at(8)},
		{ID: "demo-ledger-sbc-upgrade85", Kind: domain.LedgerSBC, Status: domain.LedgerConfirmado, GrossCoins: 62_000,
			Note: "Upgrade 85+ Garantido", OccurredAt: at(5), RecordedAt: at(5)},
		{ID: ajusteID, Kind: domain.LedgerAjuste, Status: domain.LedgerConfirmado, GrossCoins: -5_000,
			Note: "correção de saldo lançada em duplicidade", OccurredAt: at(4), RecordedAt: at(4)},
		{ID: "demo-ledger-evolucao-david", Kind: domain.LedgerEvolucao, Status: domain.LedgerConfirmado, EAID: 16, GrossCoins: 18_000,
			Note: "Caçador de Área — J. David", OccurredAt: at(3), RecordedAt: at(3)},
		{ID: "demo-ledger-reversao-ajuste", Kind: domain.LedgerReversao, Status: domain.LedgerConfirmado, ReversesID: ajusteID,
			Note: "ajuste revertido depois de conferir o saldo real", OccurredAt: at(3), RecordedAt: at(2)},
		{ID: "demo-ledger-compra-guehi", Kind: domain.LedgerCompra, Status: domain.LedgerPlanejado, EAID: 103, GrossCoins: 71_000,
			Note: "aguardando venda de reserva pra fechar caixa", OccurredAt: at(1), RecordedAt: at(1)},
		{ID: "demo-ledger-venda-avulsa", Kind: domain.LedgerVenda, Status: domain.LedgerConfirmado, EAID: 9101, GrossCoins: 32_000,
			Note: "carta avulsa do pacote, vendida na hora", OccurredAt: at(0), RecordedAt: at(0)},
	}
}

// demoPriceWalk gera `days` preços terminando EXATAMENTE em `final` — o
// último ponto é o preço "de hoje" que o resto da fixture já usa em outro
// lugar (demoClub/demoMarket), então a série de 30 dias não pode divergir
// do preço atual mostrado alhures. O ruído vem de um rand.Rand semeado à
// parte (não o `rng` de demoSnapshot) para não acoplar esta trilha à ordem
// de consumo de outra função.
func demoPriceWalk(rng *rand.Rand, final int, days int) []int {
	if days <= 0 {
		return nil
	}
	prices := make([]int, days)
	current := float64(final) * 0.82
	for i := 0; i < days; i++ {
		drift := 1 + (rng.Float64()*0.06 - 0.02) // -2% a +4% ao dia, leve viés de alta
		current *= drift
		prices[i] = int(current)
	}
	prices[days-1] = final
	return prices
}

// demoPriceHistoryDays é quantos dias de preço demoSeedPriceHistory grava —
// casa com priceSeriesWindow (internal/api) e com a retenção padrão de
// snapshots (store.snapshotRetention), as duas janelas de 30 dias que a UI
// lê.
const demoPriceHistoryDays = 30

// demoSeedPriceHistory grava demoPriceHistoryDays de preço pro mercado e
// pro clube inteiro (titulares inclusos, não só o banco — a "Referência de
// mercado" do Detalhe da carta também precisa de série real) pelo caminho
// real (DemoSeeder.SavePricesAt). SavePrices sozinho não serve aqui: usa
// time.Now() e descarta qualquer ponto a menos de 1h do anterior, o que
// colapsaria uma rajada de 30 chamadas em segundos num único ponto. As
// chamadas têm que sair em ordem CRESCENTE de data — do dia mais antigo pro
// mais novo — porque essa mesma trava assume progressão cronológica; na
// ordem contrária, todo ponto depois do primeiro pareceria "cedo demais" e
// seria descartado (foi exatamente o bug encontrado ao verificar isto ao
// vivo).
//
// CHAMAR ANTES de analyzeAndBuild: analyzeAndBuild grava o preço de HOJE do
// mercado sozinho (st.SavePrices(snap.Market), com time.Now() interno) — se
// esta função rodasse depois, aquele ponto já existiria com timestamp mais
// recente que qualquer dia do backfill, e a mesma trava de 1h descartaria
// os 29 dias inteiros do mercado por parecerem "no passado" em relação a um
// ponto que já está lá (o mesmo bug do parágrafo acima, por um caminho
// diferente — também encontrado ao verificar isto ao vivo).
//
// Três cartas do banco recebem tratamento à parte pra exercitar os estados
// de price_history_status que dependem do FORMATO do dado, não só do tempo
// (o quinto estado, falha_leitura, só acontece com erro de leitura do
// Store — não dá pra simular semeando dado). Nenhuma das três aparece na
// tabela do banco em /time (benchMinimumRating=88 corta as três, de
// propósito — CLAUDE.md, "busca ⌘K abaixo do piso de 88"), mas continuam
// alcançáveis direto por /time/:slug, onde o painel "Referência de mercado"
// mostra o mesmo status:
//   - Reijnders (12), a mesma carta com EvolutionFetchError: NUNCA recebe
//     preço — sem_historico. Combina com "falhou a coleta" desta carta.
//   - Zaïre-Emery (15): um ponto só, hoje — amostra_unica.
//   - Kolo Muani (13): todo dia, sempre extinto (ele saiu do clube no
//     ledger acima) — sem_oferta.
//
// J. David (16) é a ÚNICA carta abaixo do titular que passa dos 88 e
// aparece na tabela do banco de verdade — por isso ela NÃO entra nesse
// trio: fica no caso comum (historico_parcial, com o resto do clube), pra
// a tabela do banco mostrar uma sparkline de verdade em vez de mais um
// estado vazio.
//
// Todo o resto (mercado inteiro + banco/titulares restantes) recebe série
// cheia com preço variando — historico_parcial, o caso comum.
func demoSeedPriceHistory(ctx context.Context, seeder store.DemoSeeder, cycle string, agora time.Time) error {
	rng := rand.New(rand.NewSource(2600))
	market := demoMarket()
	club := demoClub()

	const (
		semHistorico = int64(12) // Reijnders
		amostraUnica = int64(15) // Zaïre-Emery
		semOferta    = int64(13) // Kolo Muani
	)

	walkFor := func(base domain.Player) []int { return demoPriceWalk(rng, base.Price.Coins, demoPriceHistoryDays) }
	marketWalks := make(map[int64][]int, len(market))
	for _, p := range market {
		marketWalks[p.ID] = walkFor(p)
	}
	clubByID := make(map[int64]domain.Player, len(club.Players))
	clubWalks := make(map[int64][]int, len(club.Players))
	for _, cp := range club.Players {
		clubByID[cp.ID] = cp.Player
		if cp.ID == semHistorico || cp.ID == amostraUnica {
			continue
		}
		clubWalks[cp.ID] = walkFor(cp.Player)
	}

	// Só os demoPriceHistoryDays-1 dias ANTERIORES a hoje — esta função tem
	// que ser chamada ANTES de analyzeAndBuild, que grava o ponto de HOJE do
	// mercado sozinho (mesmo preço: demoPriceWalk força o último valor da
	// caminhada a bater com o preço "atual" do resto da fixture). Semear
	// "hoje" aqui de novo cairia na trava de 1h de SavePricesAt e seria
	// descartado sem aviso — foi exatamente o bug encontrado ao verificar
	// isto ao vivo (o mercado inteiro ficava com 1 ponto só).
	for day := 0; day < demoPriceHistoryDays-1; day++ {
		daysAgo := demoPriceHistoryDays - 1 - day
		at := agora.AddDate(0, 0, -daysAgo)

		var batch []domain.Player
		for _, p := range market {
			priced := p
			priced.Price.Coins, priced.Price.UpdatedAt = marketWalks[p.ID][day], at
			batch = append(batch, priced)
		}
		for id, walk := range clubWalks {
			priced := clubByID[id]
			priced.Price.Coins, priced.Price.UpdatedAt = walk[day], at
			if id == semOferta {
				priced.Price.Coins, priced.Price.Extinct = 0, true
			}
			batch = append(batch, priced)
		}
		if err := seeder.SavePricesAt(ctx, cycle, batch, at); err != nil {
			return fmt.Errorf("semeando preço de %s: %w", at.Format("2006-01-02"), err)
		}
	}

	// Hoje: só o clube precisa do ponto explícito — analyzeAndBuild (chamado
	// por quem invoca esta função, logo depois) grava o do mercado sozinho.
	var todayClub []domain.Player
	for id, walk := range clubWalks {
		priced := clubByID[id]
		priced.Price.Coins, priced.Price.UpdatedAt = walk[demoPriceHistoryDays-1], agora
		if id == semOferta {
			priced.Price.Coins, priced.Price.Extinct = 0, true
		}
		todayClub = append(todayClub, priced)
	}
	if err := seeder.SavePricesAt(ctx, cycle, todayClub, agora); err != nil {
		return fmt.Errorf("semeando preço de hoje do clube: %w", err)
	}

	// amostraUnica só existe HOJE — um ponto isolado, nunca visto antes.
	unica := clubByID[amostraUnica]
	if err := seeder.SavePricesAt(ctx, cycle, []domain.Player{unica}, agora); err != nil {
		return fmt.Errorf("semeando amostra única: %w", err)
	}
	return nil
}

// demoHistoricalSnapshots grava dias ANTERIORES a hoje com Nota/Saldo
// derivando até o valor de hoje — pelo caminho real (store.SaveSnapshot),
// que já é indexado por snap.GeneratedAt (não por time.Now()), então isto
// não esbarra na mesma trava de relógio real de SavePrices. Só o resumo
// leve (SnapshotSummary) importa aqui; Club fica vazio de propósito — o
// snapshot de hoje (gravado por analyzeAndBuild ANTES desta função, com
// data mais recente) continua sendo o único que LatestSnapshot devolve.
func demoHistoricalSnapshots(ctx context.Context, st store.Store, cycle string, agora time.Time, finalScore float64, finalCoins int) error {
	for daysAgo := demoPriceHistoryDays - 1; daysAgo >= 1; daysAgo-- {
		progress := float64(demoPriceHistoryDays-1-daysAgo) / float64(demoPriceHistoryDays-1)
		score := finalScore - 2.1 + progress*2.1 + math.Sin(float64(daysAgo))*0.15
		coins := finalCoins - 60_000 + int(progress*60_000) + int(math.Sin(float64(daysAgo)*1.3)*8_000)
		if coins < 5_000 {
			coins = 5_000
		}
		snap := store.Snapshot{
			GeneratedAt: agora.AddDate(0, 0, -daysAgo),
			Cycle:       cycle,
			Club:        domain.Club{Coins: coins},
			SquadScore:  score,
		}
		if err := st.SaveSnapshot(ctx, snap); err != nil {
			return fmt.Errorf("semeando snapshot de %d dias atrás: %w", daysAgo, err)
		}
	}
	return nil
}
