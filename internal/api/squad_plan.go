package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/analyze"
	"github.com/gscarneiro/eafc-bot/internal/chemistry"
	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// SquadPlanLockView é o corpo JSON de um analyze.SquadPlanLock.
type SquadPlanLockView struct {
	PlayerID   int64           `json:"player_id"`
	ClubItemID string          `json:"club_item_id,omitempty"`
	Position   domain.Position `json:"position,omitempty"`
}

// SquadPlanFormationView escolhe de onde vem a formação do plano — mesma
// distinção de analyze.FormationSource ("observada" ou "manual"; um
// catálogo preset do FC 27 não existe, ver o comentário lá).
type SquadPlanFormationView struct {
	From  string             `json:"from,omitempty"`
	Slots []domain.SquadSlot `json:"slots,omitempty"` // usado quando From == "manual"
}

// SquadPlanRequestBody é o corpo de POST /api/planos/elenco. Todo campo é
// opcional — omitido cai no padrão de analyze.DefaultSquadPlanRequest.
type SquadPlanRequestBody struct {
	Goal         string                 `json:"goal,omitempty"`
	Formation    SquadPlanFormationView `json:"formation,omitempty"`
	Locks        []SquadPlanLockView    `json:"locks,omitempty"`
	Excluded     []int64                `json:"excluded,omitempty"`
	MaxScenarios int                    `json:"max_scenarios,omitempty"`
}

// SquadPlanStarterView é um titular de cenário pronto pra tela, com o slug
// da carta já resolvido (mesma convenção de RosterCard/StarterCard).
type SquadPlanStarterView struct {
	Index    int               `json:"index"`
	Position domain.Position   `json:"position"`
	Player   domain.ClubPlayer `json:"player"`
	Rating   float64           `json:"rating"`
	CardSlug string            `json:"card_slug,omitempty"`
}

type SquadPlanMoveView struct {
	Index           int                  `json:"index"`
	Position        domain.Position      `json:"position"`
	Current         SquadPlanStarterView `json:"current"`
	Suggested       SquadPlanStarterView `json:"suggested"`
	CurrentRating   float64              `json:"current_rating"`
	SuggestedRating float64              `json:"suggested_rating"`
	Gain            float64              `json:"gain"`
}

// SquadPlanScenarioView é UM ponto da fronteira nota×química, pronto pra
// tela de comparação.
type SquadPlanScenarioView struct {
	Label           string                 `json:"label"`
	ChemistryWeight float64                `json:"chemistry_weight"`
	Starters        []SquadPlanStarterView `json:"starters"`
	TotalRating     float64                `json:"total_rating"`
	AverageRating   float64                `json:"average_rating"`
	Quimica         *chemistry.Resultado   `json:"chemistry,omitempty"`
	Moves           []SquadPlanMoveView    `json:"moves"`
}

// SquadPlanResponse é a tela inteira do Squad Planner: de 3 a 5 cenários
// Pareto, necessidades de mercado (só apontadas, nunca escolhidas — ver
// analyze.SquadPlanNeed) e o capital disponível como contexto.
type SquadPlanResponse struct {
	GeneratedAt time.Time               `json:"generated_at"`
	Status      string                  `json:"status"`
	Reason      string                  `json:"reason,omitempty"`
	Formation   string                  `json:"formation"`
	Scenarios   []SquadPlanScenarioView `json:"scenarios"`
	Needs       []analyze.SquadPlanNeed `json:"needs"`
	Warnings    []string                `json:"warnings,omitempty"`
	Capital     domain.Capital          `json:"capital"`
}

func (s *Server) handleSquadPlan(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}

	var body SquadPlanRequestBody
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, fmt.Sprintf("lendo plano de elenco: %v", err), http.StatusBadRequest)
			return
		}
	}

	req := analyze.DefaultSquadPlanRequest()
	req.ChemistryModel = s.resolveChemistryModel()
	if body.Goal != "" {
		req.Goal = analyze.SquadPlanGoal(body.Goal)
	}
	if body.Formation.From != "" {
		req.FormationFrom = analyze.FormationSource(body.Formation.From)
	}
	req.ManualSlots = body.Formation.Slots
	for _, l := range body.Locks {
		req.Locks = append(req.Locks, analyze.SquadPlanLock{PlayerID: l.PlayerID, ClubItemID: l.ClubItemID, Position: l.Position})
	}
	if len(body.Excluded) > 0 {
		req.Excluded = make(map[int64]bool, len(body.Excluded))
		for _, id := range body.Excluded {
			req.Excluded[id] = true
		}
	}
	if body.MaxScenarios > 0 {
		req.MaxScenarios = body.MaxScenarios
	}

	plan := analyze.BuildSquadPlan(snap.Club, req)

	slugByID := make(map[int64]string, len(snap.Cards))
	for _, c := range snap.Cards {
		slugByID[c.Player.ID] = c.Slug
	}

	writeJSON(w, SquadPlanResponse{
		GeneratedAt: snap.GeneratedAt,
		Status:      plan.Status,
		Reason:      plan.Reason,
		Formation:   plan.Formation,
		Scenarios:   squadPlanScenarioViews(plan.Scenarios, slugByID),
		Needs:       plan.Needs,
		Warnings:    plan.Warnings,
		Capital:     snap.Club.Capital(s.EvolutionExtraBudget, s.MarketReserve, 0),
	})
}

func squadPlanScenarioViews(scenarios []analyze.SquadPlanScenario, slugByID map[int64]string) []SquadPlanScenarioView {
	out := make([]SquadPlanScenarioView, 0, len(scenarios))
	for _, sc := range scenarios {
		view := SquadPlanScenarioView{
			Label: sc.Label, ChemistryWeight: sc.ChemistryWeight,
			TotalRating: sc.TotalRating, AverageRating: sc.AverageRating, Quimica: sc.Quimica,
		}
		for _, a := range sc.Starters {
			view.Starters = append(view.Starters, squadPlanStarterView(a, slugByID))
		}
		for _, m := range sc.Moves {
			view.Moves = append(view.Moves, SquadPlanMoveView{
				Index:    m.Index,
				Position: m.Position,
				Current: squadPlanStarterView(analyze.SquadAssignment{
					Index: m.Index, Position: m.Position, Player: m.Current, Rating: m.CurrentRating,
				}, slugByID),
				Suggested: squadPlanStarterView(analyze.SquadAssignment{
					Index: m.Index, Position: m.Position, Player: m.Suggested, Rating: m.SuggestedRating,
				}, slugByID),
				CurrentRating: m.CurrentRating, SuggestedRating: m.SuggestedRating, Gain: m.Gain,
			})
		}
		out = append(out, view)
	}
	return out
}

func squadPlanStarterView(a analyze.SquadAssignment, slugByID map[int64]string) SquadPlanStarterView {
	return SquadPlanStarterView{
		Index: a.Index, Position: a.Position, Player: a.Player, Rating: a.Rating,
		CardSlug: slugByID[a.Player.ID],
	}
}
