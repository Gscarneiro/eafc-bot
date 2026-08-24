package api

import (
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/domain"
	"github.com/gscarneiro/eafc-bot/internal/query"
	"github.com/gscarneiro/eafc-bot/internal/store"
)

func text[T any](name string, get func(T) string, facet, search bool) query.Field[T] {
	return query.Field[T]{Name: name, Kind: query.String, Get: func(row T) query.Value { return query.StringValue(get(row)) }, Facet: facet, Search: search}
}

func number[T any](name string, get func(T) float64) query.Field[T] {
	return query.Field[T]{Name: name, Kind: query.Number, Get: func(row T) query.Value { return query.NumberValue(get(row)) }}
}

func integer[T any](name string, get func(T) int) query.Field[T] {
	return number(name, func(row T) float64 { return float64(get(row)) })
}

func boolean[T any](name string, get func(T) bool, facet bool) query.Field[T] {
	return query.Field[T]{Name: name, Kind: query.Boolean, Get: func(row T) query.Value { return query.BoolValue(get(row)) }, Facet: facet}
}

func timeField[T any](name string, get func(T) time.Time) query.Field[T] {
	return query.Field[T]{Name: name, Kind: query.Time, Get: func(row T) query.Value { return query.TimeValue(get(row)) }}
}

func playerText(get func(domain.Player) string, name string, search bool) query.Field[domain.Player] {
	return text(name, get, false, search)
}

func mercadoSchema() query.Schema[analyze.Upgrade] {
	return query.NewSchema("mercado", "affordable desc,efficiency desc,gain desc", 500,
		text("slot", func(v analyze.Upgrade) string { return string(v.Slot) }, true, false),
		text("current/common_name", func(v analyze.Upgrade) string { return v.Current.CommonName }, false, true),
		text("candidate/common_name", func(v analyze.Upgrade) string { return v.Candidate.CommonName }, false, true),
		text("candidate/name", func(v analyze.Upgrade) string { return v.Candidate.Name }, false, true),
		text("candidate/position", func(v analyze.Upgrade) string { return string(v.Candidate.Position) }, true, false),
		text("candidate/version", func(v analyze.Upgrade) string { return v.Candidate.Version }, true, false),
		integer("candidate/rating", func(v analyze.Upgrade) int { return v.Candidate.Rating }),
		number("gain", func(v analyze.Upgrade) float64 { return v.Gain }),
		number("efficiency", func(v analyze.Upgrade) float64 { return v.Efficiency }),
		integer("net_cost", func(v analyze.Upgrade) int { return v.NetCost }),
		integer("gross_cost", func(v analyze.Upgrade) int { return v.GrossCost }),
		boolean("affordable", func(v analyze.Upgrade) bool { return v.Affordable }, true),
		boolean("unpriced", func(v analyze.Upgrade) bool { return v.Unpriced }, true),
	)
}

func evolucoesSchema() query.Schema[EvoMatchView] {
	return query.NewSchema("evolucoes", "impact desc,final_gg_rating desc", 200,
		text("slot", func(v EvoMatchView) string { return string(v.Slot) }, true, false),
		text("acquisition", func(v EvoMatchView) string { return v.Acquisition }, true, false),
		text("player/common_name", func(v EvoMatchView) string { return v.Player.CommonName }, false, true),
		text("evolution/name", func(v EvoMatchView) string { return v.Evolution.Name }, false, true),
		integer("result/rating", func(v EvoMatchView) int { return v.Result.Rating }),
		number("impact", func(v EvoMatchView) float64 { return v.Impact }),
		number("final_gg_rating", func(v EvoMatchView) float64 { return v.FinalGGRating }),
		integer("cost", func(v EvoMatchView) int { return v.Cost }),
		boolean("affordable", func(v EvoMatchView) bool { return v.Affordable }, true),
		boolean("beats_starter", func(v EvoMatchView) bool { return v.BeatsStarter }, true),
		timeField("evolution/expires_at", func(v EvoMatchView) time.Time { return v.Evolution.ExpiresAt }),
	)
}

