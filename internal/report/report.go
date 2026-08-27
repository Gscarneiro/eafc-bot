// Package report transforma a análise num briefing HTML autocontido.
package report

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/cards"
	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/futgg"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

//go:embed report.gohtml
var tmplSource string

// MarketRow é uma linha da tabela de mercado.
type MarketRow struct {
	Name  string
	Role  string // "titular", "reserva", "alvo"
	Trend store.PriceTrend
}

// SquadCard é um titular pronto pro "elenco principal": a posição do slot
// físico (não a natural da carta — um titular pode estar fora de posição)
// junto com a carta em si.
type SquadCard struct {
	Index    int
	Position domain.Position
	Player   domain.ClubPlayer
}

// Data é tudo que o template precisa. Montada por Build.
type Data struct {
	GeneratedAt time.Time
	Duration    string
	Cycle       string
	Club        domain.Club
	SquadScore  float64
	Raisable    int
	WeakestSlot domain.Position
	WeakestName string
	// WeakestGGRating é o GG Rating do fut.gg pro jogador mais fraco — é
	// esse número, e não o Score() próprio, que decide quem aparece aqui
	// (ver analyze.WeakestLinks); 0 quando a fonte não trouxe o dado (só o
	// elenco do GG Club traz).
	WeakestGGRating float64
	// SquadSwaps são trocas de custo zero: alguém que já está no seu
	// elenco, na mesma posição, com GG Rating maior que o titular atual.
	SquadSwaps []analyze.SquadSwap
	SquadPlan  analyze.SquadPlan
	// MainSquad é o XI titular, na ordem que o fut.gg usa (0 = goleiro,
	// crescendo em direção ao ataque) — o "elenco principal" que fecha o
	// relatório, com a arte da carta de cada um.
	MainSquad []SquadCard
	NewCards  []domain.Player
	News      []domain.NewsItem
	Upgrades  []analyze.Upgrade
	// Funnel explica uma lista de Upgrades vazia carta a carta — ver o
	// comentário de analyze.UpgradeFunnel. É o que o {{else}} da seção
	// "Upgrades diretos" imprime em vez de um "nada aqui" mudo.
	Funnel      analyze.UpgradeFunnel
	Evolutions  []analyze.EvoMatch
	SBCs        []domain.SBC
	Objectives  []domain.Objective
	Market      []MarketRow
	TrendWindow string
	MarketSize  int
	// AnyUnpriced avisa o leitor que parte das sugestões veio sem cotação,
	// para ninguém ler "?" como bug.
	AnyUnpriced bool
	Stats       futgg.Stats
	Errors      []string

	// Investments, SellCandidates e FodderDemand são o agente de trading:
	// cartas do mercado ganhando valor, o que fazer com o banco de
	// reservas, e demanda de fodder de SBC esquentando — ver
	// analyze.FindInvestments/FindSellCandidates/FindFodderDemand.
	// Puramente consultivo, como o resto do bot.
	Investments      []analyze.Investment
	InvestmentFunnel analyze.InvestmentFunnel
	SellCandidates   []analyze.SellCandidate
	SellFunnel       analyze.SellFunnel
	FodderDemand     []analyze.FodderSignal
}

// Input reúne o que Build precisa para montar o briefing.
type Input struct {
	Snapshot    *futgg.Snapshot
	NewCards    []domain.Player
	FreshNews   []domain.NewsItem
	Upgrades    []analyze.Upgrade
	Funnel      analyze.UpgradeFunnel
	Evolutions  []analyze.EvoMatch
	Trends      map[int64]store.PriceTrend
	TrendWindow time.Duration
	Started     time.Time
	MaxRows     int

	// CardReports é a análise carta-a-carta (atual x potencial) do
	// elenco — mesma fonte de cards.CardReport.Best que
	// analyze.FindSellCandidates usa pra saber se vale segurar uma carta
	// do banco por potencial de evolução.
	CardReports []cards.CardReport
	// Momentum é o último valor lido da rota de momentum do fut.gg (ver
	// store.Store.LatestMomentum) — pode ser mais fresco que o resto do
	// snapshot, já que vem do ciclo de coleta rápido
	// (scheduler.FastTicker), não do job diário.
	Momentum []domain.Player
	// SBCCostTrends é a tendência de custo de cada desafio de SBC (ver
	// store.Store.SBCCostTrend), indexada por store.SBCChallengeKey.
	SBCCostTrends map[string]store.PriceTrend

	// ChemistryModel decide como SquadPlan.Quimica/CurrentQuimica são
	// calculados (ver internal/chemistry). Zero-valor cai no modelo padrão.
	ChemistryModel chemistry.Modelo
}

