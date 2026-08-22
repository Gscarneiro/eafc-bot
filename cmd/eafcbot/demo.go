package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"time"

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

	data, err := analyzeAndBuild(context.Background(), cfg, nil, snap, started, true, nil)
	if err != nil {
		return err
	}
	// Preenche as seções que dependem de histórico, para o demo mostrar
	// o relatório completo.
	data.NewCards = snap.Market[:3]
	data.Market = demoMarketRows(snap, rng)
	data.TrendWindow = "72 horas"

	if err := writeReport(*outPath, data); err != nil {
		return err
	}

	fmt.Printf("relatório de exemplo: %s\n", *outPath)
	fmt.Printf("  %d upgrades · %d evoluções · %d SBCs\n",
		len(data.Upgrades), len(data.Evolutions), len(data.SBCs))
	return nil
}

func demoSnapshot(rng *rand.Rand) *futgg.Snapshot {
	club := demoClub()
	return &futgg.Snapshot{
		Club:       club,
		Market:     demoMarket(),
		Evolutions: demoEvolutions(),
		SBCs:       demoSBCs(),
		Objectives: demoObjectives(),
		News:       demoNews(),
		Stats:      futgg.Stats{Requests: 14, CacheHits: 3, Retries: 1, Bytes: 2_400_000},
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
	}
}

func ps(name string, plus bool) domain.PlayStyle { return domain.PlayStyle{Name: name, Plus: plus} }

func demoClub() domain.Club {
	players := []domain.ClubPlayer{
		{Player: p(1, "Maignan", 87, domain.GK, "Ouro Raro", 86, 84, 70, 88, 45, 82, 38_000, ps("Far Reach", true))},
		{Player: p(2, "Frimpong", 84, domain.RB, "Ouro Raro", 94, 70, 76, 84, 78, 74, 22_000, ps("Quick Step", false))},
		{Player: p(3, "Saliba", 86, domain.CB, "Ouro Raro", 84, 40, 65, 72, 87, 84, 46_000, ps("Anticipate", true))},
		{Player: p(4, "Gvardiol", 85, domain.CB, "Ouro Raro", 82, 55, 72, 75, 85, 83, 34_000, ps("Block", false))},
		{Player: p(5, "T. Hernández", 85, domain.LB, "Ouro Raro", 93, 76, 79, 84, 79, 82, 41_000, ps("Rapid", false))},
		{Player: p(6, "Rodri", 89, domain.CDM, "Ouro Raro", 68, 78, 86, 82, 87, 85, 128_000, ps("Anticipate", false), ps("Press Proven", true))},
		{Player: p(7, "Valverde", 88, domain.CM, "Ouro Raro", 85, 84, 85, 84, 79, 86, 96_000, ps("Relentless", true))},
		{Player: p(8, "Wirtz", 87, domain.CAM, "Ouro Raro", 82, 82, 87, 89, 45, 68, 74_000, ps("Technical", true))},
		{Player: p(9, "Saka", 87, domain.RW, "Ouro Raro", 88, 84, 82, 88, 52, 72, 88_000, ps("Trickster", false))},
		{Player: p(10, "Leão", 86, domain.LW, "Ouro Raro", 95, 82, 76, 87, 38, 78, 62_000, ps("Rapid", true))},
		{Player: p(11, "Osimhen", 88, domain.ST, "Ouro Raro", 90, 87, 70, 84, 42, 85, 118_000, ps("Power Shot", false), ps("Aerial", true))},
		// Reservas: dão para vender e financiar as trocas.
		{Player: p(12, "Reijnders", 84, domain.CM, "Ouro Raro", 78, 76, 83, 84, 72, 76, 18_000)},
		{Player: p(13, "Kolo Muani", 83, domain.ST, "Ouro Raro", 89, 81, 72, 80, 40, 82, 14_000)},
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
	// Untradeable típico de recompensa: entra no time mas não vira moeda.
	players[6].Untradeable = true

	return domain.Club{
		GamerTag: "carneiro22", Platform: "ps5", Coins: 145_000,
		Players: players,
		Squad:   domain.Squad{Name: "Titular", Formation: "4-2-3-1", Chemistry: 29, Starters: starters, SyncedAt: time.Now()},
		Cycle:   "26", SyncedAt: time.Now(), Source: "demo",
	}
}

// demoCardReports monta à mão um punhado de cards.CardReport para o modo
// demo. cards.BuildReports precisa de um *futgg.Client de verdade (Roles e
// EvolutionPaths batem na rede), e o demo é sem rede por definição — sem
// isto snap.Cards fica vazio e /api/time/{slug} sempre devolve 404 aqui
// (era o aviso que serveDemo imprimia). Cobre os dois estados reais da
// tela de carta: uma com caminho de evolução disponível (Osimhen) e uma já
// no teto (Rodri, Best nil — resposta válida, não ausência de dado).
func demoCardReports(club domain.Club) []cards.CardReport {
	byID := make(map[int64]domain.ClubPlayer, len(club.Players))
	for _, cp := range club.Players {
		byID[cp.ID] = cp
	}
	osimhen, rodri := byID[11], byID[6]

	evoluido := osimhen.Player
	evoluido.Rating = 90

	return []cards.CardReport{
		{
			Slug:       "osimhen-88",
			Player:     osimhen,
			ByPosition: []cards.PositionRoles{{Position: domain.ST, PlusPlus: []string{"Finalizador"}, Plus: []string{"Alvo Avançado"}}},
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
		},
		{
			Slug:       "rodri-89",
			Player:     rodri,
			ByPosition: []cards.PositionRoles{{Position: domain.CDM, PlusPlus: []string{"Âncora"}, Plus: []string{"Armador Recuado"}}},
			Best:       nil,
		},
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
	}
}

func demoSBCs() []domain.SBC {
	return []domain.SBC{
		{ID: "s1", Name: "Upgrade 85+ Garantido", Group: "Upgrades", Repeatable: true,
			SolutionCost: 62_000, ExpiresAt: time.Now().Add(30 * time.Hour),
			Rewards: []domain.Reward{{Kind: "pack", Description: "1x jogador 85+ não negociável", PackValue: 78_000}}},
		{ID: "s2", Name: "Desafio da Liga: Serie A", Group: "Ligas",
			SolutionCost: 18_000, ExpiresAt: time.Now().Add(200 * time.Hour),
			Rewards: []domain.Reward{{Kind: "pack", Description: "Pacote Ouro Premium Jogadores", PackValue: 25_000}}},
		{ID: "s3", Name: "Marcelo Icon", Group: "Ícones",
			SolutionCost: 890_000, ExpiresAt: time.Now().Add(400 * time.Hour),
			Rewards: []domain.Reward{{Kind: "player", Description: "Marcelo 89 LB (não negociável)", PackValue: 700_000}}},
	}
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