func startersSchema() query.Schema[StarterCard] {
	return query.NewSchema("titulares", "position asc,index asc", 50,
		text("position", func(v StarterCard) string { return string(v.Position) }, true, false),
		text("player/common_name", func(v StarterCard) string { return v.Player.CommonName }, false, true),
		integer("player/rating", func(v StarterCard) int { return v.Player.Rating }),
		number("player/gg_rating", func(v StarterCard) float64 { return v.Player.GGRating }),
		number("position_gg_rating", func(v StarterCard) float64 { return v.PositionGGRating }),
		boolean("player/untradeable", func(v StarterCard) bool { return v.Player.Untradeable }, true),
		integer("index", func(v StarterCard) int { return v.Index }),
	)
}

func reservasSchema() query.Schema[RosterCard] {
	return query.NewSchema("reservas", "player/gg_rating desc,player/rating desc,player/common_name asc", 100,
		text("player/common_name", func(v RosterCard) string { return v.Player.CommonName }, false, true),
		text("player/position", func(v RosterCard) string { return string(v.Player.Position) }, true, false),
		text("player/version", func(v RosterCard) string { return v.Player.Version }, true, false),
		integer("player/rating", func(v RosterCard) int { return v.Player.Rating }),
		number("player/gg_rating", func(v RosterCard) float64 { return v.Player.GGRating }),
		boolean("player/untradeable", func(v RosterCard) bool { return v.Player.Untradeable }, true),
		boolean("player/in_squad", func(v RosterCard) bool { return v.Player.InSquad }, true),
	)
}

func investimentosSchema() query.Schema[analyze.Investment] {
	return query.NewSchema("capital/investimentos", "momentum_pct desc,implied_average desc", 200,
		text("candidate/common_name", func(v analyze.Investment) string { return v.Candidate.CommonName }, false, true),
		text("candidate/version", func(v analyze.Investment) string { return v.Candidate.Version }, true, false),
		text("candidate/position", func(v analyze.Investment) string { return string(v.Candidate.Position) }, true, false),
		integer("candidate/rating", func(v analyze.Investment) int { return v.Candidate.Rating }),
		integer("candidate/price/coins", func(v analyze.Investment) int { return v.Candidate.Price.Coins }),
		number("momentum_pct", func(v analyze.Investment) float64 { return v.MomentumPct }),
		integer("implied_average", func(v analyze.Investment) int { return v.ImpliedAverage }),
		text("signal", func(v analyze.Investment) string { return v.Signal }, true, false),
	)
}

func vendasSchema() query.Schema[analyze.SellCandidate] {
	return query.NewSchema("capital/vendas", "net_sell_value desc,player/rating desc", 200,
		text("player/common_name", func(v analyze.SellCandidate) string { return v.Player.CommonName }, false, true),
		text("player/position", func(v analyze.SellCandidate) string { return string(v.Player.Position) }, true, false),
		text("player/version", func(v analyze.SellCandidate) string { return v.Player.Version }, true, false),
		integer("player/rating", func(v analyze.SellCandidate) int { return v.Player.Rating }),
		text("recommendation", func(v analyze.SellCandidate) string { return v.Recommendation }, true, false),
		integer("net_sell_value", func(v analyze.SellCandidate) int { return v.NetSellValue }),
		number("evo_gg_gain", func(v analyze.SellCandidate) float64 { return v.EvoGGGain }),
		integer("evo_cost", func(v analyze.SellCandidate) int { return v.EvoCost }),
	)
}