// Build organiza os resultados na forma que o relatório apresenta:
// corta as listas em tamanho legível e ordena o que importa primeiro.
func Build(in Input) Data {
	if in.MaxRows <= 0 {
		in.MaxRows = 12
	}
	snap := in.Snapshot
	club := snap.Club

	_, raisable := club.Budget()

	d := Data{
		GeneratedAt: time.Now(),
		Duration:    time.Since(in.Started).Round(100 * time.Millisecond).String(),
		Cycle:       club.Cycle,
		Club:        club,
		Raisable:    raisable,
		NewCards:    trimPlayers(in.NewCards, 8),
		News:        trimNews(in.FreshNews, 6),
		Upgrades:    trimUpgrades(in.Upgrades, in.MaxRows),
		Funnel:      in.Funnel,
		Evolutions:  trimEvos(in.Evolutions, in.MaxRows),
		TrendWindow: humanDuration(in.TrendWindow),
		MarketSize:  len(snap.Market),
		Stats:       snap.Stats,
		Errors:      snap.Errors,
	}

	for _, u := range d.Upgrades {
		if u.Unpriced {
			d.AnyUnpriced = true
			break
		}
	}

	d.SquadScore, d.WeakestSlot, d.WeakestName, d.WeakestGGRating = SquadSummary(club)
	model := in.ChemistryModel
	if model.Nome == "" {
		model = chemistry.ModeloPadrao()
	}
	d.SquadPlan = analyze.OptimizeSquadWithOptions(club, analyze.SquadOptions{ChemistryModel: model})
	d.SquadSwaps = analyze.FindSquadSwaps(club)
	d.MainSquad = MainSquad(club)
	d.SBCs, d.Objectives = RankChallenges(snap.SBCs, snap.Objectives)
	d.Market = MarketRows(club, in.Upgrades, in.Trends)

	investments, invFunnel := analyze.FindInvestments(club, in.Momentum, in.NewCards, analyze.DefaultInvestmentOptions())
	d.Investments, d.InvestmentFunnel = trimInvestments(investments, in.MaxRows), invFunnel

	sellCandidates, sellFunnel := analyze.FindSellCandidates(club, in.CardReports, d.SquadSwaps, analyze.DefaultSellOptions())
	d.SellCandidates, d.SellFunnel = trimSellCandidates(sellCandidates, in.MaxRows), sellFunnel

	// FindFodderDemand mora em internal/analyze, que já é importado por
	// internal/store (store.Snapshot.Upgrades) — analyze não pode
	// importar store de volta sem virar ciclo, então a conversão de
	// store.PriceTrend pra analyze.CostTrend (mesmo formato, tipo local)
	// acontece aqui, no pacote que já importa os dois.
	costTrends := make(map[string]analyze.CostTrend, len(in.SBCCostTrends))
	for key, t := range in.SBCCostTrends {
		costTrends[key] = analyze.CostTrend{ChangePct: t.ChangePct, Samples: t.Samples}
	}
	d.FodderDemand = trimFodderSignals(
		analyze.FindFodderDemand(snap.SBCs, snap.Market, costTrends, analyze.DefaultFodderDemandOptions()),
		in.MaxRows)

	return d
}

