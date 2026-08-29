package analyze

import (
	"fmt"
	"sort"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

type MarketActionKind string

const (
	MarketBuy     MarketActionKind = "comprar"
	MarketSell    MarketActionKind = "vender"
	MarketWait    MarketActionKind = "esperar"
	MarketObserve MarketActionKind = "observar"
)

// MarketActionOrigin identifica uma decisão que nasceu de um módulo mais
// específico que o mercado. A mesa continua sendo o lugar que organiza a
// decisão, sem apagar a procedência que a Agenda precisa exibir.
type MarketActionOrigin string

const (
	MarketActionOriginEvolution MarketActionOrigin = "evolucao"
)

type MarketNeed struct {
	Position domain.Position `json:"position"`
	Reason   string          `json:"reason"`
}

type PriceAssessment struct {
	EAID       int64     `json:"ea_id"`
	Platform   string    `json:"platform,omitempty"`
	Source     string    `json:"source,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
	Coverage   int       `json:"coverage,omitempty"`
	Quality    string    `json:"quality"`
	Stale      bool      `json:"stale"`
}

type MarketAction struct {
	Kind       MarketActionKind   `json:"kind"`
	Origin     MarketActionOrigin `json:"origin,omitempty"`
	EAID       int64              `json:"ea_id,omitempty"`
	Name       string             `json:"name"`
	Position   domain.Position    `json:"position,omitempty"`
	GrossCost  int                `json:"gross_cost"`
	NetCost    int                `json:"net_cost"`
	BreakEven  int                `json:"break_even_gross,omitempty"`
	Confidence string             `json:"confidence"`
	Rationale  []string           `json:"rationale"`
	Conflicts  []string           `json:"conflicts,omitempty"`
	Deadline   *time.Time         `json:"deadline,omitempty"`
	priority   float64
}

type MarketPlanInput struct {
	Capital     domain.Capital
	Needs       []MarketNeed
	Upgrades    []Upgrade
	Evolutions  []EvoMatch
	Sells       []SellCandidate
	Watchlist   []domain.WatchlistEntry
	Prices      map[int64]PriceAssessment
	GeneratedAt time.Time
}

type MarketPlan struct {
	Capital   domain.Capital `json:"capital"`
	Actions   []MarketAction `json:"actions"`
	Conflicts []string       `json:"conflicts,omitempty"`
}

// PlanMarket escolhe o primeiro uso possível do mesmo capital global. A
// função não promete executar nada: compra, venda e espera são instruções
// manuais e a confiança cai quando o preço não é recente ou não é verificável.
func PlanMarket(in MarketPlanInput) MarketPlan {
	remaining := in.Capital.Available
	if in.GeneratedAt.IsZero() {
		in.GeneratedAt = time.Now()
	}
	plan := MarketPlan{Capital: in.Capital}
	protected := make(map[int64]bool, len(in.Watchlist))
	for _, item := range in.Watchlist {
		if item.Protected {
			protected[item.EAID] = true
		}
	}

	// Evoluções vencem uma venda da mesma carta: vender algo que já está
	// reservado para evoluir é um conflito, não uma recomendação silenciosa.
	// O catálogo pode ter dezenas de caminhos para a mesma carta. A mesa de
	// decisão deve propor UMA escolha comparável por jogador; os demais seguem
	// disponíveis no catálogo de evoluções, sem explodir a Agenda.
	evolutions := selectEvolutionsForPlan(in.Evolutions)
	evolving := make(map[int64]bool, len(evolutions))
	for _, evo := range evolutions {
		evolving[evo.Player.ID] = true
	}
	evolutionConflicts := make(map[int64][]string, len(evolutions))
	for _, sell := range in.Sells {
		// Sem cotação, o valor líquido é zero. "Vender por 0" não é uma
		// decisão acionável e escondia as recomendações reais num mar de cards.
		if sell.Recommendation != "vender" || sell.NetSellValue <= 0 {
			continue
		}
		if protected[sell.Player.ID] {
			why := "proteção na watchlist"
			if evolving[sell.Player.ID] {
				why = "evolução candidata para a mesma carta"
			}
			plan.Actions = append(plan.Actions, MarketAction{Kind: MarketWait, EAID: sell.Player.ID, Name: sell.Player.Name, GrossCost: sell.Player.SellValue(), NetCost: sell.NetSellValue, Confidence: "alta", Rationale: []string{"venda adiada para preservar a carta"}, Conflicts: []string{why}})
			plan.Conflicts = append(plan.Conflicts, fmt.Sprintf("%s: venda conflita com %s", sell.Player.Name, why))
			continue
		}
		if evolving[sell.Player.ID] {
			// A decisão principal é a evolução. Colocar também um card de
			// "esperar" para a mesma carta duplicava a mesma escolha na mesa.
			evolutionConflicts[sell.Player.ID] = append(evolutionConflicts[sell.Player.ID], "venda adiada para preservar a carta")
			plan.Conflicts = append(plan.Conflicts, fmt.Sprintf("%s: venda conflita com evolução candidata para a mesma carta", sell.Player.Name))
			continue
		}
		plan.Actions = append(plan.Actions, MarketAction{Kind: MarketSell, EAID: sell.Player.ID, Name: sell.Player.Name, GrossCost: sell.Player.SellValue(), NetCost: sell.NetSellValue, Confidence: "média", Rationale: sell.Rationale})
	}

	chosenPosition := make(map[domain.Position]string)
	for _, upgrade := range in.Upgrades {
		price, hasPrice := in.Prices[upgrade.Candidate.ID]
		action := MarketAction{EAID: upgrade.Candidate.ID, Name: upgrade.Candidate.Name, Position: upgrade.Slot, GrossCost: upgrade.GrossCost, NetCost: upgrade.NetCost, BreakEven: domain.BreakEvenGross(upgrade.GrossCost), Confidence: "alta", Rationale: append([]string(nil), upgrade.Rationale...)}
		if !hasPrice || upgrade.Unpriced || upgrade.GrossCost <= 0 {
			action.Kind, action.Confidence = MarketObserve, "incompleta"
			action.Rationale = append(action.Rationale, "cotação ausente; sem custo confirmado")
		} else if price.Stale {
			action.Kind, action.Confidence = MarketWait, "baixa"
			action.Rationale = append(action.Rationale, "cotação antiga; confirme antes de comprar")
		} else if chosenPosition[upgrade.Slot] != "" {
			action.Kind, action.Confidence = MarketObserve, "média"
			action.Conflicts = []string{"já existe compra planejada para a posição"}
		} else if upgrade.NetCost > remaining {
			action.Kind, action.Confidence = MarketWait, "alta"
			action.Conflicts = []string{"capital disponível não cobre o custo líquido"}
		} else {
			action.Kind = MarketBuy
			remaining -= upgrade.NetCost
			chosenPosition[upgrade.Slot] = upgrade.Candidate.Name
		}
		if len(action.Conflicts) > 0 {
			plan.Conflicts = append(plan.Conflicts, fmt.Sprintf("%s: %s", action.Name, action.Conflicts[0]))
		}
		plan.Actions = append(plan.Actions, action)
	}

	for _, evo := range evolutions {
		action := MarketAction{
			Origin:     MarketActionOriginEvolution,
			EAID:       evo.Player.ID,
			Name:       evo.Evolution.Name + " · " + evo.Player.Name,
			Position:   evo.Slot,
			GrossCost:  evo.Cost,
			NetCost:    evo.Cost,
			Confidence: "alta",
			Rationale:  append([]string(nil), evo.Highlights...),
			Conflicts:  append([]string(nil), evolutionConflicts[evo.Player.ID]...),
			priority:   evo.Gain,
		}
		if !evo.Evolution.ExpiresAt.IsZero() {
			deadline := evo.Evolution.ExpiresAt
			action.Deadline = &deadline
		}
		if chosenPosition[evo.Slot] != "" {
			action.Kind = MarketWait
			action.Conflicts = []string{"compra planejada para a mesma posição"}
			plan.Conflicts = append(plan.Conflicts, fmt.Sprintf("%s: evolução conflita com compra em %s", evo.Player.Name, evo.Slot))
		} else if evo.Cost > remaining {
			action.Kind = MarketWait
			action.Conflicts = []string{"capital disponível já está comprometido no plano"}
		} else {
			action.Kind = MarketObserve
			remaining -= evo.Cost
			action.Rationale = append(action.Rationale, "reserve manualmente o custo da evolução")
		}
		plan.Actions = append(plan.Actions, action)
	}

	// A ordem de saída precisa ser estável para que a mesma entrada não gere
	// uma lista que muda apenas pela ordem incidental de slices.
	sort.SliceStable(plan.Actions, func(i, j int) bool {
		priority := map[MarketActionKind]int{MarketBuy: 0, MarketSell: 1, MarketWait: 2, MarketObserve: 3}
		if priority[plan.Actions[i].Kind] != priority[plan.Actions[j].Kind] {
			return priority[plan.Actions[i].Kind] < priority[plan.Actions[j].Kind]
		}
		if plan.Actions[i].Kind == MarketSell && plan.Actions[i].NetCost != plan.Actions[j].NetCost {
			return plan.Actions[i].NetCost > plan.Actions[j].NetCost
		}
		if plan.Actions[i].Origin == MarketActionOriginEvolution && plan.Actions[j].Origin == MarketActionOriginEvolution && plan.Actions[i].priority != plan.Actions[j].priority {
			return plan.Actions[i].priority > plan.Actions[j].priority
		}
		return plan.Actions[i].Name < plan.Actions[j].Name
	})
	return plan
}

// selectEvolutionsForPlan preserva uma alternativa por carta na mesa. A
// comparação privilegia a troca que melhora o titular, depois o ganho, custo
// e prazo; empates fecham por IDs para a saída não variar entre coletas.
func selectEvolutionsForPlan(matches []EvoMatch) []EvoMatch {
	bestByPlayer := make(map[int64]EvoMatch, len(matches))
	for _, candidate := range matches {
		current, exists := bestByPlayer[candidate.Player.ID]
		if !exists || evolutionPlanComesFirst(candidate, current) {
			bestByPlayer[candidate.Player.ID] = candidate
		}
	}
	out := make([]EvoMatch, 0, len(bestByPlayer))
	for _, evo := range bestByPlayer {
		out = append(out, evo)
	}
	sort.Slice(out, func(i, j int) bool { return evolutionPlanComesFirst(out[i], out[j]) })
	return out
}

func evolutionPlanComesFirst(a, b EvoMatch) bool {
	if a.BeatsStarter != b.BeatsStarter {
		return a.BeatsStarter
	}
	if a.Gain != b.Gain {
		return a.Gain > b.Gain
	}
	if a.Affordable != b.Affordable {
		return a.Affordable
	}
	if a.Cost != b.Cost {
		return a.Cost < b.Cost
	}
	if !a.Evolution.ExpiresAt.Equal(b.Evolution.ExpiresAt) {
		if a.Evolution.ExpiresAt.IsZero() {
			return false
		}
		if b.Evolution.ExpiresAt.IsZero() {
			return true
		}
		return a.Evolution.ExpiresAt.Before(b.Evolution.ExpiresAt)
	}
	if a.Player.Name != b.Player.Name {
		return a.Player.Name < b.Player.Name
	}
	return a.Evolution.ID < b.Evolution.ID
}