func fodderSchema() query.Schema[analyze.FodderSignal] {
	return query.NewSchema("capital/sbcs", "cost_change_pct desc,cost_coins desc", 200,
		text("sbc_name", func(v analyze.FodderSignal) string { return v.SBCName }, false, true),
		text("challenge", func(v analyze.FodderSignal) string { return v.Challenge }, false, true),
		text("requirement", func(v analyze.FodderSignal) string { return v.Requirement }, false, true),
		text("phase", func(v analyze.FodderSignal) string { return v.Phase }, true, false),
		integer("cost_coins", func(v analyze.FodderSignal) int { return v.CostCoins }),
		number("cost_change_pct", func(v analyze.FodderSignal) float64 { return v.CostChangePct }),
		boolean("repeatable", func(v analyze.FodderSignal) bool { return v.Repeatable }, true),
		timeField("expires_at", func(v analyze.FodderSignal) time.Time { return v.ExpiresAt }),
		integer("pool_size", func(v analyze.FodderSignal) int { return v.PoolSize }),
	)
}

func newCardsSchema() query.Schema[domain.Player] {
	return query.NewSchema("hoje/novidades", "released_at desc,common_name asc", 200,
		playerText(func(v domain.Player) string { return v.CommonName }, "common_name", true),
		playerText(func(v domain.Player) string { return v.Name }, "name", true),
		text("position", func(v domain.Player) string { return string(v.Position) }, true, false),
		text("version", func(v domain.Player) string { return v.Version }, true, false),
		text("club", func(v domain.Player) string { return v.Club }, true, false),
		text("league", func(v domain.Player) string { return v.League }, true, false),
		integer("rating", func(v domain.Player) int { return v.Rating }),
		integer("price/coins", func(v domain.Player) int { return v.Price.Coins }),
		timeField("released_at", func(v domain.Player) time.Time { return v.ReleasedAt }),
	)
}

func newsSchema() query.Schema[domain.NewsItem] {
	return query.NewSchema("hoje/noticias", "published_at desc", 100,
		text("title", func(v domain.NewsItem) string { return v.Title }, false, true),
		text("summary", func(v domain.NewsItem) string { return v.Summary }, false, true),
		timeField("published_at", func(v domain.NewsItem) time.Time { return v.PublishedAt }),
	)
}

func activeSBCSchema() query.Schema[domain.SBC] {
	return query.NewSchema("hoje/sbcs", "expires_at asc,name asc", 100,
		text("name", func(v domain.SBC) string { return v.Name }, false, true),
		text("group", func(v domain.SBC) string { return v.Group }, true, false),
		boolean("repeatable", func(v domain.SBC) bool { return v.Repeatable }, true),
		integer("solution_cost", func(v domain.SBC) int { return v.SolutionCost }),
		timeField("expires_at", func(v domain.SBC) time.Time { return v.ExpiresAt }),
	)
}

func objectivesSchema() query.Schema[domain.Objective] {
	return query.NewSchema("hoje/objetivos", "expires_at asc,name asc", 100,
		text("name", func(v domain.Objective) string { return v.Name }, false, true),
		text("group", func(v domain.Objective) string { return v.Group }, true, false),
		integer("est_minutes", func(v domain.Objective) int { return v.EstMinutes }),
		timeField("expires_at", func(v domain.Objective) time.Time { return v.ExpiresAt }),
	)
}

func movimentoSchema() query.Schema[movimentoCard] {
	return query.NewSchema("hoje/movimentacao", "movimento asc,player/common_name asc", 100,
		text("movimento", func(v movimentoCard) string { return v.Movimento }, true, false),
		text("player/common_name", func(v movimentoCard) string { return v.Player.CommonName }, false, true),
		text("player/position", func(v movimentoCard) string { return string(v.Player.Position) }, true, false),
		integer("player/rating", func(v movimentoCard) int { return v.Player.Rating }),
		boolean("player/untradeable", func(v movimentoCard) bool { return v.Player.Untradeable }, true),
	)
}

func historySchema() query.Schema[store.SnapshotSummary] {
	return query.NewSchema("historico", "date desc", 100,
		text("date", func(v store.SnapshotSummary) string { return v.Date }, false, false),
		number("squad_score", func(v store.SnapshotSummary) float64 { return v.SquadScore }),
		integer("coins", func(v store.SnapshotSummary) int { return v.Coins }),
	)
}