// SquadSummary calcula a "Nota do time" e o "elo mais fraco" que abrem o
// briefing. Os dois usam o GG Rating do fut.gg quando o XI inteiro tem essa
// nota (ver analyze.WeakestLinks) — é o número que já se conhece do site,
// preso entre 0 e ~99, em vez do Score() deste pacote, que soma bônus de
// PlayStyle sem teto e pode passar de 99. Score() só volta a decidir quando
// a fonte não é o GG Club (csv, chrome) e por isso não traz GG Rating.
func SquadSummary(club domain.Club) (avg float64, weakSlot domain.Position, weakName string, weakGGRating float64) {
	weak := analyze.WeakestLinks(club, 1)

	var ggSum float64
	var ggN int
	var scoreSum float64
	var scoreN int
	for _, squadSlot := range club.Squad.Starters {
		p, ok := club.PlayerByID(squadSlot.PlayerID)
		if !ok {
			continue
		}
		scoreSum += analyze.EvaluateBotScore(p.Player, squadSlot.Position, analyze.DefaultBotScoreProfile).Total
		scoreN++
		if p.GGRating > 0 {
			ggSum += p.GGRating
			ggN++
		}
	}
	switch {
	case ggN > 0:
		avg = ggSum / float64(ggN)
	case scoreN > 0:
		// A fonte não trouxe GG Rating (não é o GG Club, é csv/chrome/etc):
		// cai pro Score() próprio em vez de mostrar zero.
		avg = scoreSum / float64(scoreN)
	}

	if len(weak) > 0 {
		weakSlot = weak[0].Slot
		weakName = weak[0].Player.Display()
		weakGGRating = weak[0].Player.GGRating
	} else {
		weakSlot, weakName = "—", "elenco não sincronizado"
	}
	return avg, weakSlot, weakName, weakGGRating
}

