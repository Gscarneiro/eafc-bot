package analyze

import (
	"fmt"
	"sort"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// CollectionCard é a visão aproximada da coleção. Count preserva duplicatas;
// Identity deixa explícito quando a fonte não confirmou cópias físicas.
type CollectionCard struct {
	Player         domain.ClubPlayer `json:"player"`
	Count          int               `json:"count"`
	FirstObserved  time.Time         `json:"first_observed,omitempty"`
	LastObserved   time.Time         `json:"last_observed,omitempty"`
	PermanenceDays int               `json:"permanence_days"`
	Identity       string            `json:"identity"`
	Origin         string            `json:"origin"`
	Source         string            `json:"source,omitempty"`
	ObservedAt     time.Time         `json:"observed_at,omitempty"`
	Protected      bool              `json:"protected"`
	Fodder         bool              `json:"fodder_candidate"`
}

// FodderValue é uma leitura de moeda das cartas fora do XI, nunca uma ordem
// de descarte. Carta sem preço não vira zero: entra em MissingPrices e reduz a
// confiança, pois pode ter valor de SBC ou preço ainda não observado.
type FodderValue struct {
	Cards         int    `json:"cards"`
	Tradeable     int    `json:"tradeable"`
	Untradeable   int    `json:"untradeable"`
	GrossCoins    int    `json:"gross_coins"`
	NetCoins      int    `json:"net_coins"`
	MissingPrices int    `json:"missing_prices"`
	Confidence    string `json:"confidence"`
}

type ClubInsight struct {
	Kind       string       `json:"kind"`
	Headline   string       `json:"headline"`
	Detail     string       `json:"detail"`
	Confidence string       `json:"confidence"`
	Source     string       `json:"source,omitempty"`
	ObservedAt time.Time    `json:"observed_at,omitempty"`
	Score      *BotScore    `json:"bot_score,omitempty"`
	Fodder     *FodderValue `json:"fodder_value,omitempty"`
}

// BuildCollectionMemory reduz os rollups a uma visão por versão de carta. A
// fonte sem ClubItemID só prova contagem por EAID; a identidade das cópias e
// a origem de aquisição ficam deliberadamente desconhecidas.
func BuildCollectionMemory(club domain.Club, rollups []domain.ClubRollup, protected map[int64]bool) []CollectionCard {
	type aggregate struct {
		player      domain.ClubPlayer
		count       int
		allPhysical bool
		anySquad    bool
	}
	byID := make(map[int64]*aggregate)
	for _, player := range club.Players {
		agg := byID[player.ID]
		if agg == nil {
			agg = &aggregate{player: player, allPhysical: player.ClubItemID != ""}
			byID[player.ID] = agg
		} else if player.ClubItemID == "" {
			agg.allPhysical = false
		}
		agg.count++
		agg.anySquad = agg.anySquad || player.InSquad
	}
	first := make(map[int64]time.Time)
	last := make(map[int64]time.Time)
	for _, rollup := range rollups {
		for _, entry := range rollup.Entries {
			if _, exists := byID[entry.EAID]; !exists {
				continue
			}
			if first[entry.EAID].IsZero() || rollup.ObservedAt.Before(first[entry.EAID]) {
				first[entry.EAID] = rollup.ObservedAt
			}
			if rollup.ObservedAt.After(last[entry.EAID]) {
				last[entry.EAID] = rollup.ObservedAt
			}
		}
	}
	out := make([]CollectionCard, 0, len(byID))
	for id, agg := range byID {
		identity := "incompleta"
		if agg.allPhysical {
			identity = "confirmada"
		}
		firstObserved, lastObserved := first[id], last[id]
		if lastObserved.IsZero() || club.SyncedAt.After(lastObserved) {
			lastObserved = club.SyncedAt
		}
		if firstObserved.IsZero() {
			firstObserved = lastObserved
		}
		permanence := 0
		if !firstObserved.IsZero() && !lastObserved.IsZero() {
			permanence = int(lastObserved.Sub(firstObserved).Hours() / 24)
		}
		out = append(out, CollectionCard{
			Player: agg.player, Count: agg.count, FirstObserved: firstObserved, LastObserved: lastObserved,
			PermanenceDays: permanence, Identity: identity, Origin: "desconhecida", Source: club.Source,
			ObservedAt: club.SyncedAt, Protected: protected[id], Fodder: !agg.anySquad,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Player.Rating != out[j].Player.Rating {
			return out[i].Player.Rating > out[j].Player.Rating
		}
		return out[i].Player.Display() < out[j].Player.Display()
	})
	return out
}

func BuildFodderValue(collection []CollectionCard) FodderValue {
	out := FodderValue{Confidence: "confirmada"}
	for _, item := range collection {
		if !item.Fodder || item.Protected {
			continue
		}
		out.Cards += item.Count
		if item.Player.Untradeable {
			out.Untradeable += item.Count
			continue
		}
		if item.Player.Price.Coins <= 0 {
			out.MissingPrices += item.Count
			continue
		}
		out.Tradeable += item.Count
		out.GrossCoins += item.Player.Price.Coins * item.Count
		out.NetCoins += item.Player.NetSellValue() * item.Count
	}
	if out.MissingPrices > 0 {
		out.Confidence = "incompleta"
	}
	return out
}

// BuildClubInsights produz frases curtas e rastreáveis. Um BotScore só é
// comparado com outro de mesmo perfil/ciclo/função; quando não há par válido,
// a função ainda exibe a nota individual sem fingir ranking.
func BuildClubInsights(club domain.Club, collection []CollectionCard) []ClubInsight {
	observedAt, source := club.SyncedAt, club.Source
	var best *BotScore
	var bestName string
	for _, item := range collection {
		pos := item.Player.Position
		if item.Player.InSquad && item.Player.SquadSlot != "" {
			pos = item.Player.SquadSlot
		}
		score := EvaluateBotScore(item.Player.Player, pos, DefaultBotScoreProfile)
		if best == nil {
			candidate := score
			best = &candidate
			bestName = item.Player.Display()
			continue
		}
		if delta, ok := CompareBotScores(score, *best); ok && delta > 0 {
			candidate := score
			best = &candidate
			bestName = item.Player.Display()
		}
	}
	var out []ClubInsight
	if best != nil {
		out = append(out, ClubInsight{Kind: "bot_score", Headline: "BotScore de " + bestName, Detail: fmt.Sprintf("%.2f em %s pelo perfil %s", best.Total, best.Position, best.Profile), Confidence: best.Confidence, Source: source, ObservedAt: observedAt, Score: best})
	}
	fodder := BuildFodderValue(collection)
	out = append(out, ClubInsight{Kind: "fodder_value", Headline: "Valor de cartas fora do XI", Detail: fmt.Sprintf("%d cópias · %d negociáveis · %d coins líquidos observados", fodder.Cards, fodder.Tradeable, fodder.NetCoins), Confidence: fodder.Confidence, Source: source, ObservedAt: observedAt, Fodder: &fodder})
	out = append(out, ClubInsight{Kind: "collection", Headline: "Coleção aproximada", Detail: fmt.Sprintf("%d versões e %d cópias no último retrato", len(collection), countCollectionCopies(collection)), Confidence: collectionConfidence(collection), Source: source, ObservedAt: observedAt})
	return out
}

func countCollectionCopies(collection []CollectionCard) int {
	count := 0
	for _, item := range collection {
		count += item.Count
	}
	return count
}

func collectionConfidence(collection []CollectionCard) string {
	for _, item := range collection {
		if item.Identity != "confirmada" {
			return "incompleta"
		}
	}
	return "confirmada"
}
