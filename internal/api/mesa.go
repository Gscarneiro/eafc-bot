package api

import (
	"net/http"
	"strconv"

	"github.com/gscarneiro/eafc-bot/internal/domain"
)

// MesaCandidato é uma coluna da "Mesa de decisão": um titular saindo, um
// candidato entrando — do mercado (com custo) ou do banco (custo zero, ver
// analyze.SquadSwap). GGNaPosicao usa a MESMA grandeza nos dois casos
// (Player.GGRatingAt do slot), pra comparar lado a lado sem misturar
// escalas — nunca analyze.Upgrade.Gain aqui (ver CLAUDE.md "duas notas").
type MesaCandidato struct {
	Origem      string                `json:"origem"` // "mercado" | "banco"
	Player      domain.Player         `json:"player"`
	GGNaPosicao float64               `json:"gg_na_posicao"`
	NetCost     int                   `json:"net_cost"`
	Gain        float64               `json:"gain"` // escala de Score() — só pra ordenar/legenda, não pra comparar com o banco
	Efficiency  float64               `json:"efficiency,omitempty"`
	Unpriced    bool                  `json:"unpriced,omitempty"`
	Avaliacao   domain.AvaliacaoCarta `json:"avaliacao,omitempty"`
	// EscolhaDoBot marca o candidato de MERCADO com maior eficiência
	// (ganho por moeda gasta, ver CLAUDE.md) entre os cotados e afordáveis
	// desta mesa — nunca um candidato do banco, que não tem custo pra
	// entrar nessa conta.
	EscolhaDoBot bool `json:"escolha_do_bot"`
}

type MesaResponse struct {
	Index             int                      `json:"index"`
	Position          domain.Position          `json:"position"`
	Current           domain.ClubPlayer        `json:"current"`
	CurrentGG         float64                  `json:"current_gg"`
	Avaliacao         domain.ContextoAvaliacao `json:"avaliacao"`
	CurrentEvaluation domain.AvaliacaoCarta    `json:"current_evaluation,omitempty"`
	Candidatos        []MesaCandidato          `json:"candidatos"`
}

// handleMesa monta a "Mesa de decisão": um slot físico do XI (?index=N,
// Squad.Starters[i].Index — nunca a posição lógica sozinha, que colide em
// formações com duas vagas iguais) contra até 3 candidatos de mercado
// (analyze.Upgrade já limita a MaxPerSlot por slot na coleta) mais 1 do
// banco (analyze.SquadSwap, custo zero). Não recalcula upgrade nem swap —
// só reagrupa o que snap.Upgrades/snap.SquadSwaps já decidiram.
func (s *Server) handleMesa(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.load(w, r)
	if !ok {
		return
	}
	index, err := strconv.Atoi(r.URL.Query().Get("index"))
	if err != nil {
		http.Error(w, "informe ?index= com o slot físico do titular (Squad.Starters[i].Index)", http.StatusBadRequest)
		return
	}
	var slot domain.SquadSlot
	found := false
	for _, candidate := range snap.Club.Squad.Starters {
		if candidate.Index == index {
			slot, found = candidate, true
			break
		}
	}
	if !found {
		http.Error(w, "nenhum titular neste índice de slot", http.StatusNotFound)
		return
	}
	current, _ := snap.Club.PlayerForSlot(slot)
	contexto := snap.Avaliacao
	if specific, ok := contextosAtuaisDasVagas(snap)[slot.Index]; ok {
		contexto = specific
	}
	contexto.Posicao = slot.Position
	currentEvaluation := s.resolveEvaluator().Avaliar(current.Player, slot.Position, contexto)
	currentGG := currentEvaluation.Nota

	var candidatos []MesaCandidato
	for _, u := range snap.Upgrades {
		if u.Slot != slot.Position || u.Current.IdentityKey() != current.IdentityKey() {
			continue
		}
		candidatos = append(candidatos, MesaCandidato{
			Origem: "mercado", Player: u.Candidate, GGNaPosicao: u.CandidateScore,
			NetCost: u.NetCost, Gain: u.Gain, Efficiency: u.Efficiency, Unpriced: u.Unpriced,
			Avaliacao: u.CandidateEvaluation,
		})
	}
	for _, sw := range s.currentSquadSwaps(snap) {
		if sw.Index != index {
			continue
		}
		candidatos = append(candidatos, MesaCandidato{
			Origem: "banco", Player: sw.Candidate.Player, GGNaPosicao: sw.CandidateRating, Gain: sw.GGRatingGap, Avaliacao: sw.CandidateEvaluation,
		})
	}

	bestIdx, bestEfficiency := -1, 0.0
	for i, c := range candidatos {
		if c.Origem != "mercado" || c.Unpriced {
			continue
		}
		if bestIdx == -1 || c.Efficiency > bestEfficiency {
			bestIdx, bestEfficiency = i, c.Efficiency
		}
	}
	if bestIdx >= 0 {
		candidatos[bestIdx].EscolhaDoBot = true
	}

	writeJSON(w, MesaResponse{Index: index, Position: slot.Position, Current: current, CurrentGG: currentGG,
		Avaliacao: snap.Avaliacao, CurrentEvaluation: currentEvaluation, Candidatos: candidatos})
}