// MainSquad monta o XI na ordem que o fut.gg usa (positionIdx 0..10).
// Titular sem carta correspondente no elenco (nunca deveria acontecer, mas
// ActiveSquad já pode falhar independente do elenco) fica de fora em vez de
// virar um card vazio.
func MainSquad(club domain.Club) []SquadCard {
	out := make([]SquadCard, 0, len(club.Squad.Starters))
	for _, slot := range club.Squad.Starters {
		if p, ok := club.PlayerByID(slot.PlayerID); ok {
			out = append(out, SquadCard{Index: slot.Index, Position: slot.Position, Player: p})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out
}

// RankChallenges põe na frente o que expira logo e o que paga melhor.
func RankChallenges(sbcs []domain.SBC, objs []domain.Objective) ([]domain.SBC, []domain.Objective) {
	worthwhile := sbcs[:0:0]
	for _, s := range sbcs {
		// Vale citar se paga mais do que custa, ou se está acabando.
		if s.NetValue() > 0 || s.Expiring(48*time.Hour) {
			worthwhile = append(worthwhile, s)
		}
	}
	sort.Slice(worthwhile, func(i, j int) bool {
		ei, ej := worthwhile[i].Expiring(24*time.Hour), worthwhile[j].Expiring(24*time.Hour)
		if ei != ej {
			return ei
		}
		return worthwhile[i].NetValue() > worthwhile[j].NetValue()
	})
	if len(worthwhile) > 8 {
		worthwhile = worthwhile[:8]
	}

	sort.Slice(objs, func(i, j int) bool {
		ei, ej := objs[i].Expiring(48*time.Hour), objs[j].Expiring(48*time.Hour)
		if ei != ej {
			return ei
		}
		return objs[i].ExpiresAt.Before(objs[j].ExpiresAt)
	})
	if len(objs) > 8 {
		objs = objs[:8]
	}
	return worthwhile, objs
}

// MarketRows junta as cartas que você tem com as que o bot está mirando,
// para a tabela responder "vender o quê e comprar o quê".
func MarketRows(club domain.Club, ups []analyze.Upgrade, trends map[int64]store.PriceTrend) []MarketRow {
	role := map[int64]string{}
	name := map[int64]string{}

	for _, p := range club.Players {
		r := "reserva"
		if p.InSquad {
			r = "titular"
		}
		if p.Untradeable {
			r += " (untradeable)"
		}
		role[p.ID], name[p.ID] = r, p.Display()
	}
	for _, u := range ups {
		if _, have := role[u.Candidate.ID]; !have {
			role[u.Candidate.ID] = "alvo " + string(u.Slot)
			name[u.Candidate.ID] = u.Candidate.Display()
		}
	}

	var rows []MarketRow
	for id, t := range trends {
		r, ok := role[id]
		if !ok {
			continue
		}
		// Só mostra o que se moveu de verdade — tabela cheia de "0%" é ruído.
		if t.ChangePct > -2 && t.ChangePct < 2 {
			continue
		}
		rows = append(rows, MarketRow{Name: name[id], Role: r, Trend: t})
	}
	sort.Slice(rows, func(i, j int) bool {
		return abs64(rows[i].Trend.ChangePct) > abs64(rows[j].Trend.ChangePct)
	})
	if len(rows) > 15 {
		rows = rows[:15]
	}
	return rows
}

func abs64(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func trimPlayers(in []domain.Player, n int) []domain.Player {
	sort.Slice(in, func(i, j int) bool { return in[i].Rating > in[j].Rating })
	if len(in) > n {
		return in[:n]
	}
	return in
}

func trimNews(in []domain.NewsItem, n int) []domain.NewsItem {
	sort.Slice(in, func(i, j int) bool { return in[i].PublishedAt.After(in[j].PublishedAt) })
	if len(in) > n {
		return in[:n]
	}
	return in
}

func trimInvestments(in []analyze.Investment, n int) []analyze.Investment {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func trimSellCandidates(in []analyze.SellCandidate, n int) []analyze.SellCandidate {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func trimFodderSignals(in []analyze.FodderSignal, n int) []analyze.FodderSignal {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func trimUpgrades(in []analyze.Upgrade, n int) []analyze.Upgrade {
	if len(in) > n {
		return in[:n]
	}
	return in
}

func trimEvos(in []analyze.EvoMatch, n int) []analyze.EvoMatch {
	// No máximo uma sugestão por jogador: a melhor evolução para ele.
	seen := map[int64]bool{}
	out := in[:0:0]
	for _, m := range in {
		if seen[m.Player.ID] {
			continue
		}
		seen[m.Player.ID] = true
		out = append(out, m)
		if len(out) >= n {
			break
		}
	}
	return out
}

func humanDuration(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%d dias", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%d horas", int(d.Hours()))
	default:
		return d.String()
	}
}

var funcs = template.FuncMap{
	// coins formata 1234567 como "1.234.567" (padrão brasileiro).
	"coins": func(v int) string {
		neg := v < 0
		if neg {
			v = -v
		}
		s := fmt.Sprint(v)
		var b strings.Builder
		for i, r := range s {
			if i > 0 && (len(s)-i)%3 == 0 {
				b.WriteByte('.')
			}
			b.WriteRune(r)
		}
		if neg {
			return "-" + b.String()
		}
		return b.String()
	},
	"join":   func(s []string, sep string) string { return strings.Join(s, sep) },
	"signed": func(f float64) string { return fmt.Sprintf("%+.1f", f) },
	"trendClass": func(pct float64) string {
		switch {
		case pct >= 3:
			return "cost" // subiu: caro para comprar
		case pct <= -3:
			return "gain" // caiu: bom para comprar
		default:
			return "flat"
		}
	},
	"expiring": func(t time.Time) bool {
		return !t.IsZero() && time.Until(t) <= 48*time.Hour && time.Until(t) > 0
	},
	"attrs": func(p domain.Player) string {
		a := p.Attributes
		if p.Position.IsGK() {
			return fmt.Sprintf("DIV %d · MAN %d · REF %d · POS %d",
				a.Pace, a.Shooting, a.Dribbling, a.Physical)
		}
		return fmt.Sprintf("RIT %d · FIN %d · PAS %d · DRI %d · DEF %d · FIS %d",
			a.Pace, a.Shooting, a.Passing, a.Dribbling, a.Defending, a.Physical)
	},
	"rewards": func(rs []domain.Reward) string {
		parts := make([]string, 0, len(rs))
		for _, r := range rs {
			if r.Description != "" {
				parts = append(parts, r.Description)
			}
		}
		return strings.Join(parts, " · ")
	},
}

var tmpl = template.Must(template.New("report").Funcs(funcs).Parse(tmplSource))

// Render escreve o HTML final.
func Render(w io.Writer, d Data) error {
	return tmpl.Execute(w, d)
}
